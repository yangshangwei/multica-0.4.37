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
	Repair           repairFixture           `json:"repair"`
	IncidentLearning incidentLearningFixture `json:"incident_learning"`
	Rollout          rolloutFixture          `json:"rollout"`
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
	expectedDiagnosisRef := fixture.Delivery.RCA.SourceIssue + "#" + fixture.Delivery.RCA.CommentID
	if fixture.Delivery.Repair.Issue == "" || fixture.Delivery.Repair.Issue == fixture.Delivery.RCA.SourceIssue || fixture.Delivery.Repair.DiagnosisRef != expectedDiagnosisRef || fixture.Delivery.Repair.RegressionTest == "" {
		t.Fatalf("repair fixture does not consume the RCA comment and regression contract: %+v", fixture.Delivery.Repair)
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
}

func TestLifecycleHandoffFixtures_FailureCasesProduceExplicitStops(t *testing.T) {
	fixture := loadLifecycleHandoffFixture(t)
	if len(fixture.Failures) != 5 {
		t.Fatalf("failure fixture count = %d, want 5", len(fixture.Failures))
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
