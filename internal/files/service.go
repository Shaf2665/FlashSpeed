package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

type Entry struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsDir     bool      `json:"is_dir"`
	SizeBytes int64     `json:"size_bytes"`
	MimeType  string    `json:"mime_type"`
	ParentID  *int64    `json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Service struct {
	db *db.DB
}

func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

func (s *Service) drivePath(driveID int64) (string, error) {
	var p string
	err := s.db.QueryRow(`SELECT mount_path FROM drives WHERE id=?`, driveID).Scan(&p)
	return p, err
}

func (s *Service) Mkdir(userID, driveID, parentID int64, name string) (int64, error) {
	// Blocker 1: reject names that contain path separators or are dot-paths
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return 0, fmt.Errorf("invalid directory name")
	}

	mountPath, err := s.drivePath(driveID)
	if err != nil {
		return 0, fmt.Errorf("drive not found: %w", err)
	}

	relPath := name
	if parentID != 0 {
		// Blocker 2: verify parent_id belongs to the same user and drive
		var parentRel string
		err := s.db.QueryRow(`SELECT rel_path FROM files WHERE id=? AND user_id=? AND drive_id=? AND deleted_at IS NULL`,
			parentID, userID, driveID).Scan(&parentRel)
		if err != nil {
			return 0, fmt.Errorf("parent directory not found or not owned by user")
		}
		relPath = filepath.Join(parentRel, name)
	}

	absPath := filepath.Join(mountPath, relPath)

	// Blocker 1: containment check — ensure absPath doesn't escape mountPath
	mountClean := filepath.Clean(mountPath) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(absPath)+string(os.PathSeparator), mountClean) {
		return 0, fmt.Errorf("name escapes drive root")
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return 0, fmt.Errorf("mkdir on disk: %w", err)
	}

	var pID interface{} = nil
	if parentID != 0 {
		pID = parentID
	}
	res, err := s.db.Exec(`
		INSERT INTO files(user_id, drive_id, name, rel_path, is_dir, parent_id)
		VALUES(?,?,?,?,1,?)
	`, userID, driveID, name, relPath, pID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Service) List(userID, driveID, parentID int64) ([]Entry, error) {
	var query string
	var args []interface{}

	if parentID == 0 {
		query = `SELECT id,name,is_dir,size_bytes,COALESCE(mime_type,''),parent_id,created_at,updated_at
		         FROM files WHERE user_id=? AND drive_id=? AND parent_id IS NULL AND deleted_at IS NULL`
		args = []interface{}{userID, driveID}
	} else {
		query = `SELECT id,name,is_dir,size_bytes,COALESCE(mime_type,''),parent_id,created_at,updated_at
		         FROM files WHERE user_id=? AND drive_id=? AND parent_id=? AND deleted_at IS NULL`
		args = []interface{}{userID, driveID, parentID}
	}

	dbRows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	var entries []Entry
	for dbRows.Next() {
		var e Entry
		var isDir int
		var pID *int64
		dbRows.Scan(&e.ID, &e.Name, &isDir, &e.SizeBytes, &e.MimeType, &pID, &e.CreatedAt, &e.UpdatedAt)
		e.IsDir = isDir == 1
		e.ParentID = pID
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *Service) Delete(userID, fileID int64) error {
	res, err := s.db.Exec(`
		UPDATE files SET deleted_at=CURRENT_TIMESTAMP
		WHERE id=? AND user_id=? AND deleted_at IS NULL
	`, fileID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("file not found or already deleted")
	}
	return nil
}

func (s *Service) Trash(userID int64) ([]Entry, error) {
	rows, err := s.db.Query(`
		SELECT id,name,is_dir,size_bytes,COALESCE(mime_type,''),parent_id,created_at,updated_at
		FROM files WHERE user_id=? AND deleted_at IS NOT NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var isDir int
		var pID *int64
		rows.Scan(&e.ID, &e.Name, &isDir, &e.SizeBytes, &e.MimeType, &pID, &e.CreatedAt, &e.UpdatedAt)
		e.IsDir = isDir == 1
		e.ParentID = pID
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *Service) Rename(userID, fileID int64, newName string) error {
	// Validate name
	if newName == "" || newName == "." || newName == ".." ||
		strings.ContainsAny(newName, "/\\") {
		return fmt.Errorf("invalid name")
	}

	// Fetch current record
	var oldRelPath, mountPath string
	var driveID int64
	var isDir int
	err := s.db.QueryRow(`
		SELECT f.rel_path, f.drive_id, f.is_dir, d.mount_path
		FROM files f JOIN drives d ON d.id=f.drive_id
		WHERE f.id=? AND f.user_id=? AND f.deleted_at IS NULL
	`, fileID, userID).Scan(&oldRelPath, &driveID, &isDir, &mountPath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	newRelPath := filepath.Join(filepath.Dir(oldRelPath), newName)
	oldAbs := filepath.Join(mountPath, oldRelPath)
	newAbs := filepath.Join(mountPath, newRelPath)

	// Containment check
	mountClean := filepath.Clean(mountPath) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(newAbs)+string(os.PathSeparator), mountClean) {
		return fmt.Errorf("new name escapes drive root")
	}

	// Rename on disk first
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return fmt.Errorf("rename on disk failed: %w", err)
	}

	// Update DB in transaction
	tx, err := s.db.Begin()
	if err != nil {
		// Try to undo the FS rename
		os.Rename(newAbs, oldAbs)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE files SET name=?, rel_path=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND user_id=?
	`, newName, newRelPath, fileID, userID); err != nil {
		os.Rename(newAbs, oldAbs) // undo FS rename
		return err
	}

	// For directories: update descendants' rel_path
	if isDir == 1 {
		// Get all descendants
		rows, err := tx.Query(`
			SELECT id, rel_path FROM files
			WHERE drive_id=? AND user_id=? AND deleted_at IS NULL
			AND rel_path LIKE ?
		`, driveID, userID, oldRelPath+"/%")
		if err != nil {
			os.Rename(newAbs, oldAbs)
			return err
		}
		defer rows.Close()
		type update struct {
			id      int64
			newPath string
		}
		var updates []update
		for rows.Next() {
			var id int64
			var rp string
			rows.Scan(&id, &rp)
			updates = append(updates, update{id, newRelPath + rp[len(oldRelPath):]})
		}
		rows.Close()
		for _, u := range updates {
			if _, err := tx.Exec(`UPDATE files SET rel_path=? WHERE id=?`, u.newPath, u.id); err != nil {
				os.Rename(newAbs, oldAbs)
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *Service) AbsPath(fileID int64) (string, error) {
	var relPath string
	var mountPath string
	err := s.db.QueryRow(`
		SELECT f.rel_path, d.mount_path
		FROM files f JOIN drives d ON f.drive_id=d.id
		WHERE f.id=?
	`, fileID).Scan(&relPath, &mountPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(mountPath, relPath), nil
}
