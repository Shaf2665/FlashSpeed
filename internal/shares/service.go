package shares

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

var (
	ErrWrongPassword = errors.New("wrong password")
	ErrShareExpired  = errors.New("share expired or exhausted")
	ErrNotAuthorized = errors.New("not authorized to access this share")
	ErrNotOwned      = errors.New("file not found or not owned by user")
)

// Share represents a file share record.
type Share struct {
	ID            string     `json:"id"`
	FileID        int64      `json:"file_id"`
	OwnerID       int64      `json:"owner_id"`
	TargetUserID  *int64     `json:"target_user_id"`
	ExpiresAt     *time.Time `json:"expires_at"`
	DownloadCount int        `json:"download_count"`
	MaxDownloads  *int       `json:"max_downloads"`
	CreatedAt     time.Time  `json:"created_at"`
}

// FileRow holds the file metadata returned alongside a resolved share.
type FileRow struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	MimeType   string `json:"mime_type"`
	IsDir      bool   `json:"is_dir"`
	SizeBytes  int64  `json:"size_bytes"`
	RelPath    string `json:"-"`
	DriveMount string `json:"-"`
}

// CreateShareRequest is the body expected by the Create endpoint.
type CreateShareRequest struct {
	FileID       int64      `json:"file_id"`
	TargetUserID *int64     `json:"target_user_id"`
	Password     string     `json:"password"`
	ExpiresAt    *time.Time `json:"expires_at"`
	MaxDownloads *int       `json:"max_downloads"`
}

// Service holds the DB dependency for share operations.
type Service struct {
	db *db.DB
}

// NewService constructs a Service.
func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

// Create generates a new share token for ownerID.
func (s *Service) Create(ownerID int64, req CreateShareRequest) (*Share, error) {
	var fileOwner int64
	err := s.db.QueryRow(`SELECT user_id FROM files WHERE id=? AND deleted_at IS NULL`, req.FileID).Scan(&fileOwner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotOwned
		}
		return nil, err
	}
	if fileOwner != ownerID {
		return nil, ErrNotOwned
	}

	id := uuid.New().String()

	var passwordHash *string
	if req.Password != "" {
		h, err := auth.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		passwordHash = &h
	}

	now := time.Now().UTC()
	_, err = s.db.Exec(`
		INSERT INTO shares (id, file_id, owner_id, target_user_id, password_hash, expires_at, max_downloads, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, req.FileID, ownerID, req.TargetUserID, passwordHash, req.ExpiresAt, req.MaxDownloads, now)
	if err != nil {
		return nil, err
	}

	return &Share{
		ID:            id,
		FileID:        req.FileID,
		OwnerID:       ownerID,
		TargetUserID:  req.TargetUserID,
		ExpiresAt:     req.ExpiresAt,
		DownloadCount: 0,
		MaxDownloads:  req.MaxDownloads,
		CreatedAt:     now,
	}, nil
}

// List returns all non-expired shares owned by ownerID.
func (s *Service) List(ownerID int64) ([]Share, error) {
	rows, err := s.db.Query(`
		SELECT id, file_id, owner_id, target_user_id, expires_at, download_count, max_downloads, created_at
		FROM shares
		WHERE owner_id = ?
		  AND (expires_at IS NULL OR datetime(expires_at) > datetime('now'))
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Share
	for rows.Next() {
		var sh Share
		var targetUserID sql.NullInt64
		var expiresAt sql.NullTime
		var maxDownloads sql.NullInt64
		if err := rows.Scan(
			&sh.ID, &sh.FileID, &sh.OwnerID, &targetUserID,
			&expiresAt, &sh.DownloadCount, &maxDownloads, &sh.CreatedAt,
		); err != nil {
			return nil, err
		}
		if targetUserID.Valid {
			v := targetUserID.Int64
			sh.TargetUserID = &v
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			sh.ExpiresAt = &t
		}
		if maxDownloads.Valid {
			v := int(maxDownloads.Int64)
			sh.MaxDownloads = &v
		}
		result = append(result, sh)
	}
	return result, rows.Err()
}

// Delete removes a share, enforcing ownership.
func (s *Service) Delete(ownerID int64, shareID string) error {
	res, err := s.db.Exec(`DELETE FROM shares WHERE id = ? AND owner_id = ?`, shareID, ownerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotOwned
	}
	return nil
}

// Resolve looks up a share by token, validates it, increments the download
// counter and returns the share together with the associated file row.
// callerID is optional; if the share has a target_user_id set, it must match.
func (s *Service) Resolve(token string, password string, callerID *int64) (*Share, *FileRow, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	// Fetch share row (including password_hash for check).
	var sh Share
	var passwordHash sql.NullString
	var targetUserID sql.NullInt64
	var expiresAt sql.NullTime
	var maxDownloads sql.NullInt64

	err = tx.QueryRow(`
		SELECT id, file_id, owner_id, target_user_id, password_hash,
		       expires_at, download_count, max_downloads, created_at
		FROM shares WHERE id = ?
	`, token).Scan(
		&sh.ID, &sh.FileID, &sh.OwnerID, &targetUserID, &passwordHash,
		&expiresAt, &sh.DownloadCount, &maxDownloads, &sh.CreatedAt,
	)
	if err != nil {
		return nil, nil, err // includes sql.ErrNoRows
	}

	if targetUserID.Valid {
		v := targetUserID.Int64
		sh.TargetUserID = &v
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		sh.ExpiresAt = &t
	}
	if maxDownloads.Valid {
		v := int(maxDownloads.Int64)
		sh.MaxDownloads = &v
	}

	// Enforce target_user_id restriction.
	if sh.TargetUserID != nil {
		if callerID == nil || *callerID != *sh.TargetUserID {
			return nil, nil, ErrNotAuthorized
		}
	}

	// Check expiry.
	if sh.ExpiresAt != nil && sh.ExpiresAt.Before(time.Now()) {
		return nil, nil, ErrShareExpired
	}

	// Check password (validate before incrementing to avoid burning downloads).
	if passwordHash.Valid && passwordHash.String != "" {
		if !auth.CheckPassword(password, passwordHash.String) {
			return nil, nil, ErrWrongPassword
		}
	}

	// Atomically increment download_count, enforcing max_downloads in one query.
	res, err := tx.Exec(
		`UPDATE shares SET download_count = download_count + 1
		 WHERE id = ? AND (max_downloads IS NULL OR download_count < max_downloads)`,
		token,
	)
	if err != nil {
		return nil, nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, nil, ErrShareExpired
	}
	sh.DownloadCount++

	// Fetch file metadata.
	var fr FileRow
	var isDir int
	err = tx.QueryRow(`
		SELECT f.id, f.name, COALESCE(f.mime_type, ''), f.is_dir, f.size_bytes, f.rel_path, d.mount_path
		FROM files f
		JOIN drives d ON d.id = f.drive_id
		WHERE f.id = ?
	`, sh.FileID).Scan(&fr.ID, &fr.Name, &fr.MimeType, &isDir, &fr.SizeBytes, &fr.RelPath, &fr.DriveMount)
	if err != nil {
		return nil, nil, err
	}
	fr.IsDir = isDir == 1

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	return &sh, &fr, nil
}
