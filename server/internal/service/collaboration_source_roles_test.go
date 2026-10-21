package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func sourceRoleBusinessSnapshot(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
		'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
		'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.user_id) FROM issue_participant p WHERE p.issue_id=$1),
		'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.id) FROM member m WHERE m.workspace_id=$2),
		'handoffs',(SELECT jsonb_agg(to_jsonb(h) ORDER BY h.id) FROM issue_handoff h WHERE h.issue_id=$1),
		'notifications',(SELECT jsonb_agg(to_jsonb(n) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=$1),
		'inbox',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.id) FROM inbox_item i WHERE i.issue_id=$1),
		'iterations',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.id) FROM iteration i WHERE i.workspace_id=$2),
		'participations',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.iteration_id,p.issue_id) FROM iteration_participation p WHERE p.issue_id=$1),
		'iteration_events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.iteration_id,e.sequence) FROM iteration_event e WHERE e.workspace_id=$2),
		'ordinary_tasks',(SELECT jsonb_agg(to_jsonb(q) ORDER BY q.id) FROM agent_task_queue q WHERE q.issue_id=$1),
		'batches',(SELECT jsonb_agg(to_jsonb(b) ORDER BY b.id) FROM collaboration_batch b WHERE b.session_id=$3),
		'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.group_id,i.command_id) FROM collaboration_group_input i
			WHERE i.group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$3)),
		'dependencies',(SELECT jsonb_agg(to_jsonb(d) ORDER BY d.group_id,d.command_id) FROM collaboration_group_dependency d
			WHERE d.group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$3))
	)::text`, f.issue.Issue.ID, f.issue.Issue.WorkspaceID, f.issue.SessionID).Scan(&snapshot)
	return snapshot
}

func sourceRoleExecutionSnapshot(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal([]byte(controlStateSnapshot(t, f)), &snapshot); err != nil {
		t.Fatal(err)
	}
	delete(snapshot, "events")
	var session map[string]json.RawMessage
	if err := json.Unmarshal(snapshot["session"], &session); err != nil {
		t.Fatal(err)
	}
	delete(session, "event_seq")
	snapshot["session"] = fieldJSON(t, session)
	return string(fieldJSON(t, snapshot))
}

func TestCollaborationSourceStructureAndIterationRejectNonManagers(t *testing.T) {
	for _, role := range []string{"source_member", "source_owner", "participant", "source_participant", "external_handler", "departed_handler", "departed_creator"} {
		t.Run(role, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationQueueFixture(t)
			project := util.MustParseUUID(f.fx.Project(t, "Current source project"))
			privateParentBody := "PRIVATE_SOURCE_ROLE_PARENT_BODY_" + uuid.NewString()
			parent := util.MustParseUUID(f.fx.Issue(t, "Current source parent", testutil.Cols{"description": privateParentBody}))
			stage := pgtype.Int4{Int32: 2, Valid: true}
			if _, err := f.svc.EditIssue(f.ctx, CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), ProjectID: &project, ParentIssueID: &parent, Stage: &stage}); err != nil {
				t.Fatal(err)
			}
			period := sourceFieldIteration(t, f.collaborationFixture, "Current source iteration")
			if _, err := f.svc.ChangeIteration(f.ctx, CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, 2), CurrentIterationID: period, ExpectedIssueRevision: f.issue.Issue.Revision + 1, ExpectedIterationRevision: 1}); err != nil {
				t.Fatal(err)
			}
			otherProject := util.MustParseUUID(f.fx.Project(t, "PRIVATE_UNSELECTED_PROJECT_"+uuid.NewString()))
			otherParent := util.MustParseUUID(f.fx.Issue(t, "PRIVATE_UNSELECTED_PARENT_"+uuid.NewString()))
			otherPeriod := sourceFieldIteration(t, f.collaborationFixture, "PRIVATE_UNSELECTED_ITERATION_"+uuid.NewString())
			input := f.submit(t, f.creator, "Retained input across direct source role refusals")
			f.run(t, input, 1)

			actor := f.outsider
			sourceMember, manager := false, false
			switch role {
			case "source_member", "source_owner", "source_participant":
				sourceMember = true
				memberRole := "member"
				if role == "source_owner" {
					memberRole = "owner"
				}
				f.fx.Member(t, f.workspace, actor, memberRole)
			case "external_handler":
				actor, manager = f.handler, true
			case "departed_handler":
				actor, manager = f.handler, true
				f.fx.Member(t, f.workspace, actor, "member")
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, actor)
			case "departed_creator":
				actor, manager = f.creator, true
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, actor)
			}
			if role == "participant" || role == "source_participant" {
				f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": actor}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, actor)
				f.fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": actor}, "user_id=$1", actor)
			}
			ctx := collaborationContext(actor)
			access, err := f.svc.ResolveIssueAccess(ctx, nil, f.issue.Issue.ID)
			if err != nil || !access.Capabilities.Read || access.SourceMember != sourceMember || access.Capabilities.Edit != manager || access.Capabilities.EditSourceStructure {
				t.Fatalf("direct source-role prerequisite mismatch: %+v %v", access, err)
			}
			if (role == "participant" || role == "source_participant") && !access.Capabilities.ControlAI {
				t.Fatal("other collaborator did not retain full participation rights")
			}
			fields, err := f.svc.IssueFields(ctx, access)
			if err != nil || fields.SourceStructure == nil || fields.SourceStructure.Project == nil || fields.SourceStructure.Project.ID != util.UUIDToString(project) ||
				fields.SourceStructure.ParentIssue == nil || fields.SourceStructure.ParentIssue.ID != util.UUIDToString(parent) || fields.SourceStructure.Iteration == nil || fields.SourceStructure.Iteration.ID != util.UUIDToString(period) {
				t.Fatalf("readable source references disappeared: %+v %v", fields.SourceStructure, err)
			}
			encoded := string(fieldJSON(t, fields.SourceStructure))
			for _, private := range []string{privateParentBody, util.UUIDToString(otherProject), util.UUIDToString(otherParent), util.UUIDToString(otherPeriod), `"projects"`, `"parents"`, `"iterations"`, `"url"`, `"path"`, `"email"`, `"username"`} {
				if strings.Contains(encoded, private) {
					t.Fatalf("minimal source reference leaked an unselected/private field: %q", private)
				}
			}
			options, err := f.svc.FieldOptions(ctx, f.issue.Issue.ID)
			if manager {
				if err != nil || options.SourceOptions != nil || strings.Contains(string(fieldJSON(t, options)), util.UUIDToString(otherPeriod)) {
					t.Fatalf("external/departed manager regained source catalogs: %+v %v", options, err)
				}
			} else if !collaborationErrorCode(err, "forbidden") || options.SourceOptions != nil {
				t.Fatalf("nonmanager received editing/source catalogs: %+v %v", options, err)
			}

			for _, domain := range []string{"project", "parent", "stage", "iteration"} {
				t.Run(domain, func(t *testing.T) {
					beforeBusiness, beforeExecution := sourceRoleBusinessSnapshot(t, f), controlStateSnapshot(t, f)
					request := workflowRequest(f, access.Revision)
					kind := "edit_source_structure"
					write := func() (CollaborationWorkflowReceipt, error) {
						if domain == "iteration" {
							kind = "change_iteration"
							return f.svc.ChangeIteration(ctx, CollaborationIterationParams{CollaborationWorkflowRequest: request, CurrentIterationID: otherPeriod,
								ExpectedIssueRevision: access.Issue.Revision, ExpectedIterationRevision: 1, Reason: "No authority to move the source membership"})
						}
						p := CollaborationEditParams{CollaborationWorkflowRequest: request, Title: editString("Must not partially apply")}
						switch domain {
						case "project":
							p.ProjectID = &otherProject
						case "parent":
							p.ParentIssueID = &otherParent
						case "stage":
							p.Stage = &pgtype.Int4{Int32: 3, Valid: true}
						}
						return f.svc.EditIssue(ctx, p)
					}
					receipt, err := write()
					if !collaborationErrorCode(err, "forbidden") || receipt.Code != "forbidden" || receipt.Status != 403 {
						t.Fatalf("direct %s source-role write accepted: %+v %v", domain, receipt, err)
					}
					key := db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID, ActorUserID: util.MustParseUUID(actor), RequestID: request.RequestID}
					original, err := f.svc.Queries.GetCollaborationOperation(ctx, key)
					if err != nil || original.Kind != kind || original.ResultCode.String != "forbidden" {
						t.Fatalf("wrong direct mutation branch/refusal: %+v %v", original, err)
					}
					if _, err := write(); !collaborationErrorCode(err, "forbidden") {
						t.Fatalf("direct refused key changed result: %v", err)
					}
					stored, err := f.svc.Queries.GetCollaborationOperation(ctx, key)
					if err != nil || !reflect.DeepEqual(stored, original) {
						t.Fatalf("refused operation identity/result changed on replay: %+v %v", stored, err)
					}
					if sourceRoleBusinessSnapshot(t, f) != beforeBusiness || controlStateSnapshot(t, f) != beforeExecution {
						t.Fatal("direct source-role refusal/replay partially changed source, business or execution")
					}
				})
			}
		})
	}
}

func TestCollaborationIterationRemovalRequiresReasonAndOriginalRefusal(t *testing.T) {
	for _, role := range []string{"creator", "source_handler"} {
		t.Run(role, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationQueueFixture(t)
			actor := f.creator
			if role == "source_handler" {
				actor = f.handler
				f.fx.Member(t, f.workspace, actor, "member")
			}
			period := sourceFieldIteration(t, f.collaborationFixture, "Reasoned removal source period")
			ctx := collaborationContext(actor)
			if _, err := f.svc.ChangeIteration(ctx, CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, 1), CurrentIterationID: period, ExpectedIssueRevision: f.issue.Issue.Revision, ExpectedIterationRevision: 1}); err != nil {
				t.Fatal(err)
			}
			input := f.submit(t, f.creator, "Frozen AI input survives missing removal reason")
			f.run(t, input, 1)
			joined, err := f.svc.ResolveIssueAccess(ctx, nil, f.issue.Issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforeBusiness, beforeControl := sourceRoleBusinessSnapshot(t, f), controlStateSnapshot(t, f)
			beforeAI := sourceRoleExecutionSnapshot(t, f)
			refused := []CollaborationIterationParams{}
			originals := []db.CollaborationOperation{}
			for _, reason := range []string{"", " \t\r\n"} {
				p := CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, joined.Revision), ExpectedIssueRevision: joined.Issue.Revision, Reason: reason}
				receipt, err := f.svc.ChangeIteration(ctx, p)
				if !collaborationErrorCode(err, "iteration_validation_failed") || receipt.Code != "iteration_validation_failed" || receipt.Status != 422 {
					t.Fatalf("missing/blank removal reason accepted: %+v %v", receipt, err)
				}
				if sourceRoleBusinessSnapshot(t, f) != beforeBusiness || controlStateSnapshot(t, f) != beforeControl {
					t.Fatal("missing removal reason partially changed membership/history/execution")
				}
				key := db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID, ActorUserID: util.MustParseUUID(actor), RequestID: p.RequestID}
				op, err := f.svc.Queries.GetCollaborationOperation(ctx, key)
				if err != nil || op.Kind != "change_iteration" || op.ResultCode.String != "iteration_validation_failed" {
					t.Fatalf("missing removal reason was not a retained exact refusal: %+v %v", op, err)
				}
				changed := p
				changed.Reason = "Explicitly remove from source planning"
				if _, err := f.svc.ChangeIteration(ctx, changed); !collaborationErrorCode(err, "idempotency_conflict") {
					t.Fatalf("same refused key accepted a changed reason: %v", err)
				}
				refused, originals = append(refused, p), append(originals, op)
			}
			remove := CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, joined.Revision), ExpectedIssueRevision: joined.Issue.Revision, Reason: "Explicitly remove from source planning"}
			receipt, err := f.svc.ChangeIteration(ctx, remove)
			if err != nil || receipt.Code != "iteration_changed" || receipt.CollaborationRevision != joined.Revision+1 {
				t.Fatalf("fresh reasoned removal refused: %+v %v", receipt, err)
			}
			after, err := f.svc.ResolveIssueAccess(ctx, nil, f.issue.Issue.ID)
			if err != nil || after.Issue.CurrentIterationID.Valid || after.Issue.Revision != joined.Issue.Revision+1 || after.QueueState != joined.QueueState ||
				after.AuthorityEpoch != joined.AuthorityEpoch || after.LifecycleEpoch != joined.LifecycleEpoch || after.ControlVersion != joined.ControlVersion {
				t.Fatalf("reasoned removal changed responsibility/lifecycle or retained membership: %+v %v", after, err)
			}
			unchanged := after.Issue
			unchanged.CurrentIterationID, unchanged.Revision, unchanged.UpdatedAt = joined.Issue.CurrentIterationID, joined.Issue.Revision, joined.Issue.UpdatedAt
			unchanged.LastActivityAt = joined.Issue.LastActivityAt
			if !reflect.DeepEqual(unchanged, joined.Issue) || sourceRoleExecutionSnapshot(t, f) != beforeAI ||
				f.fx.Count(t, `SELECT count(*) FROM iteration_participation WHERE issue_id=$1 AND current_joined_at IS NOT NULL`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE issue_id=$1 AND reason=$2 AND actor->>'user_id'=$3`, f.issue.Issue.ID, remove.Reason, actor) != 1 {
				t.Fatal("reasoned removal lost exact source reason/actor or changed unrelated issue/AI state")
			}
			finalBusiness, finalControl := sourceRoleBusinessSnapshot(t, f), controlStateSnapshot(t, f)
			var beforeUnchanged, afterUnchanged map[string]json.RawMessage
			if err := json.Unmarshal([]byte(beforeBusiness), &beforeUnchanged); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(finalBusiness), &afterUnchanged); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"issue", "collaboration", "iterations", "participations", "iteration_events"} {
				delete(beforeUnchanged, field)
				delete(afterUnchanged, field)
			}
			if string(fieldJSON(t, beforeUnchanged)) != string(fieldJSON(t, afterUnchanged)) {
				t.Fatal("reasoned removal changed participants, resources, notifications, AI inputs or dependencies")
			}
			for i, p := range refused {
				replayed, err := f.svc.ChangeIteration(ctx, p)
				if !collaborationErrorCode(err, "iteration_validation_failed") || replayed.Code != "iteration_validation_failed" || replayed.CollaborationRevision != joined.Revision {
					t.Fatalf("old missing-reason key recomputed after removal: %+v %v", replayed, err)
				}
				recovered, err := f.svc.GetWorkflowOperation(ctx, f.issue.Issue.ID, p.RequestID)
				if err != nil || recovered.Status != 422 || recovered.Code != "iteration_validation_failed" || recovered.CollaborationRevision != joined.Revision {
					t.Fatalf("own original removal refusal recovery changed: %+v %v", recovered, err)
				}
				stored, err := f.svc.Queries.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID, ActorUserID: util.MustParseUUID(actor), RequestID: p.RequestID})
				if err != nil || !reflect.DeepEqual(stored, originals[i]) {
					t.Fatalf("original removal refusal row was rewritten: %+v %v", stored, err)
				}
			}
			if sourceRoleBusinessSnapshot(t, f) != finalBusiness || controlStateSnapshot(t, f) != finalControl {
				t.Fatal("old refused removal replay/recovery changed the committed source membership")
			}
		})
	}
}
