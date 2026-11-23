package handler

import (
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestOrdinaryDeliveryBatchZeroRevisionConsumesExactGrant(t *testing.T) {
	f := newCompletionFixture(t)
	// Zero is a supported retained issue revision. The controlled archive and
	// authorization below still use their real production APIs and bindings.
	dbfx.Exec(t, `UPDATE issue SET revision=0 WHERE id=$1`, f.request.Target.IssueID)
	f.request.Target.ExpectedIssueRevision = 0
	f.request.Evidence.ExpectedIssueRevision = 0
	grant := f.create(t, uuid.NewString(), 201)
	if grant.IssueRevision != 0 {
		t.Fatal("fixture did not sign the actual zero-revision issue")
	}
	body := map[string]any{"issue_ids": []string{grant.IssueID}, "updates": map[string]any{"status": "done"}, "completion_bindings": []map[string]any{{"issue_id": grant.IssueID, "authorization_id": grant.AuthorizationID, "operation_id": grant.OperationID, "expected_issue_revision": int64(0)}}}
	var result BatchUpdateIssuesResponse
	testutil.Call(t, f.h.BatchUpdateIssues, f.artifactFixture.request("POST", "/api/issues/batch-update", body)).Want(200).JSON(&result)
	if result.Updated != 1 || len(result.Results) != 1 || result.Results[0].State != "committed" || result.Results[0].Revision == nil || *result.Results[0].Revision <= 0 {
		t.Fatal("zero-revision completion lost its actual committed result")
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, grant.IssueID) != 1 || dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND issue_revision_before=0`, grant.AuthorizationID) != 1 {
		t.Fatal("zero-revision batch did not commit status and the exact authorization together")
	}
}

func TestOrdinaryDeliveryBatchDuplicateAuthorizationOrOperationRefusesBeforeWrites(t *testing.T) {
	for _, repeated := range []string{"authorization", "operation"} {
		t.Run(repeated, func(t *testing.T) {
			f := newCompletionFixture(t)
			grant := f.create(t, uuid.NewString(), 201)
			other := dbfx.Issue(t, "Unattempted duplicate binding", testutil.Cols{"workspace_id": f.ws, "creator_id": f.actor})
			secondAuthorization, secondOperation := uuid.NewString(), uuid.NewString()
			if repeated == "authorization" {
				secondAuthorization = grant.AuthorizationID
			} else {
				secondOperation = grant.OperationID
			}
			body := map[string]any{"issue_ids": []string{grant.IssueID, other}, "updates": map[string]any{"status": "done"}, "completion_bindings": []map[string]any{
				{"issue_id": grant.IssueID, "authorization_id": grant.AuthorizationID, "operation_id": grant.OperationID, "expected_issue_revision": grant.IssueRevision},
				{"issue_id": other, "authorization_id": secondAuthorization, "operation_id": secondOperation, "expected_issue_revision": int64(1)},
			}}
			response := testutil.Call(t, f.h.BatchUpdateIssues, f.artifactFixture.request("POST", "/api/issues/batch-update", body))
			if response.Code != 400 {
				t.Errorf("duplicate %s was not rejected at the request boundary: status=%d body=%s", repeated, response.Code, response.Text())
			}
			if dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=ANY($1::uuid[]) AND status='done'`, []string{grant.IssueID, other}) != 0 || dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
				t.Error("malformed duplicate binding request committed a prefix before rejection")
			}
		})
	}
}
