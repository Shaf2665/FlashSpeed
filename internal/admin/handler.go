package admin

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

// Handler handles admin-only HTTP endpoints.
type Handler struct {
	db *db.DB
}

// NewHandler returns a new admin Handler.
func NewHandler(database *db.DB) *Handler { return &Handler{db: database} }

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

// ---- User Management ----

type userRow struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	QuotaBytes int64  `json:"quota_bytes"`
}

// ListUsers handles GET /api/admin/users
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	rows, err := h.db.Query(`SELECT id, username, email, role, quota_bytes FROM users ORDER BY id`)
	if err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []userRow
	for rows.Next() {
		var u userRow
		rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.QuotaBytes)
		users = append(users, u)
	}
	if users == nil {
		users = []userRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// CreateUser handles POST /api/admin/users
// Body: {"username":"...", "email":"...", "password":"...", "role":"user|admin", "quota_bytes":0}
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var body struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		Role       string `json:"role"`
		QuotaBytes int64  `json:"quota_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.Username == "" || body.Email == "" || body.Password == "" {
		http.Error(w, `{"error":"username, email and password are required"}`, http.StatusBadRequest)
		return
	}
	if body.Role == "" {
		body.Role = "user"
	}
	if body.Role != "user" && body.Role != "admin" {
		http.Error(w, `{"error":"role must be user or admin"}`, http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		http.Error(w, `{"error":"hash failed"}`, http.StatusInternalServerError)
		return
	}

	res, err := h.db.Exec(
		`INSERT INTO users(username, email, password_hash, role, quota_bytes) VALUES(?,?,?,?,?)`,
		body.Username, body.Email, hash, body.Role, body.QuotaBytes,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusConflict)
		return
	}
	id, _ := res.LastInsertId()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(userRow{
		ID: id, Username: body.Username, Email: body.Email,
		Role: body.Role, QuotaBytes: body.QuotaBytes,
	})
}

// UpdateUser handles PATCH /api/admin/users/{id}
// Body: {"role":"...", "quota_bytes":N, "password":"..."} — all fields optional
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")
	var targetID int64
	if _, err := fmt.Sscan(idStr, &targetID); err != nil || targetID == 0 {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		Role       *string `json:"role"`
		QuotaBytes *int64  `json:"quota_bytes"`
		Password   *string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if body.Role != nil {
		if *body.Role != "user" && *body.Role != "admin" {
			http.Error(w, `{"error":"role must be user or admin"}`, http.StatusBadRequest)
			return
		}
		if _, err := h.db.Exec(`UPDATE users SET role=? WHERE id=?`, *body.Role, targetID); err != nil {
			http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
			return
		}
	}
	if body.QuotaBytes != nil {
		if _, err := h.db.Exec(`UPDATE users SET quota_bytes=? WHERE id=?`, *body.QuotaBytes, targetID); err != nil {
			http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
			return
		}
	}
	if body.Password != nil && *body.Password != "" {
		hash, err := auth.HashPassword(*body.Password)
		if err != nil {
			http.Error(w, `{"error":"hash failed"}`, http.StatusInternalServerError)
			return
		}
		if _, err := h.db.Exec(`UPDATE users SET password_hash=? WHERE id=?`, hash, targetID); err != nil {
			http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
			return
		}
	}

	// Return updated user
	var u userRow
	if err := h.db.QueryRow(
		`SELECT id, username, email, role, quota_bytes FROM users WHERE id=?`, targetID,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.QuotaBytes); err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

// DeleteUser handles DELETE /api/admin/users/{id}
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")
	var targetID int64
	if _, err := fmt.Sscan(idStr, &targetID); err != nil || targetID == 0 {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	// Prevent self-deletion
	if targetID == claims.UserID {
		http.Error(w, `{"error":"cannot delete yourself"}`, http.StatusBadRequest)
		return
	}

	res, err := h.db.Exec(`DELETE FROM users WHERE id=?`, targetID)
	if err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
