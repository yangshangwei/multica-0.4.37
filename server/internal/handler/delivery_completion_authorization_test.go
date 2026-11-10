package handler

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type completionFixture struct {
	*deliveryPolicyFixture
	request    service.DeliveryCompletionAuthorizationRequest
	credential service.ArtifactCredentialSnapshot
}

// Delay only the actual terminal INSERT; all SQL and scan destinations still go
// to PostgreSQL. This observes expiry after final checks, without faking success.
type completionTerminalDelayTx struct {
	pgx.Tx
	until time.Time
	seen  bool
}

func (tx *completionTerminalDelayTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "INSERT INTO delivery_completion_terminal") {
		tx.seen = true
		// Use the same database clock as admission. The delay ends just after the
		// bound instant and does not alter the authorization or credential row.
		_, err := tx.Tx.Exec(ctx, `SELECT pg_sleep(GREATEST(0, EXTRACT(EPOCH FROM ($1::timestamptz-clock_timestamp()))) + 0.02)`, tx.until)
		if err != nil {
			return completionTerminalDelayError{err}
		}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

type completionTerminalDelayError struct{ err error }

func (e completionTerminalDelayError) Scan(...any) error { return e.err }

func newCompletionFixture(t *testing.T) *completionFixture {
	t.Helper()
	f := newDeliveryPolicyFixture(t, false)
	// Internal P1 fixture only: the public policy API remains off/observe until
	// every writer is integrated. The artifact itself came from real packaging.
	dbfx.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": f.ws, "project_id": f.project, "revision": 1, "policy_version": deliveryEvidencePolicy, "mode": "enforce", "stage": "done", "reason": "P1 internal gate fixture", "created_by": f.actor}, "workspace_id=$1 AND project_id=$2", f.ws, f.project)
	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.artifactFixture.req.ResourceID).Scan(&status)
	request := service.DeliveryCompletionAuthorizationRequest{ContractVersion: service.DeliveryCompletionContract, SourceWorkspaceID: f.ws, OperationID: uuid.NewString(), Target: service.DeliveryCompletionTarget{Kind: "ordinary", IssueID: f.artifactFixture.req.ResourceID, ExpectedIssueRevision: f.req.IssueRevision, ExpectedStatus: status, TargetStatus: "done"}, Project: service.DeliveryCompletionProject{ID: f.project, ExpectedRevision: *f.req.ProjectRevision, ExpectedPolicyRevision: 1}, Evidence: service.DeliveryCompletionEvidence{IssueID: f.artifactFixture.req.ResourceID, ExpectedIssueRevision: f.req.IssueRevision, ApprovalID: f.req.ApprovalID, ExpectedSourceRevision: f.req.SourceRevision, ExpectedSpecDigest: f.req.SpecDigest, ExpectedArtifactDigest: f.req.ArtifactDigest}, Consumer: service.DeliveryCompletionConsumer{Kind: "human", UserID: f.actor}, ExpiresAt: time.Now().UTC().Add(2 * time.Minute).Truncate(time.Microsecond), Reason: "Accept exact packaged candidate"}
	r := f.request("POST", "/unused", nil)
	principal, _ := auth.AuthenticatedPrincipalFromContext(r.Context())
	return &completionFixture{f, request, artifactCredentialSnapshot(r.Context(), principal)}
}
func (f *completionFixture) create(t *testing.T, key string, status int) service.DeliveryCompletionAuthorizationView {
	t.Helper()
	r := f.artifactFixture.request("POST", "/api/delivery-completion-authorizations", f.request)
	r.Header.Set("Idempotency-Key", key)
	var out service.DeliveryCompletionAuthorizationView
	testutil.Call(t, f.h.CreateDeliveryCompletionAuthorization, r).Want(status).JSON(&out)
	return out
}
func (f *completionFixture) get(t *testing.T, id string, status int) service.DeliveryCompletionAuthorizationView {
	t.Helper()
	r := testutil.WithURLParams(f.artifactFixture.request("GET", "/api/delivery-completion-authorizations/"+id, nil), "authorizationId", id)
	var out service.DeliveryCompletionAuthorizationView
	testutil.Call(t, f.h.GetDeliveryCompletionAuthorization, r).Want(status).JSON(&out)
	return out
}
func (f *completionFixture) prepare(t *testing.T, tx pgx.Tx, grant service.DeliveryCompletionAuthorizationView) (service.DeliveryPrepareResult, db.Issue, error) {
	t.Helper()
	q := db.New(tx)
	before, err := q.LockLifecycleIssue(context.Background(), db.LockLifecycleIssueParams{ID: parseUUID(f.request.Target.IssueID), WorkspaceID: parseUUID(f.ws)})
	if err != nil {
		return service.DeliveryPrepareResult{}, before, err
	}
	after := before
	after.Status = "done"
	out, err := service.PrepareDeliveryMutationInTx(context.Background(), tx, service.DeliveryMutationIntent{Before: before, Candidate: after, Completion: &service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: f.request.OperationID, ExpectedIssueRevision: f.request.Target.ExpectedIssueRevision}, Credential: f.credential})
	return out, before, err
}
func TestDeliveryCompletionAuthorizationActualArtifactAtomicConsumeAndReplay(t *testing.T) {
	f := newCompletionFixture(t)
	key := uuid.NewString()
	grant := f.create(t, key, 201)
	if grant.State != "issued" || grant.AuthorizationID == "" || grant.CompletionReceipt != nil {
		t.Fatal(grant)
	}
	replay := f.create(t, key, 200)
	if replay.AuthorizationID != grant.AuthorizationID || !replay.Replayed {
		t.Fatal("issue retry changed original authorization")
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	prepared, before, err := f.prepare(t, tx, grant)
	if err != nil || prepared.State != service.DeliveryPrepared {
		t.Fatalf("prepare=%+v err=%v", prepared, err)
	}
	after, err := db.New(tx).UpdateIssueStatus(context.Background(), db.UpdateIssueStatusParams{ID: before.ID, WorkspaceID: before.WorkspaceID, Status: "done"})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := service.FinishDeliveryMutationInTx(context.Background(), tx, prepared.Prepared, after, service.DeliveryWriterResult{})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if receipt.OperationID != f.request.OperationID || receipt.CompletedRevision != after.Revision {
		t.Fatal(receipt)
	}
	if dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed'`, grant.AuthorizationID) != 1 {
		t.Fatal("missing single atomic consumption")
	}
	// Reopen and edit afterward: original retry must recover the old completion,
	// never write done again or overwrite today's content.
	dbfx.Exec(t, `UPDATE issue SET status='todo',title='newer draft',revision=revision+1 WHERE id=$1`, f.request.Target.IssueID)
	tx2, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.Rollback(context.Background())
	replayed, current, err := f.prepare(t, tx2, grant)
	if err != nil || replayed.State != service.DeliveryReplayed || replayed.Receipt.CompletedRevision != after.Revision {
		t.Fatalf("replay=%+v current=%+v err=%v", replayed, current, err)
	}
	if err = tx2.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	var status, title string
	dbfx.QueryRow(t, `SELECT status,title FROM issue WHERE id=$1`, f.request.Target.IssueID).Scan(&status, &title)
	if status != "todo" || title != "newer draft" {
		t.Fatal("recovery rewrote current state")
	}
	view := f.get(t, grant.AuthorizationID, 200)
	if view.State != "consumed" || view.CompletionReceipt == nil {
		t.Fatal(view)
	}
	raw, _ := json.Marshal(view)
	for _, private := range []string{"observation", "issuer_pat", "spec_digest", "producer_task", "request_json"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("private data in minimal response")
		}
	}
}
func TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant(t *testing.T) {
	for _, mutation := range []string{"policy", "head", "issuer_revoked", "expired", "content", "wrong_consumer", "wrong_operation"} {
		t.Run(mutation, func(t *testing.T) {
			f := newCompletionFixture(t)
			grant := f.create(t, uuid.NewString(), 201)
			switch mutation {
			case "policy":
				dbfx.Exec(t, `INSERT INTO project_delivery_policy(workspace_id,project_id,revision,policy_version,mode,stage,reason,created_by) VALUES($1,$2,2,$3,'off','done','disabled',$4)`, f.ws, f.project, deliveryEvidencePolicy, f.actor)
			case "head":
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
			case "issuer_revoked":
				dbfx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, f.actor)
			case "expired":
				f.credential.Principal.ExpiresAt = time.Now().Add(-time.Minute)
			case "wrong_consumer":
				other := dbfx.User(t, "Other completion actor", "completion-"+uuid.NewString()+"@example.invalid")
				dbfx.Member(t, f.ws, other, "member")
				f.credential.Principal.UserID = other
			case "wrong_operation":
				f.request.OperationID = uuid.NewString()
			}
			tx, err := testPool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if mutation == "content" {
				q := db.New(tx)
				before, err := q.GetIssue(context.Background(), parseUUID(f.request.Target.IssueID))
				if err != nil {
					t.Fatal(err)
				}
				after := before
				after.Status = "done"
				after.Title = "unreviewed"
				_, err = service.PrepareDeliveryMutationInTx(context.Background(), tx, service.DeliveryMutationIntent{Before: before, Candidate: after, Completion: &service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: f.request.OperationID, ExpectedIssueRevision: before.Revision}, Credential: f.credential})
				if err == nil {
					t.Fatal("mixed content completion admitted")
				}
			} else if _, _, err = f.prepare(t, tx, grant); err == nil {
				t.Fatal("invalid live authority/evidence admitted")
			}
			if err = tx.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
			if dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
				t.Fatal("refusal consumed grant")
			}
			var state string
			dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.request.Target.IssueID).Scan(&state)
			if state == "done" {
				t.Fatal("refusal completed issue")
			}
		})
	}
}
func TestDeliveryCompletionAuthorizationRollbackRevokeAndTransactionIdentity(t *testing.T) {
	f := newCompletionFixture(t)
	grant := f.create(t, uuid.NewString(), 201)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	prepared, before, err := f.prepare(t, tx, grant)
	if err != nil {
		t.Fatal(err)
	}
	after, err := db.New(tx).UpdateIssueStatus(context.Background(), db.UpdateIssueStatusParams{ID: before.ID, WorkspaceID: before.WorkspaceID, Status: "done"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback(context.Background())
	if _, err = service.FinishDeliveryMutationInTx(context.Background(), other, prepared.Prepared, after, service.DeliveryWriterResult{}); err == nil {
		t.Fatal("prepared authorization moved to a different transaction")
	}
	if _, err = service.FinishDeliveryMutationInTx(context.Background(), tx, prepared.Prepared, after, service.DeliveryWriterResult{}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
		t.Fatal("rolled-back consume persisted")
	}
	key := uuid.NewString()
	r := testutil.WithURLParams(f.artifactFixture.request("POST", "/api/delivery-completion-authorizations/"+grant.AuthorizationID+"/revocations", map[string]string{"reason": "withdraw acceptance"}), "authorizationId", grant.AuthorizationID)
	r.Header.Set("Idempotency-Key", key)
	var revoked service.DeliveryCompletionAuthorizationView
	testutil.Call(t, f.h.RevokeDeliveryCompletionAuthorization, r).Want(200).JSON(&revoked)
	if revoked.State != "revoked" {
		t.Fatal(revoked)
	}
	retry := testutil.WithURLParams(f.artifactFixture.request("POST", "/api/delivery-completion-authorizations/"+grant.AuthorizationID+"/revocations", map[string]string{"reason": "withdraw acceptance"}), "authorizationId", grant.AuthorizationID)
	retry.Header.Set("Idempotency-Key", key)
	testutil.Call(t, f.h.RevokeDeliveryCompletionAuthorization, retry).Want(200)
	attempt, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer attempt.Rollback(context.Background())
	if _, _, err = f.prepare(t, attempt, grant); err == nil {
		t.Fatal("revoked grant consumed")
	}
}
func TestDeliveryCompletionAuthorizationMissingBindingAndCreatePolicy(t *testing.T) {
	f := newCompletionFixture(t)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	before, err := q.GetIssue(context.Background(), parseUUID(f.request.Target.IssueID))
	if err != nil {
		t.Fatal(err)
	}
	after := before
	after.Status = "done"
	_, err = service.PrepareDeliveryMutationInTx(context.Background(), tx, service.DeliveryMutationIntent{Before: before, Candidate: after, Credential: f.credential})
	var gate service.DeliveryCompletionError
	if !errors.As(err, &gate) || gate.Status != 428 {
		t.Fatalf("missing binding err=%v", err)
	}
	if err = service.CheckDeliveryCreateInTx(context.Background(), tx, before.WorkspaceID, before.ProjectID, "done"); err == nil {
		t.Fatal("enforce created done")
	}
	if err = service.CheckDeliveryCreateInTx(context.Background(), tx, before.WorkspaceID, before.ProjectID, "todo"); err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryCompletionAuthorizationNoGateForArchivedDoneEditing(t *testing.T) {
	f := newCompletionFixture(t)
	custom := deliveryHintCustomStatus(t, f.deliveryPolicyFixture, "archived_complete", "done")
	dbfx.Exec(t, `UPDATE issue_status SET archived_at=clock_timestamp() WHERE id=$1`, custom)
	dbfx.Exec(t, `UPDATE issue SET status='archived_complete',revision=revision+1 WHERE id=$1`, f.request.Target.IssueID)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	before, err := db.New(tx).GetIssue(context.Background(), parseUUID(f.request.Target.IssueID))
	if err != nil {
		t.Fatal(err)
	}
	after := before
	after.Title = "later clarification"
	result, err := service.PrepareDeliveryMutationInTx(context.Background(), tx, service.DeliveryMutationIntent{Before: before, Candidate: after})
	if err != nil || result.State != service.DeliveryNotApplicable {
		t.Fatalf("archived done edit incorrectly became new admission: %+v %v", result, err)
	}
}
func TestDeliveryCompletionAuthorizationTaskAndConsumerIsolation(t *testing.T) {
	f := newCompletionFixture(t)
	f.request.Consumer = service.DeliveryCompletionConsumer{Kind: "task", AgentID: f.artifactFixture.req.AgentID, TaskID: f.artifactFixture.req.TaskID, TaskGeneration: 1}
	grant := f.create(t, uuid.NewString(), 201)
	token, err := f.h.Queries.CreateTaskToken(context.Background(), db.CreateTaskTokenParams{TokenHash: auth.HashToken(uuid.NewString()), TaskID: parseUUID(f.request.Consumer.TaskID), AgentID: parseUUID(f.request.Consumer.AgentID), WorkspaceID: parseUUID(f.ws), UserID: parseUUID(f.actor), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbfx.Exec(t, `DELETE FROM task_token WHERE id=$1`, token.ID) })
	// A normal human cannot consume a task's grant merely by owning that agent.
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.prepare(t, tx, grant); err == nil {
		t.Fatal("human adopted task grant")
	}
	tx.Rollback(context.Background())
	f.credential = service.ArtifactCredentialSnapshot{Principal: auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialTask}, TaskToken: &token}
	tx, err = testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	result, before, err := f.prepare(t, tx, grant)
	if err != nil {
		t.Fatal(err)
	}
	after, err := db.New(tx).UpdateIssueStatus(context.Background(), db.UpdateIssueStatusParams{ID: before.ID, WorkspaceID: before.WorkspaceID, Status: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.FinishDeliveryMutationInTx(context.Background(), tx, result.Prepared, after, service.DeliveryWriterResult{}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	var actor string
	dbfx.QueryRow(t, `SELECT actor_kind FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID).Scan(&actor)
	if actor != "task" {
		t.Fatal("consumer history impersonated issuer")
	}
}
func TestDeliveryCompletionAuthorizationConsumeRevokeAndEvidenceLocks(t *testing.T) {
	f := newCompletionFixture(t)
	grant := f.create(t, uuid.NewString(), 201)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	prepared, before, err := f.prepare(t, tx, grant)
	if err != nil {
		t.Fatal(err)
	}
	contender, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer contender.Rollback(context.Background())
	access := service.ArtifactAccess{Credential: f.credential, RequestedWorkspaceID: f.ws}
	if _, err = service.RevokeDeliveryCompletionAuthorizationInTx(context.Background(), contender, access, parseUUID(grant.AuthorizationID), parseUUID(uuid.NewString()), "concurrent withdrawal"); err == nil {
		t.Fatal("revocation bypassed held authorization")
	}
	contender.Rollback(context.Background())
	// Mutating a bound fact within the owner transaction must still fail at finish;
	// the actual state and the failed mutation must roll back together.
	after, err := db.New(tx).UpdateIssueStatus(context.Background(), db.UpdateIssueStatusParams{ID: before.ID, WorkspaceID: before.WorkspaceID, Status: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr); err != nil {
		t.Fatal(err)
	}
	if _, err = service.FinishDeliveryMutationInTx(context.Background(), tx, prepared.Prepared, after, service.DeliveryWriterResult{}); err == nil {
		t.Fatal("finish ignored changed source fact")
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
		t.Fatal("late refusal consumed")
	}
}

func TestDeliveryCompletionAuthorizationTerminalInsertRechecksExpiry(t *testing.T) {
	for _, boundary := range []string{"authorization", "consumer"} {
		t.Run(boundary, func(t *testing.T) {
			f := newCompletionFixture(t)
			credential := f.credential
			if boundary == "consumer" {
				consumer := dbfx.User(t, "Exact completion consumer", "late-consumer-"+uuid.NewString()+"@example.invalid")
				dbfx.Insert(t, "member", testutil.Cols{"workspace_id": f.ws, "user_id": consumer, "role": "member"})
				f.request.Consumer = service.DeliveryCompletionConsumer{Kind: "human", UserID: consumer}
				credential.Principal.UserID = consumer
			}
			var expiry time.Time
			dbfx.QueryRow(t, `SELECT clock_timestamp()+interval '2 seconds'`).Scan(&expiry)
			expiry = expiry.UTC().Truncate(time.Microsecond)
			if boundary == "authorization" {
				f.request.ExpiresAt = expiry
			} else {
				credential.Principal.ExpiresAt = expiry
			}
			grant := f.create(t, uuid.NewString(), 201)
			rawTx, err := testPool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer rawTx.Rollback(t.Context())
			tx := &completionTerminalDelayTx{Tx: rawTx, until: expiry}
			f.credential = credential
			prepared, before, err := f.prepare(t, tx, grant)
			if err != nil {
				t.Fatalf("positive setup did not reach preparation before expiry: %v", err)
			}
			after, err := db.New(tx).UpdateIssueStatus(t.Context(), db.UpdateIssueStatusParams{ID: before.ID, WorkspaceID: before.WorkspaceID, Status: "done"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.FinishDeliveryMutationInTx(t.Context(), tx, prepared.Prepared, after, service.DeliveryWriterResult{})
			if !tx.seen {
				t.Fatalf("test did not reach the actual terminal insert: %v", err)
			}
			if err == nil {
				t.Error("terminal insert accepted a completion after its authorization or actual consumer credential expired")
			}
			if err = tx.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
			var status string
			dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, before.ID).Scan(&status)
			if status != before.Status || dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
				t.Fatal("expired completion left state or terminal facts after rollback")
			}
		})
	}
}
