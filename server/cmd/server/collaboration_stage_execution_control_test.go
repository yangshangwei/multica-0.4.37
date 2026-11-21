package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stageControlRequest struct {
	RequestID               string  `json:"request_id"`
	Action                  string  `json:"action"`
	WorkspaceID             string  `json:"workspace_id"`
	DeploymentIntegrationID string  `json:"deployment_integration_id"`
	MetricIntegrationID     string  `json:"metric_integration_id"`
	Environment             string  `json:"environment"`
	RemoteInstance          string  `json:"remote_instance"`
	RemoteTenant            string  `json:"remote_tenant"`
	RemoteTarget            string  `json:"remote_target"`
	ConditionalVersion      string  `json:"conditional_version"`
	CurrentArtifactDigest   *string `json:"current_artifact_digest"`
	ConfigRevision          string  `json:"config_revision"`
	QueryID                 string  `json:"query_id"`
	QueryDigest             string  `json:"query_digest"`
	RuleID                  string  `json:"rule_id"`
	RuleDigest              string  `json:"rule_digest"`
	MetricFreshnessSeconds  int64   `json:"metric_freshness_seconds"`
	Behavior                string  `json:"behavior"`
	DelayMS                 int64   `json:"delay_ms"`
	ExternalOperationID     *string `json:"external_operation_id"`
	IssueID                 string  `json:"issue_id"`
	ApprovalDecisionID      string  `json:"approval_decision_id"`
	TargetDigest            string  `json:"target_digest"`
	Candidate               *struct {
		CandidateID   string `json:"candidate_id"`
		Version       string `json:"version"`
		ContentDigest string `json:"content_digest"`
	} `json:"candidate"`
	IntentID      string    `json:"intent_id"`
	NativeID      string    `json:"native_id"`
	NativeVersion string    `json:"native_version"`
	Result        string    `json:"result"`
	DeploymentID  string    `json:"deployment_id"`
	Condition     string    `json:"condition"`
	WindowStart   time.Time `json:"window_start"`
	WindowEnd     time.Time `json:"window_end"`
	ObservedAt    time.Time `json:"observed_at"`
	ValidUntil    time.Time `json:"valid_until"`
	Summary       string    `json:"summary"`
}
type stageControlReply struct {
	RequestID string            `json:"request_id"`
	OK        bool              `json:"ok"`
	Result    any               `json:"result,omitempty"`
	Code      string            `json:"code,omitempty"`
	Calls     stageControlCalls `json:"calls"`
}
type stageExecutionController struct {
	source *stageControlledSource
	svc    *service.CollaborationService
}

func stageControlText(value string, limit int) bool {
	return len(value) > 0 && len(value) <= limit && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func stageControlUUID(value string) bool {
	id, err := util.ParseUUID(value)
	return err == nil && id.Valid && id.Bytes != [16]byte{} && util.UUIDToString(id) == value
}
func stageControlVersion(value string) bool {
	n, err := strconv.ParseInt(value, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == value
}
func decodeStageControl(raw []byte) (stageControlRequest, error) {
	var p stageControlRequest
	invalid := stageControlError("invalid_stage_control_request")
	if len(raw) > 64<<10 || iteration.ValidateObjectJSON(raw) != nil {
		return p, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&p) != nil || !stageControlUUID(p.RequestID) {
		return p, invalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return p, invalid
	}
	required, optional := []string{}, []string{}
	switch p.Action {
	case "source.configure":
		required = []string{"workspace_id", "deployment_integration_id", "metric_integration_id", "environment", "remote_instance", "remote_tenant", "remote_target", "conditional_version", "current_artifact_digest", "query_id", "query_digest", "rule_id", "rule_digest", "metric_freshness_seconds"}
		optional = []string{"config_revision"}
	case "source.behavior":
		required = []string{"behavior"}
		optional = []string{"delay_ms", "external_operation_id"}
	case "execution.prepare", "execution.receipt":
		required = []string{"issue_id", "approval_decision_id", "candidate", "target_digest"}
	case "execution.observe":
		required = []string{"intent_id", "native_id", "native_version", "result"}
		optional = []string{"external_operation_id"}
	case "metrics.observe":
		required = []string{"deployment_id", "native_id", "native_version", "condition", "window_start", "window_end", "observed_at", "valid_until"}
		optional = []string{"summary"}
	case "shutdown":
	default:
		return p, invalid
	}
	allowed := map[string]bool{"request_id": true, "action": true}
	for _, key := range required {
		allowed[key] = true
		if _, ok := fields[key]; !ok {
			return p, invalid
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range fields {
		if !allowed[key] {
			return p, invalid
		}
	}
	if p.Candidate != nil {
		var candidate map[string]json.RawMessage
		if json.Unmarshal(fields["candidate"], &candidate) != nil || len(candidate) != 3 {
			return p, invalid
		}
		for _, key := range []string{"candidate_id", "version", "content_digest"} {
			if _, ok := candidate[key]; !ok {
				return p, invalid
			}
		}
	}
	return p, nil
}
func (c *stageExecutionController) handle(ctx context.Context, raw []byte) stageControlReply {
	p, err := decodeStageControl(raw)
	reply := stageControlReply{RequestID: p.RequestID}
	var result any
	if err == nil {
		result, err = c.apply(ctx, p)
	}
	if err != nil {
		var domain *service.CollaborationError
		if errors.As(err, &domain) {
			reply.Code = domain.Code
		} else {
			reply.Code = "stage_execution_unavailable"
		}
	} else {
		reply.OK, reply.Result = true, result
	}
	reply.Calls = c.source.callSnapshot()
	return reply
}
func (c *stageExecutionController) apply(ctx context.Context, p stageControlRequest) (any, error) {
	invalid := stageControlError("invalid_stage_control_request")
	switch p.Action {
	case "source.configure":
		if !stageControlUUID(p.WorkspaceID) || !stageControlUUID(p.DeploymentIntegrationID) || !stageControlUUID(p.MetricIntegrationID) || p.DeploymentIntegrationID == p.MetricIntegrationID || p.MetricFreshnessSeconds < 1 || p.MetricFreshnessSeconds > 3600 || !service.DeliveryHex(p.QueryDigest, 64) || !service.DeliveryHex(p.RuleDigest, 64) || !stageControlText(p.QueryID, 256) || !stageControlText(p.RuleID, 256) {
			return nil, invalid
		}
		if p.ConfigRevision == "" {
			p.ConfigRevision = "1"
		}
		if !stageControlVersion(p.ConfigRevision) {
			return nil, invalid
		}
		tx, err := c.svc.TxStarter.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)
		source := func(id, adapter string) (service.StageIntegrationIdentity, error) {
			var revision int64
			err := tx.QueryRow(ctx, `SELECT trust_revision FROM github_installation WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, id, p.WorkspaceID).Scan(&revision)
			return service.StageIntegrationIdentity{Adapter: adapter, AdapterVersion: "1", IntegrationID: id, TrustRevision: strconv.FormatInt(revision, 10), ConfigRevision: p.ConfigRevision}, err
		}
		deployment, err := source(p.DeploymentIntegrationID, "controlled-stage-deployment")
		if err != nil {
			return nil, err
		}
		metric, err := source(p.MetricIntegrationID, "controlled-stage-metrics")
		if err != nil {
			return nil, err
		}
		target := service.StageExecutionTarget{WorkspaceID: p.WorkspaceID, Environment: p.Environment, Source: deployment, ConcurrencyDomain: "controlled-stage-provider", RemoteInstance: p.RemoteInstance, RemoteTenant: p.RemoteTenant, RemoteTarget: p.RemoteTarget, SupportsConditional: true, SupportsOriginalQuery: true, ConditionalVersion: p.ConditionalVersion, CurrentArtifactDigest: p.CurrentArtifactDigest}
		digest, err := service.StageExecutionTargetDigest(target)
		if err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		config := stageControlConfiguration{target: target, metricSource: metric, queryID: p.QueryID, queryDigest: p.QueryDigest, ruleID: p.RuleID, ruleDigest: p.RuleDigest, freshness: time.Duration(p.MetricFreshnessSeconds) * time.Second}
		c.source.mu.Lock()
		c.source.configured = true
		c.source.current = config
		c.source.retained[p.DeploymentIntegrationID] = config
		c.source.mu.Unlock()
		return map[string]string{"target_digest": digest}, nil
	case "source.behavior":
		if p.DelayMS < 0 || p.DelayMS > 5000 {
			return nil, invalid
		}
		switch p.Behavior {
		case "succeeded", "failed", "unknown", "not_started", "panic", "delayed":
		default:
			return nil, invalid
		}
		if p.ExternalOperationID != nil && !stageControlText(*p.ExternalOperationID, 256) {
			return nil, invalid
		}
		c.source.mu.Lock()
		c.source.behavior = p.Behavior
		c.source.delay = time.Duration(p.DelayMS) * time.Millisecond
		c.source.operationID = p.ExternalOperationID
		c.source.mu.Unlock()
		return map[string]string{"behavior": p.Behavior}, nil
	case "execution.prepare", "execution.receipt":
		if !stageControlUUID(p.IssueID) || !stageControlUUID(p.ApprovalDecisionID) || p.Candidate == nil || !stageControlUUID(p.Candidate.CandidateID) || !stageControlVersion(p.Candidate.Version) || !service.DeliveryHex(p.Candidate.ContentDigest, 64) || !service.DeliveryHex(p.TargetDigest, 64) {
			return nil, invalid
		}
		issue, err := c.svc.Queries.GetIssue(ctx, util.MustParseUUID(p.IssueID))
		if err != nil {
			return nil, err
		}
		version, _ := strconv.ParseInt(p.Candidate.Version, 10, 64)
		request := service.StageExecutionRequest{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, RequestID: p.RequestID, ApprovalDecisionID: util.MustParseUUID(p.ApprovalDecisionID), Candidate: service.CollaborationStageCandidateRef{CandidateID: util.MustParseUUID(p.Candidate.CandidateID), Version: version, ContentDigest: p.Candidate.ContentDigest}, TargetDigest: p.TargetDigest}
		c.source.mu.Lock()
		target := c.source.current.target
		c.source.mu.Unlock()
		if original, e := c.svc.Queries.GetStageExecutionIntentByApproval(ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: issue.ID, ApprovalDecisionID: request.ApprovalDecisionID}); e == nil {
			var inv service.StageExecutionInvocation
			if json.Unmarshal(original.Invocation, &inv) != nil {
				return nil, invalid
			}
			target = inv.Target
		}
		proof, err := c.source.consumer(ctx, request, target)
		if err != nil {
			return nil, err
		}
		if p.Action == "execution.receipt" {
			return c.svc.StageExecutionReceipt(ctx, request, proof)
		}
		return c.svc.PrepareStageExecution(ctx, request, proof)
	case "execution.observe":
		if !stageControlUUID(p.IntentID) || !stageControlText(p.NativeID, 256) || !stageControlText(p.NativeVersion, 256) || p.ExternalOperationID != nil && !stageControlText(*p.ExternalOperationID, 256) {
			return nil, invalid
		}
		switch p.Result {
		case "succeeded", "failed", "unknown", "not_started":
		default:
			return nil, invalid
		}
		row, err := c.svc.Queries.GetStageExecutionIntent(ctx, util.MustParseUUID(p.IntentID))
		if err != nil {
			return nil, err
		}
		var inv service.StageExecutionInvocation
		if json.Unmarshal(row.Invocation, &inv) != nil {
			return nil, invalid
		}
		proof, err := c.source.captureExecution(ctx, inv, p.NativeID, p.NativeVersion, p.Result, p.ExternalOperationID, false)
		if err != nil {
			return nil, err
		}
		return c.svc.RecordStageExecutionObservation(ctx, row.IntentID, proof)
	case "metrics.observe":
		if !stageControlUUID(p.DeploymentID) || !stageControlVersion(p.NativeVersion) || !stageControlText(p.NativeID, 256) || len(p.Summary) > 2000 || !utf8.ValidString(p.Summary) || strings.ContainsRune(p.Summary, 0) {
			return nil, invalid
		}
		fact, err := c.svc.Queries.GetStageExecutionFact(ctx, util.MustParseUUID(p.DeploymentID))
		if err != nil {
			return nil, err
		}
		intent, err := c.svc.Queries.GetStageExecutionIntent(ctx, fact.IntentID)
		if err != nil {
			return nil, err
		}
		var inv service.StageExecutionInvocation
		if json.Unmarshal(intent.Invocation, &inv) != nil {
			return nil, invalid
		}
		config, err := c.source.configuration(inv.Target.Source.IntegrationID)
		if err != nil {
			return nil, err
		}
		metric := service.StageRollbackMetric{NativeID: p.NativeID, NativeVersion: p.NativeVersion, StreamIdentity: "controlled-metric-source:" + config.metricSource.IntegrationID, Source: config.metricSource, DeploymentID: p.DeploymentID, TargetDigest: intent.TargetDigest, ArtifactDigest: intent.ArtifactDigest, QueryID: config.queryID, QueryDigest: config.queryDigest, RuleID: config.ruleID, RuleDigest: config.ruleDigest, WindowStart: p.WindowStart, WindowEnd: p.WindowEnd, ObservedAt: p.ObservedAt, ValidUntil: p.ValidUntil, Condition: p.Condition, EvidenceDigest: stageControlHash(p.NativeVersion + ":" + p.Condition + ":" + p.Summary), Summary: p.Summary}
		proof := c.source.seal(stageControlProof{Kind: "metric", Source: config.metricSource, Metric: metric})
		return c.svc.RecordStageRollbackObservation(ctx, fact.ObservationID, proof)
	case "shutdown":
		return map[string]bool{"stopped": true}, nil
	}
	return nil, invalid
}

func stageExecutionRouter(t *testing.T, background bool) (*httptest.Server, *handler.Handler, *stageExecutionController, func()) {
	t.Helper()
	queries := db.New(testPool)
	hub := realtime.NewHub()
	go hub.Run()
	bus := events.New()
	registerListeners(bus, hub)
	registerSubscriberListeners(bus, testPool)
	registerActivityListeners(bus, queries)
	registerNotificationListeners(bus, queries, testPool)
	router, h := NewRouterWithOptions(testPool, hub, bus, analytics.NoopClient{}, nil, RouterOptions{})
	source, err := newStageControlledSource(testPool)
	if err != nil {
		t.Fatal(err)
	}
	h.StageExecutionSources = source
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	svc.StageExecutionSources = source
	hub.SetAuthorizer(&collaborationScopeAuthorizer{workspace: newScopeAuthorizer(queries), service: svc})
	server := httptest.NewUnstartedServer(router)
	if background && os.Getenv("MULTICA_TEST_STAGE_HARNESS") == "1" && strings.TrimSpace(os.Getenv("PORT")) != "" {
		port := strings.TrimSpace(os.Getenv("PORT"))
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			server.Listener.Close()
			t.Fatal("stage harness requires a valid explicit loopback PORT")
		}
		if err = server.Listener.Close(); err != nil {
			t.Fatal(err)
		}
		server.Listener, err = net.Listen("tcp", "127.0.0.1:"+port)
		if err != nil {
			t.Fatal("stage harness loopback listener unavailable")
		}
	}
	server.Start()
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	run := func(fn func(context.Context)) { workers.Add(1); go func() { defer workers.Done(); fn(ctx) }() }
	if background {
		run(h.CollaborationAccountReconciler.Run)
		run(h.CollaborationConfigReconciler.Run)
		run((&service.CollaborationRealtime{Queries: queries, TxStarter: testPool, Publisher: hub}).Run)
		run((&service.CollaborationKnowledgeInvalidation{Queries: queries, TxStarter: testPool, Publisher: hub}).Run)
		run(func(ctx context.Context) {
			runRuntimeSweeper(ctx, queries, handler.NewNoopLivenessStore(), h.TaskService, bus, defaultRuntimeReconnectGrace)
		})
	}
	stop := func() { server.Close(); cancel(); workers.Wait() }
	return server, h, &stageExecutionController{source: source, svc: svc}, stop
}

func TestCollaborationStageExecutionHarness(t *testing.T) {
	if os.Getenv("MULTICA_TEST_STAGE_HARNESS") != "1" {
		t.Skip("dedicated coordinator-owned stage harness only")
	}
	for _, key := range []string{"RESEND_API_KEY", "SMTP_HOST", "GITHUB_APP_ID", "GITHUB_APP_PRIVATE_KEY"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			t.Fatal("stage harness requires isolated synthetic integration configuration")
		}
	}
	api, _, controller, stop := stageExecutionRouter(t, true)
	defer stop()
	ready, _ := json.Marshal(map[string]any{"version": 1, "api_url": api.URL, "pid": os.Getpid()})
	fmt.Println("COLLABORATION_STAGE_READY " + string(ready))
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		raw := append([]byte(nil), scanner.Bytes()...)
		reply := controller.handle(context.Background(), raw)
		body, err := json.Marshal(reply)
		if err != nil {
			t.Fatal("stage control response encoding failed")
		}
		fmt.Println("COLLABORATION_STAGE_REPLY " + string(body))
		p, err := decodeStageControl(raw)
		if err == nil && p.Action == "shutdown" && reply.OK {
			return
		}
	}
	if scanner.Err() != nil {
		t.Fatal("stage control input unavailable or oversized")
	}
}
