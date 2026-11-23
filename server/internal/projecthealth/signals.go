package projecthealth

import (
	"fmt"
	"sort"
	"time"
)

// IssueSignal resolves directory joins before the health calculation. IDs only
// identify the issue whose contribution is being calculated, never its assignee.
type IssueSignal struct {
	ID, Status, Category                    string
	DueDate                                 *string
	AssignmentValid, EnvironmentUnavailable bool
}

type SignalInput struct {
	ProjectStatus   string
	ProjectDueDate  *string
	LeadValid       bool
	InProgressSince *time.Time
	LatestUpdateAt  *time.Time
	Calendar        CalendarFacts
	Issues          []IssueSignal
}

// ComputeSignals owns the identity-free health rules. Callers supply already
// resolved validity and calendar facts; it performs no directory or tzdb reads.
// Provenance and input hashing remain the responsibility of the caller.
func ComputeSignals(in SignalInput) (Collection, error) {
	calendar := in.Calendar
	if !validCalendarDate(calendar.ReferenceDate) ||
		(in.LatestUpdateAt == nil) != (calendar.LatestUpdateDate == nil) ||
		(in.InProgressSince == nil) != (calendar.InProgressSinceDate == nil) {
		return Collection{}, fmt.Errorf("invalid frozen planning calendar")
	}
	for _, date := range []*string{calendar.LatestUpdateDate, calendar.InProgressSinceDate} {
		if date != nil && !validCalendarDate(*date) {
			return Collection{}, fmt.Errorf("invalid frozen planning date")
		}
	}
	issues := append([]IssueSignal(nil), in.Issues...)
	sort.Slice(issues, func(i, j int) bool { return issues[i].ID < issues[j].ID })
	categories, byStatus, observed := map[string]string{}, map[string]int64{}, map[string]string{}
	validCategories := StatusCategories(nil)
	for _, issue := range issues {
		if issue.Category != "" && validCategories[issue.Category] != issue.Category {
			return Collection{}, fmt.Errorf("invalid resolved issue category")
		}
		if prior, exists := observed[issue.Status]; exists && prior != issue.Category {
			return Collection{}, fmt.Errorf("inconsistent issue category")
		}
		observed[issue.Status] = issue.Category
		if issue.Category != "" {
			categories[issue.Status] = issue.Category
		}
		byStatus[issue.Status]++
	}
	counts := countScope(byStatus, categories)
	counts.Blocked, counts.Overdue, counts.Unassigned, counts.InReview, counts.RiskUnion, counts.ExecutionEnvironmentUnavailable = ptr(int64(0)), ptr(int64(0)), ptr(int64(0)), ptr(int64(0)), ptr(int64(0)), ptr(int64(0))
	out := Collection{Risks: map[string][]string{"blocked": {}, "overdue": {}, "unassigned": {}, "in_review": {}}, Categories: categories}
	seen := map[string]bool{}
	for _, issue := range issues {
		if seen[issue.ID] {
			return Collection{}, fmt.Errorf("duplicate formal issue %s", issue.ID)
		}
		seen[issue.ID] = true
		if issue.Category == "done" || issue.Category == "cancelled" {
			continue
		}
		hits := map[string]bool{"blocked": issue.Category == "blocked", "overdue": issue.DueDate != nil && *issue.DueDate < calendar.ReferenceDate, "unassigned": !issue.AssignmentValid, "in_review": issue.Category == "in_review"}
		risk := false
		for signal, hit := range hits {
			if hit {
				out.Risks[signal] = append(out.Risks[signal], issue.ID)
				risk = true
			}
		}
		if risk {
			*counts.RiskUnion++
		}
		if issue.AssignmentValid && issue.EnvironmentUnavailable {
			*counts.ExecutionEnvironmentUnavailable++
			out.EnvironmentUnavailableIDs = append(out.EnvironmentUnavailableIDs, issue.ID)
		}
	}
	*counts.Blocked = int64(len(out.Risks["blocked"]))
	*counts.Overdue = int64(len(out.Risks["overdue"]))
	*counts.Unassigned = int64(len(out.Risks["unassigned"]))
	*counts.InReview = int64(len(out.Risks["in_review"]))
	ended := in.ProjectStatus == "completed" || in.ProjectStatus == "cancelled"
	overdue := in.ProjectDueDate != nil && *in.ProjectDueDate < calendar.ReferenceDate && !ended
	snapshot := StatisticsSnapshot{ReferenceDate: calendar.ReferenceDate, Complete: *counts.UnknownStatus == 0, IncompleteReasons: []string{}, Counts: counts, ProjectOverdue: &overdue, LeadValid: &in.LeadValid, LatestUpdateAt: in.LatestUpdateAt, Reasons: []string{}, Health: "clear"}
	if *counts.Total > 0 {
		snapshot.ClosureRatio = ptr(float64(*counts.Completed+*counts.Cancelled) / float64(*counts.Total))
	}
	anchor := in.LatestUpdateAt
	if anchor != nil {
		age := calendarDateDays(*calendar.LatestUpdateDate, calendar.ReferenceDate)
		snapshot.ProgressAgeDays = &age
	} else {
		anchor = in.InProgressSince
	}
	anchorDate := calendar.LatestUpdateDate
	if in.LatestUpdateAt == nil {
		anchorDate = calendar.InProgressSinceDate
	}
	stale := in.ProjectStatus == "in_progress" && anchor != nil && anchorDate != nil && calendarDateDays(*anchorDate, calendar.ReferenceDate) >= 7
	for _, reason := range []struct {
		hit  bool
		code string
	}{
		{*counts.Blocked > 0, "blocked_issues"}, {*counts.Overdue > 0, "overdue_issues"}, {overdue, "project_overdue"}, {*counts.Unassigned > 0, "unassigned_issues"}, {*counts.InReview > 0, "in_review_issues"}, {!in.LeadValid, "invalid_project_lead"}, {stale, "stale_progress"}, {ended && *counts.Open > 0, "ended_project_open_issues"}, {*counts.ExecutionEnvironmentUnavailable > 0, "execution_environment_unavailable"},
	} {
		if reason.hit {
			snapshot.Reasons = append(snapshot.Reasons, reason.code)
		}
	}
	switch {
	case !snapshot.Complete:
		snapshot.Health = "unavailable"
		snapshot.IncompleteReasons = append(snapshot.IncompleteReasons, "unknown_status")
	case *counts.Total == 0:
		snapshot.Health = "empty"
	case *counts.Blocked > 0 || *counts.Overdue > 0 || overdue:
		snapshot.Health = "risk"
	case *counts.Unassigned > 0 || *counts.InReview > 0 || !in.LeadValid || stale:
		snapshot.Health = "attention"
	}
	out.Statistics = snapshot
	return out, nil
}
