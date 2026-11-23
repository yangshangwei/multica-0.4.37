package reportsnapshot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/projecthealth"
)

const publicSourceVersion = "public-source-facts-v1"

func boolValue(v *bool) bool { return v != nil && *v }

func publicInputFromInternal(in Input) PublicInput {
	out := PublicInput{SnapshotID: in.SnapshotID, WorkspaceID: in.WorkspaceID, ReportIssueID: in.ReportIssueID, ProjectID: copyString(in.ProjectID), ReportRevision: in.ReportRevision, CorrectionOf: copyString(in.CorrectionOf), Window: in.Window, AsOf: in.AsOf, TimeBasis: in.TimeBasis, Facts: PublicFacts{Issues: []PublicIssueFact{}, Activities: []PublicActivity{}, Sources: []PublicSource{}, Gaps: []Gap{}}}
	refs := map[string]ReferenceFact{}
	statuses := []projecthealth.Status{}
	refComplete, historyComplete := false, false
	for _, source := range in.Facts.Sources {
		if source.Kind == "references" {
			refComplete = source.ReadComplete
		}
		if source.Kind == "activities" {
			historyComplete = source.ReadComplete && source.ProducerComplete
		}
		visibility := "unavailable"
		if source.Watermark != nil {
			visibility = "internal"
		}
		out.Facts.Sources = append(out.Facts.Sources, PublicSource{Kind: source.Kind, Version: publicSourceVersion, ReadComplete: source.ReadComplete, ProducerComplete: source.ProducerComplete, ProofVisibility: visibility})
	}
	if h := in.Facts.Health; h != nil {
		for _, ref := range h.References {
			refs[ref.Kind+":"+ref.ID] = ref
		}
		for _, status := range h.Statuses {
			statuses = append(statuses, projecthealth.Status{Key: status.Key, Category: status.Category})
		}
		p := h.Project
		out.Facts.Project = &PublicProjectFact{ID: p.ID, Revision: p.Revision, Status: p.Status, DueDate: copyString(p.DueDate), InProgressSince: copyTime(p.InProgressSince), LatestUpdateAt: copyTime(h.LatestUpdateAt)}
		if refComplete {
			out.Facts.Project.LeadValid = ptr(p.LeadType != "" && p.LeadID != "" && refs[p.LeadType+":"+p.LeadID].Valid)
		}
	}
	categories := projecthealth.StatusCategories(statuses)
	for _, issue := range in.Facts.Issues {
		p := PublicIssueFact{ID: issue.ID, Revision: issue.Revision, ProjectID: copyString(issue.ProjectID), Status: issue.Status, Category: categories[issue.Status], AdmissionStatus: issue.AdmissionStatus, CreatedAt: issue.CreatedAt, DueDate: copyString(issue.DueDate), Classification: issue.Classification, ClassificationReason: publicClassificationReason(issue), CreationProjectKnown: issue.CreationProjectKnown, HistoryComplete: issue.HistoryComplete, BaselineAt: copyTime(issue.BaselineAt), BaselineStatus: issue.BaselineStatus, BaselineCategory: issue.BaselineCategory}
		if in.Facts.Health == nil {
			p.Category = ""
		}
		if refComplete && in.Facts.Health != nil {
			ref := refs[issue.AssigneeType+":"+issue.AssigneeID]
			valid := issue.AssigneeType != "" && issue.AssigneeID != "" && ref.Valid
			p.AssignmentValid, p.EnvironmentUnavailable = &valid, ptr(valid && ref.EnvironmentUnavailable)
		}
		// An incomplete or unprovable event feed is not made public merely to
		// explain an unknown metric. The boolean still preserves its exact gap.
		acts := []ActivityFact{}
		p.HistoryComplete = p.HistoryComplete && historyComplete
		if p.HistoryComplete {
			for _, activity := range in.Facts.Activities {
				if activity.IssueID != issue.ID || activity.Action != "status_changed" {
					continue
				}
				if activity.BusinessAt == nil || activity.Sequence == nil || !activity.ProjectKnown || !sameID(activity.ProjectID, in.ProjectID) || !validCategory(activity.FromCategory) || !validCategory(activity.ToCategory) {
					p.HistoryComplete = false
					break
				}
				acts = append(acts, activity)
			}
		}
		if p.HistoryComplete {
			sort.Slice(acts, func(i, j int) bool { return *acts[i].Sequence < *acts[j].Sequence })
			for _, activity := range acts {
				out.Facts.Activities = append(out.Facts.Activities, PublicActivity{FactID: fmt.Sprintf("event-%d", len(out.Facts.Activities)+1), IssueID: issue.ID, BusinessAt: *activity.BusinessAt, Sequence: *activity.Sequence, FromStatus: activity.FromStatus, ToStatus: activity.ToStatus, FromCategory: activity.FromCategory, ToCategory: activity.ToCategory, ProjectID: copyString(activity.ProjectID)})
			}
		}
		out.Facts.Issues = append(out.Facts.Issues, p)
	}
	for _, gap := range in.Facts.Gaps {
		gap.Code = publicGapCode(gap.Code)
		if len(gap.MetricIDs) > 0 {
			metrics := []string{}
			for _, id := range gap.MetricIDs {
				if publicMetricID(id) {
					metrics = append(metrics, id)
				}
			}
			if len(metrics) == 0 {
				continue
			}
			gap.MetricIDs = metrics
			if gap.Source != "" && !publicGapSource(gap.Source) {
				gap.Source = ""
			}
		} else if gap.Source != "" && !publicGapSource(gap.Source) {
			continue
		}
		out.Facts.Gaps = append(out.Facts.Gaps, gap)
	}
	context := metricContext{input: out, sources: map[string]PublicSource{}}
	for _, source := range out.Facts.Sources {
		context.sources[source.Kind] = source
	}
	needed := map[string]bool{}
	for i := range out.Facts.Issues {
		issue := &out.Facts.Issues[i]
		// Metric-specific gaps remain attached to their metrics. This probe only
		// decides whether the common lifecycle facts can be proved at all.
		known := isFormal(*issue) && issue.Classification == "business" && context.lifecycle(*issue, "public_projection").known
		issue.HistoryComplete = known
		if known {
			needed[issue.ID] = true
		} else {
			issue.BaselineAt, issue.BaselineStatus, issue.BaselineCategory = nil, "", ""
		}
	}
	events := []PublicActivity{}
	for _, event := range out.Facts.Activities {
		if needed[event.IssueID] {
			event.FactID = fmt.Sprintf("event-%d", len(events)+1)
			events = append(events, event)
		}
	}
	out.Facts.Activities = events
	retentionProved := true
	for _, issue := range out.Facts.Issues {
		if isFormal(issue) && issue.Classification != "report" && !needed[issue.ID] {
			retentionProved = false
		}
	}
	for i := range out.Facts.Sources {
		if out.Facts.Sources[i].Kind == "activities" && retentionProved {
			out.Facts.Sources[i].RetentionComplete = ptr(true)
		}
	}
	return out
}

func publicClassificationReason(issue IssueFact) string {
	if issue.ClassificationReason == "current_report" {
		return "current_report"
	}
	switch issue.Classification {
	case "report":
		if strings.HasPrefix(issue.ClassificationReason, "builtin_") {
			return "builtin_report"
		}
		return "verified_report"
	case "business":
		return "verified_business"
	default:
		return "classification_unproven"
	}
}

func publicGapCode(code string) string {
	switch code {
	case "source_unavailable", "scan_limit", "producer_history_unproven", "source_count_mismatch", "cohort_incomplete":
		return code
	default:
		return "source_evidence_unavailable"
	}
}

func publicCalendar(in PublicInput) projecthealth.CalendarFacts {
	out := projecthealth.CalendarFacts{ReferenceDate: in.TimeBasis.ReferenceDate}
	if p := in.Facts.Project; p != nil {
		if p.LatestUpdateAt != nil && in.TimeBasis.LatestUpdateOffsetSeconds != nil {
			out.LatestUpdateDate = ptr(localClock(*p.LatestUpdateAt, *in.TimeBasis.LatestUpdateOffsetSeconds).Format("2006-01-02"))
		}
		if p.InProgressSince != nil && in.TimeBasis.InProgressSinceOffsetSeconds != nil {
			out.InProgressSinceDate = ptr(localClock(*p.InProgressSince, *in.TimeBasis.InProgressSinceOffsetSeconds).Format("2006-01-02"))
		}
	}
	return out
}
