package health

import (
	"context"
	"net/http"
	"time"

	"corebe.local/api/internal/platform/response"
)

type Database interface {
	Ping(context.Context) error
}

type Handler struct {
	db Database
}

func NewHandler(db Database) Handler {
	return Handler{db: db}
}

func (h Handler) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func (h Handler) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		response.Error(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Service is not ready.", map[string]any{
			"database": "unreachable",
		})
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"status": "ready",
			"checks": map[string]any{
				"database": "ok",
			},
		},
	})
}
