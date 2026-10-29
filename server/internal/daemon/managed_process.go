package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

type managementProcessInfo struct{ hostID, bootID string }

var managementProcessValue atomic.Pointer[managementProcessInfo]
var managementProcessGroup singleflight.Group

// Endpoint security hooks injected into powershell.exe can append their own
// lines to stdout, so the identity is read only from the marked line.
const windowsIdentityMarker = "MULTICA_IDENTITY:"

// apps/desktop/src/main/managed-process.ts runs the same command; both sides must parse the same output.
const windowsIdentityCommand = `$o=Get-CimInstance Win32_OperatingSystem;$h=(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Cryptography').MachineGuid;'` + windowsIdentityMarker + `'+(@{host=$h;boot=$o.LastBootUpTime.ToUniversalTime().ToString('o')}|ConvertTo-Json -Compress)`

func processIdentityHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
func processIdentityCommand(path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, path, args...).Output()
	if err != nil {
		return "", errors.New("OS boot identity unavailable")
	}
	return strings.TrimSpace(string(raw)), nil
}

// Only a success is cached: one slow PowerShell start must not disable managed
// locking for the rest of the process. Concurrent callers share one attempt.
func managementProcessIdentity() (managementProcessInfo, error) {
	if value := managementProcessValue.Load(); value != nil {
		return *value, nil
	}
	value, err, _ := managementProcessGroup.Do("identity", func() (any, error) {
		value, err := readManagementProcessIdentity()
		if err == nil {
			managementProcessValue.Store(&value)
		}
		return value, err
	})
	if err != nil {
		return managementProcessInfo{}, err
	}
	return value.(managementProcessInfo), nil
}

// parseWindowsProcessIdentity's errors never echo stdout.
func parseWindowsProcessIdentity(raw string) (managementProcessInfo, error) {
	var marked []string
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, windowsIdentityMarker) {
			marked = append(marked, strings.TrimPrefix(line, windowsIdentityMarker))
		}
	}
	var value struct {
		Host string `json:"host"`
		Boot string `json:"boot"`
	}
	if len(marked) != 1 || json.Unmarshal([]byte(marked[0]), &value) != nil || value.Host == "" || value.Boot == "" {
		return managementProcessInfo{}, errors.New("OS boot identity unavailable")
	}
	return managementProcessInfo{processIdentityHash("windows-host:" + strings.ToLower(value.Host)), processIdentityHash("windows-boot:" + value.Boot)}, nil
}
func readManagementProcessIdentity() (managementProcessInfo, error) {
	switch runtime.GOOS {
	case "darwin":
		platform, err := processIdentityCommand("/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
		if err != nil {
			return managementProcessInfo{}, err
		}
		boot, err := processIdentityCommand("/usr/sbin/sysctl", "-n", "kern.bootsessionuuid")
		if err != nil {
			return managementProcessInfo{}, err
		}
		host := regexp.MustCompile(`(?i)"IOPlatformUUID"\s*=\s*"([0-9a-f-]+)"`).FindStringSubmatch(platform)
		if len(host) != 2 || !managedUUID.MatchString(strings.ToLower(boot)) {
			return managementProcessInfo{}, errors.New("OS boot identity unavailable")
		}
		return managementProcessInfo{processIdentityHash("darwin-host:" + strings.ToLower(host[1])), processIdentityHash("darwin-boot:" + strings.ToLower(boot))}, nil
	case "linux":
		host, err := os.ReadFile("/etc/machine-id")
		if err != nil {
			return managementProcessInfo{}, err
		}
		boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
		if err != nil {
			return managementProcessInfo{}, err
		}
		namespace, err := os.Readlink("/proc/self/ns/pid")
		if err != nil {
			return managementProcessInfo{}, err
		}
		return managementProcessInfo{processIdentityHash("linux-host:" + strings.ToLower(strings.TrimSpace(string(host))) + "\x00" + namespace), processIdentityHash("linux-boot:" + strings.ToLower(strings.TrimSpace(string(boot))))}, nil
	case "windows":
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		raw, err := processIdentityCommand(filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", windowsIdentityCommand)
		if err != nil {
			return managementProcessInfo{}, err
		}
		return parseWindowsProcessIdentity(raw)
	default:
		return managementProcessInfo{}, errors.New("managed locking is unavailable on this OS")
	}
}

// ManagementProcessExited is conservative: permission errors and invalid PIDs
// never authorize a managed replacement or a successful stop report.
func ManagementProcessExited(pid int) bool {
	return pid > 0 && uint64(pid) <= 4294967295 && managementProcessDefinitelyDead(pid)
}
