package handler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The artifact producer and the completion consumer have different tasks,
// agents, runtimes and owners. Contributor authority belongs to ordinary issue
// completion; it does not lower the Operator requirement on the actual producer.
func TestDeliveryCompletionConsumerDistinctContributor(t *testing.T) {
	f := newCompletionFixture(t)
	owner := dbfx.User(t, "Completion consumer owner", "completion-contributor-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, f.ws, owner, "member")
	fx := testutil.New(testPool, f.ws, owner)
	runtime := fx.Runtime(t, "Completion consumer runtime")
	agent := fx.Agent(t, "Completion contributor", runtime, testutil.Cols{"autonomy_level": "contributor"})
	task := fx.Task(t, agent, testutil.Cols{"status": "running", "issue_id": f.request.Target.IssueID, "runtime_id": runtime, "originator_user_id": owner, "accountable_user_id": owner, "claim_generation": 7})
	f.request.Consumer = service.DeliveryCompletionConsumer{Kind: "task", AgentID: agent, TaskID: task, TaskGeneration: 7}
	token, err := f.h.Queries.CreateTaskToken(context.Background(), db.CreateTaskTokenParams{TokenHash: auth.HashToken(uuid.NewString()), TaskID: parseUUID(task), AgentID: parseUUID(agent), WorkspaceID: parseUUID(f.ws), UserID: parseUUID(owner), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbfx.Exec(t, `DELETE FROM task_token WHERE id=$1`, token.ID) })
	grant := f.create(t, uuid.NewString(), 201)
	var producer, consumer string
	dbfx.QueryRow(t, `SELECT autonomy_level FROM agent WHERE id=$1`, f.artifactFixture.req.AgentID).Scan(&producer)
	dbfx.QueryRow(t, `SELECT autonomy_level FROM agent WHERE id=$1`, agent).Scan(&consumer)
	if producer != "operator" || consumer != "contributor" || agent == f.artifactFixture.req.AgentID || task == f.artifactFixture.req.TaskID {
		t.Fatal("fixture did not preserve distinct producer/consumer capabilities")
	}
	body := map[string]any{"status": "done", "expected_revision": f.request.Target.ExpectedIssueRevision, "completion": service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: grant.OperationID, ExpectedIssueRevision: f.request.Target.ExpectedIssueRevision}}
	request := testutil.WithURLParams(testutil.WithHeaders(testutil.JSONRequest("PUT", "/api/issues/"+f.request.Target.IssueID, body), "X-User-ID", owner, "X-Workspace-ID", f.ws, "X-Agent-ID", agent, "X-Task-ID", task), "id", f.request.Target.IssueID)
	request = request.WithContext(auth.WithVerifiedTaskToken(auth.WithAuthenticatedPrincipal(request.Context(), auth.AuthenticatedPrincipal{UserID: owner, Kind: auth.CredentialTask}), token))
	testutil.Call(t, f.h.UpdateIssue, request).Want(200)
	var status, actorKind, actorAgent string
	dbfx.QueryRow(t, `SELECT i.status,t.actor_kind,t.actor_agent_id::text FROM issue i JOIN delivery_completion_authorization a ON a.issue_id=i.id JOIN delivery_completion_terminal t ON t.authorization_id=a.id WHERE a.id=$1`, grant.AuthorizationID).Scan(&status, &actorKind, &actorAgent)
	if status != "done" || actorKind != "task" || actorAgent != agent {
		t.Fatalf("actual writer did not atomically record contributor completion: %s %s %s", status, actorKind, actorAgent)
	}
	if dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE execution_id=$1 AND result_status='succeeded'`, f.receipt.ExecutionID) != 1 {
		t.Fatal("completion replaced or re-executed original successful artifact")
	}
}
