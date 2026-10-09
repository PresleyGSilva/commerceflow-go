package http

import (
	"net/http"

	"github.com/PresleyGSilva/commerceflow-go/internal/httpx"
)

// Envelope helpers re-exported from the leaf httpx package
// (kept here so handlers importing internal/http keep working).
func JSON(w http.ResponseWriter, status int, payload any) { httpx.JSON(w, status, payload) }
func OK(w http.ResponseWriter, data any)                  { httpx.OK(w, data) }
func Error(w http.ResponseWriter, status int, code, message string) {
	httpx.Error(w, status, code, message)
}
