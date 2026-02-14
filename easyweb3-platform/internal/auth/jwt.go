package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	ProjectID string `json:"project_id"`
	Role      string `json:"role"`
	// Permissions is reserved for future RBAC expansion.
	Permissions []string `json:"permissions,omitempty"`

	jwt.RegisteredClaims
}

type JWT struct {
	Secret   []byte
	TokenTTL time.Duration
}

func (j JWT) Sign(claims Claims) (token string, expiresAt time.Time, err error) {
	now := time.Now().UTC()
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwt.NewNumericDate(now)
	}
	if claims.NotBefore == nil {
		claims.NotBefore = jwt.NewNumericDate(now.Add(-5 * time.Second))
	}
	if claims.ExpiresAt == nil {
		expiresAt = now.Add(j.TokenTTL)
		claims.ExpiresAt = jwt.NewNumericDate(expiresAt)
	} else {
		expiresAt = claims.ExpiresAt.Time
	}
	if claims.Issuer == "" {
		claims.Issuer = "easyweb3-platform"
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString(j.Secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return s, expiresAt, nil
}

func (j JWT) Verify(token string) (Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return j.Secret, nil
	})
	if err != nil {
		return Claims{}, err
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}
	return *c, nil
}
