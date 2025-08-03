package selectel

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/pkg/errors"
	v2 "github.com/selectel/domains-go/pkg/v2"
)

const (
	authTokenURL = "https://cloud.api.selcloud.ru/identity/v3/auth/tokens"
)

type wrapper struct {
	creds Credentials
	dns   v2.DNSClient[v2.Zone, v2.RRSet]
	token struct {
		issuedAt  time.Time
		expiresAt time.Time
	}

	mu sync.RWMutex
}

func newWrapper(creds Credentials) *wrapper {
	headers := make(http.Header)
	headers.Set("User-Agent", "libdns/selectel")

	return &wrapper{
		creds: creds,
		dns:   v2.NewClient(domainsApiURL, new(http.Client), headers),
	}
}

func (w *wrapper) ListZones(ctx context.Context, params *map[string]string) (result v2.Listable[v2.Zone], err error) {
	err = w.execute(ctx, func(dns DNSClient) (err error) {
		result, err = dns.ListZones(ctx, params)
		return
	})

	return
}

func (w *wrapper) ListRRSets(ctx context.Context, zoneID string, params *map[string]string) (result v2.Listable[v2.RRSet], err error) {
	err = w.execute(ctx, func(dns DNSClient) (err error) {
		result, err = dns.ListRRSets(ctx, zoneID, params)
		return
	})

	return
}

func (w *wrapper) CreateRRSet(ctx context.Context, zoneID string, rrset v2.Creatable) (result *v2.RRSet, err error) {
	err = w.execute(ctx, func(dns DNSClient) (err error) {
		result, err = dns.CreateRRSet(ctx, zoneID, rrset)
		return
	})

	return
}

func (w *wrapper) UpdateRRSet(ctx context.Context, zoneID string, rrsetid string, rrset v2.Updatable) error {
	return w.execute(ctx, func(dns DNSClient) (err error) { return dns.UpdateRRSet(ctx, zoneID, rrsetid, rrset) })
}

func (w *wrapper) DeleteRRSet(ctx context.Context, zoneID string, rrsetid string) error {
	return w.execute(ctx, func(dns DNSClient) (err error) { return dns.DeleteRRSet(ctx, zoneID, rrsetid) })
}

func (w *wrapper) authorize(ctx context.Context, issuedAt time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.token.issuedAt != issuedAt {
		return nil
	}

	var in struct {
		Auth struct {
			Identity struct {
				Methods  [1]string `json:"methods"`
				Password struct {
					User struct {
						Name   string `json:"name"`
						Domain struct {
							Name string `json:"name"`
						} `json:"domain"`
						Password string `json:"password"`
					} `json:"user"`
				} `json:"password"`
			} `json:"identity"`
			Scope struct {
				Project struct {
					Name   string `json:"name"`
					Domain struct {
						Name string `json:"name"`
					} `json:"domain"`
				} `json:"project"`
			} `json:"scope"`
		} `json:"auth"`
	}

	in.Auth.Identity.Methods = [1]string{"password"}
	in.Auth.Identity.Password.User.Name = w.creds.Username
	in.Auth.Identity.Password.User.Password = w.creds.Password
	in.Auth.Identity.Password.User.Domain.Name = w.creds.AccountID
	in.Auth.Scope.Project.Name = w.creds.ProjectName
	in.Auth.Scope.Project.Domain.Name = w.creds.AccountID

	body, err := json.Marshal(in)
	if err != nil {
		return errors.Wrap(err, "marshal body")
	}

	return retry(ctx, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, authTokenURL, bytes.NewReader(body))
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		defer discardBody(resp)
		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			return backoff.Permanent(errors.New("invalid credentials"))
		case resp.StatusCode != http.StatusCreated:
			return errors.Errorf("unexpected status code %d", resp.StatusCode)
		}

		token := resp.Header.Get("X-Subject-Token")
		if token == "" {
			return errors.New("missing token")
		}

		headers := make(http.Header)
		headers.Set("X-Auth-Token", token)

		w.dns = w.dns.WithHeaders(headers)

		var out struct {
			Token struct {
				ExpiresAt time.Time `json:"expires_at"`
				IssuedAt  time.Time `json:"issued_at"`
			} `json:"token"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			now := time.Now()
			out.Token.IssuedAt = now
			out.Token.ExpiresAt = now.Add(24 * time.Hour)
		}

		w.token.issuedAt = out.Token.IssuedAt
		w.token.expiresAt = out.Token.ExpiresAt.Add(-time.Hour)

		return nil
	})
}

func (w *wrapper) execute(ctx context.Context, fn func(dns DNSClient) error) error {
	return retry(ctx, func() error {
		for {
			w.mu.RLock()
			dns, token := w.dns, w.token
			w.mu.RUnlock()

			if token.expiresAt.Before(time.Now()) {
				if err := w.authorize(ctx, token.issuedAt); err != nil {
					return err
				}

				continue
			}

			err := fn(dns)
			if err == nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return err
			}

			var bad v2.BadResponseError
			if errors.As(err, &bad) && bad.Code == http.StatusUnauthorized {
				if err := w.authorize(ctx, token.issuedAt); err != nil {
					return err
				}

				continue
			}

			return err
		}
	})
}

func discardBody(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func retry(ctx context.Context, fn func() error) error {
	_, err := backoff.Retry[struct{}](ctx,
		func() (struct{}, error) { return struct{}{}, fn() },
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxTries(3),
		backoff.WithMaxElapsedTime(5*time.Second),
	)

	return err
}
