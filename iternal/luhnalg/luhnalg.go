package luhnalg

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

var ErrInvalidNumber error = fmt.Errorf("invalid order number")

func CheckNumber(number []byte) bool {
	var (
		sum    int
		parity = len(number) % 2
	)

	for idx, val := range number {
		a, err := strconv.Atoi(string(val))

		if err != nil {
			return false
		}

		if idx%2 == parity {
			a *= 2

			if a > 9 {
				a -= 9
			}
		}

		sum += a
	}

	return sum%10 == 0
}

func GetNumberFromBody(body io.ReadCloser) (string, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(body)

	if err != nil {
		return "", fmt.Errorf("invalid body: %w", err)
	}

	if buf.String() == "" {
		return "", fmt.Errorf("empty body: %w", err)
	}

	validID := CheckNumber(buf.Bytes())

	if !validID {
		return "", ErrInvalidNumber
	}

	return buf.String(), nil
}
