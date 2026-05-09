package admin

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// TailscaleStatus holds the current state of the Tailscale daemon.
type TailscaleStatus struct {
	Running bool   `json:"running"`
	IP      string `json:"ip,omitempty"`
	Version string `json:"version,omitempty"`
}

// tailscaleStatusJSON is the subset of `tailscale status --json` we care about.
type tailscaleStatusJSON struct {
	BackendState string   `json:"BackendState"`
	TailscaleIPs []string `json:"TailscaleIPs"`
}

// TailscaleStatusCheck returns the current Tailscale status.
// If tailscale is not installed, it returns {Running: false} with no error.
func TailscaleStatusCheck() (TailscaleStatus, error) {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		// binary not found or daemon not running — treat as not installed
		return TailscaleStatus{Running: false}, nil
	}

	var raw tailscaleStatusJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return TailscaleStatus{Running: false}, nil
	}

	status := TailscaleStatus{
		Running: raw.BackendState == "Running",
	}
	if len(raw.TailscaleIPs) > 0 {
		status.IP = raw.TailscaleIPs[0]
	}

	// Best-effort version query
	if vOut, err := exec.Command("tailscale", "version", "--short").Output(); err == nil {
		status.Version = strings.TrimSpace(string(vOut))
	}

	return status, nil
}

// TailscaleInstall installs Tailscale via the official install script.
// Only works on Linux — correct for production deployments.
func TailscaleInstall() error {
	cmd := exec.Command("sh", "-c", "curl -fsSL https://tailscale.com/install.sh | sh")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tailscale install failed: %w\n%s", err, string(out))
	}
	return nil
}

// TailscaleUp authenticates and connects Tailscale with the given auth key.
func TailscaleUp(authKey string) error {
	cmd := exec.Command("tailscale", "up", "--authkey="+authKey)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tailscale up failed: %w\n%s", err, string(out))
	}
	return nil
}
