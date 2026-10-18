package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// All identities, grants and runs here are synthetic persisted prerequisites.
// These fixtures do not enroll a supervisor, fork a process or prove termination.
func seedCollaborationTarget(t *testing.T, fx *testutil.Fixture, issue, session, owner pgtype.UUID, epoch int64) (db.CollaborationTarget, db.CollaborationGrant) {
	t.Helper()
	if fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, fx.WorkspaceID, owner) == 0 {
		fx.Member(t, fx.WorkspaceID, util.UUIDToString(owner), "member")
	}
	runtime := fx.Runtime(t, "Synthetic offline collaboration runtime", testutil.Cols{"owner_id": owner, "status": "offline"})
	agent := fx.Agent(t, "Synthetic collaboration target "+uuid.NewString(), runtime, testutil.Cols{"owner_id": owner})
	resource := collaboration.TargetResource{Role: "agent", AgentID: agent, RuntimeID: runtime, ExecutionWorkspaceID: fx.WorkspaceID, OwnerUserID: util.UUIDToString(owner)}
	roster, err := json.Marshal([]collaboration.TargetResource{resource})
	if err != nil {
		t.Fatal(err)
	}
	id := util.MustParseUUID(fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": agent, "roster": roster}))
	fx.Exec(t, `UPDATE collaboration_session SET target_set_version=target_set_version+1 WHERE id=$1`, session)
	grant := seedCollaborationGrant(t, fx, issue, id, 1, resource, epoch)
	var target db.CollaborationTarget
	all, err := db.New(fx.Pool).ListCollaborationTargets(t.Context(), session)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range all {
		if row.ID == id {
			target = row
		}
	}
	return target, grant
}

func seedCollaborationGrant(t *testing.T, fx *testutil.Fixture, issue, target pgtype.UUID, config int64, resource collaboration.TargetResource, epoch int64) db.CollaborationGrant {
	t.Helper()
	if auth.ManagedDeploymentID() == "" {
		t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
	}
	if fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, resource.ExecutionWorkspaceID, resource.OwnerUserID) == 0 {
		fx.Member(t, resource.ExecutionWorkspaceID, resource.OwnerUserID, "member")
	}
	q := db.New(fx.Pool)
	var targetRow db.CollaborationTarget
	session, err := q.GetCollaborationSession(t.Context(), issue)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := q.ListCollaborationTargets(t.Context(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range targets {
		if row.ID == target {
			targetRow = row
		}
	}
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](targetRow.Roster)
	if err != nil {
		t.Fatal(err)
	}
	configuration, code, err := collaborationConfigurationSnapshot(t.Context(), q, collaboration.TargetKind(targetRow.Kind), targetRow.ResourceID, roster)
	if err != nil || code != "" {
		t.Fatalf("synthetic resource setup: %s %v", code, err)
	}
	fx.Exec(t, `UPDATE collaboration_target SET configuration_fingerprint=$2 WHERE id=$1`, target, configuration.Fingerprint)
	participant, err := q.GetCollaborationParticipant(t.Context(), db.GetCollaborationParticipantParams{IssueID: issue, UserID: util.MustParseUUID(resource.OwnerUserID)})
	if err != nil {
		t.Fatal(err)
	}
	var binding, installation string
	err = fx.Pool.QueryRow(t.Context(), `SELECT id::text,installation_id::text FROM collaboration_runtime_enrollment WHERE runtime_id=$1 AND revoked_at IS NULL`, resource.RuntimeID).Scan(&binding, &installation)
	if errors.Is(err, pgx.ErrNoRows) {
		installation = uuid.NewString()
		binding = fx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{
			"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": resource.OwnerUserID, "execution_workspace_id": resource.ExecutionWorkspaceID,
			"runtime_id": resource.RuntimeID, "installation_id": installation, "public_key": make([]byte, 32),
		})
	} else if err != nil {
		t.Fatal(err)
	}
	id := fx.Insert(t, "collaboration_grant", testutil.Cols{
		"issue_id": issue, "target_id": target, "config_version": config, "role": resource.Role,
		"owner_user_id": resource.OwnerUserID, "owner_auth_epoch": epoch, "execution_workspace_id": resource.ExecutionWorkspaceID,
		"agent_id": resource.AgentID, "runtime_id": resource.RuntimeID, "identity_kind": "enrolled",
		"installation_id": installation, "binding_id": binding, "binding_epoch": 1,
		"target_epoch": targetRow.TargetEpoch, "owner_participant_epoch": participant.Epoch, "configuration_fingerprint": configuration.Fingerprint,
	})
	grants, err := db.New(fx.Pool).LockCollaborationTargetGrants(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	for _, grant := range grants {
		if util.UUIDToString(grant.ID) == id {
			return grant
		}
	}
	t.Fatal("synthetic grant missing")
	return db.CollaborationGrant{}
}

func seedCollaborationGroupRun(t *testing.T, fx *testutil.Fixture, session pgtype.UUID, command CollaborationQueueReceipt, attempts int64) db.CollaborationRun {
	t.Helper()
	q := db.New(fx.Pool)
	batches, err := q.ListCollaborationBatches(t.Context(), session)
	if err != nil {
		t.Fatal(err)
	}
	var batch db.CollaborationBatch
	for _, item := range batches {
		if item.ID == command.BatchID {
			batch = item
		}
	}
	if !batch.ID.Valid || batch.State != "waiting" {
		t.Fatal("synthetic run requires a waiting batch")
	}
	inputs, err := q.ListCollaborationBatchInputs(t.Context(), batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range inputs {
		if input.TargetID == batch.EffectiveTargetID && input.State == "waiting" {
			command.CommandID = input.ID
			break
		}
	}
	targets, err := q.ListCollaborationTargets(t.Context(), session)
	if err != nil {
		t.Fatal(err)
	}
	var target db.CollaborationTarget
	for _, item := range targets {
		if item.ID == batch.EffectiveTargetID {
			target = item
		}
	}
	if !target.ID.Valid {
		t.Fatal("synthetic run target missing")
	}
	grants, err := q.LockCollaborationTargetGrants(t.Context(), target.ID)
	if err != nil {
		t.Fatal(err)
	}
	var root db.CollaborationGrant
	roster := make([]collaboration.GroupResource, 0)
	for _, grant := range grants {
		if grant.RevokedAt.Valid || !grant.ConfigVersion.Valid || grant.ConfigVersion.Int64 != target.ConfigVersion {
			continue
		}
		resource := collaboration.TargetResource{Role: grant.Role.String, AgentID: util.UUIDToString(grant.AgentID), RuntimeID: util.UUIDToString(grant.RuntimeID),
			ExecutionWorkspaceID: util.UUIDToString(grant.ExecutionWorkspaceID), OwnerUserID: util.UUIDToString(grant.OwnerUserID)}
		roster = append(roster, collaboration.GroupResource{TargetResource: resource, GrantID: util.UUIDToString(grant.ID), GrantEpoch: grant.Epoch})
		if grant.Role.String == "agent" || grant.Role.String == "leader" {
			root = grant
		}
	}
	if !root.ID.Valid {
		t.Fatal("synthetic root grant missing")
	}
	encoded, err := json.Marshal(roster)
	if err != nil {
		t.Fatal(err)
	}
	group := util.MustParseUUID(fx.Insert(t, "collaboration_execution_group", testutil.Cols{
		"session_id": session, "target_id": target.ID, "command_id": command.CommandID, "generation": target.GroupGeneration + 1,
		"target_epoch": target.TargetEpoch, "config_version": target.ConfigVersion, "roster": encoded,
		"channel_id": batch.ChannelID, "batch_id": batch.ID,
	}))
	id := util.MustParseUUID(fx.Insert(t, "collaboration_run", testutil.Cols{
		"session_id": session, "command_id": command.CommandID, "target_id": target.ID, "group_id": group,
		"step_key": "root", "step_kind": root.Role.String, "agent_id": root.AgentID,
		"target_epoch": target.TargetEpoch, "config_version": target.ConfigVersion, "grant_id": root.ID, "grant_epoch": root.Epoch,
		"resource_epoch": 1, "execution_workspace_id": root.ExecutionWorkspaceID, "runtime_id": root.RuntimeID,
		"installation_id": root.InstallationID, "binding_id": root.BindingID, "binding_epoch": root.BindingEpoch, "authorized_attempt_seq": attempts,
		"channel_id": batch.ChannelID, "batch_id": batch.ID,
	}))
	fx.Exec(t, `UPDATE collaboration_batch SET state='in_flight',root_command_id=$2,group_id=$3,started_at=now() WHERE id=$1`, batch.ID, command.CommandID, group)
	fx.Exec(t, `UPDATE collaboration_channel SET active_group_id=$2,group_generation=$3,queue_state='active' WHERE id=$1`, batch.ChannelID, group, target.GroupGeneration+1)
	if err = q.StartCollaborationBatchInputs(t.Context(), batch.ID); err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_group_input WHERE group_id=$1`, group)
	fx.Cleanup(t, `DELETE FROM collaboration_execution_group WHERE id=$1`, group)
	if _, err = q.MirrorCollaborationChannelControl(t.Context(), batch.ChannelID); err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, `UPDATE collaboration_session SET queue_state='active' WHERE id=$1`, session)
	run, err := q.GetCollaborationRun(t.Context(), db.GetCollaborationRunParams{SessionID: session, RunID: id})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// Reconfiguration is a test prerequisite for later runtime-binding work. It
// changes the actual resource and records a fresh owner grant, keeping the same
// typed agent target and old immutable inputs/history.
func rebindCollaborationTestTarget(t *testing.T, fx *testutil.Fixture, issue, session, targetID, owner pgtype.UUID, epoch int64) db.CollaborationGrant {
	t.Helper()
	targets, err := db.New(fx.Pool).ListCollaborationTargets(t.Context(), session)
	if err != nil {
		t.Fatal(err)
	}
	var target db.CollaborationTarget
	for _, item := range targets {
		if item.ID == targetID {
			target = item
		}
	}
	if !target.ID.Valid || target.ActiveGroupID.Valid {
		t.Fatal("test reconfiguration requires an idle existing target")
	}
	runtime := fx.Runtime(t, "Replacement synthetic runtime", testutil.Cols{"owner_id": owner, "status": "offline"})
	fx.Exec(t, `UPDATE agent SET runtime_id=$2, owner_id=$3 WHERE id=$1`, target.ResourceID, runtime, owner)
	resource := collaboration.TargetResource{Role: "agent", AgentID: util.UUIDToString(target.ResourceID), RuntimeID: runtime,
		ExecutionWorkspaceID: fx.WorkspaceID, OwnerUserID: util.UUIDToString(owner)}
	roster, err := json.Marshal([]collaboration.TargetResource{resource})
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now(),revocation_reason='fixture_reconfiguration' WHERE target_id=$1 AND revoked_at IS NULL`, target.ID)
	fx.Exec(t, `UPDATE collaboration_target SET config_version=config_version+1,roster=$2 WHERE id=$1`, target.ID, roster)
	return seedCollaborationGrant(t, fx, issue, target.ID, target.ConfigVersion+1, resource, epoch)
}
