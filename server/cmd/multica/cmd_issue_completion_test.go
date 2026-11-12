package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	completionCLIWorkspace = "11111111-1111-4111-8111-111111111111"
	completionCLIUser      = "22222222-2222-4222-8222-222222222222"
	completionCLIIssue     = "33333333-3333-4333-8333-333333333333"
	completionCLIProject   = "44444444-4444-4444-8444-444444444444"
	completionCLIApproval  = "55555555-5555-4555-8555-555555555555"
	completionCLIAuthority = "77777777-7777-4777-8777-777777777777"
)

func TestIssueCompletionCLICommandContract(t *testing.T) {
	completion := commandChild(issueCmd, "completion")
	if completion == nil {
		t.Fatal("issue completion command is missing")
	}
	for _, name := range []string{"prepare", "authorize", "submit", "show", "revoke"} {
		if commandChild(completion, name) == nil {
			t.Errorf("issue completion command is missing %q", name)
		}
	}
	assertFlags := func(command *cobra.Command, names ...string) {
		t.Helper()
		for _, name := range names {
			if command.Flags().Lookup(name) == nil {
				t.Errorf("%s missing --%s", command.CommandPath(), name)
			}
		}
	}
	assertFlags(commandChild(completion, "prepare"), "status", "approval", "operation-file", "limit", "cursor", "output")
	assertFlags(commandChild(completion, "authorize"), "operation-file", "accept", "reason", "expires-at", "output")
	assertFlags(commandChild(completion, "submit"), "operation-file", "output")
	assertFlags(commandChild(completion, "show"), "operation-file", "authorization-id", "request-key", "source-workspace", "output")
	assertFlags(commandChild(completion, "revoke"), "operation-file", "reason", "output")
	if commandChild(completion, "submit").Flags().Lookup("no-start") != nil {
		t.Fatal("completion submit must not inherit --no-start")
	}
	if issueStatusCmd.Flags().Lookup("no-start") == nil {
		t.Fatal("legacy issue status --no-start flag was removed")
	}
}

type completionCLIFake struct {
	t                    *testing.T
	server               *httptest.Server
	now                  time.Time
	validUntil           time.Time
	maxExpires           time.Time
	authPosts            int
	authByKeyReads       int
	authReads            int
	completionPuts       int
	revocations          int
	candidateReads       int
	previewReads         int
	requestKey           string
	operationID          string
	authorizationID      string
	authorizationExpiry  string
	state                string
	currentStatus        string
	authorizationMode    string
	completionMode       string
	revocationMode       string
	redirectTarget       string
	missingAuthorization bool
}

func newCompletionCLIFake(t *testing.T) *completionCLIFake {
	t.Helper()
	f := &completionCLIFake{t: t, now: time.Now().UTC().Truncate(time.Microsecond)}
	f.validUntil, f.maxExpires = f.now.Add(25*time.Minute), f.now.Add(20*time.Minute)
	f.currentStatus = "in_review"
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	t.Setenv("MULTICA_SERVER_URL", f.server.URL)
	t.Setenv("MULTICA_WORKSPACE_ID", completionCLIWorkspace)
	t.Setenv("MULTICA_TOKEN", "mul_fake-human")
	t.Setenv("MULTICA_AGENT_ID", "")
	t.Setenv("MULTICA_TASK_ID", "")
	t.Setenv("MULTICA_DAEMON_PORT", "")
	t.Setenv("MULTICA_TASK_CONFIG_ROOT", "")
	return f
}

func (f *completionCLIFake) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/me":
		_ = json.NewEncoder(w).Encode(map[string]string{"id": completionCLIUser})
	case r.Method == http.MethodGet && r.URL.Path == "/api/issues/"+completionCLIIssue:
		revision := int64(7)
		if f.currentStatus != "in_review" {
			revision = 9
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": completionCLIIssue, "identifier": "MUL-7", "revision": revision, "status": f.currentStatus})
	case r.Method == http.MethodGet && r.URL.Path == "/api/issues/"+completionCLIIssue+"/delivery-evidence/candidates":
		f.candidateReads++
		if r.URL.Query().Get("limit") == "" || r.Header.Get("X-Workspace-ID") != completionCLIWorkspace {
			f.t.Errorf("candidate query/header=%q/%q", r.URL.RawQuery, r.Header.Get("X-Workspace-ID"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"protocol_version": 1, "issue_id": completionCLIIssue, "issue_revision": 7, "project_id": completionCLIProject, "checked_at": completionCLIFormat(f.now), "items": []any{}, "next_cursor": nil, "has_more": false})
	case r.Method == http.MethodPost && r.URL.Path == "/api/issues/"+completionCLIIssue+"/delivery-evidence/completion-preview":
		f.previewReads++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["expected_issue_revision"] != float64(7) || body["target_status"] != "done" || body["approval_id"] != completionCLIApproval {
			f.t.Errorf("preview request=%#v", body)
		}
		_ = json.NewEncoder(w).Encode(f.preview())
	case r.Method == http.MethodPost && r.URL.Path == "/api/delivery-completion-authorizations":
		f.authPosts++
		f.requestKey = r.Header.Get("Idempotency-Key")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, exists := body["evaluation_key"]; exists {
			f.t.Error("private evaluation key was sent")
		}
		f.operationID, _ = body["operation_id"].(string)
		f.authorizationID = completionCLIAuthority
		f.authorizationExpiry, _ = body["expires_at"].(string)
		f.state = "issued"
		if f.authorizationMode == "unknown" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "create_unknown"})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(f.authorization(body["expires_at"], nil))
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/delivery-completion-authorizations/by-request-key/"):
		f.authByKeyReads++
		if strings.TrimPrefix(r.URL.Path, "/api/delivery-completion-authorizations/by-request-key/") != f.requestKey || r.URL.Query().Get("workspace_id") != completionCLIWorkspace {
			f.t.Errorf("by-key query=%s?%s", r.URL.Path, r.URL.RawQuery)
		}
		if f.missingAuthorization {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(f.authorization(f.authorizationExpiry, f.receiptIfConsumed()))
	case r.Method == http.MethodGet && r.URL.Path == "/api/delivery-completion-authorizations/"+f.authorizationID:
		f.authReads++
		if f.missingAuthorization {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(f.authorization(f.authorizationExpiry, f.receiptIfConsumed()))
	case r.Method == http.MethodPut && r.URL.Path == "/api/issues/"+completionCLIIssue:
		f.completionPuts++
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body) != 2 || body["status"] == nil || body["completion"] == nil {
			f.t.Errorf("completion PUT must contain only status/completion: %#v", body)
		}
		var binding map[string]any
		_ = json.Unmarshal(body["completion"], &binding)
		if len(binding) != 3 || binding["authorization_id"] != completionCLIAuthority || binding["operation_id"] != f.operationID || binding["expected_issue_revision"] != float64(7) {
			f.t.Errorf("completion binding=%#v", binding)
		}
		if f.completionMode == "unknown" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "commit_unknown"})
			return
		}
		if f.completionMode == "redirect" {
			http.Redirect(w, r, f.redirectTarget, http.StatusTemporaryRedirect)
			return
		}
		f.state = "consumed"
		f.currentStatus = "todo"
		_ = json.NewEncoder(w).Encode(map[string]any{"id": completionCLIIssue, "revision": 8, "status": "done", "completion_receipt": f.receipt()})
	case r.Method == http.MethodPost && r.URL.Path == "/api/delivery-completion-authorizations/"+f.authorizationID+"/revocations":
		f.revocations++
		if r.Header.Get("Idempotency-Key") == "" {
			f.t.Error("revocation requires its own idempotency key")
		}
		if f.revocationMode == "unknown" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "commit_unknown"})
			return
		}
		f.state = "revoked"
		_ = json.NewEncoder(w).Encode(f.authorization(f.authorizationExpiry, nil))
	default:
		http.NotFound(w, r)
	}
}

func (f *completionCLIFake) preview() map[string]any {
	checked := f.now.Add(-time.Second)
	return map[string]any{"protocol_version": 1, "issue_id": completionCLIIssue, "issue_revision": 7, "current_status": "in_review", "current_category": "in_progress", "target_status": "done", "target_category": "done", "project_id": completionCLIProject, "project_revision": 3, "policy_mode": "enforce", "policy_revision": 4, "checked_at": completionCLIFormat(checked), "reason": "authorization_available", "max_expires_at": completionCLIFormat(f.maxExpires), "enforced": false, "usable_for_completion": false, "skip_supported": false,
		"observation":           map[string]any{"mode": "observe", "policy_version": "delivery-artifact-observe-v1", "verdict": "pass", "reason": "current evidence matches", "issue_id": completionCLIIssue, "issue_revision": 7, "project_id": completionCLIProject, "project_revision": 3, "approval_id": completionCLIApproval, "execution_id": "66666666-6666-4666-8666-666666666666", "source_revision": strings.Repeat("a", 40), "observed_head_sha": strings.Repeat("a", 40), "revision_provenance": "database_observed_pr_head_not_remote_verified", "spec_digest": strings.Repeat("b", 64), "artifact_digest": strings.Repeat("c", 64), "receipt_digest": strings.Repeat("d", 64), "observed_at": completionCLIFormat(checked), "valid_until": completionCLIFormat(f.validUntil)},
		"authorization_binding": map[string]any{"contract_version": "delivery-completion-enforcement-v1", "source_workspace_id": completionCLIWorkspace, "target": map[string]any{"kind": "ordinary", "issue_id": completionCLIIssue, "expected_issue_revision": 7, "expected_status": "in_review", "target_status": "done", "target_position": nil}, "project": map[string]any{"id": completionCLIProject, "expected_revision": 3, "expected_policy_revision": 4}, "evidence": map[string]any{"issue_id": completionCLIIssue, "expected_issue_revision": 7, "approval_id": completionCLIApproval, "expected_source_revision": strings.Repeat("a", 40), "expected_spec_digest": strings.Repeat("b", 64), "expected_artifact_digest": strings.Repeat("c", 64)}, "consumer": map[string]any{"kind": "human", "user_id": completionCLIUser}}}
}
func (f *completionCLIFake) authorization(expiry any, receipt map[string]any) map[string]any {
	applicability := "not_checked"
	if f.state == "consumed" || f.state == "revoked" {
		applicability = f.state
	}
	out := map[string]any{"authorization_id": f.authorizationID, "operation_id": f.operationID, "issue_id": completionCLIIssue, "issue_revision": 7, "target_status": "done", "contract_version": "delivery-completion-enforcement-v1", "created_at": completionCLIFormat(f.now), "expires_at": expiry, "state": f.state, "applicability": applicability, "replayed": false}
	if receipt != nil {
		out["completion_receipt"] = receipt
	}
	return out
}
func (f *completionCLIFake) receipt() map[string]any {
	return map[string]any{"authorization_id": completionCLIAuthority, "operation_id": f.operationID, "issue_id": completionCLIIssue, "completed_revision": 8, "target_status": "done", "completed_at": completionCLIFormat(f.now), "replayed": false}
}
func (f *completionCLIFake) receiptIfConsumed() map[string]any {
	if f.state != "consumed" {
		return nil
	}
	return f.receipt()
}
func completionCLIFormat(v time.Time) string { return v.UTC().Format("2006-01-02T15:04:05.000000Z") }
func runCompletionCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	resetCompletionCommandFlags()
	rootCmd.SetArgs(args)
	defer rootCmd.SetArgs(nil)
	return captureStdout(t, rootCmd.Execute)
}
func resetCompletionCommandFlags() {
	completion := commandChild(issueCmd, "completion")
	if completion == nil {
		return
	}
	for _, cmd := range completion.Commands() {
		cmd.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	}
}
func prepareCompletionOperation(t *testing.T, f *completionCLIFake) string {
	t.Helper()
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	file := filepath.Join(dir, "completion.json")
	if _, err := runCompletionCLI(t, "issue", "completion", "prepare", completionCLIIssue, "--status", "done", "--approval", completionCLIApproval, "--operation-file", file); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	return file
}
func authorizeCompletionOperation(t *testing.T, f *completionCLIFake, file string) {
	t.Helper()
	expiry := completionCLIFormat(f.now.Add(10 * time.Minute))
	if _, err := runCompletionCLI(t, "issue", "completion", "authorize", "--operation-file", file, "--accept", "--reason", "I inspected this exact current delivery", "--expires-at", expiry); err != nil {
		t.Fatalf("authorize: %v", err)
	}
}

func TestIssueCompletionExplicitAuthorizationAndSubmit(t *testing.T) {
	f := newCompletionCLIFake(t)
	file := prepareCompletionOperation(t, f)
	authorizeCompletionOperation(t, f, file)
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("operation file mode=%v err=%v", info.Mode(), err)
	}
	if strings.Contains(string(data), "evaluation_key") || strings.Contains(string(data), "mul_fake-human") {
		t.Fatalf("operation file contains private evaluation data or bearer token: %s", data)
	}
	if _, err := runCompletionCLI(t, "issue", "completion", "submit", "--operation-file", file); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := runCompletionCLI(t, "issue", "completion", "submit", "--operation-file", file); err != nil {
		t.Fatalf("receipt recovery: %v", err)
	}
	if f.authPosts != 1 || f.authByKeyReads != 0 || f.completionPuts != 1 {
		t.Fatalf("authorization POST/key reads/completion PUT=%d/%d/%d", f.authPosts, f.authByKeyReads, f.completionPuts)
	}
	for _, args := range [][]string{
		{"issue", "completion", "show", "--authorization-id", f.authorizationID},
		{"issue", "completion", "show", "--request-key", f.requestKey, "--source-workspace", completionCLIWorkspace},
	} {
		out, err := runCompletionCLI(t, args...)
		if err != nil {
			t.Fatalf("show %v: %v", args[3:], err)
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(out), &envelope); err != nil || envelope["outcome"] != "consumed" {
			t.Fatalf("show %v output=%q err=%v", args[3:], out, err)
		}
	}
}

func TestIssueCompletionUnknownAuthorizationIsLookupOnly(t *testing.T) {
	f := newCompletionCLIFake(t)
	f.authorizationMode = "unknown"
	file := prepareCompletionOperation(t, f)
	expiry := completionCLIFormat(f.now.Add(10 * time.Minute))
	first, firstErr := runCompletionCLI(t, "issue", "completion", "authorize", "--operation-file", file, "--accept", "--reason", "I inspected this exact current delivery", "--expires-at", expiry)
	var firstEnvelope map[string]any
	if err := json.Unmarshal([]byte(first), &firstEnvelope); err != nil || firstEnvelope["outcome"] != "authorized" {
		t.Fatalf("uncertain authorization output=%q err=%v", first, err)
	}
	if firstErr != nil {
		t.Fatalf("exact original-key lookup confirmed authorization: %v", firstErr)
	}
	if f.authPosts != 1 || f.authByKeyReads != 1 || f.completionPuts != 0 {
		t.Fatalf("authorization POST/key reads/completion PUT=%d/%d/%d", f.authPosts, f.authByKeyReads, f.completionPuts)
	}
	f.missingAuthorization = true
	if _, err := runCompletionCLI(t, "issue", "completion", "show", "--operation-file", file); err == nil {
		t.Fatal("missing original authorization unexpectedly recovered")
	}
	if f.authPosts != 1 || f.completionPuts != 0 {
		t.Fatalf("missing-key recovery wrote again: auth=%d put=%d", f.authPosts, f.completionPuts)
	}
}

func TestIssueCompletionUnknownSubmitIsLookupOnlyAndShowSeparatesCurrent(t *testing.T) {
	f := newCompletionCLIFake(t)
	f.completionMode = "unknown"
	file := prepareCompletionOperation(t, f)
	authorizeCompletionOperation(t, f, file)
	for i := 0; i < 2; i++ {
		out, err := runCompletionCLI(t, "issue", "completion", "submit", "--operation-file", file)
		if err == nil {
			t.Fatalf("submit attempt %d unexpectedly succeeded: %s", i+1, out)
		}
		var envelope map[string]any
		if e := json.Unmarshal([]byte(out), &envelope); e != nil || envelope["outcome"] != "unknown" {
			t.Fatalf("submit attempt %d output=%q err=%v", i+1, out, err)
		}
	}
	if f.completionPuts != 1 {
		t.Fatalf("uncertain completion PUTs=%d, want one", f.completionPuts)
	}
	unknownShow, showErr := runCompletionCLI(t, "issue", "completion", "show", "--operation-file", file)
	var unknownShowEnvelope map[string]any
	if showErr == nil || json.Unmarshal([]byte(unknownShow), &unknownShowEnvelope) != nil || unknownShowEnvelope["outcome"] != "unknown" {
		t.Fatalf("issued authorization after a recorded PUT must remain unknown: output=%q err=%v", unknownShow, showErr)
	}
	f.state = "consumed"
	f.currentStatus = "todo"
	out, err := runCompletionCLI(t, "issue", "completion", "show", "--operation-file", file)
	if err != nil {
		t.Fatalf("show original result: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatal(err)
	}
	current := envelope["current_issue"].(map[string]any)
	if envelope["outcome"] != "consumed" || current["status"] != "todo" || !strings.Contains(envelope["reason"].(string), "historical completion") {
		t.Fatalf("historical receipt/current state conflated: %s", out)
	}
}

func TestIssueCompletionUnknownRevocationIsLookupOnly(t *testing.T) {
	f := newCompletionCLIFake(t)
	f.revocationMode = "unknown"
	file := prepareCompletionOperation(t, f)
	authorizeCompletionOperation(t, f, file)
	for i := 0; i < 2; i++ {
		out, err := runCompletionCLI(t, "issue", "completion", "revoke", "--operation-file", file, "--reason", "withdraw this authorization")
		if err == nil {
			t.Fatalf("revoke attempt %d unexpectedly succeeded: %s", i+1, out)
		}
		var envelope map[string]any
		if e := json.Unmarshal([]byte(out), &envelope); e != nil || envelope["outcome"] != "unknown" {
			t.Fatalf("revoke attempt %d output=%q err=%v", i+1, out, err)
		}
	}
	if f.revocations != 1 {
		t.Fatalf("revocation POSTs=%d, want one", f.revocations)
	}
	f.state = "revoked"
	out, err := runCompletionCLI(t, "issue", "completion", "show", "--operation-file", file)
	if err != nil {
		t.Fatalf("show revoked terminal: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(out), &envelope); err != nil || envelope["outcome"] != "revoked" {
		t.Fatalf("show revoked output=%q err=%v", out, err)
	}
}

func TestIssueCompletionOperationFileLockAndIntegrityPrecedeWrites(t *testing.T) {
	f := newCompletionCLIFake(t)
	file := prepareCompletionOperation(t, f)
	lock, err := os.OpenFile(file+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_ = lock.Close()
	if _, err := runCompletionCLI(t, "issue", "completion", "authorize", "--operation-file", file, "--accept", "--reason", "accept exact result", "--expires-at", completionCLIFormat(f.now.Add(10*time.Minute))); err == nil {
		t.Fatal("locked operation unexpectedly dispatched")
	}
	_ = os.Remove(file + ".lock")
	if f.authPosts != 0 {
		t.Fatalf("locked operation authorization POSTs=%d", f.authPosts)
	}
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.Replace(body, []byte(`"current_status": "in_review"`), []byte(`"current_status": "todo"`), 1)
	if err := os.WriteFile(file, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCompletionCLI(t, "issue", "completion", "authorize", "--operation-file", file, "--accept", "--reason", "accept exact result", "--expires-at", completionCLIFormat(f.now.Add(10*time.Minute))); err == nil {
		t.Fatal("modified operation unexpectedly dispatched")
	}
	if f.authPosts != 0 {
		t.Fatalf("modified operation authorization POSTs=%d", f.authPosts)
	}
}

func TestIssueCompletionSubmitRedirectIsNotFollowed(t *testing.T) {
	targetHits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { targetHits++; w.WriteHeader(http.StatusOK) }))
	defer target.Close()
	f := newCompletionCLIFake(t)
	f.completionMode = "redirect"
	f.redirectTarget = target.URL
	file := prepareCompletionOperation(t, f)
	authorizeCompletionOperation(t, f, file)
	_, err := runCompletionCLI(t, "issue", "completion", "submit", "--operation-file", file)
	if err == nil {
		t.Fatal("redirected completion unexpectedly succeeded")
	}
	if f.completionPuts != 1 || targetHits != 0 {
		t.Fatalf("source PUTs/redirect target hits=%d/%d", f.completionPuts, targetHits)
	}
}

func TestIssueCompletionAuthorizationValidationBindsStateAndReceipt(t *testing.T) {
	now := time.Now().UTC()
	valid := completionCLIAuthorization{
		AuthorizationID: completionCLIAuthority, OperationID: "88888888-8888-4888-8888-888888888888",
		IssueID: completionCLIIssue, IssueRevision: 7, TargetStatus: "done", ContractVersion: "delivery-completion-enforcement-v1",
		CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), State: "consumed", Applicability: "consumed",
		CompletionReceipt: &completionCLIReceipt{AuthorizationID: completionCLIAuthority, OperationID: "88888888-8888-4888-8888-888888888888", IssueID: completionCLIIssue, CompletedRevision: 8, TargetStatus: "done", CompletedAt: now},
	}
	if err := completionValidateAuthorization(valid, nil); err != nil {
		t.Fatalf("valid consumed receipt rejected: %v", err)
	}

	badReceipt := valid
	wrong := *valid.CompletionReceipt
	wrong.OperationID = "99999999-9999-4999-8999-999999999999"
	badReceipt.CompletionReceipt = &wrong
	if err := completionValidateAuthorization(badReceipt, nil); err == nil {
		t.Fatal("consumed authorization with a receipt for another operation was accepted")
	}

	badState := valid
	badState.State, badState.Applicability, badState.CompletionReceipt = "issued", "consumed", nil
	if err := completionValidateAuthorization(badState, nil); err == nil {
		t.Fatal("issued authorization with consumed applicability was accepted")
	}
}

func TestIssueCompletionExpiredAuthorizationDoesNotSubmit(t *testing.T) {
	f := newCompletionCLIFake(t)
	file := prepareCompletionOperation(t, f)
	authorizeCompletionOperation(t, f, file)
	op, err := readCompletionOperation(file)
	if err != nil {
		t.Fatal(err)
	}
	op.Authorization.Applicability = "expired"
	op.Authorization.ExpiresAt = f.now.Add(-time.Second)
	if err := saveCompletionOperation(file, &op); err != nil {
		t.Fatal(err)
	}
	if _, err := runCompletionCLI(t, "issue", "completion", "submit", "--operation-file", file); err == nil {
		t.Fatal("expired authorization unexpectedly submitted")
	}
	if f.completionPuts != 0 {
		t.Fatalf("expired authorization sent %d completion PUTs, want zero", f.completionPuts)
	}
}

func TestIssueCompletionPrepareListsEvidenceWithoutAuthorizing(t *testing.T) {
	f := newCompletionCLIFake(t)
	out, err := runCompletionCLI(t, "issue", "completion", "prepare", completionCLIIssue, "--status", "done")
	if err != nil {
		t.Fatalf("prepare candidates: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("prepare output JSON: %v", err)
	}
	if envelope["outcome"] != "candidates" || f.candidateReads != 1 || f.previewReads != 0 || f.authPosts != 0 {
		t.Fatalf("candidate output=%s reads=%d previews=%d auth=%d", out, f.candidateReads, f.previewReads, f.authPosts)
	}
	table, err := runCompletionCLI(t, "issue", "completion", "prepare", completionCLIIssue, "--status", "done", "--output", "table")
	if err != nil || !strings.Contains(table, "APPROVAL") || f.authPosts != 0 {
		t.Fatalf("candidate table=%q err=%v auth posts=%d", table, err, f.authPosts)
	}
}

func commandChild(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}
