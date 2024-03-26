package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Tomap-Tomap/go-loyalty-service/iternal/hasher"
	"github.com/jackc/pgx/v5"
)

var ErrPWDNotEqual error = fmt.Errorf("passwords not equal")

type User struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Salt     string `json:"-"`
}

func NewUserByRequestBody(body io.ReadCloser) (*User, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(body)

	if err != nil {
		return nil, fmt.Errorf("read from body: %w", err)
	}

	var u User
	err = u.writeFieldsByJSON(buf.Bytes())

	if err != nil {
		return nil, fmt.Errorf("write fields for user: %w", err)
	}

	if u.Login == "" {
		return nil, fmt.Errorf("empty login")
	}

	if u.Password == "" {
		return nil, fmt.Errorf("empty password")
	}

	return &u, nil
}

func (u *User) ScanRow(rows pgx.Rows) error {
	values, err := rows.Values()
	if err != nil {
		return err
	}

	for i := range values {
		switch strings.ToLower(rows.FieldDescriptions()[i].Name) {
		case "login":
			u.Login = values[i].(string)
		case "password":
			u.Password = values[i].(string)
		case "salt":
			u.Salt = values[i].(string)
		}
	}

	return nil
}

func (u *User) writeFieldsByJSON(j []byte) error {
	err := json.Unmarshal(j, u)

	if err != nil {
		return fmt.Errorf("unmarshall json %s: %w", string(j), err)
	}

	return nil
}

func (u *User) CheckPassword(password string) error {
	hashedPwd, err := hasher.GetPasswordHash(password, u.Salt)

	if err != nil {
		return err
	}

	if hashedPwd != u.Password {
		return ErrPWDNotEqual
	}

	return nil
}

type UserBalance struct {
	Current   float64  `json:"current"`
	Withdrawn *float64 `json:"withdrawn,omitempty"`
}

func (ub *UserBalance) ScanRow(rows pgx.Rows) error {
	values, err := rows.Values()
	if err != nil {
		return err
	}

	for i := range values {
		switch strings.ToLower(rows.FieldDescriptions()[i].Name) {
		case "current":
			ub.Current = values[i].(float64)
		case "withdrawn":
			wd := values[i]
			if wd != nil {
				wd := wd.(float64)
				ub.Withdrawn = &wd
			}
		}
	}

	return nil
}
