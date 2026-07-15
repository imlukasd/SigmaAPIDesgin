package auth

import (
	"context"
	"errors"
	"strings"
	"time"

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
	sessions    SessionRepository
	passwords   PasswordHasher
	tokenIssuer TokenIssuer
	clock       func() time.Time
}

type ServiceConfig struct {
	DB          *pgxpool.Pool
	Transactor  TransactionManager
	Users       UserRepository
	Sessions    SessionRepository
	Passwords   PasswordHasher
	TokenIssuer TokenIssuer
	Clock       func() time.Time
}

func NewService(cfg ServiceConfig) Service {
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}

	return Service{
		db:          cfg.DB,
		transactor:  cfg.Transactor,
		users:       cfg.Users,
		sessions:    cfg.Sessions,
		passwords:   cfg.Passwords,
		tokenIssuer: cfg.TokenIssuer,
		clock:       clock,
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

	sessionID, err := id.NewUUID()
	if err != nil {
		return LoginResult{}, err
	}

	issued, err := s.tokenIssuer.IssueTokenPair(ctx, IssueTokenPairParams{
		UserID:    user.ID,
		SessionID: sessionID,
	})
	if err != nil {
		return LoginResult{}, err
	}

	refreshTokenID, err := id.NewUUID()
	if err != nil {
		return LoginResult{}, err
	}

	err = s.transactor.WithinTx(ctx, func(ctx context.Context, tx database.DBTX) error {
		_, err := s.sessions.CreateAuthSession(ctx, tx, CreateAuthSessionParams{
			ID:        sessionID,
			UserID:    user.ID,
			UserAgent: optionalTrimmedString(request.UserAgent),
			IPAddress: optionalTrimmedString(request.IPAddress),
			ExpiresAt: issued.Tokens.RefreshTokenExpiresAt,
		})
		if err != nil {
			return err
		}

		_, err = s.sessions.CreateRefreshToken(ctx, tx, CreateRefreshTokenParams{
			ID:        refreshTokenID,
			SessionID: sessionID,
			TokenHash: issued.RefreshTokenHash,
			ExpiresAt: issued.Tokens.RefreshTokenExpiresAt,
		})
		return err
	})
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		UserID: user.ID,
		Email:  user.Email,
		Tokens: issued.Tokens,
	}, nil
}

func (s Service) RefreshToken(ctx context.Context, request RefreshTokenRequest) (RefreshTokenResult, error) {
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if refreshToken == "" {
		return RefreshTokenResult{}, ErrInvalidRefreshToken
	}

	now := s.clock().UTC()
	tokenHash := HashRefreshToken(refreshToken)

	var result RefreshTokenResult
	err := s.transactor.WithinTx(ctx, func(ctx context.Context, tx database.DBTX) error {
		existingToken, err := s.sessions.GetRefreshTokenByHashForUpdate(ctx, tx, tokenHash)
		if err != nil {
			return mapRefreshTokenLookupError(err)
		}
		if !validRefreshTokenState(existingToken, now) {
			return ErrInvalidRefreshToken
		}

		session, err := s.sessions.GetAuthSessionByID(ctx, tx, existingToken.SessionID)
		if err != nil {
			return mapAuthSessionLookupError(err)
		}
		if !validAuthSessionState(session, now) {
			return ErrInvalidRefreshToken
		}

		user, err := s.users.GetUserByID(ctx, tx, session.UserID)
		if err != nil {
			return mapRefreshUserReadError(err)
		}
		if user.Status != identity.UserStatusActive {
			return ErrUserDisabled
		}

		issued, err := s.tokenIssuer.IssueTokenPair(ctx, IssueTokenPairParams{
			UserID:           user.ID,
			SessionID:        session.ID,
			SessionExpiresAt: session.ExpiresAt,
		})
		if err != nil {
			return err
		}

		refreshTokenID, err := id.NewUUID()
		if err != nil {
			return err
		}

		if _, err := s.sessions.MarkRefreshTokenUsed(ctx, tx, existingToken.ID, now); err != nil {
			return err
		}

		if _, err := s.sessions.CreateRefreshToken(ctx, tx, CreateRefreshTokenParams{
			ID:                 refreshTokenID,
			SessionID:          session.ID,
			TokenHash:          issued.RefreshTokenHash,
			RotatedFromTokenID: &existingToken.ID,
			ExpiresAt:          issued.Tokens.RefreshTokenExpiresAt,
		}); err != nil {
			return err
		}

		if _, err := s.sessions.MarkAuthSessionUsed(ctx, tx, session.ID, now); err != nil {
			return err
		}

		result = RefreshTokenResult{
			UserID: user.ID,
			Email:  user.Email,
			Tokens: issued.Tokens,
		}
		return nil
	})
	if err != nil {
		return RefreshTokenResult{}, err
	}

	return result, nil
}

func (s Service) Logout(ctx context.Context, request LogoutRequest) error {
	refreshToken := strings.TrimSpace(request.RefreshToken)
	if refreshToken == "" {
		return ErrInvalidRefreshToken
	}

	now := s.clock().UTC()
	tokenHash := HashRefreshToken(refreshToken)

	return s.transactor.WithinTx(ctx, func(ctx context.Context, tx database.DBTX) error {
		existingToken, err := s.sessions.GetRefreshTokenByHashForUpdate(ctx, tx, tokenHash)
		if err != nil {
			return mapLogoutRefreshTokenLookupError(err)
		}

		session, err := s.sessions.GetAuthSessionByID(ctx, tx, existingToken.SessionID)
		if err != nil {
			return mapAuthSessionLookupError(err)
		}

		if existingToken.RevokedAt == nil {
			if _, err := s.sessions.RevokeRefreshToken(ctx, tx, existingToken.ID, now); err != nil {
				return err
			}
		}

		if session.RevokedAt == nil {
			if _, err := s.sessions.RevokeAuthSession(ctx, tx, session.ID, now); err != nil {
				return err
			}
		}

		return nil
	})
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

func optionalTrimmedString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func validRefreshTokenState(token RefreshToken, now time.Time) bool {
	return token.RevokedAt == nil &&
		token.LastUsedAt == nil &&
		token.ExpiresAt.After(now)
}

func validAuthSessionState(session AuthSession, now time.Time) bool {
	return session.RevokedAt == nil &&
		session.ExpiresAt.After(now)
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

func mapRefreshTokenLookupError(err error) error {
	if errors.Is(err, ErrRefreshTokenNotFound) {
		return ErrInvalidRefreshToken
	}
	return err
}

func mapLogoutRefreshTokenLookupError(err error) error {
	if errors.Is(err, ErrRefreshTokenNotFound) {
		return nil
	}
	return err
}

func mapAuthSessionLookupError(err error) error {
	if errors.Is(err, ErrAuthSessionNotFound) {
		return ErrInvalidRefreshToken
	}
	return err
}

func mapRefreshUserReadError(err error) error {
	if errors.Is(err, identity.ErrUserNotFound) {
		return ErrInvalidRefreshToken
	}
	return err
}
