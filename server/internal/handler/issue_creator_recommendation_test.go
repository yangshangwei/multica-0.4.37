package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/llm"
)

func creatorTestHandler(t *testing.T, output func(creatorModelInput) string) (*Handler, *atomic.Int32) {
	t.Helper()
	if testPool == nil || testHandler == nil {
		t.Fatal("creator tests require the isolated database")
	}
	calls := &atomic.Int32{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
			Tools json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid", 500)
			return
		}
		if len(request.Messages) != 2 || len(request.Tools) > 0 {
			t.Error("unexpected LLM request shape")
			http.Error(w, "invalid", 500)
			return
		}
		var input creatorModelInput
		if err := json.Unmarshal([]byte(request.Messages[1].Content), &input); err != nil {
			t.Error(err)
		}
		if len(request.Messages[1].Content) > 128*1024 {
			t.Error("payload budget exceeded")
		}
		if len(input.Candidates) > 40 {
			t.Error("candidate budget exceeded")
		}
		if strings.Contains(request.Messages[1].Content, "DO_NOT_SEND_NAME") || strings.Contains(request.Messages[1].Content, "SECRET_INSTRUCTIONS") {
			t.Error("private metadata sent to LLM")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": output(input)}, "finish_reason": "stop"}}})
	}))
	t.Cleanup(provider.Close)
	retries, _ := llm.Retries(0)
	return &Handler{Queries: testHandler.Queries, LLM: llm.New(llm.Config{BaseURL: provider.URL, APIKey: "test", DefaultModel: "creator-test", MaxRetries: retries})}, calls
}
func creatorOutput(input creatorModelInput) string {
	if len(input.Candidates) == 0 {
		return `{"recommendations":[]}`
	}
	c := input.Candidates[0]
	evidence := string([]rune(c.DescriptionExcerpt)[:min(80, len([]rune(c.DescriptionExcerpt)))])
	raw, _ := json.Marshal(map[string]any{"recommendations": []any{map[string]string{"ref": c.Ref, "evidence": evidence}}})
	return string(raw)
}
func TestRecommendCreatorsReadOnlyGrounded(t *testing.T) {
	h, calls := creatorTestHandler(t, creatorOutput)
	id := dbfx.Agent(t, "DO_NOT_SEND_NAME", handlerTestRuntimeID(t), testutil.Cols{"description": "Prepare technical documentation and examples.", "instructions": "SECRET_INSTRUCTIONS"})
	var before, after int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue q JOIN agent a ON a.id=q.agent_id WHERE a.workspace_id=$1`, testWorkspaceID).Scan(&before)
	var result recommendCreatorsResponse
	testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "Write technical documentation"})).Want(200).JSON(&result)
	if len(result.Recommendations) != 1 || result.Recommendations[0].ActorID != id || result.Recommendations[0].Reason != "Prepare technical documentation and examples." || calls.Load() != 1 {
		t.Fatalf("unexpected recommendation: %+v calls=%d", result, calls.Load())
	}
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue q JOIN agent a ON a.id=q.agent_id WHERE a.workspace_id=$1`, testWorkspaceID).Scan(&after)
	if after != before {
		t.Fatal("recommendation wrote a task")
	}
}
func TestRecommendCreatorsRejectsActorCredentialsAndInvalidInputs(t *testing.T) {
	h, calls := creatorTestHandler(t, creatorOutput)
	for _, header := range []string{"X-Agent-ID", "X-Actor-Source"} {
		req := newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "task"})
		req.Header.Set(header, "task_token")
		testutil.Call(t, h.RecommendIssueCreators, req).Want(403)
	}
	for _, body := range []any{map[string]any{"text": " "}, map[string]any{"text": strings.Repeat("界", 20001)}, map[string]any{"text": "task", "agent_id": "invented"}} {
		testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", body)).Want(400)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input reached provider")
	}
}
func TestRecommendCreatorsAdminCannotInvokePrivateAndSquadUsesOwnDescription(t *testing.T) {
	h, calls := creatorTestHandler(t, creatorOutput)
	other := dbfx.User(t, "creator other", "creator-other@example.test")
	id := dbfx.Agent(t, "DO_NOT_SEND_NAME", handlerTestRuntimeID(t), testutil.Cols{"owner_id": other, "permission_mode": "private", "description": "Find secret problems."})
	dbfx.Squad(t, "private leader squad", id, testutil.Cols{"description": "Find secret problems."})
	var result recommendCreatorsResponse
	testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "secret problems"})).Want(200).JSON(&result)
	if len(result.Recommendations) != 0 || calls.Load() != 0 {
		t.Fatal("workspace administration bypassed private invocation")
	}
	leader := dbfx.Agent(t, "leader", handlerTestRuntimeID(t))
	squad := dbfx.Squad(t, "DO_NOT_SEND_NAME", leader, testutil.Cols{"description": "Coordinate release readiness."})
	testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "release readiness"})).Want(200).JSON(&result)
	if len(result.Recommendations) != 1 || result.Recommendations[0].ActorType != "squad" || result.Recommendations[0].ActorID != squad {
		t.Fatalf("squad identity lost: %+v", result)
	}
}
func TestRecommendCreatorsRevalidatesAfterGeneration(t *testing.T) {
	id := ""
	h, _ := creatorTestHandler(t, func(input creatorModelInput) string {
		dbfx.Exec(t, `UPDATE agent SET archived_at=now() WHERE id=$1`, id)
		return creatorOutput(input)
	})
	id = dbfx.Agent(t, "candidate", handlerTestRuntimeID(t), testutil.Cols{"description": "Diagnose failed tests."})
	var result recommendCreatorsResponse
	testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "Diagnose failed tests"})).Want(200).JSON(&result)
	if len(result.Recommendations) != 0 {
		t.Fatal("archived candidate survived revalidation")
	}
}
func TestRecommendCreatorsRejectsInvalidModelEvidence(t *testing.T) {
	for _, raw := range []string{`no JSON`, `{}`, `{"recommendations":null}`, `{"recommendations":[{"ref":"invented","evidence":"test"}]}`, `{"recommendations":[{"ref":"c1","evidence":"invented ability"}]}`, `{"recommendations":[{"ref":"c1","evidence":"Diagnose"},{"ref":"c1","evidence":"Diagnose"}]}`} {
		t.Run(raw, func(t *testing.T) {
			h, _ := creatorTestHandler(t, func(creatorModelInput) string { return raw })
			dbfx.Agent(t, "candidate", handlerTestRuntimeID(t), testutil.Cols{"description": "Diagnose failed tests."})
			testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "Diagnose failed tests"})).Want(502)
		})
	}
}
func TestCreatorShortlistScansTailAndBoundsExcerpts(t *testing.T) {
	actors := make([]creatorCandidate, 550)
	for i := range actors {
		actors[i] = creatorCandidate{ActorType: "agent", ActorID: fmt.Sprint(i), Description: "unrelated cooking recipes"}
	}
	actors[549].Description = strings.Repeat("ordinary content ", 100) + "迁移验证和退款修复"
	got := shortlistCreators(actors, "退款修复")
	if len(got) != 1 || got[0].ActorID != "549" || !strings.Contains(got[0].Excerpt, "退款修复") || len([]rune(got[0].Excerpt)) > 600 {
		t.Fatalf("tail match lost: %+v", got)
	}
	if got := shortlistCreators(actors, "unmatched unicorn"); len(got) != 0 {
		t.Fatal("filled with arbitrary candidates")
	}
	if got := shortlistCreators(actors[:2], "different language"); len(got) != 2 {
		t.Fatal("small catalog should allow model comparison")
	}
}
func TestLoadedInvocationDecisionPreservesPrincipalRules(t *testing.T) {
	a := db.Agent{OwnerID: parseUUID(testUserID), PermissionMode: "private"}
	targets := []db.AgentInvocationTarget{{TargetType: "workspace"}}
	if !loadedInvocationDecision(a, nil, testUserID, false, false) {
		t.Fatal("owner denied")
	}
	if loadedInvocationDecision(a, targets, "other", true, true) {
		t.Fatal("private admin/internal bypass")
	}
	a.PermissionMode = "public_to"
	if !loadedInvocationDecision(a, targets, "", false, true) {
		t.Fatal("existing internal workspace exception removed")
	}
	if loadedInvocationDecision(a, []db.AgentInvocationTarget{{TargetType: "team"}}, "", false, true) {
		t.Fatal("team admitted internal actor")
	}
}

func TestRecommendCreatorsRejectsChangedDescriptionEvenWhenQuoteRemains(t *testing.T) {
	id := ""
	h, _ := creatorTestHandler(t, func(input creatorModelInput) string {
		dbfx.Exec(t, `UPDATE agent SET description='Do not diagnose failed tests.' WHERE id=$1`, id)
		return creatorOutput(input)
	})
	id = dbfx.Agent(t, "changing description", handlerTestRuntimeID(t), testutil.Cols{"description": "diagnose failed tests."})
	var result recommendCreatorsResponse
	testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "diagnose failed tests"})).Want(200).JSON(&result)
	if len(result.Recommendations) != 0 {
		t.Fatal("changed responsibility was recommended using stale evidence")
	}
}
func TestRecommendCreatorsAdmissionAndUnavailable(t *testing.T) {
	h, calls := creatorTestHandler(t, creatorOutput)
	req := func() *http.Request {
		return newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "task"})
	}
	acquired := 0
	defer func() {
		for acquired > 0 {
			<-descriptionAssistSlots
			acquired--
		}
	}()
	for len(descriptionAssistSlots) < cap(descriptionAssistSlots) {
		descriptionAssistSlots <- struct{}{}
		acquired++
	}
	testutil.Call(t, h.RecommendIssueCreators, req()).Want(429)
	for acquired > 0 {
		<-descriptionAssistSlots
		acquired--
	}
	if calls.Load() != 0 {
		t.Fatal("busy request reached provider")
	}
	h.LLM = nil
	testutil.Call(t, h.RecommendIssueCreators, req()).Want(503)
	for _, tc := range []struct {
		user, ws string
		status   int
	}{{"", testWorkspaceID, 401}, {testUserID, "00000000-0000-0000-0000-000000000001", 404}} {
		r := req()
		r.Header.Set("X-User-ID", tc.user)
		r.Header.Set("X-Workspace-ID", tc.ws)
		testutil.Call(t, h.RecommendIssueCreators, r).Want(tc.status)
	}
}
func TestRecommendCreatorsCancellationReleasesAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, calls := creatorTestHandler(t, func(input creatorModelInput) string { cancel(); return creatorOutput(input) })
	dbfx.Agent(t, "cancel recommendation", handlerTestRuntimeID(t), testutil.Cols{"description": "Diagnose cancellation."})
	before := len(descriptionAssistSlots)
	testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "Diagnose cancellation"}).WithContext(ctx))
	if calls.Load() != 1 || len(descriptionAssistSlots) != before {
		t.Fatal("cancelled recommendation leaked admission")
	}
}
func TestRecommendCreatorsTimeout(t *testing.T) {
	h, _ := creatorTestHandler(t, func(input creatorModelInput) string { time.Sleep(1200 * time.Millisecond); return creatorOutput(input) })
	dbfx.Agent(t, "slow recommendation", handlerTestRuntimeID(t), testutil.Cols{"description": "Diagnose timeouts."})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "Diagnose timeouts"}).WithContext(ctx)).Want(504)
}
func TestRecommendCreatorsSerializedPayloadBudget(t *testing.T) {
	h, _ := creatorTestHandler(t, creatorOutput)
	runtime := handlerTestRuntimeID(t)
	for i := 0; i < 45; i++ {
		dbfx.Agent(t, fmt.Sprintf("budget %d", i), runtime, testutil.Cols{"description": "界界" + strings.Repeat("&", 253)})
	}
	testutil.Call(t, h.RecommendIssueCreators, newRequest("POST", "/api/issues/recommend-creators", map[string]any{"text": "界界" + strings.Repeat("&", 19998)})).Want(200)
}
