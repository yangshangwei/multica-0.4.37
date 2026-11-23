package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func stageGateCode(err error) string {
	var delivery DeliveryCompletionError
	if errors.As(err, &delivery) {
		return delivery.Code
	}
	var domain *CollaborationError
	if errors.As(err, &domain) {
		return domain.Code
	}
	return ""
}

// Fixture inserts bypass the workspace counter; real creation must not collide.
func (f *collaborationStageFixture) syncCounter(t *testing.T) {
	t.Helper()
	f.fx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(max(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1`, f.workspace)
}

func (f *collaborationStageFixture) agent(t *testing.T) (pgtype.UUID, pgtype.UUID) {
	t.Helper()
	runtime := f.fx.Runtime(t, "Stage fake runtime")
	agent := f.fx.Agent(t, "Stage fake agent "+uuid.NewString(), runtime)
	return util.MustParseUUID(agent), util.MustParseUUID(runtime)
}

func (f *collaborationStageFixture) dispatched(t *testing.T, tasks *TaskService, agent, runtime pgtype.UUID, issue string) db.AgentTaskQueue {
	t.Helper()
	// One active task per issue: settle earlier fixture tasks first.
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='completed', completed_at=now() WHERE issue_id=$1 AND status IN ('queued','dispatched')`, issue)
	id := f.fx.Task(t, util.UUIDToString(agent), testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "dispatched"})
	task, err := tasks.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	return task
}

// CreateInTx is reached through the real IssueService.Create owner, exactly as
// POST /api/issues, quick-create completion and comment-to-child reach it.
func TestStageChildCreationRequiresApprovedProposalForMachines(t *testing.T) {
	f := newCollaborationStageFixture(t)
	ws, root := util.MustParseUUID(f.workspace), f.issue.Issue.ID
	agent, _ := f.agent(t)
	create := func(ctx context.Context, creatorType string, creator pgtype.UUID, title string, parent pgtype.UUID) error {
		_, err := f.svc.Issues.Create(ctx, IssueCreateParams{WorkspaceID: ws, Title: title, Status: "todo", Priority: "none", CreatorType: creatorType,
			CreatorID: creator, ParentIssueID: parent, AllowDuplicate: true}, IssueCreateOpts{ActorUserID: util.MustParseUUID(f.creator)})
		return err
	}
	human := util.MustParseUUID(f.creator)
	if err := create(f.machine, "agent", agent, "Machine child before enablement", root); err != nil {
		t.Fatalf("a project without a stage policy changed machine creation: %v", err)
	}
	f.revise(t, true)
	before := f.children(t)
	hinted := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialJWT, AgentActor: true})
	for name, tc := range map[string]struct {
		ctx     context.Context
		creator string
		id      pgtype.UUID
	}{
		"task_token_agent":                   {f.machine, "agent", agent},
		"agent_creator_no_auth":              {context.Background(), "agent", agent},
		"member_token_with_agent_headers":    {hinted, "member", human},
		"self_reported_member_on_task_token": {f.machine, "member", human},
	} {
		if err := create(tc.ctx, tc.creator, tc.id, "Machine split "+name, root); stageGateCode(err) != "stage_proposal_required" {
			t.Fatalf("%s created a child of a managed root without an approved proposal: %v", name, err)
		}
	}
	if f.children(t) != before || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1`, root) != 0 {
		t.Fatal("a refused machine creation left an issue or a mapping")
	}
	// Control group: a human keeps manual child creation, and it approves nothing.
	if err := create(f.ctx, "member", human, "Manual child", root); err != nil {
		t.Fatalf("human manual child creation changed: %v", err)
	}
	if err := create(context.Background(), "member", human, "Human-attributed internal child", root); err != nil {
		t.Fatalf("trusted internal human-attributed creation changed: %v", err)
	}
	if f.children(t) != before+2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1`, root) != 0 {
		t.Fatal("manual creation failed or produced a proposal mapping")
	}
	// The whole subtree of a managed root is governed, not only its direct
	// children: a machine cannot decompose below a child either.
	var manual, grandchild pgtype.UUID
	f.fx.QueryRow(t, `SELECT id FROM issue WHERE parent_issue_id=$1 AND title='Manual child'`, root).Scan(&manual)
	if err := create(f.ctx, "member", human, "Manual grandchild", manual); err != nil {
		t.Fatalf("a human lost manual creation below a child: %v", err)
	}
	f.fx.QueryRow(t, `SELECT id FROM issue WHERE parent_issue_id=$1 AND title='Manual grandchild'`, manual).Scan(&grandchild)
	issues := f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace)
	for name, parent := range map[string]pgtype.UUID{"child": manual, "grandchild": grandchild} {
		if err := create(f.machine, "agent", agent, "Machine sub-issue of a "+name, parent); stageGateCode(err) != "stage_proposal_required" {
			t.Fatalf("a machine decomposed below a %s of a managed root: %v", name, err)
		}
		if err := create(hinted, "member", human, "Hinted sub-issue of a "+name, parent); stageGateCode(err) != "stage_proposal_required" {
			t.Fatalf("an agent-actor hint decomposed below a %s of a managed root: %v", name, err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != issues {
		t.Fatal("a refused descendant creation left an issue")
	}
	// Machines keep ordinary creation outside every managed root.
	if err := create(f.machine, "agent", agent, "Machine top-level issue", pgtype.UUID{}); err != nil {
		t.Fatalf("unrelated machine creation changed: %v", err)
	}
	var unrelated pgtype.UUID
	f.fx.QueryRow(t, `SELECT id FROM issue WHERE workspace_id=$1 AND title='Machine top-level issue'`, f.workspace).Scan(&unrelated)
	if err := create(f.machine, "agent", agent, "Machine sub-issue of an unrelated issue", unrelated); err != nil {
		t.Fatalf("machine creation under an unrelated ordinary issue changed: %v", err)
	}
	// A parent chain that cannot be resolved within the bound (here a cycle) is
	// not provably outside a managed root: machines are refused, humans are not.
	loopA := f.fx.Issue(t, "Cycle A")
	loopB := f.fx.Issue(t, "Cycle B", testutil.Cols{"parent_issue_id": loopA})
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=$2 WHERE id=$1`, loopA, loopB)
	f.syncCounter(t)
	if err := create(f.machine, "agent", agent, "Machine issue in an unresolved chain", util.MustParseUUID(loopA)); stageGateCode(err) != "stage_ancestry_unresolved" {
		t.Fatalf("a machine created below an unresolvable parent chain: %v", err)
	}
	if err := create(f.ctx, "member", human, "Human issue in an unresolved chain", util.MustParseUUID(loopA)); err != nil {
		t.Fatalf("a human was gated by an unresolvable parent chain: %v", err)
	}
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, loopA)
	f.revise(t, false)
	if err := create(f.machine, "agent", agent, "Machine child after disable", root); err != nil {
		t.Fatalf("an explicitly disabled policy still gated creation: %v", err)
	}
	if err := create(f.machine, "agent", agent, "Machine grandchild after disable", manual); err != nil {
		t.Fatalf("an explicitly disabled policy still gated descendant creation: %v", err)
	}
}

// checkIssueExecution is the shared enqueue gate; startTaskAtomically is the
// actual queued-to-started boundary.
func TestStageChildExecutionGateAtEnqueueAndStart(t *testing.T) {
	f := newCollaborationStageFixture(t)
	root := f.issue.Issue.ID
	tasks := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	agent, runtime := f.agent(t)
	machineChild := f.fx.Issue(t, "Agent decomposition", testutil.Cols{"parent_issue_id": root, "project_id": f.project, "creator_type": "agent", "creator_id": agent})
	humanChild := f.fx.Issue(t, "Manual child", testutil.Cols{"parent_issue_id": root, "project_id": f.project})
	f.syncCounter(t)
	check := func(issue string) string {
		return stageGateCode(tasks.checkIssueExecution(f.ctx, tasks.Queries, util.MustParseUUID(issue), pgtype.UUID{}, nil))
	}
	start := func(issue string) (db.AgentTaskQueue, string, error) {
		task := f.dispatched(t, tasks, agent, runtime, issue)
		_, err := tasks.StartTask(f.ctx, task.ID)
		var status string
		f.fx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, task.ID).Scan(&status)
		return task, status, err
	}
	// No policy: ordinary behaviour for both.
	running, status, err := start(machineChild)
	if err != nil || status != "running" || check(humanChild) != "" {
		t.Fatalf("a project without a stage policy gated execution: %v %s", err, status)
	}
	f.revise(t, true)
	// Enabling is not a Stop: the task that already runs keeps running.
	f.fx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, running.ID).Scan(&status)
	if status != "running" {
		t.Fatalf("enabling the policy changed running work: %s", status)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='completed', completed_at=now() WHERE id=$1`, running.ID)
	if check(machineChild) != "stage_child_unapproved" || check(humanChild) != "" {
		t.Fatalf("enqueue gate: machine=%q human=%q", check(machineChild), check(humanChild))
	}
	if _, status, err = start(machineChild); stageGateCode(err) != "stage_child_unapproved" || status != "dispatched" {
		t.Fatalf("an unapproved machine child started: %v %s", err, status)
	}
	if _, status, err = start(humanChild); err != nil || status != "running" {
		t.Fatalf("a human child lost its ordinary execution contract: %v %s", err, status)
	}
	// The in-transaction enqueue entry point refuses before any agent work.
	gated, err := tasks.Queries.GetIssue(f.ctx, util.MustParseUUID(machineChild))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = tasks.EnqueuePreparedIssueTaskInTx(f.ctx, tx, gated, agent, pgtype.UUID{}, "", attribution.Result{}, PreparedIssueTaskEnqueue{})
	_ = tx.Rollback(f.ctx)
	if stageGateCode(err) != "stage_child_unapproved" {
		t.Fatalf("EnqueuePreparedIssueTaskInTx admitted an unapproved machine child: %v", err)
	}
	// Explicit confirmation by the product approver admits the existing child.
	f.configure(t)
	view := f.view(t)
	var scope []CollaborationStageScopeChild
	for _, item := range view.Children.Items {
		scope = append(scope, CollaborationStageScopeChild{IssueID: util.MustParseUUID(item.IssueID), ContentDigest: stageReadableDigest(t, item)})
	}
	f.confirmScope(t, scope...)
	if check(machineChild) != "" {
		t.Fatalf("a confirmed machine child is still gated: %q", check(machineChild))
	}
	if _, status, err = start(machineChild); err != nil || status != "running" {
		t.Fatalf("a confirmed machine child did not start: %v %s", err, status)
	}
	// Order A: the lifecycle changes first, then enqueue/start are refused.
	cancel := lifecycleParams(t, f.collaborationQueueFixture, "cancel", "cancelled", 0, 1)
	cancel.CollaborationWorkflowRequest = f.req(t, f.ctx)
	if _, err = f.svc.ChangeLifecycle(f.ctx, cancel); err != nil {
		t.Fatal(err)
	}
	reopen := lifecycleParams(t, f.collaborationQueueFixture, "reopen", "todo", 0, 1)
	reopen.CollaborationWorkflowRequest = f.req(t, f.ctx)
	if _, err = f.svc.ChangeLifecycle(f.ctx, reopen); err != nil {
		t.Fatal(err)
	}
	if check(machineChild) != "stage_scope_unconfirmed" {
		t.Fatalf("an earlier-lifecycle approval admitted enqueue: %q", check(machineChild))
	}
	if _, status, err = start(machineChild); stageGateCode(err) != "stage_scope_unconfirmed" || status != "dispatched" {
		t.Fatalf("an earlier-lifecycle approval admitted start: %v %s", err, status)
	}
	// The mapping is the child's persistent identity: removing or replacing the
	// parent link does not erase it.
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=NULL WHERE id=$1`, machineChild)
	if check(machineChild) != "stage_scope_unconfirmed" {
		t.Fatalf("detaching the parent escaped the gate: %q", check(machineChild))
	}
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=$2 WHERE id=$1`, machineChild, humanChild)
	if _, status, err = start(machineChild); stageGateCode(err) != "stage_scope_unconfirmed" || status != "dispatched" {
		t.Fatalf("re-parenting escaped the gate: %v %s", err, status)
	}
	f.fx.Exec(t, `UPDATE issue SET parent_issue_id=$2 WHERE id=$1`, machineChild, root)
	if _, status, err = start(humanChild); err != nil || status != "running" {
		t.Fatalf("a human child with a stale confirmation lost its ordinary contract: %v %s", err, status)
	}
	// Disabling the policy explicitly returns ordinary behaviour.
	f.revise(t, false)
	if check(machineChild) != "" {
		t.Fatalf("a disabled policy still gated execution: %q", check(machineChild))
	}
}

// The start transaction takes the root lifecycle fence before the child row,
// so "reopen then start" and "start then reopen" are the only two outcomes.
func TestStageChildStartSerializesWithRootLifecycle(t *testing.T) {
	f := newCollaborationStageFixture(t)
	root := f.issue.Issue.ID
	tasks := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	agent, runtime := f.agent(t)
	f.revise(t, true)
	f.configure(t)
	f.confirmScope(t)
	created := f.propose(t, f.handlerCtx, "Materialized one", "Materialized two")
	if _, err := f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, created, "accept", "")); err != nil {
		t.Fatal(err)
	}
	var first, second string
	f.fx.QueryRow(t, `SELECT child_issue_id FROM collaboration_stage_child WHERE root_issue_id=$1 AND ordinal=1`, root).Scan(&first)
	f.fx.QueryRow(t, `SELECT child_issue_id FROM collaboration_stage_child WHERE root_issue_id=$1 AND ordinal=2`, root).Scan(&second)
	// Order B: start commits first; a later lifecycle change leaves it running.
	started := f.dispatched(t, tasks, agent, runtime, first)
	if _, err := tasks.StartTask(f.ctx, started.ID); err != nil {
		t.Fatalf("a materialized child of the current lifecycle did not start: %v", err)
	}
	// Order A under contention: an in-flight root lifecycle writer holds the row;
	// the start waits for it and then observes the committed lifecycle.
	waiting := f.dispatched(t, tasks, agent, runtime, second)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, `SELECT 1 FROM issue_collaboration WHERE issue_id=$1 FOR UPDATE`, root); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(f.ctx, `UPDATE issue_collaboration SET lifecycle_epoch=lifecycle_epoch+1 WHERE issue_id=$1`, root); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := tasks.StartTask(f.ctx, waiting.ID)
		done <- err
	}()
	select {
	case err = <-done:
		t.Fatalf("start did not wait for the in-flight root lifecycle writer: %v", err)
	case <-time.After(400 * time.Millisecond):
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("start never finished after the lifecycle writer committed")
	}
	var status string
	f.fx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, waiting.ID).Scan(&status)
	if stageGateCode(err) != "stage_scope_unconfirmed" || status != "dispatched" {
		t.Fatalf("start was admitted against a lifecycle that changed first: %v %s", err, status)
	}
	f.fx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, started.ID).Scan(&status)
	if status != "running" {
		t.Fatalf("a lifecycle change stopped work that started first: %s", status)
	}
}

// CheckStageChildUpdateInTx is called by updateIssueAtomicallyOnce (Update, Move
// and Batch) inside the real update transaction, after the child row lock.
func TestStageChildUpdateKeepsMappingAndRefusesMachineAttach(t *testing.T) {
	f := newCollaborationStageFixture(t)
	root := f.issue.Issue.ID
	loose := f.fx.Issue(t, "Loose ordinary issue", testutil.Cols{"project_id": f.project})
	mapped := f.fx.Issue(t, "Existing child", testutil.Cols{"parent_issue_id": root, "project_id": f.project, "description": "reviewed"})
	f.syncCounter(t)
	f.revise(t, true)
	f.configure(t)
	view := f.view(t)
	f.confirmScope(t, CollaborationStageScopeChild{IssueID: util.MustParseUUID(mapped), ContentDigest: stageReadableDigest(t, view.Children.Items[0])})
	run := func(ctx context.Context, id string, agentActor bool, change func(*db.Issue)) error {
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		before, err := f.svc.Queries.WithTx(tx).LockIssueForDescriptionUpdate(f.ctx, db.LockIssueForDescriptionUpdateParams{ID: util.MustParseUUID(id), WorkspaceID: util.MustParseUUID(f.workspace)})
		if err != nil {
			t.Fatal(err)
		}
		candidate := before
		change(&candidate)
		if err = CheckStageChildUpdateInTx(ctx, tx, before, candidate, agentActor); err != nil {
			return err
		}
		return tx.Commit(f.ctx)
	}
	attach := func(issue *db.Issue) { issue.ParentIssueID = root }
	if err := run(f.machine, loose, false, attach); stageGateCode(err) != "stage_proposal_required" {
		t.Fatalf("a task token attached an issue to a managed root: %v", err)
	}
	if err := run(f.ctx, loose, true, attach); stageGateCode(err) != "stage_proposal_required" {
		t.Fatalf("an agent actor attached an issue to a managed root: %v", err)
	}
	below := func(issue *db.Issue) { issue.ParentIssueID = util.MustParseUUID(mapped) }
	if err := run(f.machine, loose, false, below); stageGateCode(err) != "stage_proposal_required" {
		t.Fatalf("a task token attached an issue below a child of a managed root: %v", err)
	}
	if err := run(f.ctx, loose, false, attach); err != nil {
		t.Fatalf("a human lost the right to attach a parent later: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1`, loose) != 0 {
		t.Fatal("attaching a parent created an approval mapping")
	}
	version := func() int64 {
		var v int64
		f.fx.QueryRow(t, `SELECT semantic_version FROM collaboration_stage_child WHERE child_issue_id=$1`, mapped).Scan(&v)
		return v
	}
	// A normal status change is not a content change.
	if err := run(f.ctx, mapped, false, func(issue *db.Issue) { issue.Status = "in_progress" }); err != nil || version() != 1 {
		t.Fatalf("a status change invalidated reviewed content: %v v=%d", err, version())
	}
	if err := run(f.machine, mapped, true, func(issue *db.Issue) { issue.Title = "Existing child, reworded" }); err != nil || version() != 2 {
		t.Fatalf("a content change was not recorded on the mapping: %v v=%d", err, version())
	}
	if err := run(f.ctx, mapped, false, func(issue *db.Issue) { issue.Description = pgtype.Text{String: "changed", Valid: true} }); err != nil || version() != 3 {
		t.Fatalf("a description change was not recorded: %v v=%d", err, version())
	}
	if err := run(f.ctx, mapped, false, func(issue *db.Issue) { issue.ParentIssueID = pgtype.UUID{} }); err != nil || version() != 4 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND root_issue_id=$2`, mapped, root) != 1 {
		t.Fatalf("removing the parent erased or ignored the mapping: %v v=%d", err, version())
	}
	if err := run(f.ctx, mapped, false, func(issue *db.Issue) { issue.ProjectID = pgtype.UUID{} }); err != nil || version() != 5 {
		t.Fatalf("a project change was not recorded: %v v=%d", err, version())
	}
}

// A machine-created descendant that already exists below a child (created
// before enablement, or attached by a human) is unapproved decomposition: it
// cannot be enqueued or started until the product approver confirms it.
func TestStageDescendantsOfManagedRootAreGatedAndConfirmable(t *testing.T) {
	f := newCollaborationStageFixture(t)
	root := f.issue.Issue.ID
	tasks := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	agent, runtime := f.agent(t)
	child := f.fx.Issue(t, "Manual child", testutil.Cols{"parent_issue_id": root, "project_id": f.project})
	machineGrandchild := f.fx.Issue(t, "Agent sub-task", testutil.Cols{"parent_issue_id": child, "creator_type": "agent", "creator_id": agent})
	machineDeep := f.fx.Issue(t, "Agent sub-sub-task", testutil.Cols{"parent_issue_id": machineGrandchild, "creator_type": "agent", "creator_id": agent})
	humanGrandchild := f.fx.Issue(t, "Manual sub-task", testutil.Cols{"parent_issue_id": child})
	outside := f.fx.Issue(t, "Unrelated agent issue", testutil.Cols{"creator_type": "agent", "creator_id": agent})
	outsideChild := f.fx.Issue(t, "Unrelated agent sub-task", testutil.Cols{"parent_issue_id": outside, "creator_type": "agent", "creator_id": agent})
	f.syncCounter(t)
	check := func(issue string) string {
		return stageGateCode(tasks.checkIssueExecution(f.ctx, tasks.Queries, util.MustParseUUID(issue), pgtype.UUID{}, nil))
	}
	start := func(issue string) (string, error) {
		task := f.dispatched(t, tasks, agent, runtime, issue)
		_, err := tasks.StartTask(f.ctx, task.ID)
		var status string
		f.fx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, task.ID).Scan(&status)
		return status, err
	}
	for _, issue := range []string{machineGrandchild, machineDeep, humanGrandchild, outsideChild} {
		if check(issue) != "" {
			t.Fatalf("a project without a stage policy gated %s", issue)
		}
	}
	f.revise(t, true)
	for name, issue := range map[string]string{"grandchild": machineGrandchild, "deeper": machineDeep} {
		if check(issue) != "stage_child_unapproved" {
			t.Fatalf("an unapproved machine %s of a managed root can be enqueued: %q", name, check(issue))
		}
		if status, err := start(issue); stageGateCode(err) != "stage_child_unapproved" || status != "dispatched" {
			t.Fatalf("an unapproved machine %s of a managed root started: %v %s", name, err, status)
		}
	}
	if check(humanGrandchild) != "" || check(outsideChild) != "" {
		t.Fatalf("a human descendant or an unrelated agent issue was gated: %q %q", check(humanGrandchild), check(outsideChild))
	}
	if status, err := start(outsideChild); err != nil || status != "running" {
		t.Fatalf("an unrelated agent sub-task lost its ordinary contract: %v %s", err, status)
	}
	f.configure(t)
	view := f.view(t)
	depth := map[string]int32{}
	var scope []CollaborationStageScopeChild
	for _, item := range view.Children.Items {
		if item.Depth != nil {
			depth[item.IssueID] = *item.Depth
		}
		if item.IssueID == machineGrandchild || item.IssueID == machineDeep {
			if !item.ExecutionGated || item.CreatorType != "agent" {
				t.Fatalf("an unapproved machine descendant is not shown as gated: %+v", item)
			}
			scope = append(scope, CollaborationStageScopeChild{IssueID: util.MustParseUUID(item.IssueID), ContentDigest: stageReadableDigest(t, item)})
		}
	}
	if len(view.Children.Items) != 4 || depth[child] != 1 || depth[machineGrandchild] != 2 || depth[machineDeep] != 3 || depth[humanGrandchild] != 2 {
		t.Fatalf("the stage view does not list the governed subtree with depths: %+v", view.Children.Items)
	}
	// An issue outside this root's subtree cannot be confirmed into it.
	foreign := CollaborationStageScopeChild{IssueID: util.MustParseUUID(outsideChild), ContentDigest: collaboration.StageChildContentDigest("Unrelated agent sub-task", "")}
	if _, err := f.svc.ConfirmStageScope(f.ctx, CollaborationStageScopeParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
		ExpectedRoleRevision: 1, ExpectedScopeRevision: 0, Children: []CollaborationStageScopeChild{foreign}}); !collaborationErrorCode(err, "stage_scope_child_unavailable") {
		t.Fatalf("an issue outside the root's subtree was confirmed: %v", err)
	}
	f.confirmScope(t, scope[0])
	confirmed, other := util.UUIDToString(scope[0].IssueID), machineDeep
	if confirmed == machineDeep {
		other = machineGrandchild
	}
	if check(confirmed) != "" || check(other) != "stage_child_unapproved" {
		t.Fatalf("confirmation did not admit exactly the listed descendant: %q %q", check(confirmed), check(other))
	}
	if status, err := start(confirmed); err != nil || status != "running" {
		t.Fatalf("a confirmed machine descendant did not start: %v %s", err, status)
	}
}
