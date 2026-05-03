//go:build linux

package drives_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/drives"
)

func TestParseMounts(t *testing.T) {
	tmp := t.TempDir()
	mounts := `sysfs /sys sysfs rw 0 0
proc /proc proc rw 0 0
/dev/sda1 / ext4 rw 0 1
/dev/sdb1 /mnt/external ext4 rw 0 0
tmpfs /tmp tmpfs rw 0 0
`
	path := filepath.Join(tmp, "mounts")
	os.WriteFile(path, []byte(mounts), 0644)

	results := drives.ParseMountsFile(path)

	found := false
	for _, d := range results {
		if d.MountPath == "/mnt/external" {
			found = true
		}
		if d.MountPath == "/tmp" {
			t.Error("/tmp (tmpfs) should be excluded")
		}
		if d.MountPath == "/sys" {
			t.Error("/sys should be excluded")
		}
	}
	if !found {
		t.Error("expected /mnt/external in results")
	}
}

func TestSyncDrives(t *testing.T) {
	database, _ := db.Open(filepath.Join(t.TempDir(), "test.db"))
	defer database.Close()

	scanner := drives.NewScanner(database)
	scanner.AddManual("/mnt/custom")

	if err := scanner.Sync(nil); err != nil {
		t.Fatalf("sync: %v", err)
	}

	var count int
	database.QueryRow(`SELECT COUNT(*) FROM drives WHERE mount_path=?`, "/mnt/custom").Scan(&count)
	if count != 1 {
		t.Error("manual drive should be in DB")
	}
}
