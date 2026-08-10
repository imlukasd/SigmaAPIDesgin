package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"corebe.local/api/internal/config"
	"corebe.local/api/internal/modules/auth"
	"corebe.local/api/internal/modules/catalog"
	"corebe.local/api/internal/modules/health"
	"corebe.local/api/internal/modules/identity"
	"corebe.local/api/internal/modules/payment"
	"corebe.local/api/internal/modules/tenancy"
	"corebe.local/api/internal/platform/database"
	"corebe.local/api/internal/platform/middleware"
	"corebe.local/api/internal/platform/response"
)

func New(cfg config.Config, logger *slog.Logger, db *pgxpool.Pool) (*http.Server, error) {
	mux := http.NewServeMux()
	healthHandler := health.NewHandler(db)
	authHandler, tokenVerifier, err := newAuthHandler(cfg, db)
	if err != nil {
		return nil, err
	}
	tenancyHandler, tenancyAuthorizer := newTenancyHandler(db)
	catalogHandler := newCatalogHandler(db)
	requireAuth := auth.RequireAuth(tokenVerifier)

	mux.HandleFunc("GET /healthz", healthHandler.HandleHealthz)
	mux.HandleFunc("GET /readyz", healthHandler.HandleReadyz)
	mux.HandleFunc("POST /v1/auth/register", authHandler.HandleRegister)
	mux.HandleFunc("POST /v1/auth/login", authHandler.HandleLogin)
	mux.HandleFunc("POST /v1/auth/refresh", authHandler.HandleRefreshToken)
	mux.HandleFunc("POST /v1/auth/logout", authHandler.HandleLogout)
	mux.Handle("GET /v1/auth/me", requireAuth(http.HandlerFunc(authHandler.HandleMe)))
	mux.Handle("POST /v1/organizations", requireAuth(http.HandlerFunc(tenancyHandler.HandleCreateOrganization)))
	mux.Handle("GET /v1/organizations/{organization_id}", middleware.Chain(
		http.HandlerFunc(tenancyHandler.HandleGetOrganization),
		requireAuth,
		tenancy.RequireTenant(tenancyAuthorizer, tenancy.PermissionOrganizationRead),
	))
	mux.Handle("POST /v1/organizations/{organization_id}/products", middleware.Chain(
		http.HandlerFunc(catalogHandler.HandleCreateProduct),
		requireAuth,
		tenancy.RequireTenant(tenancyAuthorizer, tenancy.PermissionProductManage),
	))
	mux.Handle("GET /v1/organizations/{organization_id}/products/{product_id}", middleware.Chain(
		http.HandlerFunc(catalogHandler.HandleGetProduct),
		requireAuth,
		tenancy.RequireTenant(tenancyAuthorizer, tenancy.PermissionProductRead),
	))
	mux.HandleFunc("GET /v1/payments/capabilities", payment.HandleCapabilities)
	mux.HandleFunc("GET /v1", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"name":    "CoreBE API",
				"version": "v1",
				"env":     cfg.AppEnv,
			},
		})
	})

	handler := middleware.Chain(
		mux,
		middleware.Recover(logger),
		middleware.RequestID,
		middleware.AccessLog(logger),
		middleware.SecurityHeaders,
	)

	return &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}, nil
}

func newAuthHandler(cfg config.Config, db *pgxpool.Pool) (auth.Handler, auth.TokenVerifier, error) {
	var handler auth.Handler
	var verifier auth.TokenVerifier

	passwordHasher, err := auth.NewBcryptPasswordHasher(cfg.Auth.PasswordBcryptCost)
	if err != nil {
		return handler, verifier, fmt.Errorf("create password hasher: %w", err)
	}

	tokenManager, err := auth.NewTokenManager(auth.TokenConfig{
		Issuer:            cfg.Auth.AccessTokenIssuer,
		Audience:          cfg.Auth.AccessTokenAudience,
		AccessTokenSecret: cfg.Auth.AccessTokenSecret,
		AccessTokenTTL:    cfg.Auth.AccessTokenTTL,
		RefreshTokenTTL:   cfg.Auth.RefreshTokenTTL,
	})
	if err != nil {
		return handler, verifier, fmt.Errorf("create token manager: %w", err)
	}

	service := auth.NewService(auth.ServiceConfig{
		DB:          db,
		Transactor:  database.NewTransactor(db),
		Users:       identity.NewRepository(),
		Sessions:    auth.NewRepository(),
		Passwords:   passwordHasher,
		TokenIssuer: tokenManager,
	})

	return auth.NewHandler(service), tokenManager, nil
}

func newTenancyHandler(db *pgxpool.Pool) (tenancy.Handler, tenancy.Authorizer) {
	service := tenancy.NewService(tenancy.ServiceConfig{
		DB:         db,
		Transactor: database.NewTransactor(db),
		Store:      tenancy.NewRepository(),
	})

	return tenancy.NewHandler(service), service
}

func newCatalogHandler(db *pgxpool.Pool) catalog.Handler {
	service := catalog.NewService(catalog.ServiceConfig{
		DB:    db,
		Store: catalog.NewRepository(),
	})

	return catalog.NewHandler(service)
}
