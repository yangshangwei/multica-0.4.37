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

func TestCollaborationRuntimeManagedBoundaryRejectsBroadCredentials(t *testing.T) {
	for _, token := range []string{"", "mul_human", "mat_ordinary_task", "mcn_cloud", "eyJ.human.jwt", "mct_" + strings.Repeat("a", 64)} {
		called := false
		h := RequireCollaborationManagedBearer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
		r := newRequest(http.MethodGet, "/api/collaboration/runtime/runs?runtime_id="+uuid.NewString(), nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.AddCookie(&http.Cookie{Name: "multica_token", Value: "ignored_human_cookie"})
		response := testutil.Call(t, h.ServeHTTP, r).Want(http.StatusUnauthorized)
		if called || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("runtime boundary delegated a broad credential or cached its refusal")
		}
	}
}

func TestCollaborationRuntimeRecordingHTTPRejectsMalformedPrivateTelemetry(t *testing.T) {
	run := uuid.NewString()
	valid := `{"identity":{"run_id":"` + run + `"},"event_id":"` + uuid.NewString() + `","attempt_seq":"1","attempt_nonce":"` + uuid.NewString() + `","outcome":"success","body":"PRIVATE_SENTINEL","record":{"version":1,"completeness":"complete","messages":[{"kind":"message","role":"assistant","body":"recorded"}],"usage":{"input_tokens":"0"}}}`
	for _, raw := range []string{
		strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(valid, `"version":1`, `"version":1,"credential":"PRIVATE_SENTINEL"`, 1),
		strings.Replace(valid, `"body":"recorded"`, `"body":"recorded","credential":"PRIVATE_SENTINEL"`, 1),
		strings.Replace(valid, `"input_tokens":"0"`, `"input_tokens":"-1"`, 1),
		strings.Replace(valid, `"body":"recorded"`, `"body":null`, 1),
		strings.Replace(valid, `"body":"recorded"`, `"body":"`+strings.Repeat("x", protocol.CollaborationRecordTextLimit+1)+`"`, 1),
	} {
		r := withURLParam(newRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+run+"/outcome", json.RawMessage(raw)), "runId", run)
		r.Header.Set("Authorization", "Bearer "+service.CollaborationBusinessTokenPrefix+strings.Repeat("a", 64))
		response := testutil.Call(t, (&Handler{}).RecordCollaborationRuntimeOutcome, r).Want(http.StatusBadRequest)
		if strings.Contains(response.Body.String(), "PRIVATE_SENTINEL") || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("invalid recording leaked private payload or was cacheable")
		}
	}
	r := withURLParam(newRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+run+"/outcome", json.RawMessage(`{"body":"`+strings.Repeat("x", protocol.CollaborationOutcomeJSONLimit)+`"}`)), "runId", run)
	r.Header.Set("Authorization", "Bearer "+service.CollaborationBusinessTokenPrefix+strings.Repeat("a", 64))
	testutil.Call(t, (&Handler{}).RecordCollaborationRuntimeOutcome, r).Want(http.StatusRequestEntityTooLarge)
}

func TestCollaborationRuntimeHandlersRequireActualManagedSource(t *testing.T) {
	bare := &Handler{}
	for _, source := range []auth.PasswordSession{
		{UserID: uuid.NewString(), Kind: "jwt", Version: 1},
		{UserID: uuid.NewString(), Kind: "pat", Version: 1},
		{UserID: uuid.NewString(), Kind: "daemon_token", Version: 1, WorkspaceID: uuid.NewString(), DaemonID: "fixture"},
		{UserID: uuid.NewString(), Kind: "daemon_token", Version: 1, WorkspaceID: uuid.NewString(), DaemonID: "fixture", BindingID: uuid.NewString()},
	} {
		for _, h := range []http.HandlerFunc{bare.ListCollaborationRuntimeRuns, bare.ClaimCollaborationRuntimeRun, bare.PermitCollaborationRuntimeRun} {
			r := newRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+uuid.NewString()+"/claim", map[string]any{})
			r.Header.Set("Authorization", "Bearer mdt_synthetic")
			r = r.WithContext(auth.WithPasswordSession(r.Context(), source))
			response := testutil.Call(t, h, r).Want(http.StatusForbidden)
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("unbound source rejection was cacheable")
			}
		}
	}
}

func TestCollaborationRuntimeNarrowHandlersNeverFallBackToOtherPurposes(t *testing.T) {
	bare := &Handler{}
	for _, tc := range []struct {
		handler http.HandlerFunc
		tokens  []string
	}{
		{bare.AcceptCollaborationRuntimeProof, []string{"", "mul_human", "mdt_daemon", "mat_task", "eyJ.user.jwt", service.CollaborationBusinessTokenPrefix + strings.Repeat("a", 64)}},
		{bare.GetCollaborationRuntimeStopIntent, []string{"", "mul_human", "mdt_daemon", service.CollaborationBusinessTokenPrefix + strings.Repeat("a", 64)}},
		{bare.RecordCollaborationRuntimeOutcome, []string{"", "mul_human", "mdt_daemon", service.CollaborationTerminationTokenPrefix + strings.Repeat("a", 64)}},
		{bare.GetCollaborationRuntimeContext, []string{"", "mul_human", "mdt_daemon", service.CollaborationTerminationTokenPrefix + strings.Repeat("a", 64)}},
		{bare.CreateCollaborationRuntimeComment, []string{"", "mul_human", "mdt_daemon", service.CollaborationTerminationTokenPrefix + strings.Repeat("a", 64)}},
		{bare.RecordCollaborationRuntimeSquadActivity, []string{"", "mul_human", "mdt_daemon", service.CollaborationTerminationTokenPrefix + strings.Repeat("a", 64)}},
	} {
		for _, token := range tc.tokens {
			r := newRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+uuid.NewString()+"/proof", map[string]any{})
			r.Header.Set("Authorization", "Bearer "+token)
			r.AddCookie(&http.Cookie{Name: "multica_token", Value: "ignored_cookie"})
			response := testutil.Call(t, tc.handler, r).Want(http.StatusUnauthorized)
			if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), token) && token != "" {
				t.Fatal("purpose refusal cached or echoed a credential")
			}
		}
	}
}

func TestCollaborationRuntimeProofBodyIsBoundedStrictAndMatchesPath(t *testing.T) {
	bare := &Handler{}
	run := uuid.NewString()
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"identity":{"run_id":"` + uuid.NewString() + `"}}`, http.StatusBadRequest},
		{`{"identity":{"run_id":"` + run + `","run_id":"` + run + `"}}`, http.StatusBadRequest},
		{`{"identity":{"run_id":"` + run + `"},"body":"PRIVATE_SENTINEL"}`, http.StatusBadRequest},
		{`{"identity":{"run_id":"` + run + `"}} {}`, http.StatusBadRequest},
		{`{"body":"` + strings.Repeat("x", 97<<10) + `"}`, http.StatusRequestEntityTooLarge},
	} {
		r := withURLParam(newRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+run+"/proof", json.RawMessage(tc.body)), "runId", run)
		r.Header.Set("Authorization", "Bearer "+service.CollaborationTerminationTokenPrefix+strings.Repeat("a", 64))
		response := testutil.Call(t, bare.AcceptCollaborationRuntimeProof, r).Want(tc.status)
		if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), "PRIVATE_SENTINEL") {
			t.Fatal("invalid proof body escaped the private boundary")
		}
	}
}
