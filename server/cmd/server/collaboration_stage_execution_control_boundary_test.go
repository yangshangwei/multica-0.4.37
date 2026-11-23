package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestStageExecutionRouterPrivateControlAndActualArtifact(t *testing.T) {
	f := newControlledArtifactRouterFixture(t, "legacy")
	api, h, controller, stop := stageExecutionRouter(t, false)
	defer stop()
	f.router = api.Config.Handler
	if h.StageExecutionSources == nil {
		t.Fatal("test-only source injection was not forwarded")
	}
	health, err := http.Get(api.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	health.Body.Close()
	if health.StatusCode != 200 {
		t.Fatal("actual router listener unavailable")
	}
	created := f.create(t, f.taskToken)
	base := "/api/agent-approvals/" + created.Approval.ID
	f.call(t, f.token, "POST", base+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": created.SpecDigest}).Want(200)
	var packaged controlledArtifactRouterResponse
	f.call(t, f.taskToken, "POST", base+"/artifact-package/execution", map[string]string{"expected_spec_digest": created.SpecDigest}).Want(200).JSON(&packaged)
	if packaged.Status != "succeeded" {
		t.Fatal("actual artifact API failed")
	}
	var digest string
	f.fx.QueryRow(t, `SELECT output_artifact_digest FROM agent_artifact_execution WHERE approval_id=$1`, packaged.Approval.ID).Scan(&digest)
	(&deliveryEvidenceRouterFixture{controlledArtifactRouterFixture: f, approval: packaged, body: map[string]any{"expected_artifact_digest": digest}}).archive(t)
	deployment := f.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": testWorkspaceID, "installation_id": int64(780001), "account_login": "controlled-deployment", "account_type": "User"})
	metric := f.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": testWorkspaceID, "installation_id": int64(780002), "account_login": "controlled-metric", "account_type": "User"})
	configure := map[string]any{"request_id": uuid.NewString(), "action": "source.configure", "workspace_id": testWorkspaceID, "deployment_integration_id": deployment, "metric_integration_id": metric, "environment": "synthetic", "remote_instance": "private-router", "remote_tenant": "tenant", "remote_target": uuid.NewString(), "conditional_version": "one", "current_artifact_digest": digest, "query_id": "controlled-query", "query_digest": strings.Repeat("a", 64), "rule_id": "controlled-rule", "rule_digest": strings.Repeat("b", 64), "metric_freshness_seconds": 60}
	raw, _ := json.Marshal(configure)
	reply := controller.handle(t.Context(), raw)
	if !reply.OK || reply.Calls.Count != 0 {
		t.Fatalf("private source configure: %+v", reply)
	}
	payload, _ := json.Marshal(reply)
	for _, private := range []string{deployment, metric, "authentication", "proof", "private-router"} {
		if strings.Contains(string(payload), private) {
			t.Fatal("private control reply leaked source input")
		}
	}
	for _, path := range []string{"/api/collaboration/issues/" + uuid.NewString() + "/stage/executions", "/api/collaboration/stage/control"} {
		response := f.call(t, f.token, "POST", path, map[string]any{"action": "source.configure", "verified": true})
		if response.Code != 404 && response.Code != 405 {
			t.Fatalf("public producer/fixture route exists: %d", response.Code)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_execution_intent WHERE workspace_id=$1`, testWorkspaceID) != 0 {
		t.Fatal("source configuration or packaging seeded deployment intent")
	}
}

func TestStageExecutionRouterControlStrictEnvelope(t *testing.T) {
	source, err := newStageControlledSource(testPool)
	if err != nil {
		t.Fatal(err)
	}
	controller := &stageExecutionController{source: source}
	id := uuid.NewString()
	for _, raw := range []string{
		`{"request_id":"` + id + `","action":"shutdown","action":"shutdown"}`,
		`{"request_id":"` + id + `","Action":"shutdown"}`,
		`{"request_id":"` + id + `","action":"shutdown","verified":true}`,
		`{"request_id":"` + id + `","action":"execution.prepare","candidate":{"Candidate_ID":"` + id + `","version":"1","content_digest":"` + strings.Repeat("a", 64) + `"},"issue_id":"` + id + `","approval_decision_id":"` + id + `","target_digest":"` + strings.Repeat("b", 64) + `"}`,
		`{"request_id":"` + id + `","action":"unknown","secret":"PRIVATE_SENTINEL"}`,
		strings.Repeat(" ", 64<<10) + `{}`,
	} {
		reply := controller.handle(context.Background(), []byte(raw))
		encoded, _ := json.Marshal(reply)
		if reply.OK || reply.Code != "invalid_stage_control_request" || reply.Calls.Count != 0 || strings.Contains(string(encoded), "PRIVATE_SENTINEL") {
			t.Fatalf("unsafe private control input accepted: %s", encoded)
		}
	}
	shutdown := controller.handle(context.Background(), []byte(`{"request_id":"`+id+`","action":"shutdown"}`))
	if !shutdown.OK || shutdown.RequestID != id {
		t.Fatal("mechanical shutdown reply changed")
	}
}

func TestStageExecutionRouterProductionConstructorHasNoSource(t *testing.T) {
	_, h := NewRouterWithOptions(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	if h.StageExecutionSources != nil {
		t.Fatal("production constructor installed a simulated source")
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	p := service.StageExecutionRequest{WorkspaceID: util.MustParseUUID(testWorkspaceID), IssueID: util.MustParseUUID(uuid.NewString()), RequestID: uuid.NewString(), ApprovalDecisionID: util.MustParseUUID(uuid.NewString()), Candidate: service.CollaborationStageCandidateRef{CandidateID: util.MustParseUUID(uuid.NewString()), Version: 1, ContentDigest: strings.Repeat("a", 64)}, TargetDigest: strings.Repeat("b", 64)}
	if _, err := svc.PrepareStageExecution(context.Background(), p, service.StageExecutionProof{}); err == nil || err.Error() != "stage_execution_unavailable" {
		t.Fatalf("default source admitted execution: %v", err)
	}
}

func TestStageExecutionRouterCapturedNativeObservationIsImmutable(t *testing.T) {
	source, err := newStageControlledSource(testPool)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	before := strings.Repeat("a", 64)
	after := strings.Repeat("b", 64)
	config := stageControlConfiguration{target: service.StageExecutionTarget{WorkspaceID: testWorkspaceID, Environment: "synthetic", Source: service.StageIntegrationIdentity{Adapter: "controlled-stage-deployment", AdapterVersion: "1", IntegrationID: id, TrustRevision: "1", ConfigRevision: "1"}, ConcurrencyDomain: "controlled-stage-provider", RemoteInstance: "instance", RemoteTenant: "tenant", RemoteTarget: "target", SupportsConditional: true, SupportsOriginalQuery: true, ConditionalVersion: "one", CurrentArtifactDigest: &before}}
	source.configured = true
	source.current = config
	source.retained[id] = config
	digest, _ := service.StageExecutionTargetDigest(config.target)
	intent := uuid.NewString()
	inv := service.StageExecutionInvocation{IntentID: intent, IssueID: uuid.NewString(), ApprovalDecisionID: uuid.NewString(), Kind: "deployment", Target: config.target, TargetDigest: digest, ArtifactDigest: after, ExternalRequestKey: intent, DispatchExpiresAt: time.Now().Add(time.Minute)}
	first, err := source.captureExecution(t.Context(), inv, "original-native", "1", "succeeded", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.captureExecution(t.Context(), inv, "original-native", "1", "succeeded", nil, false)
	if err != nil || string(first.Payload) != string(second.Payload) {
		t.Fatal("lookup changed the first native capture")
	}
	if _, err = source.captureExecution(t.Context(), inv, "original-native", "1", "failed", nil, false); err == nil {
		t.Fatal("changed native payload overwrote captured observation")
	}
	if source.calls.Count != 0 {
		t.Fatal("observation lookup invoked the execution adapter")
	}
}
