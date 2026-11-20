package handler

// Comment-routing behaviour of the platform that the security-remediation v2
// squad instructions rely on for their human gates. The "leader", the
// "reviewers" and the "security owner" are test code driving the real
// handlers, so these tests prove what the platform does with the comments the
// instructions ask for. They do not prove that a model writes those comments.
//
// Only the shapes no other test pins live here:
//
//   - a leader, posting from its running leader task, mentions several
//     capability-restricted reviewers and a human member in ONE comment;
//   - a capability-restricted reviewer posts a mention-free result on an issue
//     assigned to the squad;
//   - a human replies UNDER the leader's comment and mentions the squad;
//   - a capability-restricted reviewer's run fails: the notice the platform
//     leaves in the gate thread, and nobody being woken;
//   - somebody else rewrites the security owner's comment, or reacts to it:
//     what the thread read still says about its author and its revision.
//
// Canonical tests for the neighbouring behaviour, which this file does not
// repeat:
//
//   - member-authored comment mixing an agent mention and a member mention:
//     TestCommentTriggerThreadExplicitMentions in
//     server/cmd/server/comment_trigger_integration_test.go.
//   - restricted comment carrying an explicit mention is blocked, and the
//     completion fallback comment stays inert:
//     TestTaskCapabilityCommentsStayInertAfterTokenCleanup and
//     TestTaskCapabilityCompletionCommentStaysInert in task_capability_test.go.
//   - the full "worker comment wakes the leader" matrix:
//     squad_worker_comment_wakes_leader_test.go and
//     TestShouldEnqueueSquadLeaderOnComment_AgentAuthoredWorkerCommentsWakeLeader
//     in squad_comment_trigger_test.go.
//   - top-level squad mention, and a mention-free member reply to the leader
//     (a plain thread-parent task): TestCreateComment_SquadMentionTriggersLeader
//     and
//     TestCreateComment_SquadLeaderThreadParentTaskDoesNotSelfTriggerAssignedFallback
//     in squad_comment_trigger_test.go.
//
// Who may release a gate is decided by agent access, not by the squad roster,
// and that is pinned elsewhere too. A squad staffed without permission_mode
// gets private agents owned by the member who staffed it and no invocation
// targets (TestCreateSquadFromTemplate_SecurityRemediationPrivateRoster in
// squad_template_security_test.go). A private leader is not queued by another
// member's squad mention
// (TestCreateComment_SquadMentionPrivateLeaderBlocksPlainMember in
// squad_comment_trigger_test.go) or by their mention-free comment on an issue
// assigned to the squad (TestComment_SquadPrivateLeader_PlainMemberNoEnqueue in
// squad_private_leader_test.go). A security owner who did not staff the squad
// can therefore release a gate only when the squad was staffed with access
// that covers them, which is how the release test below stages it.

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

// secRemSquad is one staffed security-remediation squad with an issue assigned
// to it and a human security owner seated on its roster.
type secRemSquad struct {
	runtime string
	squad   string
	name    string
	leader  string
	issue   string
	// owner is the user id of the workspace member who sits on the roster with
	// the role the instructions look for. The routing tests seat a member who
	// is NOT the squad creator; a flow scenario may seat the creator instead.
	owner  string
	byRole map[string]string
}

// stageSecurityRemediationSquad stages the roster with a second workspace
// member as the security owner and inserts a fresh issue already assigned to
// the squad. Nothing is queued for that issue: each routing test seeds the one
// task it acts through.
func stageSecurityRemediationSquad(t *testing.T, access map[string]any) secRemSquad {
	t.Helper()
	s := stageSecurityRemediationRoster(t, access, false)
	s.issue = dbfx.Issue(t, "Security remediation routing", testutil.Cols{"assignee_type": "squad", "assignee_id": s.squad})
	s.cleanupIssueRows(t)
	return s
}

// stageSecurityRemediationRoster staffs the real template on a dedicated
// runtime (so a claim on that runtime can only return this squad's tasks) and
// seats the human security owner. It creates no issue. access is merged into
// the staffing request; nil keeps the endpoint's default. creatorOwns seats the
// member who staffed the squad; otherwise a second workspace member is seated.
func stageSecurityRemediationRoster(t *testing.T, access map[string]any, creatorOwns bool) secRemSquad {
	t.Helper()
	s := secRemSquad{byRole: map[string]string{}}
	s.runtime = dbfx.Runtime(t, "Security remediation routing runtime")
	s.owner = testUserID
	if !creatorOwns {
		// The owner is created before any issue so that the issue-scoped cleanups
		// run first and no task still names this user when it is deleted.
		s.owner = dbfx.User(t, "Security Owner", "secrem-owner-"+uuid.NewString()+"@example.test")
		dbfx.Member(t, testWorkspaceID, s.owner, "member")
	}

	extra := map[string]any{"runtime_id": s.runtime}
	for key, value := range access {
		extra[key] = value
	}
	staffed := staffSquad(t, "security-remediation", extra)
	// Deleting an agent row does not delete its invocation targets; without this
	// a squad staffed with access would leave them behind.
	dbfx.Cleanup(t, `DELETE FROM agent_invocation_target WHERE agent_id = ANY($1::uuid[])`, staffed.CreatedAgents)
	if len(staffed.ReusedAgents) != 0 {
		t.Fatalf("setup: staffing reused %d agents left behind by another test; the roster must be on the dedicated runtime", len(staffed.ReusedAgents))
	}
	s.squad, s.name, s.leader = staffed.Squad.ID, staffed.Squad.Name, staffed.Squad.LeaderID
	for _, agentID := range staffed.CreatedAgents {
		var key string
		dbfx.QueryRow(t, `SELECT template_key FROM agent WHERE id = $1`, agentID).Scan(&key)
		s.byRole[key] = agentID
	}
	// The squad creator seats the owner the way the product does.
	seat := newRequest("POST", "/api/squads/"+s.squad+"/members", map[string]any{
		"member_type": "member", "member_id": s.owner, "role": "安全负责人",
	})
	testutil.Call(t, testHandler.AddSquadMember, withURLParams(seat, "workspaceId", testWorkspaceID, "id", s.squad)).
		Want(http.StatusCreated)
	dbfx.Cleanup(t, `DELETE FROM squad_member WHERE squad_id = $1 AND member_type = 'member'`, s.squad)
	return s
}

// secRemDaemonID is the daemon identity every scripted claim, start and
// completion in the security-remediation tests uses.
const secRemDaemonID = "secrem-routing-test"

// cleanupIssueRows removes the rows the handlers create under s.issue. Cleanup
// is last-in first-out: tokens, then comments, then tasks, and after them
// whatever was registered before this call (the issue and the roster).
func (s secRemSquad) cleanupIssueRows(t *testing.T) {
	t.Helper()
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id = $1`, s.issue)
	dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id = $1`, s.issue)
	dbfx.Cleanup(t, `DELETE FROM task_token WHERE task_id IN (SELECT id FROM agent_task_queue WHERE issue_id = $1)`, s.issue)
}

// agent returns the roster agent staffed from the given role template.
func (s secRemSquad) agent(t *testing.T, templateKey string) string {
	t.Helper()
	agentID, ok := s.byRole[templateKey]
	if !ok {
		t.Fatalf("setup: the staffed roster has no %q agent: %v", templateKey, s.byRole)
	}
	return agentID
}

// startLeaderTask seeds the running leader task the leader acts through: the
// row an assignment or a release comment leaves once the daemon has started
// it. It has no trigger comment, so the leader may post top-level.
func (s secRemSquad) startLeaderTask(t *testing.T, originator string) string {
	t.Helper()
	return dbfx.Task(t, s.leader, testutil.Cols{
		"runtime_id":          s.runtime,
		"issue_id":            s.issue,
		"status":              "running",
		"started_at":          testutil.Raw("now()"),
		"is_leader_task":      true,
		"squad_id":            s.squad,
		"originator_user_id":  originator,
		"accountable_user_id": originator,
	})
}

// finishTasks ends tasks the way finished runs leave them, without running the
// completion pipeline: its fallback comment and reconciliation are pinned by
// their own tests and would add comments this file does not ask for.
func (s secRemSquad) finishTasks(t *testing.T, taskIDs ...string) {
	t.Helper()
	for _, taskID := range taskIDs {
		dbfx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, taskID)
	}
}

// gateContent is the gate comment the instructions ask the leader for: the
// agents that must answer and the security owner, mentioned in one comment.
func (s secRemSquad) gateContent(agents ...string) string {
	content := "G1 待确认。"
	for _, agentID := range agents {
		content += "[@成员](mention://agent/" + agentID + ") "
	}
	return content + "请给出结果。[@安全负责人](mention://member/" + s.owner + ") 请先读审查员的结果，再逐项给结论。"
}

// claimTasks claims n queued tasks on the squad runtime through the daemon
// claim handler, starts each through the daemon start handler and returns the
// claim responses keyed by agent id. AuthToken on a response is the task token
// that agent posts with.
func (s secRemSquad) claimTasks(t *testing.T, n int) map[string]AgentTaskResponse {
	t.Helper()
	const daemonID = secRemDaemonID
	claimed := map[string]AgentTaskResponse{}
	for i := range n {
		var out struct {
			Task *AgentTaskResponse `json:"task"`
		}
		claim := newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+s.runtime+"/tasks/claim", nil, testWorkspaceID, daemonID)
		testutil.Call(t, testHandler.ClaimTaskByRuntime, withURLParam(claim, "runtimeId", s.runtime)).Want(http.StatusOK).JSON(&out)
		if out.Task == nil {
			t.Fatalf("setup: claim %d of %d on the squad runtime returned no task", i+1, n)
		}
		if out.Task.AuthToken == "" {
			t.Fatalf("setup: claim for agent %s carries no task token", out.Task.AgentID)
		}
		start := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+out.Task.ID+"/start", nil, testWorkspaceID, daemonID)
		testutil.Call(t, testHandler.StartTask, withURLParam(start, "taskId", out.Task.ID)).Want(http.StatusOK)
		claimed[out.Task.AgentID] = *out.Task
	}
	return claimed
}

// postCommentWithTaskToken posts through the authenticated HTTP route with a
// claimed task token, so the auth middleware and the task-capability guard both
// run before the create-comment handler.
func postCommentWithTaskToken(t *testing.T, token, issueID string, body map[string]any) *testutil.Response {
	t.Helper()
	return capabilityCall(t, token, "POST", "/api/issues/"+issueID+"/comments", body)
}

// postCommentAsAgent posts through the create-comment handler with the agent
// and task identity the auth middleware derives from a verified task token.
func postCommentAsAgent(t *testing.T, issueID, agentID, taskID string, body map[string]any) *testutil.Response {
	t.Helper()
	req := asAgent(newRequest("POST", "/api/issues/"+issueID+"/comments", body), agentID, taskID)
	return testutil.Call(t, testHandler.CreateComment, withURLParam(req, "id", issueID))
}

// postCommentAsMember posts through the create-comment handler as a human.
func postCommentAsMember(t *testing.T, issueID, userID string, body map[string]any) *testutil.Response {
	t.Helper()
	req := newRequestAs(userID, "POST", "/api/issues/"+issueID+"/comments", body)
	return testutil.Call(t, testHandler.CreateComment, withURLParam(req, "id", issueID))
}

// A member mention in the leader's gate comment must not suppress the explicit
// reviewer mentions next to it: every reviewer is queued exactly once, and the
// leader's own comment does not queue the leader again.
func TestSecurityRemediationRouting_LeaderGateCommentQueuesEachRestrictedReviewer(t *testing.T) {
	s := stageSecurityRemediationSquad(t, nil)
	leaderTask := s.startLeaderTask(t, testUserID)
	reviewers := []string{s.agent(t, "security-reviewer"), s.agent(t, "migration-reviewer")}
	content := s.gateContent(reviewers...)
	if !hasMemberMention(util.ParseMentions(content)) {
		t.Fatalf("setup: the router does not read a member mention in the gate comment, so the test would prove nothing: %s", content)
	}

	var gate CommentResponse
	postCommentAsAgent(t, s.issue, s.leader, leaderTask, map[string]any{"content": content}).
		Want(http.StatusCreated).JSON(&gate)

	if len(gate.TriggerOutcomes) != len(reviewers) {
		t.Fatalf("gate comment outcomes = %+v, want one per mentioned reviewer and none for the member mention", gate.TriggerOutcomes)
	}
	for _, reviewer := range reviewers {
		index := slices.IndexFunc(gate.TriggerOutcomes, func(outcome CommentTriggerOutcome) bool {
			return outcome.TargetType == "agent" && outcome.TargetID == reviewer
		})
		if index < 0 {
			t.Fatalf("reviewer %s has no outcome in %+v", reviewer, gate.TriggerOutcomes)
		}
		outcome := gate.TriggerOutcomes[index]
		if outcome.Status != DispatchQueued || outcome.ReasonCode != ReasonQueued || outcome.TaskID == "" || outcome.ExecutingAgentID != reviewer {
			t.Errorf("reviewer %s outcome = %+v, want queued for that reviewer with a task id", reviewer, outcome)
		}
		if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`, s.issue, reviewer); count != 1 {
			t.Fatalf("reviewer %s has %d tasks on the issue, want exactly one", reviewer, count)
		}
		var status, trigger, squadID, originator string
		var isLeaderTask bool
		dbfx.QueryRow(t, `SELECT status, COALESCE(trigger_comment_id::text, ''), is_leader_task, COALESCE(squad_id::text, ''),
			COALESCE(originator_user_id::text, '') FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`, s.issue, reviewer).
			Scan(&status, &trigger, &isLeaderTask, &squadID, &originator)
		if status != "queued" || trigger != gate.ID || isLeaderTask || squadID != "" || originator != testUserID {
			t.Errorf("reviewer %s task = status %s, trigger %s, leader task %v, squad %q, originator %s; want a queued plain task triggered by gate comment %s for the leader task's originator",
				reviewer, status, trigger, isLeaderTask, squadID, originator, gate.ID)
		}
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`, s.issue, s.leader); count != 1 {
		t.Errorf("leader has %d tasks on the issue, want only the running one it posted from", count)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, s.issue); count != 1+len(reviewers) {
		t.Errorf("issue has %d tasks, want the leader task plus one per reviewer", count)
	}
}

// A restricted reviewer's mention-free result is stored but routes to nobody:
// the leader is not woken, so a gate stays closed until a human releases it.
// The control at the end posts the same shape from an unrestricted roster
// member through the same route and does wake the leader; the full matrix for
// that side lives in squad_worker_comment_wakes_leader_test.go.
func TestSecurityRemediationRouting_RestrictedReviewerResultDoesNotWakeLeader(t *testing.T) {
	s := stageSecurityRemediationSquad(t, nil)
	leaderTask := s.startLeaderTask(t, testUserID)
	reviewer, worker := s.agent(t, "security-reviewer"), s.agent(t, "implementer")

	var gate CommentResponse
	postCommentAsAgent(t, s.issue, s.leader, leaderTask, map[string]any{"content": s.gateContent(reviewer, worker)}).
		Want(http.StatusCreated).JSON(&gate)
	// The leader's run ends after the gate comment; from here on nothing of the
	// leader's is pending or running.
	s.finishTasks(t, leaderTask)

	claimed := s.claimTasks(t, 2)
	reviewerTask, ok := claimed[reviewer]
	if !ok {
		t.Fatalf("setup: the reviewer's task was not among the claimed tasks: %v", claimed)
	}
	workerTask, ok := claimed[worker]
	if !ok {
		t.Fatalf("setup: the worker's task was not among the claimed tasks: %v", claimed)
	}

	var reviewerCapability, workerCapability []byte
	dbfx.QueryRow(t, `SELECT api_capability FROM task_token WHERE task_id = $1`, reviewerTask.ID).Scan(&reviewerCapability)
	dbfx.QueryRow(t, `SELECT api_capability FROM task_token WHERE task_id = $1`, workerTask.ID).Scan(&workerCapability)
	capability, err := auth.ParseTaskCapability(reviewerCapability)
	if err != nil {
		t.Fatalf("reviewer token is not capability-restricted (api_capability = %q): %v", reviewerCapability, err)
	}
	if capability.Profile != auth.TaskCapabilityReviewer || capability.IssueID != s.issue || capability.OriginatorID != testUserID || capability.RuntimeID != s.runtime {
		t.Fatalf("reviewer capability = %+v, want an issue reviewer frozen to this issue, runtime and originator", capability)
	}
	if workerCapability != nil {
		t.Fatalf("control premise: the worker token is restricted too (api_capability = %q)", workerCapability)
	}

	tasksBefore := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, s.issue)

	// A task started by a comment may only answer inside that comment's thread,
	// and the rejection names the comment to reply under.
	rejected := postCommentWithTaskToken(t, reviewerTask.AuthToken, s.issue, map[string]any{"content": "审查结果：未发现问题。"}).
		Want(http.StatusConflict)
	if !strings.Contains(rejected.Text(), gate.ID) {
		t.Errorf("the top-level rejection does not point the reviewer at gate comment %s: %s", gate.ID, rejected.Text())
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id = $1 AND author_id = $2`, s.issue, reviewer); count != 0 {
		t.Fatalf("the rejected top-level result left %d reviewer comments behind", count)
	}

	var result CommentResponse
	postCommentWithTaskToken(t, reviewerTask.AuthToken, s.issue, map[string]any{"content": "审查结果：未发现问题。", "parent_id": gate.ID}).
		Want(http.StatusCreated).JSON(&result)
	if len(result.TriggerOutcomes) != 0 {
		t.Errorf("reviewer result outcomes = %+v, want none for a mention-free comment", result.TriggerOutcomes)
	}
	var authorType, authorID string
	var eligible bool
	dbfx.QueryRow(t, `SELECT author_type, author_id::text, dispatch_eligible FROM comment WHERE id = $1`, result.ID).
		Scan(&authorType, &authorID, &eligible)
	if authorType != "agent" || authorID != reviewer || eligible {
		t.Errorf("stored reviewer result = author %s/%s, dispatch_eligible %v; want the reviewer's own comment, not eligible for dispatch", authorType, authorID, eligible)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 AND id <> $3`, s.issue, s.leader, leaderTask); count != 0 {
		t.Fatalf("the reviewer's result queued %d leader tasks, want none", count)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, s.issue); count != tasksBefore {
		t.Fatalf("the reviewer's result changed the issue's task count from %d to %d, want no task for anyone", tasksBefore, count)
	}

	// Control: the same reply from an unrestricted member does wake the leader,
	// so the silence above comes from the restriction and not from the fixture.
	var report CommentResponse
	postCommentWithTaskToken(t, workerTask.AuthToken, s.issue, map[string]any{"content": "实现结果：补丁已提交。", "parent_id": gate.ID}).
		Want(http.StatusCreated).JSON(&report)
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 AND id <> $3
		AND status = 'queued' AND is_leader_task AND squad_id = $4 AND trigger_comment_id = $5`, s.issue, s.leader, leaderTask, s.squad, report.ID); count != 1 {
		t.Fatalf("control: the unrestricted member's result queued %d leader tasks for this squad, want exactly one", count)
	}
}

// The security owner releases a gate by replying under the leader's gate
// comment with the squad mentioned mid-sentence. The squad mention alone
// decides the routing: one leader task that carries the squad, and no plain
// thread-parent task for the comment's author next to it.
//
// The squad is staffed with workspace access, because the owner is not the
// member who created it (see the file header).
func TestSecurityRemediationRouting_OwnerReplyMentioningSquadQueuesOneLeaderTask(t *testing.T) {
	s := stageSecurityRemediationSquad(t, map[string]any{
		"permission_mode":    "public_to",
		"invocation_targets": []map[string]any{{"target_type": "workspace"}},
	})
	leaderTask := s.startLeaderTask(t, testUserID)
	reviewers := []string{s.agent(t, "security-reviewer"), s.agent(t, "migration-reviewer")}

	var gate CommentResponse
	postCommentAsAgent(t, s.issue, s.leader, leaderTask, map[string]any{"content": s.gateContent(reviewers...)}).
		Want(http.StatusCreated).JSON(&gate)
	if len(gate.TriggerOutcomes) != len(reviewers) {
		t.Fatalf("setup: gate comment outcomes = %+v, want one queued reviewer task each", gate.TriggerOutcomes)
	}
	// Every run the gate started is over before the owner answers, so nothing is
	// pending and any task the reply causes has to appear as a new row.
	finished := []string{leaderTask}
	for _, outcome := range gate.TriggerOutcomes {
		finished = append(finished, outcome.TaskID)
	}
	s.finishTasks(t, finished...)
	if pending := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND status <> 'completed'`, s.issue); pending != 0 {
		t.Fatalf("setup: %d tasks are still open on the issue before the owner replies", pending)
	}
	tasksBefore := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, s.issue)

	var release CommentResponse
	content := "已读完审查员的结果，G1 同意按高危处理。[@" + s.name + "](mention://squad/" + s.squad + ") 请继续修复。"
	postCommentAsMember(t, s.issue, s.owner, map[string]any{"content": content, "parent_id": gate.ID}).
		Want(http.StatusCreated).JSON(&release)

	if len(release.TriggerOutcomes) != 1 {
		t.Fatalf("release reply outcomes = %+v, want exactly one, for the squad", release.TriggerOutcomes)
	}
	outcome := release.TriggerOutcomes[0]
	if outcome.TargetType != "squad" || outcome.TargetID != s.squad || outcome.Status != DispatchQueued || outcome.ReasonCode != ReasonQueued ||
		outcome.ExecutingAgentID != s.leader || outcome.ExecutingSquadID != s.squad || outcome.TaskID == "" {
		t.Fatalf("release reply outcome = %+v, want the squad queued with its leader %s executing", outcome, s.leader)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, s.issue); count != tasksBefore+1 {
		t.Fatalf("the owner's reply changed the issue's task count from %d to %d, want exactly one new task", tasksBefore, count)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 AND NOT is_leader_task`, s.issue, s.leader); count != 0 {
		t.Fatalf("the leader got %d plain tasks: replying to its comment must not also queue it as the thread's parent author", count)
	}
	var agentID, status, squadID, trigger, originator string
	var isLeaderTask bool
	dbfx.QueryRow(t, `SELECT agent_id::text, status, is_leader_task, COALESCE(squad_id::text, ''), COALESCE(trigger_comment_id::text, ''),
		COALESCE(originator_user_id::text, '') FROM agent_task_queue WHERE id = $1`, outcome.TaskID).
		Scan(&agentID, &status, &isLeaderTask, &squadID, &trigger, &originator)
	if agentID != s.leader || status != "queued" || !isLeaderTask || squadID != s.squad {
		t.Fatalf("release task = agent %s, status %s, leader task %v, squad %q; want a queued leader task for leader %s carrying squad %s",
			agentID, status, isLeaderTask, squadID, s.leader, s.squad)
	}
	if trigger != release.ID || originator != s.owner {
		t.Errorf("release task trigger/originator = %s/%s, want the owner's reply %s and the owner %s", trigger, originator, release.ID, s.owner)
	}
}

// A reviewer whose run fails leaves no result, and nothing tells the leader.
// The platform posts the error under the reviewer's own name, in the gate
// comment's thread, with the failed run as its source. Read back through the
// thread read the instructions name, only `type` separates that notice from a
// result, which is why the result check in the instructions names the type.
// Nobody gains a task, so the gate stays closed until a person comes back.
//
// That a restricted agent's failure wakes no coordinator is owned by
// TestTaskCapabilityFailureCannotWakeCoordinator in
// service/task_capability_test.go. What lives here is the notice as the
// thread read returns it, on this squad's own roster.
//
// The control fails an unrestricted member's run the same way, and that does
// hand the failure back to the leader. The recovery path itself is pinned in
// service/delegated_failure_recovery_test.go.
func TestSecurityRemediationRouting_FailedReviewerLeavesANoticeAndWakesNobody(t *testing.T) {
	s := stageSecurityRemediationSquad(t, nil)
	leaderTask := s.startLeaderTask(t, testUserID)
	reviewer, worker := s.agent(t, "security-reviewer"), s.agent(t, "implementer")

	var gate CommentResponse
	postCommentAsAgent(t, s.issue, s.leader, leaderTask, map[string]any{"content": s.gateContent(reviewer, worker)}).
		Want(http.StatusCreated).JSON(&gate)
	s.finishTasks(t, leaderTask)

	claimed := s.claimTasks(t, 2)
	reviewerTask, ok := claimed[reviewer]
	if !ok {
		t.Fatalf("setup: the reviewer's task was not among the claimed tasks: %v", claimed)
	}
	workerTask, ok := claimed[worker]
	if !ok {
		t.Fatalf("setup: the worker's task was not among the claimed tasks: %v", claimed)
	}

	// agent_error is not retried, so the failure is final when the call returns.
	fail := func(taskID, message string) {
		t.Helper()
		req := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/fail",
			map[string]any{"error": message, "failure_reason": "agent_error"}, testWorkspaceID, secRemDaemonID)
		testutil.Call(t, testHandler.FailTask, withURLParam(req, "taskId", taskID)).Want(http.StatusOK)
	}
	// The read the instructions name for author checks:
	// multica issue comment list <issue> --thread <comment> --tail 30 --output json
	gateThread := func() []CommentResponse {
		t.Helper()
		var comments []CommentResponse
		req := newRequest("GET", "/api/issues/"+s.issue+"/comments?thread="+gate.ID+"&tail=30", nil)
		testutil.Call(t, testHandler.ListComments, withURLParam(req, "id", s.issue)).Want(http.StatusOK).JSON(&comments)
		return comments
	}
	leaderTasks := func() int {
		t.Helper()
		return dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2 AND id <> $3`, s.issue, s.leader, leaderTask)
	}

	tasksBefore := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, s.issue)
	const reviewerError = "This role requires an issue task with an authorizing human."
	fail(reviewerTask.ID, reviewerError)

	var notices []CommentResponse
	for _, comment := range gateThread() {
		if comment.AuthorID == reviewer {
			notices = append(notices, comment)
		}
	}
	if len(notices) != 1 {
		t.Fatalf("the gate thread holds %d comments under the reviewer's name after its run failed, want the platform's notice only: %+v", len(notices), notices)
	}
	notice := notices[0]
	if notice.AuthorType != "agent" || notice.ParentID == nil || *notice.ParentID != gate.ID ||
		notice.SourceTaskID == nil || *notice.SourceTaskID != reviewerTask.ID {
		t.Fatalf("failure notice = %+v; want it under the reviewer's own name, replying to gate comment %s, with the failed run %s as its source",
			notice, gate.ID, reviewerTask.ID)
	}
	if notice.Type != "system" || notice.Content != reviewerError {
		t.Fatalf("failure notice type/content = %q/%q, want type system carrying the run's error: the type is all that tells it from a result",
			notice.Type, notice.Content)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, s.issue); count != tasksBefore {
		t.Fatalf("the reviewer's failure changed the issue's task count from %d to %d, want no task for anyone", tasksBefore, count)
	}
	if count := leaderTasks(); count != 0 {
		t.Fatalf("the reviewer's failure queued %d leader tasks, want none", count)
	}

	// Control: the same failure from an unrestricted member is handed back to
	// the leader, so the silence above comes from the restriction and not from
	// the fixture.
	fail(workerTask.ID, "agent crashed")
	if count := leaderTasks(); count != 1 {
		t.Fatalf("control: the unrestricted member's failure queued %d leader tasks, want exactly one", count)
	}
}

// A workspace admin may rewrite another member's comment, and the rewritten
// comment still names its first author. Read back through the thread read the
// instructions name, `revision` is the only field that always shows the change
// (`updated_at` moves on an edit too, but the read prints it to the second, so
// an edit made in the second the comment was created leaves it equal to
// `created_at`), which is why the release check refuses a comment whose
// revision is above 1.
//
// The same field moves when somebody merely reacts, so that refusal also meets
// comments nobody rewrote. The instructions name the reaction among the causes
// and the docs tell the owner to leave a release alone until the squad answers;
// the second half pins the fact both sentences rest on.
//
// That each mutation advances the revision exactly once is owned by
// issue_revision_test.go: the content edit by
// TestRevisionConflictsPreserveLatestIssueAndComment, the reaction (with the
// resolve and attachment changes, which this test does not read) by
// TestAuxiliaryVisibleMutationsAdvanceOwnerRevisionExactlyOnce. What lives here
// is the comment as the leader's read returns it.
func TestSecurityRemediationRouting_ChangedReleaseCommentKeepsItsAuthor(t *testing.T) {
	s := stageSecurityRemediationSquad(t, nil)

	// multica issue comment list <issue> --thread <comment> --tail 30 --output json
	read := func(commentID string) CommentResponse {
		t.Helper()
		var comments []CommentResponse
		req := newRequest("GET", "/api/issues/"+s.issue+"/comments?thread="+commentID+"&tail=30", nil)
		testutil.Call(t, testHandler.ListComments, withURLParam(req, "id", s.issue)).Want(http.StatusOK).JSON(&comments)
		for _, comment := range comments {
			if comment.ID == commentID {
				return comment
			}
		}
		t.Fatalf("the thread read does not return comment %s: %+v", commentID, comments)
		return CommentResponse{}
	}
	post := func(content string) CommentResponse {
		t.Helper()
		var created CommentResponse
		postCommentAsMember(t, s.issue, s.owner, map[string]any{"content": content}).Want(http.StatusCreated).JSON(&created)
		if got := read(created.ID); got.Revision != 1 {
			t.Fatalf("setup: a comment nobody touched reads back with revision %d, want 1", got.Revision)
		}
		return created
	}

	// The owner returns the fix; the member who staffed the squad, a workspace
	// owner, turns that into an acceptance.
	const rewritten = "G2：接受。"
	release := post("G2：退回，缺少租户边界的回归测试。")
	edit := newRequest("PUT", "/api/comments/"+release.ID, map[string]any{"content": rewritten})
	testutil.Call(t, testHandler.UpdateComment, withURLParam(edit, "commentId", release.ID)).Want(http.StatusOK)

	got := read(release.ID)
	if got.Content != rewritten {
		t.Fatalf("setup: the comment reads %q after the edit, want the rewritten text %q", got.Content, rewritten)
	}
	if got.AuthorType != "member" || got.AuthorID != s.owner {
		t.Fatalf("rewritten comment author = %s/%s, want it still under the security owner %s: the author check alone cannot see the rewrite",
			got.AuthorType, got.AuthorID, s.owner)
	}
	if got.Revision != 2 {
		t.Fatalf("rewritten comment revision = %d, want 2: the revision is what the release check reads to refuse it", got.Revision)
	}

	// A reaction leaves the text alone and moves the same field.
	const untouched = "G2：接受。"
	reacted := post(untouched)
	react := newRequest("POST", "/api/comments/"+reacted.ID+"/reactions", map[string]any{"emoji": "👍"})
	testutil.Call(t, testHandler.AddReaction, withURLParam(react, "commentId", reacted.ID)).Want(http.StatusCreated)
	dbfx.Cleanup(t, `DELETE FROM comment_reaction WHERE comment_id = $1`, reacted.ID)

	got = read(reacted.ID)
	if got.Content != untouched || got.AuthorID != s.owner {
		t.Fatalf("setup: the reacted-to comment reads %q under %s, want the owner's own text", got.Content, got.AuthorID)
	}
	if got.Revision != 2 || len(got.Reactions) != 1 {
		t.Fatalf("reacted-to comment revision/reactions = %d/%d, want 2/1: a reaction alone makes the release check refuse the comment, which the instructions and the docs say",
			got.Revision, len(got.Reactions))
	}
}
