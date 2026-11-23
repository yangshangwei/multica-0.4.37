package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationKnowledgeRuntimeHTTPRequiresManagedNegotiation(t *testing.T) {
	h := &Handler{}
	for _, endpoint := range []http.HandlerFunc{h.PrepareCollaborationRuntimeKnowledge, h.AuthorizeCollaborationRuntimeKnowledge} {
		for _, token := range []string{"", "mul_human", "mat_task", "mcb_" + strings.Repeat("a", 64), "mct_" + strings.Repeat("a", 64)} {
			r := newRequest("POST", "/api/collaboration/runtime/runs/"+uuid.NewString()+"/knowledge-context/prepare", map[string]any{"content": "PRIVATE_C4_HTTP"})
			r.Header.Set("Authorization", "Bearer "+token)
			response := testutil.Call(t, endpoint, r).Want(http.StatusForbidden)
			if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Text(), "PRIVATE_C4_HTTP") {
				t.Fatal("private endpoint leaked or cached rejection")
			}
		}
	}
	r := newRequest("GET", "/status", nil)
	r.Header.Set("Authorization", "Bearer mcb_"+strings.Repeat("b", 64))
	testutil.Call(t, h.GetCollaborationRuntimeKnowledgeStatus, r).Want(http.StatusUnauthorized)
}

func knowledgeRuntimeHTTPRequest(run string, body any) *http.Request {
	r := withURLParam(newRequest("POST", "/api/collaboration/runtime/runs/"+run+"/knowledge-context/prepare", body), "runId", run)
	r.Header.Set("Authorization", "Bearer mdt_fixture")
	return r.WithContext(auth.WithPasswordSession(r.Context(), auth.PasswordSession{Kind: "daemon_token", UserID: uuid.NewString(), Version: 1, WorkspaceID: uuid.NewString(), DaemonID: "fixture", BindingID: uuid.NewString(), BindingEpoch: 1}))
}

func TestCollaborationKnowledgeRuntimeHTTPStrictBodiesAndClosedGate(t *testing.T) {
	run := uuid.NewString()
	p := protocol.CollaborationRunKnowledgePrepare{ProtocolVersion: 1, Identity: protocol.CollaborationRunIdentity{RunID: run}, RequestID: uuid.NewString(), HandoffID: uuid.NewString(), InvocationFingerprint: strings.Repeat("a", 64)}
	raw, _ := json.Marshal(p)
	for _, body := range []string{strings.Replace(string(raw), `"protocol_version":1`, `"protocol_version":1,"protocol_version":1`, 1), strings.Replace(string(raw), `"protocol_version":1`, `"protocol_version":1,"originator_user_id":"PRIVATE_C4_HTTP"`, 1)} {
		testutil.Call(t, (&Handler{}).PrepareCollaborationRuntimeKnowledge, knowledgeRuntimeHTTPRequest(run, json.RawMessage(body))).Want(http.StatusBadRequest)
	}
	h := &Handler{Queries: testHandler.Queries, TxStarter: testHandler.TxStarter, TaskService: &service.TaskService{}}
	r := testutil.Call(t, h.PrepareCollaborationRuntimeKnowledge, knowledgeRuntimeHTTPRequest(run, p)).Want(http.StatusConflict)
	if r.Map()["code"] != "knowledge_invocation_unsupported" || r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("closed gate did not refuse before content")
	}
}

func TestCollaborationKnowledgeRuntimeClaimRejectsSpoofedBodyCapability(t *testing.T) {
	run := uuid.NewString()
	r := knowledgeRuntimeHTTPRequest(run, json.RawMessage(`{"protocol_version":3,"knowledge_version":1}`))
	testutil.Call(t, (&Handler{}).ClaimCollaborationRuntimeRun, r).Want(http.StatusBadRequest)
	for _, query := range []string{"knowledge_version=2", "knowledge_version=1&knowledge_version=1"} {
		r = knowledgeRuntimeHTTPRequest(run, protocol.CollaborationRunClaimRequest{ProtocolVersion: 3})
		r.URL.RawQuery = query
		testutil.Call(t, (&Handler{}).ClaimCollaborationRuntimeRun, r).Want(http.StatusBadRequest)
	}
}
