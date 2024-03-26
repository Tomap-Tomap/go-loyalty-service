package models

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
)

type RowsMockedObject struct {
	pgx.Rows
	mock.Mock
}

func (rm *RowsMockedObject) Values() ([]any, error) {
	args := rm.Called()

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]any), args.Error(1)
}

func (rm *RowsMockedObject) FieldDescriptions() []pgconn.FieldDescription {
	args := rm.Called()

	return args.Get(0).([]pgconn.FieldDescription)
}
