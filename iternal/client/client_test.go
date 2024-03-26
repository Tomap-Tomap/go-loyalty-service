package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tomap-Tomap/go-loyalty-service/iternal/models"
	"github.com/stretchr/testify/require"
)

func TestClient_GetOrder(t *testing.T) {
	handlerBadReques := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "test error", http.StatusBadRequest)
	}

	t.Run("test status not OK", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(handlerBadReques))
		defer ts.Close()

		c := NewClient(ts.URL)
		_, err := c.GetOrder(context.Background(), "error")

		require.Error(t, err)
	})

	handlerTooManyRequests := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}

	t.Run("test too many request", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(handlerTooManyRequests))
		defer ts.Close()

		c := NewClient(ts.URL)
		_, err := c.GetOrder(context.Background(), "error")

		require.Error(t, err)
	})

	handlerErrorBody := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("error"))
	}

	t.Run("test bad body", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(handlerErrorBody))
		defer ts.Close()

		c := NewClient(ts.URL)
		_, err := c.GetOrder(context.Background(), "error")

		require.Error(t, err)
	})

	handlerOK := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"order": "test",
			"status": "PROCESSED",
			"accrual": 500
		}`))
	}

	accrual := float64(500)
	order := &models.Order{
		Number:  "test",
		Status:  "PROCESSED",
		Accrual: &accrual,
	}

	t.Run("test OK", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(handlerOK))
		defer ts.Close()

		c := NewClient(ts.URL)
		o, err := c.GetOrder(context.Background(), "test")

		require.NoError(t, err)
		require.Equal(t, order, o)
	})

	t.Run("test brocken server", func(t *testing.T) {
		c := NewClient("test")
		_, err := c.GetOrder(context.Background(), "test")

		require.Error(t, err)
	})
}
