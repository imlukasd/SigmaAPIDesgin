package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"corebe.local/api/internal/modules/tenancy"
)

type fakeProductHTTPService struct {
	createRequest CreateProductRequest
	createResult  CreateProductResult
	createErr     error
	getRequest    GetProductRequest
	getResult     GetProductResult
	getErr        error
}

func (f *fakeProductHTTPService) CreateProduct(ctx context.Context, request CreateProductRequest) (CreateProductResult, error) {
	f.createRequest = request
	if f.createErr != nil {
		return CreateProductResult{}, f.createErr
	}
	return f.createResult, nil
}

func (f *fakeProductHTTPService) GetProduct(ctx context.Context, request GetProductRequest) (GetProductResult, error) {
	f.getRequest = request
	if f.getErr != nil {
		return GetProductResult{}, f.getErr
	}
	return f.getResult, nil
}

func TestHandlerCreateProduct(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	description := "Test product"
	service := &fakeProductHTTPService{
		createResult: CreateProductResult{
			Product: Product{
				ID:             "product_123",
				OrganizationID: "org_123",
				SKU:            "SKU_123",
				Name:           "Test Product",
				Description:    &description,
				PriceAmount:    1299,
				PriceCurrency:  "USD",
				Status:         ProductStatusDraft,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	handler := NewHandler(service)
	req := newProductHTTPRequest(t, http.MethodPost, "/v1/organizations/org_123/products", `{
		"sku": "sku_123",
		"name": "Test Product",
		"description": "Test product",
		"price_amount": 1299,
		"price_currency": "usd"
	}`)
	req = req.WithContext(tenancy.ContextWithTenant(req.Context(), tenancy.TenantContext{
		Organization: tenancy.Organization{ID: "org_123"},
		Permission:   tenancy.PermissionProductManage,
	}))
	rec := httptest.NewRecorder()

	handler.HandleCreateProduct(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.createRequest.OrganizationID != "org_123" {
		t.Fatalf("expected organization id from tenant context, got %s", service.createRequest.OrganizationID)
	}
	if service.createRequest.SKU != "sku_123" {
		t.Fatalf("expected raw sku passed to service, got %s", service.createRequest.SKU)
	}
	if service.createRequest.PriceAmount != 1299 {
		t.Fatalf("expected price amount 1299, got %d", service.createRequest.PriceAmount)
	}
	assertCatalogJSONField(t, rec.Body.String(), "data.product.id", "product_123")
	assertCatalogJSONField(t, rec.Body.String(), "data.product.sku", "SKU_123")
	assertCatalogJSONField(t, rec.Body.String(), "data.product.status", string(ProductStatusDraft))
}

func TestHandlerCreateProductMapsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid input", err: ErrInvalidProductInput, wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST"},
		{name: "duplicate sku", err: ErrProductSKUExists, wantStatus: http.StatusConflict, wantCode: "PRODUCT_SKU_EXISTS"},
		{name: "unknown", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeProductHTTPService{createErr: tt.err}
			handler := NewHandler(service)
			req := newProductHTTPRequest(t, http.MethodPost, "/v1/organizations/org_123/products", `{
				"sku": "SKU_123",
				"name": "Test Product",
				"price_amount": 1299,
				"price_currency": "USD"
			}`)
			req = req.WithContext(tenancy.ContextWithTenant(req.Context(), tenancy.TenantContext{
				Organization: tenancy.Organization{ID: "org_123"},
				Permission:   tenancy.PermissionProductManage,
			}))
			rec := httptest.NewRecorder()

			handler.HandleCreateProduct(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			assertCatalogJSONField(t, rec.Body.String(), "error.code", tt.wantCode)
		})
	}
}

func TestHandlerCreateProductRejectsMissingPriceAmount(t *testing.T) {
	t.Parallel()

	service := &fakeProductHTTPService{}
	handler := NewHandler(service)
	req := newProductHTTPRequest(t, http.MethodPost, "/v1/organizations/org_123/products", `{
		"sku": "SKU_123",
		"name": "Test Product",
		"price_currency": "USD"
	}`)
	req = req.WithContext(tenancy.ContextWithTenant(req.Context(), tenancy.TenantContext{
		Organization: tenancy.Organization{ID: "org_123"},
		Permission:   tenancy.PermissionProductManage,
	}))
	rec := httptest.NewRecorder()

	handler.HandleCreateProduct(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	assertCatalogJSONField(t, rec.Body.String(), "error.code", "INVALID_REQUEST")
	if service.createRequest.OrganizationID != "" {
		t.Fatalf("expected service not to be called, got %+v", service.createRequest)
	}
}

func TestHandlerCreateProductRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	service := &fakeProductHTTPService{}
	handler := NewHandler(service)
	req := newProductHTTPRequest(t, http.MethodPost, "/v1/organizations/org_123/products", `{
		"sku": "SKU_123",
		"name": "Test Product",
		"price_amount": 1299,
		"price_currency": "USD",
		"unexpected": true
	}`)
	req = req.WithContext(tenancy.ContextWithTenant(req.Context(), tenancy.TenantContext{
		Organization: tenancy.Organization{ID: "org_123"},
		Permission:   tenancy.PermissionProductManage,
	}))
	rec := httptest.NewRecorder()

	handler.HandleCreateProduct(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	assertCatalogJSONField(t, rec.Body.String(), "error.code", "INVALID_JSON")
	if service.createRequest.OrganizationID != "" {
		t.Fatalf("expected service not to be called, got %+v", service.createRequest)
	}
}

func TestHandlerCreateProductRejectsMissingTenantContext(t *testing.T) {
	t.Parallel()

	service := &fakeProductHTTPService{}
	handler := NewHandler(service)
	req := newProductHTTPRequest(t, http.MethodPost, "/v1/organizations/org_123/products", `{
		"sku": "SKU_123",
		"name": "Test Product",
		"price_amount": 1299,
		"price_currency": "USD"
	}`)
	rec := httptest.NewRecorder()

	handler.HandleCreateProduct(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.createRequest.OrganizationID != "" {
		t.Fatalf("expected service not to be called, got %+v", service.createRequest)
	}
}

func TestHandlerGetProduct(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	service := &fakeProductHTTPService{
		getResult: GetProductResult{
			Product: Product{
				ID:             "product_123",
				OrganizationID: "org_123",
				SKU:            "SKU_123",
				Name:           "Test Product",
				PriceAmount:    1299,
				PriceCurrency:  "USD",
				Status:         ProductStatusDraft,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	handler := NewHandler(service)
	req := httptest.NewRequest(http.MethodGet, "/v1/organizations/org_123/products/product_123", nil)
	req.SetPathValue(productIDPathParam, " product_123 ")
	req = req.WithContext(tenancy.ContextWithTenant(req.Context(), tenancy.TenantContext{
		Organization: tenancy.Organization{ID: "org_123"},
		Permission:   tenancy.PermissionProductRead,
	}))
	rec := httptest.NewRecorder()

	handler.HandleGetProduct(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.getRequest.OrganizationID != "org_123" {
		t.Fatalf("expected organization id from tenant context, got %s", service.getRequest.OrganizationID)
	}
	if service.getRequest.ProductID != " product_123 " {
		t.Fatalf("expected raw product id path value, got %s", service.getRequest.ProductID)
	}
	assertCatalogJSONField(t, rec.Body.String(), "data.product.id", "product_123")
	assertCatalogJSONField(t, rec.Body.String(), "data.product.sku", "SKU_123")
}

func TestHandlerGetProductMapsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid input", err: ErrInvalidProductInput, wantStatus: http.StatusBadRequest, wantCode: "INVALID_REQUEST"},
		{name: "not found", err: ErrProductNotFound, wantStatus: http.StatusNotFound, wantCode: "PRODUCT_NOT_FOUND"},
		{name: "unknown", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeProductHTTPService{getErr: tt.err}
			handler := NewHandler(service)
			req := httptest.NewRequest(http.MethodGet, "/v1/organizations/org_123/products/product_123", nil)
			req.SetPathValue(productIDPathParam, "product_123")
			req = req.WithContext(tenancy.ContextWithTenant(req.Context(), tenancy.TenantContext{
				Organization: tenancy.Organization{ID: "org_123"},
				Permission:   tenancy.PermissionProductRead,
			}))
			rec := httptest.NewRecorder()

			handler.HandleGetProduct(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			assertCatalogJSONField(t, rec.Body.String(), "error.code", tt.wantCode)
		})
	}
}

func TestHandlerGetProductRejectsMissingTenantContext(t *testing.T) {
	t.Parallel()

	service := &fakeProductHTTPService{}
	handler := NewHandler(service)
	req := httptest.NewRequest(http.MethodGet, "/v1/organizations/org_123/products/product_123", nil)
	req.SetPathValue(productIDPathParam, "product_123")
	rec := httptest.NewRecorder()

	handler.HandleGetProduct(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if service.getRequest.ProductID != "" {
		t.Fatalf("expected service not to be called, got %+v", service.getRequest)
	}
}

func newProductHTTPRequest(t *testing.T, method string, target string, body string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertCatalogJSONField(t *testing.T, body string, path string, want string) {
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
