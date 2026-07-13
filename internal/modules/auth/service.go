package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"corebe.local/api/internal/modules/identity"
	"corebe.local/api/internal/platform/database"
	"corebe.local/api/internal/platform/id"
)

const minPasswordLength = 8

const usersEmailNormalizedUniqueConstraint = "users_email_normalized_unique"

type Service struct {
	db          *pgxpool.Pool
	transactor  TransactionManager
	users       UserRepository
	passwords   PasswordHasher
	tokenIssuer TokenIssuer
}

type ServiceConfig struct {
	DB          *pgxpool.Pool
	Transactor  TransactionManager
	Users       UserRepository
	Passwords   PasswordHasher
	TokenIssuer TokenIssuer
}

func NewService(cfg ServiceConfig) Service {
	return Service{
		db:          cfg.DB,
		transactor:  cfg.Transactor,
		users:       cfg.Users,
		passwords:   cfg.Passwords,
		tokenIssuer: cfg.TokenIssuer,
	}
}

func (s Service) Register(ctx context.Context, request RegisterRequest) (RegisterResult, error) {
	emailNormalized := NormalizeEmail(request.Email)
	displayName := strings.TrimSpace(request.DisplayName)
	if !validRegisterInput(emailNormalized, request.Password, displayName) {
		return RegisterResult{}, ErrInvalidRegistrationInput
	}

	passwordHash, err := s.passwords.HashPassword(ctx, request.Password)
	if err != nil {
		return RegisterResult{}, err
	}

	userID, err := id.NewUUID()
	if err != nil {
		return RegisterResult{}, err
	}

	var user identity.User
	err = s.transactor.WithinTx(ctx, func(ctx context.Context, tx database.DBTX) error {
		created, err := s.users.CreateUser(ctx, tx, identity.CreateUserParams{
			ID:              userID,
			Email:           strings.TrimSpace(request.Email),
			EmailNormalized: emailNormalized,
			PasswordHash:    passwordHash,
			DisplayName:     displayName,
		})
		if err != nil {
			return mapRegisterError(err)
		}

		user = created
		return nil
	})
	if err != nil {
		return RegisterResult{}, err
	}

	return RegisterResult{
		UserID: user.ID,
		Email:  user.Email,
	}, nil
}

func (s Service) Login(ctx context.Context, request LoginRequest) (LoginResult, error) {
	emailNormalized := NormalizeEmail(request.Email)
	if !validLoginInput(emailNormalized, request.Password) {
		return LoginResult{}, ErrInvalidCredentials
	}

	user, err := s.users.GetUserByEmailNormalized(ctx, s.db, emailNormalized)
	if err != nil {
		return LoginResult{}, mapLoginUserReadError(err)
	}

	if err := s.passwords.ComparePassword(ctx, request.Password, user.PasswordHash); err != nil {
		return LoginResult{}, err
	}

	if user.Status != identity.UserStatusActive {
		return LoginResult{}, ErrUserDisabled
	}

	return LoginResult{
		UserID: user.ID,
		Email:  user.Email,
	}, nil
}

func validRegisterInput(emailNormalized string, password string, displayName string) bool {
	return emailNormalized != "" &&
		strings.Contains(emailNormalized, "@") &&
		len(password) >= minPasswordLength &&
		displayName != ""
}

func validLoginInput(emailNormalized string, password string) bool {
	return emailNormalized != "" &&
		strings.Contains(emailNormalized, "@") &&
		len(password) >= minPasswordLength
}

func mapRegisterError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == usersEmailNormalizedUniqueConstraint {
		return ErrEmailAlreadyExists
	}
	return err
}

func mapLoginUserReadError(err error) error {
	if errors.Is(err, identity.ErrUserNotFound) {
		return ErrInvalidCredentials
	}
	return err
}
