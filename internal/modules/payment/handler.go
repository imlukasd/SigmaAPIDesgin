package payment

import (
	"net/http"

	"corebe.local/api/internal/platform/response"
)

func HandleCapabilities(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"stage": "planned",
			"principles": []string{
				"idempotency-required-for-money-moving-requests",
				"webhook-signature-verification",
				"provider-agnostic-domain-model",
				"audit-log-for-sensitive-events",
			},
		},
	})
}
