package hasher

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hashedPassword, password string) error
}

type BcryptHasher struct {
	cost int
}

func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

func (h *BcryptHasher) Hash(password string) (string, error) {

	if len(password) == 0 {
		return "", fmt.Errorf("пароль не может быть пустым")
	}

	if len(password) > 72 {
		return "", fmt.Errorf("пароль слишком длинный (максимум 72 байта)")
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("ошибка хеширования пароля bcrypt: %w", err)
	}
	return string(hashedBytes), nil
}

func (h *BcryptHasher) Compare(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return fmt.Errorf("ошибка сравнения пароля bcrypt: %w", err)
	}
	return nil
}
