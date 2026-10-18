package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func sharedConfigurationSkill(t *testing.T, f *configInvalidationFixture) pgtype.UUID {
	t.Helper()
	skill := util.MustParseUUID(f.fx.Insert(t, "skill", testutil.Cols{"workspace_id": f.workspace, "name": "Shared configuration " + uuid.NewString(), "content": "original instructions", "created_by": f.creator}))
	if err := f.svc.Queries.AddAgentSkill(f.ctx, db.AddAgentSkillParams{AgentID: f.grant.AgentID, SkillID: skill}); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM agent_skill WHERE skill_id=$1`, skill)
	refreshSkillConsent(t, f, f.targetID, f.grant.AgentID)
	f.drain(t)
	return skill
}

func refreshSkillConsent(t *testing.T, f *configInvalidationFixture, targetID, agent pgtype.UUID) db.CollaborationGrant {
	t.Helper()
	target := queueTarget(t, f.collaborationQueueFixture, targetID)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ReconfigureTarget(f.ctx, CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(target), Roster: roster}); err != nil {
		t.Fatal(err)
	}
	grant := consentManagedControlTarget(t, f.managedCollaborationFixture, queueTarget(t, f.collaborationQueueFixture, targetID), agent)
	if targetID == f.targetID {
		f.grant = grant
	}
	return grant
}

func TestCollaborationSharedSkillWritersInvalidateEnabledDependents(t *testing.T) {
	for _, change := range []string{"body", "version", "file_add", "file_update", "file_delete", "file_move", "delete"} {
		t.Run(change, func(t *testing.T) {
			f := newConfigInvalidationFixture(t)
			skill := sharedConfigurationSkill(t, f)
			q := f.svc.Queries
			file, err := q.UpsertSkillFile(f.ctx, db.UpsertSkillFileParams{SkillID: skill, Path: "references/guide.md", Content: "original file"})
			if err != nil {
				t.Fatal(err)
			}
			refreshSkillConsent(t, f, f.targetID, f.grant.AgentID)
			alias, aliasGrant := registerManagedControlTarget(t, f.managedCollaborationFixture, true)
			other, otherGrant := registerManagedControlTarget(t, f.managedCollaborationFixture, false)
			f.owned = append(f.owned, alias.ResourceID, otherGrant.AgentID)
			if err = q.AddAgentSkill(f.ctx, db.AddAgentSkillParams{AgentID: otherGrant.AgentID, SkillID: skill}); err != nil {
				t.Fatal(err)
			}
			if _, err = q.SetAgentSkillEnabled(f.ctx, db.SetAgentSkillEnabledParams{AgentID: otherGrant.AgentID, SkillID: skill, Enabled: false}); err != nil {
				t.Fatal(err)
			}
			otherGrant = refreshSkillConsent(t, f, other.ID, otherGrant.AgentID)
			// A second authorized target proves fanout beyond the first agent.
			boundTarget, boundGrant := registerManagedControlTarget(t, f.managedCollaborationFixture, false)
			bound := boundGrant.AgentID
			f.owned = append(f.owned, bound)
			if err = q.AddAgentSkill(f.ctx, db.AddAgentSkillParams{AgentID: bound, SkillID: skill}); err != nil {
				t.Fatal(err)
			}
			boundGrant = refreshSkillConsent(t, f, boundTarget.ID, bound)
			f.drain(t)
			beforeBound, _ := q.GetAgent(f.ctx, bound)
			beforeDisabled, _ := q.GetAgent(f.ctx, otherGrant.AgentID)
			run, claim, permit := startResourceCollaboration(t, f.managedCollaborationFixture)
			other = queueTarget(t, f.collaborationQueueFixture, other.ID)
			waiting := f.submitManaged(t, f.handler, "keep waiting through shared skill replacement")
			channel := resourceChannelState(t, f.managedCollaborationFixture)
			switch change {
			case "body":
				_, err = q.UpdateSkill(f.ctx, db.UpdateSkillParams{ID: skill, Content: pgtype.Text{String: "new instructions", Valid: true}})
			case "version":
				_, err = q.UpdateSkill(f.ctx, db.UpdateSkillParams{ID: skill, Config: []byte(`{"origin":{"type":"clawhub","version":"2.0.0"}}`)})
			case "file_add":
				_, err = q.UpsertSkillFile(f.ctx, db.UpsertSkillFileParams{SkillID: skill, Path: "scripts/run.sh", Content: "new executable instructions"})
			case "file_update":
				_, err = q.UpsertSkillFile(f.ctx, db.UpsertSkillFileParams{SkillID: skill, Path: file.Path, Content: "changed file instructions"})
			case "file_delete":
				err = q.DeleteSkillFile(f.ctx, file.ID)
			case "file_move":
				_, err = f.pool.Exec(f.ctx, `UPDATE skill_file SET path='references/renamed.md' WHERE id=$1`, file.ID)
			case "delete":
				err = q.DeleteSkill(f.ctx, db.DeleteSkillParams{ID: skill, WorkspaceID: util.MustParseUUID(f.workspace)})
			}
			if err != nil {
				t.Fatal(err)
			}
			afterBound, _ := q.GetAgent(f.ctx, bound)
			afterDisabled, _ := q.GetAgent(f.ctx, otherGrant.AgentID)
			if afterBound.CollaborationConfigEpoch <= beforeBound.CollaborationConfigEpoch || afterDisabled.CollaborationConfigEpoch != beforeDisabled.CollaborationConfigEpoch {
				t.Fatal("shared skill change did not advance exactly the enabled agent bindings")
			}
			target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			if _, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: "stale skill authority"}); !collaborationErrorCode(err, "target_configuration_changed") {
				t.Fatalf("shared skill commit did not immediately reject obsolete admission: %v", err)
			}
			// A new consumer must discover the committed writer without callbacks.
			f.worker = NewCollaborationConfigReconciler(q, f.pool)
			f.drain(t)
			if resourceChannelState(t, f.managedCollaborationFixture) != channel || !reflect.DeepEqual(other, queueTarget(t, f.collaborationQueueFixture, other.ID)) || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL`, otherGrant.ID) != 1 {
				t.Fatal("skill invalidation changed lane pause or a disabled-binding target")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=ANY($1::uuid[]) AND hold_reason='config_invalid'`, []pgtype.UUID{f.targetID, alias.ID, boundTarget.ID}) != 3 || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=ANY($1::uuid[]) AND revoked_at IS NOT NULL`, []pgtype.UUID{f.grant.ID, aliasGrant.ID, boundGrant.ID}) != 3 || f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 {
				t.Fatal("skill invalidation omitted a bound direct/squad target or consumed waiting input")
			}
			stop, err := f.svc.RuntimeStopIntent(t.Context(), run, claim.TerminationToken)
			if err != nil || !stop.StopRequested || stop.Identity != claim.Identity {
				t.Fatalf("skill invalidation lost original supervisor stop identity: %+v %v", stop, err)
			}
			if _, err = f.svc.AcceptRuntimeProof(t.Context(), run, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND settled_at IS NOT NULL`, run) != 1 {
				t.Fatal("original skill-dependent run could not settle with its retained proof")
			}
		})
	}
}

func TestCollaborationSharedSkillOldIntentPreservesLaterConsent(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	skill := sharedConfigurationSkill(t, f)
	if _, err := f.svc.Queries.UpdateSkill(f.ctx, db.UpdateSkillParams{ID: skill, Content: pgtype.Text{String: "new content", Valid: true}}); err != nil {
		t.Fatal(err)
	}
	fresh := refreshSkillConsent(t, f, f.targetID, f.grant.AgentID)
	run, claim, _ := startResourceCollaboration(t, f.managedCollaborationFixture)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	f.worker = NewCollaborationConfigReconciler(f.svc.Queries, f.pool)
	f.drain(t)
	stop, err := f.svc.RuntimeStopIntent(t.Context(), run, claim.TerminationToken)
	if err != nil || stop.StopRequested || before != controlStateSnapshot(t, f.collaborationQueueFixture) || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL`, fresh.ID) != 1 {
		t.Fatalf("delayed shared-skill intent invalidated a fresh authorized run: %+v %v", stop, err)
	}
}

func TestCollaborationSharedSkillWriterFencesConcurrentBindings(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	skill := sharedConfigurationSkill(t, f)
	other := util.MustParseUUID(f.fx.Agent(t, "Concurrent skill binding "+uuid.NewString(), util.UUIDToString(f.runtime.ID)))
	f.owned = append(f.owned, other)
	for _, change := range []string{"bind", "file"} {
		t.Run(change, func(t *testing.T) {
			tx, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(f.ctx, `UPDATE skill SET content='uncommitted change' WHERE id=$1`, skill); err != nil {
				t.Fatal(err)
			}
			if change == "bind" {
				err = f.svc.Queries.AddAgentSkill(f.ctx, db.AddAgentSkillParams{AgentID: other, SkillID: skill})
			} else {
				_, err = f.svc.Queries.UpsertSkillFile(f.ctx, db.UpsertSkillFileParams{SkillID: skill, Path: "concurrent.md", Content: "uncommitted bundle"})
			}
			var conflict *pgconn.PgError
			if !errors.As(err, &conflict) || conflict.Code != "55P03" {
				t.Fatalf("writer did not fail atomically at the shared skill fence: %v", err)
			}
			if err = tx.Rollback(f.ctx); err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM agent_skill WHERE agent_id=$1 AND skill_id=$2`, other, skill) != 0 || f.fx.Count(t, `SELECT count(*) FROM skill_file WHERE skill_id=$1`, skill) != 0 {
				t.Fatal("conflicting skill writer left a partial binding or bundle")
			}
		})
	}
}

func TestCollaborationSharedSkillWriterRollsBackAgainstAdmissionFence(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	skill := sharedConfigurationSkill(t, f)
	before := f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.grant.AgentID)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(f.ctx, `SELECT id FROM agent WHERE id=$1 FOR UPDATE`, f.grant.AgentID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Queries.UpdateSkill(f.ctx, db.UpdateSkillParams{ID: skill, Content: pgtype.Text{String: "must roll back", Valid: true}})
	var conflict *pgconn.PgError
	if !errors.As(err, &conflict) || conflict.Code != "55P03" {
		t.Fatalf("skill writer did not reject the occupied admission fence: %v", err)
	}
	if err = tx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.grant.AgentID) != before || f.fx.Count(t, `SELECT count(*) FROM skill WHERE id=$1 AND content='original instructions'`, skill) != 1 {
		t.Fatal("conflicting shared skill write committed a partial content or authority change")
	}
}
