package tus

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

const tusVersion = "1.0.0"

type Handler struct {
	db      *db.DB
	tempDir string
}

func NewHandler(database *db.DB, tempDir string) *Handler {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		panic("tus: failed to create temp dir: " + err.Error())
	}
	return &Handler{db: database, tempDir: tempDir}
}

// Create handles POST /api/tus/ — initiates a new upload
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	uploadLength, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil || uploadLength < 0 {
		w.Header().Set("Tus-Resumable", tusVersion)
		http.Error(w, "invalid Upload-Length", http.StatusBadRequest)
		return
	}

	meta := parseMetadata(r.Header.Get("Upload-Metadata"))
	driveID, _ := strconv.ParseInt(meta["drive_id"], 10, 64)
	if driveID == 0 {
		w.Header().Set("Tus-Resumable", tusVersion)
		http.Error(w, "drive_id required in Upload-Metadata", http.StatusBadRequest)
		return
	}
	filename := meta["filename"]
	if filename == "" {
		filename = "upload"
	}
	filename = filepath.Base(filename) // strip any path components — defense in depth

	id := uuid.New().String()
	tempPath := filepath.Join(h.tempDir, id+".tmp")

	f, err := os.Create(tempPath)
	if err != nil {
		http.Error(w, "create temp file failed", http.StatusInternalServerError)
		return
	}
	f.Close()

	metaJSON, _ := json.Marshal(meta)
	_, err = h.db.Exec(`
		INSERT INTO tus_uploads(id, user_id, drive_id, dest_path, upload_length, temp_path, metadata)
		VALUES(?,?,?,?,?,?,?)
	`, id, claims.UserID, driveID, filename, uploadLength, tempPath, string(metaJSON))
	if err != nil {
		os.Remove(tempPath)
		http.Error(w, "db insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", "/api/tus/"+id)
	w.Header().Set("Tus-Resumable", tusVersion)
	w.WriteHeader(http.StatusCreated)
}

// Head handles HEAD /api/tus/:id — returns current upload offset
func (h *Handler) Head(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var offset, length int64
	var storedUserID int64
	err := h.db.QueryRow(`SELECT upload_offset, upload_length, user_id FROM tus_uploads WHERE id=?`, id).
		Scan(&offset, &length, &storedUserID)
	if err != nil {
		w.Header().Set("Tus-Resumable", tusVersion)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	claims := auth.ClaimsFromCtx(r)
	if claims == nil || storedUserID != claims.UserID {
		w.Header().Set("Tus-Resumable", tusVersion)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
	w.Header().Set("Upload-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Tus-Resumable", tusVersion)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

// Upload handles PATCH /api/tus/:id — appends bytes
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var uploadOffset, uploadLength int64
	var tempPath, destPath string
	var userID, driveID int64
	var metaJSON string

	err := h.db.QueryRow(`
		SELECT upload_offset, upload_length, temp_path, dest_path, user_id, drive_id, metadata
		FROM tus_uploads WHERE id=?
	`, id).Scan(&uploadOffset, &uploadLength, &tempPath, &destPath, &userID, &driveID, &metaJSON)
	if err != nil {
		w.Header().Set("Tus-Resumable", tusVersion)
		http.Error(w, "upload not found", http.StatusNotFound)
		return
	}

	claims := auth.ClaimsFromCtx(r)
	if claims == nil || userID != claims.UserID {
		w.Header().Set("Tus-Resumable", tusVersion)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	offset, err := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if err != nil || offset != uploadOffset {
		http.Error(w, "incorrect offset", http.StatusConflict)
		return
	}

	f, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		http.Error(w, "open temp file failed", http.StatusInternalServerError)
		return
	}
	written, err := io.Copy(f, io.LimitReader(r.Body, uploadLength-uploadOffset))
	f.Close()
	if err != nil {
		http.Error(w, "write failed", http.StatusInternalServerError)
		return
	}

	newOffset := uploadOffset + written
	if newOffset > uploadLength {
		http.Error(w, "upload exceeds declared length", http.StatusRequestEntityTooLarge)
		return
	}
	if _, err := h.db.Exec(`UPDATE tus_uploads SET upload_offset=? WHERE id=?`, newOffset, id); err != nil {
		http.Error(w, "offset update failed", http.StatusInternalServerError)
		return
	}

	if newOffset >= uploadLength {
		if err := h.finalize(id, userID, driveID, tempPath, destPath, uploadLength); err != nil {
			http.Error(w, "finalize failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	w.Header().Set("Tus-Resumable", tusVersion)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) finalize(uploadID string, userID, driveID int64, tempPath, destPath string, size int64) error {
	var mountPath string
	if err := h.db.QueryRow(`SELECT mount_path FROM drives WHERE id=?`, driveID).Scan(&mountPath); err != nil {
		return fmt.Errorf("drive not found: %w", err)
	}

	finalPath := filepath.Join(mountPath, destPath)
	// Ensure path stays within drive root — prevent path traversal attacks
	mountClean := filepath.Clean(mountPath) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(finalPath)+string(os.PathSeparator), mountClean) {
		return fmt.Errorf("dest_path escapes drive root")
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return err
	}

	mime := detectMIME(tempPath)

	tx, err := h.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var fileID int64
	res, err := tx.Exec(`
		INSERT INTO files(user_id, drive_id, name, rel_path, size_bytes, mime_type, is_dir)
		VALUES(?,?,?,?,?,?,0)
	`, userID, driveID, filepath.Base(destPath), destPath, size, mime)
	if err != nil {
		return err
	}
	fileID, _ = res.LastInsertId()

	if _, err := tx.Exec(`DELETE FROM tus_uploads WHERE id=?`, uploadID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// FS move happens after DB commit — if rename fails, clean up the DB row
	if err := os.Rename(tempPath, finalPath); err != nil {
		h.db.Exec(`DELETE FROM files WHERE id=?`, fileID)
		return fmt.Errorf("rename failed: %w", err)
	}

	return nil
}

func parseMetadata(header string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(header, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, " ", 2)
		if len(parts) == 2 {
			decoded, err := base64.StdEncoding.DecodeString(parts[1])
			if err == nil {
				result[parts[0]] = string(decoded)
			}
		} else if len(parts) == 1 && parts[0] != "" {
			result[parts[0]] = ""
		}
	}
	return result
}

func detectMIME(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return http.DetectContentType(buf[:n])
}
