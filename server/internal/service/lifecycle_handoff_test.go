package service

import "testing"

func TestValidateRCARouteRequiresExplicitHandoff(t *testing.T) {
	tests := []struct {
		name     string
		input    RCARouteInput
		decision LifecycleDecision
		wantErr  bool
	}{
		{
			name:     "known bug bypass",
			input:    RCARouteInput{Route: "bug-fix", CauseState: "known", Reason: "regression branch is identified", DownstreamRepairIssue: "FIX-1"},
			decision: LifecycleDecisionDirectRepair,
		},
		{
			name:     "unknown maintenance",
			input:    RCARouteInput{Route: "maintenance", CauseState: "unknown", DownstreamRepairIssue: "DIAG-1"},
			decision: LifecycleDecisionDiagnosis,
		},
		{
			name:     "incident after recovery",
			input:    RCARouteInput{Route: "incident", CauseState: "unknown", MitigationComplete: true, SeparateFollowUp: true, DownstreamRepairIssue: "RCA-1"},
			decision: LifecycleDecisionPostRecoveryRCA,
		},
		{
			name:    "known bypass without reason",
			input:   RCARouteInput{Route: "bug-fix", CauseState: "known", DownstreamRepairIssue: "FIX-1"},
			wantErr: true,
		},
		{
			name:    "incident before mitigation",
			input:   RCARouteInput{Route: "incident", CauseState: "unknown", SeparateFollowUp: true, DownstreamRepairIssue: "RCA-1"},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := ValidateRCARoute(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ValidateRCARoute() error = nil, want error (decision %q)", decision)
				}
				if decision != LifecycleDecisionUnknown {
					t.Fatalf("decision = %q, want unknown on invalid handoff", decision)
				}
				return
			}
			if err != nil || decision != test.decision {
				t.Fatalf("ValidateRCARoute() = %q, %v, want %q", decision, err, test.decision)
			}
		})
	}
}

func TestValidateIncidentLearningRequiresOwnedPreventionEvidence(t *testing.T) {
	base := IncidentLearningEvidence{
		SourceIssue:             "INC-1",
		Facts:                   []string{"alert fired"},
		Inferences:              []string{"retry amplified impact"},
		Unknowns:                []string{"dependency interval missing"},
		ExistingPreventionTasks: []string{"PREV-1"},
		PreventionTasks: []PreventionTaskEvidence{{
			Issue: "PREV-2", Title: "Add saturation alert", Owner: "reliability",
			Priority: "high", AcceptanceSignal: "alert catches saturation", RelatedIncident: "INC-1",
		}},
	}
	if err := ValidateIncidentLearning(base); err != nil {
		t.Fatalf("ValidateIncidentLearning() error = %v", err)
	}

	base.PreventionTasks[0].Owner = ""
	if err := ValidateIncidentLearning(base); err == nil {
		t.Fatal("ValidateIncidentLearning() accepted prevention task without owner")
	}
	base.PreventionTasks[0].Owner = "reliability"
	base.PreventionTasks[0].RelatedIncident = "INC-OTHER"
	if err := ValidateIncidentLearning(base); err == nil {
		t.Fatal("ValidateIncidentLearning() accepted prevention task linked to another incident")
	}
}

func TestEvaluateRolloutEvidenceFailsClosed(t *testing.T) {
	base := RolloutEvidence{
		ApprovedDigest: "sha256:release-1", ArtifactDigest: "sha256:release-1",
		Baseline:               map[string]float64{"error_rate": 0.2},
		ObservationWindowStart: "2026-09-24T10:00:00Z", ObservationWindowEnd: "2026-09-24T10:15:00Z", WindowComplete: true,
		Signals: []RolloutSignalEvidence{{Name: "error_rate", Value: 0.4, Threshold: 1}},
	}
	if got := EvaluateRolloutEvidence(base); got != LifecycleDecisionContinue {
		t.Fatalf("healthy rollout decision = %q, want continue", got)
	}

	for name, mutate := range map[string]func(*RolloutEvidence){
		"digest mismatch": func(e *RolloutEvidence) { e.ArtifactDigest = "sha256:rebuilt" },
		"window open":     func(e *RolloutEvidence) { e.WindowComplete = false },
		"window missing":  func(e *RolloutEvidence) { e.ObservationWindowEnd = "" },
		"window reversed": func(e *RolloutEvidence) { e.ObservationWindowEnd = e.ObservationWindowStart },
		"missing baseline": func(e *RolloutEvidence) {
			e.Baseline = nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if got := EvaluateRolloutEvidence(candidate); got != LifecycleDecisionUnknown {
				t.Fatalf("decision = %q, want unknown", got)
			}
		})
	}

	base.Signals[0].Value = 2
	if got := EvaluateRolloutEvidence(base); got != LifecycleDecisionRollbackRecommend {
		t.Fatalf("unapproved threshold breach = %q, want rollback recommendation", got)
	}
	base.RollbackApproved = true
	if got := EvaluateRolloutEvidence(base); got != LifecycleDecisionHold {
		t.Fatalf("approved but unexecuted rollback = %q, want hold", got)
	}
	base.RollbackExecuted = true
	if got := EvaluateRolloutEvidence(base); got != LifecycleDecisionRollbackExecuted {
		t.Fatalf("executed rollback = %q, want rollback-executed", got)
	}
}

func TestValidateAgentEvaluationRequiresSixVersionedCaseCategories(t *testing.T) {
	evidence := AgentEvaluationEvidence{
		ArtifactDigest: "sha256:eval-1", BaselineVersion: "agent-v1", CandidateVersion: "agent-v2", SkillVersion: "skill-v1", MCPVersion: "mcp-v1",
		Cases: []AgentEvaluationCaseEvidence{
			{Category: "correctness", Trace: []string{"read", "answer"}, StopReason: "completed", Result: "confirmed"},
			{Category: "tool-failure", Trace: []string{"timeout"}, StopReason: "tool-timeout", Result: "unknown"},
			{Category: "safety", Trace: []string{"credential request", "refuse"}, StopReason: "safety-boundary", Result: "blocked"},
			{Category: "cost", Trace: []string{"budget exceeded"}, StopReason: "cost-threshold", Result: "hold"},
			{Category: "latency", Trace: []string{"slow response"}, StopReason: "latency-threshold", Result: "hold"},
			{Category: "drift", Trace: []string{"schema changed"}, StopReason: "schema-drift", Result: "hold"},
		},
	}
	if got := ValidateAgentEvaluation(evidence); got != LifecycleDecisionPass {
		t.Fatalf("complete evaluation = %q, want pass", got)
	}
	evidence.ArtifactDigest = ""
	if got := ValidateAgentEvaluation(evidence); got != LifecycleDecisionUnknown {
		t.Fatalf("missing artifact digest evaluation = %q, want unknown", got)
	}

	evidence.Cases[0].Trace = nil
	if got := ValidateAgentEvaluation(evidence); got != LifecycleDecisionUnknown {
		t.Fatalf("missing trace evaluation = %q, want unknown", got)
	}
}
