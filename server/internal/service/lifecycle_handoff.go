package service

import (
	"fmt"
	"strings"
	"time"
)

// LifecycleDecision is the fail-closed outcome shared by lifecycle handoffs.
// It is intentionally small: callers may add a richer report, but they must
// not turn an unknown or hold into a successful gate.
type LifecycleDecision string

const (
	LifecycleDecisionDirectRepair      LifecycleDecision = "direct-repair"
	LifecycleDecisionDiagnosis         LifecycleDecision = "diagnosis"
	LifecycleDecisionPostRecoveryRCA   LifecycleDecision = "post-recovery-rca"
	LifecycleDecisionContinue          LifecycleDecision = "continue"
	LifecycleDecisionRollbackRecommend LifecycleDecision = "rollback-recommendation"
	LifecycleDecisionRollbackExecuted  LifecycleDecision = "rollback-executed"
	LifecycleDecisionPass              LifecycleDecision = "pass"
	LifecycleDecisionHold              LifecycleDecision = "hold"
	LifecycleDecisionUnknown           LifecycleDecision = "unknown"
)

// RCARouteInput describes the evidence a squad leader records before routing
// a failure. The route names match the built-in squad templates.
type RCARouteInput struct {
	Route                 string
	CauseState            string
	Reason                string
	MitigationComplete    bool
	SeparateFollowUp      bool
	DownstreamRepairIssue string
}

// RCAArtifact is the provider-independent evidence handed from diagnosis to
// the implementation issue. References are optional for the initial
// diagnosis task, but when present they must identify both the source and its
// regression test.
type RCAArtifact struct {
	DiagnosisRef   string
	RegressionTest string
	Conclusion     string
	Evidence       []string
	Unknowns       []string
}

// ValidateRCAArtifact keeps a repair handoff from claiming a confirmed cause
// without a traceable evidence item. An empty artifact is valid for the
// diagnosis-first leg, where the diagnostician has not concluded yet.
func ValidateRCAArtifact(input RCAArtifact) error {
	diagnosisRef := strings.TrimSpace(input.DiagnosisRef)
	regressionTest := strings.TrimSpace(input.RegressionTest)
	conclusion := strings.TrimSpace(input.Conclusion)
	if (diagnosisRef == "") != (regressionTest == "") {
		return fmt.Errorf("RCA diagnosis_ref and regression_test must be provided together")
	}
	if conclusion == "" && (diagnosisRef != "" || len(input.Evidence) > 0 || len(input.Unknowns) > 0) {
		return fmt.Errorf("RCA evidence requires a conclusion")
	}
	if conclusion != "" && conclusion != "confirmed" && conclusion != "suspected" && conclusion != "unknown" {
		return fmt.Errorf("unsupported RCA conclusion %q", conclusion)
	}
	if conclusion == "confirmed" && len(input.Evidence) == 0 {
		return fmt.Errorf("confirmed RCA requires evidence")
	}
	for _, item := range append(append([]string{}, input.Evidence...), input.Unknowns...) {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("RCA evidence and unknowns cannot contain empty items")
		}
	}
	return nil
}

// ValidateRCARoute enforces the distinction between a known-cause repair,
// diagnosis-first maintenance, and incident recovery followed by RCA.
func ValidateRCARoute(input RCARouteInput) (LifecycleDecision, error) {
	input.Route = strings.TrimSpace(input.Route)
	input.CauseState = strings.TrimSpace(input.CauseState)
	input.Reason = strings.TrimSpace(input.Reason)
	input.DownstreamRepairIssue = strings.TrimSpace(input.DownstreamRepairIssue)

	switch input.Route {
	case "bug-fix":
		if input.CauseState == "known" {
			if input.Reason == "" {
				return LifecycleDecisionUnknown, fmt.Errorf("known bug-fix bypass requires a reason")
			}
			if input.DownstreamRepairIssue == "" {
				return LifecycleDecisionUnknown, fmt.Errorf("known bug-fix bypass requires a downstream repair issue")
			}
			return LifecycleDecisionDirectRepair, nil
		}
		if input.CauseState == "unknown" && input.DownstreamRepairIssue != "" {
			return LifecycleDecisionDiagnosis, nil
		}
		return LifecycleDecisionUnknown, fmt.Errorf("bug-fix route requires known cause or an explicit diagnosis follow-up")
	case "maintenance":
		if input.CauseState != "unknown" || input.DownstreamRepairIssue == "" {
			return LifecycleDecisionUnknown, fmt.Errorf("maintenance route requires an unknown cause and diagnosis follow-up")
		}
		return LifecycleDecisionDiagnosis, nil
	case "incident":
		if !input.MitigationComplete {
			return LifecycleDecisionUnknown, fmt.Errorf("incident RCA cannot start before mitigation is complete")
		}
		if !input.SeparateFollowUp || input.DownstreamRepairIssue == "" {
			return LifecycleDecisionUnknown, fmt.Errorf("incident RCA requires a separate follow-up and downstream repair issue")
		}
		return LifecycleDecisionPostRecoveryRCA, nil
	default:
		return LifecycleDecisionUnknown, fmt.Errorf("unsupported RCA route %q", input.Route)
	}
}

// PreventionTaskEvidence is the minimum downstream contract for an
// incident-learning action.
type PreventionTaskEvidence struct {
	Issue            string
	Title            string
	Owner            string
	Priority         string
	AcceptanceSignal string
	RelatedIncident  string
}

// IncidentLearningEvidence keeps facts, inferences and unknowns separate so
// a missing signal cannot be mistaken for a conclusion.
type IncidentLearningEvidence struct {
	SourceIssue             string
	Facts                   []string
	Inferences              []string
	Unknowns                []string
	ExistingPreventionTasks []string
	DuplicateIncidentLinks  []string
	PreventionTasks         []PreventionTaskEvidence
}

// ValidateIncidentLearning checks that every prevention task is attributable
// and that duplicate work can be linked instead of recreated.
func ValidateIncidentLearning(input IncidentLearningEvidence) error {
	if strings.TrimSpace(input.SourceIssue) == "" {
		return fmt.Errorf("incident-learning requires a source issue")
	}
	if len(input.Facts) == 0 || len(input.Inferences) == 0 || len(input.Unknowns) == 0 {
		return fmt.Errorf("incident-learning must separate facts, inferences and unknowns")
	}
	if len(input.ExistingPreventionTasks) == 0 && len(input.PreventionTasks) == 0 {
		return fmt.Errorf("incident-learning requires an existing or new prevention task")
	}
	for _, task := range input.PreventionTasks {
		if strings.TrimSpace(task.Issue) == "" || strings.TrimSpace(task.Title) == "" ||
			strings.TrimSpace(task.Owner) == "" || strings.TrimSpace(task.Priority) == "" ||
			strings.TrimSpace(task.AcceptanceSignal) == "" {
			return fmt.Errorf("prevention task %q is missing owner or acceptance signal", task.Title)
		}
		if strings.TrimSpace(task.RelatedIncident) != strings.TrimSpace(input.SourceIssue) {
			return fmt.Errorf("prevention task %q is not linked to source incident", task.Title)
		}
	}
	return nil
}

type RolloutSignalEvidence struct {
	Name      string
	Value     float64
	Threshold float64
}

// RolloutEvidence is deliberately independent of an observability provider.
// A provider adapter can populate it later without gaining rollback authority.
type RolloutEvidence struct {
	ApprovedDigest         string
	ArtifactDigest         string
	Baseline               map[string]float64
	ObservationWindowStart string
	ObservationWindowEnd   string
	WindowComplete         bool
	Signals                []RolloutSignalEvidence
	RollbackApproved       bool
	RollbackExecuted       bool
}

// EvaluateRolloutEvidence returns a fail-closed decision. It never performs a
// rollback; the release operator and human approval boundary remain separate.
func EvaluateRolloutEvidence(input RolloutEvidence) LifecycleDecision {
	if strings.TrimSpace(input.ApprovedDigest) == "" ||
		strings.TrimSpace(input.ArtifactDigest) == "" ||
		input.ApprovedDigest != input.ArtifactDigest ||
		len(input.Baseline) == 0 ||
		strings.TrimSpace(input.ObservationWindowStart) == "" ||
		strings.TrimSpace(input.ObservationWindowEnd) == "" ||
		!input.WindowComplete ||
		len(input.Signals) == 0 {
		return LifecycleDecisionUnknown
	}
	startedAt, err := time.Parse(time.RFC3339Nano, input.ObservationWindowStart)
	if err != nil {
		return LifecycleDecisionUnknown
	}
	endedAt, err := time.Parse(time.RFC3339Nano, input.ObservationWindowEnd)
	if err != nil || !endedAt.After(startedAt) {
		return LifecycleDecisionUnknown
	}
	thresholdExceeded := false
	for _, signal := range input.Signals {
		if strings.TrimSpace(signal.Name) == "" {
			return LifecycleDecisionUnknown
		}
		if signal.Value > signal.Threshold {
			thresholdExceeded = true
		}
	}
	if !thresholdExceeded {
		return LifecycleDecisionContinue
	}
	if !input.RollbackApproved {
		return LifecycleDecisionRollbackRecommend
	}
	if !input.RollbackExecuted {
		return LifecycleDecisionHold
	}
	return LifecycleDecisionRollbackExecuted
}

type AgentEvaluationCaseEvidence struct {
	Category   string
	Trace      []string
	StopReason string
	Result     string
}

type AgentEvaluationEvidence struct {
	ArtifactDigest   string
	BaselineVersion  string
	CandidateVersion string
	SkillVersion     string
	MCPVersion       string
	Cases            []AgentEvaluationCaseEvidence
}

// ValidateAgentEvaluation requires a reproducible case for each quality
// dimension before a review or release gate can claim a pass.
func ValidateAgentEvaluation(input AgentEvaluationEvidence) LifecycleDecision {
	if strings.TrimSpace(input.ArtifactDigest) == "" ||
		strings.TrimSpace(input.BaselineVersion) == "" ||
		strings.TrimSpace(input.CandidateVersion) == "" ||
		strings.TrimSpace(input.SkillVersion) == "" ||
		strings.TrimSpace(input.MCPVersion) == "" {
		return LifecycleDecisionUnknown
	}
	required := map[string]bool{
		"correctness":  false,
		"tool-failure": false,
		"safety":       false,
		"cost":         false,
		"latency":      false,
		"drift":        false,
	}
	for _, testCase := range input.Cases {
		if _, ok := required[testCase.Category]; ok {
			required[testCase.Category] = true
		}
		if len(testCase.Trace) == 0 || strings.TrimSpace(testCase.StopReason) == "" || strings.TrimSpace(testCase.Result) == "" {
			return LifecycleDecisionUnknown
		}
		if testCase.Category == "safety" && testCase.Result != "blocked" {
			return LifecycleDecisionHold
		}
	}
	for _, seen := range required {
		if !seen {
			return LifecycleDecisionUnknown
		}
	}
	return LifecycleDecisionPass
}

// GovernanceSignalEvidence is a redacted, provider-independent result for a
// single governance check. The status is deliberately small so callers cannot
// turn a missing artifact into a successful gate.
type GovernanceSignalEvidence struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type GovernanceEvidence struct {
	Capability string
	Signals    []GovernanceSignalEvidence
}

var governanceRequiredSignals = map[string][]string{
	"contract-compatibility": {"old-client-matrix", "plugin-boundary", "parse-with-fallback"},
	"threat-modeling":        {"trust-boundaries", "abuse-paths", "mitigations"},
	"supply-chain":           {"lockfile", "provenance", "sbom", "license-review"},
	"product-outcome":        {"baseline", "observation-window", "observed-result", "owner"},
	"disaster-recovery":      {"backup", "restore-drill", "rpo-rto", "degradation-path"},
}

// ValidateGovernanceEvidence is the shared fail-closed gate for the five
// governance capabilities. A failed signal requires a human decision (hold),
// while an absent or unknown signal stays unknown.
func ValidateGovernanceEvidence(input GovernanceEvidence) LifecycleDecision {
	capability := strings.TrimSpace(input.Capability)
	required, ok := governanceRequiredSignals[capability]
	if !ok {
		return LifecycleDecisionUnknown
	}
	seen := make(map[string]string, len(input.Signals))
	for _, signal := range input.Signals {
		name := strings.TrimSpace(signal.Name)
		status := strings.TrimSpace(signal.Status)
		if name == "" || (status != "pass" && status != "fail" && status != "unknown") {
			return LifecycleDecisionUnknown
		}
		if _, exists := seen[name]; exists {
			return LifecycleDecisionUnknown
		}
		seen[name] = status
	}
	unknown := false
	for _, name := range required {
		switch seen[name] {
		case "pass":
			continue
		case "fail":
			return LifecycleDecisionHold
		case "unknown", "":
			unknown = true
		}
	}
	if unknown {
		return LifecycleDecisionUnknown
	}
	return LifecycleDecisionPass
}
