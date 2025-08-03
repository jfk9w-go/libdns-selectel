package selectel

import (
	"context"
	"iter"
	"maps"
	"slices"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	v2 "github.com/selectel/domains-go/pkg/v2"
)

const (
	domainsApiURL = "https://api.selectel.ru/domains/v2"
	defaultLimit  = 100
)

// Credentials describes data required for obtaining project-scoped API token.
// See https://docs.selectel.ru/en/api/authorization/#iam-token-project-scoped
type Credentials struct {
	// Service user name.
	Username string
	// Service user password.
	Password string
	// Your account ID.
	AccountID string
	// Name of the project containing required zones.
	ProjectName string
}

type client struct {
	dns   DNSClient
	limit int
	zones map[string]string
	mu    sync.RWMutex
}

// NewClient creates a Selectel DNS API client.
// It handles retries and obtaining a project-scoped token for managing DNS zones & records.
func NewClient(creds Credentials) Client {
	return &client{
		dns:   newWrapper(creds),
		limit: defaultLimit,
		zones: make(map[string]string),
	}
}

// GetZones retrieves zone names and IDs for the project and caches them.
func (c *client) GetZones(ctx context.Context) ([]string, error) {
	zoneIDs, err := c.getZoneIDs(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, "get zone IDs")
	}

	return slices.Collect(maps.Keys(zoneIDs)), nil
}

// GetRRSets retrieves RR sets for the specified zone name.
func (c *client) GetRRSets(ctx context.Context, zone string) (map[RRSetKey]*RRSet, error) {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get zone ID")
	}

	iterator := iterate(c, func(params *map[string]string) (v2.Listable[v2.RRSet], error) {
		return c.dns.ListRRSets(ctx, zoneID, params)
	})

	result := make(map[RRSetKey]*RRSet)
	for rrs, err := range iterator {
		if err != nil {
			return nil, errors.Wrap(err, "get RR sets")
		}

		set := fromSelectel(rrs, zone)
		result[set.Key] = set
	}

	return result, nil
}

// CreateRRSet creates a RR set in the specified zone name.
// If successful, set ID will be set in the provided set.
func (c *client) CreateRRSet(ctx context.Context, zone string, set *RRSet) error {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return errors.Wrap(err, "get zone ID")
	}

	rrs, err := c.dns.CreateRRSet(ctx, zoneID, set.toSelectel(zone))
	if err != nil {
		return err
	}

	set.ID = rrs.ID
	return nil
}

// UpdateRRSet updates a RR set in the specified zone name.
func (c *client) UpdateRRSet(ctx context.Context, zone string, set *RRSet) error {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return errors.Wrap(err, "get zone ID")
	}

	return c.dns.UpdateRRSet(ctx, zoneID, set.ID, set.toSelectel(zone))
}

// DeleteRRSet deletes a RR set with the specified ID in the specified zone name.
func (c *client) DeleteRRSet(ctx context.Context, zone string, setID string) error {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return errors.Wrap(err, "get zone ID")
	}

	return c.dns.DeleteRRSet(ctx, zoneID, setID)
}

func (c *client) getZoneID(ctx context.Context, name string) (string, error) {
	c.mu.RLock()
	zoneID, ok := c.zones[name]
	c.mu.RUnlock()

	if ok {
		return zoneID, nil
	}

	zoneIDs, err := c.getZoneIDs(ctx, name)
	if err != nil {
		return "", errors.Wrap(err, "get zone IDs")
	}

	zoneID, ok = zoneIDs[name]
	if !ok {
		return "", errors.New("zone not found")
	}

	return zoneID, nil
}

func (c *client) getZoneIDs(ctx context.Context, name string) (map[string]string, error) {
	iterator := iterate(c, func(params *map[string]string) (v2.Listable[v2.Zone], error) {
		if params != nil && name != "" {
			(*params)["filter"] = name
		}

		return c.dns.ListZones(ctx, params)
	})

	result := make(map[string]string)
	for zone, err := range iterator {
		if err != nil {
			return nil, errors.Wrap(err, "get zones")
		}

		result[zone.Name] = zone.ID
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	maps.Copy(c.zones, result)

	return result, nil
}

func iterate[T any](c *client, fn func(params *map[string]string) (v2.Listable[T], error)) iter.Seq2[*T, error] {
	offset := 0
	return func(yield func(*T, error) bool) {
		for {
			params := &map[string]string{
				"offset": strconv.Itoa(offset),
				"limit":  strconv.Itoa(c.limit),
			}

			resp, err := fn(params)
			if err != nil {
				yield(nil, err)
				return
			}

			for _, item := range resp.GetItems() {
				if !yield(item, nil) {
					return
				}
			}

			if resp.GetCount() < c.limit {
				return
			}

			offset = resp.GetNextOffset()
		}
	}
}
