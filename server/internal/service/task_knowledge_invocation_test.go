package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestKnowledgeUnsafeV1SelectorsRefuseBeforeOrdinaryFallback(t *testing.T) {
	for _, source := range []string{"defaults", "custom"} {
		t.Run(source, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if source == "defaults" {
				f.inv.ExtraArgs = []string{"--continue"}
			} else {
				f.fx.Exec(t, `UPDATE agent SET custom_args='["--continue"]'::jsonb WHERE id=$1`, f.task.AgentID)
			}
			if _, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
				t.Fatalf("unsafe v1 selection silently downgraded: %v", err)
			}
		})
	}
}

func TestKnowledgeOrdinaryNoSessionPersistenceRemainsUnsupported(t *testing.T) {
	for _, source := range []string{"defaults", "custom"} {
		t.Run(source, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if source == "defaults" {
				f.inv.ExtraArgs = []string{"--no-session-persistence"}
			} else {
				f.fx.Exec(t, `UPDATE agent SET custom_args='["--no-session-persistence"]'::jsonb WHERE id=$1`, f.task.AgentID)
			}
			if _, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
				t.Fatalf("ordinary reusable history accepted disabled persistence: %v", err)
			}
		})
	}
}

func TestKnowledgeUnsupportedBackendCannotResumeManagedSessionAcrossScopes(t *testing.T) {
	for _, namespace := range []string{"same_runtime", "same_daemon_new_runtime", "same_owner_new_daemon", "deleted_runtime"} {
		t.Run(namespace, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			f.recordSession(t, "private-history")
			if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "private-history", "/shared/repo", "", false, "", ""); err != nil {
				t.Fatal(err)
			}
			oldRuntime := f.task.RuntimeID
			runtime, err := f.tasks.Queries.GetAgentRuntime(f.ctx, oldRuntime)
			if err != nil {
				t.Fatal(err)
			}
			newRuntime := util.UUIDToString(oldRuntime)
			if namespace != "same_runtime" {
				daemon := runtime.DaemonID.String
				if namespace == "same_owner_new_daemon" {
					daemon = "replacement-" + uuid.NewString()
				}
				profile := f.fx.Insert(t, "runtime_profile", testutil.Cols{"workspace_id": f.workspace, "display_name": "Alternate Claude family", "protocol_family": "claude", "command_name": "test-owned-wrapper", "created_by": f.creator, "fixed_args": []byte(`["--continue"]`)})
				newRuntime = f.fx.Runtime(t, "Other protocol-family runtime", testutil.Cols{"runtime_mode": "local", "provider": "claude", "daemon_id": daemon, "profile_id": profile})
			}
			if namespace == "deleted_runtime" {
				f.fx.Exec(t, `UPDATE agent_task_queue SET runtime_id=NULL WHERE id=$1`, f.task.ID)
				f.fx.Exec(t, `UPDATE agent SET runtime_id=NULL WHERE id=$1`, f.task.AgentID)
				f.fx.Exec(t, `DELETE FROM agent_runtime WHERE id=$1`, oldRuntime)
			}
			agentID := f.fx.Agent(t, "Different-scope legacy agent", newRuntime)
			issueID := f.fx.Issue(t, "Different task scope", testutil.Cols{"assignee_type": "agent", "assignee_id": agentID})
			id := f.fx.Task(t, agentID, testutil.Cols{"runtime_id": newRuntime, "issue_id": issueID, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1})
			task, err := f.tasks.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, task, nil, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
				t.Fatalf("implicit continuation could restore another scope's history: %v", err)
			}
		})
	}
}

func TestKnowledgeValidEmptyAndClaimedHistoryPreserveLegacyWork(t *testing.T) {
	for _, phase := range []string{"claimed", "empty_disclosure"} {
		t.Run(phase, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if phase == "claimed" {
				f.finalize(t, "")
				if _, err := f.tasks.CancelTaskWithResult(f.ctx, f.task.ID, CancelTaskOptions{UserInitiated: true}); err != nil {
					t.Fatal(err)
				}
			} else {
				f.withdraw(t, f.candidate.CandidateID)
				f.recordSession(t, "empty-native")
				if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "empty-native", "", "", false, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			f.nextTurn(t)
			d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, nil, "empty-native")
			if err != nil || d != nil || prior != "empty-native" {
				t.Fatalf("no-exposure history reset legacy work: %q %v", prior, err)
			}
		})
	}
}

func TestKnowledgeRuntimeHistoryRefusesCorruptOrTruncatedProof(t *testing.T) {
	for _, condition := range []string{"corrupt", "truncated"} {
		t.Run(condition, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			f.finalize(t, "")
			row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
			if err != nil {
				t.Fatal(err)
			}
			meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
			if err != nil {
				t.Fatal(err)
			}
			if condition == "corrupt" {
				f.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_delivery='{}'::jsonb WHERE id=$1`, row.ID)
			} else {
				for range 129 {
					id := uuid.NewString()
					meta.Claims[0].Descriptor.TaskID = id
					raw, err := encodeTaskKnowledge(meta)
					if err != nil {
						t.Fatal(err)
					}
					f.fx.Task(t, util.UUIDToString(row.AgentID), testutil.Cols{"id": id, "runtime_id": row.RuntimeID, "issue_id": row.IssueID, "status": "completed", "claim_generation": 1, "knowledge_context_version": 1, "knowledge_delivery": raw})
				}
			}
			newAgent := f.fx.Agent(t, "Legacy history guard", util.UUIDToString(row.RuntimeID))
			issue := f.fx.Issue(t, "Unrelated scope")
			id := f.fx.Task(t, newAgent, testutil.Cols{"runtime_id": row.RuntimeID, "issue_id": issue, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1})
			task, err := f.tasks.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, task, nil, ""); !collaborationErrorCode(err, "knowledge_context_changed") {
				t.Fatalf("unprovable runtime history became legacy: %v", err)
			}
		})
	}
}

func TestKnowledgeClaimDoesNotNegotiateAfterProviderChanges(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "")
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_runtime SET provider='opencode' WHERE id=$1`, f.task.RuntimeID)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if err = f.tasks.finalizeTaskKnowledgeClaim(f.ctx, f.tasks.Queries.WithTx(tx), f.task, d); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("changed effective provider was finalized: %v", err)
	}
}
