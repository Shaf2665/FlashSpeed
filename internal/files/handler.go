package files

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

type Handler struct {
	db  *db.DB
	svc *Service
}

func NewHandler(database *db.DB, svc *Service) *Handler {
	return &Handler{db: database, svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	driveID, _ := strconv.ParseInt(r.URL.Query().Get("drive_id"), 10, 64)
	parentID, _ := strconv.ParseInt(r.URL.Query().Get("parent_id"), 10, 64)

	entries, err := h.svc.List(claims.UserID, driveID, parentID)
	if err != nil {
		http.Error(w, `{"error":"list failed"}`, http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *Handler) Mkdir(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var body struct {
		DriveID  int64  `json:"drive_id"`
		ParentID int64  `json:"parent_id"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	id, err := h.svc.Mkdir(claims.UserID, body.DriveID, body.ParentID, body.Name)
	if err != nil {
		http.Error(w, `{"error":"mkdir failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"bad id"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.Delete(claims.UserID, fileID); err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"bad id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.Rename(claims.UserID, fileID, body.Name); err != nil {
		http.Error(w, `{"error":"rename failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"bad id"}`, http.StatusBadRequest)
		return
	}

	// verify ownership
	var ownerID int64
	var name string
	if err := h.db.QueryRow(`SELECT user_id, name FROM files WHERE id=? AND deleted_at IS NULL`, fileID).
		Scan(&ownerID, &name); err != nil || ownerID != claims.UserID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	absPath, err := h.svc.AbsPath(fileID)
	if err != nil {
		http.Error(w, `{"error":"resolve path failed"}`, http.StatusInternalServerError)
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		http.Error(w, `{"error":"file open failed"}`, http.StatusInternalServerError)
		return
	}
	defer f.Close()

	info, _ := f.Stat()
	var modTime time.Time
	if info != nil {
		modTime = info.ModTime()
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	w.Header().Set("Content-Disposition", disposition)
	http.ServeContent(w, r, name, modTime, f)
}

func (h *Handler) TrashList(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	entries, err := h.svc.Trash(claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"trash list failed"}`, http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"bad id"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.Restore(claims.UserID, fileID); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PermanentDelete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"bad id"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.PermanentDelete(claims.UserID, fileID); err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) EmptyTrash(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if err := h.svc.EmptyTrash(claims.UserID); err != nil {
		http.Error(w, `{"error":"empty trash failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Search handles GET /api/files/search?q=<term>
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	q := r.URL.Query().Get("q")
	entries, err := h.svc.Search(claims.UserID, q)
	if err != nil {
		http.Error(w, `{"error":"search failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// BulkDelete handles DELETE /api/files with body {"ids": [1, 2, 3]}
func (h *Handler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
		http.Error(w, `{"error":"ids required"}`, http.StatusBadRequest)
		return
	}
	var failed []int64
	for _, id := range body.IDs {
		if err := h.svc.Delete(claims.UserID, id); err != nil {
			failed = append(failed, id)
		}
	}
	if len(failed) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMultiStatus)
		json.NewEncoder(w).Encode(map[string]interface{}{"failed_ids": failed})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ZipDownload handles POST /api/files/zip with body {"ids": [1, 2, 3]}
// Streams a ZIP archive of the requested files (non-directories, ownership verified).
func (h *Handler) ZipDownload(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
		http.Error(w, `{"error":"ids required"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="files.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, id := range body.IDs {
		var ownerID int64
		var name string
		var isDir int
		if err := h.db.QueryRow(
			`SELECT user_id, name, is_dir FROM files WHERE id=? AND deleted_at IS NULL`, id,
		).Scan(&ownerID, &name, &isDir); err != nil || ownerID != claims.UserID || isDir == 1 {
			continue // skip not-found, not-owned, or directories
		}

		absPath, err := h.svc.AbsPath(id)
		if err != nil {
			continue
		}

		f, err := os.Open(absPath)
		if err != nil {
			continue
		}

		fw, err := zw.Create(fmt.Sprintf("%d_%s", id, name))
		if err != nil {
			f.Close()
			continue
		}
		io.Copy(fw, f)
		f.Close()
	}
}
