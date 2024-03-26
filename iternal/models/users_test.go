package models

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/Tomap-Tomap/go-loyalty-service/iternal/hasher"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestNewUserByRequestBody(t *testing.T) {
	t.Run("positive test", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{
			"login": "test",
			"password": "test"
		}`))

		u, err := NewUserByRequestBody(body)

		require.NoError(t, err)
		require.Equal(t, User{Login: "test", Password: "test"}, *u)
	})

	t.Run("empty login", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{
			"login": "",
			"password": "test"
		}`))

		_, err := NewUserByRequestBody(body)

		require.Error(t, err)
	})

	t.Run("empty pwd", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(`{
			"login": "test",
			"password": ""
		}`))

		_, err := NewUserByRequestBody(body)

		require.Error(t, err)
	})

	t.Run("body json error", func(t *testing.T) {
		body := io.NopCloser(bytes.NewBufferString(""))

		_, err := NewUserByRequestBody(body)

		require.Error(t, err)
	})
}

func TestUser_ScanRow(t *testing.T) {
	t.Run("test error", func(t *testing.T) {
		ro := new(RowsMockedObject)
		ro.On("Values").Return(nil, fmt.Errorf("test"))
		u := new(User)
		err := u.ScanRow(ro)
		require.Error(t, err)
		ro.AssertExpectations(t)
	})

	t.Run("positive", func(t *testing.T) {
		ro := new(RowsMockedObject)
		ro.On("Values").Return([]any{"test", "test", "test", 1}, nil)
		ro.On("FieldDescriptions").Return([]pgconn.FieldDescription{
			{Name: "login"},
			{Name: "password"},
			{Name: "salt"},
			{Name: "test"},
		}, nil)
		u := new(User)
		err := u.ScanRow(ro)
		require.NoError(t, err)
		require.Equal(t, User{Login: "test", Password: "test", Salt: "test"}, *u)
		ro.AssertExpectations(t)
	})
}

func TestUser_CheckPassword(t *testing.T) {
	t.Run("test error", func(t *testing.T) {
		u := User{"test", "test", "test"}
		err := u.CheckPassword("123")
		require.Error(t, err)
	})

	t.Run("test pwd not equal", func(t *testing.T) {
		sp, err := hasher.NewSaltPassword("test")
		require.NoError(t, err)
		u := User{"test", sp.Password, sp.Salt}
		err = u.CheckPassword("123")
		require.Error(t, err)
		require.Equal(t, ErrPWDNotEqual, err)
	})

	t.Run("positive test", func(t *testing.T) {
		sp, err := hasher.NewSaltPassword("test")
		require.NoError(t, err)
		u := User{"test", sp.Password, sp.Salt}
		err = u.CheckPassword("test")
		require.NoError(t, err)
	})
}

func TestUserBalance_ScanRow(t *testing.T) {
	t.Run("test error", func(t *testing.T) {
		ro := new(RowsMockedObject)
		ro.On("Values").Return(nil, fmt.Errorf("test"))
		ub := new(UserBalance)
		err := ub.ScanRow(ro)
		require.Error(t, err)
		ro.AssertExpectations(t)
	})

	t.Run("positive test no withdrawn", func(t *testing.T) {
		ro := new(RowsMockedObject)
		ro.On("Values").Return([]any{float64(1), "test"}, nil)
		ro.On("FieldDescriptions").Return([]pgconn.FieldDescription{
			{Name: "current"},
			{Name: "test"},
		}, nil)
		ub := new(UserBalance)
		err := ub.ScanRow(ro)
		require.NoError(t, err)
		require.Equal(t, UserBalance{Current: float64(1)}, *ub)
		ro.AssertExpectations(t)
	})

	t.Run("positive test", func(t *testing.T) {
		ro := new(RowsMockedObject)
		w := float64(2)
		ro.On("Values").Return([]any{float64(1), w, "test"}, nil)
		ro.On("FieldDescriptions").Return([]pgconn.FieldDescription{
			{Name: "current"},
			{Name: "withdrawn"},
			{Name: "test"},
		}, nil)
		ub := new(UserBalance)
		err := ub.ScanRow(ro)
		require.NoError(t, err)
		require.Equal(t, UserBalance{Current: float64(1), Withdrawn: &w}, *ub)
		ro.AssertExpectations(t)
	})
}
