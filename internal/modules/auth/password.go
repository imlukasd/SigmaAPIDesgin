package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const DefaultBcryptCost = 12

var ErrInvalidPasswordCost = errors.New("auth invalid password cost")

type BcryptPasswordHasher struct {
	cost int
}

func NewBcryptPasswordHasher(cost int) (BcryptPasswordHasher, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return BcryptPasswordHasher{}, ErrInvalidPasswordCost
	}
	return BcryptPasswordHasher{cost: cost}, nil
}

func (h BcryptPasswordHasher) HashPassword(ctx context.Context, password string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func (h BcryptPasswordHasher) ComparePassword(ctx context.Context, password string, passwordHash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}

	return nil
}
