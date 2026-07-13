package auth

import "errors"

var (
	ErrInvalidCredentials       = errors.New("auth invalid credentials")
	ErrEmailAlreadyExists       = errors.New("auth email already exists")
	ErrUserDisabled             = errors.New("auth user disabled")
	ErrInvalidRegistrationInput = errors.New("auth invalid registration input")
	ErrInvalidTokenConfig       = errors.New("auth invalid token config")
	ErrInvalidTokenSubject      = errors.New("auth invalid token subject")
	ErrTokenGenerationFailed    = errors.New("auth token generation failed")
)
