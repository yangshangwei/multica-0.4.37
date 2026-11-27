//go:build materialintegration

package daemon

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Browser acceptance attaches a real daemon execution loop to an isolated API.
// Only test-created executable paths are accepted; no installed model CLI runs.
func TestMaterialBrowserDaemon(t *testing.T) {
	file := os.Getenv("MATERIAL_BROWSER_REGISTRATION")
	if file == "" {
		t.Skip("requires private browser registration")
	}
	var c struct{ API, Token, Runtime, Daemon, Executable, Root string }
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(c.API)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" {
		t.Fatal("loopback API required")
	}
	if !filepath.IsAbs(c.Executable) || !filepath.IsAbs(c.Root) || c.Runtime == "" || c.Daemon == "" || c.Token == "" {
		t.Fatal("incomplete registration")
	}
	d := New(Config{ServerBaseURL: c.API, WorkspacesRoot: filepath.Join(c.Root, "workspaces"), DaemonID: c.Daemon, AgentTimeout: time.Minute, Agents: map[string]AgentEntry{"claude": {Path: c.Executable}}}, slog.Default())
	d.executionEnvironmentCommand = nil
	d.client.SetToken(c.Token)
	d.runtimeIndex[c.Runtime] = Runtime{ID: c.Runtime, Provider: "claude"}
	local := httptest.NewServer(d.materialHandler())
	defer local.Close()
	localURL, _ := url.Parse(local.URL)
	d.cfg.HealthPort, _ = strconv.Atoi(localURL.Port())
	if err = os.WriteFile(filepath.Join(c.Root, "daemon-ready"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err = os.Stat(filepath.Join(c.Root, "stop")); err == nil {
			return
		}
		task, err := d.client.ClaimTask(t.Context(), c.Runtime)
		if err != nil {
			t.Fatal(err)
		}
		if task == nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		result, err := d.runTask(t.Context(), *task, "claude", 0, d.logger)
		if err != nil || result.Status != "completed" {
			t.Fatalf("runtime stage: status=%s error=%v", result.Status, err)
		}
		if err = d.client.CompleteTask(t.Context(), task.ID, result.Comment, result.BranchName, result.SessionID, result.WorkDir, false, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("browser daemon deadline expired")
}
