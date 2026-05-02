package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

const tokenTTL = 24 * time.Hour

type Handler struct {
	db     *db.DB
	secret []byte
}

func NewHandler(database *db.DB, secret []byte) *Handler {
	return &Handler{db: database, secret: secret}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	var id int64
	var hash, role string
	err := h.db.QueryRow(
		`SELECT id, password_hash, role FROM users WHERE username=?`, req.Username,
	).Scan(&id, &hash, &role)
	if err != nil || !CheckPassword(req.Password, hash) {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token, err := SignToken(id, role, h.secret, tokenTTL)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r)
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var username, email, role string
	var id int64
	err := h.db.QueryRow(
		`SELECT id, username, email, role FROM users WHERE id=?`, claims.UserID,
	).Scan(&id, &username, &email, &role)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": id, "username": username, "email": email, "role": role,
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// JWT is stateless — client discards token. Server-side revocation in Phase 2.
	w.WriteHeader(http.StatusNoContent)
}
