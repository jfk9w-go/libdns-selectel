package selectel

import (
	"context"
	"sync"
	"time"

	"github.com/libdns/libdns"
	"github.com/pkg/errors"
)

type Provider struct {
	client  Client
	zoneIDs map[string]string
	mu      sync.RWMutex
}

func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	zoneID, err := p.getZoneID(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get zone ID")
	}

	var records []libdns.Record
	for rrs, err := range p.client.ListRRSets(ctx, zoneID) {
		if err != nil {
			return nil, errors.Wrap(err, "list RR sets")
		}

		for _, rr := range rrs.Records {
			if rr.Disabled {
				continue
			}

			records = append(records, Record{
				ID:   rrs.ID,
				Name: rrs.Name,
				TTL:  time.Duration(rrs.TTL) * time.Second,
				Type: string(rrs.Type),
				Data: rr.Content,
			})
		}
	}

	return records, nil
}

func (p *Provider) SetRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) getZoneID(ctx context.Context, name string) (string, error) {
	p.mu.RLock()
	id, ok := p.zoneIDs[name]
	p.mu.RUnlock()

	if ok {
		return id, nil
	}

	for zone, err := range p.client.ListZones(ctx, name) {
		if err != nil {
			return "", errors.Wrap(err, "list zones")
		}

		p.mu.Lock()
		defer p.mu.Unlock()
		p.zoneIDs[name] = zone.ID
		return zone.ID, nil
	}

	return "", errors.New("zone not found")
}

var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
