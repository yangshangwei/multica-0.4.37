package handler

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestDeliveryCompletionR38UnenforcedLegacyEvidenceRemainsAdministrative(t *testing.T) {
	for _, mode := range []string{"no_project", "off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			f := newAdminCollaborationHTTPFixture(t)
			if mode != "no_project" {
				fx := testutil.New(testPool, f.workspace, f.creator.User.ID)
				project := fx.Project(t, "Administrative compatibility")
				fx.Exec(t, `UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2`, project, f.issue)
				fx.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": f.workspace, "project_id": project, "revision": 1, "policy_version": "delivery-artifact-observe-v1", "mode": mode, "stage": "done", "reason": "Retained policy compatibility fixture", "created_by": f.creator.User.ID}, "project_id=$1", project)
			}
			// Simulate retained corrupt history, not a valid submitted result. R38
			// administration must not fabricate acceptance from this record.
			event := dbfx.Insert(t, "collaboration_event", testutil.Cols{"issue_id": f.issue, "event_seq": 1, "actor_user_id": f.handler.User.ID, "kind": "acceptance_submitted", "payload": `{}`, "outbox_state": "delivered"})
			dbfx.Exec(t, `UPDATE collaboration_session SET event_seq=1 WHERE issue_id=$1`, f.issue)
			dbfx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, f.creator.User.ID)
			path := "/api/admin/collaboration/issues/" + f.issue
			var detail service.AdminCollaborationDetail
			f.call(t, "GET", path, nil, "").Want(200).JSON(&detail)
			body := adminCollaborationHTTPBody(detail, "complete")
			want := 200
			if mode == "enforce" {
				want = 428
			}
			f.call(t, "POST", path+"/lifecycle", body, uuid.NewString()).Want(want)
			var status, payload string
			dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.issue).Scan(&status)
			dbfx.QueryRow(t, `SELECT payload::text FROM collaboration_event WHERE id=$1`, event).Scan(&payload)
			if (status == "done") != (mode != "enforce") || payload != "{}" {
				t.Fatal("administrative result changed old evidence or ignored current enforcement", fmt.Sprint(status, " ", payload))
			}
			if dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->'delivery_decision'->>'kind'='accepted'`, f.issue) != 0 || dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal x JOIN delivery_completion_authorization a ON a.id=x.authorization_id WHERE a.issue_id=$1`, f.issue) != 0 {
				t.Fatal("administrative fallback fabricated acceptance or a gate receipt")
			}
		})
	}
}

func TestDeliveryCompletionR38BindsActualPendingSubmission(t *testing.T) {
	f := newCompletionSharedFixture(t, "admin")
	// Submit a genuine result while the creator is active, then enter the existing
	// disabled-creator exception. The source child and tar use the real fixture.
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.actor)
	f.admin.router.Get("/api/collaboration/issues/{id}/status-options", f.h.GetCollaborationStatusOptions)
	f.admin.router.Post("/api/collaboration/issues/{id}/acceptance-submissions", f.h.SubmitCollaborationAcceptance)
	statusReq := testutil.JSONRequest("GET", "/api/collaboration/issues/"+f.issue+"/status-options?action=submit_acceptance", nil)
	statusReq.Header.Set("Authorization", "Bearer "+f.admin.handler.Token)
	var options service.CollaborationStatusOptions
	testutil.Call(t, f.admin.router.ServeHTTP, statusReq).Want(200).JSON(&options)
	var revision, lifecycle, participant, account int64
	f.fx.QueryRow(t, `SELECT c.revision,c.lifecycle_epoch,p.epoch,a.epoch FROM issue_collaboration c JOIN issue_participant p ON p.issue_id=c.issue_id AND p.user_id=$2 JOIN collaboration_account_fence a ON a.user_id=p.user_id WHERE c.issue_id=$1`, f.issue, f.admin.handler.User.ID).Scan(&revision, &lifecycle, &participant, &account)
	submissionBody := map[string]any{"expected_collaboration_revision": fmt.Sprint(revision), "expected_lifecycle_epoch": fmt.Sprint(lifecycle), "expected_participant_epoch": fmt.Sprint(participant), "expected_account_epoch": fmt.Sprint(account), "expected_status": options.DefaultStatus, "status_options_version": options.StatusOptionsVersion, "note": "Actual external delivery", "basis": "Exact source archive"}
	submit := testutil.JSONRequest("POST", "/api/collaboration/issues/"+f.issue+"/acceptance-submissions", submissionBody)
	submit.Header.Set("Authorization", "Bearer "+f.admin.handler.Token)
	submit.Header.Set("Idempotency-Key", uuid.NewString())
	testutil.Call(t, f.admin.router.ServeHTTP, submit).Want(200)
	var submission string
	f.fx.QueryRow(t, `SELECT id FROM issue_handoff WHERE issue_id=$1 AND purpose='acceptance' ORDER BY created_at DESC LIMIT 1`, f.issue).Scan(&submission)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, f.actor)
	req, body := f.authorization(t, "admin")
	req.Target.SubmissionID = &submission
	grant := f.grant(t, req)
	body["completion"] = service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: req.OperationID, ExpectedIssueRevision: req.Target.ExpectedIssueRevision}
	f.admin.call(t, "POST", "/api/admin/collaboration/issues/"+f.issue+"/lifecycle", body, req.OperationID).Want(200)
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.issue) != 1 || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND actor_kind='admin' AND result_submission_id IS NULL`, grant.AuthorizationID) != 1 {
		t.Fatal("current pending submission did not produce the separate administrative completion")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->'delivery_decision'->>'kind'='accepted'`, f.issue) != 0 {
		t.Fatal("administrative completion fabricated an initiator acceptance")
	}
}
