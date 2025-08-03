package selectel

import (
	"fmt"
	"iter"
	"maps"
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

const (
	enabled  int = 0
	disabled int = 1
)

type RRs = [2]Set[string]

type RRSet struct {
	Key RRSetKey
	ID  string
	TTL time.Duration
	RRs RRs
}

func (s *RRSet) matchEnabledRRs(other *RRSet) bool {
	return maps.Equal(s.RRs[enabled], other.RRs[enabled])
}

func fromSelectel(rrs *v2.RRSet, zone string) *RRSet {
	set := &RRSet{
		Key: RRSetKey{
			Name: libdns.RelativeName(rrs.Name, zone),
			Type: string(rrs.Type),
		},
		ID:  rrs.ID,
		TTL: getTTL(time.Duration(rrs.TTL) * time.Second),
	}

	for _, record := range rrs.Records {
		idx := enabled
		if record.Disabled {
			idx = disabled
		}

		if set.RRs[idx] == nil {
			set.RRs[idx] = make(Set[string])
		}

		set.RRs[idx][record.Content] = true
	}

	return set
}

func (s *RRSet) toSelectel(zone string) *v2.RRSet {
	set := &v2.RRSet{
		ID:   s.ID,
		Name: libdns.AbsoluteName(s.Key.Name, zone),
		Type: v2.RecordType(s.Key.Type),
		TTL:  int(getTTL(s.TTL).Seconds()),
		Records: slices.Collect(func(yield func(v2.RecordItem) bool) {
			for idx := range s.RRs {
				disabled := idx == disabled
				for data := range s.RRs[idx] {
					record := v2.RecordItem{
						Disabled: disabled,
						Content:  data,
					}

					if !yield(record) {
						return
					}
				}
			}
		}),
	}

	// for tests
	sort.Slice(set.Records, func(i, j int) bool { return set.Records[i].Content < set.Records[j].Content })

	return set
}

func fromRecords(records []libdns.Record) map[RRSetKey]*RRSet {
	result := make(map[RRSetKey]*RRSet)
	for _, record := range records {
		key := RRSetKey{
			Name: record.RR().Name,
			Type: record.RR().Type,
		}

		set := result[key]
		if set == nil {
			set = &RRSet{
				Key: key,
				RRs: [2]Set[string]{
					enabled: make(Set[string]),
				},
			}

			result[key] = set
		}

		set.TTL = getTTL(set.TTL, record.RR().TTL)
		set.RRs[enabled][record.RR().Data] = true
	}

	return result
}

func (s *RRSet) toRecords() iter.Seq[libdns.Record] {
	return func(yield func(libdns.Record) bool) {
		for data := range s.RRs[enabled] {
			rr := libdns.RR{
				Name: s.Key.Name,
				Type: s.Key.Type,
				TTL:  getTTL(s.TTL),
				Data: data,
			}

			record, err := rr.Parse()
			if err != nil {
				record = rr
			}

			if !yield(record) {
				return
			}
		}
	}
}
