package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/Tomap-Tomap/go-loyalty-service/iternal/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type ClientMockedObject struct {
	mock.Mock
}

func (cm *ClientMockedObject) GetOrder(ctx context.Context, number string) (*models.Order, error) {
	args := cm.Called(ctx, number)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

type RepositoryMockedObject struct {
	mock.Mock
}

func (rm *RepositoryMockedObject) GetNotProcessedOrders(ctx context.Context) ([]string, error) {
	args := rm.Called(ctx)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (rm *RepositoryMockedObject) UpdateOrder(ctx context.Context, o models.Order) error {
	args := rm.Called(ctx, o)

	return args.Error(0)
}

func TestAgent_updateOrder(t *testing.T) {
	accrual := float64(1)
	retOrderErr := &models.Order{
		Number:  "DBError",
		Status:  "NEW",
		Accrual: &accrual,
	}
	retOrder := &models.Order{
		Number:  "noError",
		Status:  "NEW",
		Accrual: &accrual,
	}
	cm := new(ClientMockedObject)
	cm.On("GetOrder", context.Background(), "error").Return(nil, fmt.Errorf("test error"))
	cm.On("GetOrder", context.Background(), "DBError").Return(retOrderErr, nil)
	cm.On("GetOrder", context.Background(), "NoError").Return(retOrder, nil)

	rm := new(RepositoryMockedObject)
	rm.On("UpdateOrder", context.Background(), *retOrderErr).Return(fmt.Errorf("test error"))
	rm.On("UpdateOrder", context.Background(), *retOrder).Return(nil)

	a := NewAgent(rm, cm, 0, 0)

	t.Run("get order error", func(t *testing.T) {
		err := a.updateOrder(context.Background(), "error")
		require.Error(t, err)
	})

	t.Run("get db error", func(t *testing.T) {
		err := a.updateOrder(context.Background(), "DBError")
		require.Error(t, err)
	})

	t.Run("get no error", func(t *testing.T) {
		err := a.updateOrder(context.Background(), "NoError")
		require.NoError(t, err)
	})

	cm.AssertExpectations(t)
}

func TestAgent_processingOrders(t *testing.T) {
	cm := new(ClientMockedObject)
	rm := new(RepositoryMockedObject)
	rm.On("GetNotProcessedOrders", context.Background()).Return(nil, pgx.ErrNoRows)

	a := NewAgent(rm, cm, 0, 0)

	t.Run("get order error", func(t *testing.T) {
		err := a.processingOrders(context.Background(), nil)
		require.NoError(t, err)
	})

	cm.AssertExpectations(t)

	rm = new(RepositoryMockedObject)
	rm.On("GetNotProcessedOrders", context.Background()).Return(nil, fmt.Errorf("test error"))
	a = NewAgent(rm, cm, 0, 0)
	t.Run("get order error", func(t *testing.T) {
		err := a.processingOrders(context.Background(), nil)
		require.Error(t, err)
	})

	cm.AssertExpectations(t)
}
