package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBcryptPasswordHasher(t *testing.T) {
	t.Parallel()

	hasher, err := NewBcryptPasswordHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatalf("new bcrypt password hasher: %v", err)
	}

	hash, err := hasher.HashPassword(context.Background(), "correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "" || hash == "correct-password" {
		t.Fatalf("password hash was not generated safely")
	}

	if err := hasher.ComparePassword(context.Background(), "correct-password", hash); err != nil {
		t.Fatalf("compare correct password: %v", err)
	}

	if err := hasher.ComparePassword(context.Background(), "wrong-password", hash); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestNewBcryptPasswordHasherRejectsInvalidCost(t *testing.T) {
	t.Parallel()

	_, err := NewBcryptPasswordHasher(bcrypt.MinCost - 1)
	if !errors.Is(err, ErrInvalidPasswordCost) {
		t.Fatalf("expected ErrInvalidPasswordCost, got %v", err)
	}
}
