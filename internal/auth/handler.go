package auth

import (
	"net/http"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

// Handler handles HTTP authentication endpoints.
// The full implementation (Login, middleware, etc.) is added in Task 5.
type Handler struct {
	db     *db.DB
	secret []byte
}

// NewHandler constructs an auth Handler.
func NewHandler(database *db.DB, secret []byte) *Handler {
	return &Handler{db: database, secret: secret}
}

// Login is a placeholder; the real implementation is added in Task 5.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
