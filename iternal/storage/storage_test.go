package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_isonnectionException(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "random error",
			args: args{errors.New("test")},
			want: false,
		},
		{
			name: "connection error",
			args: args{&pgconn.PgError{Code: "08000"}},
			want: true,
		},
		{
			name: "not connection error",
			args: args{&pgconn.PgError{Code: "02000"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isonnectionException(tt.args.err))
		})
	}
}

func Test_retry2(t *testing.T) {
	t.Run("test no error", func(t *testing.T) {
		t.Parallel()
		rp := retryPolicy{
			retryCount: 3,
			duration:   1,
			increment:  2,
		}
		got, err := retry2[int](context.Background(), rp, func() (int, error) {
			return 0, nil
		})

		require.NoError(t, err)
		require.Equal(t, 0, got)
	})

	t.Run("test no error connection", func(t *testing.T) {
		t.Parallel()
		rp := retryPolicy{
			retryCount: 3,
			duration:   1,
			increment:  2,
		}
		_, err := retry2[*int](context.Background(), rp, func() (*int, error) {
			return nil, &pgconn.PgError{Code: "02000"}
		})

		require.Error(t, err)
	})

	t.Run("test error connection", func(t *testing.T) {
		t.Parallel()
		rp := retryPolicy{
			retryCount: 3,
			duration:   1,
			increment:  2,
		}
		_, err := retry2[*int](context.Background(), rp, func() (*int, error) {
			return nil, &pgconn.PgError{Code: "08000"}
		})

		require.Error(t, err)
	})

	t.Run("test error resolved", func(t *testing.T) {
		t.Parallel()
		rp := retryPolicy{
			retryCount: 3,
			duration:   1,
			increment:  2,
		}

		var errConn error = &pgconn.PgError{Code: "08000"}
		var mu sync.RWMutex

		go func() {
			time.Sleep(5 * time.Second)
			mu.Lock()
			defer mu.Unlock()
			errConn = nil
		}()

		_, err := retry2[*int](context.Background(), rp, func() (*int, error) {
			mu.RLock()
			defer mu.RUnlock()
			return nil, errConn
		})

		require.NoError(t, err)
	})
}
