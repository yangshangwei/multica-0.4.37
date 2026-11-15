package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Drive the real callers of the shared admission check, not only the helper.
func TestStageDescendantEnqueueEntryPoints(t *testing.T) {
	f := newCollaborationStageFixture(t)
	tasks := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	agent, _ := f.agent(t)
	parent := f.fx.Issue(t, "Manual parent", testutil.Cols{"parent_issue_id": f.issue.Issue.ID})
	id := f.fx.Issue(t, "Unapproved machine descendant", testutil.Cols{"parent_issue_id": parent, "creator_type": "agent", "creator_id": agent,
		"assignee_type": "agent", "assignee_id": agent})
	comment := util.MustParseUUID(f.fx.Comment(t, id, "Synthetic enqueue input"))
	squad := util.MustParseUUID(f.fx.Squad(t, "Synthetic stage squad", util.UUIDToString(agent)))
	f.revise(t, true)
	issue, err := tasks.Queries.GetIssue(f.ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	actor := util.MustParseUUID(f.creator)
	paths := map[string]func() (db.AgentTaskQueue, error){
		"assignment": func() (db.AgentTaskQueue, error) { return tasks.EnqueueTaskForIssue(f.ctx, issue, comment) },
		"handoff": func() (db.AgentTaskQueue, error) {
			return tasks.EnqueueTaskForIssueWithHandoff(f.ctx, issue, "Synthetic handoff", actor)
		},
		"mention": func() (db.AgentTaskQueue, error) { return tasks.EnqueueTaskForMention(f.ctx, issue, agent, comment) },
		"thread": func() (db.AgentTaskQueue, error) {
			return tasks.EnqueueTaskForThreadParent(f.ctx, issue, agent, comment)
		},
		"squad": func() (db.AgentTaskQueue, error) {
			return tasks.EnqueueTaskForSquadLeader(f.ctx, issue, agent, squad, comment)
		},
		"squad_handoff": func() (db.AgentTaskQueue, error) {
			return tasks.EnqueueTaskForSquadLeaderWithHandoff(f.ctx, issue, agent, squad, "Synthetic handoff", actor)
		},
		"deferred_fallback": func() (db.AgentTaskQueue, error) {
			return tasks.EnqueueDeferredAssigneeFallback(f.ctx, issue, agent, pgtype.UUID{}, pgtype.UUID{}, comment, time.Now())
		},
		"deferred_channel": func() (db.AgentTaskQueue, error) {
			return tasks.EnqueueDeferredChannelIssueTask(f.ctx, issue, time.Now())
		},
	}
	published := 0
	tasks.Bus.SubscribeAll(func(events.Event) { published++ })
	for name, enqueue := range paths {
		t.Run(name, func(t *testing.T) {
			task, err := enqueue()
			if stageGateCode(err) != "stage_child_unapproved" || task.ID.Valid {
				t.Fatalf("unapproved descendant enqueue: task=%v err=%v", task.ID, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, id) != 0 || published != 0 {
				t.Fatal("refused enqueue left a task or execution event")
			}
		})
	}
}

func TestStagePreparedEnqueueAdmitsExactlyConfirmedDescendant(t *testing.T) {
	f := newCollaborationStageFixture(t)
	tasks := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	agent, runtime := f.agent(t)
	parent := f.fx.Issue(t, "Manual parent", testutil.Cols{"parent_issue_id": f.issue.Issue.ID})
	id := f.fx.Issue(t, "Prepared machine descendant", testutil.Cols{"parent_issue_id": parent, "creator_type": "agent", "creator_id": agent,
		"assignee_type": "agent", "assignee_id": agent})
	human := f.fx.Issue(t, "Prepared human descendant", testutil.Cols{"parent_issue_id": parent, "assignee_type": "agent", "assignee_id": agent})
	actor := util.MustParseUUID(f.creator)
	attr := attribution.Result{UserID: actor, AccountableUserID: actor, Source: attribution.SourceDirectHuman}
	prepared, err := tasks.PrepareIssueTaskEnqueue(f.ctx, agent, attr)
	if err != nil {
		t.Fatal(err)
	}
	// This is the same borrowed-transaction boundary as lifecycle handoff: the
	// actual child row is held, and successful inserts become durable on commit.
	enqueue := func(id string) (db.AgentTaskQueue, bool, error) {
		ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
		defer cancel()
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		issue, err := tasks.Queries.WithTx(tx).LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: util.MustParseUUID(id), WorkspaceID: util.MustParseUUID(f.workspace)})
		if err != nil {
			t.Fatal(err)
		}
		task, reused, err := tasks.EnqueuePreparedIssueTaskInTx(ctx, tx, issue, agent, pgtype.UUID{}, "Synthetic prepared handoff", attr, prepared)
		if err == nil {
			err = tx.Commit(ctx)
		}
		return task, reused, err
	}
	f.revise(t, true)
	if task, reused, err := enqueue(id); stageGateCode(err) != "stage_child_unapproved" || task.ID.Valid || reused {
		t.Fatalf("unapproved prepared enqueue: %+v %v %v", task, reused, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, id) != 0 {
		t.Fatal("refused prepared enqueue left a task")
	}
	if task, reused, err := enqueue(human); err != nil || reused || task.Status != "queued" {
		t.Fatalf("human descendant lost ordinary prepared enqueue: %+v %v %v", task, reused, err)
	}
	f.configure(t)
	item := stageChildView(t, f.view(t), id)
	f.confirmScope(t, CollaborationStageScopeChild{IssueID: util.MustParseUUID(id), ContentDigest: stageReadableDigest(t, item)})
	// Root-to-child workflow writers must not deadlock with this child-to-root
	// caller. An in-flight lifecycle writer produces NOWAIT, never a queued task.
	rootTx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rootTx.Rollback(f.ctx)
	if _, err = rootTx.Exec(f.ctx, `SELECT 1 FROM issue_collaboration WHERE issue_id=$1 FOR UPDATE`, f.issue.Issue.ID); err != nil {
		t.Fatal(err)
	}
	_, _, err = enqueue(id)
	var busy *pgconn.PgError
	if !errors.As(err, &busy) || busy.Code != "55P03" || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, id) != 0 {
		t.Fatalf("prepared enqueue did not refuse a busy lifecycle fence: %v", err)
	}
	if err = rootTx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}
	task, reused, err := enqueue(id)
	if err != nil || reused || task.Status != "queued" || task.RuntimeID != runtime || task.AgentID != agent || task.OriginatorUserID != actor {
		t.Fatalf("confirmed prepared enqueue: %+v %v %v", task, reused, err)
	}
	again, reused, err := enqueue(id)
	if err != nil || !reused || again.ID != task.ID || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, id) != 1 {
		t.Fatalf("prepared enqueue retry did not reuse the committed task: %+v %v %v", again, reused, err)
	}
	// Existing queued work still needs the current lifecycle even on reuse.
	f.fx.Exec(t, `UPDATE issue_collaboration SET lifecycle_epoch=lifecycle_epoch+1 WHERE issue_id=$1`, f.issue.Issue.ID)
	if next, reused, err := enqueue(id); stageGateCode(err) != "stage_scope_unconfirmed" || next.ID.Valid || reused {
		t.Fatalf("stale confirmation reused queued work: %+v %v %v", next, reused, err)
	}
	if item := stageChildView(t, f.view(t), id); !item.ExecutionGated {
		t.Fatal("queued-before-reopen work is not visible as execution-gated")
	}
}

func TestStageApprovedChildCannotAuthorizeMachineDecomposition(t *testing.T) {
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	f.configure(t)
	f.confirmScope(t)
	proposal := f.propose(t, f.handlerCtx, "Approved child")
	if _, err := f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, proposal, "accept", "")); err != nil {
		t.Fatal(err)
	}
	var child pgtype.UUID
	f.fx.QueryRow(t, `SELECT child_issue_id FROM collaboration_stage_child WHERE root_issue_id=$1 AND proposal_id=$2`, f.issue.Issue.ID, proposal.ProposalID).Scan(&child)
	agent, _ := f.agent(t)
	params := IssueCreateParams{WorkspaceID: util.MustParseUUID(f.workspace), ParentIssueID: child, Title: "Machine decomposition of approved work",
		Status: "todo", Priority: "none", CreatorType: "agent", CreatorID: agent, AllowDuplicate: true}
	if _, err := f.svc.Issues.Create(f.machine, params, IssueCreateOpts{ActorUserID: util.MustParseUUID(f.creator)}); stageGateCode(err) != "stage_proposal_required" {
		t.Fatalf("an approved child authorized another machine decomposition: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, child) != 0 {
		t.Fatal("refused decomposition left a child")
	}
	params.CreatorType, params.CreatorID = "member", util.MustParseUUID(f.creator)
	if _, err := f.svc.Issues.Create(f.ctx, params, IssueCreateOpts{ActorUserID: params.CreatorID}); err != nil {
		t.Fatalf("an approved child lost manual human decomposition: %v", err)
	}
}
