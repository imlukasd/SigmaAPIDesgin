package response

import (
	"encoding/json"
	"net/http"
)

type ErrorBody struct {
	Error ErrorDetail    `json:"error"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type ErrorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func Error(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	JSON(w, status, ErrorBody{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
