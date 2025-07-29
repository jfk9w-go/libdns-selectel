package selectel

import (
	"context"
	"iter"
	"strconv"

	v2 "github.com/selectel/domains-go/pkg/v2"
)

type Client interface {
	ListZones(ctx context.Context, name string) iter.Seq2[*v2.Zone, error]
	ListRRSets(ctx context.Context, zoneID string) iter.Seq2[*v2.RRSet, error]
}

type client struct {
	api v2.DNSClient[v2.Zone, v2.RRSet]
}

func (c *client) ListZones(ctx context.Context, name string) iter.Seq2[*v2.Zone, error] {
	return iterate(func(params *map[string]string) (v2.Listable[v2.Zone], error) {
		if params != nil {
			(*params)["filter"] = name
		}

		return c.api.ListZones(ctx, params)
	})
}

func (c *client) ListRRSets(ctx context.Context, zoneID string) iter.Seq2[*v2.RRSet, error] {
	return iterate(func(params *map[string]string) (v2.Listable[v2.RRSet], error) {
		return c.api.ListRRSets(ctx, zoneID, params)
	})
}

func iterate[T any](fn func(params *map[string]string) (v2.Listable[T], error)) iter.Seq2[*T, error] {
	const limit = 10000
	offset := 0
	return func(yield func(*T, error) bool) {
		for {
			params := &map[string]string{
				"offset": strconv.Itoa(offset),
				"limit":  strconv.Itoa(limit),
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

			if resp.GetCount() < limit {
				return
			}

			offset = resp.GetNextOffset()
		}
	}
}
