package handler

import (
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestDeliveryCompletionResourceDeletionRetainsOriginalAuthorizationAndTerminal(t *testing.T) {
	for _, target := range []string{"project", "issue", "workspace"} {
		t.Run(target, func(t *testing.T) {
			f := newCompletionFixture(t) // Own workspace, real controlled archive/receipt.
			consumed := f.create(t, uuid.NewString(), 201)
			f.request.OperationID = uuid.NewString()
			revoked := f.create(t, uuid.NewString(), 201)
			binding := service.DeliveryCompletionBinding{AuthorizationID: consumed.AuthorizationID, OperationID: consumed.OperationID, ExpectedIssueRevision: consumed.IssueRevision}
			var completed IssueResponse
			testutil.Call(t, f.h.UpdateIssue, ordinaryAuthorizedRequest(f.deliveryPolicyFixture, map[string]any{"status": "done", "completion": binding})).Want(200).JSON(&completed)
			if completed.CompletionReceipt == nil || completed.CompletionReceipt.AuthorizationID != consumed.AuthorizationID {
				t.Fatal("real completion did not retain the original authorization identity")
			}
			revoke := testutil.WithURLParams(f.artifactFixture.request("POST", "/api/delivery-completion-authorizations/"+revoked.AuthorizationID+"/revocations", map[string]string{"reason": "Retain original withdrawal before resource deletion"}), "authorizationId", revoked.AuthorizationID)
			revoke.Header.Set("Idempotency-Key", uuid.NewString())
			testutil.Call(t, f.h.RevokeDeliveryCompletionAuthorization, revoke).Want(200)
			ids := []string{consumed.AuthorizationID, revoked.AuthorizationID}
			if dbfx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE id=ANY($1::uuid[])`, ids) != 2 || dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=ANY($1::uuid[])`, ids) != 2 {
				t.Fatal("setup lacks the actual immutable authorizations and both terminal facts")
			}
			retained := func() [32]byte {
				var raw string
				dbfx.QueryRow(t, `SELECT jsonb_build_object(
				 'authorizations',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM delivery_completion_authorization a WHERE id=ANY($1::uuid[])),
				 'terminals',(SELECT jsonb_agg(to_jsonb(x) ORDER BY authorization_id) FROM delivery_completion_terminal x WHERE authorization_id=ANY($1::uuid[])))::text`, ids).Scan(&raw)
				return sha256.Sum256([]byte(raw))
			}
			before := retained()
			switch target {
			case "project":
				f.policy(t, "PUT", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "Explicit policy rollback before project removal"}, 200)
				if retained() != before {
					t.Fatal("policy rollback rewrote completion history")
				}
				testutil.Call(t, f.h.DeleteProject, testutil.WithURLParams(f.artifactFixture.request("DELETE", "/api/projects/"+f.project, nil), "id", f.project)).Want(204)
				if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND project_id IS NULL`, f.request.Target.IssueID) != 1 || dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 2 {
					t.Fatal("actual project removal lost the surviving issue or policy history")
				}
			case "issue":
				testutil.Call(t, f.h.DeleteIssue, testutil.WithURLParams(f.artifactFixture.request("DELETE", "/api/issues/"+f.request.Target.IssueID, nil), "id", f.request.Target.IssueID)).Want(204)
			case "workspace":
				f.deleteWorkspace(t)
			}
			if retained() != before {
				t.Fatal("resource deletion changed retained authorization or terminal bytes")
			}
			want := 404
			if target == "project" {
				want = 200
			}
			for _, prior := range []service.DeliveryCompletionAuthorizationView{consumed, revoked} {
				view := f.get(t, prior.AuthorizationID, want)
				if target == "project" {
					state := "revoked"
					if prior.AuthorizationID == consumed.AuthorizationID {
						state = "consumed"
					}
					if view.State != state || !view.ExpiresAt.Equal(prior.ExpiresAt) || state == "consumed" && (view.CompletionReceipt == nil || view.CompletionReceipt.CompletedRevision != completed.Revision) {
						t.Fatal("surviving-issue recovery renewed or reinterpreted the original completion")
					}
				}
			}
			if retained() != before {
				t.Fatal("read recovery rewrote retained completion facts")
			}
		})
	}
}
