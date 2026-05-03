package shares

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

// Handler exposes HTTP endpoints for the shares package.
type Handler struct {
	svc       *Service
	jwtSecret []byte
}

// NewHandler constructs a Handler wired to the given DB and JWT secret.
func NewHandler(database *db.DB, jwtSecret []byte) *Handler {
	return &Handler{
		svc:       NewService(database),
		jwtSecret: jwtSecret,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// List handles GET /api/shares — returns all non-expired shares for the caller.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	shareList, err := h.svc.List(claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if shareList == nil {
		shareList = []Share{}
	}
	writeJSON(w, http.StatusOK, shareList)
}

// Create handles POST /api/shares — creates a new share and returns 201.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MaxDownloads != nil && *req.MaxDownloads <= 0 {
		http.Error(w, `{"error":"max_downloads must be positive"}`, http.StatusBadRequest)
		return
	}

	share, err := h.svc.Create(claims.UserID, req)
	if err != nil {
		if errors.Is(err, ErrNotOwned) {
			writeError(w, http.StatusForbidden, "file not found or not owned by user")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, share)
}

// Delete handles DELETE /api/shares/{id} — removes a share; 403 if not owner.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	shareID := chi.URLParam(r, "id")
	if err := h.svc.Delete(claims.UserID, shareID); err != nil {
		if errors.Is(err, ErrNotOwned) {
			writeError(w, http.StatusForbidden, "share not found or not owned by you")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Resolve handles GET /api/s/{token} — public endpoint, no auth required.
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	password := r.Header.Get("X-Share-Password")

	var callerID *int64
	if hdr := r.Header.Get("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
		if claims, err := auth.VerifyToken(strings.TrimPrefix(hdr, "Bearer "), h.jwtSecret); err == nil {
			callerID = &claims.UserID
		}
	}

	share, file, err := h.svc.Resolve(token, password, callerID)
	if err != nil {
		if errors.Is(err, ErrNotAuthorized) {
			writeError(w, http.StatusForbidden, "not authorized")
			return
		}
		if errors.Is(err, ErrWrongPassword) {
			writeError(w, http.StatusUnauthorized, "wrong password")
			return
		}
		if errors.Is(err, ErrShareExpired) {
			writeError(w, http.StatusGone, "share expired")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "share not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type fileResp struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		MimeType string `json:"mime_type"`
		IsDir    bool   `json:"is_dir"`
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"share": share,
		"file": fileResp{
			ID:       file.ID,
			Name:     file.Name,
			MimeType: file.MimeType,
			IsDir:    file.IsDir,
		},
	})
}
