package admin

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flashyspeed/flashyspeed/internal/auth"
)

// Handler handles admin-only HTTP endpoints.
type Handler struct{}

// NewHandler returns a new admin Handler.
func NewHandler() *Handler { return &Handler{} }

// TailscaleStatus handles GET /api/admin/tailscale/status
func (h *Handler) TailscaleStatus(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	status, err := TailscaleStatusCheck()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// TailscaleInstall handles POST /api/admin/tailscale/install
func (h *Handler) TailscaleInstall(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	if err := TailscaleInstall(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"installed"}`)
}

// TailscaleUp handles POST /api/admin/tailscale/up
// Body: {"auth_key": "tskey-..."}
func (h *Handler) TailscaleUp(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var body struct {
		AuthKey string `json:"auth_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.AuthKey == "" {
		http.Error(w, `{"error":"auth_key is required"}`, http.StatusBadRequest)
		return
	}

	if err := TailscaleUp(body.AuthKey); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"connected"}`)
}
