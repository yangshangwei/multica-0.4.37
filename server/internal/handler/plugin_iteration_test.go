package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestPatchPluginIssueIterationFacts(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	installationID := installPluginForAction(t, []string{"issues:read", "issues:write"})
	token, err := testHandler.PluginService.IssueInstallToken(context.Background(), parseUUID(installationID))
	if err != nil {
		t.Fatal(err)
	}
	req := pluginInstallTokenRequest(http.MethodPatch, "/v1/issues/"+issueID, token, map[string]any{"title": "Plugin fact"}, map[string]string{"issue_ref": issueID})
	req.Header.Set("If-Match", `W/"1"`)
	testutil.Call(t, testHandler.PatchPluginIssue, req).Want(http.StatusOK)
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	if count != 1 {
		t.Fatalf("plugin title PATCH must record one event, got %d", count)
	}
}

func TestPatchPluginIssueIterationActorAndNoOp(t *testing.T) {
	for _, kind := range []string{"member", "plugin"} {
		t.Run(kind, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			installationID := installPluginForAction(t, []string{"issues:read", "issues:write"})
			token, err := testHandler.PluginService.IssueInstallToken(context.Background(), parseUUID(installationID))
			if err != nil {
				t.Fatal(err)
			}
			request := func(body map[string]any) *http.Request {
				if kind == "plugin" {
					return pluginInstallTokenRequest(http.MethodPatch, "/v1/issues/"+issueID, token, body, map[string]string{"issue_ref": issueID})
				}
				return pluginActionRequest(http.MethodPatch, "/v1/issues/"+issueID, installationID, body, map[string]string{"issue_ref": issueID})
			}
			testutil.Call(t, testHandler.PatchPluginIssue, request(map[string]any{"title": "Original commitment"})).Want(200)
			testutil.Call(t, testHandler.PatchPluginIssue, request(map[string]any{"description": "Only content"})).Want(200)
			var events, scope, revision, rollover int64
			var pointer string
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
			if events != 0 {
				t.Fatalf("non-factual PATCH manufactured %d events", events)
			}
			req := request(map[string]any{"title": "Attribution"})
			req.Header.Set("If-Match", `W/"2"`)
			response := testutil.Call(t, testHandler.PatchPluginIssue, req).Want(200)
			if response.Header().Get("ETag") != `W/"3"` {
				t.Fatalf("ETag=%s", response.Header().Get("ETag"))
			}
			var actor []byte
			dbfx.QueryRow(t, `SELECT actor FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&actor)
			var identity map[string]any
			if err = json.Unmarshal(actor, &identity); err != nil {
				t.Fatal(err)
			}
			if identity["type"] != kind {
				t.Fatalf("actor=%s", actor)
			}
			if kind == "plugin" && (identity["id"] != installationID || identity["user_id"] != nil) {
				t.Fatalf("plugin impersonated user: %s", actor)
			}
			if kind == "member" && (identity["id"] != testUserID || identity["via_plugin_id"] != installationID) {
				t.Fatalf("member provenance missing: %s", actor)
			}
			dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
			dbfx.QueryRow(t, `SELECT revision,current_iteration_id,iteration_rollover_count FROM issue WHERE id=$1`, issueID).Scan(&revision, &pointer, &rollover)
			if scope != 2 || revision != 3 || pointer != iterationID || rollover != 2 {
				t.Fatalf("PATCH lost atomic compatibility scope=%d revision=%d pointer=%s rollover=%d", scope, revision, pointer, rollover)
			}
			stale := request(map[string]any{"title": "Stale"})
			stale.Header.Set("If-Match", `W/"2"`)
			testutil.Call(t, testHandler.PatchPluginIssue, stale).Want(409)
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
			if events != 1 {
				t.Fatalf("CAS conflict appended %d events", events)
			}
		})
	}
}

func TestPatchPluginIssueIterationEventFailure(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	installationID := installPluginForAction(t, []string{"issues:write"})
	h := *testHandler
	svc := *h.IssueService
	svc.TxStarter = iterationEventFailureStarter{inner: svc.TxStarter}
	h.IssueService = &svc
	testutil.Call(t, h.PatchPluginIssue, pluginActionRequest("PATCH", "/v1/issues/"+issueID, installationID, map[string]any{"title": "Must roll back", "description": "also rollback"}, map[string]string{"issue_ref": issueID})).Want(500)
	var title string
	var revision, scope, events int64
	var description *string
	dbfx.QueryRow(t, `SELECT title,revision,description FROM issue WHERE id=$1`, issueID).Scan(&title, &revision, &description)
	dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
	if title != "Original commitment" || revision != 1 || scope != 1 || events != 0 || description != nil {
		t.Fatalf("partial PATCH commit title=%s revision=%d scope=%d events=%d description=%v", title, revision, scope, events, description)
	}
}

func TestPatchPluginIssueIterationAuthorizationChanges(t *testing.T) {
	for _, change := range []string{"disabled", "scope", "token", "uninstalled", "member"} {
		t.Run(change, func(t *testing.T) {
			issueID, iterationID := iterationIssueFixture(t)
			installationID := installPluginForAction(t, []string{"issues:write"})
			token, err := testHandler.PluginService.IssueInstallToken(context.Background(), parseUUID(installationID))
			if err != nil {
				t.Fatal(err)
			}
			request := pluginInstallTokenRequest("PATCH", "/v1/issues/"+issueID, token, map[string]any{"title": "Unauthorized"}, map[string]string{"issue_ref": issueID})
			if change == "member" {
				request = pluginActionRequest("PATCH", "/v1/issues/"+issueID, installationID, map[string]any{"title": "Unauthorized"}, map[string]string{"issue_ref": issueID})
			}
			reached, release := make(chan struct{}, 1), make(chan struct{})
			h := *testHandler
			svc := *h.IssueService
			svc.TxStarter = iterationBeginBarrier{svc.TxStarter, reached, release}
			h.IssueService = &svc
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, h.PatchPluginIssue, request) }()
			waitIterationBarrier(t, reached)
			switch change {
			case "disabled":
				dbfx.Exec(t, `UPDATE plugin_installation SET enabled=false WHERE id=$1`, installationID)
			case "scope":
				dbfx.Exec(t, `UPDATE plugin_installation SET granted_scopes='[]' WHERE id=$1`, installationID)
			case "token":
				if _, err = testHandler.PluginService.IssueInstallToken(context.Background(), parseUUID(installationID)); err != nil {
					t.Fatal(err)
				}
			case "uninstalled":
				dbfx.Exec(t, `DELETE FROM plugin_installation WHERE id=$1`, installationID)
			case "member":
				var member []byte
				dbfx.QueryRow(t, `SELECT row_to_json(member) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID).Scan(&member)
				dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID)
				t.Cleanup(func() {
					dbfx.Exec(t, `INSERT INTO member SELECT * FROM json_populate_record(NULL::member,$1::json)`, string(member))
				})
			}
			close(release)
			status := 403
			if change == "uninstalled" || change == "member" {
				status = 404
			}
			(<-result).Want(status)
			var title string
			var events int64
			dbfx.QueryRow(t, `SELECT title FROM issue WHERE id=$1`, issueID).Scan(&title)
			dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
			if title != "Original commitment" || events != 0 {
				t.Fatalf("revoked PATCH committed title=%q events=%d", title, events)
			}
		})
	}
}

func TestPatchPluginIssueIterationRetainsAuthorizationLocks(t *testing.T) {
	for _, resource := range []string{"installation", "member"} {
		t.Run(resource, func(t *testing.T) {
			issueID, _ := iterationIssueFixture(t)
			installationID := installPluginForAction(t, []string{"issues:write"})
			reached, release := make(chan struct{}, 1), make(chan struct{})
			h := *testHandler
			svc := *h.IssueService
			svc.TxStarter = projectAssociationCommitStarter{base: svc.TxStarter, reached: reached, release: release, once: &sync.Once{}}
			h.IssueService = &svc
			result := make(chan *testutil.Response, 1)
			go func() {
				result <- testutil.Call(t, h.PatchPluginIssue, pluginActionRequest("PATCH", "/v1/issues/"+issueID, installationID, map[string]any{"title": "Authorized commit"}, map[string]string{"issue_ref": issueID}))
			}()
			waitIterationBarrier(t, reached)
			tx, err := testPool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(context.Background(), `SET LOCAL lock_timeout='75ms'`); err != nil {
				t.Fatal(err)
			}
			if resource == "installation" {
				_, err = tx.Exec(context.Background(), `UPDATE plugin_installation SET granted_scopes='[]' WHERE id=$1`, installationID)
			} else {
				_, err = tx.Exec(context.Background(), `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
				t.Fatalf("%s revocation passed uncommitted PATCH: %v", resource, err)
			}
			if err = tx.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
			close(release)
			(<-result).Want(200)
		})
	}
}

func TestPatchPluginIssueIterationCallbackRecheckedAfterFence(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	installationID := installPluginForAction(t, []string{"issues:write"})
	installation, err := testHandler.Queries.GetPluginInstallation(context.Background(), parseUUID(installationID))
	if err != nil {
		t.Fatal(err)
	}
	h := *testHandler
	plugins := *h.PluginService
	plugins.Callbacks = service.NewCallbackTokens()
	h.PluginService = &plugins
	token, err := plugins.Callbacks.Issue(context.Background(), service.HookInvocation{Installation: installation, Actor: service.HookActor{Type: "plugin", ID: installation.ID}, IssueID: parseUUID(issueID)})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err = iteration.LockWorkspace(context.Background(), tx, parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{}, 1)
	svc := *h.IssueService
	svc.TxStarter = iterationFenceBarrierStarter{inner: svc.TxStarter, reached: reached}
	h.IssueService = &svc
	result := make(chan *testutil.Response, 1)
	go func() {
		result <- testutil.Call(t, h.PatchPluginIssue, pluginInstallTokenRequest("PATCH", "/v1/issues/"+issueID, token, map[string]any{"title": "Expired authorization"}, map[string]string{"issue_ref": issueID}))
	}()
	waitIterationBarrier(t, reached)
	plugins.Callbacks.Revoke(token)
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	(<-result).Want(403)
	var events int
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
	if events != 0 {
		t.Fatal("revoked callback wrote after fence wait")
	}
}

func TestPatchPluginIssueIterationConcurrentRevision(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	installationID := installPluginForAction(t, []string{"issues:write"})
	reached, release := make(chan struct{}, 2), make(chan struct{})
	h := *testHandler
	svc := *h.IssueService
	svc.TxStarter = iterationBeginBarrier{svc.TxStarter, reached, release}
	h.IssueService = &svc
	results := make(chan *testutil.Response, 2)
	for _, title := range []string{"Plugin winner A", "Plugin winner B"} {
		go func(title string) {
			req := pluginActionRequest("PATCH", "/v1/issues/"+issueID, installationID, map[string]any{"title": title}, map[string]string{"issue_ref": issueID})
			req.Header.Set("If-Match", `W/"1"`)
			results <- testutil.Call(t, h.PatchPluginIssue, req)
		}(title)
	}
	waitIterationBarrier(t, reached)
	waitIterationBarrier(t, reached)
	close(release)
	a, b := <-results, <-results
	if !((a.Code == 200 && b.Code == 409) || (a.Code == 409 && b.Code == 200)) {
		t.Fatalf("PATCH CAS status=%d/%d body=%s/%s", a.Code, b.Code, a.Body, b.Body)
	}
	var events, scope, revision int64
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
	dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, issueID).Scan(&revision)
	if events != 1 || scope != 2 || revision != 2 {
		t.Fatalf("CAS loser leaked events=%d scope=%d issue=%d", events, scope, revision)
	}
}

func TestPatchPluginIssueIterationDeleteBeforeTransactionPreservesConflict(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	installationID := installPluginForAction(t, []string{"issues:write"})
	reached, release := make(chan struct{}, 1), make(chan struct{})
	h := *testHandler
	svc := *h.IssueService
	svc.TxStarter = iterationBeginBarrier{svc.TxStarter, reached, release}
	h.IssueService = &svc
	request := pluginActionRequest("PATCH", "/v1/issues/"+issueID, installationID, map[string]any{"title": "Deleted issue"}, map[string]string{"issue_ref": issueID})
	request.Header.Set("If-Match", `W/"1"`)
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, h.PatchPluginIssue, request) }()
	waitIterationBarrier(t, reached)
	dbfx.Exec(t, `DELETE FROM issue WHERE id=$1`, issueID)
	close(release)
	(<-result).Want(409)
	var events int
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&events)
	if events != 0 {
		t.Fatal("deleted conflict manufactured iteration facts")
	}
}
