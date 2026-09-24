package service

import (
	"embed"
	"encoding/json"
	"strings"
	"testing"
)

//go:embed testdata/lifecycle/lifecycle_handoff_fixtures.json
var lifecycleHandoffFixtureFS embed.FS

type lifecycleHandoffFixture struct {
	Delivery     deliveryFixture        `json:"delivery"`
	AgentQuality agentQualityFixture    `json:"agent_quality"`
	Failures     []lifecycleFailureCase `json:"failure_cases"`
}

type deliveryFixture struct {
	RCA              rcaFixture              `json:"rca"`
	RCARoutes        []rcaRouteFixture       `json:"rca_routes"`
	Repair           repairFixture           `json:"repair"`
	IncidentLearning incidentLearningFixture `json:"incident_learning"`
	Rollout          rolloutFixture          `json:"rollout"`
	Governance       []governanceFixture     `json:"governance"`
}

type rcaRouteFixture struct {
	Route                 string `json:"route"`
	CauseState            string `json:"cause_state"`
	Decision              string `json:"decision"`
	Reason                string `json:"reason"`
	MitigationBeforeRCA   bool   `json:"mitigation_before_rca"`
	SeparateFollowUp      bool   `json:"separate_follow_up"`
	DownstreamRepairIssue string `json:"downstream_repair_issue"`
}

type rcaFixture struct {
	SourceIssue      string   `json:"source_issue"`
	CommentID        string   `json:"comment_id"`
	Conclusion       string   `json:"conclusion"`
	Evidence         []string `json:"evidence"`
	Unknowns         []string `json:"unknowns"`
	SeparateFollowUp bool     `json:"separate_follow_up"`
}

type repairFixture struct {
	Issue          string `json:"issue"`
	DiagnosisRef   string `json:"diagnosis_ref"`
	RegressionTest string `json:"regression_test"`
}

type incidentLearningFixture struct {
	SourceIssue             string           `json:"source_issue"`
	Facts                   []string         `json:"facts"`
	Inferences              []string         `json:"inferences"`
	Unknowns                []string         `json:"unknowns"`
	ExistingPreventionTasks []string         `json:"existing_prevention_tasks"`
	PreventionTasks         []preventionTask `json:"prevention_tasks"`
	DuplicateIncidentLinks  []string         `json:"duplicate_incident_links"`
	Decision                string           `json:"decision"`
}

type preventionTask struct {
	Issue            string `json:"issue"`
	Title            string `json:"title"`
	Owner            string `json:"owner"`
	Priority         string `json:"priority"`
	AcceptanceSignal string `json:"acceptance_signal"`
	RelatedIncident  string `json:"related_incident"`
}

type rolloutFixture struct {
	ApprovedDigest    string             `json:"approved_digest"`
	ArtifactDigest    string             `json:"artifact_digest"`
	Baseline          map[string]float64 `json:"baseline"`
	ObservationWindow observationWindow  `json:"observation_window"`
	Signals           []rolloutSignal    `json:"signals"`
	Decision          string             `json:"decision"`
	Rollback          rollbackFixture    `json:"rollback"`
}

type observationWindow struct {
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at"`
	Complete  bool   `json:"complete"`
}

type rolloutSignal struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}

type rollbackFixture struct {
	Approved bool   `json:"approved"`
	Executed bool   `json:"executed"`
	Outcome  string `json:"outcome"`
}

type governanceFixture struct {
	Capability string                    `json:"capability"`
	Signals    []governanceSignalFixture `json:"signals"`
	Decision   string                    `json:"decision"`
}

type governanceSignalFixture struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type agentQualityFixture struct {
	ArtifactDigest   string           `json:"artifact_digest"`
	BaselineVersion  string           `json:"baseline_version"`
	CandidateVersion string           `json:"candidate_version"`
	SkillVersion     string           `json:"skill_version"`
	MCPVersion       string           `json:"mcp_version"`
	Cases            []evaluationCase `json:"cases"`
}

type evaluationCase struct {
	ID         string   `json:"id"`
	Category   string   `json:"category"`
	Trace      []string `json:"trace"`
	StopReason string   `json:"stop_reason"`
	Result     string   `json:"result"`
	CostUSD    float64  `json:"cost_usd"`
	LatencyMS  int      `json:"latency_ms"`
}

type lifecycleFailureCase struct {
	Name                   string `json:"name"`
	Kind                   string `json:"kind"`
	ApprovedDigest         string `json:"approved_digest"`
	ObservedDigest         string `json:"observed_digest"`
	WindowComplete         bool   `json:"window_complete"`
	BaselinePresent        bool   `json:"baseline_present"`
	SourceIssue            string `json:"source_issue"`
	ExistingPreventionTask string `json:"existing_prevention_task"`
	BaselineVersion        string `json:"baseline_version"`
	CandidateVersion       string `json:"candidate_version"`
	TracePresent           bool   `json:"trace_present"`
	GovernanceCapability   string `json:"governance_capability"`
	ExpectedDecision       string `json:"expected_decision"`
}

func loadLifecycleHandoffFixture(t *testing.T) lifecycleHandoffFixture {
	t.Helper()
	body, err := lifecycleHandoffFixtureFS.ReadFile("testdata/lifecycle/lifecycle_handoff_fixtures.json")
	if err != nil {
		t.Fatalf("read lifecycle handoff fixture: %v", err)
	}
	var fixture lifecycleHandoffFixture
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatalf("decode lifecycle handoff fixture: %v", err)
	}
	return fixture
}

func TestLifecycleHandoffFixtures_DeliveryArtifactsAreConsumable(t *testing.T) {
	fixture := loadLifecycleHandoffFixture(t)

	if fixture.Delivery.RCA.Conclusion != "confirmed" || len(fixture.Delivery.RCA.Evidence) == 0 {
		t.Fatalf("RCA fixture must carry a confirmed evidence-backed conclusion: %+v", fixture.Delivery.RCA)
	}
	if !fixture.Delivery.RCA.SeparateFollowUp {
		t.Fatal("RCA fixture must be a separate follow-up so incident mitigation is not blocked")
	}
	if len(fixture.Delivery.RCARoutes) != 3 {
		t.Fatalf("RCA route fixture count = %d, want 3", len(fixture.Delivery.RCARoutes))
	}
	seenRoutes := map[string]bool{}
	for _, route := range fixture.Delivery.RCARoutes {
		seenRoutes[route.Route] = true
		decision, err := ValidateRCARoute(RCARouteInput{
			Route: route.Route, CauseState: route.CauseState, Reason: route.Reason,
			MitigationComplete: route.MitigationBeforeRCA, SeparateFollowUp: route.SeparateFollowUp,
			DownstreamRepairIssue: route.DownstreamRepairIssue,
		})
		if err != nil || decision != LifecycleDecision(route.Decision) {
			t.Errorf("RCA route %q contract = %q, %v; want %q", route.Route, decision, err, route.Decision)
		}
		if route.Route == "bug-fix" && (route.CauseState != "known" || route.Decision != "direct-repair" || route.Reason == "") {
			t.Errorf("bug-fix known-cause bypass must preserve a reason: %+v", route)
		}
		if route.Route == "maintenance" && (route.CauseState != "unknown" || route.Decision != "diagnosis" || route.DownstreamRepairIssue == "") {
			t.Errorf("maintenance unknown cause must route diagnosis before repair: %+v", route)
		}
		if route.Route == "incident" && (!route.MitigationBeforeRCA || !route.SeparateFollowUp || route.Decision != "post-recovery-rca") {
			t.Errorf("incident route must mitigate before an independent RCA: %+v", route)
		}
	}
	for _, route := range []string{"bug-fix", "maintenance", "incident"} {
		if !seenRoutes[route] {
			t.Errorf("RCA route fixture missing %q", route)
		}
	}
	expectedDiagnosisRef := fixture.Delivery.RCA.SourceIssue + "#" + fixture.Delivery.RCA.CommentID
	if fixture.Delivery.Repair.Issue == "" || fixture.Delivery.Repair.Issue == fixture.Delivery.RCA.SourceIssue || fixture.Delivery.Repair.DiagnosisRef != expectedDiagnosisRef || fixture.Delivery.Repair.RegressionTest == "" {
		t.Fatalf("repair fixture does not consume the RCA comment and regression contract: %+v", fixture.Delivery.Repair)
	}
	if err := ValidateRCAArtifact(RCAArtifact{
		DiagnosisRef: fixture.Delivery.Repair.DiagnosisRef, RegressionTest: fixture.Delivery.Repair.RegressionTest,
		Conclusion: fixture.Delivery.RCA.Conclusion, Evidence: fixture.Delivery.RCA.Evidence, Unknowns: fixture.Delivery.RCA.Unknowns,
	}); err != nil {
		t.Fatalf("repair fixture RCA artifact is not consumable: %v", err)
	}

	learning := fixture.Delivery.IncidentLearning
	if learning.SourceIssue != fixture.Delivery.RCA.SourceIssue || learning.Decision != "unknown" {
		t.Fatalf("incident-learning fixture must consume the incident and preserve unknown evidence: %+v", learning)
	}
	if len(learning.Facts) == 0 || len(learning.Inferences) == 0 || len(learning.Unknowns) == 0 {
		t.Fatalf("incident-learning fixture must separate facts, inferences and unknowns: %+v", learning)
	}
	if len(learning.ExistingPreventionTasks) == 0 || len(learning.DuplicateIncidentLinks) == 0 {
		t.Fatalf("incident-learning fixture must link existing prevention and duplicate work: %+v", learning)
	}
	for _, task := range learning.PreventionTasks {
		for field, value := range map[string]string{
			"issue": task.Issue, "title": task.Title, "owner": task.Owner,
			"priority": task.Priority, "acceptance_signal": task.AcceptanceSignal,
			"related_incident": task.RelatedIncident,
		} {
			if strings.TrimSpace(value) == "" {
				t.Errorf("prevention task %q has empty %s", task.Title, field)
			}
		}
		if task.RelatedIncident != learning.SourceIssue {
			t.Errorf("prevention task %q points to %q, want source incident %q", task.Title, task.RelatedIncident, learning.SourceIssue)
		}
	}
	preventionTasks := make([]PreventionTaskEvidence, 0, len(learning.PreventionTasks))
	for _, task := range learning.PreventionTasks {
		preventionTasks = append(preventionTasks, PreventionTaskEvidence{
			Issue: task.Issue, Title: task.Title, Owner: task.Owner, Priority: task.Priority,
			AcceptanceSignal: task.AcceptanceSignal, RelatedIncident: task.RelatedIncident,
		})
	}
	if err := ValidateIncidentLearning(IncidentLearningEvidence{
		SourceIssue: learning.SourceIssue, Facts: learning.Facts, Inferences: learning.Inferences,
		Unknowns: learning.Unknowns, ExistingPreventionTasks: learning.ExistingPreventionTasks,
		DuplicateIncidentLinks: learning.DuplicateIncidentLinks, PreventionTasks: preventionTasks,
	}); err != nil {
		t.Errorf("incident-learning contract rejected fixture: %v", err)
	}

	rollout := fixture.Delivery.Rollout
	if rollout.ApprovedDigest != rollout.ArtifactDigest || len(rollout.Baseline) == 0 || len(rollout.Signals) == 0 {
		t.Fatalf("rollout fixture does not bind evidence to the approved artifact: %+v", rollout)
	}
	if !rollout.ObservationWindow.Complete || rollout.ObservationWindow.StartedAt == "" || rollout.ObservationWindow.EndedAt == "" {
		t.Fatalf("rollout fixture must have a completed observation window: %+v", rollout.ObservationWindow)
	}
	if rollout.Decision != "rollback-recommendation" || rollout.Rollback.Approved || rollout.Rollback.Executed {
		t.Fatalf("rollout fixture must recommend, but not execute, an unapproved rollback: %+v", rollout)
	}
	rolloutSignals := make([]RolloutSignalEvidence, 0, len(rollout.Signals))
	for _, signal := range rollout.Signals {
		rolloutSignals = append(rolloutSignals, RolloutSignalEvidence{Name: signal.Name, Value: signal.Value, Threshold: signal.Threshold})
	}
	if got := EvaluateRolloutEvidence(RolloutEvidence{
		ApprovedDigest: rollout.ApprovedDigest, ArtifactDigest: rollout.ArtifactDigest,
		Baseline: rollout.Baseline, ObservationWindowStart: rollout.ObservationWindow.StartedAt,
		ObservationWindowEnd: rollout.ObservationWindow.EndedAt, WindowComplete: rollout.ObservationWindow.Complete,
		Signals: rolloutSignals, RollbackApproved: rollout.Rollback.Approved, RollbackExecuted: rollout.Rollback.Executed,
	}); got != LifecycleDecision(rollout.Decision) {
		t.Errorf("rollout contract = %q, want %q", got, rollout.Decision)
	}
	if len(fixture.Delivery.Governance) != 5 {
		t.Fatalf("governance fixture count = %d, want 5", len(fixture.Delivery.Governance))
	}
	for _, governance := range fixture.Delivery.Governance {
		signals := make([]GovernanceSignalEvidence, 0, len(governance.Signals))
		for _, signal := range governance.Signals {
			signals = append(signals, GovernanceSignalEvidence{Name: signal.Name, Status: signal.Status})
		}
		if got := ValidateGovernanceEvidence(GovernanceEvidence{Capability: governance.Capability, Signals: signals}); got != LifecycleDecision(governance.Decision) {
			t.Errorf("governance %q = %q, want %q", governance.Capability, got, governance.Decision)
		}
	}
}

func TestLifecycleHandoffFixtures_AgentQualityGateHasReproducibleCases(t *testing.T) {
	fixture := loadLifecycleHandoffFixture(t)
	evaluation := fixture.AgentQuality
	if evaluation.ArtifactDigest == "" || evaluation.BaselineVersion == "" || evaluation.CandidateVersion == "" || evaluation.SkillVersion == "" || evaluation.MCPVersion == "" {
		t.Fatalf("agent evaluation fixture is missing version provenance: %+v", evaluation)
	}
	requiredCategories := map[string]bool{
		"correctness": false, "tool-failure": false, "safety": false,
		"cost": false, "latency": false, "drift": false,
	}
	for _, testCase := range evaluation.Cases {
		if _, ok := requiredCategories[testCase.Category]; ok {
			requiredCategories[testCase.Category] = true
		}
		if testCase.ID == "credential-request" && (testCase.StopReason != "safety-boundary" || testCase.Result != "blocked") {
			t.Errorf("credential case must stop at the safety boundary: %+v", testCase)
		}
		if len(testCase.Trace) == 0 || testCase.StopReason == "" || testCase.Result == "" {
			t.Errorf("evaluation case %q is not reproducible: %+v", testCase.ID, testCase)
		}
	}
	for category, seen := range requiredCategories {
		if !seen {
			t.Errorf("evaluation fixture is missing required category %q", category)
		}
	}
	evaluationCases := make([]AgentEvaluationCaseEvidence, 0, len(evaluation.Cases))
	for _, testCase := range evaluation.Cases {
		evaluationCases = append(evaluationCases, AgentEvaluationCaseEvidence{
			Category: testCase.Category, Trace: testCase.Trace, StopReason: testCase.StopReason, Result: testCase.Result,
		})
	}
	if got := ValidateAgentEvaluation(AgentEvaluationEvidence{
		ArtifactDigest: evaluation.ArtifactDigest, BaselineVersion: evaluation.BaselineVersion, CandidateVersion: evaluation.CandidateVersion,
		SkillVersion: evaluation.SkillVersion, MCPVersion: evaluation.MCPVersion, Cases: evaluationCases,
	}); got != LifecycleDecisionPass {
		t.Errorf("agent evaluation contract = %q, want pass", got)
	}
}

func TestLifecycleHandoffFixtures_FailureCasesProduceExplicitStops(t *testing.T) {
	fixture := loadLifecycleHandoffFixture(t)
	if len(fixture.Failures) != 10 {
		t.Fatalf("failure fixture count = %d, want 10", len(fixture.Failures))
	}
	for _, failure := range fixture.Failures {
		t.Run(failure.Name, func(t *testing.T) {
			if failure.ExpectedDecision == "" {
				t.Fatal("failure case has no expected decision")
			}
			switch failure.Kind {
			case "rollout":
				if failure.ApprovedDigest == failure.ObservedDigest && failure.Name == "rollout-digest-mismatch" {
					t.Fatal("digest mismatch fixture accidentally uses the approved digest")
				}
				if failure.Name == "rollout-window-open" && failure.WindowComplete {
					t.Fatal("open-window fixture is marked complete")
				}
				if failure.Name == "rollout-missing-baseline" && failure.BaselinePresent {
					t.Fatal("missing-baseline fixture is marked present")
				}
				if failure.ExpectedDecision != "unknown" {
					t.Errorf("rollout failure decision = %q, want unknown", failure.ExpectedDecision)
				}
			case "incident-learning":
				if failure.SourceIssue == "" || failure.ExistingPreventionTask == "" || failure.ExpectedDecision != "link-existing-task" {
					t.Fatalf("duplicate incident must link an existing task: %+v", failure)
				}
			case "agent-evaluation":
				if failure.BaselineVersion == "" || failure.CandidateVersion == "" || failure.TracePresent || failure.ExpectedDecision != "unknown" {
					t.Fatalf("missing-trace evaluation must stop as unknown: %+v", failure)
				}
			case "governance":
				if failure.GovernanceCapability == "" || failure.ExpectedDecision == "pass" {
					t.Fatalf("governance failure must identify a capability and stop: %+v", failure)
				}
			default:
				t.Fatalf("unknown failure kind %q", failure.Kind)
			}
		})
	}
}

func TestLifecycleHandoffFixtures_AreDesensitized(t *testing.T) {
	body, err := lifecycleHandoffFixtureFS.ReadFile("testdata/lifecycle/lifecycle_handoff_fixtures.json")
	if err != nil {
		t.Fatalf("read lifecycle handoff fixture: %v", err)
	}
	for _, forbidden := range []string{"BEGIN PRIVATE KEY", "Authorization:", "Bearer ", "customer@example.com", "production-token"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("fixture contains forbidden sensitive marker %q", forbidden)
		}
	}
	if strings.Contains(string(body), "TODO") {
		t.Error("fixture contains an unresolved TODO instead of an explicit evidence value")
	}
}
