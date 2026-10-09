package httpx

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON payload with status code.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// OK wraps data in the {data} envelope.
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, map[string]any{"data": data})
}

// Error writes the standard {error:{code,message}} envelope.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}
