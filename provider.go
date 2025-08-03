package selectel

import (
	"context"
	"slices"

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

	for key, next := range next {
		_, ok := prev[key]
		if ok {
			continue
		}

		err := p.client.CreateRRSet(ctx, zone, next)
		if !multierr.AppendInto(&errs, errors.Wrapf(err, "create %s", key)) {
			result = append(result, slices.Collect(next.toRecords())...)
		}
	}

	for _, prev := range prev {
		next, ok := next[prev.Key]
		switch {
		case !ok:
			switch {
			case len(prev.RRs[enabled]) == 0:
				continue
			case len(prev.RRs[disabled]) == 0:
				err := p.client.DeleteRRSet(ctx, zone, prev.ID)
				_ = multierr.AppendInto(&errs, errors.Wrapf(err, "delete %s", prev.Key))
				continue
			default:
				next = new(RRSet)
			}

		case !prev.match(next):
			prev.TTL = next.TTL
			prev.RRs[enabled] = next.RRs[enabled]
			for data := range prev.RRs[enabled] {
				delete(prev.RRs[disabled], data)
			}

		default:
			continue
		}

		err := p.client.UpdateRRSet(ctx, zone, prev)
		if !multierr.AppendInto(&errs, errors.Wrapf(err, "update %s", prev.Key)) {
			result = append(result, slices.Collect(prev.toRecords())...)
		}
	}

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

	for key, next := range next {
		_, ok := prev[key]
		if ok {
			continue
		}

		err := p.client.CreateRRSet(ctx, zone, next)
		if !multierr.AppendInto(&errs, errors.Wrapf(err, "create %s", key)) {
			result = append(result, slices.Collect(next.toRecords())...)
		}
	}

	for _, prev := range prev {
		next, ok := next[prev.Key]
		if !ok {
			continue
		}

		var radd []libdns.Record
		update := prev.TTL == next.TTL
		prev.TTL = next.TTL
		for data := range next.RRs[enabled] {
			if prev.RRs[enabled][data] {
				continue
			}

			update = true
			prev.RRs[enabled][data] = true
			delete(prev.RRs[disabled], data)

			rr := libdns.RR{
				Name: prev.Key.Name,
				Type: prev.Key.Type,
				TTL:  prev.TTL,
				Data: data,
			}

			record, err := rr.Parse()
			if err != nil {
				radd = append(radd, rr)
			} else {
				radd = append(radd, record)
			}
		}

		if !update {
			continue
		}

		err := p.client.UpdateRRSet(ctx, zone, prev)
		if !multierr.AppendInto(&errs, errors.Wrapf(err, "update %s", prev.Key)) {
			result = append(result, radd...)
		}
	}

	return
}

func (p *Provider) DeleteRecords(
	ctx context.Context,
	zone string,
	records []libdns.Record,
) (result []libdns.Record, errs error) {
	_, err := p.client.GetRRSets(ctx, zone)
	if err != nil {
		return nil, errors.Wrap(err, "get RR sets")
	}

	_ = fromRecords(records)

	return
}

var (
	_ libdns.ZoneLister     = (*Provider)(nil)
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
