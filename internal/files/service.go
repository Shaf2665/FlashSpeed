package files

import (
	"fmt"
	"os"
	"path/filepath"
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
	mountPath, err := s.drivePath(driveID)
	if err != nil {
		return 0, fmt.Errorf("drive not found: %w", err)
	}

	relPath := name
	if parentID != 0 {
		var parentRel string
		s.db.QueryRow(`SELECT rel_path FROM files WHERE id=?`, parentID).Scan(&parentRel)
		relPath = filepath.Join(parentRel, name)
	}

	absPath := filepath.Join(mountPath, relPath)
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
	_, err := s.db.Exec(`
		UPDATE files SET name=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND user_id=? AND deleted_at IS NULL
	`, newName, fileID, userID)
	return err
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
