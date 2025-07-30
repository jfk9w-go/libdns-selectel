package selectel

import (
	"fmt"
	"iter"
	"reflect"
	"slices"
	"sort"
	"time"

	"github.com/libdns/libdns"
	v2 "github.com/selectel/domains-go/pkg/v2"
)

type RRSetKey struct {
	Name string
	Type string
}

func (k RRSetKey) String() string {
	return fmt.Sprintf("%s %s", k.Type, k.Name)
}

type RR struct {
	Disabled bool
	Content  string
}

type RRSet struct {
	Key RRSetKey
	ID  string
	TTL time.Duration
	RRs []RR
}

func fromSelectel(rrs *v2.RRSet) RRSet {
	set := RRSet{
		Key: RRSetKey{
			Name: rrs.Name,
			Type: string(rrs.Type),
		},
		ID:  rrs.ID,
		TTL: time.Duration(rrs.TTL) * time.Second,
		RRs: slices.Collect(func(yield func(RR) bool) {
			for _, item := range rrs.Records {
				rr := RR{
					Disabled: item.Disabled,
					Content:  item.Content,
				}

				if !yield(rr) {
					return
				}
			}
		}),
	}

	sort.Slice(set.RRs, func(i, j int) bool { return set.RRs[i].Content < set.RRs[j].Content })

	return set
}

func (s RRSet) isEqual(other RRSet) bool {
	a, b := s.toRecords(), other.toRecords()
	return reflect.DeepEqual(a, b)
}

func (s RRSet) toSelectel() *v2.RRSet {
	return &v2.RRSet{
		ID:   s.ID,
		Name: s.Key.Name,
		Type: v2.RecordType(s.Key.Type),
		TTL:  int(max(s.TTL, time.Minute).Seconds()),
		Records: slices.Collect(func(yield func(v2.RecordItem) bool) {
			for _, rr := range s.RRs {
				item := v2.RecordItem{
					Disabled: rr.Disabled,
					Content:  rr.Content,
				}

				if !yield(item) {
					return
				}
			}
		}),
	}
}

func fromRecords(records []libdns.Record) map[RRSetKey]RRSet {
	result := make(map[RRSetKey]RRSet)
	for _, record := range records {
		key := RRSetKey{
			Name: record.RR().Name,
			Type: record.RR().Type,
		}

		set := result[key]

		set.Key = key
		set.TTL = max(set.TTL, record.RR().TTL)
		set.RRs = append(set.RRs, RR{Content: record.RR().Data})
		sort.Slice(set.RRs, func(i, j int) bool { return set.RRs[i].Content < set.RRs[j].Content })

		result[key] = set
	}

	return result
}

func (s RRSet) toRecords() iter.Seq[libdns.Record] {
	return func(yield func(libdns.Record) bool) {
		for _, rr := range s.RRs {
			if rr.Disabled {
				continue
			}

			record := libdns.RR{
				Name: s.Key.Name,
				Type: s.Key.Type,
				TTL:  s.TTL,
				Data: rr.Content,
			}

			if !yield(record) {
				return
			}
		}
	}
}

type RRSetDiff struct {
	Removed  []RRSet
	Modified []RRSet
	Matched  []RRSet
}

func diffRRSets(prev, next map[RRSetKey]RRSet) (del []RRSet, mod []RRSet, same []RRSet) {
	for key, prev := range prev {
		next, ok := next[key]
		switch {
		case !ok:
			del = append(del, prev)
		case !prev.isEqual(next):
			mod = append(mod, prev)
		default:
			same = append(same, prev)
		}
	}

	return
}

func toRecords(sets iter.Seq[RRSet]) iter.Seq[libdns.Record] {
	return func(yield func(libdns.Record) bool) {
		for set := range sets {
			for record := range set.toRecords() {
				if !yield(record) {
					return
				}
			}
		}
	}
}
