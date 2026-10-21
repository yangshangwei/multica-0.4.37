package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationWorkspaceDeletionPreservesPrivateAttachmentsAndPermissions(t *testing.T) {
	for _, status := range []string{"todo", "done", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
			h := collaborationFileHandler(t)
			source := dbfx.Workspace(t, "Preserved collaboration source", "preserved-source-"+uuid.NewString())
			dbfx.Member(t, source, testUserID, "owner")
			issue := collaborationBoundaryIssue(t, source)
			owner := dbfx.User(t, "External preserved resource owner", "preserved-owner-"+uuid.NewString()+"@example.test")
			execution := dbfx.Workspace(t, "Preserved private execution", "preserved-execution-"+uuid.NewString())
			dbfx.Member(t, execution, owner, "owner")
			dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": owner}, "issue_id=$1 AND user_id=$2", issue, owner)
			for _, user := range []string{testUserID, owner} {
				dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": user}, "user_id=$1", user)
			}
			for _, table := range []string{"comment", "collaboration_event", "collaboration_notification", "collaboration_grant"} {
				dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
			}
			dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
			dbfx.Cleanup(t, `DELETE FROM collaboration_channel WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)`, issue)
			dbfx.Cleanup(t, `DELETE FROM collaboration_target WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)`, issue)
			var resources []string
			t.Cleanup(func() {
				if _, err := testPool.Exec(context.Background(), `DELETE FROM collaboration_config_invalidation WHERE resource_id=ANY($1::uuid[])`, resources); err != nil {
					t.Errorf("cleanup preservation resource intents: %v", err)
				}
			})
			fx := testutil.New(testPool, execution, owner)
			runtime := fx.Runtime(t, "Preserved private offline runtime", testutil.Cols{"status": "offline"})
			agent := fx.Agent(t, "Preserved private agent", runtime)
			resources = []string{runtime, agent}
			installation := uuid.NewString()
			binding := fx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": owner,
				"execution_workspace_id": execution, "runtime_id": runtime, "installation_id": installation, "public_key": make([]byte, 32)})
			registration := switchHTTPBody()
			registration["kind"], registration["resource_id"], registration["expected_target_set_version"] = "agent", agent, "1"
			registration["roster"] = []map[string]string{{"role": "agent", "agent_id": agent, "runtime_id": runtime, "owner_user_id": owner, "execution_workspace_id": execution}}
			var target, consent service.CollaborationQueueReceipt
			testutil.Call(t, h.RegisterCollaborationTarget, collaborationContentRequest(http.MethodPost, issue, "", testUserID, uuid.NewString(), registration)).Want(http.StatusCreated).JSON(&target)
			if !target.TargetID.Valid {
				t.Fatal("preservation fixture did not register an exact target")
			}
			consentBody := switchHTTPBody()
			consentBody["agent_id"], consentBody["expected_target_epoch"], consentBody["expected_config_version"] = agent, "1", "1"
			consentBody["identity"] = map[string]string{"kind": "enrolled", "installation_id": installation, "binding_id": binding, "binding_epoch": "1"}
			testutil.Call(t, h.ConsentCollaborationTarget, withURLParams(collaborationContentRequest(http.MethodPost, issue, "", owner, uuid.NewString(), consentBody),
				"targetId", uuidToString(target.TargetID))).Want(http.StatusCreated).JSON(&consent)
			if !consent.GrantID.Valid || dbfx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND issue_id=$2 AND target_id=$3
				AND owner_user_id=$4 AND execution_workspace_id=$5 AND agent_id=$6 AND runtime_id=$7 AND binding_id=$8 AND revoked_at IS NULL`,
				consent.GrantID, issue, target.TargetID, owner, execution, agent, runtime, binding) != 1 {
				t.Fatal("preservation fixture lacks exact live owner consent")
			}
			draft, attachment := collaborationUploadedFile(t, h, issue, owner)
			var comment service.CollaborationCommentReceipt
			testutil.Call(t, h.CreateCollaborationComment, collaborationContentRequest(http.MethodPost, issue, "", owner, uuid.NewString(),
				map[string]any{"content": "Preserved external discussion", "expected_lifecycle_epoch": "1", "draft_id": draft,
					"attachment_ids": []string{uuidToString(attachment.ID)}})).Want(http.StatusCreated).JSON(&comment)
			if dbfx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id=$2 AND uploader_user_id=$3
				AND source_workspace_id=$4 AND encryption_key IS NOT NULL AND delivered_at IS NOT NULL`, attachment.ID, issue, owner, source) != 1 ||
				dbfx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND removed_at IS NULL
					AND ((reference_kind='issue' AND reference_id=$2) OR (reference_kind='comment' AND reference_id=$3))`, attachment.ID, issue, comment.CommentID) != 2 {
				t.Fatal("preservation fixture lacks delivered encrypted bytes and exact issue/comment references")
			}
			if dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2 AND role='owner'`, source, testUserID) != 1 ||
				dbfx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2 AND role='owner'`, execution, owner) != 1 ||
				dbfx.Count(t, `SELECT count(*) FROM member WHERE (workspace_id=$1 AND user_id=$2) OR (workspace_id=$3 AND user_id=$4)`, source, owner, execution, testUserID) != 0 ||
				dbfx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=ANY($2::uuid[]) AND revoked_at IS NULL`, issue, []string{testUserID, owner}) != 2 {
				t.Fatal("fixture does not separate source membership, external participation and execution ownership")
			}
			dbfx.Exec(t, `UPDATE issue SET status=$2 WHERE id=$1`, issue, status)
			readBytes := func() {
				t.Helper()
				for _, viewer := range []string{testUserID, owner} {
					response := testutil.Call(t, h.GetCollaborationAttachmentContent, collaborationFileRequest(http.MethodGet, issue, "", uuidToString(attachment.ID), viewer, nil)).Want(http.StatusOK)
					if response.Text() != "controlled private bytes" {
						t.Fatal("preservation lost private blob bytes or one independent current read basis")
					}
				}
			}
			readBytes()
			snapshot := func() string {
				var result string
				dbfx.QueryRow(t, `SELECT jsonb_build_object(
					'source',(SELECT to_jsonb(w) FROM workspace w WHERE w.id=$1),
					'execution',(SELECT to_jsonb(w) FROM workspace w WHERE w.id=$3),
					'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$2),
					'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$2),
					'session',(SELECT to_jsonb(s) FROM collaboration_session s WHERE s.issue_id=$2),
					'accounts',(SELECT jsonb_agg(to_jsonb(u) ORDER BY u.id) FROM "user" u WHERE u.id=ANY($4::uuid[])),
					'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.workspace_id,m.user_id) FROM member m WHERE m.workspace_id IN ($1,$3)),
					'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.user_id) FROM issue_participant p WHERE p.issue_id=$2),
					'fences',(SELECT jsonb_agg(to_jsonb(f) ORDER BY f.user_id) FROM collaboration_account_fence f WHERE f.user_id=ANY($4::uuid[])),
					'targets',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM collaboration_target t WHERE t.session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$2)),
					'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_grant g WHERE g.issue_id=$2),
					'enrollments',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM collaboration_runtime_enrollment e WHERE e.id IN (SELECT binding_id FROM collaboration_grant WHERE issue_id=$2)),
					'agents',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM agent a WHERE a.id IN (SELECT agent_id FROM collaboration_grant WHERE issue_id=$2)),
					'runtimes',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM agent_runtime r WHERE r.id IN (SELECT runtime_id FROM collaboration_grant WHERE issue_id=$2)),
					'attachments',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM collaboration_attachment a WHERE a.issue_id=$2),
					'references',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.attachment_id,r.reference_kind,r.reference_id,r.reference_revision)
						FROM collaboration_attachment_reference r WHERE r.attachment_id IN (SELECT id FROM collaboration_attachment WHERE issue_id=$2)),
					'comments',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM comment c WHERE c.issue_id=$2),
					'events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=$2),
					'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_operation o WHERE o.scope_kind='issue' AND o.scope_id=$2)
					)::text`, source, issue, execution, []string{testUserID, owner}).Scan(&result)
				return result
			}
			before := snapshot()
			var state map[string]json.RawMessage
			if err := json.Unmarshal([]byte(before), &state); err != nil {
				t.Fatal(err)
			}
			for _, relation := range []string{"accounts", "members", "participants", "fences", "targets", "grants", "enrollments", "agents", "runtimes", "attachments", "references", "comments", "events", "operations"} {
				var rows []json.RawMessage
				if err := json.Unmarshal(state[relation], &rows); err != nil || len(rows) == 0 {
					t.Fatalf("preservation snapshot lacks actual %s evidence: %v", relation, err)
				}
			}
			request := withURLParam(newRequest(http.MethodDelete, "/api/workspaces/"+source, nil), "id", source)
			response := testutil.Call(t, h.DeleteWorkspace, request).Want(http.StatusConflict).Map()
			if response["code"] != "workspace_has_collaboration_records" || snapshot() != before {
				t.Fatal("source deletion refusal partially changed retained records or permissions")
			}
			readBytes()
		})
	}
}
