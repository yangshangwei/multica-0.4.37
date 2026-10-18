package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationWholeControlRejectsMissingTargetWithRetainedRun(t *testing.T) {
	for _, scenario := range []struct {
		name, action string
		removeGroup  bool
	}{
		{name: "whole_stop", action: "stop"},
		{name: "complete", action: "complete"},
		{name: "whole_stop_without_group", action: "stop", removeGroup: true},
		{name: "target_scope_mismatch", action: "target_stop"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCollaborationLifecycleFixture(t)
			run := f.run(t, f.submit(t, f.creator, "retained possibly live execution"), 1)
			targetStop := f.stopParams(run, 1)
			// Deliberate fixture corruption: retained work still belongs to this
			// issue even when its target row can no longer be enumerated.
			f.fx.Exec(t, `DELETE FROM collaboration_target WHERE id=$1`, run.TargetID)
			if scenario.removeGroup {
				f.fx.Exec(t, `DELETE FROM collaboration_execution_group WHERE id=$1`, run.GroupID)
			}
			beforeEvents := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID)
			beforeOperations := f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, f.issue.Issue.ID)
			var err error
			switch scenario.action {
			case "stop":
				_, err = f.svc.StopQueue(f.ctx, f.issueControl(t))
			case "target_stop":
				_, err = f.svc.StopQueue(f.ctx, targetStop)
				if !collaborationErrorCode(err, "stale_scope") {
					t.Fatalf("missing selected target must retain the normal scope refusal: %v", err)
				}
			default:
				_, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "complete", "done", 1, 1))
			}
			if err == nil {
				t.Fatal("whole-issue control committed while silently omitting a retained run with a missing target")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='active' AND control_version=1`, f.issue.SessionID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND proof_state='unknown' AND settled_at IS NULL`, run.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight'`, run.CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=1 AND lifecycle_epoch=1`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status=$2`, f.issue.Issue.ID, f.issue.Issue.Status) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != beforeEvents {
				t.Fatal("refused whole-issue control partially changed the retained state")
			}
			if scenario.action != "target_stop" && f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, f.issue.Issue.ID) != beforeOperations {
				t.Fatal("integrity failure committed a partial operation receipt")
			}
		})
	}
}

func TestCollaborationSquadInputRechecksEveryResourceConsent(t *testing.T) {
	for _, scenario := range []struct{ name, code string }{
		{name: "complete_offline_roster"},
		{name: "missing_member_consent", code: "grant_required"},
		{name: "member_owner_epoch_changed", code: "grant_required"},
		{name: "member_runtime_owner_changed", code: "target_configuration_changed"},
		{name: "member_enrollment_revoked", code: "target_configuration_changed"},
		{name: "squad_leader_changed", code: "target_configuration_changed"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			squad, member := seedQueueSquad(t, f, true)
			switch scenario.name {
			case "missing_member_consent":
				f.fx.Exec(t, `DELETE FROM collaboration_grant WHERE id=$1`, member.ID)
			case "member_owner_epoch_changed":
				f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, member.OwnerUserID)
			case "member_runtime_owner_changed":
				f.fx.Exec(t, `UPDATE agent_runtime SET owner_id=$2 WHERE id=$1`, member.RuntimeID, f.outsider)
			case "member_enrollment_revoked":
				f.fx.Exec(t, `UPDATE collaboration_runtime_enrollment SET revoked_at=now() WHERE id=$1`, member.BindingID)
			case "squad_leader_changed":
				f.fx.Exec(t, `UPDATE squad SET leader_id=$2 WHERE id=$1`, squad.ResourceID, member.AgentID)
			}
			r, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{
				CollaborationQueueRequest: f.request(), TargetID: squad.ID, InputEventID: util.MustParseUUID(uuid.NewString()),
				ExpectedTargetEpoch: squad.TargetEpoch, ExpectedConfigVersion: squad.ConfigVersion, Content: "one complete squad input",
			})
			wantCommands := 0
			if scenario.code == "" {
				if err != nil || !r.CommandID.Valid || r.Code != "command_queued" {
					t.Fatalf("fully consented offline squad must retain waiting input: %+v, %v", r, err)
				}
				wantCommands = 1
			} else if !collaborationErrorCode(err, scenario.code) || r.CommandID.Valid {
				t.Fatalf("invalid squad member configuration must refuse input with %s: %+v, %v", scenario.code, r, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1 AND state='waiting'`, squad.ID) != wantCommands ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1`, f.issue.SessionID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='paused'`, f.issue.SessionID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='paused' AND active_group_id IS NULL`, squad.ID) != 1 {
				t.Fatal("input consent checking created partial work or changed a pause")
			}
		})
	}
}

func TestCollaborationWholeStopRejectsMissingCurrentGroup(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	missing := util.MustParseUUID(uuid.NewString())
	f.fx.Exec(t, `UPDATE collaboration_target SET active_group_id=$2,group_generation=1 WHERE id=$1`, f.targetID, missing)
	beforeEvents := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID)
	r, err := f.svc.StopQueue(f.ctx, f.issueControl(t))
	if err == nil {
		t.Fatalf("missing current execution group was reported as settled/ended: %+v", r)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='paused' AND control_version=1 AND active_group_id=$2`, f.targetID, missing) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != beforeEvents {
		t.Fatal("missing current group failure changed the pause or historical barrier")
	}
}

func TestCollaborationGroupStopRejectsCorruptDescendantLineage(t *testing.T) {
	for _, scenario := range []string{"foreign_parent", "cyclic_parents", "wrong_command"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			squad, member := seedQueueSquad(t, f, true)
			root := f.run(t, submitTarget(t, f, squad.ID, f.creator, "squad input", util.MustParseUUID(uuid.NewString())), 0)
			first, second := uuid.NewString(), uuid.NewString()
			cols := testutil.Cols{
				"id": first, "session_id": root.SessionID, "target_id": root.TargetID, "group_id": root.GroupID, "command_id": root.CommandID,
				"parent_run_id": second, "step_key": "review-member-1", "step_kind": "member", "agent_id": member.AgentID,
				"target_epoch": root.TargetEpoch.Int64, "config_version": root.ConfigVersion.Int64, "grant_id": member.ID, "grant_epoch": member.Epoch,
				"resource_epoch": 1, "execution_workspace_id": member.ExecutionWorkspaceID, "runtime_id": member.RuntimeID,
				"installation_id": member.InstallationID, "binding_id": member.BindingID, "binding_epoch": member.BindingEpoch,
			}
			if scenario == "wrong_command" {
				other := f.submit(t, f.creator, "independent input")
				cols["parent_run_id"], cols["command_id"] = root.ID, other.CommandID
			}
			f.fx.Insert(t, "collaboration_run", cols)
			if scenario == "cyclic_parents" {
				cols["id"], cols["parent_run_id"], cols["step_key"] = second, first, "review-member-2"
				f.fx.Insert(t, "collaboration_run", cols)
			}
			beforeEvents := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID)
			if _, err := f.svc.StopQueue(f.ctx, f.stopParams(root, 1)); err == nil {
				t.Fatal("corrupt descendant lineage was accepted as a complete execution group")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='active' AND control_version=1 AND active_group_id=$2`, squad.ID, root.GroupID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND expansion_sealed_at IS NULL AND settled_at IS NULL AND forced_disposition IS NULL`, root.GroupID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE group_id=$1 AND (proof_state<>'unknown' OR settled_at IS NOT NULL OR forced_disposition IS NOT NULL)`, root.GroupID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != beforeEvents {
				t.Fatal("corrupt lineage failure partially stopped or settled the group")
			}
		})
	}
}
