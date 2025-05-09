package jwtauth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

type Manager struct {
	secretKey []byte
	tokenTTL  time.Duration
}

func NewManager(secretKey string, tokenTTL time.Duration) (*Manager, error) {
	if len(secretKey) < 32 {
		return nil, fmt.Errorf("секретный ключ JWT должен быть длиной не менее 32 символов")
	}
	if tokenTTL <= 0 {
		return nil, fmt.Errorf("время жизни токена (TTL) должно быть положительным")
	}
	return &Manager{
		secretKey: []byte(secretKey),
		tokenTTL:  tokenTTL,
	}, nil
}

func (m *Manager) Generate(userID string) (string, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return "", fmt.Errorf("невалидный формат UserID для генерации токена: %w", err)
	}

	expirationTime := time.Now().Add(m.tokenTTL)
	issuedAtTime := time.Now()

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(issuedAtTime),
			NotBefore: jwt.NewNumericDate(issuedAtTime),
			Issuer:    "calculator-app",
			Subject:   userID,
		},
		UserID: userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", fmt.Errorf("ошибка подписи JWT токена: %w", err)
	}

	return tokenString, nil
}

func (m *Manager) Verify(tokenString string) (userID string, err error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", fmt.Errorf("токен истек: %w", err)
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return "", fmt.Errorf("токен еще не действителен: %w", err)
		}
		return "", fmt.Errorf("невалидный токен: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("невалидный токен или claims")
	}

	if _, parseErr := uuid.Parse(claims.UserID); parseErr != nil {
		return "", fmt.Errorf("невалидный UserID в claims токена: %w", parseErr)
	}

	return claims.UserID, nil
}
