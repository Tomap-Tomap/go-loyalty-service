package parameters

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	t.Run("test default", func(t *testing.T) {
		p := ParseFlags()

		dp := Parameters{
			RunAddr:           "localhost:8081",
			DataBaseURI:       "host=localhost user=test password=test dbname=loyaltyservice sslmode=disable",
			AccrualSystemAddr: "http://localhost:8080",
			SecretKey:         "secret",
			SecetKeyLife:      time.Hour * 3,
			GetInterval:       5,
			WorkerLimit:       5,
		}

		require.Equal(t, dp, p)
	})

	t.Run("test flags", func(t *testing.T) {
		os.Args = []string{"test", "-a=testA", "-d=testD",
			"-r=testR", "-k=testK", "-kl=5", "-gi=1", "-wl=1"}
		p := ParseFlags()

		dp := Parameters{
			RunAddr:           "testA",
			DataBaseURI:       "testD",
			AccrualSystemAddr: "testR",
			SecretKey:         "testK",
			SecetKeyLife:      time.Hour * 5,
			GetInterval:       1,
			WorkerLimit:       1,
		}

		require.Equal(t, dp, p)
		os.Args = []string{"test"}
	})

	t.Run("test env", func(t *testing.T) {
		os.Setenv("RUN_ADDRESS", "testA")
		os.Setenv("DATABASE_URI", "testD")
		os.Setenv("ACCRUAL_SYSTEM_ADDRESS", "testR")
		os.Setenv("SECRET_KEY", "testK")
		os.Setenv("SECRET_KEY_LIFE", "5")
		os.Setenv("GET_INTERVAL", "1")
		os.Setenv("WORKER_LIMIT", "1")

		p := ParseFlags()

		dp := Parameters{
			RunAddr:           "testA",
			DataBaseURI:       "testD",
			AccrualSystemAddr: "testR",
			SecretKey:         "testK",
			SecetKeyLife:      time.Hour * 5,
			GetInterval:       1,
			WorkerLimit:       1,
		}

		require.Equal(t, dp, p)
		os.Clearenv()
	})
}
