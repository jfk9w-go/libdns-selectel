package selectel

import (
	"context"
	"maps"
	"slices"

	"github.com/libdns/libdns"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

type Provider struct {
	client Client
}

func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	sets, err := p.client.GetRRSets(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get RR sets")
	}

	return slices.Collect(func(yield func(libdns.Record) bool) {
		for _, set := range sets {
			for record := range set.toRecords() {
				if !yield(record) {
					return
				}
			}
		}
	}), nil
}

func (p *Provider) SetRecords(ctx context.Context, zone string, recs []libdns.Record) (result []libdns.Record, errs error) {
	prev, err := p.client.GetRRSets(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get RR sets")
	}

	next := fromRecords(recs)

	del, mod, _ := diffRRSets(prev, next)
	add, _, _ := diffRRSets(next, prev)

	for i := range mod {
		set := &mod[i]
		next := next[set.Key]
		set.TTL = next.TTL
		set.RRs = next.RRs
	}

	added, err := p.client.CreateRRSets(ctx, zone, slices.Values(add))
	if !multierr.AppendInto(&errs, errors.Wrap(err, "create RR sets")) {
		result = slices.AppendSeq(result, toRecords(maps.Values(added)))
	}

	err = p.client.UpdateRRSets(ctx, zone, slices.Values(mod))
	if !multierr.AppendInto(&errs, errors.Wrap(err, "update RR sets")) {
		result = slices.AppendSeq(result, toRecords(slices.Values(mod)))
	}

	err = p.client.DeleteRRSets(ctx, zone, slices.Values(del))
	_ = multierr.AppendInto(&errs, errors.Wrap(err, "delete RR sets"))

	return
}

func (p *Provider) AppendRecords(ctx context.Context, zone string, recs []libdns.Record) (result []libdns.Record, errs error) {
	prev, err := p.client.GetRRSets(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get RR sets")
	}

	next := fromRecords(recs)
	_, mod, _ := diffRRSets(prev, next)
	add, _, _ := diffRRSets(next, prev)

	var radd []libdns.Record
	for i := range mod {
		set := &mod[i]
		next := next[set.Key]
		for _, nrr := range next.RRs {
			found := false
			for i := range set.RRs {
				srr := &set.RRs[i]
				if srr.Content == nrr.Content {
					if srr.Disabled {
						srr.Disabled = false
						radd = append(radd, libdns.RR{
							Name: set.Key.Name,
							Type: set.Key.Type,
							TTL:  set.TTL,
							Data: srr.Content,
						})
					}

					found = true
					break
				}
			}

			if !found {
				set.RRs = append(set.RRs, nrr)
				radd = append(radd, libdns.RR{
					Name: set.Key.Name,
					Type: set.Key.Type,
					TTL:  set.TTL,
					Data: nrr.Content,
				})
			}
		}
	}

	added, err := p.client.CreateRRSets(ctx, zone, slices.Values(add))
	if !multierr.AppendInto(&errs, errors.Wrap(err, "create RR sets")) {
		result = slices.AppendSeq(result, toRecords(maps.Values(added)))
	}

	err = p.client.UpdateRRSets(ctx, zone, slices.Values(mod))
	if !multierr.AppendInto(&errs, errors.Wrap(err, "update RR sets")) {
		result = append(result, radd...)
	}

	return
}

func (p *Provider) DeleteRecords(ctx context.Context, zone string, recs []libdns.Record) ([]libdns.Record, error) {
	//TODO implement me
	panic("implement me")
}

func (p *Provider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	panic("unimplemented")
}

var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
	_ libdns.ZoneLister     = (*Provider)(nil)
)
