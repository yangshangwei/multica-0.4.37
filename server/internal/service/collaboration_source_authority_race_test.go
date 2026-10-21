package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
)

func sourceAuthorityExecutionState(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	var state string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'session',(SELECT to_jsonb(s)-'event_seq' FROM collaboration_session s WHERE s.id=$1),
		'channels',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_channel c WHERE c.session_id=$1),
		'targets',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM collaboration_target t WHERE t.session_id=$1),
		'commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$1),
		'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_execution_group g WHERE g.session_id=$1),
		'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.session_id=$1),
		'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_grant g WHERE g.issue_id=$2),
		'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.group_id,i.command_id) FROM collaboration_group_input i
			WHERE i.group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1))
		)::text`, f.issue.SessionID, f.issue.Issue.ID).Scan(&state)
	return state
}

// Pause the real first writer's commit, then require PostgreSQL to show the
// second writer blocked by that transaction before releasing it.
func sourceAuthorityCommitOrder(t *testing.T, f *collaborationQueueFixture, ctx context.Context,
	leading, following func(TxStarter) error) (error, error) {
	t.Helper()
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
	firstDone := make(chan error, 1)
	go func() { firstDone <- leading(barrier) }()
	var blocker int
	select {
	case blocker = <-barrier.reached:
	case err := <-firstDone:
		t.Fatalf("first source writer did not reach its locked commit: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	secondDone := make(chan error, 1)
	go func() { secondDone <- following(f.pool) }()
	waitWorkflowBlocked(t, f, blocker, secondDone)
	unblock()
	return <-firstDone, <-secondDone
}

func TestCollaborationSourceStructureAndIterationTransferRecheckCurrentHandler(t *testing.T) {
	for _, domain := range []string{"structure", "iteration"} {
		for _, first := range []string{"source_change", "transfer"} {
			t.Run(domain+"/"+first+"_first", func(t *testing.T) {
				t.Setenv("MULTICA_AUTH_MODE", "legacy")
				f := newCollaborationQueueFixture(t)
				f.fx.Member(t, f.workspace, f.handler, "member")
				f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
				input := f.submit(t, f.creator, "frozen running input survives source authority change")
				f.run(t, input, 1)
				beforeAI := sourceAuthorityExecutionState(t, f)
				project := util.MustParseUUID(f.fx.Project(t, "Chosen source project"))
				parent := util.MustParseUUID(f.fx.Issue(t, "Chosen source parent"))
				period := sourceFieldIteration(t, f.collaborationFixture, "Chosen source iteration")
				ctx, cancel := context.WithTimeout(collaborationContext(f.handler), 15*time.Second)
				defer cancel()
				change := func(starter TxStarter) error {
					svc := *f.svc
					svc.TxStarter = starter
					if domain == "structure" {
						_, err := svc.EditIssue(ctx, CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1),
							ProjectID: &project, ParentIssueID: &parent, Title: editString("Committed source change")})
						return err
					}
					_, err := svc.ChangeIteration(ctx, CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, 1),
						CurrentIterationID: period, ExpectedIssueRevision: f.issue.Issue.Revision, ExpectedIterationRevision: 1})
					return err
				}
				transferRequest := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
					CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.creator)}}
				transfer := func(starter TxStarter) error {
					svc := *f.svc
					svc.TxStarter = starter
					_, err := svc.Transfer(ctx, transferRequest)
					return err
				}
				leading, following := change, transfer
				want := "stale_revision"
				if first == "transfer" {
					leading, following, want = transfer, change, "forbidden"
				}
				lead, follow := sourceAuthorityCommitOrder(t, f, ctx, leading, following)
				if lead != nil || !collaborationErrorCode(follow, want) {
					t.Fatalf("source/transfer ordering used stale authority: leading=%v following=%v", lead, follow)
				}
				var currentProject, currentParent, currentPeriod, handler pgtype.UUID
				var title string
				f.fx.QueryRow(t, `SELECT project_id,parent_issue_id,current_iteration_id,assignee_id,title FROM issue WHERE id=$1`, f.issue.Issue.ID).
					Scan(&currentProject, &currentParent, &currentPeriod, &handler, &title)
				if first == "transfer" {
					if currentProject.Valid || currentParent.Valid || currentPeriod.Valid || title != f.issue.Issue.Title || handler != util.MustParseUUID(f.creator) {
						t.Fatal("late source change partially applied after handler transfer")
					}
				} else {
					if handler != util.MustParseUUID(f.handler) || domain == "structure" && (currentProject != project || currentParent != parent || title != "Committed source change" || currentPeriod.Valid) ||
						domain == "iteration" && (currentPeriod != period || currentProject.Valid || currentParent.Valid || title != f.issue.Issue.Title) {
						t.Fatal("winning source change mixed domains or was overwritten by stale transfer")
					}
					transferRequest.CollaborationWorkflowRequest = workflowRequest(f, 2)
					if _, err := f.svc.Transfer(ctx, transferRequest); err != nil {
						t.Fatalf("fresh transfer after source commit: %v", err)
					}
				}
				if _, err := f.svc.FieldOptions(ctx, f.issue.Issue.ID); !collaborationErrorCode(err, "forbidden") {
					t.Fatalf("former noncreator handler retained a source catalog: %v", err)
				}
				if sourceAuthorityExecutionState(t, f) != beforeAI || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='transfer'`, f.issue.Issue.ID) != 1 {
					t.Fatal("source/handler change altered AI state or duplicated responsibility history")
				}
			})
		}
	}
}

func TestCollaborationSourceStructureAndIterationHoldMembershipUntilCommit(t *testing.T) {
	for _, domain := range []string{"structure", "iteration"} {
		for _, first := range []string{"source_change", "departure"} {
			t.Run(domain+"/"+first+"_first", func(t *testing.T) {
				t.Setenv("MULTICA_AUTH_MODE", "legacy")
				f := newCollaborationQueueFixture(t)
				f.fx.Member(t, f.workspace, f.handler, "member")
				input := f.submit(t, f.creator, "frozen running input survives source departure")
				f.run(t, input, 1)
				beforeAI := sourceAuthorityExecutionState(t, f)
				project := util.MustParseUUID(f.fx.Project(t, "Departure fenced project"))
				parent := util.MustParseUUID(f.fx.Issue(t, "Departure fenced parent"))
				period := sourceFieldIteration(t, f.collaborationFixture, "Departure fenced iteration")
				ctx, cancel := context.WithTimeout(collaborationContext(f.handler), 15*time.Second)
				defer cancel()
				request := workflowRequest(f, 1)
				change := func(starter TxStarter) error {
					svc := *f.svc
					svc.TxStarter = starter
					if domain == "structure" {
						_, err := svc.EditIssue(ctx, CollaborationEditParams{CollaborationWorkflowRequest: request,
							ProjectID: &project, ParentIssueID: &parent, Title: editString("Membership-fenced change")})
						return err
					}
					_, err := svc.ChangeIteration(ctx, CollaborationIterationParams{CollaborationWorkflowRequest: request,
						CurrentIterationID: period, ExpectedIssueRevision: f.issue.Issue.Revision, ExpectedIterationRevision: 1})
					return err
				}
				if first == "departure" {
					tx, err := f.pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(context.Background())
					if _, err = tx.Exec(ctx, `SELECT id FROM member WHERE workspace_id=$1 AND user_id=$2 FOR UPDATE`, f.workspace, f.handler); err != nil {
						t.Fatal(err)
					}
					done := make(chan error, 1)
					go func() { done <- change(f.pool) }()
					waitWorkflowBlocked(t, f, int(tx.Conn().PgConn().PID()), done)
					if _, err = tx.Exec(ctx, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler); err != nil {
						t.Fatal(err)
					}
					if err = tx.Commit(ctx); err != nil {
						t.Fatal(err)
					}
					if err = <-done; !collaborationErrorCode(err, "forbidden") {
						t.Fatalf("source write accepted pre-wait membership: %v", err)
					}
				} else {
					departure := func(starter TxStarter) error {
						tx, err := starter.Begin(ctx)
						if err != nil {
							return err
						}
						defer tx.Rollback(context.Background())
						if _, err = tx.Exec(ctx, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler); err != nil {
							return err
						}
						return tx.Commit(ctx)
					}
					lead, follow := sourceAuthorityCommitOrder(t, f, ctx, change, departure)
					if lead != nil || follow != nil {
						t.Fatalf("membership was not held through the winning source commit: %v %v", lead, follow)
					}
				}
				var currentProject, currentParent, currentPeriod pgtype.UUID
				var title string
				f.fx.QueryRow(t, `SELECT project_id,parent_issue_id,current_iteration_id,title FROM issue WHERE id=$1`, f.issue.Issue.ID).
					Scan(&currentProject, &currentParent, &currentPeriod, &title)
				if first == "departure" && (currentProject.Valid || currentParent.Valid || currentPeriod.Valid || title != f.issue.Issue.Title) ||
					first == "source_change" && domain == "structure" && (currentProject != project || currentParent != parent || title != "Membership-fenced change" || currentPeriod.Valid) ||
					first == "source_change" && domain == "iteration" && (currentPeriod != period || currentProject.Valid || currentParent.Valid || title != f.issue.Issue.Title) {
					t.Fatal("membership contention left partial or mixed source writes")
				}
				request.RequestID = uuid.NewString()
				before := controlStateSnapshot(t, f)
				if err := change(f.pool); !collaborationErrorCode(err, "forbidden") || controlStateSnapshot(t, f) != before {
					t.Fatalf("departed source handler regained writes or emitted partial history: %v", err)
				}
				options, err := f.svc.FieldOptions(ctx, f.issue.Issue.ID)
				if err != nil || options.SourceOptions != nil || f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 ||
					sourceAuthorityExecutionState(t, f) != beforeAI {
					t.Fatalf("external handler regained catalogs or changed AI: %+v %v", options, err)
				}
			})
		}
	}
}

func TestCollaborationIterationAssignmentSerializesWithTargetRevision(t *testing.T) {
	for _, first := range []string{"membership", "iteration_edit"} {
		t.Run(first+"_first", func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", "legacy")
			f := newCollaborationQueueFixture(t)
			period := sourceFieldIteration(t, f.collaborationFixture, "Revision-fenced period")
			input := f.submit(t, f.creator, "immutable input across iteration revision contention")
			f.run(t, input, 1)
			beforeAI := sourceAuthorityExecutionState(t, f)
			ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
			defer cancel()
			assignment := CollaborationIterationParams{CollaborationWorkflowRequest: workflowRequest(f, 1), CurrentIterationID: period,
				ExpectedIssueRevision: f.issue.Issue.Revision, ExpectedIterationRevision: 1}
			membership := func(starter TxStarter) error {
				svc := *f.svc
				svc.TxStarter = starter
				_, err := svc.ChangeIteration(ctx, assignment)
				return err
			}
			metadata := func(starter TxStarter) error {
				svc := &IterationService{TxStarter: starter}
				_, err := svc.Edit(ctx, util.MustParseUUID(f.workspace), util.MustParseUUID(f.creator), period,
					EditIterationInput{RequestID: uuid.NewString(), ExpectedRevision: 1, Fields: map[string]json.RawMessage{"name": json.RawMessage(`"Committed iteration rename"`)}}, lifecycleAuth)
				return err
			}
			leading, following := membership, metadata
			if first == "iteration_edit" {
				leading, following = metadata, membership
			}
			lead, follow := sourceAuthorityCommitOrder(t, f, ctx, leading, following)
			if lead != nil || first == "membership" && follow != nil || first == "iteration_edit" && !collaborationErrorCode(follow, "iteration_revision_conflict") {
				var domain *iteration.OperationError
				if errors.As(follow, &domain) {
					t.Fatalf("iteration operation refused after wrong commit ordering: %+v", domain)
				}
				t.Fatalf("iteration revision contention was not rechecked: %v %v", lead, follow)
			}
			if f.fx.Count(t, `SELECT count(*) FROM iteration WHERE id=$1 AND name='Committed iteration rename' AND revision=2`, period) != 1 {
				t.Fatal("iteration metadata did not commit its own exact revision")
			}
			if first == "iteration_edit" {
				if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id IS NULL AND revision=$2`, f.issue.Issue.ID, f.issue.Issue.Revision) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM iteration_participation WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='iteration_changed'`, f.issue.Issue.ID) != 0 {
					t.Fatal("stale target iteration partially changed membership or collaboration history")
				}
			} else {
				before := controlStateSnapshot(t, f)
				replayed, err := f.svc.ChangeIteration(ctx, assignment)
				if err != nil || replayed.Code != "iteration_changed" || before != controlStateSnapshot(t, f) ||
					f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2`, f.issue.Issue.ID, period) != 1 {
					t.Fatalf("later iteration metadata recomputed original membership: %+v %v", replayed, err)
				}
			}
			if sourceAuthorityExecutionState(t, f) != beforeAI {
				t.Fatal("iteration version contention changed the existing AI input or execution")
			}
		})
	}
}
