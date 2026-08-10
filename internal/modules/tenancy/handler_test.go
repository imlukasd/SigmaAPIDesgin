package tenancy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"corebe.local/api/internal/modules/auth"
)

type fakeOrganizationHTTPService struct {
	createRequest CreateOrganizationRequest
	createResult  CreateOrganizationResult
	createErr     error
}

func (f *fakeOrganizationHTTPService) CreateOrganization(ctx context.Context, request CreateOrganizationRequest) (CreateOrganizationResult, error) {
	f.createRequest = request
	if f.createErr != nil {
		return CreateOrganizationResult{}, f.createErr
	}
	return f.createResult, nil
}

func TestHandlerCreateOrganization(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	service := &fakeOrganizationHTTPService{
		createResult: CreateOrganizationResult{
			Organization: Organization{
				ID:        "org_123",
				Name:      "Test Organization",
				Slug:      "test-organization",
				Status:    OrganizationStatusActive,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Owner: Membership{
				OrganizationID: "org_123",
				UserID:         "user_123",
				Role:           MembershipRoleOwner,
				Status:         MembershipStatusActive,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	handler := NewHandler(service)
	req := newOrganizationHTTPRequest(t, http.MethodPost, "/v1/organizations", `{
		"name": "Test Organization",
		"slug": "test-organization"
	}`)
	req = req.WithContext(auth.ContextWithAuthenticatedUser(req.Context(), auth.AuthenticatedUser{
		UserID: "user_123",
	}))
	rec := httptest.NewRecorder()

	handler.HandleCreateOrganization(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.createRequest.OwnerUserID != "user_123" {
		t.Fatalf("expected owner user id user_123, got %s", service.createRequest.OwnerUserID)
	}
	if service.createRequest.Name != "Test Organization" {
		t.Fatalf("expected organization name, got %s", service.createRequest.Name)
	}
	if service.createRequest.Slug != "test-organization" {
		t.Fatalf("expected organization slug, got %s", service.createRequest.Slug)
	}
	assertJSONField(t, rec.Body.String(), "data.organization.id", "org_123")
	assertJSONField(t, rec.Body.String(), "data.organization.slug", "test-organization")
	assertJSONField(t, rec.Body.String(), "data.owner_membership.role", string(MembershipRoleOwner))
}

func TestHandlerCreateOrganizationMapsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid input", err: ErrInvalidOrganizationInput, wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST"},
		{name: "duplicate slug", err: ErrOrganizationSlugExists, wantStatus: http.StatusConflict, wantCode: "ORGANIZATION_SLUG_EXISTS"},
		{name: "unknown", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeOrganizationHTTPService{createErr: tt.err}
			handler := NewHandler(service)
			req := newOrganizationHTTPRequest(t, http.MethodPost, "/v1/organizations", `{
				"name": "Test Organization",
				"slug": "test-organization"
			}`)
			req = req.WithContext(auth.ContextWithAuthenticatedUser(req.Context(), auth.AuthenticatedUser{
				UserID: "user_123",
			}))
			rec := httptest.NewRecorder()

			handler.HandleCreateOrganization(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			assertJSONField(t, rec.Body.String(), "error.code", tt.wantCode)
		})
	}
}

func TestHandlerCreateOrganizationRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	service := &fakeOrganizationHTTPService{}
	handler := NewHandler(service)
	req := newOrganizationHTTPRequest(t, http.MethodPost, "/v1/organizations", `{
		"name": "Test Organization",
		"slug": "test-organization",
		"unexpected": true
	}`)
	req = req.WithContext(auth.ContextWithAuthenticatedUser(req.Context(), auth.AuthenticatedUser{
		UserID: "user_123",
	}))
	rec := httptest.NewRecorder()

	handler.HandleCreateOrganization(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	assertJSONField(t, rec.Body.String(), "error.code", "INVALID_JSON")
	if service.createRequest.Name != "" {
		t.Fatalf("expected service not to be called, got %+v", service.createRequest)
	}
}

func TestHandlerCreateOrganizationRejectsMissingAuthContext(t *testing.T) {
	t.Parallel()

	service := &fakeOrganizationHTTPService{}
	handler := NewHandler(service)
	req := newOrganizationHTTPRequest(t, http.MethodPost, "/v1/organizations", `{
		"name": "Test Organization",
		"slug": "test-organization"
	}`)
	rec := httptest.NewRecorder()

	handler.HandleCreateOrganization(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.createRequest.Name != "" {
		t.Fatalf("expected service not to be called, got %+v", service.createRequest)
	}
}

func TestHandlerGetOrganization(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	handler := NewHandler(&fakeOrganizationHTTPService{})
	req := httptest.NewRequest(http.MethodGet, "/v1/organizations/org_123", nil)
	req = req.WithContext(ContextWithTenant(req.Context(), TenantContext{
		Organization: Organization{
			ID:        "org_123",
			Name:      "Test Organization",
			Slug:      "test-organization",
			Status:    OrganizationStatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Membership: Membership{
			OrganizationID: "org_123",
			UserID:         "user_123",
			Role:           MembershipRoleAdmin,
			Status:         MembershipStatusActive,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		Permission: PermissionOrganizationRead,
	}))
	rec := httptest.NewRecorder()

	handler.HandleGetOrganization(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	assertJSONField(t, rec.Body.String(), "data.organization.id", "org_123")
	assertJSONField(t, rec.Body.String(), "data.membership.role", string(MembershipRoleAdmin))
	assertJSONField(t, rec.Body.String(), "data.permission", string(PermissionOrganizationRead))
}

func TestHandlerGetOrganizationRejectsMissingTenantContext(t *testing.T) {
	t.Parallel()

	handler := NewHandler(&fakeOrganizationHTTPService{})
	req := httptest.NewRequest(http.MethodGet, "/v1/organizations/org_123", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetOrganization(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
	assertJSONField(t, rec.Body.String(), "error.code", "INTERNAL_ERROR")
}

func newOrganizationHTTPRequest(t *testing.T, method string, target string, body string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertJSONField(t *testing.T, body string, path string, want string) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}

	parts := strings.Split(path, ".")
	var current any = payload
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected object at %s in %s", part, body)
		}
		current = object[part]
	}

	got, ok := current.(string)
	if !ok {
		t.Fatalf("expected string at %s, got %T", path, current)
	}
	if got != want {
		t.Fatalf("expected %s to be %s, got %s", path, want, got)
	}
}
