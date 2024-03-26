package tokenworker

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type TokenWorker struct {
	secret string
	exp    time.Duration
}

func NewToken(secret string, exp time.Duration) *TokenWorker {
	return &TokenWorker{secret: secret, exp: exp}
}

func (t *TokenWorker) GetToken(sub string) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(t.exp)),
		},
	)
	tokenString, err := token.SignedString([]byte(t.secret))

	if err != nil {
		return "", fmt.Errorf("signed token: %w", err)
	}

	return tokenString, nil
}

func (t *TokenWorker) GetSubFromToken(token string) (string, bool) {
	claims := &jwt.RegisteredClaims{}
	jwtToken, err := jwt.ParseWithClaims(token, claims, func(jwtT *jwt.Token) (interface{}, error) {
		if _, ok := jwtT.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", jwtT.Header["alg"])
		}

		return []byte(t.secret), nil
	})

	if err != nil {
		return "", false
	}

	if !jwtToken.Valid {
		return "", false
	}

	return claims.Subject, true
}

func (t *TokenWorker) WriteTokenInCookie(w http.ResponseWriter, login string) error {
	tokenString, err := t.GetToken(login)

	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}

	cookie := &http.Cookie{
		Name:  "token",
		Value: tokenString,
	}

	http.SetCookie(w, cookie)

	return nil
}

func (t *TokenWorker) RequestToken(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		tokenCookie, err := r.Cookie("token")

		if err != nil || tokenCookie == nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		sub, tokenValid := t.GetSubFromToken(tokenCookie.Value)

		if !tokenValid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		r.Header.Add("login", sub)
		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(logFn)
}
