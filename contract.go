//go:generate mockgen -destination mocks.go -package selectel . DNSClient,Client,Listable
package selectel

import (
	"context"
	"iter"

	v2 "github.com/selectel/domains-go/pkg/v2"
)

type Listable[T any] = v2.Listable[T]

type DNSClient interface {
	ListZones(ctx context.Context, params *map[string]string) (Listable[v2.Zone], error)
	ListRRSets(ctx context.Context, zoneID string, params *map[string]string) (Listable[v2.RRSet], error)
	CreateRRSet(ctx context.Context, zoneID string, rrset v2.Creatable) (*v2.RRSet, error)
	UpdateRRSet(ctx context.Context, zoneID string, rrsetid string, rrset v2.Updatable) error
	DeleteRRSet(ctx context.Context, zoneID string, rrsetid string) error
}

type Client interface {
	GetZones(ctx context.Context) ([]string, error)
	GetRRSets(ctx context.Context, zone string) (map[RRSetKey]RRSet, error)
	CreateRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) (map[RRSetKey]RRSet, error)
	UpdateRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) error
	DeleteRRSets(ctx context.Context, zone string, sets iter.Seq[RRSet]) error
}
