package main

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Handler tests own the exhaustive dispatch matrix. These tests exercise actual
// authentication, durable recovery and the database identities behind admission.
type durableDelegationRuntimeObservation struct {
	Status     string `json:"status"`
	ObservedAt string `json:"observed_at"`
}
type durableDelegationTarget struct {
	TargetType         string                               `json:"target_type"`
	TargetID           string                               `json:"target_id"`
	Status             string                               `json:"status"`
	Reason             string                               `json:"reason_code"`
	TaskID             string                               `json:"task_id"`
	AgentID            string                               `json:"executing_agent_id"`
	SquadID            string                               `json:"executing_squad_id"`
	RuntimeObservation *durableDelegationRuntimeObservation `json:"runtime_observation"`
}
type durableDelegationReceipt struct {
	Key       string                    `json:"key"`
	State     string                    `json:"state"`
	CommentID string                    `json:"comment_id"`
	Targets   []durableDelegationTarget `json:"targets"`
}
type durableDelegationComment struct {
	ID         string                    `json:"id"`
	Content    string                    `json:"content"`
	AuthorType string                    `json:"author_type"`
	AuthorID   string                    `json:"author_id"`
	Outcomes   []durableDelegationTarget `json:"trigger_outcomes"`
	Receipt    *durableDelegationReceipt `json:"delegation_receipt"`
}
type durableDelegationFixture struct {
	fx                                         *testutil.Fixture
	router                                     http.Handler
	user, member, token, runtime, issue, agent string
}

func newDurableDelegationFixture(t *testing.T) *durableDelegationFixture {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	t.Setenv("MULTICA_DEVICE_AUTH_ENABLED", "false")
	t.Setenv("MULTICA_CLOUD_URL", "")
	setup := testutil.New(testPool, testWorkspaceID, testUserID)
	user := setup.User(t, "Delegation principal", "delegation-"+uuid.NewString()+"@example.test")
	member := setup.Member(t, testWorkspaceID, user, "member")
	fx := testutil.New(testPool, testWorkspaceID, user)
	runtime := fx.Runtime(t, "Delegation runtime")
	agent := fx.Agent(t, "Delegation target", runtime)
	issue := fx.Issue(t, "Durable delegation")
	// Register every row produced through the public API, not only fixture rows.
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1`, issue)
	fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, issue)
	fx.Cleanup(t, `DELETE FROM comment_delegation_receipt WHERE issue_id=$1`, issue)
	return &durableDelegationFixture{fx: fx, router: controlledArtifactRouter(t), user: user, member: member, token: controlledArtifactJWT(t, user, 0, time.Now().Add(time.Hour)), runtime: runtime, issue: issue, agent: agent}
}
func (f *durableDelegationFixture) comments() string { return "/api/issues/" + f.issue + "/comments" }
func (f *durableDelegationFixture) receipt(key string) string {
	return "/api/issues/" + f.issue + "/delegation-receipts/" + key
}
func (f *durableDelegationFixture) request(token, method, path, key string, body any) *http.Request {
	req := testutil.WithHeaders(testutil.JSONRequest(method, path, body), "Authorization", "Bearer "+token, "X-Workspace-ID", testWorkspaceID)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	return req
}
func (f *durableDelegationFixture) call(t *testing.T, token, method, path, key string, body any) *testutil.Response {
	t.Helper()
	return testutil.Call(t, f.router.ServeHTTP, f.request(token, method, path, key, body))
}
func (f *durableDelegationFixture) body() map[string]any {
	return map[string]any{"content": "Please handle [@Target](mention://agent/" + f.agent + ")."}
}
func (f *durableDelegationFixture) post(t *testing.T, token, key string, body any) durableDelegationComment {
	t.Helper()
	var out durableDelegationComment
	f.call(t, token, "POST", f.comments(), key, body).Want(201).JSON(&out)
	if out.Receipt == nil || out.Receipt.Key != key || out.Receipt.CommentID != out.ID || out.Receipt.State != "recorded" {
		t.Fatalf("missing recorded comment receipt: %+v", out)
	}
	if !reflect.DeepEqual(out.Receipt.Targets, out.Outcomes) {
		t.Fatalf("persisted targets differ from immediate outcomes: %+v", out)
	}
	return out
}
func (f *durableDelegationFixture) read(t *testing.T, token, key string) durableDelegationReceipt {
	t.Helper()
	var out durableDelegationReceipt
	r := f.call(t, token, "GET", f.receipt(key), "", nil).Want(200).JSON(&out)
	if r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("historical receipt can be cached")
	}
	return out
}
func assertDurableTask(t *testing.T, f *durableDelegationFixture, out durableDelegationComment, status string) durableDelegationTarget {
	t.Helper()
	if out.Receipt == nil || len(out.Receipt.Targets) != 1 {
		t.Fatalf("expected one target: %+v", out)
	}
	target := out.Receipt.Targets[0]
	if target.Status != status || target.TaskID == "" || target.AgentID != f.agent || target.TargetID != f.agent || target.TargetType != "agent" || target.Reason == "" {
		t.Fatalf("wrong admission identity: %+v", target)
	}
	var agent, issue, trigger string
	var covered bool
	f.fx.QueryRow(t, `SELECT agent_id::text,issue_id::text,trigger_comment_id::text,($2::uuid=trigger_comment_id OR $2::uuid=ANY(coalesced_comment_ids)) FROM agent_task_queue WHERE id=$1`, target.TaskID, out.ID).Scan(&agent, &issue, &trigger, &covered)
	if agent != target.AgentID || issue != f.issue || !covered {
		t.Fatalf("receipt does not match actual task: agent=%s issue=%s trigger=%s covered=%v", agent, issue, trigger, covered)
	}
	return target
}
func (f *durableDelegationFixture) taskToken(t *testing.T, agent, originator string) (token, task, tokenID string) {
	t.Helper()
	task = f.fx.Task(t, agent, testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "running", "originator_user_id": originator, "accountable_user_id": originator, "originator_source": "direct_human", "claim_generation": 1})
	var err error
	token, err = auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	tokenID = f.fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(token), "task_id": task, "agent_id": agent, "workspace_id": testWorkspaceID, "user_id": originator, "expires_at": time.Now().Add(time.Hour)})
	return
}
func (f *durableDelegationFixture) counts(t *testing.T) (int, int) {
	t.Helper()
	return f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue), f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue)
}

func TestDurableDelegationRouterJWTAndPATRecoverOriginalReceipt(t *testing.T) {
	f := newDurableDelegationFixture(t)
	key := uuid.NewString()
	original := f.post(t, f.token, key, f.body())
	target := assertDurableTask(t, f, original, "queued")
	// A newly constructed router has no request-local or leader memory.
	f.router = controlledArtifactRouter(t)
	if got := f.read(t, f.token, key); !reflect.DeepEqual(got, *original.Receipt) {
		t.Fatalf("restart changed receipt: %+v", got)
	}
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	patID := f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.user, "name": "Delegation PAT", "token_hash": auth.HashToken(pat), "token_prefix": pat[:8], "expires_at": time.Now().Add(time.Hour)})
	f.call(t, pat, "GET", f.comments(), "", nil).Want(200)
	deadline := time.Now().Add(5 * time.Second)
	for {
		var used bool
		f.fx.QueryRow(t, `SELECT last_used_at IS NOT NULL FROM personal_access_token WHERE id=$1`, patID).Scan(&used)
		if used {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("PAT authentication write did not commit")
		}
		time.Sleep(time.Millisecond)
	}
	if got := f.read(t, pat, key); !reflect.DeepEqual(got, *original.Receipt) {
		t.Fatal("same principal PAT changed original receipt")
	}
	var replay durableDelegationComment
	f.call(t, pat, "POST", f.comments(), key, f.body()).Want(200).JSON(&replay)
	if replay.ID != original.ID || !reflect.DeepEqual(replay.Receipt, original.Receipt) {
		t.Fatal("retry created or rewrote original receipt")
	}
	f.call(t, f.token, "POST", f.comments(), key, map[string]any{"content": "Changed body"}).Want(409)
	if comments, tasks := f.counts(t); comments != 1 || tasks != 1 {
		t.Fatalf("recovery repeated effects: %d comments, %d tasks", comments, tasks)
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment_delegation_receipt WHERE issue_id=$1 AND comment_id=$2`, f.issue, original.ID) != 1 {
		t.Fatal("missing unique durable receipt")
	}
	f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, patID)
	f.call(t, pat, "GET", f.receipt(key), "", nil).Want(401)
	if f.read(t, f.token, key).Targets[0].TaskID != target.TaskID {
		t.Fatal("PAT revocation rewrote history")
	}
}

func TestDurableDelegationRouterConcurrentRetryDispatchesOnce(t *testing.T) {
	f := newDurableDelegationFixture(t)
	key := uuid.NewString()
	const n = 8
	start := make(chan struct{})
	responses := make([]*testutil.Response, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			responses[i] = f.call(t, f.token, "POST", f.comments(), key, f.body())
		}()
	}
	close(start)
	wg.Wait()
	var comment string
	fresh := 0
	for _, r := range responses {
		if r.Code == 409 {
			if !strings.Contains(r.Text(), "delegation authority busy") {
				t.Fatalf("unexpected contention response: %s", r.Text())
			}
			continue
		}
		r.WantOneOf(200, 201)
		var out durableDelegationComment
		r.JSON(&out)
		if r.Code == 201 {
			fresh++
		}
		if comment == "" {
			comment = out.ID
		}
		if out.ID != comment || out.Receipt == nil || out.Receipt.CommentID != comment {
			t.Fatalf("concurrent key resolved different identities: %s", r.Text())
		}
		if out.Receipt.State != "recorded" && out.Receipt.State != "unknown" {
			t.Fatalf("invalid in-flight state: %s", r.Text())
		}
	}
	if fresh != 1 {
		t.Fatalf("fresh winners=%d, want1", fresh)
	}
	receipt := f.read(t, f.token, key)
	if receipt.State != "recorded" || len(receipt.Targets) != 1 || receipt.Targets[0].TaskID == "" {
		t.Fatalf("winner lacks final identity: %+v", receipt)
	}
	if comments, tasks := f.counts(t); comments != 1 || tasks != 1 {
		t.Fatalf("duplicate effects: comments=%d tasks=%d", comments, tasks)
	}
}

func TestDurableDelegationRouterQueuedMergeAndDeferredHaveRealTask(t *testing.T) {
	f := newDurableDelegationFixture(t)
	first := f.post(t, f.token, uuid.NewString(), f.body())
	queued := assertDurableTask(t, f, first, "queued")
	second := f.post(t, f.token, uuid.NewString(), f.body())
	merged := assertDurableTask(t, f, second, "coalesced")
	if merged.TaskID != queued.TaskID {
		t.Fatal("merge fabricated a different task")
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',dispatched_at=now(),claim_generation=1 WHERE id=$1`, queued.TaskID)
	third := f.post(t, f.token, uuid.NewString(), f.body())
	deferred := assertDurableTask(t, f, third, "deferred")
	if deferred.TaskID != queued.TaskID {
		t.Fatal("deferral lacks actual covering task")
	}
	if comments, tasks := f.counts(t); comments != 3 || tasks != 1 {
		t.Fatalf("coverage created extra task: comments=%d tasks=%d", comments, tasks)
	}
}

func TestDurableDelegationRouterTaskRestartKeepsActorAndOriginator(t *testing.T) {
	f := newDurableDelegationFixture(t)
	leader := f.fx.Agent(t, "Delegation leader", f.runtime)
	token, task, _ := f.taskToken(t, leader, f.user)
	key := uuid.NewString()
	original := f.post(t, token, key, f.body())
	assertDurableTask(t, f, original, "queued")
	if original.AuthorType != "agent" || original.AuthorID != leader {
		t.Fatalf("real task credential lost actor: %+v", original)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, task)
	fresh, newTask, newToken := f.taskToken(t, leader, f.user)
	f.router = controlledArtifactRouter(t)
	if got := f.read(t, fresh, key); !reflect.DeepEqual(got, *original.Receipt) {
		t.Fatal("new leader task did not recover original receipt")
	}
	var sourceTask string
	f.fx.QueryRow(t, `SELECT source_task_id::text FROM comment WHERE id=$1`, original.ID).Scan(&sourceTask)
	if sourceTask != task || sourceTask == newTask {
		t.Fatal("recovery rewrote original source task")
	}
	outsider := f.fx.User(t, "Other originator", "delegation-other-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, outsider, "member")
	// Preserve the agent identity while changing only the currently verified human.
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, newTask)
	f.fx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, leader)
	f.fx.InsertNoID(t, "agent_invocation_target", testutil.Cols{"agent_id": leader, "target_type": "member", "target_id": outsider, "created_by": f.user}, "agent_id=$1", leader)
	other, _, _ := f.taskToken(t, leader, outsider)
	f.call(t, other, "GET", f.receipt(key), "", nil).Want(404)
	f.fx.Exec(t, `DELETE FROM task_token WHERE id=$1`, newToken)
	f.call(t, fresh, "GET", f.receipt(key), "", nil).Want(401)
}

func TestDurableDelegationRouterSquadReplacementDoesNotRewriteTarget(t *testing.T) {
	f := newDurableDelegationFixture(t)
	squad := f.fx.Squad(t, "Original squad", f.agent)
	f.fx.SquadMember(t, squad, "agent", f.agent)
	key := uuid.NewString()
	body := map[string]any{"content": "Delegate [@Squad](mention://squad/" + squad + ")."}
	original := f.post(t, f.token, key, body)
	if len(original.Receipt.Targets) != 1 {
		t.Fatalf("missing squad receipt: %+v", original)
	}
	target := original.Receipt.Targets[0]
	if target.TargetType != "squad" || target.TargetID != squad || target.AgentID != f.agent || target.TaskID == "" || target.Status != "queued" {
		t.Fatalf("wrong original squad identity: %+v", target)
	}
	replacement := f.fx.Agent(t, "Replacement leader", f.runtime)
	f.fx.SquadMember(t, squad, "agent", replacement)
	f.fx.Exec(t, `UPDATE squad SET leader_id=$2 WHERE id=$1`, squad, replacement)
	f.fx.Exec(t, `DELETE FROM squad_member WHERE squad_id=$1 AND member_id=$2`, squad, f.agent)
	var replay durableDelegationComment
	f.call(t, f.token, "POST", f.comments(), key, body).Want(200).JSON(&replay)
	if !reflect.DeepEqual(replay.Receipt, original.Receipt) || !reflect.DeepEqual(f.read(t, f.token, key), *original.Receipt) {
		t.Fatal("replacement rebound historical target")
	}
	var agent string
	f.fx.QueryRow(t, `SELECT agent_id::text FROM agent_task_queue WHERE id=$1`, target.TaskID).Scan(&agent)
	if agent != f.agent || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2`, f.issue, replacement) != 0 {
		t.Fatal("retry dispatched replacement leader")
	}
}

func TestDurableDelegationRouterRejectedTargetsRemainTraceable(t *testing.T) {
	for _, kind := range []string{"unbound", "private"} {
		t.Run(kind, func(t *testing.T) {
			f := newDurableDelegationFixture(t)
			if kind == "unbound" {
				f.fx.Exec(t, `UPDATE agent SET runtime_id=NULL WHERE id=$1`, f.agent)
			} else {
				other := f.fx.User(t, "Private owner", "private-"+uuid.NewString()+"@example.test")
				f.fx.Member(t, testWorkspaceID, other, "member")
				f.fx.Exec(t, `UPDATE agent SET owner_id=$2 WHERE id=$1`, f.agent, other)
				f.fx.Exec(t, `UPDATE agent_runtime SET status='offline',metadata='{"offline_reason":{"code":"not_executable","detail":"private runtime sentinel"}}'::jsonb WHERE id=$1`, f.runtime)
			}
			key := uuid.NewString()
			original := f.post(t, f.token, key, f.body())
			if len(original.Receipt.Targets) != 1 {
				t.Fatalf("rejection lost intended target: %+v", original)
			}
			target := original.Receipt.Targets[0]
			wantReason := "invocation_not_allowed"
			if kind == "unbound" {
				wantReason = "agent_runtime_required"
			}
			if target.Reason != wantReason {
				t.Fatalf("refusal reason=%q,want %q", target.Reason, wantReason)
			}
			if target.Status != "blocked" || target.TargetID != f.agent || target.Reason == "" || target.TaskID != "" || target.RuntimeObservation != nil {
				t.Fatalf("untruthful rejection: %+v", target)
			}
			if !reflect.DeepEqual(f.read(t, f.token, key), *original.Receipt) {
				t.Fatal("rejection not durable")
			}
			if comments, tasks := f.counts(t); comments != 1 || tasks != 0 {
				t.Fatalf("blocked target ran: comments=%d tasks=%d", comments, tasks)
			}
		})
	}
}

func TestDurableDelegationRouterReadsRequireCurrentMembershipAndScope(t *testing.T) {
	f := newDurableDelegationFixture(t)
	key := uuid.NewString()
	f.post(t, f.token, key, f.body())
	other := f.fx.User(t, "Other reader", "reader-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, testWorkspaceID, other, "member")
	otherToken := controlledArtifactJWT(t, other, 0, time.Now().Add(time.Hour))
	denied := f.call(t, otherToken, "GET", f.receipt(key), "", nil).Want(404)
	if strings.Contains(denied.Text(), f.agent) {
		t.Fatal("receipt leaked private target")
	}
	otherIssue := f.fx.Issue(t, "Other resource")
	f.call(t, f.token, "GET", "/api/issues/"+otherIssue+"/delegation-receipts/"+key, "", nil).Want(404)
	f.fx.Exec(t, `DELETE FROM member WHERE id=$1`, f.member)
	revoked := f.call(t, f.token, "GET", f.receipt(key), "", nil).Want(404)
	if !strings.Contains(revoked.Text(), "workspace_access_denied") {
		t.Fatal("revocation did not reach workspace boundary")
	}
	f.call(t, f.token, "POST", f.comments(), key, f.body()).Want(404)
	if comments, tasks := f.counts(t); comments != 1 || tasks != 1 {
		t.Fatalf("revoked replay changed effects: %d %d", comments, tasks)
	}
}

func TestDurableDelegationRouterUnknownNeverReplaysAndLegacyWritesWork(t *testing.T) {
	f := newDurableDelegationFixture(t)
	key := uuid.NewString()
	original := f.post(t, f.token, key, f.body())
	// Model a retained claim with no final receipt. This is recovery-contract
	// coverage; the handler fault tests separately exercise interruption itself.
	f.fx.Exec(t, `UPDATE comment_delegation_receipt SET state='unknown',targets=(SELECT jsonb_agg((target - 'task_id' - 'executing_agent_id' - 'executing_squad_id' - 'runtime_observation') || '{"status":"unknown","reason_code":"dispatch_unconfirmed"}'::jsonb) FROM jsonb_array_elements(targets) AS target),response=response - 'delegation_receipt' - 'trigger_outcomes'  WHERE issue_id=$1 AND comment_id=$2`, f.issue, original.ID)
	f.router = controlledArtifactRouter(t)
	for range 2 {
		if got := f.read(t, f.token, key); got.State != "unknown" || got.CommentID != original.ID || len(got.Targets) != 1 || got.Targets[0].TargetID != f.agent || got.Targets[0].Status != "unknown" || got.Targets[0].TaskID != "" {
			t.Fatalf("unknown was promoted: %+v", got)
		}
		var replay durableDelegationComment
		f.call(t, f.token, "POST", f.comments(), key, f.body()).Want(200).JSON(&replay)
		if replay.Receipt == nil || replay.Receipt.State != "unknown" || replay.ID != original.ID {
			t.Fatalf("unknown replay fabricated completion: %+v", replay)
		}
	}
	if comments, tasks := f.counts(t); comments != 1 || tasks != 1 {
		t.Fatalf("unknown re-executed: comments=%d tasks=%d", comments, tasks)
	}
	var legacy durableDelegationComment
	f.call(t, f.token, "POST", f.comments(), "", map[string]any{"content": "/note Legacy comment"}).Want(201).JSON(&legacy)
	if legacy.Receipt != nil {
		t.Fatal("legacy request acquired new receipt")
	}
	f.call(t, f.token, "PUT", "/api/comments/"+legacy.ID, "", map[string]any{"content": "/note Legacy edit"}).Want(200)
}

func TestDurableDelegationRouterRejectsMalformedKeyWithoutEffects(t *testing.T) {
	f := newDurableDelegationFixture(t)
	for _, key := range []string{"not-a-uuid", uuid.Nil.String(), strings.ToUpper(uuid.NewString())} {
		f.call(t, f.token, "POST", f.comments(), key, f.body()).Want(400)
	}
	if comments, tasks := f.counts(t); comments != 0 || tasks != 0 {
		t.Fatalf("invalid key admitted effects: comments=%d tasks=%d", comments, tasks)
	}
}

// Keep cancellation failures visible rather than hiding them with retries.
func TestDurableDelegationRouterCanceledRequestDoesNotCreateReceipt(t *testing.T) {
	f := newDurableDelegationFixture(t)
	req := f.request(f.token, "POST", f.comments(), uuid.NewString(), f.body())
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	r := testutil.Call(t, f.router.ServeHTTP, req.WithContext(ctx))
	if r.Code >= 200 && r.Code < 300 {
		t.Fatalf("canceled request succeeded: %s", r.Text())
	}
	if comments, tasks := f.counts(t); comments != 0 || tasks != 0 {
		t.Fatalf("canceled admission wrote effects: %d %d", comments, tasks)
	}
}

func TestDurableDelegationRouterRejectsAttachmentsBeforeCreatingComment(t *testing.T) {
	f := newDurableDelegationFixture(t)
	body := f.body()
	body["attachment_ids"] = []string{uuid.NewString()}
	f.call(t, f.token, "POST", f.comments(), uuid.NewString(), body).Want(400)
	if comments, tasks := f.counts(t); comments != 0 || tasks != 0 {
		t.Fatalf("unsupported attachments admitted effects: %d %d", comments, tasks)
	}
}

func TestDurableDelegationRouterRevocationAfterAuthentication(t *testing.T) {
	for _, kind := range []string{"membership", "pat"} {
		t.Run(kind, func(t *testing.T) {
			f := newDurableDelegationFixture(t)
			token := f.token
			var patID string
			if kind == "pat" {
				var err error
				token, err = auth.GeneratePATToken()
				if err != nil {
					t.Fatal(err)
				}
				patID = f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": f.user, "name": "Revoked during admission", "token_hash": auth.HashToken(token), "token_prefix": token[:8], "expires_at": time.Now().Add(time.Hour)})
			}
			req := f.request(token, "POST", f.comments(), uuid.NewString(), f.body())
			revoked := false
			req.Body = &artifactRevokeOnRead{ReadCloser: req.Body, revoke: func() {
				if kind == "membership" {
					f.fx.Exec(t, `DELETE FROM member WHERE id=$1`, f.member)
				} else {
					f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, patID)
				}
				revoked = true
			}}
			r := testutil.Call(t, f.router.ServeHTTP, req)
			if !revoked {
				t.Fatal("request did not reach post-authentication mutation")
			}
			if kind == "pat" {
				r.Want(404)
				if !strings.Contains(r.Text(), "delegation receipt not found") {
					t.Fatal("revoked PAT bypassed current credential fence")
				}
			} else {
				r.Want(403)
				if !strings.Contains(r.Text(), "delegation authority unavailable") {
					t.Fatal("revoked membership bypassed current authority fence")
				}
			}
			if comments, tasks := f.counts(t); comments != 0 || tasks != 0 {
				t.Fatalf("revoked authority admitted effects: %d %d", comments, tasks)
			}
		})
	}
}

func TestDurableDelegationRouterHumanHeadersCannotImpersonateAgent(t *testing.T) {
	f := newDurableDelegationFixture(t)
	leader := f.fx.Agent(t, "Header target", f.runtime)
	_, task, _ := f.taskToken(t, leader, f.user)
	req := f.request(f.token, "POST", f.comments(), uuid.NewString(), f.body())
	req.Header.Set("X-Agent-ID", leader)
	req.Header.Set("X-Task-ID", task)
	denied := testutil.Call(t, f.router.ServeHTTP, req).Want(403)
	if !strings.Contains(denied.Text(), "delegation authority unavailable") {
		t.Fatal("spoofed actor did not reach durable principal binding")
	}
	if comments, tasks := f.counts(t); comments != 0 || tasks != 1 {
		t.Fatalf("spoofed actor dispatched: comments=%d tasks=%d", comments, tasks)
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment_delegation_receipt WHERE issue_id=$1`, f.issue) != 0 {
		t.Fatal("spoofed actor acquired durable key")
	}
}

func TestDurableDelegationRouterRestrictedCredentialRecoversOnlyOwnScope(t *testing.T) {
	f := newDurableDelegationFixture(t)
	reviewer := f.fx.Agent(t, "Receipt reviewer", f.runtime, testutil.Cols{"template_key": "code-reviewer", "template_version": 2, "autonomy_level": "observer"})
	token, task, tokenID := f.taskToken(t, reviewer, f.user)
	capability, err := json.Marshal(auth.TaskCapability{Version: 1, Profile: auth.TaskCapabilityReviewer, IssueID: f.issue, OriginatorID: f.user, RuntimeID: f.runtime, ClaimGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE task_token SET api_capability=$2 WHERE id=$1`, tokenID, capability)
	key := uuid.NewString()
	original := f.post(t, token, key, f.body())
	if len(original.Receipt.Targets) != 1 || original.Receipt.Targets[0].Status != "blocked" || original.Receipt.Targets[0].Reason != "invocation_not_allowed" || original.Receipt.Targets[0].TaskID != "" {
		t.Fatalf("restricted reviewer acquired dispatch authority: %+v", original.Receipt)
	}
	var eligible bool
	f.fx.QueryRow(t, `SELECT dispatch_eligible FROM comment WHERE id=$1 AND source_task_id=$2`, original.ID, task).Scan(&eligible)
	if eligible {
		t.Fatal("restricted comment became executable")
	}
	f.router = controlledArtifactRouter(t)
	if got := f.read(t, token, key); !reflect.DeepEqual(got, *original.Receipt) {
		t.Fatal("restricted same-scope receipt not recoverable")
	}
	outside := f.fx.Issue(t, "Outside restricted receipt scope")
	f.call(t, token, "GET", "/api/issues/"+outside+"/delegation-receipts/"+key, "", nil).Want(403)
	f.call(t, token, "GET", f.receipt(uuid.NewString()), "", nil).Want(404)
	f.call(t, token, "GET", f.receipt("malformed-key"), "", nil).Want(403)
	if comments, tasks := f.counts(t); comments != 1 || tasks != 1 {
		t.Fatalf("restricted recovery dispatched effects: comments=%d tasks=%d", comments, tasks)
	}
}

func TestDurableDelegationRouterOfflineQueuesAndRetainsObservation(t *testing.T) {
	f := newDurableDelegationFixture(t)
	f.fx.Exec(t, `UPDATE agent_runtime SET status='offline',last_seen_at=now() WHERE id=$1`, f.runtime)
	started := time.Now().Add(-time.Second)
	key := uuid.NewString()
	original := f.post(t, f.token, key, f.body())
	target := assertDurableTask(t, f, original, "queued")
	if target.RuntimeObservation == nil || target.RuntimeObservation.Status != "offline" {
		t.Fatalf("queued offline task lost observed availability: %+v", target)
	}
	observed, err := time.Parse(time.RFC3339Nano, target.RuntimeObservation.ObservedAt)
	if err != nil || !strings.HasSuffix(target.RuntimeObservation.ObservedAt, "Z") || observed.Before(started) || observed.After(time.Now().Add(time.Second)) {
		t.Fatalf("availability observation lacks bounded UTC timestamp: %+v error=%v", target.RuntimeObservation, err)
	}
	var taskStatus, runtimeID, retained string
	f.fx.QueryRow(t, `SELECT status,runtime_id::text FROM agent_task_queue WHERE id=$1`, target.TaskID).Scan(&taskStatus, &runtimeID)
	if taskStatus != "queued" || runtimeID != f.runtime {
		t.Fatalf("offline admission did not wait on original runtime: %s %s", taskStatus, runtimeID)
	}
	f.fx.QueryRow(t, `SELECT targets::text FROM comment_delegation_receipt WHERE issue_id=$1 AND comment_id=$2`, f.issue, original.ID).Scan(&retained)
	if strings.Contains(retained, f.runtime) {
		t.Fatal("availability observation leaked runtime identity")
	}
	f.fx.Exec(t, `UPDATE agent_runtime SET status='online',last_seen_at=now() WHERE id=$1`, f.runtime)
	replacementRuntime := f.fx.Runtime(t, "Replacement online runtime")
	f.fx.Exec(t, `UPDATE agent SET runtime_id=$2 WHERE id=$1`, f.agent, replacementRuntime)
	f.router = controlledArtifactRouter(t)
	if got := f.read(t, f.token, key); !reflect.DeepEqual(got, *original.Receipt) {
		t.Fatal("restart recomputed offline receipt from online runtime")
	}
	var replay durableDelegationComment
	f.call(t, f.token, "POST", f.comments(), key, f.body()).Want(200).JSON(&replay)
	if !reflect.DeepEqual(replay.Receipt, original.Receipt) {
		t.Fatal("online retry rewrote original offline observation")
	}
	var after string
	f.fx.QueryRow(t, `SELECT targets::text FROM comment_delegation_receipt WHERE issue_id=$1 AND comment_id=$2`, f.issue, original.ID).Scan(&after)
	if after != retained {
		t.Fatal("recovery changed durable availability snapshot")
	}
	var taskRuntime, agentRuntime string
	f.fx.QueryRow(t, `SELECT t.runtime_id::text,a.runtime_id::text FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id WHERE t.id=$1`, target.TaskID).Scan(&taskRuntime, &agentRuntime)
	if taskRuntime != f.runtime || agentRuntime != replacementRuntime {
		t.Fatalf("recovery changed historical task binding: task runtime=%s,current agent runtime=%s", taskRuntime, agentRuntime)
	}
	if comments, tasks := f.counts(t); comments != 1 || tasks != 1 {
		t.Fatalf("offline recovery repeated effects: comments=%d tasks=%d", comments, tasks)
	}
}

func TestDurableDelegationRouterUnusableRejectsWithoutRepeatNotice(t *testing.T) {
	f := newDurableDelegationFixture(t)
	f.fx.Exec(t, `UPDATE agent_runtime SET status='offline',metadata='{"offline_reason":{"code":"not_executable","detail":"synthetic unusable CLI"}}'::jsonb WHERE id=$1`, f.runtime)
	key := uuid.NewString()
	original := f.post(t, f.token, key, f.body())
	if len(original.Receipt.Targets) != 1 {
		t.Fatalf("missing unusable target receipt: %+v", original.Receipt)
	}
	target := original.Receipt.Targets[0]
	if target.TargetID != f.agent || target.TargetType != "agent" || target.Status != "blocked" || target.Reason != "runtime_unusable" || target.TaskID != "" || target.RuntimeObservation != nil {
		t.Fatalf("unusable runtime received false admission: %+v", target)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND author_type='system' AND type='system'`, f.issue); got != 1 {
		t.Fatalf("unusable runtime notice count=%d,want1", got)
	}
	var replay durableDelegationComment
	f.call(t, f.token, "POST", f.comments(), key, f.body()).Want(200).JSON(&replay)
	if !reflect.DeepEqual(replay.Receipt, original.Receipt) || !reflect.DeepEqual(f.read(t, f.token, key), *original.Receipt) {
		t.Fatal("unusable refusal did not persist")
	}
	if comments, tasks := f.counts(t); comments != 2 || tasks != 0 {
		t.Fatalf("unusable retry repeated notice or queued work: comments=%d tasks=%d", comments, tasks)
	}
}
