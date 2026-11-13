package projecthealth

import (
	"testing"
	"time"
)

func TestSignalsKeepHealthPriorityAndCalendarSemantics(t *testing.T) {
	date := "2026-11-05"
	anchor := time.Date(2026, 11, 5, 23, 0, 0, 0, time.UTC)
	in := SignalInput{ProjectStatus: "in_progress", LeadValid: true, InProgressSince: &anchor, Calendar: CalendarFacts{ReferenceDate: "2026-11-13", InProgressSinceDate: &date}, Issues: []IssueSignal{{ID: "one", Status: "todo", Category: "todo", AssignmentValid: true}}}
	out, err := ComputeSignals(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Statistics.ProgressAgeDays != nil || out.Statistics.Health != "attention" || !hasSignalReason(out.Statistics.Reasons, "stale_progress") {
		t.Fatalf("no-update stale semantics changed: %+v", out.Statistics)
	}
	in.ProjectStatus = "paused"
	in.Issues[0].EnvironmentUnavailable = true
	out, err = ComputeSignals(in)
	if err != nil || out.Statistics.Health != "clear" || !hasSignalReason(out.Statistics.Reasons, "execution_environment_unavailable") || hasSignalReason(out.Statistics.Reasons, "stale_progress") {
		t.Fatalf("reason incorrectly changed health: %+v / %v", out.Statistics, err)
	}
	in.ProjectStatus = "completed"
	out, err = ComputeSignals(in)
	if err != nil || out.Statistics.Health != "clear" || !hasSignalReason(out.Statistics.Reasons, "ended_project_open_issues") {
		t.Fatalf("ended-project reason changed health: %+v / %v", out.Statistics, err)
	}
	in.ProjectStatus = "in_progress"
	in.ProjectDueDate = ptr("2026-11-12")
	in.Issues = nil
	out, err = ComputeSignals(in)
	if err != nil || out.Statistics.Health != "empty" || !*out.Statistics.ProjectOverdue {
		t.Fatalf("empty priority changed: %+v / %v", out.Statistics, err)
	}
}

func TestSignalsPreserveUnknownAndTerminalPartitions(t *testing.T) {
	in := SignalInput{ProjectStatus: "in_progress", LeadValid: true, Calendar: CalendarFacts{ReferenceDate: "2026-11-13"}, Issues: []IssueSignal{{ID: "one", Status: "lost-custom-status", DueDate: ptr("2026-11-12"), AssignmentValid: true}, {ID: "two", Status: "done", Category: "done", DueDate: ptr("2026-11-12")}}}
	out, err := ComputeSignals(in)
	if err != nil {
		t.Fatal(err)
	}
	s := out.Statistics
	if *s.Counts.Total != 2 || *s.Counts.Open != 1 || *s.Counts.UnknownStatus != 1 || *s.Counts.Completed != 1 || *s.Counts.Overdue != 1 || *s.Counts.Unassigned != 0 || s.Complete || s.Health != "unavailable" {
		t.Fatalf("unknown or terminal partition changed: %+v", s)
	}
}

func hasSignalReason(reasons []string, reason string) bool {
	for _, got := range reasons {
		if got == reason {
			return true
		}
	}
	return false
}
