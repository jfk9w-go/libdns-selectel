package selectel

import (
	"context"
	"slices"
	"testing"
	"time"

	v2 "github.com/selectel/domains-go/pkg/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClient_GetZones(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	dns := NewMockDNSClient(ctrl)
	dns.EXPECT().ListZones(ctx, &map[string]string{
		"offset": "0",
		"limit":  "10",
	}).DoAndReturn(func(context.Context, *map[string]string) (v2.Listable[v2.Zone], error) {
		list := NewMockListable[v2.Zone](ctrl)
		list.EXPECT().GetItems().Return([]*v2.Zone{
			{Name: "zone1.org", ID: "zone1-id"},
			{Name: "zone2.org", ID: "zone2-id"},
		})
		list.EXPECT().GetCount().Return(2)
		return list, nil
	})

	client := &client{
		dns:   dns,
		limit: 10,
		zones: make(map[string]string),
	}

	zones, err := client.GetZones(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"zone1.org", "zone2.org"}, zones)
	assert.Equal(t, map[string]string{
		"zone1.org": "zone1-id",
		"zone2.org": "zone2-id",
	}, client.zones)
}

func TestClient_GetZones_Pagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	dns := NewMockDNSClient(ctrl)
	dns.EXPECT().ListZones(ctx, &map[string]string{
		"offset": "0",
		"limit":  "2",
	}).DoAndReturn(func(context.Context, *map[string]string) (v2.Listable[v2.Zone], error) {
		list := NewMockListable[v2.Zone](ctrl)
		list.EXPECT().GetItems().Return([]*v2.Zone{
			{Name: "zone1.org", ID: "zone1-id"},
			{Name: "zone2.org", ID: "zone2-id"},
		})
		list.EXPECT().GetCount().Return(2)
		list.EXPECT().GetNextOffset().Return(2)
		return list, nil
	})
	dns.EXPECT().ListZones(ctx, &map[string]string{
		"offset": "2",
		"limit":  "2",
	}).DoAndReturn(func(context.Context, *map[string]string) (v2.Listable[v2.Zone], error) {
		list := NewMockListable[v2.Zone](ctrl)
		list.EXPECT().GetItems().Return([]*v2.Zone{
			{Name: "zone3.org", ID: "zone3-id"},
		})
		list.EXPECT().GetCount().Return(1)
		return list, nil
	})

	client := &client{
		dns:   dns,
		limit: 2,
		zones: make(map[string]string),
	}

	zones, err := client.GetZones(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"zone1.org", "zone2.org", "zone3.org"}, zones)
}

func TestClient_GetRRSets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	dns := NewMockDNSClient(ctrl)
	dns.EXPECT().ListZones(ctx, &map[string]string{
		"offset": "0",
		"limit":  "10",
		"filter": "zone1.org",
	}).DoAndReturn(func(context.Context, *map[string]string) (v2.Listable[v2.Zone], error) {
		list := NewMockListable[v2.Zone](ctrl)
		list.EXPECT().GetItems().Return([]*v2.Zone{
			{Name: "zone1.org", ID: "zone1-id"},
		})
		list.EXPECT().GetCount().Return(1)
		return list, nil
	})
	dns.EXPECT().ListRRSets(ctx, "zone1-id", &map[string]string{
		"offset": "0",
		"limit":  "10",
	}).DoAndReturn(func(context.Context, string, *map[string]string) (v2.Listable[v2.RRSet], error) {
		list := NewMockListable[v2.RRSet](ctrl)
		list.EXPECT().GetItems().Return([]*v2.RRSet{
			{
				Name: "rrset1",
				ID:   "rrset1-id",
				TTL:  120,
				Type: "A",
				Records: []v2.RecordItem{
					{Content: "1.1.1.1", Disabled: true},
					{Content: "2.2.2.2"},
				},
			},
			{
				Name: "rrset2",
				ID:   "rrset2-id",
				TTL:  180,
				Type: "CNAME",
				Records: []v2.RecordItem{
					{Content: "rrset1.zone1.org"},
				},
			},
		})
		list.EXPECT().GetCount().Return(2)
		return list, nil
	})

	client := &client{
		dns:   dns,
		limit: 10,
		zones: make(map[string]string),
	}

	sets, err := client.GetRRSets(ctx, "zone1.org")
	require.NoError(t, err)
	assert.Equal(t, map[RRSetKey]RRSet{
		RRSetKey{Name: "rrset1", Type: "A"}: {
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-id",
			TTL: 120 * time.Second,
			RRs: []RR{
				{Content: "1.1.1.1", Disabled: true},
				{Content: "2.2.2.2"},
			},
		},
		RRSetKey{Name: "rrset2", Type: "CNAME"}: {
			Key: RRSetKey{Name: "rrset2", Type: "CNAME"},
			ID:  "rrset2-id",
			TTL: 180 * time.Second,
			RRs: []RR{
				{Content: "rrset1.zone1.org"},
			},
		},
	}, sets)
	assert.Equal(t, map[string]string{
		"zone1.org": "zone1-id",
	}, client.zones)
}

func TestClient_CreateRRSets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	dns := NewMockDNSClient(ctrl)
	dns.EXPECT().CreateRRSet(ctx, "zone1-id", &v2.RRSet{
		Name: "rrset1",
		TTL:  120,
		Type: "A",
		Records: []v2.RecordItem{
			{Content: "1.1.1.1", Disabled: true},
			{Content: "2.2.2.2"},
		},
	}).Return(&v2.RRSet{
		Name: "rrset1",
		ID:   "rrset1-id",
		TTL:  120,
		Type: "A",
		Records: []v2.RecordItem{
			{Content: "1.1.1.1", Disabled: true},
			{Content: "2.2.2.2"},
		},
	}, nil)
	dns.EXPECT().CreateRRSet(ctx, "zone1-id", &v2.RRSet{
		Name: "rrset2",
		TTL:  60,
		Type: "CNAME",
		Records: []v2.RecordItem{
			{Content: "rrset1.zone1.org"},
		},
	}).Return(&v2.RRSet{
		Name: "rrset2",
		ID:   "rrset2-id",
		TTL:  60,
		Type: "CNAME",
		Records: []v2.RecordItem{
			{Content: "rrset1.zone1.org"},
		},
	}, nil)

	client := &client{
		dns:   dns,
		limit: 10,
		zones: map[string]string{
			"zone1.org": "zone1-id",
		},
	}

	sets, err := client.CreateRRSets(ctx, "zone1.org", slices.Values([]RRSet{
		{
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			TTL: 2 * time.Minute,
			RRs: []RR{
				{Content: "1.1.1.1", Disabled: true},
				{Content: "2.2.2.2"},
			},
		},
		{
			Key: RRSetKey{Name: "rrset2", Type: "CNAME"},
			RRs: []RR{
				{Content: "rrset1.zone1.org"},
			},
		},
	}))
	require.NoError(t, err)
	assert.Equal(t, map[RRSetKey]RRSet{
		RRSetKey{Name: "rrset1", Type: "A"}: {
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-id",
			TTL: 120 * time.Second,
			RRs: []RR{
				{Content: "1.1.1.1", Disabled: true},
				{Content: "2.2.2.2"},
			},
		},
		RRSetKey{Name: "rrset2", Type: "CNAME"}: {
			Key: RRSetKey{Name: "rrset2", Type: "CNAME"},
			ID:  "rrset2-id",
			TTL: 60 * time.Second,
			RRs: []RR{
				{Content: "rrset1.zone1.org"},
			},
		},
	}, sets)
}

func TestClient_UpdateRRSets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	dns := NewMockDNSClient(ctrl)
	dns.EXPECT().UpdateRRSet(ctx, "zone1-id", "rrset1-id", &v2.RRSet{
		Name: "rrset1",
		ID:   "rrset1-id",
		TTL:  120,
		Type: "A",
		Records: []v2.RecordItem{
			{Content: "1.1.1.1", Disabled: true},
			{Content: "2.2.2.2"},
		},
	}).Return(nil)

	client := &client{
		dns:   dns,
		limit: 10,
		zones: map[string]string{
			"zone1.org": "zone1-id",
		},
	}

	err := client.UpdateRRSets(ctx, "zone1.org", slices.Values([]RRSet{
		{
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-id",
			TTL: 2 * time.Minute,
			RRs: []RR{
				{Content: "1.1.1.1", Disabled: true},
				{Content: "2.2.2.2"},
			},
		},
	}))
	require.NoError(t, err)
}

func TestClient_DeleteRRSets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	dns := NewMockDNSClient(ctrl)
	dns.EXPECT().DeleteRRSet(ctx, "zone1-id", "rrset1-id").Return(nil)

	client := &client{
		dns:   dns,
		limit: 10,
		zones: map[string]string{
			"zone1.org": "zone1-id",
		},
	}

	err := client.DeleteRRSets(ctx, "zone1.org", slices.Values([]RRSet{
		{ID: "rrset1-id"},
	}))
	require.NoError(t, err)
}
