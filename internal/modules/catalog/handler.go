package catalog

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"corebe.local/api/internal/modules/tenancy"
	"corebe.local/api/internal/platform/response"
)

const (
	maxProductRequestBodyBytes = 1 << 20
	productIDPathParam         = "product_id"
)

type Handler struct {
	service ProductService
}

type createProductHTTPBody struct {
	SKU           string  `json:"sku"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	PriceAmount   *int64  `json:"price_amount"`
	PriceCurrency string  `json:"price_currency"`
}

type productHTTPBody struct {
	ID             string        `json:"id"`
	OrganizationID string        `json:"organization_id"`
	SKU            string        `json:"sku"`
	Name           string        `json:"name"`
	Description    *string       `json:"description"`
	PriceAmount    int64         `json:"price_amount"`
	PriceCurrency  string        `json:"price_currency"`
	Status         ProductStatus `json:"status"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func NewHandler(service ProductService) Handler {
	return Handler{service: service}
}

func (h Handler) HandleCreateProduct(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenancy.TenantFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Tenant missing from request context.", nil)
		return
	}

	var body createProductHTTPBody
	if !decodeProductJSON(w, r, &body) {
		return
	}
	if body.PriceAmount == nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid product input.", nil)
		return
	}

	result, err := h.service.CreateProduct(r.Context(), CreateProductRequest{
		OrganizationID: tenant.Organization.ID,
		SKU:            body.SKU,
		Name:           body.Name,
		Description:    body.Description,
		PriceAmount:    *body.PriceAmount,
		PriceCurrency:  body.PriceCurrency,
	})
	if err != nil {
		writeProductError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"product": productResponse(result.Product),
		},
	})
}

func (h Handler) HandleGetProduct(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenancy.TenantFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Tenant missing from request context.", nil)
		return
	}

	result, err := h.service.GetProduct(r.Context(), GetProductRequest{
		OrganizationID: tenant.Organization.ID,
		ProductID:      r.PathValue(productIDPathParam),
	})
	if err != nil {
		writeProductError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"product": productResponse(result.Product),
		},
	})
}

func decodeProductJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if !hasJSONContentType(r) {
		response.Error(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Request body must be JSON.", nil)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxProductRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Request body must be valid JSON.", nil)
		return false
	}

	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "Request body must contain a single JSON object.", nil)
		return false
	}

	return true
}

func productResponse(product Product) productHTTPBody {
	return productHTTPBody{
		ID:             product.ID,
		OrganizationID: product.OrganizationID,
		SKU:            product.SKU,
		Name:           product.Name,
		Description:    product.Description,
		PriceAmount:    product.PriceAmount,
		PriceCurrency:  product.PriceCurrency,
		Status:         product.Status,
		CreatedAt:      product.CreatedAt,
		UpdatedAt:      product.UpdatedAt,
	}
}

func hasJSONContentType(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "application/json"
}

func writeProductError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidProductInput):
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid product input.", nil)
	case errors.Is(err, ErrProductSKUExists):
		response.Error(w, http.StatusConflict, "PRODUCT_SKU_EXISTS", "Product SKU already exists.", nil)
	case errors.Is(err, ErrProductNotFound):
		response.Error(w, http.StatusNotFound, "PRODUCT_NOT_FOUND", "Product not found.", nil)
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
