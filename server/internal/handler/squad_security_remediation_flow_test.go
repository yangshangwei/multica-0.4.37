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

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// secRemSquadBodyHeading exists only in the squad instructions, never in an
// agent's own. Finding it in a claim proves the squad instructions were
// appended; checkHeadingPremise keeps that from passing by accident.
const secRemSquadBodyHeading = "## 最终证据"

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

// The owner created the squad, which is staffed with the endpoint's default
// (private) access. On the "other" route the owner first answers by replying
// to the leader's gate comment without a mention, which is not a release: the
// leader is woken as a plain agent, without the squad instructions, and can
// only explain. The owner then releases the valid way with a decision to
// shelve, and the leader closes without dispatching anyone.
func TestSecurityRemediationFlow_OtherRouteInvalidReleaseThenShelve(t *testing.T) {
	f := newSecRemFlow(stageSecurityRemediationRoster(t, nil, true))
	f.checkHeadingPremise(t)
	leader, reviewer := f.leader, f.agent(t, "security-reviewer")
	var gate, invalid, shelve *CommentResponse

	f.run(t, []flowStep{
		{
			name: "the owner files the report and assigns it to the squad",
			act: func() *CommentResponse {
				f.file(t, f.owner, "构建产物签名校验可被绕过")
				return nil
			},
			queued: []flowTask{f.leaderRun()},
			status: "todo",
		},
		{
			name: "intake: the leader takes the parent in progress and opens G1 for the security reviewer and the owner",
			act: func() *CommentResponse {
				f.claim(t, leader)
				if isLeaderTask, withSquadBody := f.claimShape(t, leader); !isLeaderTask || !withSquadBody {
					t.Fatalf("first leader claim: leader task = %v, squad instructions appended = %v; want both", isLeaderTask, withSquadBody)
				}
				f.setStatus(t, "in_progress")
				gate = f.say(t, leader, "", f.gateComment("G1", reviewer))
				return gate
			},
			queued:   []flowTask{{agent: reviewer}},
			outcomes: []CommentTriggerOutcome{agentQueued(reviewer)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "waiting at G1 for the security owner", "in_progress"),
		{
			name: "the security reviewer answers under the gate comment",
			act: func() *CommentResponse {
				f.claim(t, reviewer)
				result := f.say(t, reviewer, gate.ID, "审查结果：来源可信，信任边界在构建流水线；证据缺口是签名密钥的轮换记录。")
				f.finish(t, reviewer)
				return result
			},
			status: "in_progress",
		},
		{
			name: "the owner replies to the leader's gate comment without a mention, which is not a release",
			act: func() *CommentResponse {
				invalid = f.ownerSays(t, gate.ID, "同意，先搁置吧。")
				return invalid
			},
			queued: []flowTask{{agent: leader}},
			status: "in_progress",
		},
		{
			name: "that run of the leader starts without the squad instructions",
			act: func() *CommentResponse {
				f.claim(t, leader)
				if isLeaderTask, withSquadBody := f.claimShape(t, leader); isLeaderTask || withSquadBody {
					t.Fatalf("claim after the invalid release: leader task = %v, squad instructions appended = %v; want a plain run of the leader with neither",
						isLeaderTask, withSquadBody)
				}
				return nil
			},
			status: "in_progress",
		},
		{
			name: "the leader only explains how to release, and its run ends",
			act: func() *CommentResponse {
				explanation := f.say(t, leader, invalid.ID, "直接回复智能体的评论不算放行。请在本任务评论并 @"+f.name+
					"，或发表不带任何 @ 的顶层评论，并逐项写明结论。")
				f.finish(t, leader)
				return explanation
			},
			status: "in_progress",
		},
		{
			name: "the owner shelves with a mention-free top-level comment",
			act: func() *CommentResponse {
				shelve = f.ownerSays(t, "", "已读审查员的结果。结论：搁置。理由：签名密钥的轮换记录两周后才能拿到。复查日期：2026-12-03。")
				return shelve
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader posts the closing summary and dispatches nobody",
			act: func() *CommentResponse {
				f.claim(t, leader)
				if isLeaderTask, withSquadBody := f.claimShape(t, leader); !isLeaderTask || !withSquadBody {
					t.Fatalf("leader claim after the valid release: leader task = %v, squad instructions appended = %v; want both", isLeaderTask, withSquadBody)
				}
				return f.say(t, leader, shelve.ID, "收口总结：hold。负责人决定搁置，理由和复查日期见上一条评论（复查日期 2026-12-03）。"+
					"风险尚未关闭，本小队没有派出任何成员修复。")
			},
			status: "in_progress",
		},
		{
			name: "the leader records the close, moves the parent to in_review, and its run ends",
			act: func() *CommentResponse {
				f.evaluate(t, "action", "closed on the owner's decision to shelve")
				f.setStatus(t, "in_review")
				f.finish(t, leader)
				return nil
			},
			recorded: "action",
			status:   "in_review",
		},
	})

	want := map[string]int{
		f.taskLabel(leader, true, true):     2, // intake and the valid release
		f.taskLabel(leader, false, false):   1, // the invalid release
		f.taskLabel(reviewer, false, false): 1, // G1
	}
	if got := f.ledger(t); !maps.Equal(got, want) {
		t.Fatalf("tasks over the whole shelve walk = %v, want %v", got, want)
	}
	f.wantOnlyScriptedComments(t)
}

// The dependency route end to end with an owner who did not create the squad,
// so the squad is staffed with workspace access and the owner files the issue
// too. Both gates are walked. Reviewer results wake nobody; each result of a
// flow-back member wakes the leader once; each release way is used once and
// wakes the leader once; the parent ends in in_review.
func TestSecurityRemediationFlow_DependencyRouteWalksBothGates(t *testing.T) {
	f := newSecRemFlow(stageSecurityRemediationRoster(t, secRemWorkspaceAccess(), false))
	f.checkHeadingPremise(t)
	leader := f.leader
	securityReviewer, migrationReviewer, codeReviewer := f.agent(t, "security-reviewer"), f.agent(t, "migration-reviewer"), f.agent(t, "code-reviewer")
	implementer, tester := f.agent(t, "implementer"), f.agent(t, "qa-engineer")
	var g1, release1, toImplementer, patch, toTester, verified, g2, release2 *CommentResponse

	f.run(t, []flowStep{
		{
			name: "the owner files the advisory and assigns it to the squad",
			act: func() *CommentResponse {
				f.file(t, f.owner, "依赖组件公告：反序列化漏洞影响当前锁定版本")
				return nil
			},
			queued: []flowTask{f.leaderRun()},
			status: "todo",
		},
		{
			name: "intake: the leader takes the parent in progress and opens G1 for both reviewers and the owner",
			act: func() *CommentResponse {
				f.claim(t, leader)
				if isLeaderTask, withSquadBody := f.claimShape(t, leader); !isLeaderTask || !withSquadBody {
					t.Fatalf("first leader claim: leader task = %v, squad instructions appended = %v; want both", isLeaderTask, withSquadBody)
				}
				f.setStatus(t, "in_progress")
				g1 = f.say(t, leader, "", f.gateComment("G1", securityReviewer, migrationReviewer))
				return g1
			},
			queued:   []flowTask{{agent: securityReviewer}, {agent: migrationReviewer}},
			outcomes: []CommentTriggerOutcome{agentQueued(securityReviewer), agentQueued(migrationReviewer)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "waiting at G1 for the security owner", "in_progress"),
		{
			name: "the security reviewer answers under the G1 comment",
			act: func() *CommentResponse {
				f.claim(t, securityReviewer, migrationReviewer)
				result := f.say(t, securityReviewer, g1.ID, "审查结果：公告适用。解析版本 2.4.1 由直接依赖引入，受影响函数在导入路径上可达；可信修复版本是 2.4.3。")
				f.finish(t, securityReviewer)
				return result
			},
			status: "in_progress",
		},
		{
			name: "the migration reviewer answers under the G1 comment",
			act: func() *CommentResponse {
				result := f.say(t, migrationReviewer, g1.ID, "审查结果：升级不改存储格式，没有兼容窗口限制，可以直接回滚。")
				f.finish(t, migrationReviewer)
				return result
			},
			status: "in_progress",
		},
		{
			name: "the owner releases G1 with a mention-free top-level comment",
			act: func() *CommentResponse {
				release1 = f.ownerSays(t, "", "已读两位审查员的结果。定级：高危；处置：修复；路由：dependency；期限：本周；复现：不适用；知情范围：本小队。")
				return release1
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader dispatches the implementer",
			act: func() *CommentResponse {
				f.claim(t, leader)
				toImplementer = f.say(t, leader, release1.ID, f.mention(implementer)+
					" 把该依赖升级到 2.4.3，只修这一项，保留锁文件和来源证据，补一条无修复时失败的回归测试。结果评论不要 @ 任何人或智能体。")
				return toImplementer
			},
			queued:   []flowTask{{agent: implementer}},
			outcomes: []CommentTriggerOutcome{agentQueued(implementer)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "dispatched the implementer", "in_progress"),
		{
			name: "the implementer reports without mentioning anyone, which wakes the leader",
			act: func() *CommentResponse {
				f.claim(t, implementer)
				patch = f.say(t, implementer, toImplementer.ID, "实现结果：修复提交 3f9c2ab，依赖升级到 2.4.3；回归测试在旧版本上失败，在新版本上通过。")
				f.finish(t, implementer)
				return patch
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader dispatches the test engineer",
			act: func() *CommentResponse {
				f.claim(t, leader)
				toTester = f.say(t, leader, patch.ID, f.mention(tester)+
					" 在修复提交 3f9c2ab 上独立验证原始风险和回归结果。结果评论不要 @ 任何人或智能体。")
				return toTester
			},
			queued:   []flowTask{{agent: tester}},
			outcomes: []CommentTriggerOutcome{agentQueued(tester)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "dispatched the test engineer", "in_progress"),
		{
			name: "the test engineer reports without mentioning anyone, which wakes the leader",
			act: func() *CommentResponse {
				f.claim(t, tester)
				verified = f.say(t, tester, toTester.ID, "验证结果：在提交 3f9c2ab 上原始风险不可复现，回归测试通过。")
				f.finish(t, tester)
				return verified
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader opens G2 for the code reviewer, the security reviewer and the owner",
			act: func() *CommentResponse {
				f.claim(t, leader)
				g2 = f.say(t, leader, verified.ID, f.gateComment("G2", codeReviewer, securityReviewer))
				return g2
			},
			queued:   []flowTask{{agent: codeReviewer}, {agent: securityReviewer}},
			outcomes: []CommentTriggerOutcome{agentQueued(codeReviewer), agentQueued(securityReviewer)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "waiting at G2 for the security owner", "in_progress"),
		{
			name: "the code reviewer answers under the G2 comment",
			act: func() *CommentResponse {
				f.claim(t, codeReviewer, securityReviewer)
				result := f.say(t, codeReviewer, g2.ID, "复审结果：提交 3f9c2ab 只改了依赖版本和回归测试，接受。")
				f.finish(t, codeReviewer)
				return result
			},
			status: "in_progress",
		},
		{
			name: "the security reviewer answers under the G2 comment",
			act: func() *CommentResponse {
				result := f.say(t, securityReviewer, g2.ID, "复审结果：提交 3f9c2ab 消除了可达路径，同一依赖没有其他使用点，接受。")
				f.finish(t, securityReviewer)
				return result
			},
			status: "in_progress",
		},
		{
			name: "the owner releases G2 with a reply that mentions the squad",
			act: func() *CommentResponse {
				release2 = f.ownerSays(t, g2.ID, "已读两位审查员的复审结果。复审结论：接受；变体清单：无；回移范围：无。[@"+f.name+"](mention://squad/"+f.squad+") 请收口。")
				return release2
			},
			queued:   []flowTask{f.leaderRun()},
			outcomes: []CommentTriggerOutcome{f.squadQueued()},
			status:   "in_progress",
		},
		{
			name: "the leader posts the closing summary and dispatches nobody",
			act: func() *CommentResponse {
				f.claim(t, leader)
				return f.say(t, leader, release2.ID, "收口总结：依赖已升级到 2.4.3（提交 3f9c2ab），独立验证和两项复审都接受，负责人已在 G2 放行。没有变体，没有回移。")
			},
			status: "in_progress",
		},
		{
			name: "the leader records the close, moves the parent to in_review, and its run ends",
			act: func() *CommentResponse {
				f.evaluate(t, "action", "closed after G2")
				f.setStatus(t, "in_review")
				f.finish(t, leader)
				return nil
			},
			recorded: "action",
			status:   "in_review",
		},
	})

	want := map[string]int{
		f.taskLabel(leader, true, true):              5, // intake, G1 release, implementer result, test engineer result, G2 release
		f.taskLabel(securityReviewer, false, false):  2, // G1 and G2
		f.taskLabel(migrationReviewer, false, false): 1,
		f.taskLabel(implementer, false, false):       1,
		f.taskLabel(tester, false, false):            1,
		f.taskLabel(codeReviewer, false, false):      1,
	}
	if got := f.ledger(t); !maps.Equal(got, want) {
		t.Fatalf("tasks over the whole dependency walk = %v, want %v", got, want)
	}
	f.wantOnlyScriptedComments(t)
}

// The code route end to end with a vulnerability verifier seated on the
// roster, an owner who created the squad, and the endpoint's default (private)
// access. The verifier reproduces before the repair and re-runs after it. Each
// result of a flow-back member (the verifier twice, the implementer, the test
// engineer) wakes the leader exactly once. After G2 the leader creates the one
// variant the owner confirmed, and nobody is queued for it.
func TestSecurityRemediationFlow_CodeRouteWithVerifierAndVariant(t *testing.T) {
	f := newSecRemFlow(stageSecurityRemediationRoster(t, nil, true))
	f.checkHeadingPremise(t)
	// One extra plain agent, seated the way a workspace adds its own verifier:
	// the instructions find it by its role text.
	verifier := dbfx.Agent(t, "Vulnerability verifier "+uuid.NewString()[:8], f.runtime)
	seat := newRequest("POST", "/api/squads/"+f.squad+"/members", map[string]any{
		"member_type": "agent", "member_id": verifier, "role": "漏洞验证",
	})
	testutil.Call(t, testHandler.AddSquadMember, withURLParams(seat, "workspaceId", testWorkspaceID, "id", f.squad)).
		Want(http.StatusCreated)
	dbfx.Cleanup(t, `DELETE FROM squad_member WHERE squad_id = $1 AND member_id = $2`, f.squad, verifier)
	f.byRole["vulnerability-verifier"] = verifier

	leader := f.leader
	securityReviewer, codeReviewer := f.agent(t, "security-reviewer"), f.agent(t, "code-reviewer")
	implementer, tester := f.agent(t, "implementer"), f.agent(t, "qa-engineer")
	var g1, release1, toReproduce, reproduced, toImplementer, patch, toRerun, rerun, toTester, verified, g2, release2 *CommentResponse
	var variant string

	f.run(t, []flowStep{
		{
			name: "the owner files the report and assigns it to the squad",
			act: func() *CommentResponse {
				f.file(t, f.owner, "订单导出接口缺少租户校验")
				return nil
			},
			queued: []flowTask{f.leaderRun()},
			status: "todo",
		},
		{
			name: "intake: the leader takes the parent in progress and opens G1 for the security reviewer and the owner",
			act: func() *CommentResponse {
				f.claim(t, leader)
				if isLeaderTask, withSquadBody := f.claimShape(t, leader); !isLeaderTask || !withSquadBody {
					t.Fatalf("first leader claim: leader task = %v, squad instructions appended = %v; want both", isLeaderTask, withSquadBody)
				}
				f.setStatus(t, "in_progress")
				g1 = f.say(t, leader, "", f.gateComment("G1", securityReviewer))
				return g1
			},
			queued:   []flowTask{{agent: securityReviewer}},
			outcomes: []CommentTriggerOutcome{agentQueued(securityReviewer)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "waiting at G1 for the security owner", "in_progress"),
		{
			name: "the security reviewer answers under the G1 comment",
			act: func() *CommentResponse {
				f.claim(t, securityReviewer)
				result := f.say(t, securityReviewer, g1.ID, "审查结果：导出接口按订单号取数，没有核对租户，跨租户读取真实可达。")
				f.finish(t, securityReviewer)
				return result
			},
			status: "in_progress",
		},
		{
			name: "the owner releases G1 with a reply that mentions the squad",
			act: func() *CommentResponse {
				release1 = f.ownerSays(t, g1.ID, "已读审查员的结果。定级：高危；处置：修复；路由：code；期限：三天；复现：由漏洞验证成员承担；知情范围：本小队。"+
					"[@"+f.name+"](mention://squad/"+f.squad+") 请继续。")
				return release1
			},
			queued:   []flowTask{f.leaderRun()},
			outcomes: []CommentTriggerOutcome{f.squadQueued()},
			status:   "in_progress",
		},
		{
			name: "the leader asks the verifier to reproduce",
			act: func() *CommentResponse {
				f.claim(t, leader)
				toReproduce = f.say(t, leader, release1.ID, f.mention(verifier)+
					" 按\"最小安全复现\"写一个在基线提交上失败的自动化测试：只用本地夹具和合成数据，证明可达且可触发即停。"+
					"交付测试路径、基线提交、失败输出和同类写法清单。结果评论不要 @ 任何人或智能体。")
				return toReproduce
			},
			queued:   []flowTask{{agent: verifier}},
			outcomes: []CommentTriggerOutcome{agentQueued(verifier)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "dispatched the verifier to reproduce", "in_progress"),
		{
			name: "the verifier reports the reproduction without mentioning anyone, which wakes the leader",
			act: func() *CommentResponse {
				f.claim(t, verifier)
				reproduced = f.say(t, verifier, toReproduce.ID, "复现结果：失败测试 internal/export/tenant_scope_test.go，基线提交 a1c9e04；go test 退出码 1，"+
					"本地夹具里合成租户 A 的会话读到了合成租户 B 的订单。同类写法清单：导出接口一处已确认可达，对账接口一处写法相同、待确认。")
				f.finish(t, verifier)
				return reproduced
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader dispatches the implementer",
			act: func() *CommentResponse {
				f.claim(t, leader)
				toImplementer = f.say(t, leader, reproduced.ID, f.mention(implementer)+
					" 给导出接口补上租户校验，只做这一项最小补丁和回归测试，结果写明修复提交。"+
					"不得修改复现测试，需要调整时退回复现者。结果评论不要 @ 任何人或智能体。")
				return toImplementer
			},
			queued:   []flowTask{{agent: implementer}},
			outcomes: []CommentTriggerOutcome{agentQueued(implementer)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "dispatched the implementer", "in_progress"),
		{
			name: "the implementer reports without mentioning anyone, which wakes the leader",
			act: func() *CommentResponse {
				f.claim(t, implementer)
				patch = f.say(t, implementer, toImplementer.ID, "实现结果：修复提交 7be41d0，导出接口按租户过滤，回归测试已补。")
				f.finish(t, implementer)
				return patch
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader asks the verifier to re-run on the fix commit",
			act: func() *CommentResponse {
				f.claim(t, leader)
				toRerun = f.say(t, leader, patch.ID, f.mention(verifier)+
					" 在修复提交 7be41d0 上重跑复现测试。结果评论不要 @ 任何人或智能体。")
				return toRerun
			},
			queued:   []flowTask{{agent: verifier}},
			outcomes: []CommentTriggerOutcome{agentQueued(verifier)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "dispatched the verifier to re-run", "in_progress"),
		{
			name: "the verifier reports the re-run without mentioning anyone, which wakes the leader",
			act: func() *CommentResponse {
				f.claim(t, verifier)
				rerun = f.say(t, verifier, toRerun.ID, "重跑结果：在提交 7be41d0 上复现测试不再通过越权路径，租户 B 的订单被拒绝。")
				f.finish(t, verifier)
				return rerun
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader dispatches the test engineer",
			act: func() *CommentResponse {
				f.claim(t, leader)
				toTester = f.say(t, leader, rerun.ID, f.mention(tester)+
					" 在修复提交 7be41d0 上验证原始风险、权限拒绝、租户边界、错误路径和资源上限。结果评论不要 @ 任何人或智能体。")
				return toTester
			},
			queued:   []flowTask{{agent: tester}},
			outcomes: []CommentTriggerOutcome{agentQueued(tester)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "dispatched the test engineer", "in_progress"),
		{
			name: "the test engineer reports without mentioning anyone, which wakes the leader",
			act: func() *CommentResponse {
				f.claim(t, tester)
				verified = f.say(t, tester, toTester.ID, "验证结果：在提交 7be41d0 上五项都通过，跨租户导出返回 403。")
				f.finish(t, tester)
				return verified
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader opens G2 for the code reviewer, the security reviewer and the owner",
			act: func() *CommentResponse {
				f.claim(t, leader)
				g2 = f.say(t, leader, verified.ID, f.gateComment("G2", codeReviewer, securityReviewer))
				return g2
			},
			queued:   []flowTask{{agent: codeReviewer}, {agent: securityReviewer}},
			outcomes: []CommentTriggerOutcome{agentQueued(codeReviewer), agentQueued(securityReviewer)},
			status:   "in_progress",
		},
		f.leaderRecords(t, "waiting at G2 for the security owner", "in_progress"),
		{
			name: "the code reviewer answers under the G2 comment",
			act: func() *CommentResponse {
				f.claim(t, codeReviewer, securityReviewer)
				result := f.say(t, codeReviewer, g2.ID, "复审结果：提交 7be41d0 的改动范围最小，测试覆盖拒绝路径，接受。")
				f.finish(t, codeReviewer)
				return result
			},
			status: "in_progress",
		},
		{
			name: "the security reviewer answers under the G2 comment",
			act: func() *CommentResponse {
				result := f.say(t, securityReviewer, g2.ID, "复审结果：提交 7be41d0 关闭了跨租户读取；对账接口是同一写法的变体，建议单独处理。接受。")
				f.finish(t, securityReviewer)
				return result
			},
			status: "in_progress",
		},
		{
			name: "the owner releases G2 with a mention-free top-level comment that confirms one variant",
			act: func() *CommentResponse {
				release2 = f.ownerSays(t, "", "已读两位审查员的复审结果。复审结论：接受；变体清单：确认对账接口一项；回移范围：无。")
				return release2
			},
			queued: []flowTask{f.leaderRun()},
			status: "in_progress",
		},
		{
			name: "the leader creates the confirmed variant as an unassigned backlog issue, which starts nobody",
			act: func() *CommentResponse {
				f.claim(t, leader)
				variant = f.createIssueAsLeader(t, map[string]any{
					"title": "SEC-2026-014-1", "description": "SEC-2026-014 的变体，细节见来源任务 " + f.issue, "status": "backlog",
				})
				var assigneeType, assigneeID, parent, status string
				dbfx.QueryRow(t, `SELECT COALESCE(assignee_type, ''), COALESCE(assignee_id::text, ''), COALESCE(parent_issue_id::text, ''), status
					FROM issue WHERE id = $1`, variant).Scan(&assigneeType, &assigneeID, &parent, &status)
				if assigneeType != "" || assigneeID != "" || parent != "" || status != "backlog" {
					t.Fatalf("variant issue = assignee %q/%q, parent %q, status %q; want unassigned, no parent, backlog", assigneeType, assigneeID, parent, status)
				}
				if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, variant); count != 0 {
					t.Fatalf("creating the variant issue queued %d tasks on it, want none", count)
				}
				return nil
			},
			status: "in_progress",
		},
		{
			name: "the leader posts the closing summary and dispatches nobody",
			act: func() *CommentResponse {
				return f.say(t, leader, release2.ID, "收口总结：导出接口已补租户校验（提交 7be41d0），独立复现、重跑、验证和两项复审都接受，负责人已在 G2 放行。"+
					"变体一项已另建任务 SEC-2026-014-1，没有并入本补丁。没有回移。")
			},
			status: "in_progress",
		},
		{
			name: "the leader records the close, moves the parent to in_review, and its run ends",
			act: func() *CommentResponse {
				f.evaluate(t, "action", "closed after G2")
				f.setStatus(t, "in_review")
				f.finish(t, leader)
				return nil
			},
			recorded: "action",
			status:   "in_review",
		},
	})

	want := map[string]int{
		f.taskLabel(leader, true, true):             7, // intake, G1 release, verifier twice, implementer, test engineer, G2 release
		f.taskLabel(securityReviewer, false, false): 2, // G1 and G2
		f.taskLabel(codeReviewer, false, false):     1,
		f.taskLabel(verifier, false, false):         2, // reproduce and re-run
		f.taskLabel(implementer, false, false):      1,
		f.taskLabel(tester, false, false):           1,
	}
	if got := f.ledger(t); !maps.Equal(got, want) {
		t.Fatalf("tasks over the whole code walk = %v, want %v", got, want)
	}
	// Still nobody at the end of the walk: an unassigned backlog issue is never
	// picked up on its own.
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, variant); count != 0 {
		t.Fatalf("the variant issue has %d tasks at the end of the walk, want none", count)
	}
	f.wantOnlyScriptedComments(t)
}
