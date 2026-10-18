package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func collaborationCreationHTTPFixture(t *testing.T) (*Handler, *testutil.Fixture, string, string) {
	t.Helper()
	fx := testutil.New(testPool, "", "")
	actor := fx.User(t, "Creation actor", uuid.NewString()+"@example.test")
	recipient := fx.User(t, "Private recipient", uuid.NewString()+"@example.test")
	workspace := fx.Workspace(t, "Creation source", "create-"+uuid.NewString())
	fx.Member(t, workspace, actor, "owner")
	fx.UserID, fx.WorkspaceID = actor, workspace
	fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
	fx.Cleanup(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
	fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, workspace)
	for _, table := range []string{"issue_collaboration", "issue_participant", "collaboration_session", "collaboration_event", "collaboration_notification"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, workspace)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1`, workspace)
	fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{actor, recipient})
	h := *testHandler
	return &h, fx, actor, recipient
}

func collaborationCreationHTTPRequest(method, source, key, actor string, body any, result bool) *http.Request {
	path := "/api/collaboration/issues"
	if method == http.MethodGet {
		path = "/api/collaboration/creations/" + key
		if result {
			path += "/result"
		}
		path += "?source_workspace_id=" + source
	}
	r := withURLParam(testutil.JSONRequest(method, path, body), "requestId", key)
	r.Header.Set("Idempotency-Key", key)
	r.Header.Set("X-Workspace-ID", "unrelated-ambient-workspace")
	if actor != "" {
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
	}
	return r
}

func TestCollaborationCreationResultHTTPRecoversOnlyOwnSettledRejection(t *testing.T) {
	h, fx, actor, recipient := collaborationCreationHTTPFixture(t)
	key := uuid.NewString()
	body := map[string]any{"source_workspace_id": fx.WorkspaceID, "handler_user_id": recipient, "title": "private-title-sentinel", "description": "private-body-sentinel"}
	fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, recipient)
	post := func(key string, body any) *testutil.Response {
		t.Helper()
		return testutil.Call(t, h.CreateCollaborationIssue, collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, key, actor, body, false))
	}
	refused := post(key, body).Want(http.StatusForbidden)
	if refused.Map()["code"] != "account_unavailable" {
		t.Fatal("recipient failure did not preserve the existing error contract")
	}
	request := collaborationCreationHTTPRequest(http.MethodGet, fx.WorkspaceID, key, actor, nil, true)
	recovered := testutil.Call(t, h.GetCollaborationCreationResult, request).Want(http.StatusOK)
	result := recovered.Map()
	if len(result) != 5 || result["request_id"] != key || result["source_workspace_id"] != fx.WorkspaceID || result["actor_user_id"] != actor || result["outcome"] != "rejected" || result["code"] != "account_unavailable" {
		t.Fatalf("creation result did not preserve the narrow correlation contract: %v", result)
	}
	if recovered.Header().Get("Cache-Control") != "no-store" || recovered.Header().Get("Referrer-Policy") != "no-referrer" || request.Header.Get("X-Workspace-ID") != "" || strings.Contains(recovered.Text(), "private-") || strings.Contains(recovered.Text(), recipient) {
		t.Fatal("creation result retained ambient scope or disclosed private receipt data")
	}
	legacy := collaborationCreationHTTPRequest(http.MethodGet, fx.WorkspaceID, key, actor, nil, false)
	testutil.Call(t, h.GetCollaborationCreation, legacy).Want(http.StatusNotFound)
	other := fx.User(t, "Unrelated caller", uuid.NewString()+"@example.test")
	for _, query := range []struct{ actor, source, request string }{
		{other, fx.WorkspaceID, key}, {actor, uuid.NewString(), key}, {actor, fx.WorkspaceID, uuid.NewString()},
	} {
		response := testutil.Call(t, h.GetCollaborationCreationResult, collaborationCreationHTTPRequest(http.MethodGet, query.source, query.request, query.actor, nil, true)).Want(http.StatusNotFound)
		if response.Map()["code"] != "operation_not_found" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("foreign/missing own-operation lookup changed the private boundary")
		}
	}
	corrected := map[string]any{"source_workspace_id": fx.WorkspaceID, "title": body["title"], "description": body["description"]}
	newKey := uuid.NewString()
	created := post(newKey, corrected).Want(http.StatusCreated).Map()
	fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, recipient)
	post(key, body).Want(http.StatusForbidden)
	if post(newKey, corrected).Want(http.StatusCreated).Map()["id"] != created["id"] || fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, fx.WorkspaceID) != 1 {
		t.Fatal("old refusal or new success replay duplicated creation")
	}
	good := testutil.Call(t, h.GetCollaborationCreation, collaborationCreationHTTPRequest(http.MethodGet, fx.WorkspaceID, newKey, actor, nil, false)).Want(http.StatusOK)
	if good.Map()["id"] != created["id"] {
		t.Fatal("existing successful creation recovery response changed")
	}
	// The stored result is not a passthrough: corrupted private values are never
	// usable proof or reflected in a dependency-error response.
	fx.Exec(t, `UPDATE collaboration_operation SET result='{"outcome":"private-result-sentinel"}'::jsonb WHERE scope_kind='create' AND scope_id=$1 AND request_id=$2`, fx.WorkspaceID, key)
	corrupt := testutil.Call(t, h.GetCollaborationCreationResult, collaborationCreationHTTPRequest(http.MethodGet, fx.WorkspaceID, key, actor, nil, true)).Want(http.StatusServiceUnavailable)
	if corrupt.Map()["code"] != "collaboration_unavailable" || strings.Contains(corrupt.Text(), "sentinel") || strings.Contains(corrupt.Text(), "outcome") {
		t.Fatal("corrupted own result was disclosed or reported as settled")
	}
}

func TestCollaborationCreationResultHTTPRejectsUntrustedAndMalformedRequests(t *testing.T) {
	h, fx, actor, _ := collaborationCreationHTTPFixture(t)
	key := uuid.NewString()
	for _, principal := range []auth.AuthenticatedPrincipal{
		{UserID: actor, Kind: auth.CredentialTask}, {UserID: actor, Kind: auth.CredentialCloud},
		{UserID: actor, Kind: auth.CredentialJWT, Restricted: true}, {UserID: actor, Kind: auth.CredentialJWT, AgentActor: true}, {},
	} {
		r := collaborationCreationHTTPRequest(http.MethodGet, fx.WorkspaceID, key, "", nil, true)
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), principal))
		testutil.Call(t, h.GetCollaborationCreationResult, r).Want(http.StatusNotFound)
	}
	for _, query := range []string{"", "source_workspace_id=invalid", "source_workspace_id=" + fx.WorkspaceID + "&source_workspace_id=" + fx.WorkspaceID} {
		r := collaborationCreationHTTPRequest(http.MethodGet, fx.WorkspaceID, key, actor, nil, true)
		r.URL.RawQuery = query
		testutil.Call(t, h.GetCollaborationCreationResult, r).Want(http.StatusBadRequest)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, fx.WorkspaceID) != 0 {
		t.Fatal("read/invalid requests created operation state")
	}
}

type collaborationCreationHTTPCommitStarter struct {
	service.TxStarter
	commit bool
}

type collaborationCreationHTTPCommitTx struct {
	pgx.Tx
	commit bool
}

func (s collaborationCreationHTTPCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return collaborationCreationHTTPCommitTx{Tx: tx, commit: s.commit}, nil
}

func (tx collaborationCreationHTTPCommitTx) Commit(ctx context.Context) error {
	if tx.commit {
		if err := tx.Tx.Commit(ctx); err != nil {
			return err
		}
	}
	return errors.New("private-commit-error-sentinel")
}

func TestCollaborationCreationResultHTTPCommitUncertaintyDoesNotReportRejection(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "rolled_back", true: "acknowledgement_lost"}[committed], func(t *testing.T) {
			h, fx, actor, recipient := collaborationCreationHTTPFixture(t)
			fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, recipient)
			key := uuid.NewString()
			faulty := *h
			faulty.TxStarter = collaborationCreationHTTPCommitStarter{TxStarter: h.TxStarter, commit: committed}
			body := map[string]any{"source_workspace_id": fx.WorkspaceID, "handler_user_id": recipient, "title": "Uncertain creation"}
			unknown := testutil.Call(t, faulty.CreateCollaborationIssue, collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, key, actor, body, false)).Want(http.StatusServiceUnavailable)
			if unknown.Map()["code"] != "collaboration_unavailable" || strings.Contains(unknown.Text(), "sentinel") || strings.Contains(unknown.Text(), "account_unavailable") || strings.Contains(unknown.Text(), "rejected") {
				t.Fatal("unknown creation commit leaked an error or claimed settled rejection")
			}
			response := testutil.Call(t, h.GetCollaborationCreationResult, collaborationCreationHTTPRequest(http.MethodGet, fx.WorkspaceID, key, actor, nil, true))
			if committed {
				result := response.Want(http.StatusOK).Map()
				if result["outcome"] != "rejected" || result["code"] != "account_unavailable" {
					t.Fatal("committed refusal was not recovered by its original key")
				}
			} else if response.Want(http.StatusNotFound).Map()["code"] != "operation_not_found" {
				t.Fatal("uncommitted refusal appeared durable")
			}
		})
	}
}
