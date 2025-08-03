//go:build integration

package selectel

import (
	"context"
	"testing"

	"github.com/caarlos0/env/v11"
	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	"github.com/libdns/libdns"
	"github.com/stretchr/testify/require"
)

func TestProvider(t *testing.T) {
	err := godotenv.Load()
	require.NoError(t, err)

	var creds struct {
		Username    string `env:"USERNAME,required"`
		Password    string `env:"PASSWORD,required"`
		AccountID   string `env:"ACCOUNT_ID,required"`
		ProjectName string `env:"PROJECT_NAME,required"`
	}

	err = env.Parse(&creds)
	require.NoError(t, err)

	provider := NewProvider(NewClient(Credentials{
		Username:    creds.Username,
		Password:    creds.Password,
		AccountID:   creds.AccountID,
		ProjectName: creds.ProjectName,
	}))

	ctx := context.Background()

	zones, err := provider.ListZones(ctx)
	require.NoError(t, err)
	spew.Dump(zones)

	if len(zones) > 0 {
		records, err := provider.GetRecords(ctx, zones[0].Name)
		require.NoError(t, err)
		spew.Dump(records)

		records, err = provider.AppendRecords(ctx, zones[0].Name, []libdns.Record{
			libdns.TXT{Name: "libdns-integration-test", Text: `4"5"6`},
		})

		require.NoError(t, err)
		spew.Dump(records)
	}
}
