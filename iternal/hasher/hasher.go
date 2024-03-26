package hasher

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type SaltPassword struct {
	Password string
	Salt     string
}

func NewSaltPassword(password string) (*SaltPassword, error) {
	b := make([]byte, 75)

	_, err := rand.Read(b)

	if err != nil {
		return nil, fmt.Errorf("generate random value: %w", err)
	}

	var sp SaltPassword

	sp.Salt = hex.EncodeToString(b)
	hash := sha256.New()
	data := append(b, password...)
	hash.Write(data)
	dst := hash.Sum(nil)

	sp.Password = hex.EncodeToString(dst)
	return &sp, nil
}

func GetPasswordHash(password, salt string) (string, error) {
	decodeSalt, err := hex.DecodeString(salt)

	if err != nil {
		return "", fmt.Errorf("decode salt: %w", err)
	}

	hash := sha256.New()
	data := append(decodeSalt, password...)
	hash.Write(data)
	dst := hash.Sum(nil)

	return hex.EncodeToString(dst), nil
}
