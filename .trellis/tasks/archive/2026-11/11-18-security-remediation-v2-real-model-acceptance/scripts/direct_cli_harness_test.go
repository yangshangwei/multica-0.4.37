//go:build agentintegration && !windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The existing fixture still owns real Claim, complete runtime preparation and
// production Report. A declared test-only backend seam selects official exec;
// no app-server RPC or client identity emulation is involved in this cohort.
func secRemAuthorizeDirectCLI(t *testing.T, mode string) {
	t.Helper()
	if mode == "real" && os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "1" {
		t.Skip("official CLI direct entry requires real-agent opt-in")
	}
	if mode == "fake" && os.Getenv("MULTICA_RUN_REAL_AGENT_SMOKE") != "" {
		t.Fatal("fake CLI entry refuses real opt-in")
	}
	raw, err := secRemPrivateFile(os.Getenv("SECREM_HARNESS_CONFIG"), os.Getenv("SECREM_HARNESS_CONFIG_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Mode          string `json:"mode"`
		ExecutionMode string `json:"execution_mode"`
		OutputDir     string `json:"output_dir"`
	}
	if json.Unmarshal(raw, &cfg) != nil || cfg.Mode != mode || cfg.ExecutionMode != "official-cli-direct" || !filepath.IsAbs(cfg.OutputDir) {
		t.Fatal("explicit official CLI direct registration required")
	}
	t.Setenv("SECREM_DIRECT_CLI", "1")
}

func TestSecurityRemediationDirectP1Real(t *testing.T) {
	secRemAuthorizeDirectCLI(t, "real")
	secRemRunP1(t, "real")
}
func TestSecurityRemediationDirectP1Fake(t *testing.T) {
	secRemAuthorizeDirectCLI(t, "fake")
	secRemRunP1(t, "fake")
}
