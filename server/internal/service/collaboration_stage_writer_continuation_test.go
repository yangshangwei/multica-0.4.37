package service

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestStageWriterContinuationManualAndMachineCreateM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			f := newCollaborationStageFixture(t)
			stageExitServiceM1(t, f, mode)
			f.revise(t, true)
			agent, _ := f.agent(t)
			f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.workspace, "key": "writer_custom_done", "name": "Writer custom done",
				"category": "done", "is_system": false, "color": "#000000", "position": 100})
			for _, status := range []string{"todo", "done", "writer_custom_done"} {
				before := f.children(t)
				var counter int64
				f.fx.QueryRow(t, `SELECT issue_counter FROM workspace WHERE id=$1`, f.workspace).Scan(&counter)
				for _, creatorType := range []string{"agent", "member"} {
					creator := util.MustParseUUID(f.creator)
					if creatorType == "agent" {
						creator = agent
					}
					_, err := f.svc.Issues.Create(f.machine, IssueCreateParams{WorkspaceID: util.MustParseUUID(f.workspace), Title: "Unapproved machine " + creatorType + " " + status,
						Status: status, Priority: "none", CreatorType: creatorType, CreatorID: creator, ParentIssueID: f.issue.Issue.ID, AllowDuplicate: true},
						IssueCreateOpts{ActorUserID: util.MustParseUUID(f.creator)})
					if stageGateCode(err) != "stage_proposal_required" {
						t.Fatalf("stage-enabled %s machine %s create-as-%s escaped: %v", mode, creatorType, status, err)
					}
				}
				var afterCounter int64
				f.fx.QueryRow(t, `SELECT issue_counter FROM workspace WHERE id=$1`, f.workspace).Scan(&afterCounter)
				if f.children(t) != before || afterCounter != counter || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, agent) != 0 {
					t.Fatal("machine refusal left an allocated child, number or queued task")
				}
				created, err := f.svc.Issues.Create(f.ctx, IssueCreateParams{WorkspaceID: util.MustParseUUID(f.workspace), Title: "Manual independent " + status,
					Status: status, Priority: "none", CreatorType: "member", CreatorID: util.MustParseUUID(f.creator), ParentIssueID: f.issue.Issue.ID, AllowDuplicate: true},
					IssueCreateOpts{ActorUserID: util.MustParseUUID(f.creator)})
				if mode == "enforce" && status != "todo" {
					if stageGateCode(err) != "delivery_create_done_forbidden" || f.children(t) != before {
						t.Fatalf("stage changed the existing enforce create-as-done refusal: result=%+v err=%v", created, err)
					}
				} else if err != nil || created.Issue.Status != status || created.Issue.ProjectID != util.MustParseUUID(f.project) || f.children(t) != before+1 {
					t.Fatalf("stage imposed root verification on a human child or lost inherited project: result=%+v err=%v", created, err)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1`, f.issue.Issue.ID) != 0 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
					t.Fatal("human manual creation fabricated AI proposal approval or mapping")
				}
			}
		})
	}
}

func TestStageWriterContinuationApprovedMaterializationM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			f := newCollaborationStageFixture(t)
			stageExitServiceM1(t, f, mode)
			f.revise(t, true)
			f.configure(t)
			f.confirmScope(t)
			proposal := f.propose(t, f.ctx, "Explicitly approved machine child")
			p := f.decision(t, f.ctx, proposal, "accept", "Synthetic human product approval")
			receipt, err := f.svc.DecideStageProposal(f.ctx, p)
			if err != nil || receipt.Code != "stage_proposal_accepted" {
				t.Fatalf("M1 %s blocked the actual approved materialization: %+v %v", mode, receipt, err)
			}
			var child pgtype.UUID
			f.fx.QueryRow(t, `SELECT child_issue_id FROM collaboration_stage_child WHERE root_issue_id=$1 AND origin='materialized'`, f.issue.Issue.ID).Scan(&child)
			if f.children(t) != 1 || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, child) != 0 {
				t.Fatal("plain product acceptance created a duplicate child or automatically dispatched")
			}
			tasks := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
			if err := tasks.checkIssueExecution(f.ctx, tasks.Queries, child, pgtype.UUID{}, nil); err != nil {
				t.Fatalf("approved machine child lost its current ordinary execution admission: %v", err)
			}
			agent, _ := f.agent(t)
			_, err = f.svc.Issues.Create(f.machine, IssueCreateParams{WorkspaceID: util.MustParseUUID(f.workspace), Title: "Unapproved decomposition of approved child",
				Status: "todo", Priority: "none", CreatorType: "agent", CreatorID: agent, ParentIssueID: child, AllowDuplicate: true}, IssueCreateOpts{ActorUserID: util.MustParseUUID(f.creator)})
			if stageGateCode(err) != "stage_proposal_required" || f.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, child) != 0 {
				t.Fatalf("approved child was reused as an automatic decomposition permission: %v", err)
			}
		})
	}
}

func TestStageWriterContinuationSystemRootRefusalM1Modes(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			f := newCollaborationStageFixture(t)
			stageExitServiceM1(t, f, mode)
			f.revise(t, true)
			before := stageExitServiceSnapshot(t, f)
			for _, source := range []string{"pull_request_close_aggregate", "failed_task_reset"} {
				policyCalls := 0
				allow := func(context.Context, *db.Queries, db.Issue) (bool, error) { policyCalls++; return true, nil }
				_, after, changed, err := WriteIssueStatus(f.ctx, f.svc.TxStarter, f.svc.Queries, f.issue.Issue, "done", source, allow, nil, nil)
				if err != nil || changed || policyCalls != 0 || after.Status != f.issue.Issue.Status || stageExitServiceSnapshot(t, f) != before {
					t.Fatalf("%s completed a managed root under M1 %s: changed=%v policy_calls=%d status=%s err=%v", source, mode, changed, policyCalls, after.Status, err)
				}
			}
		})
	}
}

func TestStageWriterContinuationOrdinaryChildStatusKeepsCandidate(t *testing.T) {
	for _, mode := range []string{"off", "observe", "enforce"} {
		t.Run(mode, func(t *testing.T) {
			f := newStageReviewFixture(t)
			stageExitServiceM1(t, f.collaborationStageFixture, mode)
			candidate := f.code(t, nil)
			allow := func(context.Context, *db.Queries, db.Issue) (bool, error) { return true, nil }
			child, err := f.svc.Queries.GetIssue(f.ctx, util.MustParseUUID(f.child))
			if err != nil {
				t.Fatal(err)
			}
			_, working, changed, err := WriteIssueStatus(f.ctx, f.svc.TxStarter, f.svc.Queries, child, "in_progress", "ordinary_child_status", allow, nil, nil)
			if err != nil || !changed || working.Status != "in_progress" {
				t.Fatalf("stage imposed root verification on ordinary child status: %+v %v", working, err)
			}
			_, done, changed, err := WriteIssueStatus(f.ctx, f.svc.TxStarter, f.svc.Queries, working, "done", "pull_request_close_aggregate", allow, nil, nil)
			if mode == "enforce" {
				if stageGateCode(err) != "delivery_completion_denied" || changed {
					t.Fatalf("unsigned system completion weakened ordinary child M1: changed=%v err=%v", changed, err)
				}
			} else if err != nil || !changed || done.Status != "done" {
				t.Fatalf("M1 %s ordinary child completion acquired the root stage flow: changed=%v status=%s err=%v", mode, changed, done.Status, err)
			}
			view := f.evidence(t, f.ctx, candidate)
			var semantic int64
			f.fx.QueryRow(t, `SELECT semantic_version FROM collaboration_stage_child WHERE child_issue_id=$1`, f.child).Scan(&semantic)
			if semantic != 1 || view.Candidate.State != "current" || f.view(t).Stage.Current != "development" {
				t.Fatalf("ordinary child status invalidated code or advanced root: semantic=%d candidate=%s", semantic, view.Candidate.State)
			}
		})
	}
}
