package tenancy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"corebe.local/api/internal/modules/auth"
)

type fakeAuthorizer struct {
	request AuthorizeRequest
	result  AuthorizationResult
	err     error
}

func (f *fakeAuthorizer) Authorize(ctx context.Context, request AuthorizeRequest) (AuthorizationResult, error) {
	f.request = request
	if f.err != nil {
		return AuthorizationResult{}, f.err
	}
	return f.result, nil
}

func TestRequireTenant(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{
		result: AuthorizationResult{
			Organization: Organization{
				ID:     "org_123",
				Name:   "Test Organization",
				Status: OrganizationStatusActive,
			},
			Membership: Membership{
				OrganizationID: "org_123",
				UserID:         "user_123",
				Role:           MembershipRoleAdmin,
				Status:         MembershipStatusActive,
			},
		},
	}
	var gotTenant TenantContext
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		var ok bool
		gotTenant, ok = TenantFromContext(r.Context())
		if !ok {
			t.Fatal("expected tenant context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequireTenant(authorizer, PermissionOrganizationManage)(next)
	req := newTenantRequest("/v1/organizations/org_123", " org_123 ")
	req = req.WithContext(auth.ContextWithAuthenticatedUser(req.Context(), auth.AuthenticatedUser{
		UserID:        "user_123",
		SessionID:     "session_123",
		AccessTokenID: "access_token_123",
		IssuedAt:      time.Now(),
		ExpiresAt:     time.Now().Add(time.Minute),
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if authorizer.request.OrganizationID != "org_123" {
		t.Fatalf("expected organization id org_123, got %s", authorizer.request.OrganizationID)
	}
	if authorizer.request.UserID != "user_123" {
		t.Fatalf("expected user id user_123, got %s", authorizer.request.UserID)
	}
	if authorizer.request.Permission != PermissionOrganizationManage {
		t.Fatalf("expected permission %s, got %s", PermissionOrganizationManage, authorizer.request.Permission)
	}
	if gotTenant.Organization.ID != "org_123" {
		t.Fatalf("expected tenant organization org_123, got %+v", gotTenant.Organization)
	}
	if gotTenant.Membership.Role != MembershipRoleAdmin {
		t.Fatalf("expected tenant membership role admin, got %+v", gotTenant.Membership)
	}
	if gotTenant.Permission != PermissionOrganizationManage {
		t.Fatalf("expected tenant permission %s, got %s", PermissionOrganizationManage, gotTenant.Permission)
	}
}

func TestRequireTenantRejectsMissingAuthContext(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{}
	handler := RequireTenant(authorizer, PermissionOrganizationRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	}))
	req := newTenantRequest("/v1/organizations/org_123", "org_123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if authorizer.request.OrganizationID != "" {
		t.Fatalf("expected authorizer not to be called, got %+v", authorizer.request)
	}
}

func TestRequireTenantRejectsMissingOrganizationID(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{}
	handler := RequireTenant(authorizer, PermissionOrganizationRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	}))
	req := newTenantRequest("/v1/organizations/", "")
	req = req.WithContext(auth.ContextWithAuthenticatedUser(req.Context(), auth.AuthenticatedUser{
		UserID: "user_123",
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if authorizer.request.OrganizationID != "" {
		t.Fatalf("expected authorizer not to be called, got %+v", authorizer.request)
	}
}

func TestRequireTenantRejectsDeniedAccess(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{err: ErrTenantAccessDenied}
	handler := RequireTenant(authorizer, PermissionOrganizationManage)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	}))
	req := newAuthenticatedTenantRequest("org_123", "user_123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireTenantMapsInvalidAuthorization(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{err: ErrInvalidAuthorization}
	handler := RequireTenant(authorizer, PermissionOrganizationRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	}))
	req := newAuthenticatedTenantRequest("org_123", "user_123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireTenantMapsUnexpectedAuthorizationError(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{err: errors.New("database failed")}
	handler := RequireTenant(authorizer, PermissionOrganizationRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	}))
	req := newAuthenticatedTenantRequest("org_123", "user_123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireTenantRejectsInvalidMiddlewareConfiguration(t *testing.T) {
	t.Parallel()

	handler := RequireTenant(nil, PermissionOrganizationRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not be called")
	}))
	req := newAuthenticatedTenantRequest("org_123", "user_123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func newAuthenticatedTenantRequest(organizationID string, userID string) *http.Request {
	req := newTenantRequest("/v1/organizations/"+organizationID, organizationID)
	return req.WithContext(auth.ContextWithAuthenticatedUser(req.Context(), auth.AuthenticatedUser{
		UserID: userID,
	}))
}

func newTenantRequest(target string, organizationID string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue(organizationIDPathParam, organizationID)
	return req
}
