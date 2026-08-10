package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"corebe.local/api/internal/config"
	"corebe.local/api/internal/modules/auth"
	"corebe.local/api/internal/platform/testdb"
)

func TestAuthHTTPFlowIntegration(t *testing.T) {
	db := testdb.Open(t)

	email := "http-auth-" + testdb.UniqueSuffix(t) + "@example.com"
	organizationSlug := "http-org-" + testdb.UniqueSuffix(t)[:24]
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM users WHERE email_normalized = $1`, email)
	})
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM organizations WHERE slug = $1`, organizationSlug)
	})

	server, err := New(testHTTPConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	if err != nil {
		t.Fatalf("new http server: %v", err)
	}

	registerResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":        email,
		"password":     "secret123",
		"display_name": "HTTP Auth User",
	}, "")
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("expected register status 201, got %d: %s", registerResponse.Code, registerResponse.Body.String())
	}

	loginResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": "secret123",
	}, "")
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
	login := decodeAuthIntegrationResponse(t, loginResponse.Body.Bytes())
	if login.Data.UserID == "" {
		t.Fatal("expected login user id")
	}
	if login.Data.Tokens.AccessToken == "" {
		t.Fatal("expected login access token")
	}
	if login.Data.Tokens.RefreshToken == "" {
		t.Fatal("expected login refresh token")
	}

	unauthorizedMeResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/auth/me", nil, "")
	if unauthorizedMeResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated me status 401, got %d: %s", unauthorizedMeResponse.Code, unauthorizedMeResponse.Body.String())
	}

	meResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/auth/me", nil, login.Data.Tokens.AccessToken)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("expected me status 200, got %d: %s", meResponse.Code, meResponse.Body.String())
	}
	me := decodeAuthIntegrationResponse(t, meResponse.Body.Bytes())
	if me.Data.UserID != login.Data.UserID {
		t.Fatalf("expected me user id %s, got %s", login.Data.UserID, me.Data.UserID)
	}

	createOrganizationResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/organizations", map[string]any{
		"name": "HTTP Test Organization",
		"slug": organizationSlug,
	}, login.Data.Tokens.AccessToken)
	if createOrganizationResponse.Code != http.StatusCreated {
		t.Fatalf("expected create organization status 201, got %d: %s", createOrganizationResponse.Code, createOrganizationResponse.Body.String())
	}
	createdOrganization := decodeOrganizationIntegrationResponse(t, createOrganizationResponse.Body.Bytes())
	if createdOrganization.Data.Organization.ID == "" {
		t.Fatal("expected created organization id")
	}
	if createdOrganization.Data.Organization.Slug != organizationSlug {
		t.Fatalf("expected organization slug %s, got %s", organizationSlug, createdOrganization.Data.Organization.Slug)
	}
	if createdOrganization.Data.OwnerMembership.Role != "owner" {
		t.Fatalf("expected owner membership role, got %s", createdOrganization.Data.OwnerMembership.Role)
	}

	getOrganizationResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/organizations/"+createdOrganization.Data.Organization.ID, nil, login.Data.Tokens.AccessToken)
	if getOrganizationResponse.Code != http.StatusOK {
		t.Fatalf("expected get organization status 200, got %d: %s", getOrganizationResponse.Code, getOrganizationResponse.Body.String())
	}
	gotOrganization := decodeOrganizationIntegrationResponse(t, getOrganizationResponse.Body.Bytes())
	if gotOrganization.Data.Organization.ID != createdOrganization.Data.Organization.ID {
		t.Fatalf("expected organization id %s, got %s", createdOrganization.Data.Organization.ID, gotOrganization.Data.Organization.ID)
	}
	if gotOrganization.Data.Membership.Role != "owner" {
		t.Fatalf("expected organization membership role owner, got %s", gotOrganization.Data.Membership.Role)
	}
	if gotOrganization.Data.Permission != "organization:read" {
		t.Fatalf("expected organization read permission, got %s", gotOrganization.Data.Permission)
	}

	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM products WHERE organization_id = $1`, createdOrganization.Data.Organization.ID)
	})

	createProductResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/organizations/"+createdOrganization.Data.Organization.ID+"/products", map[string]any{
		"sku":            "http_sku_" + testdb.UniqueSuffix(t)[:16],
		"name":           "HTTP Test Product",
		"description":    "HTTP integration product",
		"price_amount":   1299,
		"price_currency": "usd",
	}, login.Data.Tokens.AccessToken)
	if createProductResponse.Code != http.StatusCreated {
		t.Fatalf("expected create product status 201, got %d: %s", createProductResponse.Code, createProductResponse.Body.String())
	}
	createdProduct := decodeProductIntegrationResponse(t, createProductResponse.Body.Bytes())
	if createdProduct.Data.Product.ID == "" {
		t.Fatal("expected created product id")
	}
	if createdProduct.Data.Product.OrganizationID != createdOrganization.Data.Organization.ID {
		t.Fatalf("expected product organization id %s, got %s", createdOrganization.Data.Organization.ID, createdProduct.Data.Product.OrganizationID)
	}
	if !strings.HasPrefix(createdProduct.Data.Product.SKU, "HTTP_SKU_") {
		t.Fatalf("expected normalized product sku, got %s", createdProduct.Data.Product.SKU)
	}

	getProductResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/organizations/"+createdOrganization.Data.Organization.ID+"/products/"+createdProduct.Data.Product.ID, nil, login.Data.Tokens.AccessToken)
	if getProductResponse.Code != http.StatusOK {
		t.Fatalf("expected get product status 200, got %d: %s", getProductResponse.Code, getProductResponse.Body.String())
	}
	gotProduct := decodeProductIntegrationResponse(t, getProductResponse.Body.Bytes())
	if gotProduct.Data.Product.ID != createdProduct.Data.Product.ID {
		t.Fatalf("expected product id %s, got %s", createdProduct.Data.Product.ID, gotProduct.Data.Product.ID)
	}

	refreshResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": login.Data.Tokens.RefreshToken,
	}, "")
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("expected refresh status 200, got %d: %s", refreshResponse.Code, refreshResponse.Body.String())
	}
	refreshed := decodeAuthIntegrationResponse(t, refreshResponse.Body.Bytes())
	if refreshed.Data.Tokens.AccessToken == "" {
		t.Fatal("expected refreshed access token")
	}
	if refreshed.Data.Tokens.RefreshToken == "" {
		t.Fatal("expected refreshed refresh token")
	}
	if refreshed.Data.Tokens.RefreshToken == login.Data.Tokens.RefreshToken {
		t.Fatal("expected refresh token rotation")
	}

	reuseResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": login.Data.Tokens.RefreshToken,
	}, "")
	if reuseResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected reused refresh token status 401, got %d: %s", reuseResponse.Code, reuseResponse.Body.String())
	}

	logoutResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/logout", map[string]any{
		"refresh_token": refreshed.Data.Tokens.RefreshToken,
	}, "")
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("expected logout status 204, got %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}

	refreshAfterLogoutResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": refreshed.Data.Tokens.RefreshToken,
	}, "")
	if refreshAfterLogoutResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected refresh after logout status 401, got %d: %s", refreshAfterLogoutResponse.Code, refreshAfterLogoutResponse.Body.String())
	}
}

func TestOrganizationHTTPRejectsCrossTenantAccessIntegration(t *testing.T) {
	db := testdb.Open(t)

	suffix := testdb.UniqueSuffix(t)
	ownerEmail := "tenant-owner-" + suffix + "@example.com"
	outsiderEmail := "tenant-outsider-" + suffix + "@example.com"
	organizationSlug := "tenant-boundary-" + suffix[:16]
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM users WHERE email_normalized IN ($1, $2)`, ownerEmail, outsiderEmail)
	})
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM organizations WHERE slug = $1`, organizationSlug)
	})

	server, err := New(testHTTPConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), db)
	if err != nil {
		t.Fatalf("new http server: %v", err)
	}

	ownerLogin := registerAndLoginUser(t, server.Handler, ownerEmail, "Tenant Owner")
	outsiderLogin := registerAndLoginUser(t, server.Handler, outsiderEmail, "Tenant Outsider")

	createOrganizationResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/organizations", map[string]any{
		"name": "Tenant Boundary Organization",
		"slug": organizationSlug,
	}, ownerLogin.Data.Tokens.AccessToken)
	if createOrganizationResponse.Code != http.StatusCreated {
		t.Fatalf("expected create organization status 201, got %d: %s", createOrganizationResponse.Code, createOrganizationResponse.Body.String())
	}
	createdOrganization := decodeOrganizationIntegrationResponse(t, createOrganizationResponse.Body.Bytes())
	if createdOrganization.Data.Organization.ID == "" {
		t.Fatal("expected created organization id")
	}
	t.Cleanup(func() {
		testdb.Exec(t, db, `DELETE FROM products WHERE organization_id = $1`, createdOrganization.Data.Organization.ID)
	})

	ownerReadResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/organizations/"+createdOrganization.Data.Organization.ID, nil, ownerLogin.Data.Tokens.AccessToken)
	if ownerReadResponse.Code != http.StatusOK {
		t.Fatalf("expected owner get organization status 200, got %d: %s", ownerReadResponse.Code, ownerReadResponse.Body.String())
	}

	createProductResponse := doJSONRequest(t, server.Handler, http.MethodPost, "/v1/organizations/"+createdOrganization.Data.Organization.ID+"/products", map[string]any{
		"sku":            "BOUNDARY_SKU_" + strings.ToUpper(suffix[:12]),
		"name":           "Tenant Boundary Product",
		"price_amount":   1299,
		"price_currency": "USD",
	}, ownerLogin.Data.Tokens.AccessToken)
	if createProductResponse.Code != http.StatusCreated {
		t.Fatalf("expected owner create product status 201, got %d: %s", createProductResponse.Code, createProductResponse.Body.String())
	}
	createdProduct := decodeProductIntegrationResponse(t, createProductResponse.Body.Bytes())
	if createdProduct.Data.Product.ID == "" {
		t.Fatal("expected created product id")
	}

	outsiderReadResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/organizations/"+createdOrganization.Data.Organization.ID, nil, outsiderLogin.Data.Tokens.AccessToken)
	if outsiderReadResponse.Code != http.StatusForbidden {
		t.Fatalf("expected outsider get organization status 403, got %d: %s", outsiderReadResponse.Code, outsiderReadResponse.Body.String())
	}
	var errorResponse struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(outsiderReadResponse.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("decode outsider error response: %v: %s", err, outsiderReadResponse.Body.String())
	}
	if errorResponse.Error.Code != "TENANT_ACCESS_DENIED" {
		t.Fatalf("expected TENANT_ACCESS_DENIED, got %s", errorResponse.Error.Code)
	}

	outsiderProductResponse := doJSONRequest(t, server.Handler, http.MethodGet, "/v1/organizations/"+createdOrganization.Data.Organization.ID+"/products/"+createdProduct.Data.Product.ID, nil, outsiderLogin.Data.Tokens.AccessToken)
	if outsiderProductResponse.Code != http.StatusForbidden {
		t.Fatalf("expected outsider get product status 403, got %d: %s", outsiderProductResponse.Code, outsiderProductResponse.Body.String())
	}
	if err := json.Unmarshal(outsiderProductResponse.Body.Bytes(), &errorResponse); err != nil {
		t.Fatalf("decode outsider product error response: %v: %s", err, outsiderProductResponse.Body.String())
	}
	if errorResponse.Error.Code != "TENANT_ACCESS_DENIED" {
		t.Fatalf("expected TENANT_ACCESS_DENIED, got %s", errorResponse.Error.Code)
	}
}

func testHTTPConfig() config.Config {
	return config.Config{
		AppEnv:          "test",
		HTTPAddr:        ":0",
		ReadTimeout:     time.Second,
		WriteTimeout:    time.Second,
		IdleTimeout:     time.Second,
		ShutdownTimeout: time.Second,
		Auth: config.AuthConfig{
			PasswordBcryptCost:  bcrypt.MinCost,
			AccessTokenIssuer:   "corebe-api",
			AccessTokenAudience: "corebe-api",
			AccessTokenSecret:   strings.Repeat("s", auth.MinAccessTokenSecretBytes),
			AccessTokenTTL:      15 * time.Minute,
			RefreshTokenTTL:     30 * 24 * time.Hour,
		},
	}
}

func doJSONRequest(t *testing.T, handler http.Handler, method string, target string, body any, accessToken string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func registerAndLoginUser(t *testing.T, handler http.Handler, email string, displayName string) authIntegrationResponse {
	t.Helper()

	registerResponse := doJSONRequest(t, handler, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":        email,
		"password":     "secret123",
		"display_name": displayName,
	}, "")
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("expected register status 201 for %s, got %d: %s", email, registerResponse.Code, registerResponse.Body.String())
	}

	loginResponse := doJSONRequest(t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    email,
		"password": "secret123",
	}, "")
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("expected login status 200 for %s, got %d: %s", email, loginResponse.Code, loginResponse.Body.String())
	}

	login := decodeAuthIntegrationResponse(t, loginResponse.Body.Bytes())
	if login.Data.UserID == "" {
		t.Fatalf("expected login user id for %s", email)
	}
	if login.Data.Tokens.AccessToken == "" {
		t.Fatalf("expected login access token for %s", email)
	}
	return login
}

type authIntegrationResponse struct {
	Data struct {
		UserID        string `json:"user_id"`
		SessionID     string `json:"session_id"`
		AccessTokenID string `json:"access_token_id"`
		Email         string `json:"email"`
		Tokens        struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	} `json:"data"`
}

func decodeAuthIntegrationResponse(t *testing.T, payload []byte) authIntegrationResponse {
	t.Helper()

	var response authIntegrationResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode auth response: %v: %s", err, string(payload))
	}
	return response
}

type organizationIntegrationResponse struct {
	Data struct {
		Organization struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"organization"`
		OwnerMembership struct {
			Role string `json:"role"`
		} `json:"owner_membership"`
		Membership struct {
			Role string `json:"role"`
		} `json:"membership"`
		Permission string `json:"permission"`
	} `json:"data"`
}

func decodeOrganizationIntegrationResponse(t *testing.T, payload []byte) organizationIntegrationResponse {
	t.Helper()

	var response organizationIntegrationResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode organization response: %v: %s", err, string(payload))
	}
	return response
}

type productIntegrationResponse struct {
	Data struct {
		Product struct {
			ID             string `json:"id"`
			OrganizationID string `json:"organization_id"`
			SKU            string `json:"sku"`
		} `json:"product"`
	} `json:"data"`
}

func decodeProductIntegrationResponse(t *testing.T, payload []byte) productIntegrationResponse {
	t.Helper()

	var response productIntegrationResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decode product response: %v: %s", err, string(payload))
	}
	return response
}
