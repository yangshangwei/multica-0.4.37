package main

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func routeIterationOperationFixture(t *testing.T, fx *testutil.Fixture, workspaceID, actorID, requestID string) iteration.WriteResult {
	t.Helper()
	result := iteration.WriteResult{WorkspaceID: workspaceID, RequestID: requestID, OperationID: uuid.NewString(), Operation: "delete", IterationIDs: []string{uuid.NewString()}, Result: iteration.WriteSummary{Deleted: true, SettingsRevision: 1}, CommittedAt: time.Now().UTC()}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "iteration_operation", testutil.Cols{"id": result.OperationID, "workspace_id": workspaceID, "actor_user_id": actorID, "request_id": requestID, "operation": result.Operation, "payload_hash": "stored-router-intent", "result": string(raw)})
	return result
}

func requestIterationOperationRoute(t *testing.T, workspaceID, requestID, token, claimedUserID string, wantStatus int) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testServer.URL+"/api/workspaces/"+workspaceID+"/iteration-operations/"+requestID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if claimedUserID != "" {
		request.Header.Set("X-User-ID", claimedUserID)
	}
	// Intentionally omit X-Workspace-ID. URL middleware must establish the
	// exact workspace context; the handler must not pick another membership.
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("GET operation in %s: status=%d want=%d body=%s", workspaceID, response.StatusCode, wantStatus, body)
	}
	if wantStatus == http.StatusOK && response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("operation result was cacheable")
	}
	return body
}

func TestIterationOperationRouteBindsURLWorkspaceWithoutHeader(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	otherWorkspace := fx.Workspace(t, "Other iteration operation space", uuid.NewString())
	fx.Member(t, otherWorkspace, testUserID, "member")
	requestID := uuid.NewString()
	first := routeIterationOperationFixture(t, fx, testWorkspaceID, testUserID, requestID)
	second := routeIterationOperationFixture(t, fx, otherWorkspace, testUserID, requestID)
	for _, stored := range []iteration.WriteResult{first, second} {
		raw := requestIterationOperationRoute(t, stored.WorkspaceID, requestID, testToken, "", http.StatusOK)
		var got iteration.WriteResult
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.WorkspaceID != stored.WorkspaceID || got.OperationID != stored.OperationID || !got.Replayed {
			t.Fatalf("URL workspace context selected wrong durable result: got=%+v want=%+v", got, stored)
		}
	}
}

func TestIterationOperationRouteRequiresAuthenticatedActor(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	email := uuid.NewString() + "@example.invalid"
	other := fx.User(t, "Other route operation owner", email)
	fx.Member(t, testWorkspaceID, other, "member")
	stored := routeIterationOperationFixture(t, fx, testWorkspaceID, other, uuid.NewString())
	requestIterationOperationRoute(t, testWorkspaceID, stored.RequestID, "", other, http.StatusUnauthorized)
	requestIterationOperationRoute(t, testWorkspaceID, stored.RequestID, testToken, other, http.StatusNotFound)
	otherToken, err := generateTestJWT(other, email, "Other route operation owner")
	if err != nil {
		t.Fatal(err)
	}
	requestIterationOperationRoute(t, testWorkspaceID, stored.RequestID, otherToken, testUserID, http.StatusOK)
}
