package tenancy

import (
	"errors"
	"net/http"
	"strings"

	"corebe.local/api/internal/modules/auth"
	"corebe.local/api/internal/platform/middleware"
	"corebe.local/api/internal/platform/response"
)

const organizationIDPathParam = "organization_id"

func RequireTenant(authorizer Authorizer, permission Permission) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.AuthenticatedUserFromContext(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication is required.", nil)
				return
			}

			organizationID := strings.TrimSpace(r.PathValue(organizationIDPathParam))
			if organizationID == "" {
				response.Error(w, http.StatusBadRequest, "INVALID_TENANT", "Organization id is required.", nil)
				return
			}

			if authorizer == nil || permission == "" {
				response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
				return
			}

			result, err := authorizer.Authorize(r.Context(), AuthorizeRequest{
				OrganizationID: organizationID,
				UserID:         user.UserID,
				Permission:     permission,
			})
			if err != nil {
				writeTenantAuthorizationError(w, err)
				return
			}

			tenant := TenantContext{
				Organization: result.Organization,
				Membership:   result.Membership,
				Permission:   permission,
			}
			next.ServeHTTP(w, r.WithContext(ContextWithTenant(r.Context(), tenant)))
		})
	}
}

func writeTenantAuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidAuthorization):
		response.Error(w, http.StatusBadRequest, "INVALID_TENANT", "Invalid tenant authorization request.", nil)
	case errors.Is(err, ErrTenantAccessDenied):
		response.Error(w, http.StatusForbidden, "TENANT_ACCESS_DENIED", "Tenant access denied.", nil)
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}
