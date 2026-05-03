package drives

import (
	"encoding/json"
	"net/http"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

type Handler struct {
	db      *db.DB
	scanner *Scanner
}

func NewHandler(database *db.DB, scanner *Scanner) *Handler {
	return &Handler{db: database, scanner: scanner}
}

type driveRow struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	IsAuto    bool   `json:"is_auto_detected"`
	Enabled   bool   `json:"enabled"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, name, mount_path, is_auto_detected, enabled FROM drives`)
	if err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var result []driveRow
	for rows.Next() {
		var d driveRow
		var isAuto, enabled int
		rows.Scan(&d.ID, &d.Name, &d.MountPath, &isAuto, &enabled)
		d.IsAuto = isAuto == 1
		d.Enabled = enabled == 1
		result = append(result, d)
	}
	if result == nil {
		result = []driveRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	detected := ScanSystem()
	if err := h.scanner.Sync(detected); err != nil {
		http.Error(w, `{"error":"scan failed"}`, http.StatusInternalServerError)
		return
	}
	h.List(w, r)
}
