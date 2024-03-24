package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Order struct {
	Number     int64     `json:"number"`
	Status     string    `json:"status"`
	Accrual    *float64  `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

func (o *Order) ScanRow(rows pgx.Rows) error {
	values, err := rows.Values()
	if err != nil {
		return err
	}

	for i := range values {
		switch strings.ToLower(rows.FieldDescriptions()[i].Name) {
		case "number":
			o.Number = values[i].(int64)
		case "status":
			o.Status = values[i].(string)
		case "accrual":
			acc := values[i].(float64)
			o.Accrual = &acc
		case "uploadedat":
			o.UploadedAt = values[i].(time.Time)
		}
	}

	return nil
}

type OrderBalance struct {
	Order       int64      `json:"order"`
	Sum         float64    `json:"sum"`
	ProcessedAt *time.Time `json:"processed_at"`
}

func NewOrderBalanceByRequestBody(body io.ReadCloser) (*OrderBalance, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(body)

	if err != nil {
		return nil, fmt.Errorf("read from body: %w", err)
	}

	var ob OrderBalance
	err = ob.writeFieldsByJSON(buf.Bytes())

	if err != nil {
		return nil, fmt.Errorf("write fields for order balance: %w", err)
	}

	return &ob, nil
}

func (ob *OrderBalance) ScanRow(rows pgx.Rows) error {
	values, err := rows.Values()
	if err != nil {
		return err
	}

	for i := range values {
		switch strings.ToLower(rows.FieldDescriptions()[i].Name) {
		case "order":
			ob.Order = values[i].(int64)
		case "sum":
			ob.Sum = values[i].(float64)
		case "processedat":
			pa := values[i].(time.Time)
			ob.ProcessedAt = &pa
		}
	}

	return nil
}

func (ob *OrderBalance) UnmarshalJSON(data []byte) (err error) {
	type OrderBalanceAlias OrderBalance

	aliasOrderBalance := &struct {
		*OrderBalanceAlias
		Order string `json:"order"`
	}{
		OrderBalanceAlias: (*OrderBalanceAlias)(ob),
	}

	if err = json.Unmarshal(data, aliasOrderBalance); err != nil {
		return
	}
	ob.Order, err = strconv.ParseInt(aliasOrderBalance.Order, 10, 64)
	return
}

func (ob *OrderBalance) writeFieldsByJSON(j []byte) error {
	err := json.Unmarshal(j, ob)

	if err != nil {
		return fmt.Errorf("unmarshall json %s: %w", string(j), err)
	}

	return nil
}