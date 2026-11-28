package handler

// Scripted walks through the security-remediation squad (design 9.4).
//
// The "leader" here is test code, and so are the reviewers, the workers and
// the security owner. Each scenario is a fixed table of steps, and every step
// calls the real handler that role would call. So these scenarios prove one
// thing: when every role does what the instructions say, the platform hands
// control to whoever the instructions expect at each step. They do not prove
// that a model follows the instructions.
//
// The comment texts are shortened. The platform routes on a comment's author,
// its position (top-level or reply) and its mentions, so a text here is not a
// model answer for everything the instructions require a comment to contain.
//
// After each step the runner compares what the step changed with what its row
// says it must change: which agent gained a queued task, whether that is a
// leader task, whether it carries the squad, the trigger outcomes on the
// comment the step posted, the evaluation the leader recorded, and the issue
// status. A row that lists no task asserts that nobody gained one.
//
// The single routing rules these walks string together are pinned one at a
// time in squad_security_remediation_routing_test.go.
//
// Not walked here: the leader subscribing the owner before the first gate
// (TestSubscriberAPI in subscriber_test.go) and the notification a reviewer's
// result sends (cmd/server/squad_security_remediation_notification_test.go).

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// secRemSquadBodyHeading exists only in the squad instructions, never in an
// agent's own. Finding it in a claim proves the squad instructions were
// appended; checkHeadingPremise keeps that from passing by accident.
const secRemSquadBodyHeading = "# 安全漏洞修复小队工作分配规则"

// secRemWorkspaceAccess staffs the squad so that any workspace member may wake
// it. Without it only the member who staffed the squad can.
func secRemWorkspaceAccess() map[string]any {
	return map[string]any{
		"permission_mode":    "public_to",
		"invocation_targets": []map[string]any{{"target_type": "workspace"}},
	}
}

// flowTask is one task a step must have queued.
type flowTask struct {
	agent  string // who gained it
	leader bool   // it is a leader task
	squad  bool   // it carries the squad
}

// flowStep is one row of a scenario. act plays one role through real handlers
// and returns the comment it posted, or nil when the step posts none.
type flowStep struct {
	name string
	act  func() *CommentResponse

	queued   []flowTask              // tasks the step queued; none listed means none for anyone
	outcomes []CommentTriggerOutcome // target and status of each trigger outcome on the comment
	recorded string                  // outcome of the leader evaluation the step wrote; "" means none
	status   string                  // issue status after the step
}

// flowTaskRow is one agent_task_queue row of the flow's issue.
type flowTaskRow struct {
	ID     string `json:"id"`
	Agent  string `json:"agent"`
	Status string `json:"status"`
	Leader bool   `json:"leader"`
	Squad  string `json:"squad"`
}

// flowEvaluation is one squad_leader_evaluated activity row of the flow's issue.
type flowEvaluation struct {
	ActorType string `json:"actor_type"`
	Actor     string `json:"actor"`
	Squad     string `json:"squad"`
	Task      string `json:"task"`
	Outcome   string `json:"outcome"`
}

// secRemFlow is one scripted run of a staffed squad on one issue.
type secRemFlow struct {
	secRemSquad
	running   map[string]AgentTaskResponse // claimed and started runs, by agent
	seen      map[string]bool              // tasks that earlier steps accounted for
	evaluated int                          // evaluations that earlier steps accounted for
	comments  int                          // comments the script posted
}

func newSecRemFlow(s secRemSquad) *secRemFlow {
	return &secRemFlow{secRemSquad: s, running: map[string]AgentTaskResponse{}, seen: map[string]bool{}}
}

// who names an agent by its template key, for failure messages.
func (f *secRemFlow) who(agentID string) string {
	for key, id := range f.byRole {
		if id == agentID {
			return key
		}
	}
	return "agent " + agentID
}

// members lists every roster agent except the leader.
func (f *secRemFlow) members() []string {
	var ids []string
	for _, id := range f.byRole {
		if id != f.leader {
			ids = append(ids, id)
		}
	}
	return ids
}

// checkHeadingPremise fails when the heading the leader claims are checked for
// could come from somewhere other than the squad instructions.
func (f *secRemFlow) checkHeadingPremise(t *testing.T) {
	t.Helper()
	var leaderOwn, squadBody string
	dbfx.QueryRow(t, `SELECT COALESCE(instructions, '') FROM agent WHERE id = $1`, f.leader).Scan(&leaderOwn)
	dbfx.QueryRow(t, `SELECT COALESCE(instructions, '') FROM squad WHERE id = $1`, f.squad).Scan(&squadBody)
	if strings.Contains(leaderOwn, secRemSquadBodyHeading) || !strings.Contains(squadBody, secRemSquadBodyHeading) {
		t.Fatalf("setup: %q must be in the squad instructions only (leader's own: %v, squad: %v)", secRemSquadBodyHeading,
			strings.Contains(leaderOwn, secRemSquadBodyHeading), strings.Contains(squadBody, secRemSquadBodyHeading))
	}
}

// file creates the issue through the real handler, assigned to the squad, the
// way a member files a report.
func (f *secRemFlow) file(t *testing.T, reporter, title string) {
	t.Helper()
	syncIssueCounter(t)
	var created IssueResponse
	req := newRequestAs(reporter, "POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": title, "status": "todo", "assignee_type": "squad", "assignee_id": f.squad,
	})
	testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated).JSON(&created)
	f.issue = created.ID
	dbfx.Cleanup(t, `DELETE FROM issue WHERE id = $1`, f.issue)
	dbfx.Cleanup(t, `DELETE FROM activity_log WHERE issue_id = $1`, f.issue)
	f.cleanupIssueRows(t)
}

// syncIssueCounter brings the workspace issue counter up to date. Fixture
// issues do not advance it, so without this the handler could allocate a
// number a fixture already took.
func syncIssueCounter(t *testing.T) {
	t.Helper()
	dbfx.Exec(t, `UPDATE workspace SET issue_counter = GREATEST(issue_counter,
		(SELECT COALESCE(max(number), 0) FROM issue WHERE workspace_id = $1)) WHERE id = $1`, testWorkspaceID)
}

// createIssueAsLeader creates an issue from the leader's running task through
// the real handler and returns its id.
func (f *secRemFlow) createIssueAsLeader(t *testing.T, body map[string]any) string {
	t.Helper()
	syncIssueCounter(t)
	var created IssueResponse
	req := asAgent(newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, body), f.leader, f.task(t, f.leader).ID)
	testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated).JSON(&created)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE id = $1`, created.ID)
	dbfx.Cleanup(t, `DELETE FROM activity_log WHERE issue_id = $1`, created.ID)
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id = $1`, created.ID)
	return created.ID
}

// leaderRun is the task a valid wake-up of the squad queues: a leader task
// that carries the squad.
func (f *secRemFlow) leaderRun() flowTask {
	return flowTask{agent: f.leader, leader: true, squad: true}
}

// agentQueued is the trigger outcome of a comment that mentions an agent and
// queues a task for it.
func agentQueued(agentID string) CommentTriggerOutcome {
	return CommentTriggerOutcome{TargetType: "agent", TargetID: agentID, Status: DispatchQueued}
}

// squadQueued is the trigger outcome of a comment that mentions the squad and
// queues its leader.
func (f *secRemFlow) squadQueued() CommentTriggerOutcome {
	return CommentTriggerOutcome{TargetType: "squad", TargetID: f.squad, Status: DispatchQueued}
}

// mention is the roster mention of an agent, the form that dispatches it.
func (f *secRemFlow) mention(agentID string) string {
	return "[@" + f.who(agentID) + "](mention://agent/" + agentID + ")"
}

// gateComment is the gate comment the instructions ask the leader for: every
// reviewer of the gate and the owner mentioned in one comment, then the fixed
// release wording with the squad named in plain text, not as a mention.
func (f *secRemFlow) gateComment(gate string, reviewers ...string) string {
	content := gate + " 待确认。"
	for _, agentID := range reviewers {
		content += f.mention(agentID) + " "
	}
	return content + "请给出结果。负责人：[@安全负责人](mention://member/" + f.owner + ")。" +
		"请先读审查员的结果，再逐项给结论。放行方式一：在本任务评论并 @" + f.name + "（输入 @ 后从列表里选中，任意位置均可）。" +
		"放行方式二：发表不带任何 @ 的顶层评论。直接回复智能体的评论不算放行。" +
		"在队长明确回复本次放行已通过校验前，所有成员都不要编辑该评论、增删表情回应、标记解决、设为结论或变更附件；" +
		"发生这些改动后，请负责人读完本轮审查结果，再新发一条放行评论。" +
		"审查员迟迟没有结果时，@" + f.name + " 说明。" +
		"放行后本小队迟迟没有回应时，请确认自己可以运行本小队的智能体：是它们的所有者，或它们已向自己开放。"
}

// leaderRecords is the row that ends a run in which the leader acted: it
// records `action` with the reason and the run ends. The parent keeps the
// given status.
func (f *secRemFlow) leaderRecords(t *testing.T, reason, status string) flowStep {
	return flowStep{
		name: "the leader records its evaluation and its run ends: " + reason,
		act: func() *CommentResponse {
			f.evaluate(t, "action", reason)
			f.finish(t, f.leader)
			return nil
		},
		recorded: "action",
		status:   status,
	}
}

// claimShape reports, for the agent's running task, whether the claim marked
// it as a leader task and whether the squad instructions were appended to the
// agent's own.
func (f *secRemFlow) claimShape(t *testing.T, agentID string) (leaderTask, squadInstructions bool) {
	t.Helper()
	task := f.task(t, agentID)
	if task.Agent == nil {
		t.Fatalf("the claim for %s carries no agent block to read instructions from", f.who(agentID))
	}
	return task.IsLeaderTask, strings.Contains(task.Agent.Instructions, secRemSquadBodyHeading)
}

// ledger counts the tasks the whole walk left on the issue, keyed by
// taskLabel.
func (f *secRemFlow) ledger(t *testing.T) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, row := range f.taskRows(t) {
		counts[f.taskLabel(row.Agent, row.Leader, row.Squad == f.squad)]++
	}
	return counts
}

// wantOnlyScriptedComments fails when the issue holds a comment the script did
// not post, such as a completion fallback.
func (f *secRemFlow) wantOnlyScriptedComments(t *testing.T) {
	t.Helper()
	if count := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id = $1`, f.issue); count != f.comments {
		t.Fatalf("the issue has %d comments, want only the %d the script posted (no completion fallback, no system comment)", count, f.comments)
	}
}

// claim claims and starts one queued task per named agent on the squad
// runtime, as the daemon does, and fails when a claim went to anyone else.
func (f *secRemFlow) claim(t *testing.T, agents ...string) {
	t.Helper()
	claimed := f.claimTasks(t, len(agents))
	for _, agentID := range agents {
		task, ok := claimed[agentID]
		if !ok {
			var got []string
			for id := range claimed {
				got = append(got, f.who(id))
			}
			t.Fatalf("the claim returned no task for %s; it returned tasks for %v", f.who(agentID), got)
		}
		f.running[agentID] = task
	}
}

// task returns the agent's running task.
func (f *secRemFlow) task(t *testing.T, agentID string) AgentTaskResponse {
	t.Helper()
	task, ok := f.running[agentID]
	if !ok {
		t.Fatalf("script error: %s has no running task", f.who(agentID))
	}
	return task
}

// say posts a comment as the agent, from its running task and with that
// task's token, through the route the agent CLI uses. The instructions tell
// the leader to accept a result only from the member itself, read off the
// comment's author and source task, so the platform must record both.
func (f *secRemFlow) say(t *testing.T, agentID, parentID, content string) *CommentResponse {
	t.Helper()
	body := map[string]any{"content": content}
	if parentID != "" {
		body["parent_id"] = parentID
	}
	task := f.task(t, agentID)
	var comment CommentResponse
	postCommentWithTaskToken(t, task.AuthToken, f.issue, body).Want(http.StatusCreated).JSON(&comment)
	f.comments++
	if comment.AuthorType != "agent" || comment.AuthorID != agentID {
		t.Fatalf("comment posted by %s is recorded with author %s %s, want agent %s",
			f.who(agentID), comment.AuthorType, comment.AuthorID, agentID)
	}
	if comment.SourceTaskID == nil || *comment.SourceTaskID != task.ID {
		t.Fatalf("comment posted by %s is recorded with source task %v, want its running task %s",
			f.who(agentID), comment.SourceTaskID, task.ID)
	}
	return &comment
}

// ownerSays posts a comment as the human security owner. A release counts
// only when the comment's author is the owner's id from the roster link, so
// the platform must record that same id.
func (f *secRemFlow) ownerSays(t *testing.T, parentID, content string) *CommentResponse {
	t.Helper()
	body := map[string]any{"content": content}
	if parentID != "" {
		body["parent_id"] = parentID
	}
	var comment CommentResponse
	postCommentAsMember(t, f.issue, f.owner, body).Want(http.StatusCreated).JSON(&comment)
	f.comments++
	if comment.AuthorType != "member" || comment.AuthorID != f.owner {
		t.Fatalf("the owner's comment is recorded with author %s %s, want member %s",
			comment.AuthorType, comment.AuthorID, f.owner)
	}
	return &comment
}

// evaluate records the leader's evaluation of its running task.
func (f *secRemFlow) evaluate(t *testing.T, outcome, reason string) {
	t.Helper()
	req := asAgent(newRequest("POST", "/api/issues/"+f.issue+"/squad-evaluated", map[string]any{
		"outcome": outcome, "reason": reason,
	}), f.leader, f.task(t, f.leader).ID)
	testutil.Call(t, testHandler.RecordSquadLeaderEvaluation, withURLParam(req, "id", f.issue)).Want(http.StatusCreated)
}

// setStatus moves the parent issue as the leader, with its task token.
func (f *secRemFlow) setStatus(t *testing.T, status string) {
	t.Helper()
	capabilityCall(t, f.task(t, f.leader).AuthToken, "PUT", "/api/issues/"+f.issue, map[string]any{"status": status}).
		Want(http.StatusOK)
}

// finish ends the agent's run through the daemon completion route.
func (f *secRemFlow) finish(t *testing.T, agentID string) {
	t.Helper()
	taskID := f.task(t, agentID).ID
	req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/complete", map[string]any{"output": "本轮结束。"},
		testWorkspaceID, secRemDaemonID)
	testutil.Call(t, testHandler.CompleteTask, withURLParam(req, "taskId", taskID)).Want(http.StatusOK)
	delete(f.running, agentID)
}

// taskRows returns every task of the flow's issue, oldest first.
func (f *secRemFlow) taskRows(t *testing.T) []flowTaskRow {
	t.Helper()
	var raw string
	dbfx.QueryRow(t, `SELECT COALESCE(json_agg(json_build_object(
			'id', id, 'agent', agent_id, 'status', status,
			'leader', COALESCE(is_leader_task, false), 'squad', COALESCE(squad_id::text, '')
		) ORDER BY created_at, id), '[]')::text
		FROM agent_task_queue WHERE issue_id = $1`, f.issue).Scan(&raw)
	var rows []flowTaskRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode task rows: %v: %s", err, raw)
	}
	return rows
}

// evaluations returns every leader evaluation recorded on the flow's issue,
// oldest first.
func (f *secRemFlow) evaluations(t *testing.T) []flowEvaluation {
	t.Helper()
	var raw string
	dbfx.QueryRow(t, `SELECT COALESCE(json_agg(json_build_object(
			'actor_type', actor_type, 'actor', actor_id, 'squad', details->>'squad_id',
			'task', details->>'task_id', 'outcome', details->>'outcome'
		) ORDER BY created_at), '[]')::text
		FROM activity_log WHERE issue_id = $1 AND action = 'squad_leader_evaluated'`, f.issue).Scan(&raw)
	var rows []flowEvaluation
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode evaluation rows: %v: %s", err, raw)
	}
	return rows
}

// taskLabel renders one task for comparison and for failure messages.
func (f *secRemFlow) taskLabel(agentID string, leader, squad bool) string {
	label := f.who(agentID) + ": plain task"
	if leader {
		label = f.who(agentID) + ": leader task"
	}
	if squad {
		return label + " with the squad"
	}
	return label + " without a squad"
}

// outcomeLabel renders the target and status of one trigger outcome.
func (f *secRemFlow) outcomeLabel(outcome CommentTriggerOutcome) string {
	target := f.who(outcome.TargetID)
	if outcome.TargetType == "squad" && outcome.TargetID == f.squad {
		target = "this squad"
	}
	return fmt.Sprintf("%s %s: %v", outcome.TargetType, target, outcome.Status)
}

// run executes the table. After each step it compares what changed on the
// issue with what the row says must have changed, and stops the scenario at
// the first difference.
func (f *secRemFlow) run(t *testing.T, steps []flowStep) {
	t.Helper()
	for i, step := range steps {
		label := fmt.Sprintf("step %d (%s)", i+1, step.name)
		t.Log(label)
		comment := step.act()

		// Which agent gained a task, and what kind.
		all := f.taskRows(t)
		var fresh []flowTaskRow
		gotTasks, wantTasks := []string{}, []string{}
		for _, row := range all {
			if f.seen[row.ID] {
				continue
			}
			f.seen[row.ID] = true
			fresh = append(fresh, row)
			got := f.taskLabel(row.Agent, row.Leader, row.Squad == f.squad)
			if row.Status != "queued" {
				got += " [status " + row.Status + "]"
			}
			if row.Squad != "" && row.Squad != f.squad {
				got += " [another squad " + row.Squad + "]"
			}
			gotTasks = append(gotTasks, got)
		}
		for _, want := range step.queued {
			wantTasks = append(wantTasks, f.taskLabel(want.agent, want.leader, want.squad))
		}
		slices.Sort(gotTasks)
		slices.Sort(wantTasks)
		if !slices.Equal(gotTasks, wantTasks) {
			t.Fatalf("%s: tasks queued by the step = %q, want %q", label, gotTasks, wantTasks)
		}

		// What the comment reported about the tasks it triggered.
		gotOutcomes, wantOutcomes := []string{}, []string{}
		if comment != nil {
			for _, outcome := range comment.TriggerOutcomes {
				gotOutcomes = append(gotOutcomes, f.outcomeLabel(outcome))
				if outcome.Status != DispatchQueued {
					continue
				}
				index := slices.IndexFunc(fresh, func(row flowTaskRow) bool { return row.ID == outcome.TaskID })
				if index < 0 || fresh[index].Agent != outcome.ExecutingAgentID {
					t.Fatalf("%s: queued outcome %+v does not name a task this step queued for its executing agent", label, outcome)
				}
			}
		}
		for _, want := range step.outcomes {
			wantOutcomes = append(wantOutcomes, f.outcomeLabel(want))
		}
		slices.Sort(gotOutcomes)
		slices.Sort(wantOutcomes)
		if !slices.Equal(gotOutcomes, wantOutcomes) {
			t.Fatalf("%s: trigger outcomes = %q, want %q", label, gotOutcomes, wantOutcomes)
		}

		// The activity record the leader wrote.
		evaluations := f.evaluations(t)
		recorded := evaluations[f.evaluated:]
		f.evaluated = len(evaluations)
		if step.recorded == "" && len(recorded) != 0 {
			t.Fatalf("%s: the step recorded leader evaluations %+v, want none", label, recorded)
		}
		if step.recorded != "" {
			if len(recorded) != 1 {
				t.Fatalf("%s: the step recorded %d leader evaluations (%+v), want exactly one", label, len(recorded), recorded)
			}
			got := recorded[0]
			index := slices.IndexFunc(all, func(row flowTaskRow) bool { return row.ID == got.Task })
			if got.ActorType != "agent" || got.Actor != f.leader || got.Squad != f.squad || got.Outcome != step.recorded ||
				index < 0 || all[index].Agent != f.leader || !all[index].Leader {
				t.Fatalf("%s: leader evaluation = %+v, want outcome %q written by leader %s for squad %s from one of its leader tasks",
					label, got, step.recorded, f.leader, f.squad)
			}
		}

		var status string
		dbfx.QueryRow(t, `SELECT status FROM issue WHERE id = $1`, f.issue).Scan(&status)
		if status != step.status {
			t.Fatalf("%s: issue status = %q, want %q", label, status, step.status)
		}
	}
}

// A report that points at production stops at intake. The leader hands the
// case to the human owner and dispatches nobody: no reviewer and no worker
// ever gets a task, and the parent goes to in_review, never to done.
func TestSecurityRemediationFlow_ProductionEscalationDispatchesNobody(t *testing.T) {
	// The owner did not create the squad, so the squad is staffed with access
	// that covers them.
	f := newSecRemFlow(stageSecurityRemediationRoster(t, secRemWorkspaceAccess(), false))
	f.checkHeadingPremise(t)
	leader := f.leader

	f.run(t, []flowStep{
		{
			name: "a member files the report and assigns it to the squad",
			act: func() *CommentResponse {
				f.file(t, testUserID, "生产环境疑似正在被利用的越权漏洞")
				return nil
			},
			queued: []flowTask{f.leaderRun()},
			status: "todo",
		},
		{
			name: "the leader's first run starts with the squad instructions",
			act: func() *CommentResponse {
				f.claim(t, leader)
				if isLeaderTask, withSquadBody := f.claimShape(t, leader); !isLeaderTask || !withSquadBody {
					t.Fatalf("first leader claim: leader task = %v, squad instructions appended = %v; want both", isLeaderTask, withSquadBody)
				}
				return nil
			},
			status: "todo",
		},
		{
			name: "the leader hands the case to the human owner",
			act: func() *CommentResponse {
				return f.say(t, leader, "", "分诊结论：hold/unknown。报告指向生产环境里正在被利用的漏洞，命中停止条件，"+
					"本小队不分派任何成员，风险尚未关闭。[@安全负责人](mention://member/"+f.owner+") 请接手人工处置。")
			},
			status: "todo",
		},
		{
			name: "the leader records that it acted",
			act: func() *CommentResponse {
				f.evaluate(t, "action", "production escalation handed to the security owner")
				return nil
			},
			recorded: "action",
			status:   "todo",
		},
		{
			name: "the leader moves the parent to in_review after the hold/unknown handoff",
			act: func() *CommentResponse {
				f.setStatus(t, "in_review")
				return nil
			},
			status: "in_review",
		},
		{
			name: "the leader's run ends",
			act: func() *CommentResponse {
				f.finish(t, leader)
				return nil
			},
			status: "in_review",
		},
	})

	// The roster was created for this test, so a task for any member, on this
	// issue or anywhere else, would have come from this walk.
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id = ANY($1::uuid[])`, f.members()); count != 0 {
		t.Fatalf("squad members were given %d tasks, want none: the leader must dispatch nobody", count)
	}
	if got, want := f.ledger(t), map[string]int{f.taskLabel(leader, true, true): 1}; !maps.Equal(got, want) {
		t.Fatalf("tasks over the whole escalation walk = %v, want %v", got, want)
	}
	f.wantOnlyScriptedComments(t)
}

// This scripted walk tests real queue, token, comment and completion plumbing;
// it does not claim that a model follows the security workflow instructions.
func TestSecurityRemediationFlow_AllAgentsAdvanceWithoutHumanRelease(t *testing.T) {
	for _, route := range []string{"code", "dependency"} {
		t.Run(route, func(t *testing.T) {
			f := newSecRemFlow(stageSecurityRemediationRoster(t, nil, true))
			dbfx.Exec(t, `DELETE FROM squad_member WHERE squad_id=$1 AND member_type='member'`, f.squad)
			if n := dbfx.Count(t, `SELECT count(*) FROM squad_member WHERE squad_id=$1 AND member_type<>'agent'`, f.squad); n != 0 {
				t.Fatalf("roster has %d non-agent members", n)
			}
			// File through an actual agent request, inheriting its existing invocation
			// authority. The issue has no human creator and receives no human comments.
			bootstrapIssue := dbfx.Issue(t, "security report bootstrap", nil)
			bootstrapTask := dbfx.Task(t, f.leader, testutil.Cols{
				"runtime_id": f.runtime, "issue_id": bootstrapIssue, "status": "running", "started_at": testutil.Raw("now()"),
				"originator_user_id": testUserID, "accountable_user_id": testUserID,
			})
			f.running[f.leader] = AgentTaskResponse{ID: bootstrapTask}
			f.issue = f.createIssueAsLeader(t, map[string]any{"title": "Local security regression", "status": "todo", "assignee_type": "squad", "assignee_id": f.squad})
			f.cleanupIssueRows(t)
			dbfx.Cleanup(t, `DELETE FROM activity_log WHERE issue_id=$1`, f.issue)
			f.finishTasks(t, bootstrapTask)
			delete(f.running, f.leader)
			var creatorType string
			dbfx.QueryRow(t, `SELECT creator_type FROM issue WHERE id=$1`, f.issue).Scan(&creatorType)
			if creatorType != "agent" {
				t.Fatalf("creator=%s, want agent", creatorType)
			}
			f.claim(t, f.leader)
			f.setStatus(t, "in_progress")
			security, qa, implementer, code := f.agent(t, "security-reviewer"), f.agent(t, "qa-engineer"), f.agent(t, "implementer"), f.agent(t, "code-reviewer")
			phases := []struct {
				name   string
				agents []string
				result string
			}{
				{"G1 assessment", []string{security}, "Local synthetic fixture is sufficient; affected code confirmed; nonproduction repair permitted."},
				{"reproduction", []string{qa}, "Baseline abc: local security regression fails; fixture path security_test.go; exit 1."},
				{"repair", []string{implementer}, "Fix def: minimal authorization repair; regression preserved; exit 0."},
				{"independent verification", []string{qa}, "Fix def: original regression, denial and tenant cases pass; exit 0."},
				{"G2 independent review", []string{code, security}, "Fix def: evidence and independent tests reviewed, no unresolved findings."},
			}
			if route == "dependency" {
				migration := f.agent(t, "migration-reviewer")
				phases[0].agents = []string{security, migration}
				phases[0].result = "Resolved vulnerable dependency and reachable call confirmed; compatible fixed version and rollback verified."
				phases = append(phases[:1], phases[2:]...)
				phases[len(phases)-1].agents = []string{code, security, migration}
			}
			for _, phase := range phases {
				t.Run(phase.name, func(t *testing.T) {
					parent := ""
					if id := f.task(t, f.leader).TriggerCommentID; id != nil {
						parent = *id
					}
					content := phase.name + ": provide the current result. "
					for _, agent := range phase.agents {
						content += f.mention(agent) + " "
					}
					dispatch := f.say(t, f.leader, parent, content)
					f.evaluate(t, "action", phase.name)
					f.finish(t, f.leader)
					f.claim(t, phase.agents...)
					for _, agent := range phase.agents {
						result := f.say(t, agent, dispatch.ID, phase.result)
						if agent == security || agent == code {
							var eligible bool
							dbfx.QueryRow(t, `SELECT dispatch_eligible FROM comment WHERE id=$1`, result.ID).Scan(&eligible)
							if eligible {
								t.Fatal("restricted result became dispatch eligible")
							}
						}
						f.finish(t, agent)
					}
					if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued'`, f.issue); n != 1 {
						t.Fatalf("pending coordination tasks=%d, want one", n)
					}
					f.claim(t, f.leader)
					if leaderTask, withSquad := f.claimShape(t, f.leader); !leaderTask || !withSquad {
						t.Fatal("return lost leader/squad context")
					}
				})
			}
			f.setStatus(t, "in_review")
			f.evaluate(t, "no_action", "Independent results for fix def complete; delivered for acceptance")
			f.finish(t, f.leader)
			var status string
			dbfx.QueryRow(t, `SELECT status FROM issue WHERE id=$1`, f.issue).Scan(&status)
			if status != "in_review" {
				t.Fatalf("final status=%s", status)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND author_type='member'`, f.issue); n != 0 {
				t.Fatalf("flow used %d human comments", n)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status IN ('queued','dispatched','running')`, f.issue); n != 0 {
				t.Fatalf("flow left %d active tasks", n)
			}
		})
	}
}
