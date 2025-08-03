package selectel

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/libdns/libdns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProvider_ListZones(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := NewMockClient(ctrl)
	client.EXPECT().GetZones(ctx).Return([]string{"zone1.org.", "zone2.org."}, nil)

	provider := &Provider{client: client}
	zones, err := provider.ListZones(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []libdns.Zone{
		{Name: "zone1.org."},
		{Name: "zone2.org."},
	}, zones)
}

func TestProvider_GetRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := NewMockClient(ctrl)
	client.EXPECT().GetRRSets(ctx, "zone1.org.").Return(map[RRSetKey]*RRSet{
		{Name: "rrset1", Type: "A"}: {
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			TTL: time.Hour,
			RRs: RRs{
				enabled:  SetOf("2.2.2.2"),
				disabled: SetOf("1.1.1.1"),
			},
		},
		{Name: "rrset2", Type: "CNAME"}: {
			Key: RRSetKey{Name: "rrset2", Type: "CNAME"},
			TTL: time.Minute,
			RRs: RRs{
				enabled: SetOf("rrset1.zone1.org."),
			},
		},
	}, nil)

	provider := &Provider{client: client}
	records, err := provider.GetRecords(ctx, "zone1.org.")
	require.NoError(t, err)
	assert.ElementsMatch(t, []libdns.Record{
		libdns.Address{Name: "rrset1", TTL: time.Hour, IP: netip.AddrFrom4([4]byte{2, 2, 2, 2})},
		libdns.CNAME{Name: "rrset2", TTL: time.Minute, Target: "rrset1.zone1.org."},
	}, records)
}

func TestProvider_SetRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := NewMockClient(ctrl)

	client.EXPECT().GetRRSets(ctx, "zone1.org.").Return(map[RRSetKey]*RRSet{
		{Name: "rrset1", Type: "A"}: {
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-a",
			TTL: time.Hour,
			RRs: RRs{
				enabled:  SetOf("2.2.2.2"),
				disabled: SetOf("1.1.1.1", "4.4.4.4"),
			},
		},
		{Name: "rrset2", Type: "CNAME"}: {
			Key: RRSetKey{Name: "rrset2", Type: "CNAME"},
			ID:  "rrset2-cname",
			TTL: time.Minute,
			RRs: RRs{
				enabled: SetOf("rrset1.zone1.org."),
			},
		},
		{Name: "rrset4", Type: "TXT"}: {
			Key: RRSetKey{Name: "rrset4", Type: "TXT"},
			ID:  "rrset4-txt",
			TTL: time.Minute,
			RRs: RRs{
				disabled: SetOf("GOODBYE"),
			},
		},
		{Name: "rrset5", Type: "TXT"}: {
			Key: RRSetKey{Name: "rrset5", Type: "TXT"},
			ID:  "rrset5-txt",
			TTL: time.Minute,
			RRs: RRs{
				enabled: SetOf("HELLO"),
			},
		},
	}, nil)

	for _, set := range []*RRSet{
		{
			Key: RRSetKey{Name: "rrset3", Type: "TXT"},
			TTL: time.Minute,
			RRs: RRs{
				enabled: SetOf("HELLO"),
			},
		},
		{
			Key: RRSetKey{Name: "rrset1", Type: "CNAME"},
			TTL: time.Minute,
			RRs: RRs{
				enabled: SetOf("rrset3.zone1.org."),
			},
		},
	} {
		client.EXPECT().CreateRRSet(ctx, "zone1.org.", set).Return(nil)
	}

	for _, set := range []*RRSet{
		{
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-a",
			TTL: time.Hour,
			RRs: RRs{
				enabled:  SetOf("1.1.1.1", "3.3.3.3"),
				disabled: SetOf("4.4.4.4"),
			},
		},
	} {
		client.EXPECT().UpdateRRSet(ctx, "zone1.org.", set).Return(nil)
	}

	for _, setID := range []string{
		"rrset2-cname",
	} {
		client.EXPECT().DeleteRRSet(ctx, "zone1.org.", setID).Return(nil)
	}

	provider := &Provider{client: client}
	records, err := provider.SetRecords(ctx, "zone1.org.", []libdns.Record{
		libdns.Address{
			Name: "rrset1",
			TTL:  time.Hour,
			IP:   netip.AddrFrom4([4]byte{1, 1, 1, 1}),
		},
		libdns.Address{
			Name: "rrset1",
			TTL:  2 * time.Hour,
			IP:   netip.AddrFrom4([4]byte{3, 3, 3, 3}),
		},
		libdns.CNAME{
			Name:   "rrset1",
			TTL:    time.Minute,
			Target: "rrset3.zone1.org.",
		},
		libdns.TXT{
			Name: "rrset3",
			TTL:  time.Minute,
			Text: "HELLO",
		},
		libdns.TXT{
			Name: "rrset5",
			TTL:  time.Minute,
			Text: "HELLO",
		},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []libdns.Record{
		libdns.Address{Name: "rrset1", TTL: time.Hour, IP: netip.AddrFrom4([4]byte{1, 1, 1, 1})},
		libdns.Address{Name: "rrset1", TTL: time.Hour, IP: netip.AddrFrom4([4]byte{3, 3, 3, 3})},
		libdns.CNAME{Name: "rrset1", TTL: time.Minute, Target: "rrset3.zone1.org."},
		libdns.TXT{Name: "rrset3", TTL: time.Minute, Text: "HELLO"},
	}, records)
}

func TestProvider_AppendRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := NewMockClient(ctrl)

	client.EXPECT().GetRRSets(ctx, "zone1.org.").Return(map[RRSetKey]*RRSet{
		{Name: "rrset1", Type: "A"}: {
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-a",
			TTL: time.Hour,
			RRs: RRs{
				enabled:  SetOf("2.2.2.2"),
				disabled: SetOf("1.1.1.1", "4.4.4.4"),
			},
		},
		{Name: "rrset2", Type: "CNAME"}: {
			Key: RRSetKey{Name: "rrset2", Type: "CNAME"},
			ID:  "rrset2-cname",
			TTL: time.Minute,
			RRs: RRs{
				enabled: SetOf("rrset1.zone1.org."),
			},
		},
		{Name: "rrset4", Type: "TXT"}: {
			Key: RRSetKey{Name: "rrset4", Type: "TXT"},
			ID:  "rrset4-txt",
			TTL: time.Minute,
			RRs: RRs{
				disabled: SetOf("GOODBYE"),
			},
		},
	}, nil)

	for _, set := range []*RRSet{
		{
			Key: RRSetKey{Name: "rrset3", Type: "TXT"},
			TTL: time.Minute,
			RRs: RRs{
				enabled: SetOf("HELLO"),
			},
		},
	} {
		client.EXPECT().CreateRRSet(ctx, "zone1.org.", set).Return(nil)
	}

	for _, set := range []*RRSet{
		{
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-a",
			TTL: time.Hour,
			RRs: RRs{
				enabled:  SetOf("2.2.2.2", "3.3.3.3", "4.4.4.4"),
				disabled: SetOf("1.1.1.1"),
			},
		},
	} {
		client.EXPECT().UpdateRRSet(ctx, "zone1.org.", set).Return(nil)
	}

	provider := &Provider{client: client}
	records, err := provider.AppendRecords(ctx, "zone1.org.", []libdns.Record{
		libdns.Address{
			Name: "rrset1",
			TTL:  time.Hour,
			IP:   netip.AddrFrom4([4]byte{2, 2, 2, 2}),
		},
		libdns.Address{
			Name: "rrset1",
			TTL:  time.Hour,
			IP:   netip.AddrFrom4([4]byte{3, 3, 3, 3}),
		},
		libdns.Address{
			Name: "rrset1",
			TTL:  2 * time.Hour,
			IP:   netip.AddrFrom4([4]byte{4, 4, 4, 4}),
		},
		libdns.TXT{
			Name: "rrset3",
			TTL:  time.Minute,
			Text: "HELLO",
		},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []libdns.Record{
		libdns.Address{Name: "rrset1", TTL: time.Hour, IP: netip.AddrFrom4([4]byte{3, 3, 3, 3})},
		libdns.Address{Name: "rrset1", TTL: time.Hour, IP: netip.AddrFrom4([4]byte{4, 4, 4, 4})},
		libdns.TXT{Name: "rrset3", TTL: time.Minute, Text: "HELLO"},
	}, records)
}

func TestProvider_DeleteRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	client := NewMockClient(ctrl)

	client.EXPECT().GetRRSets(ctx, "zone1.org.").Return(map[RRSetKey]*RRSet{
		{Name: "rrset1", Type: "A"}: {
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-a",
			TTL: time.Hour,
			RRs: RRs{
				enabled:  SetOf("2.2.2.2"),
				disabled: SetOf("1.1.1.1", "4.4.4.4"),
			},
		},
		{Name: "rrset2", Type: "CNAME"}: {
			Key: RRSetKey{Name: "rrset2", Type: "CNAME"},
			ID:  "rrset2-cname",
			TTL: time.Minute,
			RRs: RRs{
				enabled: SetOf("rrset1.zone1.org."),
			},
		},
		{Name: "rrset4", Type: "TXT"}: {
			Key: RRSetKey{Name: "rrset4", Type: "TXT"},
			ID:  "rrset4-txt",
			TTL: time.Minute,
			RRs: RRs{
				enabled:  SetOf("HELLO"),
				disabled: SetOf("GOODBYE"),
			},
		},
	}, nil)

	for _, setID := range []string{
		"rrset2-cname",
	} {
		client.EXPECT().DeleteRRSet(ctx, "zone1.org.", setID).Return(nil)
	}

	for _, set := range []*RRSet{
		{
			Key: RRSetKey{Name: "rrset1", Type: "A"},
			ID:  "rrset1-a",
			TTL: time.Hour,
			RRs: RRs{
				enabled:  SetOf[string](),
				disabled: SetOf("1.1.1.1", "4.4.4.4"),
			},
		},
		{
			Key: RRSetKey{Name: "rrset4", Type: "TXT"},
			ID:  "rrset4-txt",
			TTL: time.Minute,
			RRs: RRs{
				enabled:  SetOf[string](),
				disabled: SetOf("GOODBYE"),
			},
		},
	} {
		client.EXPECT().UpdateRRSet(ctx, "zone1.org.", set).Return(nil)
	}

	provider := &Provider{client: client}
	records, err := provider.DeleteRecords(ctx, "zone1.org.", []libdns.Record{
		libdns.Address{
			Name: "rrset1",
			TTL:  time.Hour,
			IP:   netip.AddrFrom4([4]byte{2, 2, 2, 2}),
		},
		libdns.Address{
			Name: "rrset1",
			TTL:  time.Hour,
			IP:   netip.AddrFrom4([4]byte{3, 3, 3, 3}),
		},
		libdns.Address{
			Name: "rrset1",
			TTL:  2 * time.Hour,
			IP:   netip.AddrFrom4([4]byte{4, 4, 4, 4}),
		},
		libdns.CNAME{
			Name:   "rrset2",
			TTL:    time.Minute,
			Target: "rrset1.zone1.org.",
		},
		libdns.RR{
			Name: "rrset4",
		},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []libdns.Record{
		libdns.Address{Name: "rrset1", TTL: time.Hour, IP: netip.AddrFrom4([4]byte{2, 2, 2, 2})},
		libdns.CNAME{Name: "rrset2", TTL: time.Minute, Target: "rrset1.zone1.org."},
		libdns.TXT{Name: "rrset4", TTL: time.Minute, Text: "HELLO"},
	}, records)
}
