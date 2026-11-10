package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type controlledArtifactRouterFixture struct {
	fx                                      *testutil.Fixture
	router                                  http.Handler
	request                                 artifact.Request
	user, token, taskToken, tokenID, output string
}
type controlledArtifactRouterResponse struct {
	Approval struct {
		ID        string  `json:"id"`
		Status    string  `json:"status"`
		DecidedBy *string `json:"decided_by"`
		Plan      string  `json:"plan"`
	} `json:"approval"`
	ExecutionID   string `json:"execution_id"`
	SpecDigest    string `json:"spec_digest"`
	Status        string `json:"status"`
	ReceiptDigest string `json:"receipt_digest"`
}

func controlledArtifactJWT(t *testing.T, user string, version int64, expiry time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": user, "email": "artifact-router@example.test", "name": "Artifact router", "iat": time.Now().Add(-time.Minute).Unix(), "exp": expiry.Unix()}
	if version > 0 {
		claims["auth_version"] = version
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(auth.JWTSecret())
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func controlledArtifactRouter(t *testing.T) http.Handler {
	t.Helper()
	hub := realtime.NewHub()
	return NewRouter(testPool, hub, events.New(), analytics.NoopClient{}, nil)
}
func newControlledArtifactRouterFixture(t *testing.T, mode string) *controlledArtifactRouterFixture {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", mode)
	t.Setenv("MULTICA_DEVICE_AUTH_ENABLED", "false")
	t.Setenv("MULTICA_CLOUD_URL", "")
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
	setup := testutil.New(testPool, testWorkspaceID, testUserID)
	user := setup.User(t, "Artifact router user", "artifact-"+uuid.NewString()+"@example.test")
	setup.Member(t, testWorkspaceID, user, "member")
	fx := testutil.New(testPool, testWorkspaceID, user)
	runtime := fx.Runtime(t, "Artifact router runtime")
	agent := fx.Agent(t, "Artifact router agent", runtime, testutil.Cols{"autonomy_level": "operator"})
	issue := fx.Issue(t, "Artifact router issue")
	task := fx.Task(t, agent, testutil.Cols{"status": "running", "issue_id": issue, "runtime_id": runtime, "originator_user_id": user, "accountable_user_id": user, "claim_generation": 1})
	version := int64(0)
	if mode == "password" {
		version = 1
		fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": user, "username": "artifact-" + uuid.NewString(), "password_hash": "unused-by-signed-jwt-fixture", "session_version": 1, "must_change_password": false}, "user_id=$1", user)
	}
	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	tokenID := fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(token), "task_id": task, "agent_id": agent, "workspace_id": testWorkspaceID, "user_id": user, "expires_at": time.Now().Add(time.Hour), "auth_version": version})
	entry := artifact.Entry{Path: "candidate.txt", SizeBytes: 9, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("candidate")))}
	input, err := artifact.InputDigest([]artifact.Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	manifest := artifact.StagingManifest{WorkspaceID: testWorkspaceID, AgentID: agent, TaskID: task, TaskGeneration: 1, ResourceID: issue, SourceRevision: strings.Repeat("a", 40), InputManifestDigest: input}
	digest, err := artifact.StagingDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	request := artifact.Request{AgentID: agent, TaskID: task, ResourceKind: "issue", ResourceID: issue, Action: artifact.Action, StagingID: uuid.NewString(), SourceRevision: manifest.SourceRevision, StagingManifestDigest: digest, InputManifestDigest: input, Entries: []artifact.Entry{entry}, TargetEnvironment: artifact.Environment, PolicyVersion: artifact.Policy}
	base := t.TempDir()
	stage := filepath.Join(base, "stage")
	output := filepath.Join(base, "output")
	files := filepath.Join(stage, request.StagingID, "files")
	if err = os.MkdirAll(files, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(stage, request.StagingID, "manifest.json"), raw, 0400); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(files, "candidate.txt"), []byte("candidate"), 0400); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{files, filepath.Dir(files), stage} {
		if err = os.Chmod(dir, 0500); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, dir := range []string{stage, filepath.Dir(files), files} {
			_ = os.Chmod(dir, 0700)
		}
	})
	t.Setenv("MULTICA_ARTIFACT_STAGING_ROOT", stage)
	t.Setenv("MULTICA_ARTIFACT_OUTPUT_ROOT", output)
	t.Setenv("MULTICA_ARTIFACT_CONFIG_ID", "artifact-router")
	fx.Cleanup(t, `DELETE FROM agent_approval_request WHERE task_id=$1`, task)
	fx.Cleanup(t, `DELETE FROM agent_artifact_execution WHERE task_id=$1`, task)
	return &controlledArtifactRouterFixture{fx: fx, router: controlledArtifactRouter(t), request: request, user: user, token: controlledArtifactJWT(t, user, version, time.Now().Add(time.Hour)), taskToken: token, tokenID: tokenID, output: output}
}
func (f *controlledArtifactRouterFixture) call(t *testing.T, token, method, path string, body any, headers ...string) *testutil.Response {
	t.Helper()
	req := testutil.WithHeaders(testutil.JSONRequest(method, path, body), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID, "Idempotency-Key", uuid.NewString())
	testutil.WithHeaders(req, headers...)
	return testutil.Call(t, f.router.ServeHTTP, req)
}
func (f *controlledArtifactRouterFixture) create(t *testing.T, token string) controlledArtifactRouterResponse {
	t.Helper()
	var out controlledArtifactRouterResponse
	f.call(t, token, "POST", "/api/agent-approvals/artifact-package", f.request).Want(201).JSON(&out)
	if out.Approval.Status != "pending" || out.Status != "unclaimed" || out.SpecDigest == "" {
		t.Fatalf("invalid created contract: %+v", out)
	}
	return out
}
func (f *controlledArtifactRouterFixture) noClaim(t *testing.T, id string) {
	t.Helper()
	if f.fx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NOT NULL`, id) != 0 {
		t.Fatal("denied request claimed execution")
	}
	entries, err := os.ReadDir(f.output)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("denied request produced artifact")
	}
}
func TestControlledArtifactRouterTaskIdentityAndHumanDecision(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			f := newControlledArtifactRouterFixture(t, mode)
			out := f.create(t, f.taskToken)
			base := "/api/agent-approvals/" + out.Approval.ID
			var detail controlledArtifactRouterResponse
			f.call(t, f.taskToken, "GET", base+"/artifact-package", nil).Want(200).JSON(&detail)
			if detail.ExecutionID != out.ExecutionID {
				t.Fatal("detail changed identity")
			}
			decision := map[string]string{"decision": "approve", "note": "exact frozen spec reviewed", "expected_spec_digest": out.SpecDigest}
			f.call(t, f.taskToken, "POST", base+"/decision", decision, "X-Actor-Source", "member").Want(403)
			f.noClaim(t, out.Approval.ID)
			wrong := map[string]string{"decision": "approve", "expected_spec_digest": strings.Repeat("0", 64)}
			f.call(t, f.token, "POST", base+"/decision", wrong).Want(409)
			f.call(t, f.token, "POST", base+"/decision", decision).Want(200)
			var done controlledArtifactRouterResponse
			f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}).Want(200).JSON(&done)
			if done.Status != "succeeded" || done.Approval.Status != "executed" || done.ReceiptDigest == "" || done.Approval.DecidedBy == nil || *done.Approval.DecidedBy != f.user {
				t.Fatalf("router did not preserve approved execution: %+v", done)
			}
			var replay controlledArtifactRouterResponse
			f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}).Want(200).JSON(&replay)
			if replay.ExecutionID != done.ExecutionID || replay.ReceiptDigest != done.ReceiptDigest {
				t.Fatal("replay rewrote receipt")
			}
		})
	}
}
func TestControlledArtifactRouterScopeAndHeaders(t *testing.T) {
	f := newControlledArtifactRouterFixture(t, "legacy")
	out := f.create(t, f.token)
	base := "/api/agent-approvals/" + out.Approval.ID
	var runtime string
	f.fx.QueryRow(t, `SELECT runtime_id::text FROM agent_task_queue WHERE id=$1`, f.request.TaskID).Scan(&runtime)
	otherTask := f.fx.Task(t, f.request.AgentID, testutil.Cols{"status": "running", "runtime_id": runtime, "issue_id": f.request.ResourceID, "originator_user_id": f.user, "accountable_user_id": f.user, "claim_generation": 1})
	otherToken, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(otherToken), "task_id": otherTask, "agent_id": f.request.AgentID, "workspace_id": testWorkspaceID, "user_id": f.user, "expires_at": time.Now().Add(time.Hour)})
	for _, tc := range []struct {
		name, token, method, path string
		body                      any
		headers                   []string
	}{
		{"another_task_create", otherToken, "POST", "/api/agent-approvals/artifact-package", f.request, nil},
		{"another_task_detail", otherToken, "GET", base + "/artifact-package", nil, nil},
		{"another_task_execute", otherToken, "POST", base + "/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}, nil},
		{"human_agent_header", f.token, "POST", "/api/agent-approvals/artifact-package", f.request, []string{"X-Agent-ID", f.request.AgentID}},
		{"human_task_header", f.token, "POST", "/api/agent-approvals/artifact-package", f.request, []string{"X-Task-ID", f.request.TaskID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.call(t, tc.token, tc.method, tc.path, tc.body, tc.headers...).Want(403)
			f.noClaim(t, out.Approval.ID)
		})
	}
	foreignWS := f.fx.Workspace(t, "Other artifact workspace", "artifact-other-"+uuid.NewString())
	f.fx.Member(t, foreignWS, f.user, "owner")
	f.call(t, f.token, "POST", "/api/agent-approvals/artifact-package", f.request, "X-Workspace-ID", foreignWS).Want(403)
	// Task middleware replaces forged actor/workspace headers with token provenance.
	var scoped controlledArtifactRouterResponse
	f.call(t, f.taskToken, "POST", "/api/agent-approvals/artifact-package", f.request, "X-Workspace-ID", foreignWS, "X-Task-ID", otherTask).Want(201).JSON(&scoped)
	var actualWS, actualTask string
	f.fx.QueryRow(t, `SELECT workspace_id::text,task_id::text FROM agent_artifact_execution WHERE approval_id=$1`, scoped.Approval.ID).Scan(&actualWS, &actualTask)
	if actualWS != testWorkspaceID || actualTask != f.request.TaskID {
		t.Fatal("untrusted headers replaced verified task identity")
	}
	missing := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/agent-approvals/artifact-package", f.request), "X-User-ID", f.user, "X-Workspace-ID", testWorkspaceID, "X-Agent-ID", f.request.AgentID, "X-Task-ID", f.request.TaskID)
	testutil.Call(t, f.router.ServeHTTP, missing).Want(401)
}
func TestControlledArtifactRouterRestrictedCapabilities(t *testing.T) {
	for _, role := range []struct{ key, profile string }{{"code-reviewer", auth.TaskCapabilityReviewer}, {"progress-reporter", auth.TaskCapabilityReporter}} {
		t.Run(role.key, func(t *testing.T) {
			f := newControlledArtifactRouterFixture(t, "legacy")
			out := f.create(t, f.token)
			var runtime string
			f.fx.QueryRow(t, `SELECT runtime_id::text FROM agent_task_queue WHERE id=$1`, f.request.TaskID).Scan(&runtime)
			project := f.fx.Project(t, "Artifact profile project")
			f.fx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, project, f.request.ResourceID)
			f.fx.Exec(t, `UPDATE agent SET template_key=$1,template_version=2 WHERE id=$2`, role.key, f.request.AgentID)
			capability, err := json.Marshal(auth.TaskCapability{Version: 1, Profile: role.profile, IssueID: f.request.ResourceID, ProjectID: project, OriginatorID: f.user, RuntimeID: runtime, ClaimGeneration: 1})
			if err != nil {
				t.Fatal(err)
			}
			f.fx.Exec(t, `UPDATE task_token SET api_capability=$1 WHERE id=$2`, capability, f.tokenID)
			// Positive scoped read proves this is a valid token rather than malformed auth.
			f.call(t, f.taskToken, "GET", "/api/issues/"+f.request.ResourceID, nil).Want(200)
			base := "/api/agent-approvals/" + out.Approval.ID
			f.call(t, f.taskToken, "POST", "/api/agent-approvals/artifact-package", f.request).Want(403)
			f.call(t, f.taskToken, "GET", base+"/artifact-package", nil).Want(403)
			f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}).Want(403)
			f.noClaim(t, out.Approval.ID)
		})
	}
}
func TestControlledArtifactRouterPasswordRevocationAndExpiry(t *testing.T) {
	f := newControlledArtifactRouterFixture(t, "password")
	out := f.create(t, f.token)
	base := "/api/agent-approvals/" + out.Approval.ID
	decision := map[string]string{"decision": "approve", "expected_spec_digest": out.SpecDigest}
	f.call(t, f.token, "POST", base+"/decision", decision).Want(200)
	f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.user)
	f.call(t, f.token, "GET", base+"/artifact-package", nil).Want(401)
	f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}).Want(401)
	f.noClaim(t, out.Approval.ID)
	current := controlledArtifactJWT(t, f.user, 2, time.Now().Add(time.Hour))
	f.call(t, current, "GET", base+"/artifact-package", nil).Want(200)
	// Revoke the original JWT version after real authentication but before the
	// artifact handler's transactional credential recheck.
	req := testutil.WithHeaders(testutil.JSONRequest("POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}), "Authorization", "Bearer "+current, "X-Workspace-ID", testWorkspaceID)
	revoked := false
	req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() {
		f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.user)
		revoked = true
	}}
	denied := testutil.Call(t, f.router.ServeHTTP, req).Want(403)
	if !revoked || !strings.Contains(denied.Text(), "artifact_credential_revoked") {
		t.Fatal("password mutation did not reach original-version transaction recheck")
	}
	f.noClaim(t, out.Approval.ID)
	current = controlledArtifactJWT(t, f.user, 3, time.Now().Add(time.Hour))

	expired := controlledArtifactJWT(t, f.user, 3, time.Now().Add(-time.Second))
	f.call(t, expired, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}).Want(401)
	f.noClaim(t, out.Approval.ID)
	f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, f.user)
	f.call(t, current, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}).Want(403)
	f.noClaim(t, out.Approval.ID)
}
func TestControlledArtifactRouterCloudIdentityCannotBecomeHuman(t *testing.T) {
	f := newControlledArtifactRouterFixture(t, "legacy")
	var verifies atomic.Int32
	fleet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verifies.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "owner_id": f.user})
	}))
	t.Cleanup(fleet.Close)
	t.Setenv("MULTICA_CLOUD_URL", fleet.URL)
	f.router = controlledArtifactRouter(t)
	response := f.call(t, "mcn_synthetic-artifact-only", "POST", "/api/agent-approvals/artifact-package", f.request, "X-Actor-Source", "member").Want(403)
	if !strings.Contains(response.Text(), "artifact_authority_denied") || verifies.Load() != 1 {
		t.Fatal("cloud probe did not reach verified identity denial")
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE task_id=$1`, f.request.TaskID) != 0 {
		t.Fatal("cloud identity created approval")
	}
}

type artifactRevokeOnRead struct {
	io.ReadCloser
	revoke func()
}

func (r *artifactRevokeOnRead) Read(p []byte) (int, error) {
	if r.revoke != nil {
		revoke := r.revoke
		r.revoke = nil
		revoke()
	}
	return r.ReadCloser.Read(p)
}
func TestControlledArtifactRouterPATRevocationAfterAuthentication(t *testing.T) {
	f := newControlledArtifactRouterFixture(t, "legacy")
	token, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	id := f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.user, "name": "Artifact router PAT", "token_hash": auth.HashToken(token), "token_prefix": token[:8], "expires_at": time.Now().Add(time.Hour)})
	// Human JWT creates/approves; the PAT's real Auth middleware succeeds before
	// the body reader revokes its stored hash, exposing the handler's live fence.
	out := f.create(t, f.token)
	base := "/api/agent-approvals/" + out.Approval.ID
	f.call(t, f.token, "POST", base+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": out.SpecDigest}).Want(200)
	req := testutil.WithHeaders(testutil.JSONRequest("POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID)
	revoked := false
	req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() { f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, id); revoked = true }}
	denied := testutil.Call(t, f.router.ServeHTTP, req).Want(404)
	if !revoked || !strings.Contains(denied.Text(), "artifact_scope_not_found") {
		t.Fatal("probe did not reach live controlled credential check")
	}
	f.noClaim(t, out.Approval.ID)
	f.call(t, token, "GET", base+"/artifact-package", nil).Want(401)
}

func TestControlledArtifactRouterPrivateSpecStaysOutOfWorkspaceQueue(t *testing.T) {
	f := newControlledArtifactRouterFixture(t, "legacy")
	out := f.create(t, f.token)
	outsider := f.fx.User(t, "Artifact ordinary member", "artifact-reader-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, outsider, "member")
	token := controlledArtifactJWT(t, outsider, 0, time.Now().Add(time.Hour))
	queue := f.call(t, token, "GET", "/api/agent-approvals/", nil).Want(200)
	if !strings.Contains(queue.Text(), out.Approval.ID) || strings.Contains(queue.Text(), "candidate.txt") || strings.Contains(queue.Text(), f.request.StagingID) || strings.Contains(queue.Text(), f.output) {
		t.Fatal("queue omitted safe approval or leaked private specification")
	}
	denied := f.call(t, token, "GET", "/api/agent-approvals/"+out.Approval.ID+"/artifact-package", nil).Want(403)
	if strings.Contains(denied.Text(), "candidate.txt") || strings.Contains(denied.Text(), f.request.StagingID) {
		t.Fatal("private spec leaked in refusal")
	}
	detail := f.call(t, f.token, "GET", "/api/agent-approvals/"+out.Approval.ID+"/artifact-package", nil).Want(200)
	if detail.Header().Get("Cache-Control") != "no-store" || !strings.Contains(detail.Text(), "candidate.txt") {
		t.Fatal("authorized spec is missing or cacheable")
	}
	f.noClaim(t, out.Approval.ID)
}

func bindControlledArtifactRouter(t *testing.T, f *controlledArtifactRouterFixture) (binding, installation string) {
	t.Helper()
	deployment := uuid.NewString()
	t.Setenv("MULTICA_DEPLOYMENT_ID", deployment)
	org, err := db.New(testPool).EnsureInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM organization_workspace WHERE workspace_id=$1`, testWorkspaceID) == 0 {
		f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": org.ID, "workspace_id": testWorkspaceID}, "workspace_id=$1", testWorkspaceID)
	}
	installation = f.fx.Insert(t, "managed_installation", testutil.Cols{"deployment_id": deployment, "organization_id": org.ID, "public_key": make([]byte, 32), "key_fingerprint": auth.HashToken(uuid.NewString()), "responsible_user_id": f.user})
	daemon := uuid.NewString()
	binding = f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": installation, "workspace_id": testWorkspaceID, "daemon_id": daemon, "principal_user_id": f.user, "auth_version": 1, "binding_epoch": 1})
	f.fx.Exec(t, `UPDATE task_token SET installation_binding_id=$1,installation_binding_epoch=1 WHERE id=$2`, binding, f.tokenID)
	f.fx.Exec(t, `UPDATE agent_task_queue SET execution_binding_id=$1,execution_binding_epoch=1,execution_installation_id=$2 WHERE id=$3`, binding, installation, f.request.TaskID)
	f.fx.Exec(t, `UPDATE agent_runtime SET daemon_id=$1,runtime_mode='local' WHERE id=(SELECT runtime_id FROM agent_task_queue WHERE id=$2)`, daemon, f.request.TaskID)
	return binding, installation
}
func TestControlledArtifactRouterManagedBindingLiveFences(t *testing.T) {
	for _, change := range []string{"allowed", "binding_epoch", "binding_revoked", "installation_retired", "organization_mapping_removed", "binding_lock_busy"} {
		t.Run(change, func(t *testing.T) {
			f := newControlledArtifactRouterFixture(t, "password")
			binding, installation := bindControlledArtifactRouter(t, f)
			out := f.create(t, f.taskToken)
			base := "/api/agent-approvals/" + out.Approval.ID
			f.call(t, f.token, "POST", base+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": out.SpecDigest}).Want(200)
			req := testutil.WithHeaders(testutil.JSONRequest("POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}), "Authorization", "Bearer "+f.taskToken, "X-Workspace-ID", testWorkspaceID)
			var held pgx.Tx
			changed := false
			if change != "allowed" {
				req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() {
					changed = true
					switch change {
					case "binding_epoch":
						f.fx.Exec(t, `UPDATE installation_daemon_binding SET binding_epoch=binding_epoch+1 WHERE id=$1`, binding)
					case "binding_revoked":
						f.fx.Exec(t, `UPDATE installation_daemon_binding SET state='revoked',revoked_at=now() WHERE id=$1`, binding)
					case "installation_retired":
						f.fx.Exec(t, `UPDATE managed_installation SET lifecycle='retired' WHERE id=$1`, installation)
					case "organization_mapping_removed":
						f.fx.Exec(t, `DELETE FROM organization_workspace WHERE workspace_id=$1`, testWorkspaceID)
					case "binding_lock_busy":
						var err error
						held, err = testPool.Begin(t.Context())
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = held.Rollback(context.Background()) })
						if _, err = held.Exec(t.Context(), `SELECT id FROM installation_daemon_binding WHERE id=$1 FOR UPDATE`, binding); err != nil {
							t.Fatal(err)
						}
					}
				}}
			}
			response := testutil.Call(t, f.router.ServeHTTP, req)
			if change == "allowed" {
				var done controlledArtifactRouterResponse
				response.Want(200).JSON(&done)
				if done.Status != "succeeded" || done.Approval.Status != "executed" {
					t.Fatal("valid bound execution failed")
				}
				return
			}
			want := 403
			code := "artifact_credential_revoked"
			if change == "binding_lock_busy" {
				want = 409
				code = "artifact_authority_busy"
			}
			if change == "organization_mapping_removed" {
				want = 404
				code = "artifact_scope_not_found"
			}
			response.Want(want)
			if !changed || !strings.Contains(response.Text(), code) {
				t.Fatalf("post-auth %s missed live fence: %s", change, response.Text())
			}
			f.noClaim(t, out.Approval.ID)
			if held != nil {
				if err := held.Rollback(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
