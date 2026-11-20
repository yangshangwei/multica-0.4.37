package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// A task owner cannot lend their human signing authority to a claimed Agent.
// Ordinary tasks explicitly designated by a human grant retain their separate
// completion contract; this regression targets human grant issuance/revocation.
func TestCompletionAuthorizationProtectedThroughClaimedTaskRouter(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	activateCompletionPolicy(t, f, 0)
	actors := completionClaimedActors(t, f.controlledArtifactRouterFixture, f.user, f.token, f.project)
	originalBody := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
	originalKey := uuid.NewString()
	var original completionAuthorizationRouterResponse
	f.call(t, f.token, http.MethodPost, completionAuthorizationRouterPath, originalBody, "Idempotency-Key", originalKey).Want(http.StatusCreated).JSON(&original)
	if original.AuthorizationID == "" || original.State != "issued" || original.OperationID != originalBody["operation_id"] {
		t.Fatal("human did not issue the exact original completion authorization")
	}
	originalPath := completionAuthorizationRouterPath + "/by-request-key/" + originalKey + "?workspace_id=" + testWorkspaceID
	originalReceipt := f.call(t, f.token, http.MethodGet, originalPath, nil).Want(http.StatusOK).Map()
	newBody := completionAuthorizationRouterBody(t, f, 1, map[string]any{"kind": "human", "user_id": f.user})
	createKey, revokeKey := uuid.NewString(), uuid.NewString()
	revokeBody := map[string]string{"reason": "Only the human issuer may withdraw this exact acceptance"}
	revokePath := completionAuthorizationRouterPath + "/" + original.AuthorizationID + "/revocations"
	before, archive := completionRefusalFacts(t, f.fx), f.archive(t)
	encoded, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("refusal baseline collections=%d sha256=%x", len(before), sha256.Sum256(encoded))
	logCompletionProof(t, map[string]any{
		"phase": "authorization_baseline", "issue_id": f.request.ResourceID,
		"original_request": originalBody, "original_key": originalKey, "original_receipt": originalReceipt,
		"create_request": newBody, "create_key": createKey, "revocation_request": revokeBody, "revocation_key": revokeKey,
		"artifact_digest": f.body["expected_artifact_digest"],
	})
	for _, actor := range actors {
		for _, action := range []string{"create_authorization", "revoke_original_authorization"} {
			t.Run(actor.name+"/"+action, func(t *testing.T) {
				path, key, body := completionAuthorizationRouterPath, createKey, any(newBody)
				if action == "revoke_original_authorization" {
					path, key, body = revokePath, revokeKey, revokeBody
				}
				headers := append([]string{"Idempotency-Key", key}, actor.headers...)
				denied := f.call(t, actor.token, http.MethodPost, path, body, headers...).Want(http.StatusForbidden).Map()
				if actor.name == "generic_claimed_task" {
					// Auth stamps the task credential source, so RequireHumanActor
					// refuses here before the M1 handler/service. A JWT carrying
					// an Agent hint instead reaches the handler's human backstop.
					if !reflect.DeepEqual(denied, map[string]any{"error": "this endpoint is only available to human actors"}) {
						t.Fatalf("claimed machine missed the human-only router guard: %v", denied)
					}
				} else {
					assertCompletionMachineRefusal(t, denied, actor.restricted, "delivery_completion_denied")
				}
				if !reflect.DeepEqual(completionRefusalFacts(t, f.fx), before) || !bytes.Equal(f.archive(t), archive) {
					t.Fatal("machine authorization request changed retained acceptance or business facts")
				}
				if got := f.call(t, f.token, http.MethodGet, originalPath, nil).Want(http.StatusOK).Map(); !reflect.DeepEqual(got, originalReceipt) {
					t.Fatal("machine request changed the original human authorization receipt")
				}
				logCompletionProof(t, map[string]any{"phase": "authorization_refusal", "actor": actor.name, "action": action, "request_key": key, "status": http.StatusForbidden, "response": denied, "full_business_collections_unchanged": len(before)})
			})
		}
	}
	// Identical bodies and original request keys succeed as the actual human:
	// the denials above cannot be explained by malformed or expired requests.
	var created completionAuthorizationRouterResponse
	f.call(t, f.token, http.MethodPost, completionAuthorizationRouterPath, newBody, "Idempotency-Key", createKey).Want(http.StatusCreated).JSON(&created)
	if created.AuthorizationID == "" || created.AuthorizationID == original.AuthorizationID || created.OperationID != newBody["operation_id"] || created.State != "issued" {
		t.Fatal("same authorization body did not succeed as a new human acceptance")
	}
	var revoked completionAuthorizationRouterResponse
	f.call(t, f.token, http.MethodPost, revokePath, revokeBody, "Idempotency-Key", revokeKey).Want(http.StatusOK).JSON(&revoked)
	if revoked.AuthorizationID != original.AuthorizationID || revoked.OperationID != original.OperationID || revoked.State != "revoked" {
		t.Fatal("same original-grant revocation did not succeed as the human issuer")
	}
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='revoked' AND actor_kind='human' AND actor_user_id=$2 AND revocation_request_key=$3 AND revocation_reason=$4`, original.AuthorizationID, f.user, revokeKey, revokeBody["reason"]) != 1 {
		t.Fatal("human revocation did not retain its exact actor, key and reason")
	}
	logCompletionProof(t, map[string]any{"phase": "authorization_human_controls", "issue_id": f.request.ResourceID, "created": created, "revoked": revoked, "create_key": createKey, "revocation_key": revokeKey})
}

// Even a valid human grant and delivery cannot turn its initiator's claimed
// Agent into the human who is allowed to accept and close a collaboration.
func TestCollaborationCompletionProtectedThroughClaimedTaskRouter(t *testing.T) {
	f := newCompletionCollaborationRouterFixture(t, "submission")
	// Explicit fixture membership allows the initiator to own ordinary runtime
	// claims. The source issuer and external delivery author remain distinct.
	f.fx.Member(t, testWorkspaceID, f.initiator, "member")
	actors := completionClaimedActors(t, f.controlledArtifactRouterFixture, f.initiator, f.initiatorToken, f.project)
	req, body := f.authorization(t, "submission")
	grant := f.grant(t, req)
	if req.Target.SubmissionID == nil || grant.State != "issued" || grant.OperationID != req.OperationID || req.Consumer.UserID != f.initiator {
		t.Fatal("completion requires an exact external submission and human consumer grant")
	}
	body["completion"] = service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: req.OperationID, ExpectedIssueRevision: req.Target.ExpectedIssueRevision}
	var submissionKey, grantKey string
	f.fx.QueryRow(t, `SELECT request_id::text FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND actor_user_id=$2 AND kind='submit_acceptance' AND result->'delivery_confirmation'->>'submission_id'=$3`, f.parent, f.recipient, *req.Target.SubmissionID).Scan(&submissionKey)
	f.fx.QueryRow(t, `SELECT request_key::text FROM delivery_completion_authorization WHERE id=$1`, grant.AuthorizationID).Scan(&grantKey)
	submissionPath := f.pathFor("/operations/" + submissionKey)
	grantPath := completionAuthorizationRouterPath + "/by-request-key/" + grantKey + "?workspace_id=" + testWorkspaceID
	submissionReceipt := f.call(t, f.recipientToken, http.MethodGet, submissionPath, nil).Want(http.StatusOK).Map()
	grantReceipt := f.call(t, f.token, http.MethodGet, grantPath, nil).Want(http.StatusOK).Map()
	var delivery service.CollaborationDeliveryHistory
	f.call(t, f.initiatorToken, http.MethodGet, f.pathFor("/acceptance-submissions"), nil).Want(http.StatusOK).JSON(&delivery)
	before, archive := completionRefusalFacts(t, f.fx), f.archive(t)
	encoded, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("refusal baseline collections=%d sha256=%x", len(before), sha256.Sum256(encoded))
	logCompletionProof(t, map[string]any{
		"phase": "collaboration_baseline", "issue_id": f.parent, "submission_id": *req.Target.SubmissionID,
		"submission_key": submissionKey, "submission_receipt": submissionReceipt, "delivery_history": delivery,
		"authorization_request": req, "grant_key": grantKey, "grant_receipt": grantReceipt,
		"completion_request": body, "operation_id": req.OperationID, "artifact_digest": f.body["expected_artifact_digest"],
	})
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->'delivery_decision'->>'kind'='accepted'`, f.parent) != 0 {
		t.Fatal("fixture must not start with an accepted delivery")
	}
	for _, actor := range actors {
		t.Run(actor.name+"/complete_exact_submission", func(t *testing.T) {
			headers := append([]string{"Idempotency-Key", req.OperationID}, actor.headers...)
			denied := f.call(t, actor.token, http.MethodPost, f.pathFor("/lifecycle"), body, headers...).Want(http.StatusForbidden).Map()
			assertCompletionMachineRefusal(t, denied, actor.restricted, "forbidden")
			if !reflect.DeepEqual(completionRefusalFacts(t, f.fx), before) || !bytes.Equal(f.archive(t), archive) {
				t.Fatal("machine completion changed the original delivery, grant, state or business effects")
			}
			if got := f.call(t, f.recipientToken, http.MethodGet, submissionPath, nil).Want(http.StatusOK).Map(); !reflect.DeepEqual(got, submissionReceipt) {
				t.Fatal("machine completion changed the original human submission receipt")
			}
			if got := f.call(t, f.token, http.MethodGet, grantPath, nil).Want(http.StatusOK).Map(); !reflect.DeepEqual(got, grantReceipt) {
				t.Fatal("machine completion changed the original human grant receipt")
			}
			var current service.CollaborationDeliveryHistory
			f.call(t, f.initiatorToken, http.MethodGet, f.pathFor("/acceptance-submissions"), nil).Want(http.StatusOK).JSON(&current)
			if !reflect.DeepEqual(current, delivery) {
				t.Fatal("machine completion changed immutable delivery history")
			}
			logCompletionProof(t, map[string]any{"phase": "collaboration_refusal", "actor": actor.name, "operation_id": req.OperationID, "status": http.StatusForbidden, "response": denied, "full_business_collections_unchanged": len(before)})
		})
	}
	completed := f.complete(t, req, body, &grant).Want(http.StatusOK).Map()
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.parent) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE issue_id=$1 AND queue_state='closed'`, f.parent) != 1 {
		t.Fatal("same current completion body did not close the collaboration as its human initiator")
	}
	if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal t JOIN collaboration_operation o ON o.id=t.collaboration_operation_id WHERE t.authorization_id=$1 AND t.kind='consumed' AND t.actor_kind='human' AND t.actor_user_id=$2 AND t.operation_id=$3 AND t.result_submission_id=$4 AND t.issue_revision_after>t.issue_revision_before AND o.scope_id=$5 AND o.request_id=$3::text`, grant.AuthorizationID, f.initiator, req.OperationID, *req.Target.SubmissionID, f.parent) != 1 {
		t.Fatal("human completion did not atomically bind the exact grant, original operation and submission")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND actor_user_id=$2 AND payload->'delivery_decision'->>'kind'='accepted' AND payload->'delivery_decision'->>'submission_id'=$3`, f.parent, f.initiator, *req.Target.SubmissionID) != 1 {
		t.Fatal("successful human control omitted the exact initiator acceptance decision")
	}
	if got := f.call(t, f.recipientToken, http.MethodGet, submissionPath, nil).Want(http.StatusOK).Map(); !reflect.DeepEqual(got, submissionReceipt) {
		t.Fatal("subsequent human completion rewrote the original submission receipt")
	}
	var accepted service.CollaborationDeliveryHistory
	f.call(t, f.initiatorToken, http.MethodGet, f.pathFor("/acceptance-submissions"), nil).Want(http.StatusOK).JSON(&accepted)
	consumed := f.call(t, f.initiatorToken, http.MethodGet, completionAuthorizationRouterPath+"/"+grant.AuthorizationID, nil).Want(http.StatusOK).Map()
	logCompletionProof(t, map[string]any{"phase": "collaboration_human_control", "issue_id": f.parent, "operation_id": req.OperationID, "completion_receipt": completed, "consumed_grant": consumed, "delivery_history": accepted})
}

type completionMachineActor struct {
	name, token string
	restricted  bool
	headers     []string
}

func completionClaimedActors(t *testing.T, f *controlledArtifactRouterFixture, owner, ownerToken, project string) []completionMachineActor {
	t.Helper()
	fx := testutil.New(testPool, testWorkspaceID, owner)
	actors := []completionMachineActor{}
	var genericAgent string
	for _, restricted := range []bool{false, true} {
		name := "generic_claimed_task"
		cols := testutil.Cols{"autonomy_level": "operator"}
		if restricted {
			name = "restricted_claimed_task"
			cols["template_key"], cols["template_version"] = "code-reviewer", 2
		}
		runtime := fx.Runtime(t, "Completion refusal runtime")
		agent := fx.Agent(t, name, runtime, cols)
		ordinary := fx.Issue(t, "Positive claimed-credential read", testutil.Cols{"project_id": project})
		task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": ordinary, "originator_user_id": owner, "accountable_user_id": owner})
		fx.Cleanup(t, `DELETE FROM task_token WHERE task_id=$1`, task)
		var claimed struct {
			Task *handler.AgentTaskResponse `json:"task"`
		}
		f.call(t, ownerToken, http.MethodPost, "/api/daemon/runtimes/"+runtime+"/tasks/claim", nil).Want(http.StatusOK).JSON(&claimed)
		if claimed.Task == nil || claimed.Task.ID != task || claimed.Task.AgentID != agent || claimed.Task.IssueID != ordinary || !strings.HasPrefix(claimed.Task.AuthToken, "mat_") {
			t.Fatal("actual daemon claim did not issue the selected task credential")
		}
		token, err := db.New(testPool).GetTaskTokenByHash(t.Context(), auth.HashToken(claimed.Task.AuthToken))
		if err != nil || token.UserID != util.MustParseUUID(owner) || token.TaskID != util.MustParseUUID(task) || token.AgentID != util.MustParseUUID(agent) || token.WorkspaceID != util.MustParseUUID(testWorkspaceID) {
			t.Fatal("claimed credential does not bind its exact owner, task, Agent and workspace")
		}
		if restricted {
			capability, err := auth.ParseTaskCapability(token.ApiCapability)
			if err != nil || capability.Profile != auth.TaskCapabilityReviewer || capability.IssueID != ordinary || capability.RuntimeID != runtime || capability.ClaimGeneration < 1 {
				t.Fatal("restricted credential lacks its exact server-frozen capability")
			}
		} else {
			if len(token.ApiCapability) != 0 {
				t.Fatal("generic task unexpectedly acquired a restricted route ceiling")
			}
			genericAgent = agent
		}
		allowed := f.call(t, claimed.Task.AuthToken, http.MethodGet, "/api/issues/"+ordinary, nil).Want(http.StatusOK).Map()
		if allowed["id"] != ordinary || allowed["workspace_id"] != testWorkspaceID {
			t.Fatal("same claimed credential did not successfully read its supported ordinary issue")
		}
		actors = append(actors, completionMachineActor{name: name, token: claimed.Task.AuthToken, restricted: restricted})
		t.Logf("positive claim name=%s owner=%s agent=%s runtime=%s task=%s issue=%s", name, owner, agent, runtime, task, ordinary)
	}
	return append(actors, completionMachineActor{name: "human_with_agent_hint", token: ownerToken, headers: []string{"X-Agent-ID", genericAgent}})
}

func assertCompletionMachineRefusal(t *testing.T, response map[string]any, restricted bool, code string) {
	t.Helper()
	if restricted {
		if response["error"] != "task capability does not authorize this action or resource" {
			t.Fatalf("restricted credential missed the capability route boundary: %v", response)
		}
	} else if response["code"] != code {
		t.Fatalf("authenticated machine missed the human-only boundary: %v", response)
	}
}

func completionRefusalFacts(t *testing.T, fx *testutil.Fixture) map[string]json.RawMessage {
	t.Helper()
	facts := tc13RouterQAFacts(t, fx, testWorkspaceID)
	for table, predicate := range map[string]string{
		"delivery_completion_authorization": "workspace_id=$1",
		"delivery_completion_terminal":      "authorization_id IN (SELECT id FROM delivery_completion_authorization WHERE workspace_id=$1)",
		"issue_handoff":                     "issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)",
		"issue_participant":                 "issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)",
		"agent_approval_request":            "workspace_id=$1",
		"agent_artifact_execution":          "workspace_id=$1",
		"delivery_evidence_evaluation":      "workspace_id=$1",
		"project_delivery_policy":           "workspace_id=$1",
		"project":                           "workspace_id=$1",
		"inbox_item":                        "workspace_id=$1",
		"iteration_event":                   "workspace_id=$1",
		"iteration_operation":               "workspace_id=$1",
		"iteration_notification":            "workspace_id=$1",
	} {
		var raw string
		fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM `+table+` x WHERE `+predicate, testWorkspaceID).Scan(&raw)
		facts[table] = json.RawMessage(raw)
	}
	return facts
}

// Keep safe public history in the raw test result before fixture teardown.
// Full SQL authorization rows include private issuer metadata and stay in memory.
func logCompletionProof(t *testing.T, evidence map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("completion proof %s", encoded)
}
