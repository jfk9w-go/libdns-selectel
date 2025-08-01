package selectel

import (
	"context"
	"maps"
	"slices"
	"sort"

	"github.com/libdns/libdns"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

type Provider struct {
	client Client
}

func (p *Provider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	zones, err := p.client.GetZones(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get zones")
	}

	result := make([]libdns.Zone, len(zones))
	for i, zone := range zones {
		result[i] = libdns.Zone{
			Name: zone,
		}
	}

	return result, nil
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

func (p *Provider) SetRecords(
	ctx context.Context,
	zone string,
	records []libdns.Record,
) (result []libdns.Record, errs error) {
	prev, err := p.client.GetRRSets(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get RR sets")
	}

	next := fromRecords(records)

	del, mod, _ := diffRRSets(prev, next)
	add, _, _ := diffRRSets(next, prev)

	for i := range mod {
		set := &mod[i]
		next := next[set.Key]
		var rrs []RR
		rrs = append(rrs, next.RRs...)
		for _, srr := range set.RRs {
			if !srr.Disabled {
				continue
			}

			found := false
			for _, nrr := range next.RRs {
				if nrr.Content == srr.Content {
					found = true
					break
				}
			}

			if !found {
				rrs = append(rrs, srr)
			}
		}

		sort.Slice(rrs, func(i, j int) bool { return rrs[i].Content < rrs[j].Content })

		set.TTL = getTTL(set.TTL, next.TTL)
		set.RRs = rrs
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

func (p *Provider) AppendRecords(
	ctx context.Context,
	zone string,
	records []libdns.Record,
) (result []libdns.Record, errs error) {
	prev, err := p.client.GetRRSets(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get RR sets")
	}

	next := fromRecords(records)
	_, mod, _ := diffRRSets(prev, next)
	add, _, _ := diffRRSets(next, prev)

	var radd []libdns.Record
	for i := range mod {
		set := &mod[i]
		next := next[set.Key]
		ttl := getTTL(set.TTL, next.TTL)
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
							TTL:  ttl,
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
					TTL:  ttl,
					Data: nrr.Content,
				})
			}
		}

		sort.Slice(set.RRs, func(i, j int) bool { return set.RRs[i].Content < set.RRs[j].Content })

		set.TTL = ttl
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

func (p *Provider) DeleteRecords(
	ctx context.Context,
	zone string,
	records []libdns.Record,
) (result []libdns.Record, errs error) {
	prev, err := p.client.GetRRSets(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get RR sets")
	}

	next := fromRecords(records)
	_, mod, del := diffRRSets(prev, next)

	var rdel []libdns.Record
	for i := range mod {
		set := &mod[i]
		next := next[set.Key]
		var rrs []RR
		for i := range set.RRs {
			srr := &set.RRs[i]
			found := false
			for _, nrr := range next.RRs {
				if srr.Content == nrr.Content && !srr.Disabled {
					found = true
					break
				}
			}

			if !found {
				rrs = append(rrs, *srr)
			} else {
				rdel = append(rdel, libdns.RR{
					Name: set.Key.Name,
					Type: set.Key.Type,
					TTL:  set.TTL,
					Data: srr.Content,
				})
			}
		}

		sort.Slice(rrs, func(i, j int) bool { return rrs[i].Content < rrs[j].Content })

		set.RRs = rrs
	}

	err = p.client.DeleteRRSets(ctx, zone, slices.Values(del))
	if !multierr.AppendInto(&errs, errors.Wrap(err, "delete RR sets")) {
		result = slices.AppendSeq(result, toRecords(slices.Values(del)))
	}

	err = p.client.UpdateRRSets(ctx, zone, slices.Values(mod))
	if !multierr.AppendInto(&errs, errors.Wrap(err, "update RR sets")) {
		result = slices.AppendSeq(result, toRecords(slices.Values(mod)))
	}

	return
}

var (
	_ libdns.ZoneLister     = (*Provider)(nil)
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
