package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Order struct {
	Number     string     `json:"number"`
	Status     string     `json:"status"`
	Accrual    *float64   `json:"accrual,omitempty"`
	UploadedAt *time.Time `json:"uploaded_at"`
}

func (o *Order) ScanRow(rows pgx.Rows) error {
	values, err := rows.Values()
	if err != nil {
		return err
	}

	for i := range values {
		switch strings.ToLower(rows.FieldDescriptions()[i].Name) {
		case "number":
			o.Number = values[i].(string)
		case "status":
			o.Status = values[i].(string)
		case "accrual":
			acc := values[i]

			if acc != nil {
				acc := acc.(float64)
				o.Accrual = &acc
			}
		case "uploadedat":
			ua := values[i].(time.Time)
			o.UploadedAt = &ua
		}
	}

	return nil
}

func (o Order) MarshalJSON() ([]byte, error) {
	type OrderAlias Order

	aliasOrder := struct {
		OrderAlias
		UploadedAt string `json:"uploaded_at"`
	}{
		OrderAlias: OrderAlias(o),
	}

	if o.UploadedAt != nil {
		aliasOrder.UploadedAt = o.UploadedAt.Format(time.RFC3339)
	}

	return json.Marshal(aliasOrder)
}

func (o *Order) UnmarshalJSON(data []byte) (err error) {
	type OrderFromService struct {
		Order   string   `json:"order"`
		Status  string   `json:"status"`
		Accrual *float64 `json:"accrual,omitempty"`
	}

	ofs := &OrderFromService{}

	if err = json.Unmarshal(data, ofs); err != nil || ofs.Order == "" {
		type OrderAlias Order
		var oa = (*OrderAlias)(o)
		err = json.Unmarshal(data, oa)

		return
	}

	o.Number = ofs.Order
	o.Status = ofs.Status

	if ofs.Status == "REGISTERED" {
		o.Status = StatusNew
	}

	o.Accrual = ofs.Accrual

	return
}

type OrderBalance struct {
	Order       string     `json:"order"`
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
			ob.Order = values[i].(string)
		case "sum":
			ob.Sum = values[i].(float64)
		case "processedat":
			pa := values[i].(time.Time)
			ob.ProcessedAt = &pa
		}
	}

	return nil
}

func (ob OrderBalance) MarshalJSON() ([]byte, error) {
	type OrderBalanceAlias OrderBalance

	aliasOrderBalance := struct {
		OrderBalanceAlias
		ProcessedAt string `json:"processed_at"`
	}{
		OrderBalanceAlias: OrderBalanceAlias(ob),
	}

	if ob.ProcessedAt != nil {
		aliasOrderBalance.ProcessedAt = ob.ProcessedAt.Format(time.RFC3339)
	}

	return json.Marshal(aliasOrderBalance)
}

func (ob *OrderBalance) writeFieldsByJSON(j []byte) error {
	err := json.Unmarshal(j, ob)

	if err != nil {
		return fmt.Errorf("unmarshall json %s: %w", string(j), err)
	}

	return nil
}
