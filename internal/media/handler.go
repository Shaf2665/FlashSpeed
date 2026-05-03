package media

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/files"
)

// Handler handles media streaming requests.
type Handler struct {
	db  *db.DB
	svc *files.Service
}

// NewHandler creates a new media Handler.
func NewHandler(database *db.DB, svc *files.Service) *Handler {
	return &Handler{db: database, svc: svc}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// fileRecord holds the columns we need from the files table.
type fileRecord struct {
	id       int64
	name     string
	mimeType string
	isDir    bool
	modTime  time.Time
}

// Stream handles GET /api/files/:id/stream
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	// Parse :id from URL
	idStr := chi.URLParam(r, "id")
	fileID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid file id"})
		return
	}

	// Get claims from context
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{"unauthorized"})
		return
	}

	// Verify file ownership: user_id must match and file must not be deleted
	var rec fileRecord
	var isDir int
	var mimeType sql.NullString
	err = h.db.QueryRow(`
		SELECT id, name, COALESCE(mime_type,''), is_dir, updated_at
		FROM files
		WHERE id=? AND user_id=? AND deleted_at IS NULL
	`, fileID, claims.UserID).Scan(&rec.id, &rec.name, &mimeType, &isDir, &rec.modTime)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, errorResponse{"file not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"db error"})
		return
	}
	rec.isDir = isDir == 1
	rec.mimeType = mimeType.String

	// Directories cannot be streamed
	if rec.isDir {
		writeJSON(w, http.StatusBadRequest, errorResponse{"cannot stream a directory"})
		return
	}

	// Resolve absolute path
	absPath, err := h.svc.AbsPath(fileID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"could not resolve file path"})
		return
	}

	// Open the file
	f, err := os.Open(absPath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse{"file not found on disk"})
		return
	}
	defer f.Close()

	// Detect MIME type
	mimeStr := rec.mimeType
	if mimeStr == "" {
		// Sniff content type from first 512 bytes
		buf := make([]byte, 512)
		n, _ := io.ReadFull(f, buf)
		mimeStr = http.DetectContentType(buf[:n])
		// Seek back to beginning for ServeContent
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{"seek error"})
			return
		}
	}

	// Set Content-Type before ServeContent so it won't re-sniff and override
	w.Header().Set("Content-Type", mimeStr)

	// Get file stat for modtime
	stat, err := f.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{"stat error"})
		return
	}

	// http.ServeContent handles Range requests, 206 Partial Content, ETag, Last-Modified automatically
	http.ServeContent(w, r, rec.name, stat.ModTime(), f)
}
