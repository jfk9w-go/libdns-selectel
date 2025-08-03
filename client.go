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

const defaultLimit = 100

type client struct {
	dns   DNSClient
	limit int
	zones map[string]string
	mu    sync.RWMutex
}

func NewClient(dns DNSClient) Client {
	return &client{
		dns:   dns,
		limit: defaultLimit,
		zones: make(map[string]string),
	}
}

func (c *client) GetZones(ctx context.Context) ([]string, error) {
	zoneIDs, err := c.getZoneIDs(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, "get zone IDs")
	}

	return slices.Collect(maps.Keys(zoneIDs)), nil
}

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

func (c *client) UpdateRRSet(ctx context.Context, zone string, set *RRSet) error {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return errors.Wrap(err, "get zone ID")
	}

	return c.dns.UpdateRRSet(ctx, zoneID, set.ID, set.toSelectel(zone))
}

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
