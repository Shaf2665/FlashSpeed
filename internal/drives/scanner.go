package drives

import (
	"bufio"
	"os"
	"strings"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

var skipFSTypes = map[string]bool{
	"sysfs": true, "proc": true, "tmpfs": true, "devtmpfs": true,
	"devpts": true, "cgroup": true, "cgroup2": true, "pstore": true,
	"mqueue": true, "hugetlbfs": true, "debugfs": true, "securityfs": true,
	"fusectl": true, "bpf": true, "overlay": true,
}

type Drive struct {
	Name      string
	MountPath string
	IsAuto    bool
}

func ParseMountsFile(path string) []Drive {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []Drive
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 3 {
			continue
		}
		device, mountPath, fsType := parts[0], parts[1], parts[2]
		if skipFSTypes[fsType] {
			continue
		}
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}
		results = append(results, Drive{
			Name:      mountPath,
			MountPath: mountPath,
			IsAuto:    true,
		})
	}
	return results
}

type Scanner struct {
	db          *db.DB
	manualPaths []string
}

func NewScanner(database *db.DB) *Scanner {
	return &Scanner{db: database}
}

func (s *Scanner) AddManual(path string) {
	s.manualPaths = append(s.manualPaths, path)
}

// Sync upserts drives into DB. Pass nil autoDetected to skip auto-detection.
func (s *Scanner) Sync(autoDetected []Drive) error {
	all := append([]Drive{}, autoDetected...)
	for _, p := range s.manualPaths {
		all = append(all, Drive{Name: p, MountPath: p, IsAuto: false})
	}

	for _, d := range all {
		isAuto := 0
		if d.IsAuto {
			isAuto = 1
		}
		_, err := s.db.Exec(`
			INSERT INTO drives(name, mount_path, is_auto_detected)
			VALUES(?,?,?)
			ON CONFLICT(mount_path) DO UPDATE SET name=excluded.name
		`, d.Name, d.MountPath, isAuto)
		if err != nil {
			return err
		}
	}
	return nil
}

func ScanSystem() []Drive {
	return ParseMountsFile("/proc/mounts")
}
