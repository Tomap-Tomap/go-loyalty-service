package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestOrder_ScanRow(t *testing.T) {
	t.Run("test error", func(t *testing.T) {
		ro := new(RowsMockedObject)
		ro.On("Values").Return(nil, fmt.Errorf("test"))
		o := new(Order)
		err := o.ScanRow(ro)
		require.Error(t, err)
		ro.AssertExpectations(t)
	})

	t.Run("no accrual", func(t *testing.T) {
		ro := new(RowsMockedObject)
		curTime := time.Now()
		ro.On("Values").Return([]any{"test", "test", curTime, "test"}, nil)
		ro.On("FieldDescriptions").Return([]pgconn.FieldDescription{
			{Name: "number"},
			{Name: "status"},
			{Name: "uploadedat"},
			{Name: "test"},
		}, nil)
		o := new(Order)
		err := o.ScanRow(ro)
		require.NoError(t, err)
		require.Equal(t, Order{Number: "test", Status: "test", UploadedAt: &curTime}, *o)
		ro.AssertExpectations(t)
	})

	t.Run("full fields", func(t *testing.T) {
		ro := new(RowsMockedObject)
		curTime := time.Now()
		accrual := float64(65)
		ro.On("Values").Return([]any{"test", "test", accrual, curTime}, nil)
		ro.On("FieldDescriptions").Return([]pgconn.FieldDescription{
			{Name: "number"},
			{Name: "status"},
			{Name: "accrual"},
			{Name: "uploadedat"},
			{Name: "test"},
		}, nil)
		o := new(Order)
		err := o.ScanRow(ro)
		require.NoError(t, err)
		require.Equal(t, Order{Number: "test", Status: "test", UploadedAt: &curTime, Accrual: &accrual}, *o)
		ro.AssertExpectations(t)
	})
}

func TestOrderBalance_ScanRow(t *testing.T) {
	t.Run("test error", func(t *testing.T) {
		ro := new(RowsMockedObject)
		ro.On("Values").Return(nil, fmt.Errorf("test"))
		ob := new(OrderBalance)
		err := ob.ScanRow(ro)
		require.Error(t, err)
		ro.AssertExpectations(t)
	})

	t.Run("full fields", func(t *testing.T) {
		ro := new(RowsMockedObject)
		curTime := time.Now()
		sum := float64(65)
		ro.On("Values").Return([]any{"test", sum, curTime}, nil)
		ro.On("FieldDescriptions").Return([]pgconn.FieldDescription{
			{Name: "order"},
			{Name: "sum"},
			{Name: "processedat"},
			{Name: "uploadedat"},
			{Name: "test"},
		}, nil)
		ob := new(OrderBalance)
		err := ob.ScanRow(ro)
		require.NoError(t, err)
		require.Equal(t, OrderBalance{Order: "test", Sum: sum, ProcessedAt: &curTime}, *ob)
		ro.AssertExpectations(t)
	})
}

func TestOrder_UnMarshalMarshalJSON(t *testing.T) {
	t.Run("test unmarshal marshall with number", func(t *testing.T) {
		curTime := time.Now()
		js := fmt.Sprintf(`
		{
			"number": "test",
			"status": "test",
			"accrual": 1,
			"uploaded_at": "%s"
		}
		`, curTime.Format(time.RFC3339))

		var o Order
		err := json.Unmarshal([]byte(js), &o)
		require.NoError(t, err)
		r, err := o.MarshalJSON()

		require.NoError(t, err)
		require.JSONEq(t, js, string(r))

	})

	t.Run("test unmarshal marshall with order", func(t *testing.T) {
		js := `
		{
			"order": "test",
			"status": "test",
			"accrual": 1
		}
		`

		var o Order
		err := json.Unmarshal([]byte(js), &o)
		require.NoError(t, err)
		r, err := o.MarshalJSON()

		require.NoError(t, err)
		jsEx := `
		{
			"number": "test",
			"status": "test",
			"accrual": 1,
			"uploaded_at":""
		}
		`
		require.JSONEq(t, jsEx, string(r))

	})
}

func TestNewOrderBalanceByRequestBody(t *testing.T) {
	t.Run("positive test", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{
			"order": "2377225624",
			"sum": 751
		}`))

		ob, err := NewOrderBalanceByRequestBody(body)

		require.NoError(t, err)
		require.Equal(t, OrderBalance{Order: "2377225624", Sum: 751}, *ob)
	})

	t.Run("body json error", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(""))

		_, err := NewOrderBalanceByRequestBody(body)

		require.Error(t, err)
	})
}

func TestOrderBalance_MarshalJSON(t *testing.T) {
	t.Run("test marshall", func(t *testing.T) {
		curTime := time.Now()
		js := fmt.Sprintf(`
		{
			"order": "test",
			"sum": 1,
			"processed_at": "%s"
		}
		`, curTime.Format(time.RFC3339))

		var o OrderBalance
		err := json.Unmarshal([]byte(js), &o)
		require.NoError(t, err)
		r, err := o.MarshalJSON()

		require.NoError(t, err)
		require.JSONEq(t, js, string(r))

	})
}
