package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Claimed-only turns carry no disclosure. Repeating that valid history must
// neither manufacture exposure nor hide an older exposure or damaged row.
func (f *taskKnowledgeFixture) insertClaimedHistory(t *testing.T, count int) {
	t.Helper()
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil || meta == nil {
		t.Fatalf("history seed is not negotiated: %v", err)
	}
	meta.Claims = meta.Claims[:1]
	meta.Claims[0].Phase = "claimed"
	meta.Claims[0].Prepared = nil
	meta.Claims[0].Attempts = []taskKnowledgeAttempt{}
	meta.Claims[0].Descriptor.Session.NativeSessionID = ""
	meta.Claims[0].Descriptor.Session.Generation = uuid.NewString()
	for range count {
		id := uuid.NewString()
		meta.Claims[0].Descriptor.TaskID = id
		raw, err := encodeTaskKnowledge(meta)
		if err != nil {
			t.Fatal(err)
		}
		f.fx.Task(t, util.UUIDToString(row.AgentID), testutil.Cols{"id": id, "runtime_id": row.RuntimeID, "issue_id": row.IssueID, "chat_session_id": row.ChatSessionID, "created_at": testutil.Raw("date_trunc('second',now())"), "status": "completed", "claim_generation": 1, "knowledge_context_version": 1, "knowledge_delivery": raw})
	}
}

func TestKnowledgeResumeReadsCompleteLongSessionHistory(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	first := f.recordSession(t, "long-native")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "long-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET created_at=created_at-interval '1 day' WHERE id=$1`, f.task.ID)
	f.insertClaimedHistory(t, 260)
	f.nextTurn(t)
	d := f.finalize(t, "long-native")
	if d.Session.Generation != first.Session.Generation || d.Session.NativeSessionID != "long-native" {
		t.Fatal("long history failed to preserve the original native binding")
	}
	if _, err := f.tasks.StartTask(f.ctx, f.task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d)); err != nil {
		t.Fatalf("prepare must read the full session history too: %v", err)
	}
}

func TestKnowledgeResumeDeepExposureStillGoverns(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	first := f.recordSession(t, "deep-native")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "deep-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET created_at=created_at-interval '1 day' WHERE id=$1`, f.task.ID)
	f.insertClaimedHistory(t, 260)
	f.withdraw(t, f.candidate.CandidateID)
	f.nextTurn(t)
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "deep-native")
	if err != nil || prior != "" || d == nil || d.Session.NativeSessionID != "" || d.Session.Generation == first.Session.Generation {
		t.Fatalf("withdrawn exposure beyond two pages must retire its native session: %q %v", prior, err)
	}
}

func TestKnowledgeResumeCorruptRowAnywhereRefuses(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	first := f.recordSession(t, "corrupt-deep-native")
	old := f.task.ID
	if _, err := f.tasks.CompleteTask(f.ctx, old, []byte(`{}`), "corrupt-deep-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET created_at=created_at-interval '1 day' WHERE id=$1`, old)
	f.insertClaimedHistory(t, 260)
	f.nextTurn(t)
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "corrupt-deep-native")
	if err != nil || prior != "corrupt-deep-native" || d == nil || d.Session.Generation != first.Session.Generation {
		t.Fatalf("valid deep history control must resume: %q %v", prior, err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_delivery='{}'::jsonb WHERE id=$1`, old)
	if _, prior, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "corrupt-deep-native"); !collaborationErrorCode(err, "knowledge_context_changed") || prior != "" {
		t.Fatalf("a damaged row beyond two pages was skipped: %q %v", prior, err)
	}
}

func TestKnowledgeResumeEqualTimestampHistoryStillChecksLaterIDs(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	first := f.recordSession(t, "equal-timestamp-native")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "equal-timestamp-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.insertClaimedHistory(t, 260)
	f.fx.Exec(t, `UPDATE agent_task_queue SET created_at='2026-11-01T00:00:00Z'::timestamptz
		WHERE agent_id=$1 AND knowledge_context_version IS NOT NULL`, f.task.AgentID)
	var rows, timestamps int
	f.fx.QueryRow(t, `SELECT count(*),count(DISTINCT created_at) FROM agent_task_queue
		WHERE agent_id=$1 AND knowledge_context_version IS NOT NULL`, f.task.AgentID).Scan(&rows, &timestamps)
	if rows != 261 || timestamps != 1 {
		t.Fatalf("equal-timestamp history control: rows=%d timestamps=%d, want 261 and 1", rows, timestamps)
	}
	var secondPageID pgtype.UUID
	f.fx.QueryRow(t, `SELECT id FROM agent_task_queue
		WHERE agent_id=$1 AND knowledge_context_version IS NOT NULL
		ORDER BY created_at DESC,id DESC OFFSET 128 LIMIT 1`, f.task.AgentID).Scan(&secondPageID)
	f.nextTurn(t)
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "equal-timestamp-native")
	if err != nil || prior != "equal-timestamp-native" || d == nil || d.Session.Generation != first.Session.Generation {
		t.Fatalf("valid tied history must preserve the original binding: %q %v", prior, err)
	}
	// Row 129 shares the first page's timestamp, so a timestamp-only cursor
	// would skip it together with every remaining row in this group.
	f.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_delivery='{}'::jsonb WHERE id=$1`, secondPageID)
	if _, prior, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "equal-timestamp-native"); !collaborationErrorCode(err, "knowledge_context_changed") || prior != "" {
		t.Fatalf("damaged metadata after 128 tied rows was skipped: %q %v", prior, err)
	}
}

func TestKnowledgeRuntimeHistoryReadsCompleteNamespace(t *testing.T) {
	for _, condition := range []string{"claimed_only_growth", "deep_exposure", "deep_corrupt"} {
		t.Run(condition, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if condition == "deep_exposure" {
				f.recordSession(t, "runtime-deep-native")
			} else {
				f.finalize(t, "")
			}
			old := f.task.ID
			f.fx.Exec(t, `UPDATE agent_task_queue SET created_at=created_at-interval '1 day' WHERE id=$1`, old)
			f.insertClaimedHistory(t, 260)
			agentID := f.fx.Agent(t, "Unrelated history consumer", util.UUIDToString(f.task.RuntimeID))
			issue := f.fx.Issue(t, "Unrelated scope")
			id := f.fx.Task(t, agentID, testutil.Cols{"runtime_id": f.task.RuntimeID, "issue_id": issue, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1})
			task, err := f.tasks.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
			if err != nil {
				t.Fatal(err)
			}
			d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, task, nil, "")
			if condition == "deep_exposure" {
				if !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
					t.Fatalf("deep private exposure must refuse unsupported work: %v", err)
				}
				return
			}
			if err != nil || d != nil || prior != "" {
				t.Fatalf("claimed-only namespace must remain ordinary: %q %v", prior, err)
			}
			if condition == "deep_corrupt" {
				f.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_delivery='{}'::jsonb WHERE id=$1`, old)
				if _, _, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, task, nil, ""); !collaborationErrorCode(err, "knowledge_context_changed") {
					t.Fatalf("damaged deep namespace history was skipped: %v", err)
				}
			}
		})
	}
}

func TestKnowledgeRuntimeHistoryIsIndexDriven(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	f.finalize(t, "")
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err := tx.Exec(f.ctx, `SET LOCAL enable_seqscan=off`); err != nil {
		t.Fatal(err)
	}
	probe := &knowledgeHistoryPlanProbe{DBTX: tx, t: t}
	queries := db.New(probe)
	runtime, err := queries.GetAgentRuntime(f.ctx, f.task.RuntimeID)
	if err != nil {
		t.Fatal(err)
	}
	tasks := NewTaskService(queries, f.pool, nil, events.New())
	exposed, err := tasks.taskKnowledgeRuntimeExposure(f.ctx, runtime)
	if err != nil || exposed {
		t.Fatalf("claimed-only history control: exposed=%v err=%v", exposed, err)
	}
	if probe.plans == 0 {
		t.Fatal("runtime history did not inspect a query plan")
	}
}

func TestKnowledgeRuntimeHistoryLostMetadataRefuses(t *testing.T) {
	for _, change := range []string{"deleted", "version_cleared"} {
		t.Run(change, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			f.finalize(t, "")
			tx, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			probe := &knowledgeHistoryLostMetadataProbe{DBTX: tx, change: change}
			tasks := NewTaskService(db.New(probe), f.pool, nil, events.New())
			runtime, err := tasks.Queries.GetAgentRuntime(f.ctx, f.task.RuntimeID)
			if err != nil {
				t.Fatal(err)
			}
			exposed, err := tasks.taskKnowledgeRuntimeExposure(f.ctx, runtime)
			if !collaborationErrorCode(err, "knowledge_context_changed") || exposed || !probe.changed {
				t.Fatalf("selected namespace metadata disappeared without refusal: changed=%v exposed=%v err=%v", probe.changed, exposed, err)
			}
		})
	}
}

type knowledgeHistoryLostMetadataProbe struct {
	db.DBTX
	change  string
	changed bool
}

func (p *knowledgeHistoryLostMetadataProbe) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if !p.changed && strings.Contains(sql, "-- name: GetTaskKnowledgeHistoryRows") {
		ids := args[0].([]pgtype.UUID)
		statement := "DELETE FROM agent_task_queue WHERE id=$1"
		if p.change == "version_cleared" {
			statement = "UPDATE agent_task_queue SET knowledge_context_version=NULL,knowledge_delivery=NULL WHERE id=$1"
		}
		if _, err := p.DBTX.Exec(ctx, statement, ids[0]); err != nil {
			return nil, err
		}
		p.changed = true
	}
	return p.DBTX.Query(ctx, sql, args...)
}

type knowledgeHistoryPlanProbe struct {
	db.DBTX
	t     *testing.T
	plans int
}

func (p *knowledgeHistoryPlanProbe) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "-- name: ListTaskKnowledgeRuntimeHistory") {
		var raw []byte
		if err := p.DBTX.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&raw); err != nil {
			p.t.Fatal(err)
		}
		var plan any
		if err := json.Unmarshal(raw, &plan); err != nil {
			p.t.Fatal(err)
		}
		partialHistoryIndex := false
		var check func(any)
		check = func(node any) {
			switch n := node.(type) {
			case map[string]any:
				if name, ok := n["Index Name"].(string); ok && strings.HasPrefix(name, "collaboration_knowledge_") && strings.HasSuffix(name, "_history_idx") {
					partialHistoryIndex = true
				}
				if n["Node Type"] == "Seq Scan" && n["Relation Name"] == "agent_task_queue" {
					p.t.Errorf("runtime namespace has no index-driven plan: %s", raw)
				}
				for _, v := range n {
					check(v)
				}
			case []any:
				for _, v := range n {
					check(v)
				}
			}
		}
		check(plan)
		if !partialHistoryIndex {
			p.t.Errorf("runtime namespace scans legacy task history instead of negotiated-row indexes: %s", raw)
		}
		compact, _ := json.Marshal(plan)
		p.t.Logf("runtime namespace plan: %s", compact)
		p.plans++
	}
	return p.DBTX.Query(ctx, sql, args...)
}
