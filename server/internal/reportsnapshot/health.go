package reportsnapshot

import (
	"time"

	"github.com/multica-ai/multica/server/internal/projecthealth"
)

// FreezeHealth copies only the fields needed by the rule owner. Internal
// runtime IDs, roster members, labels and arbitrary Reference.Facts are omitted.
func FreezeHealth(in projecthealth.Input) HealthFacts {
	p := in.Project
	out := HealthFacts{Project: ProjectFact{WorkspaceID: p.WorkspaceID, ID: p.ID, Revision: p.Revision, Status: p.Status, DueDate: copyString(p.DueDate), LeadType: p.LeadType, LeadID: p.LeadID, InProgressSince: copyTime(p.InProgressSince), InProgressSinceSource: p.InProgressSinceSource}, LatestUpdateAt: copyTime(in.LatestUpdateAt), Statuses: []StatusFact{}, References: []ReferenceFact{}}
	for _, s := range in.Statuses {
		out.Statuses = append(out.Statuses, StatusFact{Key: s.Key, Category: s.Category})
	}
	for _, r := range in.References {
		out.References = append(out.References, ReferenceFact{Kind: r.Kind, ID: r.ID, Valid: r.Valid, EnvironmentUnavailable: r.EnvironmentUnavailable})
	}
	return out
}

func copyString(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}
func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}
