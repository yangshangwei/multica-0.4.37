package handler

import (
	"context"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Ordinary children retain the ordinary numeric, optional revision protocol.
// These requests explicitly supply it; they do not exercise a blind write.
func TestCollaborationOrdinaryChildPriorityAssigneeCASCommitOrders(t *testing.T) {
	for _, firstField := range []string{"priority", "assignee"} {
		t.Run(firstField+"_first", func(t *testing.T) {
			// The shared creation fixture closes creation during teardown. Restore
			// its original choice after that cleanup, without changing the epoch.
			gate := testutil.New(testPool, "", "")
			var creationEnabled bool
			gate.QueryRow(t, `SELECT creation_enabled FROM collaboration_execution_gate WHERE id=1`).Scan(&creationEnabled)
			gate.Cleanup(t, `UPDATE collaboration_execution_gate SET creation_enabled=$1 WHERE id=1`, creationEnabled)
			s := stageHTTPFixture(t)
			assignee := s.fx.User(t, "Ordinary child assignee", uuid.NewString()+"@example.test")
			s.fx.Member(t, s.fx.WorkspaceID, assignee, "member")
			child := s.fx.Issue(t, "Ordinary child CAS", testutil.Cols{
				"description": "Retained ordinary child content", "priority": "low",
				"parent_issue_id": s.issue, "project_id": s.project,
				"assignee_type": "member", "assignee_id": s.actor,
			})
			beforeChild, err := s.h.Queries.GetIssue(t.Context(), parseUUID(child))
			if err != nil {
				t.Fatal(err)
			}
			if beforeChild.Revision < 1 || beforeChild.ParentIssueID != parseUUID(s.issue) ||
				s.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1`, child) != 0 ||
				s.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1 AND enabled`, s.project) != 0 {
				t.Fatal("CAS requires an ordinary linked child and disabled stage policy")
			}

			parentState := func() string {
				t.Helper()
				var state string
				s.fx.QueryRow(t, `WITH sessions AS (SELECT id FROM collaboration_session WHERE issue_id=$1)
SELECT jsonb_build_object(
 'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
 'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
 'session',(SELECT to_jsonb(s) FROM collaboration_session s WHERE s.issue_id=$1),
 'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.user_id) FROM issue_participant p WHERE p.issue_id=$1),
 'handoffs',(SELECT jsonb_agg(to_jsonb(h) ORDER BY h.id) FROM issue_handoff h WHERE h.issue_id=$1),
 'events',(SELECT jsonb_agg(to_jsonb(e)-'outbox_state' ORDER BY e.event_seq,e.id) FROM collaboration_event e WHERE e.issue_id=$1),
 'notifications',(SELECT jsonb_agg(jsonb_build_object('id',n.id,'recipient',n.recipient_user_id,'event_seq',n.event_seq,'kind',n.kind,'mention',n.is_mention,'created',n.created_at) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=$1),
 'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_operation o WHERE (o.scope_kind='issue' AND o.scope_id=$1) OR (o.scope_kind='create' AND o.scope_id=$2)),
 'commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id IN (SELECT id FROM sessions)),
 'targets',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM collaboration_target t WHERE t.session_id IN (SELECT id FROM sessions)),
 'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_execution_group g WHERE g.session_id IN (SELECT id FROM sessions)),
 'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_grant g WHERE g.issue_id=$1),
 'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.session_id IN (SELECT id FROM sessions)),
 'run_events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM collaboration_run_event e WHERE e.run_id IN (SELECT id FROM collaboration_run WHERE session_id IN (SELECT id FROM sessions))),
 'terminations',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM collaboration_termination t WHERE t.issue_id=$1),
 'tasks',(SELECT jsonb_agg(to_jsonb(q) ORDER BY q.id) FROM agent_task_queue q WHERE q.issue_id IN ($1,$3))
)::text`, s.issue, s.fx.WorkspaceID, child).Scan(&state)
				return state
			}
			beforeParent, beforeBusiness := parentState(), s.stagePolicyBusinessSnapshot(t)
			assertParent := func() {
				t.Helper()
				if parentState() != beforeParent || s.stagePolicyBusinessSnapshot(t) != beforeBusiness {
					t.Fatal("ordinary child CAS changed parent collaboration, history, stage or execution facts")
				}
			}

			// Observe the existing bus without replacing either handler's services.
			// Delivery markers are separate from the retained parent business facts.
			var eventMu sync.Mutex
			var updates, hints []events.Event
			s.h.Bus.Subscribe(protocol.EventIssueUpdated, func(event events.Event) {
				if event.WorkspaceID == s.fx.WorkspaceID {
					eventMu.Lock()
					updates = append(updates, event)
					eventMu.Unlock()
				}
			})
			s.h.IssueService.Bus.Subscribe(service.CollaborationChildrenChanged, func(event events.Event) {
				payload, ok := event.Payload.(map[string]any)
				if ok && payload["issue_id"] == s.issue {
					eventMu.Lock()
					hints = append(hints, event)
					eventMu.Unlock()
				}
			})
			assertPublished := func(want int) {
				t.Helper()
				eventMu.Lock()
				defer eventMu.Unlock()
				if len(updates) != want || len(hints) != 0 {
					t.Fatalf("committed child updates/hints = %d/%d, want %d updates and zero hints for unchanged child references", len(updates), len(hints), want)
				}
				for i, event := range updates {
					payload, ok := event.Payload.(map[string]any)
					if !ok {
						t.Fatalf("child update %d lost its payload: %+v", i, event)
					}
					issue, ok := payload["issue"].(IssueResponse)
					priorityChanged := (i == 0) == (firstField == "priority")
					if !ok || issue.ID != child || issue.Revision != beforeChild.Revision+int64(i)+1 ||
						event.ActorType != "member" || event.ActorID != s.actor ||
						payload["priority_changed"] != priorityChanged || payload["assignee_changed"] != !priorityChanged || payload["status_changed"] != false {
						t.Fatalf("child update %d does not describe its one committed field: %+v", i, event)
					}
				}
			}

			priorityBody := map[string]any{"priority": "urgent", "expected_revision": beforeChild.Revision}
			assigneeBody := map[string]any{"assignee_id": assignee, "expected_revision": beforeChild.Revision}
			firstBody, secondBody := priorityBody, assigneeBody
			if firstField == "assignee" {
				firstBody, secondBody = assigneeBody, priorityBody
			}
			reached, release := make(chan struct{}), make(chan struct{})
			var barrierOnce, releaseOnce sync.Once
			var pid atomic.Uint32
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			first := *s.h
			first.TxStarter = stageExitHTTPBarrierStarter{
				inner: s.h.TxStarter, reached: reached, release: release,
				beforeCommit: true, once: &barrierOnce, pid: &pid,
			}
			responses := [2]chan *testutil.Response{make(chan *testutil.Response, 1), make(chan *testutil.Response, 1)}
			var started, collected [2]bool
			var cancellations []context.CancelFunc
			var requests sync.WaitGroup
			t.Cleanup(func() {
				unblock()
				for _, cancel := range cancellations {
					cancel()
				}
				requests.Wait()
				for i := range responses {
					if started[i] && !collected[i] {
						<-responses[i]
					}
				}
			})
			start := func(index int, handler http.HandlerFunc, body map[string]any) {
				started[index] = true
				request := stageExitHTTPRequest(s, http.MethodPut, "/api/issues/"+child, child, body)
				// Cancellation retains the helper's verified account, workspace
				// headers and actual chi resource parameter.
				requestCtx, requestCancel := context.WithCancel(request.Context())
				cancellations = append(cancellations, requestCancel)
				requests.Add(1)
				go func() {
					defer requests.Done()
					defer requestCancel()
					responses[index] <- testutil.Call(t, handler, request.WithContext(requestCtx))
				}()
			}
			collect := func(index int) *testutil.Response {
				t.Helper()
				select {
				case response := <-responses[index]:
					collected[index] = true
					return response
				case <-time.After(10 * time.Second):
					t.Fatal("ordinary child CAS request did not finish after releasing its commit barrier")
					return nil
				}
			}
			start(0, first.UpdateIssue, firstBody)
			select {
			case <-reached:
			case <-time.After(10 * time.Second):
				t.Fatal("ordinary child writer never reached its actual transaction commit")
			}
			if pid.Load() == 0 {
				t.Fatal("commit barrier did not retain its PostgreSQL holder identity")
			}
			start(1, s.h.UpdateIssue, secondBody)
			stageExitHTTPWaiter(t, s, pid.Load())
			assertPublished(0)
			unblock()
			winnerResponse, loserResponse := collect(0), collect(1)

			var winner IssueResponse
			winnerResponse.Want(http.StatusOK).JSON(&winner)
			type revisionConflict struct {
				Error            string `json:"error"`
				Code             string `json:"code"`
				ResourceType     string `json:"resource_type"`
				ResourceID       string `json:"resource_id"`
				ExpectedRevision int64  `json:"expected_revision"`
				ActualRevision   int64  `json:"actual_revision"`
			}
			var conflict revisionConflict
			loserResponse.Want(http.StatusConflict).JSON(&conflict)
			if conflict != (revisionConflict{
				Error: "resource changed since it was loaded", Code: "revision_conflict", ResourceType: "issue", ResourceID: child,
				ExpectedRevision: beforeChild.Revision, ActualRevision: beforeChild.Revision + 1,
			}) {
				t.Fatalf("losing ordinary child request did not retain the exact numeric CAS conflict: %+v", conflict)
			}
			assertChild := func(response IssueResponse, revision int64, priority, user string) {
				t.Helper()
				current, err := s.h.Queries.GetIssue(t.Context(), parseUUID(child))
				if err != nil {
					t.Fatal(err)
				}
				want := beforeChild
				want.Revision, want.Priority, want.AssigneeID = revision, priority, parseUUID(user)
				// The ordinary writer advances its database edit/activity clocks.
				// Every other unrequested child column must retain its old value.
				want.UpdatedAt, want.LastActivityAt = current.UpdatedAt, current.LastActivityAt
				if !reflect.DeepEqual(current, want) || response.ID != child || response.WorkspaceID != s.fx.WorkspaceID ||
					response.Revision != revision || response.Priority != priority || response.AssigneeType == nil || *response.AssigneeType != "member" ||
					response.AssigneeID == nil || *response.AssigneeID != user || response.ParentIssueID == nil || *response.ParentIssueID != s.issue || response.Collaboration != nil {
					t.Fatalf("ordinary child did not change only its winning field: response=%+v current=%+v want=%+v", response, current, want)
				}
			}
			winningPriority, winningAssignee := "urgent", s.actor
			if firstField == "assignee" {
				winningPriority, winningAssignee = beforeChild.Priority, assignee
			}
			assertChild(winner, beforeChild.Revision+1, winningPriority, winningAssignee)
			assertPublished(1)
			assertParent()

			// A client deliberately adopts the winner's numeric revision and
			// sends a new request for the remaining field, preserving old input.
			freshBody := map[string]any{"expected_revision": winner.Revision}
			if firstField == "priority" {
				freshBody["assignee_id"] = assignee
			} else {
				freshBody["priority"] = "urgent"
			}
			var fresh IssueResponse
			testutil.Call(t, s.h.UpdateIssue, stageExitHTTPRequest(s, http.MethodPut, "/api/issues/"+child, child, freshBody)).Want(http.StatusOK).JSON(&fresh)
			assertChild(fresh, beforeChild.Revision+2, "urgent", assignee)
			assertPublished(2)
			assertParent()
			t.Logf("ordinary child %s: %s committed at numeric revision %d; original competitor refused expected=%d actual=%d; explicit fresh request committed revision %d; parent retained",
				child, firstField, winner.Revision, conflict.ExpectedRevision, conflict.ActualRevision, fresh.Revision)
		})
	}
}
