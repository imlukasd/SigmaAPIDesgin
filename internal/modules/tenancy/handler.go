package tenancy

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"corebe.local/api/internal/modules/auth"
	"corebe.local/api/internal/platform/response"
)

const maxOrganizationRequestBodyBytes = 1 << 20

type Handler struct {
	service OrganizationService
}

type createOrganizationHTTPBody struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type organizationHTTPBody struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Slug      string             `json:"slug"`
	Status    OrganizationStatus `json:"status"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type membershipHTTPBody struct {
	OrganizationID string           `json:"organization_id"`
	UserID         string           `json:"user_id"`
	Role           MembershipRole   `json:"role"`
	Status         MembershipStatus `json:"status"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

func NewHandler(service OrganizationService) Handler {
	return Handler{service: service}
}

func (h Handler) HandleCreateOrganization(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.AuthenticatedUserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Authenticated user missing from request context.", nil)
		return
	}

	var body createOrganizationHTTPBody
	if !decodeOrganizationJSON(w, r, &body) {
		return
	}

	result, err := h.service.CreateOrganization(r.Context(), CreateOrganizationRequest{
		Name:        body.Name,
		Slug:        body.Slug,
		OwnerUserID: user.UserID,
	})
	if err != nil {
		writeOrganizationError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"organization":     organizationResponse(result.Organization),
			"owner_membership": membershipResponse(result.Owner),
		},
	})
}

func (h Handler) HandleGetOrganization(w http.ResponseWriter, r *http.Request) {
	tenant, ok := TenantFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Tenant missing from request context.", nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"organization": organizationResponse(tenant.Organization),
			"membership":   membershipResponse(tenant.Membership),
			"permission":   tenant.Permission,
		},
	})
}

func decodeOrganizationJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if !hasJSONContentType(r) {
		response.Error(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Request body must be JSON.", nil)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxOrganizationRequestBodyBytes)
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

func organizationResponse(organization Organization) organizationHTTPBody {
	return organizationHTTPBody{
		ID:        organization.ID,
		Name:      organization.Name,
		Slug:      organization.Slug,
		Status:    organization.Status,
		CreatedAt: organization.CreatedAt,
		UpdatedAt: organization.UpdatedAt,
	}
}

func membershipResponse(membership Membership) membershipHTTPBody {
	return membershipHTTPBody{
		OrganizationID: membership.OrganizationID,
		UserID:         membership.UserID,
		Role:           membership.Role,
		Status:         membership.Status,
		CreatedAt:      membership.CreatedAt,
		UpdatedAt:      membership.UpdatedAt,
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

func writeOrganizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidOrganizationInput):
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid organization input.", nil)
	case errors.Is(err, ErrOrganizationSlugExists):
		response.Error(w, http.StatusConflict, "ORGANIZATION_SLUG_EXISTS", "Organization slug already exists.", nil)
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
