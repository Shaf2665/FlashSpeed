package db

import "fmt"

const schema = `
CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT    UNIQUE NOT NULL,
  email         TEXT    UNIQUE NOT NULL,
  password_hash TEXT    NOT NULL,
  role          TEXT    NOT NULL DEFAULT 'user',
  quota_bytes   INTEGER NOT NULL DEFAULT 0,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
  id          TEXT PRIMARY KEY,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT    NOT NULL,
  expires_at  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS drives (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  name             TEXT    NOT NULL,
  mount_path       TEXT    UNIQUE NOT NULL,
  is_auto_detected INTEGER NOT NULL DEFAULT 0,
  enabled          INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS files (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
  drive_id    INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
  name        TEXT    NOT NULL,
  rel_path    TEXT    NOT NULL,
  size_bytes  INTEGER NOT NULL DEFAULT 0,
  mime_type   TEXT,
  is_dir      INTEGER NOT NULL DEFAULT 0,
  parent_id   INTEGER REFERENCES files(id) ON DELETE SET NULL,
  deleted_at  DATETIME,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS shares (
  id             TEXT    PRIMARY KEY,
  file_id        INTEGER NOT NULL REFERENCES files(id)  ON DELETE CASCADE,
  owner_id       INTEGER NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
  target_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
  password_hash  TEXT,
  expires_at     DATETIME,
  download_count INTEGER NOT NULL DEFAULT 0,
  max_downloads  INTEGER,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tus_uploads (
  id            TEXT    PRIMARY KEY,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  drive_id      INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
  dest_path     TEXT    NOT NULL,
  upload_length INTEGER NOT NULL,
  upload_offset INTEGER NOT NULL DEFAULT 0,
  temp_path     TEXT    NOT NULL,
  metadata      TEXT,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

const currentVersion = 2

func (db *DB) migrate() error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}

	var version int
	_ = db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version)

	if version < 1 {
		if _, err := db.Exec(schema); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		version = 1
	}

	if version < 2 {
		if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_shares_owner ON shares(owner_id)`); err != nil {
			return fmt.Errorf("migration v2: %w", err)
		}
		version = 2
	}

	_, err := db.Exec(`INSERT OR REPLACE INTO schema_version(version) VALUES(?)`, version)
	return err
}
