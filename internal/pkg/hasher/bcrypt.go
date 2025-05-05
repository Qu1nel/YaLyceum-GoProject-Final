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
	cost int // Сложность хеширования bcrypt.
}

func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

// Hash генерирует хеш bcrypt для заданного пароля.
func (h *BcryptHasher) Hash(password string) (string, error) {
	// Проверка длины пароля перед хешированием (bcrypt имеет ограничение в 72 байта)
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

// Compare сравнивает хеш bcrypt с паролем.
// Возвращает nil при совпадении, иначе ошибку (включая bcrypt.ErrMismatchedHashAndPassword).
func (h *BcryptHasher) Compare(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return fmt.Errorf("ошибка сравнения пароля bcrypt: %w", err)
	}
	return nil
}