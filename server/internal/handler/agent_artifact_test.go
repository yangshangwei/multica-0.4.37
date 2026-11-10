package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestControlledArtifactDisabled(t *testing.T) {
	h := *testHandler
	h.ArtifactPackager = nil
	testutil.Call(t, http.HandlerFunc(h.CreateArtifactPackage), httptest.NewRequest(http.MethodPost, "/api/agent-approvals/artifact-package", nil)).Want(http.StatusServiceUnavailable)
}

type artifactFixture struct {
	h     Handler
	req   artifact.Request
	actor string
	ws    string
	key   string
}

func newArtifactFixture(t *testing.T, isolated ...bool) *artifactFixture {
	t.Helper()
	f := &artifactFixture{h: *testHandler, actor: testUserID, ws: testWorkspaceID, key: uuid.NewString()}
	fx := dbfx
	if len(isolated) > 0 && isolated[0] {
		f.actor = dbfx.User(t, "Artifact private owner", "artifact-owner-"+uuid.NewString()+"@example.invalid")
		f.ws = dbfx.Workspace(t, "Artifact private workspace", "artifact-"+uuid.NewString())
		dbfx.Member(t, f.ws, f.actor, "owner")
		fx = testutil.New(testPool, f.ws, f.actor)
	}

	issue := fx.Issue(t, "controlled packaging")
	runtime := fx.Runtime(t, "Artifact runtime")
	agent := fx.Agent(t, "artifact operator", runtime, testutil.Cols{"autonomy_level": "operator"})
	task := fx.Task(t, agent, testutil.Cols{"status": "running", "issue_id": issue, "runtime_id": runtime, "originator_user_id": f.actor, "accountable_user_id": f.actor, "claim_generation": int64(1)})
	entry := artifact.Entry{Path: "candidate.txt", SizeBytes: 9, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("candidate")))}
	input, err := artifact.InputDigest([]artifact.Entry{entry})
	if err != nil {
		t.Fatal(err)
	}
	manifest := artifact.StagingManifest{WorkspaceID: f.ws, AgentID: agent, TaskID: task, TaskGeneration: 1, ResourceID: issue, SourceRevision: strings.Repeat("a", 40), InputManifestDigest: input}
	digest, err := artifact.StagingDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	f.req = artifact.Request{AgentID: agent, TaskID: task, ResourceKind: "issue", ResourceID: issue, Action: artifact.Action, StagingID: uuid.NewString(), SourceRevision: manifest.SourceRevision, InputManifestDigest: input, StagingManifestDigest: digest, Entries: []artifact.Entry{entry}, TargetEnvironment: artifact.Environment, PolicyVersion: artifact.Policy}
	base := t.TempDir()
	stage := filepath.Join(base, "stage")
	output := filepath.Join(base, "output")
	files := filepath.Join(stage, f.req.StagingID, "files")
	if err = os.MkdirAll(files, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(manifest)
	if err = os.WriteFile(filepath.Join(stage, f.req.StagingID, "manifest.json"), raw, 0400); err != nil {
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
	p, err := artifact.New(artifact.Config{StagingRoot: stage, OutputRoot: output, Identity: "artifact-test"})
	if err != nil {
		t.Fatal(err)
	}
	f.h.ArtifactPackager = p
	dbfx.Cleanup(t, `DELETE FROM agent_artifact_execution WHERE task_id=$1`, task)
	dbfx.Cleanup(t, `DELETE FROM agent_approval_request WHERE task_id=$1`, task)
	return f
}
func (f *artifactFixture) request(method, path string, body any) *http.Request {
	req := testutil.WithHeaders(testutil.JSONRequest(method, path, body), "X-User-ID", f.actor, "X-Workspace-ID", f.ws, "Idempotency-Key", f.key)
	return req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
}
func (f *artifactFixture) create(t *testing.T) artifactResponse {
	t.Helper()
	var out artifactResponse
	testutil.Call(t, f.h.CreateArtifactPackage, f.request("POST", "/api/agent-approvals/artifact-package", f.req)).Want(201).JSON(&out)
	if out.Status != "unclaimed" || out.Approval.Status != "pending" {
		t.Fatal(out)
	}
	if strings.Contains(out.Approval.Plan, "candidate.txt") || strings.Contains(out.Approval.Plan, f.req.StagingID) {
		t.Fatal("private manifest leaked into public approval")
	}
	return out
}
func (f *artifactFixture) route(id, action string, body any) *http.Request {
	return testutil.WithURLParams(f.request("POST", "/api/agent-approvals/"+id+"/"+action, body), "approvalId", id)
}
func (f *artifactFixture) approve(t *testing.T, out artifactResponse) {
	t.Helper()
	testutil.Call(t, f.h.DecideAgentApproval, f.route(out.Approval.ID, "decision", map[string]string{"decision": "approve", "note": "reviewed", "expected_spec_digest": out.SpecDigest})).Want(200)
}
func (f *artifactFixture) execute(t *testing.T, out artifactResponse, status int) artifactResponse {
	t.Helper()
	var result artifactResponse
	testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(status).JSON(&result)
	return result
}
func TestControlledArtifactLifecycle(t *testing.T) {
	f := newArtifactFixture(t)
	out := f.create(t)
	var replay artifactResponse
	testutil.Call(t, f.h.CreateArtifactPackage, f.request("POST", "/api/agent-approvals/artifact-package", f.req)).Want(200).JSON(&replay)
	if replay.ExecutionID != out.ExecutionID || replay.SpecDigest != out.SpecDigest {
		t.Fatal("idempotency changed binding")
	}
	f.approve(t, out)
	done := f.execute(t, out, 200)
	if done.Status != "succeeded" || done.Approval.Status != "executed" || done.ReceiptDigest == "" {
		t.Fatal("no durable successful receipt", done)
	}
	again := f.execute(t, out, 200)
	if !bytes.Equal(again.Receipt, done.Receipt) || again.ExecutionID != done.ExecutionID {
		t.Fatal("replay changed result")
	}
	testutil.Call(t, f.h.RecordAgentApprovalExecution, f.route(out.Approval.ID, "execution", map[string]string{"note": "fabricated"})).Want(409)
	testutil.Call(t, f.h.CancelAgentApproval, f.route(out.Approval.ID, "cancel", nil)).Want(409)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, f.req.TaskID)
	var historical artifactResponse
	testutil.Call(t, f.h.CreateArtifactPackage, f.request("POST", "/api/agent-approvals/artifact-package", f.req)).Want(200).JSON(&historical)
	if historical.ExecutionID != out.ExecutionID || historical.Approval.Status != "executed" {
		t.Fatal("creation replay renewed terminal task binding")
	}
}
func TestControlledArtifactRejections(t *testing.T) {
	cases := []string{"unapproved", "digest", "cancel", "expiry", "role", "task", "tokenless_agent", "unknown_body", "duplicate_body", "changed_key", "changed_actor", "changed_approver"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			f := newArtifactFixture(t)
			out := f.create(t)
			if name != "unapproved" && name != "cancel" {
				f.approve(t, out)
			}
			req := f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})
			want := 409
			switch name {
			case "digest":
				req = f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": strings.Repeat("0", 64)})
			case "cancel":
				testutil.Call(t, f.h.CancelAgentApproval, f.route(out.Approval.ID, "cancel", nil)).Want(200)
			case "expiry":
				var oldSpec artifact.Spec
				json.Unmarshal(out.Spec, &oldSpec)
				past := time.Now().UTC().Add(-16 * time.Minute).Truncate(time.Microsecond)
				oldSpec.CreatedAt = past.Format("2006-01-02T15:04:05.000000Z")
				oldSpec.ExpiresAt = past.Add(15 * time.Minute).Format("2006-01-02T15:04:05.000000Z")
				raw, digest, err := artifact.CanonicalSpec(oldSpec)
				if err != nil {
					t.Fatal(err)
				}
				dbfx.Exec(t, `UPDATE agent_artifact_execution SET created_at=$1,expires_at=$2,canonical_spec=$3,spec_digest=$4 WHERE approval_id=$5`, past, past.Add(15*time.Minute), raw, digest, out.Approval.ID)
				req = f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": digest})
				want = 410
			case "role":
				dbfx.Exec(t, `UPDATE agent SET autonomy_level='contributor' WHERE id=$1`, f.req.AgentID)
				want = 403
			case "task":
				dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, f.req.TaskID)
				want = 403
			case "tokenless_agent":
				req.Header.Set("X-Agent-ID", f.req.AgentID)
				want = 403
			case "unknown_body":
				req = f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest, "argv": "sh"})
				want = 422
			case "duplicate_body":
				req = f.route(out.Approval.ID, "artifact-package/execution", `{"expected_spec_digest":"`+out.SpecDigest+`","expected_spec_digest":"`+out.SpecDigest+`"}`)
				want = 422
			case "changed_key":
				f.req.SourceRevision = strings.Repeat("b", 40)
				testutil.Call(t, f.h.CreateArtifactPackage, f.request("POST", "/api/agent-approvals/artifact-package", f.req)).Want(409)
				return
			case "changed_actor":
				req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialCloud}))
				want = 403
			case "changed_approver":
				dbfx.Exec(t, `UPDATE agent_approval_request SET decided_by=$1 WHERE id=$2`, uuid.NewString(), out.Approval.ID)
				want = 403
			}
			testutil.Call(t, f.h.ExecuteArtifactPackage, req).Want(want)
			if count := dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NOT NULL`, out.Approval.ID); count != 0 {
				t.Fatal("refused request acquired a claim")
			}
		})
	}
}

type gatedArtifact struct {
	artifactExecutor
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
	panic   bool
}

func (p *gatedArtifact) Pack(ctx context.Context, id string, s artifact.Spec) artifact.Outcome {
	p.calls.Add(1)
	close(p.entered)
	<-p.release
	if p.panic {
		panic("private adapter error")
	}
	return p.artifactExecutor.Pack(ctx, id, s)
}
func TestControlledArtifactOnePhysicalInvocation(t *testing.T) {
	f := newArtifactFixture(t)
	out := f.create(t)
	f.approve(t, out)
	p := &gatedArtifact{artifactExecutor: f.h.ArtifactPackager, entered: make(chan struct{}), release: make(chan struct{})}
	f.h.ArtifactPackager = p
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		f.h.ExecuteArtifactPackage(w, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}))
		done <- w
	}()
	select {
	case <-p.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("adapter never entered")
	}
	replay := f.execute(t, out, 200)
	if replay.Status != "unknown" {
		t.Fatal("claim must be durable before adapter")
	}
	testutil.Call(t, f.h.CancelAgentApproval, f.route(out.Approval.ID, "cancel", nil)).Want(409)
	close(p.release)
	w := <-done
	if w.Code != 200 {
		t.Fatalf("execute=%d %s", w.Code, w.Body.String())
	}
	if p.calls.Load() != 1 {
		t.Fatal("adapter replayed")
	}
	f.execute(t, out, 200)
	if p.calls.Load() != 1 {
		t.Fatal("terminal replay invoked adapter")
	}
}
func TestControlledArtifactUnknownNeverRetries(t *testing.T) {
	f := newArtifactFixture(t)
	out := f.create(t)
	f.approve(t, out)
	p := &gatedArtifact{artifactExecutor: f.h.ArtifactPackager, entered: make(chan struct{}), release: make(chan struct{}), panic: true}
	close(p.release)
	f.h.ArtifactPackager = p
	result := f.execute(t, out, 503)
	if result.Status != "unknown" || result.Approval.Status != "approved" {
		t.Fatal(result)
	}
	f.execute(t, out, 200)
	if p.calls.Load() != 1 {
		t.Fatal("unknown executed again")
	}
}
func TestControlledArtifactAuthorityLocks(t *testing.T) {
	for _, target := range []string{"workspace", "member", "user", "task", "agent"} {
		t.Run(target, func(t *testing.T) {
			f := newArtifactFixture(t)
			out := f.create(t)
			f.approve(t, out)
			ctx := context.Background()
			tx, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			var stmt string
			var id string
			switch target {
			case "workspace":
				stmt = `SELECT id FROM workspace WHERE id=$1 FOR UPDATE`
				id = f.ws
			case "member":
				stmt = `SELECT id FROM member WHERE workspace_id=$1 AND user_id='` + f.actor + `' FOR UPDATE`
				id = f.ws
			case "user":
				stmt = `SELECT id FROM "user" WHERE id=$1 FOR UPDATE`
				id = f.actor
			case "task":
				stmt = `SELECT id FROM agent_task_queue WHERE id=$1 FOR UPDATE`
				id = f.req.TaskID
			case "agent":
				stmt = `SELECT id FROM agent WHERE id=$1 FOR UPDATE`
				id = f.req.AgentID
			}
			if _, err = tx.Exec(ctx, stmt, id); err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(409)
			if time.Since(start) > 2*time.Second {
				t.Fatal("authority lock unexpectedly waited")
			}
			if dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NOT NULL`, out.Approval.ID) != 0 {
				t.Fatal("busy claim persisted")
			}
		})
	}
}

func (f *artifactFixture) deleteWorkspace(t *testing.T) {
	t.Helper()
	testutil.Call(t, f.h.DeleteWorkspace, testutil.WithURLParams(f.request("DELETE", "/api/workspaces/"+f.ws, nil), "id", f.ws)).Want(204)
}
func TestControlledArtifactWorkspaceDeletion(t *testing.T) {
	for _, state := range []string{"pending", "approved", "succeeded", "unknown", "failed", "cancelled", "rejected", "inflight"} {
		t.Run(state, func(t *testing.T) {
			f := newArtifactFixture(t, true)
			out := f.create(t)
			legacy := dbfx.Insert(t, "agent_approval_request", testutil.Cols{"workspace_id": f.ws, "agent_id": f.req.AgentID, "risk_class": "production_release", "summary": "legacy"})
			if state != "pending" && state != "cancelled" && state != "rejected" {
				f.approve(t, out)
			}
			var done chan *httptest.ResponseRecorder
			var pack *gatedArtifact
			switch state {
			case "cancelled":
				testutil.Call(t, f.h.CancelAgentApproval, f.route(out.Approval.ID, "cancel", nil)).Want(200)
			case "rejected":
				testutil.Call(t, f.h.DecideAgentApproval, f.route(out.Approval.ID, "decision", map[string]string{"decision": "reject", "expected_spec_digest": out.SpecDigest})).Want(200)
			case "succeeded":
				f.execute(t, out, 200)
			case "unknown":
				pack = &gatedArtifact{artifactExecutor: f.h.ArtifactPackager, entered: make(chan struct{}), release: make(chan struct{}), panic: true}
				close(pack.release)
				f.h.ArtifactPackager = pack
				f.execute(t, out, 503)
			case "failed":
				f.h.ArtifactPackager = &failedArtifact{artifactExecutor: f.h.ArtifactPackager}
				if f.execute(t, out, 200).Status != "failed" {
					t.Fatal("failed adapter not retained")
				}
			case "inflight":
				pack = &gatedArtifact{artifactExecutor: f.h.ArtifactPackager, entered: make(chan struct{}), release: make(chan struct{})}
				f.h.ArtifactPackager = pack
				done = make(chan *httptest.ResponseRecorder, 1)
				go func() {
					w := httptest.NewRecorder()
					f.h.ExecuteArtifactPackage(w, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}))
					done <- w
				}()
				select {
				case <-pack.entered:
				case <-time.After(5 * time.Second):
					t.Fatal("no definite claim")
				}
			}
			before, err := f.h.Queries.GetAgentApprovalRequest(context.Background(), parseUUID(out.Approval.ID))
			if err != nil {
				t.Fatal(err)
			}
			f.deleteWorkspace(t)
			if state == "inflight" {
				close(pack.release)
				w := <-done
				if w.Code != 200 {
					t.Fatalf("finalization after deletion: %d %s", w.Code, w.Body.String())
				}
				if pack.calls.Load() != 1 {
					t.Fatal("multiple side effects")
				}
			}
			after, err := f.h.Queries.GetAgentApprovalRequest(context.Background(), parseUUID(out.Approval.ID))
			if err != nil {
				t.Fatal("approval lost", err)
			}
			if before.DecidedBy != after.DecidedBy || before.DecidedAt != after.DecidedAt || before.DecisionNote != after.DecisionNote {
				t.Fatal("human decision lost")
			}
			if dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1`, out.Approval.ID) != 1 {
				t.Fatal("receipt lost")
			}
			if dbfx.Count(t, `SELECT count(*) FROM agent_approval_request WHERE id=$1`, legacy) != 0 {
				t.Fatal("legacy cleanup changed")
			}
			if state == "inflight" && after.Status != "executed" {
				t.Fatal("success and legacy marker were not atomic")
			}
			testutil.Call(t, f.h.GetArtifactPackage, f.route(out.Approval.ID, "artifact-package", nil)).Want(404)
			testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(404)
		})
	}
}

type failedArtifact struct{ artifactExecutor }

func (p *failedArtifact) Pack(context.Context, string, artifact.Spec) artifact.Outcome {
	return artifact.Outcome{Status: "failed", Code: "input_changed"}
}

func TestControlledArtifactStoredBindingDrift(t *testing.T) {
	for _, column := range []string{"source_revision", "input_manifest_digest", "staging_manifest_digest", "invocation_digest", "root_config_identity"} {
		t.Run(column, func(t *testing.T) {
			f := newArtifactFixture(t)
			out := f.create(t)
			f.approve(t, out)
			value := strings.Repeat("b", 64)
			if column == "source_revision" {
				value = strings.Repeat("b", 40)
			}
			dbfx.Exec(t, `UPDATE agent_artifact_execution SET `+column+`=$1 WHERE approval_id=$2`, value, out.Approval.ID)
			testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(409)
			if dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NOT NULL`, out.Approval.ID) != 0 {
				t.Fatal("corrupt binding claimed")
			}
		})
	}
}
func TestControlledArtifactJWTExpiredAtWrite(t *testing.T) {
	f := newArtifactFixture(t)
	out := f.create(t)
	f.approve(t, out)
	req := f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})
	req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(-time.Second)}))
	testutil.Call(t, f.h.ExecuteArtifactPackage, req).Want(403)
	if dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NOT NULL`, out.Approval.ID) != 0 {
		t.Fatal("expired credential claimed")
	}
}

type artifactCounted struct {
	artifactExecutor
	calls atomic.Int32
}

func (p *artifactCounted) Pack(ctx context.Context, id string, s artifact.Spec) artifact.Outcome {
	p.calls.Add(1)
	return p.artifactExecutor.Pack(ctx, id, s)
}

type artifactFaultStarter struct {
	txStarter
	count   int
	failAt  int
	after   bool
	between bool
}

func (s *artifactFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	s.count++
	return &artifactFaultTx{Tx: tx, fail: s.count == s.failAt, after: s.after, between: s.between}, nil
}

type artifactFaultTx struct {
	pgx.Tx
	fail, after, between bool
}

func (t *artifactFaultTx) Commit(ctx context.Context) error {
	if !t.fail || t.between {
		return t.Tx.Commit(ctx)
	}
	if t.after {
		if err := t.Tx.Commit(ctx); err != nil {
			return err
		}
	}
	return errors.New("injected ambiguous commit")
}
func (t *artifactFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.fail && t.between && strings.Contains(sql, "-- name: MarkAgentApprovalRequestExecuted") {
		return artifactErrorRow{}
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}

type artifactErrorRow struct{}

func (artifactErrorRow) Scan(...any) error {
	return errors.New("injected failure between receipt and approval")
}
func TestControlledArtifactCommitUncertainty(t *testing.T) {
	for _, mode := range []string{"claim_before", "claim_after", "receipt_before", "receipt_after", "between_receipt_and_approval"} {
		t.Run(mode, func(t *testing.T) {
			f := newArtifactFixture(t)
			out := f.create(t)
			f.approve(t, out)
			p := &artifactCounted{artifactExecutor: f.h.ArtifactPackager}
			f.h.ArtifactPackager = p
			at := 2
			if strings.HasPrefix(mode, "claim_") {
				at = 1
			}
			f.h.TxStarter = &artifactFaultStarter{txStarter: f.h.TxStarter, failAt: at, after: strings.HasSuffix(mode, "_after"), between: strings.HasPrefix(mode, "between_")}
			testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(503)
			row, err := f.h.Queries.GetAgentArtifactExecutionByApproval(context.Background(), parseUUID(out.Approval.ID))
			if err != nil {
				t.Fatal(err)
			}
			a, err := f.h.Queries.GetAgentApprovalRequest(context.Background(), row.ApprovalID)
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := int32(1)
			if at == 1 {
				wantCalls = 0
			}
			if p.calls.Load() != wantCalls {
				t.Fatal("ambiguous claim invoked adapter")
			}
			if mode == "receipt_after" {
				if row.ResultStatus.String != "succeeded" || a.Status != "executed" {
					t.Fatal("actual committed result unavailable")
				}
			} else if mode == "claim_before" {
				if row.ClaimedAt.Valid || a.Status != "approved" {
					t.Fatal("rolled back claim persisted")
				}
			} else {
				if row.ResultStatus.String != "unknown" || a.Status != "approved" {
					t.Fatal("uncertain outcome falsely finalized")
				}
			}
			if mode != "claim_before" {
				f.execute(t, out, 200)
				if p.calls.Load() != wantCalls {
					t.Fatal("uncertain result invoked adapter again")
				}
			}
		})
	}
}

func TestControlledArtifactSameKeyConcurrency(t *testing.T) {
	f := newArtifactFixture(t)
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			w := httptest.NewRecorder()
			f.h.CreateArtifactPackage(w, f.request("POST", "/api/agent-approvals/artifact-package", f.req))
			responses <- w
		}()
	}
	close(start)
	var successful artifactResponse
	for i := 0; i < 2; i++ {
		w := <-responses
		if w.Code != 201 && w.Code != 200 && w.Code != 409 {
			t.Fatalf("creation race status%d %s", w.Code, w.Body.String())
		}
		if w.Code != 409 {
			if err := json.Unmarshal(w.Body.Bytes(), &successful); err != nil {
				t.Fatal(err)
			}
		}
	}
	if successful.ExecutionID == "" {
		t.Fatal("no create won")
	}
	var replay artifactResponse
	testutil.Call(t, f.h.CreateArtifactPackage, f.request("POST", "/api/agent-approvals/artifact-package", f.req)).Want(200).JSON(&replay)
	if replay.ExecutionID != successful.ExecutionID || dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE task_id=$1`, f.req.TaskID) != 1 || dbfx.Count(t, `SELECT count(*) FROM agent_approval_request WHERE task_id=$1`, f.req.TaskID) != 1 {
		t.Fatal("duplicate creation or provisional approval leaked")
	}
}

func TestControlledArtifactSeparateApproverRevocation(t *testing.T) {
	f := newArtifactFixture(t)
	out := f.create(t)
	approver := dbfx.User(t, "Artifact approver", "artifact-approver-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, f.ws, approver, "admin")
	req := f.route(out.Approval.ID, "decision", map[string]string{"decision": "approve", "expected_spec_digest": out.SpecDigest})
	req.Header.Set("X-User-ID", approver)
	req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: approver, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
	// Management is insufficient to invoke a private agent; explicitly grant this approver access.
	dbfx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, f.req.AgentID)
	dbfx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.req.AgentID, "target_type": "member", "target_id": approver})
	testutil.Call(t, f.h.DecideAgentApproval, req).Want(200)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.ws, approver); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(409)
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(403)
}

func TestControlledArtifactCredentialRevocation(t *testing.T) {
	for _, kind := range []string{"password", "pat", "task_token"} {
		t.Run(kind, func(t *testing.T) {
			f := newArtifactFixture(t, true)
			out := f.create(t)
			f.approve(t, out)
			ctx := context.Background()
			req := f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})
			var token db.TaskToken
			switch kind {
			case "password":
				dbfx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": f.actor, "username": "artifact" + strings.ReplaceAll(f.actor, "-", ""), "password_hash": "test-only", "session_version": int64(1), "must_change_password": false}, "user_id=$1", f.actor)
				req = req.WithContext(auth.WithPasswordSession(req.Context(), auth.PasswordSession{UserID: f.actor, Version: 1, Kind: "jwt"}))
			case "pat":
				hash := auth.HashToken(uuid.NewString())
				dbfx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.actor, "name": "artifact", "token_hash": hash, "token_prefix": "synthetic", "revoked": false})
				req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialPAT, PATTokenHash: hash}))
			case "task_token":
				var err error
				token, err = f.h.Queries.CreateTaskToken(ctx, db.CreateTaskTokenParams{TokenHash: auth.HashToken(uuid.NewString()), TaskID: parseUUID(f.req.TaskID), AgentID: parseUUID(f.req.AgentID), WorkspaceID: parseUUID(f.ws), UserID: parseUUID(f.actor), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}})
				if err != nil {
					t.Fatal(err)
				}
				dbfx.Cleanup(t, `DELETE FROM task_token WHERE id=$1`, uuidToString(token.ID))
				req = req.WithContext(auth.WithVerifiedTaskToken(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialTask}), token))
			}
			tx, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			switch kind {
			case "password":
				if _, err = tx.Exec(ctx, `SELECT id FROM "user" WHERE id=$1 FOR UPDATE`, f.actor); err == nil {
					_, err = tx.Exec(ctx, `UPDATE user_password_credential SET session_version=2 WHERE user_id=$1`, f.actor)
				}
			case "pat":
				_, err = tx.Exec(ctx, `UPDATE personal_access_token SET revoked=true WHERE user_id=$1`, f.actor)
			case "task_token":
				_, err = tx.Exec(ctx, `DELETE FROM task_token WHERE id=$1`, token.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			testutil.Call(t, f.h.ExecuteArtifactPackage, req).Want(409)
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			want := 403
			if kind != "password" {
				want = 404
			}
			testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}).WithContext(req.Context())).Want(want)
			if dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NOT NULL`, out.Approval.ID) != 0 {
				t.Fatal("revoked credential claimed")
			}
		})
	}
}

// Reverse order: actual authority locks prevent revocation committing before admission.
func TestControlledArtifactAdmissionFencesLaterRevocation(t *testing.T) {
	for _, kind := range []string{"membership", "password", "workspace"} {
		t.Run(kind, func(t *testing.T) {
			f := newArtifactFixture(t, true)
			out := f.create(t)
			f.approve(t, out)
			ctx := context.Background()
			tx, q, err := f.h.artifactTx(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			row, spec, err := f.h.artifactLoad(f.route(out.Approval.ID, "artifact-package", nil))
			if err != nil {
				t.Fatal(err)
			}
			req := f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})
			if _, err = f.h.artifactAuthority(req, q, tx, spec.Request, &row, f.actor, true, true); err != nil {
				t.Fatal(err)
			}
			writer, err := testPool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Release()
			pid := int32(writer.Conn().PgConn().PID())
			result := make(chan error, 1)
			query := `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`
			if kind == "password" {
				query = `UPDATE "user" SET disabled_at=now() WHERE $1::uuid IS NOT NULL AND id=$2`
			}
			if kind == "workspace" {
				query = `SELECT id FROM workspace WHERE id=$1 AND $2::uuid IS NOT NULL FOR UPDATE`
			}
			go func() { _, e := writer.Exec(ctx, query, f.ws, f.actor); result <- e }()
			waitForBlockedWriter(t, ctx, pid)
			if _, err = q.LockAgentApprovalRequest(ctx, row.ApprovalID); err != nil {
				t.Fatal(err)
			}
			if _, err = q.LockAgentArtifactExecutionByApproval(ctx, row.ApprovalID); err != nil {
				t.Fatal(err)
			}
			if _, err = q.ClaimAgentArtifactExecution(ctx, db.ClaimAgentArtifactExecutionParams{ExecutionID: row.ExecutionID, SpecDigest: row.SpecDigest, ClaimNonce: dbid.NewV7(), ClaimedKind: pgtype.Text{String: "human", Valid: true}, ClaimedBy: parseUUID(f.actor)}); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = <-result; err != nil {
				t.Fatal(err)
			}
			if dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND result_status='unknown'`, row.ApprovalID) != 1 {
				t.Fatal("admitted claim lost")
			}
		})
	}
}

func TestControlledArtifactAgentReadRechecksInvocation(t *testing.T) {
	f := newArtifactFixture(t, true)
	origin := dbfx.User(t, "Distinct originator", "artifact-origin-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, f.ws, origin, "member")
	dbfx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, f.req.AgentID)
	target := dbfx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.req.AgentID, "target_type": "member", "target_id": origin})
	dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, origin, f.req.TaskID)
	out := f.create(t)
	f.approve(t, out)
	physical := &artifactCounted{artifactExecutor: f.h.ArtifactPackager}
	f.h.ArtifactPackager = physical
	revoked, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = revoked.Exec(context.Background(), `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.ws, origin); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(409)
	if err = revoked.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(404)
	if physical.calls.Load() != 0 {
		t.Fatal("revoked originator physically invoked adapter")
	}
	dbfx.Member(t, f.ws, origin, "member")
	f.execute(t, out, 200)
	token, err := f.h.Queries.CreateTaskToken(context.Background(), db.CreateTaskTokenParams{TokenHash: auth.HashToken(uuid.NewString()), TaskID: parseUUID(f.req.TaskID), AgentID: parseUUID(f.req.AgentID), WorkspaceID: parseUUID(f.ws), UserID: parseUUID(f.actor), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM task_token WHERE id=$1`, token.ID)
	agentRequest := func(action string) *http.Request {
		req := f.route(out.Approval.ID, action, map[string]string{"expected_spec_digest": out.SpecDigest})
		return req.WithContext(auth.WithVerifiedTaskToken(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialTask}), token))
	}
	testutil.Call(t, f.h.GetArtifactPackage, agentRequest("artifact-package")).Want(200)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `SELECT id FROM agent WHERE id=$1 FOR UPDATE`, f.req.AgentID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `DELETE FROM agent_invocation_target WHERE id=$1`, target); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, f.h.GetArtifactPackage, agentRequest("artifact-package")).Want(403)
	testutil.Call(t, f.h.ExecuteArtifactPackage, agentRequest("artifact-package/execution")).Want(403)
	// The authorized human manager retains audit access without granting agent execution.
	testutil.Call(t, f.h.GetArtifactPackage, f.route(out.Approval.ID, "artifact-package", nil)).Want(200)
}
func TestControlledArtifactFinalizationLockConflict(t *testing.T) {
	f := newArtifactFixture(t)
	out := f.create(t)
	f.approve(t, out)
	p := &gatedArtifact{artifactExecutor: f.h.ArtifactPackager, entered: make(chan struct{}), release: make(chan struct{})}
	f.h.ArtifactPackager = p
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		f.h.ExecuteArtifactPackage(w, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}))
		done <- w
	}()
	<-p.entered
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `SELECT id FROM agent_approval_request WHERE id=$1 FOR UPDATE`, out.Approval.ID); err != nil {
		t.Fatal(err)
	}
	close(p.release)
	w := <-done
	if w.Code != 503 {
		t.Fatalf("finalization contention=%d %s", w.Code, w.Body.String())
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	replay := f.execute(t, out, 200)
	if replay.Status != "unknown" || replay.Approval.Status != "approved" || p.calls.Load() != 1 {
		t.Fatal("uncertain result was retried or reported succeeded")
	}
}

type artifactBarrierStarter struct {
	txStarter
	entered, release chan struct{}
	count            int
}

func (s *artifactBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	s.count++
	if s.count == 1 {
		return &artifactBarrierTx{Tx: tx, entered: s.entered, release: s.release}, nil
	}
	return tx, nil
}

type artifactBarrierTx struct {
	pgx.Tx
	entered, release chan struct{}
}

func (t *artifactBarrierTx) Commit(ctx context.Context) error {
	close(t.entered)
	<-t.release
	return t.Tx.Commit(ctx)
}
func TestControlledArtifactActualAdmissionBeforeRevocation(t *testing.T) {
	for _, kind := range []string{"distinct_initiator_membership", "current_claimer_membership", "originator_membership", "runtime_owner_membership", "approver_membership", "password", "pat", "task_token", "workspace"} {
		t.Run(kind, func(t *testing.T) {
			f := newArtifactFixture(t, true)
			owner := f.actor
			human := dbfx.User(t, "Artifact manager", "artifact-manager-"+uuid.NewString()+"@example.invalid")
			dbfx.Member(t, f.ws, human, "admin")
			dbfx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, f.req.AgentID)
			dbfx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.req.AgentID, "target_type": "member", "target_id": human})
			f.actor = human
			out := f.create(t)
			f.actor = owner
			f.approve(t, out)
			if kind != "distinct_initiator_membership" {
				f.actor = human
			}
			req := f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})
			if kind == "password" {
				dbfx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": human, "username": "artifact" + strings.ReplaceAll(human, "-", ""), "password_hash": "test-only", "session_version": int64(1), "must_change_password": false}, "user_id=$1", human)
				req = req.WithContext(auth.WithPasswordSession(req.Context(), auth.PasswordSession{UserID: human, Version: 1, Kind: "jwt"}))
			}
			var taskToken db.TaskToken
			revokeUser := human
			if kind == "originator_membership" || kind == "runtime_owner_membership" || kind == "approver_membership" {
				revokeUser = owner
			}
			if kind == "pat" {
				hash := auth.HashToken(uuid.NewString())
				dbfx.Insert(t, "personal_access_token", testutil.Cols{"user_id": human, "name": "artifact", "token_hash": hash, "token_prefix": "synthetic", "revoked": false})
				req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: human, Kind: auth.CredentialPAT, PATTokenHash: hash}))
			}
			if kind == "task_token" {
				var e error
				taskToken, e = f.h.Queries.CreateTaskToken(context.Background(), db.CreateTaskTokenParams{TokenHash: auth.HashToken(uuid.NewString()), TaskID: parseUUID(f.req.TaskID), AgentID: parseUUID(f.req.AgentID), WorkspaceID: parseUUID(f.ws), UserID: parseUUID(owner), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}})
				if e != nil {
					t.Fatal(e)
				}
				dbfx.Cleanup(t, `DELETE FROM task_token WHERE id=$1`, taskToken.ID)
				req = req.WithContext(auth.WithVerifiedTaskToken(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: owner, Kind: auth.CredentialTask}), taskToken))
			}
			barrier := &artifactBarrierStarter{txStarter: f.h.TxStarter, entered: make(chan struct{}), release: make(chan struct{})}
			f.h.TxStarter = barrier
			pack := &artifactCounted{artifactExecutor: f.h.ArtifactPackager}
			f.h.ArtifactPackager = pack
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { w := httptest.NewRecorder(); f.h.ExecuteArtifactPackage(w, req); done <- w }()
			select {
			case <-barrier.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("actual Execute never reached precommit barrier")
			}
			writer, err := testPool.Acquire(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Release()
			pid := int32(writer.Conn().PgConn().PID())
			result := make(chan error, 1)
			query := `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`
			if kind == "password" {
				query = `UPDATE "user" SET disabled_at=now() WHERE $1::uuid IS NOT NULL AND id=$2`
			}
			if kind == "pat" {
				query = `UPDATE personal_access_token SET revoked=true WHERE $1::uuid IS NOT NULL AND user_id=$2`
			}
			if kind == "task_token" {
				query = `DELETE FROM task_token WHERE $1::uuid IS NOT NULL AND user_id=$2`
				revokeUser = owner
			}
			if kind == "workspace" {
				query = `SELECT id FROM workspace WHERE id=$1 AND $2::uuid IS NOT NULL FOR UPDATE`
			}
			go func() { _, e := writer.Exec(context.Background(), query, f.ws, revokeUser); result <- e }()
			waitForBlockedWriter(t, context.Background(), pid)
			if pack.calls.Load() != 0 {
				t.Fatal("adapter invoked before definite claim commit")
			}
			close(barrier.release)
			if err = <-result; err != nil {
				t.Fatal(err)
			}
			w := <-done
			if w.Code != 200 {
				t.Fatalf("actual admitted operation failed after revocation: %d %s", w.Code, w.Body.String())
			}
			if pack.calls.Load() != 1 {
				t.Fatal("admitted operation not executed exactly once")
			}
			var completed artifactResponse
			if err = json.Unmarshal(w.Body.Bytes(), &completed); err != nil {
				t.Fatal(err)
			}
			if completed.Status != "succeeded" || completed.Approval.Status != "executed" {
				t.Fatal("no atomic receipt")
			}
		})
	}
}

func TestControlledArtifactCreateBeforeWorkspaceDelete(t *testing.T) {
	f := newArtifactFixture(t, true)
	base := f.h
	barrier := &artifactBarrierStarter{txStarter: f.h.TxStarter, entered: make(chan struct{}), release: make(chan struct{})}
	f.h.TxStarter = barrier
	created := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		f.h.CreateArtifactPackage(w, f.request("POST", "/api/agent-approvals/artifact-package", f.req))
		created <- w
	}()
	select {
	case <-barrier.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("create never reached commit")
	}
	deleted := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		base.DeleteWorkspace(w, testutil.WithURLParams(f.request("DELETE", "/api/workspaces/"+f.ws, nil), "id", f.ws))
		deleted <- w
	}()
	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		var n int
		dbfx.QueryRow(t, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%LockWorkspaceForDelete%'`).Scan(&n)
		if n > 0 {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		close(barrier.release)
		t.Fatal("actual workspace deletion never waited on creation fence")
	}
	close(barrier.release)
	c := <-created
	d := <-deleted
	if c.Code != 201 || d.Code != 204 {
		t.Fatalf("create/delete=%d/%d %s %s", c.Code, d.Code, c.Body.String(), d.Body.String())
	}
	var out artifactResponse
	if err := json.Unmarshal(c.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if dbfx.Count(t, `SELECT count(*) FROM agent_approval_request WHERE id=$1`, out.Approval.ID) != 1 || dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NULL`, out.Approval.ID) != 1 {
		t.Fatal("delete missed or purged admitted binding")
	}
	testutil.Call(t, base.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(404)
}
