package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"corebe.local/api/internal/modules/identity"
	"corebe.local/api/internal/platform/database"
)

type fakeTransactionManager struct {
	db database.DBTX
}

func (f fakeTransactionManager) WithinTx(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, f.db)
}

type fakePasswordHasher struct {
	hashErr    error
	compareErr error
}

func (f fakePasswordHasher) HashPassword(ctx context.Context, password string) (string, error) {
	if f.hashErr != nil {
		return "", f.hashErr
	}
	return "hashed:" + password, nil
}

func (f fakePasswordHasher) ComparePassword(ctx context.Context, password string, passwordHash string) error {
	if f.compareErr != nil {
		return f.compareErr
	}
	return nil
}

type fakeUserRepository struct {
	createParams       identity.CreateUserParams
	createErr          error
	getEmailNormalized string
	getUser            identity.User
	getErr             error
}

func (f *fakeUserRepository) CreateUser(ctx context.Context, db database.DBTX, params identity.CreateUserParams) (identity.User, error) {
	f.createParams = params
	if f.createErr != nil {
		return identity.User{}, f.createErr
	}

	return identity.User{
		ID:              params.ID,
		Email:           params.Email,
		EmailNormalized: params.EmailNormalized,
		PasswordHash:    params.PasswordHash,
		DisplayName:     params.DisplayName,
		Status:          identity.UserStatusActive,
	}, nil
}

func (f *fakeUserRepository) GetUserByEmailNormalized(ctx context.Context, db database.DBTX, emailNormalized string) (identity.User, error) {
	f.getEmailNormalized = emailNormalized
	if f.getErr != nil {
		return identity.User{}, f.getErr
	}
	if f.getUser.ID == "" {
		return identity.User{}, identity.ErrUserNotFound
	}
	return f.getUser, nil
}

func TestServiceRegister(t *testing.T) {
	t.Parallel()

	users := &fakeUserRepository{}
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users:      users,
		Passwords:  fakePasswordHasher{},
	})

	result, err := service.Register(context.Background(), RegisterRequest{
		Email:       " User@Example.COM ",
		Password:    "secret123",
		DisplayName: " Test User ",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if result.UserID == "" {
		t.Fatal("expected generated user id")
	}
	if result.Email != "User@Example.COM" {
		t.Fatalf("expected trimmed original email, got %s", result.Email)
	}
	if users.createParams.EmailNormalized != "user@example.com" {
		t.Fatalf("expected normalized email, got %s", users.createParams.EmailNormalized)
	}
	if users.createParams.PasswordHash != "hashed:secret123" {
		t.Fatalf("expected hashed password, got %s", users.createParams.PasswordHash)
	}
	if users.createParams.DisplayName != "Test User" {
		t.Fatalf("expected trimmed display name, got %s", users.createParams.DisplayName)
	}
}

func TestServiceRegisterRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users:      &fakeUserRepository{},
		Passwords:  fakePasswordHasher{},
	})

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:       "invalid-email",
		Password:    "short",
		DisplayName: "",
	})
	if !errors.Is(err, ErrInvalidRegistrationInput) {
		t.Fatalf("expected ErrInvalidRegistrationInput, got %v", err)
	}
}

func TestServiceRegisterMapsDuplicateEmail(t *testing.T) {
	t.Parallel()

	users := &fakeUserRepository{
		createErr: fmt.Errorf("create user: %w", &pgconn.PgError{
			ConstraintName: usersEmailNormalizedUniqueConstraint,
		}),
	}
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users:      users,
		Passwords:  fakePasswordHasher{},
	})

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:       "taken@example.com",
		Password:    "secret123",
		DisplayName: "Taken User",
	})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestServiceRegisterReturnsPasswordHashError(t *testing.T) {
	t.Parallel()

	hashErr := errors.New("hash failed")
	service := NewService(ServiceConfig{
		Transactor: fakeTransactionManager{},
		Users:      &fakeUserRepository{},
		Passwords:  fakePasswordHasher{hashErr: hashErr},
	})

	_, err := service.Register(context.Background(), RegisterRequest{
		Email:       "user@example.com",
		Password:    "secret123",
		DisplayName: "Test User",
	})
	if !errors.Is(err, hashErr) {
		t.Fatalf("expected hash error, got %v", err)
	}
}

func TestServiceLogin(t *testing.T) {
	t.Parallel()

	users := &fakeUserRepository{
		getUser: identity.User{
			ID:              "user_123",
			Email:           "User@Example.COM",
			EmailNormalized: "user@example.com",
			PasswordHash:    "hashed:secret123",
			Status:          identity.UserStatusActive,
		},
	}
	service := NewService(ServiceConfig{
		Users:     users,
		Passwords: fakePasswordHasher{},
	})

	result, err := service.Login(context.Background(), LoginRequest{
		Email:    " User@Example.COM ",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if users.getEmailNormalized != "user@example.com" {
		t.Fatalf("expected normalized email lookup, got %s", users.getEmailNormalized)
	}
	if result.UserID != "user_123" {
		t.Fatalf("expected user id, got %s", result.UserID)
	}
	if result.Email != "User@Example.COM" {
		t.Fatalf("expected original stored email, got %s", result.Email)
	}
}

func TestServiceLoginRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	users := &fakeUserRepository{}
	service := NewService(ServiceConfig{
		Users:     users,
		Passwords: fakePasswordHasher{},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "invalid-email",
		Password: "short",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if users.getEmailNormalized != "" {
		t.Fatalf("expected no repository lookup, got %s", users.getEmailNormalized)
	}
}

func TestServiceLoginMapsUnknownEmail(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Users:     &fakeUserRepository{},
		Passwords: fakePasswordHasher{},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "missing@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestServiceLoginMapsWrongPassword(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Users: &fakeUserRepository{
			getUser: identity.User{
				ID:           "user_123",
				Email:        "user@example.com",
				PasswordHash: "hashed:secret123",
				Status:       identity.UserStatusActive,
			},
		},
		Passwords: fakePasswordHasher{compareErr: ErrInvalidCredentials},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestServiceLoginRejectsDisabledUser(t *testing.T) {
	t.Parallel()

	service := NewService(ServiceConfig{
		Users: &fakeUserRepository{
			getUser: identity.User{
				ID:           "user_123",
				Email:        "user@example.com",
				PasswordHash: "hashed:secret123",
				Status:       identity.UserStatusDisabled,
			},
		},
		Passwords: fakePasswordHasher{},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
}

func TestServiceLoginReturnsRepositoryError(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("database offline")
	service := NewService(ServiceConfig{
		Users:     &fakeUserRepository{getErr: repoErr},
		Passwords: fakePasswordHasher{},
	})

	_, err := service.Login(context.Background(), LoginRequest{
		Email:    "user@example.com",
		Password: "secret123",
	})
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}
}
