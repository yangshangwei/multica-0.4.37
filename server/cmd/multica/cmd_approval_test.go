package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/spf13/cobra"
)

func newApprovalRequestTestCmd() *cobra.Command {
	c := &cobra.Command{Use: "request"}
	c.Flags().String("risk-class", "", "")
	c.Flags().String("summary", "", "")
	c.Flags().String("plan", "", "")
	c.Flags().String("plan-file", "", "")
	c.Flags().String("issue", "", "")
	c.Flags().String("agent-id", "", "")
	c.Flags().String("output", "table", "")
	return c
}

// utf16LEWithBOM builds the byte shape PowerShell 5.1's `Out-File` writes, which
// is what an agent produces on Windows unless it names an encoding explicitly.
func utf16LEWithBOM(s string) []byte {
	out := []byte{0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

// TestRunApprovalRequestDecodesUTF16PlanFile covers the wiring only; the
// encoding matrix itself lives in internal/util/text_encoding_test.go.
//
// A plan is the artifact a human reads before authorising something
// irreversible. Before this decode existed, a UTF-16 plan file reached the API
// as NUL-riddled bytes, so the reviewer either saw mojibake at the moment of
// decision or the INSERT failed on them.
func TestRunApprovalRequestDecodesUTF16PlanFile(t *testing.T) {
	const plan = "步骤 1: 停止写入\n步骤 2: 回滚到上一版本"

	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent-approvals" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":         "apr-1",
			"risk_class": "database_migration",
			"status":     "pending",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("plan.md", utf16LEWithBOM(plan), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")

	cmd := newApprovalRequestTestCmd()
	_ = cmd.Flags().Set("risk-class", "database_migration")
	_ = cmd.Flags().Set("summary", "Roll back the write path")
	_ = cmd.Flags().Set("plan-file", "plan.md")
	_ = cmd.Flags().Set("output", "json")

	if err := runApprovalRequest(cmd, nil); err != nil {
		t.Fatalf("runApprovalRequest: %v", err)
	}
	if got := body["plan"]; got != plan {
		t.Errorf("plan = %#v, want the decoded UTF-16LE body %q", got, plan)
	}
}

func TestRunApprovalRequestRefusesBOMlessANSIPlanFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request reached the API; a plan that cannot be decoded must fail before it is filed")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Chdir(dir)
	// `Set-Content` on a CP936 machine: no BOM, so the code page would have to
	// be guessed, and a wrong guess is silent mojibake in the artifact a human
	// approves from.
	if err := os.WriteFile("plan.md", []byte{0xD6, 0xD0, 0xCE, 0xC4}, 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	t.Setenv("MULTICA_SERVER_URL", srv.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", "ws-1")
	t.Setenv("MULTICA_TOKEN", "test-token")

	cmd := newApprovalRequestTestCmd()
	_ = cmd.Flags().Set("risk-class", "database_migration")
	_ = cmd.Flags().Set("summary", "Roll back the write path")
	_ = cmd.Flags().Set("plan-file", "plan.md")

	err := runApprovalRequest(cmd, nil)
	if err == nil {
		t.Fatal("expected an undecodable plan file to be refused")
	}
	for _, want := range []string{"--plan-file", "no BOM to decode from", "UTF8Encoding"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
