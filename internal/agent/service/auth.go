package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/repository"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/hasher"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidLoginFormat    = errors.New("неверный формат логина (3-30 символов, буквы, цифры, '_')")
	ErrInvalidPasswordFormat = errors.New("неверный формат пароля (минимум 6 символов)")
	ErrInvalidCredentials    = errors.New("неверный логин или пароль")
	ErrRegistrationFailed    = errors.New("ошибка регистрации")
	ErrLoginFailed           = errors.New("ошибка входа")
	// ErrLoginAlreadyExists используется из пакета repository
)

var loginRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)

type AuthService interface {
	Register(ctx context.Context, login, password string) (userID string, err error)
	Login(ctx context.Context, login, password string) (userID string, token string, err error)
}

type authService struct {
	userRepo repository.UserRepository
	hasher   hasher.PasswordHasher    
	log      *zap.Logger              
}

func NewAuthService(userRepo repository.UserRepository, hasher hasher.PasswordHasher, log *zap.Logger) AuthService {
	return &authService{
		userRepo: userRepo,
		hasher:   hasher,
		log:      log,
	}
}

func (s *authService) Register(ctx context.Context, login, password string) (string, error) {
	if !loginRegex.MatchString(login) {
		return "", ErrInvalidLoginFormat
	}
	if len(password) < 6 {
		return "", ErrInvalidPasswordFormat
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		s.log.Error("Не удалось захешировать пароль при регистрации", zap.Error(err), zap.String("login", login))
		return "", fmt.Errorf("%w: %v", ErrRegistrationFailed, err)
	}

	userID, err := s.userRepo.CreateUser(ctx, login, passwordHash)
	if err != nil {
		if errors.Is(err, repository.ErrLoginAlreadyExists) {
			return "", repository.ErrLoginAlreadyExists
		}
		s.log.Error("Не удалось создать пользователя через репозиторий", zap.Error(err), zap.String("login", login))
		return "", fmt.Errorf("%w: ошибка репозитория", ErrRegistrationFailed)
	}

	s.log.Info("Пользователь успешно зарегистрирован", zap.String("login", login), zap.String("userID", userID.String()))
	return userID.String(), nil
}

func (s *authService) Login(ctx context.Context, login, password string) (string, string, error) {
	user, err := s.userRepo.GetUserByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			s.log.Warn("Попытка входа для несуществующего пользователя", zap.String("login", login))
			return "", "", ErrInvalidCredentials
		}
		s.log.Error("Не удалось получить пользователя по логину при входе", zap.Error(err), zap.String("login", login))
		return "", "", fmt.Errorf("%w: ошибка репозитория", ErrLoginFailed)
	}

	err = s.hasher.Compare(user.PasswordHash, password)
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			s.log.Warn("Неудачная попытка входа (неверный пароль)", zap.String("login", login))
			return "", "", ErrInvalidCredentials
		}
		s.log.Error("Ошибка при сравнении хеша пароля", zap.Error(err), zap.String("login", login))
		return "", "", fmt.Errorf("%w: ошибка сравнения хеша", ErrLoginFailed)
	}

	// Генерация JWT токена (ЗАГЛУШКА)
	// Здесь будет вызов jwtManager.Generate(user.ID.String())
	token := "jwt_token_placeholder_" + user.ID.String() // Временная заглушка

	s.log.Info("Пользователь успешно вошел в систему", zap.String("login", login), zap.String("userID", user.ID.String()))
	return user.ID.String(), token, nil
}