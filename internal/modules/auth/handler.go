package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"corebe.local/api/internal/platform/response"
)

const maxAuthRequestBodyBytes = 1 << 20

type Handler struct {
	service AuthService
}

type registerHTTPBody struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginHTTPBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshHTTPBody struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutHTTPBody struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenPairHTTPBody struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func NewHandler(service AuthService) Handler {
	return Handler{service: service}
}

func (h Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var body registerHTTPBody
	if !decodeAuthJSON(w, r, &body) {
		return
	}

	result, err := h.service.Register(r.Context(), RegisterRequest{
		Email:       body.Email,
		Password:    body.Password,
		DisplayName: body.DisplayName,
	})
	if err != nil {
		writeAuthError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"data": map[string]any{
			"user_id": result.UserID,
			"email":   result.Email,
		},
	})
}

func (h Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var body loginHTTPBody
	if !decodeAuthJSON(w, r, &body) {
		return
	}

	result, err := h.service.Login(r.Context(), LoginRequest{
		Email:     body.Email,
		Password:  body.Password,
		UserAgent: r.UserAgent(),
		IPAddress: clientIPFromRequest(r),
	})
	if err != nil {
		writeAuthError(w, err)
		return
	}

	writeTokenResult(w, http.StatusOK, result.UserID, result.Email, result.Tokens)
}

func (h Handler) HandleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var body refreshHTTPBody
	if !decodeAuthJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.RefreshToken) == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Refresh token is required.", nil)
		return
	}

	result, err := h.service.RefreshToken(r.Context(), RefreshTokenRequest{
		RefreshToken: body.RefreshToken,
	})
	if err != nil {
		writeAuthError(w, err)
		return
	}

	writeTokenResult(w, http.StatusOK, result.UserID, result.Email, result.Tokens)
}

func (h Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	var body logoutHTTPBody
	if !decodeAuthJSON(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.RefreshToken) == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Refresh token is required.", nil)
		return
	}

	if err := h.service.Logout(r.Context(), LogoutRequest{
		RefreshToken: body.RefreshToken,
	}); err != nil {
		writeAuthError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := AuthenticatedUserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Authenticated user missing from request context.", nil)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user_id":         user.UserID,
			"session_id":      user.SessionID,
			"access_token_id": user.AccessTokenID,
			"issued_at":       user.IssuedAt,
			"expires_at":      user.ExpiresAt,
		},
	})
}

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if !hasJSONContentType(r) {
		response.Error(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Request body must be JSON.", nil)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBodyBytes)
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

func hasJSONContentType(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "application/json"
}

func writeTokenResult(w http.ResponseWriter, status int, userID string, email string, tokens TokenPair) {
	response.JSON(w, status, map[string]any{
		"data": map[string]any{
			"user_id": userID,
			"email":   email,
			"tokens": tokenPairHTTPBody{
				AccessToken:           tokens.AccessToken,
				AccessTokenExpiresAt:  tokens.AccessTokenExpiresAt,
				RefreshToken:          tokens.RefreshToken,
				RefreshTokenExpiresAt: tokens.RefreshTokenExpiresAt,
			},
		},
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRegistrationInput):
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid registration input.", nil)
	case errors.Is(err, ErrEmailAlreadyExists):
		response.Error(w, http.StatusConflict, "EMAIL_ALREADY_EXISTS", "Email already exists.", nil)
	case errors.Is(err, ErrInvalidCredentials):
		response.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.", nil)
	case errors.Is(err, ErrUserDisabled):
		response.Error(w, http.StatusForbidden, "USER_DISABLED", "User is disabled.", nil)
	case errors.Is(err, ErrInvalidRefreshToken):
		response.Error(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Invalid refresh token.", nil)
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error.", nil)
	}
}

func clientIPFromRequest(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
