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
	"go.uber.org/multierr"
)

type Client interface {
	GetZones(ctx context.Context) ([]string, error)
	GetRRSets(ctx context.Context, zone string) (map[RRSetKey]RRSet, error)
	CreateRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) (map[RRSetKey]RRSet, error)
	UpdateRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) error
	DeleteRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) error
}

type client struct {
	api     v2.DNSClient[v2.Zone, v2.RRSet]
	zoneIDs map[string]string
	mu      sync.RWMutex
}

func (c *client) GetZones(ctx context.Context) ([]string, error) {
	zoneIDs, err := c.getZoneIDs(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, "get zone IDs")
	}

	return slices.Collect(maps.Keys(zoneIDs)), nil
}

func (c *client) GetRRSets(ctx context.Context, zone string) (map[RRSetKey]RRSet, error) {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get zone ID")
	}

	iterator := iterate(func(params *map[string]string) (v2.Listable[v2.RRSet], error) {
		return c.api.ListRRSets(ctx, zoneID, params)
	})

	result := make(map[RRSetKey]RRSet)
	for rrs, err := range iterator {
		if err != nil {
			return nil, errors.Wrap(err, "get RR sets")
		}

		set := fromSelectel(rrs)
		result[set.Key] = set
	}

	return result, nil
}

func (c *client) CreateRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) (result map[RRSetKey]RRSet, errs error) {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get zone ID")
	}

	result = make(map[RRSetKey]RRSet)
	for set := range sets {
		rrs, err := c.api.CreateRRSet(ctx, zoneID, set.toSelectel())
		if !multierr.AppendInto(&errs, errors.Wrapf(err, "create %s", set.Key)) {
			set := fromSelectel(rrs)
			result[set.Key] = set
		}
	}

	return
}

func (c *client) UpdateRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) (errs error) {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return errors.Wrap(err, "get zone ID")
	}

	for set := range sets {
		err := c.api.UpdateRRSet(ctx, zoneID, set.ID, set.toSelectel())
		_ = multierr.AppendInto(&errs, errors.Wrapf(err, "update %s", set.Key))
	}

	return
}

func (c *client) DeleteRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) (errs error) {
	zoneID, err := c.getZoneID(ctx, zone)
	if err != nil {
		return errors.Wrap(err, "get zone ID")
	}

	for set := range sets {
		err := c.api.DeleteRRSet(ctx, zoneID, set.ID)
		_ = multierr.AppendInto(&errs, errors.Wrapf(err, "delete %s", set.Key))
	}

	return
}

func (c *client) getZoneID(ctx context.Context, name string) (string, error) {
	c.mu.RLock()
	zoneID, ok := c.zoneIDs[name]
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
	iterator := iterate(func(params *map[string]string) (v2.Listable[v2.Zone], error) {
		if params != nil && name != "" {
			(*params)["filter"] = name
		}

		return c.api.ListZones(ctx, params)
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
	maps.Copy(c.zoneIDs, result)

	return result, nil
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
