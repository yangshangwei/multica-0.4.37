package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeOldClaimZeroSessionReportsCannotMutateReclaimedTask(t *testing.T) {
	for _, mode := range []string{"negotiated", "downgraded", "gate_closed", "no_current_metadata"} {
		t.Run(mode, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			old := f.finalize(t, "")
			oldFence := &protocol.TaskKnowledgeFence{ClaimGeneration: old.ClaimGeneration, DispatchedAt: old.DispatchedAt}
			f.fx.Exec(t, `UPDATE agent_task_queue SET claim_generation=2,dispatched_at=now() WHERE id=$1`, f.task.ID)
			var err error
			f.task, err = f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "negotiated":
				f.finalize(t, "")
			case "gate_closed":
				f.tasks.KnowledgeContextEnabled = false
			case "no_current_metadata":
				f.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_context_version=NULL,knowledge_delivery=NULL WHERE id=$1`, f.task.ID)
			}
			if _, err = f.tasks.StartTask(f.ctx, f.task.ID, oldFence); !collaborationErrorCode(err, "knowledge_context_changed") {
				t.Fatalf("old start fence accepted: %v", err)
			}
			if _, err = f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{"output":"old completion"}`), "", "", "", false, "", "", oldFence); !collaborationErrorCode(err, "knowledge_context_changed") {
				t.Fatalf("old zero-ID completion accepted: %v", err)
			}
			if _, err = f.tasks.FailTask(f.ctx, f.task.ID, "knowledge_context_changed", "", "", "", "knowledge_context_changed", false, "", "", oldFence); !collaborationErrorCode(err, "knowledge_context_changed") {
				t.Fatalf("old zero-ID failure accepted: %v", err)
			}
			if err = ValidateTaskKnowledgeReportFence(f.ctx, f.tasks.Queries, f.task.ID, oldFence); !collaborationErrorCode(err, "knowledge_context_changed") {
				t.Fatalf("old pin fence accepted: %v", err)
			}
			row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if row.Status != "dispatched" || row.ClaimGeneration != 2 || row.StartedAt.Valid {
				t.Fatal("stale report changed newer claim state")
			}
			if mode != "no_current_metadata" {
				if _, err = f.tasks.StartTask(f.ctx, f.task.ID, nil); !collaborationErrorCode(err, "knowledge_context_changed") {
					t.Fatalf("missing fence downgraded retained history: %v", err)
				}
				if mode != "negotiated" {
					if _, _, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, row, nil, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
						t.Fatalf("admitted an unfenced reclaimed legacy claim: %v", err)
					}
				}
			}
			currentFence := &protocol.TaskKnowledgeFence{ClaimGeneration: strconv.FormatInt(row.ClaimGeneration, 10), DispatchedAt: row.DispatchedAt.Time.UTC().Format(time.RFC3339Nano)}
			if _, err = f.tasks.StartTask(f.ctx, row.ID, currentFence); err != nil {
				t.Fatalf("current scoped start rejected: %v", err)
			}
			if _, err = f.tasks.FailTask(f.ctx, row.ID, "knowledge_context_changed", "", "", "", "knowledge_context_changed", false, "", "", currentFence); err != nil {
				t.Fatalf("current scoped failure rejected: %v", err)
			}
		})
	}
}

func TestKnowledgeTrustedServerFailureCanSettleRetainedTask(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	f.finalize(t, "")
	if _, err := f.tasks.FailTask(f.ctx, f.task.ID, "knowledge_unavailable", "", "", "", "knowledge_unavailable", false, "", ""); err != nil {
		t.Fatal(err)
	}
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "failed" || row.FailureReason.String != "knowledge_unavailable" {
		t.Fatal("trusted server could not settle its own refusal")
	}
}
