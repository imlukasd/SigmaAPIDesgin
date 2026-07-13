package identity

import (
	"context"
	"errors"
	"testing"

	"corebe.local/api/internal/platform/testdb"
)

func TestRepositoryCreateAndReadUserIntegration(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	repo := NewRepository()

	suffix := testdb.UniqueSuffix(t)
	userID := testdb.NewUUID(t)
	email := "user-" + suffix + "@example.com"
	params := CreateUserParams{
		ID:              userID,
		Email:           email,
		EmailNormalized: email,
		PasswordHash:    "hashed-password-for-test",
		DisplayName:     "Test User",
	}

	created, err := repo.CreateUser(ctx, db, params)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM users WHERE id = $1`, userID)
	})

	if created.ID != userID {
		t.Fatalf("expected user id %s, got %s", userID, created.ID)
	}
	if created.Status != UserStatusActive {
		t.Fatalf("expected active status, got %s", created.Status)
	}

	byID, err := repo.GetUserByID(ctx, db, userID)
	if err != nil {
		t.Fatalf("get user by id: %v", err)
	}
	if byID.Email != email {
		t.Fatalf("expected email %s, got %s", email, byID.Email)
	}

	byEmail, err := repo.GetUserByEmailNormalized(ctx, db, email)
	if err != nil {
		t.Fatalf("get user by email normalized: %v", err)
	}
	if byEmail.ID != userID {
		t.Fatalf("expected user id %s, got %s", userID, byEmail.ID)
	}

	_, err = repo.GetUserByID(ctx, db, testdb.NewUUID(t))
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
