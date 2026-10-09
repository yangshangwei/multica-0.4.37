package handler

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// Each owner must wait before it takes any issue, attachment or agent lock.
// The second connection proves the rows remain writable during that wait.
func TestCreationIterationFenceAtOwnerEntry(t *testing.T) {
	for _, owner := range []string{"ordinary", "autopilot", "lifecycle", "runtime_onboarding", "no_runtime_onboarding", "triage_intake", "triage_import", "triage_accept", "triage_reject", "triage_reopen"} {
		t.Run(owner, func(t *testing.T) {
			_, iterationID := iterationIssueFixture(t)
			h := *testHandler
			source := lifecycleAtomicSource(t, "Creation fence source")
			agent := dbfx.Agent(t, "Creation fence agent", testRuntimeID)
			project := dbfx.Project(t, "Creation fence project")
			attachment := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "uploader_type": "member", "uploader_id": testUserID, "filename": "fence.txt", "url": "/fence.txt", "content_type": "text/plain", "size_bytes": 1})
			var item TriageItem
			var imported TriageImportPreview
			if owner == "triage_import" {
				triageImportSetup(t)
				imported = triageImportPreviewForTest(t, "title\nIteration import fence\n")
			} else if owner == "triage_intake" || owner == "triage_accept" || owner == "triage_reject" || owner == "triage_reopen" {
				triageEnableForTest(t)
				if owner != "triage_intake" {
					item = triageCreateForTest(t)
					if owner == "triage_reopen" {
						item = triageActionForTest(t, item, "reject", map[string]any{"reason": "Review later"}, 200).Item
					}
				}
			}
			dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title IN ('Iteration create fence','Iteration intake fence','Iteration import fence',$2,$3)`, testWorkspaceID, onboardingIssueTitle, noRuntimeIssueTitle)
			dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id IN (SELECT id FROM agent WHERE workspace_id=$1 AND name=$2)`, testWorkspaceID, onboardingAssistantName)
			dbfx.Cleanup(t, `DELETE FROM agent WHERE workspace_id=$1 AND name=$2`, testWorkspaceID, onboardingAssistantName)
			reached := make(chan struct{}, 8)
			h.TxStarter = iterationFenceBarrierStarter{h.TxStarter, reached}
			issueService := *h.IssueService
			issueService.TxStarter = h.TxStarter
			h.IssueService = &issueService
			autopilotService := *h.AutopilotService
			autopilotService.TxStarter = h.TxStarter
			h.AutopilotService = &autopilotService
			var handler http.HandlerFunc
			var request *http.Request
			want := 201
			switch owner {
			case "ordinary":
				handler = h.CreateIssue
				request = newRequest("POST", "/api/issues", map[string]any{"title": "Iteration create fence", "status": "backlog"})
			case "autopilot":
				ap := dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Iteration create fence", "assignee_type": "agent", "assignee_id": agent, "status": "active", "execution_mode": "create_issue", "created_by_type": "member", "created_by_id": testUserID, "project_id": project})
				dbfx.Cleanup(t, `DELETE FROM issue WHERE origin_id=$1`, ap)
				dbfx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, ap)
				dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agent)
				handler, want = h.TriggerAutopilot, 200
				request = withURLParam(newRequest("POST", "/api/autopilots/"+ap+"/trigger", nil), "id", ap)
			case "lifecycle":
				handler = h.CreateLifecycleHandoff
				request = withURLParam(newRequest("POST", "/api/issues/"+source+"/lifecycle-handoffs", map[string]any{"kind": "rca", "route": "maintenance", "cause_state": "unknown", "follow_up_title": "Iteration create fence", "assignee_type": "agent", "assignee_id": agent}), "id", source)
			case "runtime_onboarding":
				handler, want = h.BootstrapOnboardingRuntime, 200
				request = newRequest("POST", "/api/me/onboarding/runtime-bootstrap", map[string]any{"workspace_id": testWorkspaceID, "runtime_id": testRuntimeID})
			case "no_runtime_onboarding":
				handler, want = h.BootstrapOnboardingNoRuntime, 200
				request = newRequest("POST", "/api/me/onboarding/no-runtime-bootstrap", map[string]any{"workspace_id": testWorkspaceID})
			case "triage_intake":
				handler = h.CreateTriageItem
				request = newRequest("POST", "/api/triage/items", map[string]any{"request_id": uuidToString(dbid.NewV7()), "title": "Iteration intake fence", "attachment_ids": []string{attachment}})
			case "triage_import":
				handler, want = h.CommitTriageImport, 200
				request = withURLParam(newRequest("POST", "/api/triage/imports/"+imported.BatchID+"/commit", map[string]any{"rows": []map[string]any{{"row_number": 1}}}), "id", imported.BatchID)
			default:
				handler, want = h.ActOnTriageItem, 200
				action := map[string]string{"triage_accept": "accept", "triage_reject": "reject", "triage_reopen": "reopen"}[owner]
				request = withURLParam(newRequest("POST", "/api/triage/items/"+item.Issue.ID+"/actions", map[string]any{"request_id": uuidToString(dbid.NewV7()), "expected_revision": item.Issue.Revision, "action": action, "reason": "Fence test"}), "id", item.Issue.ID)
			}
			ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
			defer cancel()
			holder, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			if err = iteration.LockWorkspace(ctx, holder, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, handler, request.WithContext(ctx)) }()
			select {
			case <-reached:
			case response := <-result:
				t.Fatalf("%s bypassed the held I1 fence: HTTP %d %s", owner, response.Code, response.Body.String())
			case <-ctx.Done():
				t.Fatal("owner did not reach the iteration fence")
			}
			for _, row := range []struct{ table, id string }{{"issue", source}, {"agent", agent}, {"attachment", attachment}, {"project", project}} {
				if _, err = holder.Exec(ctx, "SELECT id FROM "+row.table+" WHERE id=$1 FOR UPDATE NOWAIT", row.id); err != nil {
					t.Fatalf("%s locked before fence: %v", row.table, err)
				}
			}
			if item.Issue.ID != "" {
				if _, err = holder.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, item.Issue.ID); err != nil {
					t.Fatalf("triage issue locked before fence: %v", err)
				}
			}
			if imported.BatchID != "" {
				if _, err = holder.Exec(ctx, `SELECT batch_id FROM triage_import_row WHERE batch_id=$1 FOR UPDATE NOWAIT`, imported.BatchID); err != nil {
					t.Fatalf("import row locked before fence: %v", err)
				}
			}
			if err = holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			(<-result).Want(want)
			var count, scope int
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
			dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
			if count != 0 || scope != 1 {
				t.Fatalf("unassociated creation/review fabricated facts: count=%d scope=%d", count, scope)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE current_iteration_id=$1`, iterationID); n != 1 {
				t.Fatalf("creation/review implicitly joined the iteration: %d members", n)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM iteration_participation WHERE iteration_id=$1`, iterationID); n != 1 {
				t.Fatalf("creation/review created participation: %d rows", n)
			}
		})
	}
}

func TestTriageIterationInvalidMembershipFailsClosed(t *testing.T) {
	for _, action := range []string{"accept", "reject", "duplicate", "reopen", "snooze"} {
		t.Run(action, func(t *testing.T) {
			_, iterationID := iterationIssueFixture(t)
			syncCreationIterationCounter(t)
			triageEnableForTest(t)
			item := triageCreateForTest(t)
			if action == "reopen" {
				item = triageActionForTest(t, item, "reject", map[string]any{"reason": "Later"}, 200).Item
			}
			dbfx.Exec(t, `UPDATE issue SET current_iteration_id=$1 WHERE id=$2`, iterationID, item.Issue.ID)
			extra := map[string]any{"reason": "Must not write", "snoozed_until": time.Now().Add(time.Hour).Format(time.RFC3339), "duplicate_issue_id": dbfx.Issue(t, "Duplicate target")}
			triageActionForTest(t, item, action, extra, 409)
			var admission string
			var revision, scope, events int
			dbfx.QueryRow(t, `SELECT admission_status,revision FROM issue WHERE id=$1`, item.Issue.ID).Scan(&admission, &revision)
			dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
			if admission != item.Issue.AdmissionStatus || int64(revision) != item.Issue.Revision || scope != 1 || events != 0 {
				t.Fatalf("invalid membership partially mutated: admission=%s revision=%d scope=%d events=%d", admission, revision, scope, events)
			}
		})
	}
}

func TestTriageIterationExplicitAssignmentRequiresEnabledWorkspace(t *testing.T) {
	_, iterationID := iterationIssueFixture(t)
	syncCreationIterationCounter(t)
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	triageActionForTest(t, item, "accept", map[string]any{"fields": map[string]any{"current_iteration_id": iterationID}}, 409)
	var admission string
	var unassociated bool
	dbfx.QueryRow(t, `SELECT admission_status,current_iteration_id IS NULL FROM issue WHERE id=$1`, item.Issue.ID).Scan(&admission, &unassociated)
	if admission != "pending" || !unassociated {
		t.Fatal("disabled assignment changed intake")
	}
}

func TestSourceContextCreateIterationFencePrecedesSourceLock(t *testing.T) {
	source, _ := iterationIssueFixture(t)
	syncCreationIterationCounter(t)
	comment := dbfx.Comment(t, source, "Context for new issue")
	build, err := service.BuildSourceContext(t.Context(), testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(comment))
	if err != nil {
		t.Fatal(err)
	}
	capture, err := service.PrepareSourceContextCapture(build, dbid.NewV7(), parseUUID(testWorkspaceID), parseUUID(testUserID), time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title='Captured creation fence'`, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM issue_source_context WHERE id=$1`, capture.ID)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	holder, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if err = iteration.LockWorkspace(ctx, holder, parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{}, 2)
	svc := *testHandler.IssueService
	svc.TxStarter = iterationFenceBarrierStarter{svc.TxStarter, reached}
	result := make(chan error, 1)
	go func() {
		_, e := svc.Create(ctx, service.IssueCreateParams{WorkspaceID: parseUUID(testWorkspaceID), Title: "Captured creation fence", Status: "backlog", Priority: "none", CreatorType: "member", CreatorID: parseUUID(testUserID), SourceContext: &capture}, service.IssueCreateOpts{})
		result <- e
	}()
	select {
	case <-reached:
	case err = <-result:
		t.Fatalf("source creation bypassed held I1 fence: %v", err)
	case <-ctx.Done():
		t.Fatal("source creation did not reach I1 fence")
	}
	for _, row := range []struct{ table, id string }{{"issue", source}, {"comment", comment}} {
		if _, err = holder.Exec(ctx, "SELECT id FROM "+row.table+" WHERE id=$1 FOR UPDATE NOWAIT", row.id); err != nil {
			t.Fatalf("source locked before I1 fence: %v", err)
		}
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-result; err != nil {
		t.Fatal(err)
	}
}

func TestAutopilotCreationUsesAssignmentAfterSquadDeletion(t *testing.T) {
	_, iterationID := iterationIssueFixture(t)
	syncCreationIterationCounter(t)
	agent := dbfx.Agent(t, "Creation dispatch leader", testRuntimeID)
	squad := dbfx.Squad(t, "Creation dispatch squad", agent)
	autopilot := dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Creation dispatch transfer", "assignee_type": "squad", "assignee_id": squad, "status": "active", "execution_mode": "create_issue", "created_by_type": "member", "created_by_id": testUserID})
	dbfx.Cleanup(t, `DELETE FROM issue WHERE origin_id=$1`, autopilot)
	dbfx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, autopilot)
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agent)
	ap, err := testHandler.Queries.GetAutopilot(t.Context(), parseUUID(autopilot))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	reached, release := make(chan struct{}, 1), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	svc := *testHandler.AutopilotService
	svc.TxStarter = iterationBeginBarrier{svc.TxStarter, reached, release}
	result := make(chan error, 1)
	go func() {
		_, _, e := svc.DispatchAutopilotManual(ctx, ap, pgtype.UUID{}, nil, parseUUID(testUserID))
		result <- e
	}()
	select {
	case <-reached:
	case err = <-result:
		t.Fatalf("dispatch stopped before owner transaction: %v", err)
	case <-ctx.Done():
		t.Fatal("dispatch did not reach owner transaction")
	}
	testutil.Call(t, testHandler.DeleteSquad, testutil.WithURLParams(newRequest("DELETE", "/api/squads/"+squad, nil), "id", squad, "workspaceId", testWorkspaceID)).Want(204)
	close(release)
	if err = <-result; err != nil {
		t.Fatalf("dispatch should follow committed transfer: %v", err)
	}
	var kind, id string
	var unassociated bool
	dbfx.QueryRow(t, `SELECT assignee_type,assignee_id,current_iteration_id IS NULL FROM issue WHERE origin_id=$1`, autopilot).Scan(&kind, &id, &unassociated)
	if kind != "agent" || id != agent || !unassociated {
		t.Fatalf("created stale squad association: %s %s unassociated=%v", kind, id, unassociated)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND issue_id IN (SELECT id FROM issue WHERE origin_id=$2)`, agent, autopilot); n != 1 {
		t.Fatalf("transferred dispatch must enqueue once: %d tasks", n)
	}
	var events, scope int
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
	dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	if events != 0 || scope != 1 {
		t.Fatalf("unassociated dispatch fabricated facts: events=%d scope=%d", events, scope)
	}
}

func syncCreationIterationCounter(t *testing.T) {
	t.Helper()
	dbfx.Exec(t, `UPDATE workspace SET issue_counter=GREATEST(issue_counter,COALESCE((SELECT max(number) FROM issue WHERE workspace_id=$1),0)) WHERE id=$1`, testWorkspaceID)
}

func TestCreationIterationRevocationBeforeWrite(t *testing.T) {
	for _, owner := range []string{"ordinary", "lifecycle"} {
		t.Run(owner, func(t *testing.T) {
			source := lifecycleAtomicSource(t, "Revoked creation source")
			user := dbfx.User(t, "Revoked creation actor", owner+"-creation-revoked@example.test")
			member := dbfx.Member(t, testWorkspaceID, user, "member")
			dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title='Revoked creation must not commit'`, testWorkspaceID)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			revoker, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer revoker.Rollback(context.Background())
			q := testHandler.Queries.WithTx(revoker)
			if _, err = q.LockWorkspaceForChatSessionCreate(ctx, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(user)}); err != nil {
				t.Fatal(err)
			}
			if err = iteration.LockWorkspace(ctx, revoker, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			h := *testHandler
			var call http.HandlerFunc
			request := newRequest("POST", "/api/issues", map[string]any{"title": "Revoked creation must not commit", "status": "backlog"}).WithContext(ctx)
			call = h.CreateIssue
			if owner == "lifecycle" {
				call = h.CreateLifecycleHandoff
				request = withURLParam(newRequest("POST", "/api/issues/"+source+"/lifecycle-handoffs", map[string]any{"kind": "rca", "route": "maintenance", "cause_state": "unknown", "follow_up_title": "Revoked creation must not commit"}).WithContext(ctx), "id", source)
			}
			request.Header.Set("X-User-ID", user)
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, call, request) }()
			if !waitForWaiterBlockedBy(t, int(revoker.Conn().PgConn().PID()), 3*time.Second) {
				t.Fatal("creator did not wait on revocation transaction")
			}
			if _, err = revoker.Exec(ctx, `DELETE FROM member WHERE id=$1`, member); err != nil {
				t.Fatal(err)
			}
			if err = revoker.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			(<-result).Want(403)
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title='Revoked creation must not commit'`, testWorkspaceID); n != 0 {
				t.Fatalf("revoked actor created %d issues", n)
			}
		})
	}
}

func TestAutopilotCreationRetainsLeaderReferencesUntilCommit(t *testing.T) {
	_, _ = iterationIssueFixture(t)
	syncCreationIterationCounter(t)
	agent := dbfx.Agent(t, "Stable creation leader", testRuntimeID)
	squad := dbfx.Squad(t, "Stable creation squad", agent)
	apID := dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Stable creation references", "assignee_type": "squad", "assignee_id": squad, "status": "active", "execution_mode": "create_issue", "created_by_type": "member", "created_by_id": testUserID})
	dbfx.Cleanup(t, `DELETE FROM issue WHERE origin_id=$1`, apID)
	dbfx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, apID)
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agent)
	ap, err := testHandler.Queries.GetAutopilot(t.Context(), parseUUID(apID))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	barrier := projectAssociationCommitStarter{base: testPool, reached: make(chan struct{}, 1), release: make(chan struct{}), once: &sync.Once{}}
	defer func() {
		select {
		case <-barrier.release:
		default:
			close(barrier.release)
		}
	}()
	svc := *testHandler.AutopilotService
	svc.TxStarter = barrier
	result := make(chan error, 1)
	go func() {
		_, _, e := svc.DispatchAutopilotManual(ctx, ap, pgtype.UUID{}, nil, parseUUID(testUserID))
		result <- e
	}()
	select {
	case <-barrier.reached:
	case err = <-result:
		t.Fatalf("dispatch stopped before commit: %v", err)
	case <-ctx.Done():
		t.Fatal("dispatch did not reach commit")
	}
	for _, row := range []struct{ table, id string }{{"squad", squad}, {"agent", agent}} {
		probe, e := testPool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		_, e = probe.Exec(ctx, "SELECT id FROM "+row.table+" WHERE id=$1 FOR NO KEY UPDATE NOWAIT", row.id)
		_ = probe.Rollback(context.Background())
		var busy *pgconn.PgError
		if !errors.As(e, &busy) || busy.Code != "55P03" {
			t.Errorf("%s reference can change before create commits: %v", row.table, e)
		}
	}
	close(barrier.release)
	if err = <-result; err != nil {
		t.Fatal(err)
	}
}

func TestAutopilotCreationRevocationBeforeWrite(t *testing.T) {
	for _, source := range []string{"manual", "schedule"} {
		t.Run(source, func(t *testing.T) {
			user := dbfx.User(t, "Revoked autopilot creator", source+"-autopilot-revoked@example.test")
			member := dbfx.Member(t, testWorkspaceID, user, "member")
			agent := dbfx.Agent(t, "Revoked autopilot target", testRuntimeID, testutil.Cols{"owner_id": user})
			apID := dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Revoked automation must not create", "assignee_type": "agent", "assignee_id": agent, "status": "active", "execution_mode": "create_issue", "created_by_type": "member", "created_by_id": testUserID})
			trigger := dbfx.Insert(t, "autopilot_trigger", testutil.Cols{"autopilot_id": apID, "kind": "schedule", "enabled": true, "cron_expression": "0 * * * *", "created_by_type": "member", "created_by_id": user})
			dbfx.Cleanup(t, `DELETE FROM issue WHERE origin_id=$1`, apID)
			dbfx.Cleanup(t, `DELETE FROM autopilot_run WHERE autopilot_id=$1`, apID)
			dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agent)
			ap, err := testHandler.Queries.GetAutopilot(t.Context(), parseUUID(apID))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			revoker, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer revoker.Rollback(context.Background())
			q := testHandler.Queries.WithTx(revoker)
			if _, err = q.LockWorkspaceForChatSessionCreate(ctx, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			if err = q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(user)}); err != nil {
				t.Fatal(err)
			}
			if err = iteration.LockWorkspace(ctx, revoker, parseUUID(testWorkspaceID)); err != nil {
				t.Fatal(err)
			}
			type outcome struct {
				run *db.AutopilotRun
				err error
			}
			result := make(chan outcome, 1)
			go func() {
				var run *db.AutopilotRun
				var e error
				if source == "manual" {
					run, _, e = testHandler.AutopilotService.DispatchAutopilotManual(ctx, ap, parseUUID(trigger), nil, parseUUID(user))
				} else {
					run, e = testHandler.AutopilotService.DispatchAutopilot(ctx, ap, parseUUID(trigger), "schedule", nil)
				}
				result <- outcome{run, e}
			}()
			if !waitForWaiterBlockedBy(t, int(revoker.Conn().PgConn().PID()), 3*time.Second) {
				t.Fatal("dispatch did not wait on revocation")
			}
			if _, err = revoker.Exec(ctx, `DELETE FROM member WHERE id=$1`, member); err != nil {
				t.Fatal(err)
			}
			if err = revoker.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			got := <-result
			if got.err != nil || got.run == nil || got.run.Status != "skipped" || got.run.IssueID.Valid {
				t.Errorf("revoked principal must skip without issue: run=%+v err=%v", got.run, got.err)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE origin_id=$1`, apID); n != 0 {
				t.Errorf("revoked principal created %d issues", n)
			}
		})
	}
}
