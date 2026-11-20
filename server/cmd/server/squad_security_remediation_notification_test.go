package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// securityRemediationNotificationRouter builds the production router on a bus
// that carries the realtime, subscriber, activity and notification listeners,
// in the order main.go registers them. The shared testServer registers only
// the realtime listeners (see TestMain), so a request sent to it never reaches
// the subscriber or notification listeners.
func securityRemediationNotificationRouter(t *testing.T) (http.HandlerFunc, *db.Queries) {
	t.Helper()
	queries := db.New(testPool)
	hub := realtime.NewHub()
	go hub.Run()
	bus := events.New()
	registerListeners(bus, hub)
	// Subscriber listeners register before notification listeners: the
	// notification listener reads the subscriber table to pick recipients.
	registerSubscriberListeners(bus, testPool)
	registerActivityListeners(bus, queries)
	registerNotificationListeners(bus, queries)
	router, _ := NewRouterWithOptions(testPool, hub, bus, analytics.NoopClient{}, nil, RouterOptions{})
	return router.ServeHTTP, queries
}

// The security-remediation v2 instructions have a human security owner read
// the reviewers' results and release the gate. Reviewers post with a
// restricted task token, so their comments are stored dispatch-ineligible and
// wake no agent; the owner hears about a result only through the inbox, which
// is why the leader subscribes them to the issue first.
//
// This pins that a restricted reviewer's comment, posted through the real
// create-comment route, notifies a member whose only tie to the issue is that
// subscription. TestNotification_CommentCreated covers the listener alone,
// with a hand-published event and a member author.
func TestSecurityRemediationNotification_RestrictedReviewerCommentReachesSubscribedMember(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	serve, queries := securityRemediationNotificationRouter(t)
	suffix := uuid.NewString()
	asSuiteMember := func(req *http.Request) *http.Request {
		return testutil.WithHeaders(req, "Authorization", "Bearer "+testToken, "X-Workspace-ID", testWorkspaceID)
	}

	// The security owner is not the issue's creator, its assignee or the
	// task's originator (the suite user is the first and the last, and the
	// issue has no assignee), so the subscription is the only reason the
	// platform has to notify them. The control member differs from the owner
	// only in not being subscribed.
	owner := fx.User(t, "Security owner", "secrem-owner-"+suffix+"@example.test")
	fx.Member(t, testWorkspaceID, owner, "member")
	control := fx.User(t, "Unsubscribed member", "secrem-control-"+suffix+"@example.test")
	fx.Member(t, testWorkspaceID, control, "member")

	// The reviewer is the built-in role, created through the from-template
	// route, so the server stamps its template key and version and the claim
	// below decides its capability from the registry version that ships.
	runtime := fx.Runtime(t, "Security remediation reviewer runtime")
	var before time.Time
	fx.QueryRow(t, `SELECT now()`).Scan(&before)
	var reviewer handler.AgentResponse
	testutil.Call(t, serve, asSuiteMember(testutil.JSONRequest(http.MethodPost, "/api/agents/from-template", map[string]any{
		"template_key": "security-reviewer",
		"runtime_id":   runtime,
		"name":         "Security reviewer " + suffix,
	}))).Want(http.StatusCreated).JSON(&reviewer)
	// Role skills are workspace assets that outlive the agent. Remove the ones
	// this request materialized, after the agent that holds them is gone, and
	// leave any that were already there.
	for _, skill := range reviewer.Skills {
		fx.Cleanup(t, `DELETE FROM skill WHERE id=$1 AND created_at >= $2`, skill.ID, before)
	}
	fx.Cleanup(t, `DELETE FROM agent WHERE id=$1`, reviewer.ID)
	if reviewer.ID == "" || reviewer.TemplateKey != "security-reviewer" {
		t.Fatalf("reviewer: created agent %q carries template key %q, want security-reviewer", reviewer.ID, reviewer.TemplateKey)
	}

	issue := fx.Issue(t, "Security remediation notification "+suffix)
	// Handlers and listeners write these rows.
	fx.Cleanup(t, `DELETE FROM inbox_item WHERE issue_id=$1`, issue)
	fx.Cleanup(t, `DELETE FROM issue_subscriber WHERE issue_id=$1`, issue)
	task := fx.Task(t, reviewer.ID, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "originator_user_id": testUserID, "accountable_user_id": testUserID})
	fx.Cleanup(t, `DELETE FROM task_token WHERE task_id=$1`, task)

	// Someone else subscribes the owner through the real route, with the body
	// `multica issue subscriber add --user-id` sends.
	testutil.Call(t, serve, asSuiteMember(testutil.JSONRequest(http.MethodPost, "/api/issues/"+issue+"/subscribe", map[string]any{
		"user_type": "member",
		"user_id":   owner,
	}))).Want(http.StatusOK)
	if n := fx.Count(t, `SELECT count(*) FROM issue_subscriber WHERE issue_id=$1 AND user_type='member' AND user_id=$2 AND reason='manual'`, issue, owner); n != 1 {
		t.Fatalf("subscribe: owner has %d manual subscriptions on the issue, want 1", n)
	}
	if n := fx.Count(t, `SELECT count(*) FROM issue_subscriber WHERE issue_id=$1 AND user_id=$2`, issue, control); n != 0 {
		t.Fatalf("subscribe: control member has %d subscriptions on the issue, want 0", n)
	}
	const inboxRows = `SELECT count(*) FROM inbox_item WHERE recipient_type='member' AND recipient_id=$1`
	if n := fx.Count(t, inboxRows, owner); n != 0 {
		t.Fatalf("subscribe: owner has %d inbox items before the reviewer commented, want 0", n)
	}

	// The reviewer's task is claimed through the daemon route, so the server
	// itself decides which capability the task token carries.
	var claimed struct {
		Task *handler.AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, serve, asSuiteMember(testutil.JSONRequest(http.MethodPost, "/api/daemon/runtimes/"+runtime+"/tasks/claim", nil))).
		Want(http.StatusOK).JSON(&claimed)
	if claimed.Task == nil {
		t.Fatal("claim: the runtime claimed no task")
	}
	if claimed.Task.ID != task {
		t.Fatalf("claim: claimed task %s, want the reviewer task %s", claimed.Task.ID, task)
	}
	token := claimed.Task.AuthToken
	if token == "" {
		t.Fatal("claim: response carries no task token")
	}

	// Without a restricted token the rest of the test proves nothing: the
	// stored capability is the reviewer profile bound to this issue, and the
	// router enforces it by refusing the reviewer an edit of that same issue.
	var rawCapability []byte
	fx.QueryRow(t, `SELECT api_capability FROM task_token WHERE token_hash=$1 AND task_id=$2`, auth.HashToken(token), task).Scan(&rawCapability)
	capability, err := auth.ParseTaskCapability(rawCapability)
	if err != nil {
		t.Fatalf("claim: the reviewer's task token carries no valid capability (stored %q): %v", rawCapability, err)
	}
	if capability.Profile != auth.TaskCapabilityReviewer || capability.IssueID != issue {
		t.Fatalf("claim: capability = %+v, want profile %q bound to issue %s", capability, auth.TaskCapabilityReviewer, issue)
	}
	asReviewer := func(req *http.Request) *http.Request {
		return testutil.WithHeaders(req, "Authorization", "Bearer "+token)
	}
	testutil.Call(t, serve, asReviewer(testutil.JSONRequest(http.MethodPut, "/api/issues/"+issue, map[string]any{"title": "Reviewer edit"}))).
		Want(http.StatusForbidden)

	// The reviewer reports its result: top level, no mention.
	const content = "Security review result: no blocking findings."
	var comment handler.CommentResponse
	testutil.Call(t, serve, asReviewer(testutil.JSONRequest(http.MethodPost, "/api/issues/"+issue+"/comments", map[string]any{"content": content}))).
		Want(http.StatusCreated).JSON(&comment)
	if comment.ID == "" {
		t.Fatal("comment: create-comment response carries no comment id")
	}
	var eligible bool
	var authorType, authorID string
	fx.QueryRow(t, `SELECT dispatch_eligible, author_type, author_id::text FROM comment WHERE id=$1 AND issue_id=$2`, comment.ID, issue).Scan(&eligible, &authorType, &authorID)
	if authorType != "agent" || authorID != reviewer.ID {
		t.Fatalf("comment: stored author = %s %s, want agent %s", authorType, authorID, reviewer.ID)
	}
	if eligible {
		t.Fatal("comment: a restricted reviewer's comment was stored dispatch_eligible = true")
	}

	// No waiting: the route publishes comment:created before it responds and
	// events.Bus runs its listeners inline, so the inbox row is already there.
	// The raw count sees every row addressed to the owner, archived or not;
	// the listing is the query their inbox is served from.
	if n := fx.Count(t, inboxRows, owner); n != 1 {
		t.Fatalf("notify: subscribed member has %d inbox items after the restricted reviewer's comment, want exactly 1", n)
	}
	items := inboxItemsForRecipient(t, queries, owner)
	if len(items) != 1 {
		t.Fatalf("notify: subscribed member's inbox lists %d items, want exactly 1", len(items))
	}
	item := items[0]
	var details struct {
		CommentID string `json:"comment_id"`
	}
	if err := json.Unmarshal(item.Details, &details); err != nil {
		t.Fatalf("notify: inbox item details %q are not JSON: %v", item.Details, err)
	}
	t.Logf("subscribed member's inbox item: type=%s issue=%s comment=%s actor=%s %s read=%t", item.Type, util.UUIDToString(item.IssueID), details.CommentID, item.ActorType.String, util.UUIDToString(item.ActorID), item.Read)
	if item.Type != "new_comment" {
		t.Fatalf("notify: inbox item type = %q, want new_comment", item.Type)
	}
	if got := util.UUIDToString(item.IssueID); got != issue {
		t.Fatalf("notify: inbox item points at issue %s, want %s", got, issue)
	}
	if details.CommentID != comment.ID {
		t.Fatalf("notify: inbox item points at comment %q, want the reviewer's comment %s", details.CommentID, comment.ID)
	}
	if got := util.UUIDToString(item.ActorID); item.ActorType.String != "agent" || got != reviewer.ID {
		t.Fatalf("notify: inbox item actor = %s %s, want agent %s", item.ActorType.String, got, reviewer.ID)
	}
	if item.Read {
		t.Fatal("notify: the new_comment inbox item was written already read")
	}

	// Control: workspace membership alone earns no notification.
	if n := fx.Count(t, inboxRows, control); n != 0 {
		t.Fatalf("notify: unsubscribed member has %d inbox items, want 0", n)
	}
}
