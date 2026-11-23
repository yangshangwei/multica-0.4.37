package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type completionManagedRouterFixture struct {
	*deliveryPolicyRouterFixture
	owner, agent, task, runtime, token, tokenID, binding, installation string
	grant                                                              completionAuthorizationRouterResponse
	body                                                               map[string]any
}

// Existing controlled-artifact HTTP endpoints create, approve and physically
// package issuer A's Operator artifact. A separate Contributor task owned by B
// supplies the real managed task token to the ordinary UpdateIssue route.
func newCompletionManagedRouterFixture(t *testing.T, bind bool) *completionManagedRouterFixture {
	t.Helper()
	f := newDeliveryPolicyRouterFixture(t, "password", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	f.archive(t)
	owner := f.fx.User(t, "Managed completion owner", "managed-completion-"+uuid.NewString()+"@example.invalid")
	f.fx.Member(t, testWorkspaceID, owner, "member")
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": owner, "username": "completion-" + uuid.NewString(), "password_hash": "unused-by-task-token-fixture", "session_version": 1, "must_change_password": false}, "user_id=$1", owner)
	ownerFixture := testutil.New(testPool, testWorkspaceID, owner)
	runtime := ownerFixture.Runtime(t, "Managed completion runtime", testutil.Cols{"runtime_mode": "local", "daemon_id": uuid.NewString()})
	agent := ownerFixture.Agent(t, "Managed completion contributor", runtime, testutil.Cols{"autonomy_level": "contributor"})
	task := ownerFixture.Task(t, agent, testutil.Cols{"status": "running", "issue_id": f.request.ResourceID, "runtime_id": runtime, "originator_user_id": owner, "accountable_user_id": owner, "claim_generation": 7})
	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	tokenID := ownerFixture.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(token), "task_id": task, "agent_id": agent, "workspace_id": testWorkspaceID, "user_id": owner, "expires_at": time.Now().Add(time.Hour), "auth_version": 1})
	result := &completionManagedRouterFixture{deliveryPolicyRouterFixture: f, owner: owner, agent: agent, task: task, runtime: runtime, token: token, tokenID: tokenID}
	if bind {
		result.bind(t)
	}
	result.body = completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "task", "agent_id": agent, "task_id": task, "task_generation": 7})
	f.call(t, f.token, "POST", completionAuthorizationRouterPath, result.body, "Idempotency-Key", uuid.NewString()).Want(http.StatusCreated).JSON(&result.grant)
	var issuer, consumerTask, producer, storedBinding string
	var epoch *int64
	f.fx.QueryRow(t, `SELECT issuer_user_id::text,consumer_task_id::text,producer_agent_id::text,COALESCE(consumer_binding_id::text,''),consumer_binding_epoch FROM delivery_completion_authorization WHERE id=$1`, result.grant.AuthorizationID).Scan(&issuer, &consumerTask, &producer, &storedBinding, &epoch)
	if issuer != f.user || issuer == owner || consumerTask != task || producer != f.request.AgentID || producer == agent {
		t.Fatal("authorization did not preserve distinct issuer, producer and managed consumer")
	}
	if bind && (storedBinding != result.binding || epoch == nil || *epoch != 1) {
		t.Fatal("grant did not freeze the actual managed execution binding")
	}
	if !bind && (storedBinding != "" || epoch != nil) {
		t.Fatal("unmanaged grant did not freeze explicit binding absence")
	}
	return result
}
func (f *completionManagedRouterFixture) bind(t *testing.T) {
	t.Helper()
	// Only the managed identity fixture is reused here. No artifact success row
	// is inserted: the producer archive above was made by the real packager.
	consumer := &controlledArtifactRouterFixture{fx: testutil.New(testPool, testWorkspaceID, f.owner), user: f.owner, tokenID: f.tokenID, request: artifact.Request{TaskID: f.task}}
	f.binding, f.installation = bindControlledArtifactRouter(t, consumer)
}
func (f *completionManagedRouterFixture) completionRequest(t *testing.T) *http.Request {
	t.Helper()
	return testutil.WithHeaders(testutil.JSONRequest("PUT", "/api/issues/"+f.request.ResourceID, map[string]any{
		"status": "done", "expected_revision": f.grant.IssueRevision,
		"completion": map[string]any{"authorization_id": f.grant.AuthorizationID, "operation_id": f.grant.OperationID, "expected_issue_revision": f.grant.IssueRevision},
	}), "Authorization", "Bearer "+f.token, "X-Workspace-ID", testWorkspaceID)
}
func completionManagedFacts(t *testing.T, f *completionManagedRouterFixture) (status string, revision int64, terminals int) {
	t.Helper()
	f.fx.QueryRow(t, `SELECT status,revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&status, &revision)
	terminals = f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, f.grant.AuthorizationID)
	return
}

func TestDeliveryCompletionManagedRouterDistinctIssuerAndContributor(t *testing.T) {
	f := newCompletionManagedRouterFixture(t, true)
	archive := f.archive(t)
	testutil.Call(t, f.router.ServeHTTP, f.completionRequest(t)).Want(200)
	status, revision, count := completionManagedFacts(t, f)
	if status != "done" || revision <= f.grant.IssueRevision || count != 1 {
		t.Fatal("managed UpdateIssue did not atomically complete and consume")
	}
	var actorKind, actorUser, actorAgent, issuer string
	f.fx.QueryRow(t, `SELECT t.actor_kind,t.actor_user_id::text,t.actor_agent_id::text,a.issuer_user_id::text FROM delivery_completion_terminal t JOIN delivery_completion_authorization a ON a.id=t.authorization_id WHERE a.id=$1`, f.grant.AuthorizationID).Scan(&actorKind, &actorUser, &actorAgent, &issuer)
	if actorKind != "task" || actorUser != f.owner || actorAgent != f.agent || issuer != f.user || issuer == actorUser {
		t.Fatal("completion recorded issuer as consumer or lost managed task identity")
	}
	// A repeat passes real authentication and restores the exact consumed fact.
	testutil.Call(t, f.router.ServeHTTP, f.completionRequest(t)).Want(200)
	againStatus, againRevision, againCount := completionManagedFacts(t, f)
	if againStatus != status || againRevision != revision || againCount != 1 || !bytes.Equal(archive, f.archive(t)) {
		t.Fatal("replay changed state, consumer fact or physical producer archive")
	}
	var minimal map[string]any
	response := f.call(t, f.token, "GET", completionAuthorizationRouterPath+"/"+f.grant.AuthorizationID, nil).Want(200).JSON(&minimal)
	for _, private := range []string{"issuer_pat_hash", "request_json", "observation", "spec_digest", "artifact_digest", "consumer_binding_id"} {
		if strings.Contains(response.Text(), private) {
			t.Fatal("consumer recovery exposed private source or credential fields")
		}
	}
	if minimal["state"] != "consumed" {
		t.Fatal("managed consumer could not recover original minimal receipt")
	}
}

func TestDeliveryCompletionManagedRouterLiveFences(t *testing.T) {
	for _, change := range []string{"binding_revoked", "binding_epoch", "installation_retired", "organization_mapping_removed", "issuer_password_version", "consumer_password_version", "source_member_removed"} {
		t.Run(change, func(t *testing.T) {
			f := newCompletionManagedRouterFixture(t, true)
			before, revision, _ := completionManagedFacts(t, f)
			req := f.completionRequest(t)
			changed := false
			// The hook runs only when the actual authenticated handler reads the body.
			// Middleware already accepted B's original managed credential at this point.
			req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() {
				changed = true
				switch change {
				case "binding_revoked":
					f.fx.Exec(t, `UPDATE installation_daemon_binding SET state='revoked',revoked_at=clock_timestamp() WHERE id=$1`, f.binding)
				case "binding_epoch":
					f.fx.Exec(t, `UPDATE installation_daemon_binding SET binding_epoch=binding_epoch+1 WHERE id=$1`, f.binding)
				case "installation_retired":
					f.fx.Exec(t, `UPDATE managed_installation SET lifecycle='retired' WHERE id=$1`, f.installation)
				case "organization_mapping_removed":
					f.fx.Exec(t, `DELETE FROM organization_workspace WHERE workspace_id=$1`, testWorkspaceID)
				case "issuer_password_version":
					f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.user)
				case "consumer_password_version":
					f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.owner)
				case "source_member_removed":
					f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.user)
				}
			}}
			response := testutil.Call(t, f.router.ServeHTTP, req)
			want := http.StatusForbidden
			if change == "organization_mapping_removed" {
				want = http.StatusNotFound
			}
			response.Want(want)
			after, afterRevision, count := completionManagedFacts(t, f)
			if !changed || after != before || afterRevision != revision || count != 0 {
				t.Fatal("current authority refusal changed status or consumed grant")
			}
			if f.fx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE execution_id=$1 AND result_status='succeeded'`, f.approval.ExecutionID) != 1 {
				t.Fatal("authority refusal rewrote producer history")
			}
		})
	}
}

func TestDeliveryCompletionManagedRouterBindingAbsenceCannotUpgrade(t *testing.T) {
	f := newCompletionManagedRouterFixture(t, false)
	f.bind(t)
	// The original token row and real authentication now observe a valid bound
	// runtime. The old grant froze absence, so it must not adopt the new binding.
	before, revision, _ := completionManagedFacts(t, f)
	response := testutil.Call(t, f.router.ServeHTTP, f.completionRequest(t))
	response.Want(http.StatusForbidden)
	after, afterRevision, count := completionManagedFacts(t, f)
	if after != before || afterRevision != revision || count != 0 {
		t.Fatal("binding upgrade consumed old grant")
	}
}
