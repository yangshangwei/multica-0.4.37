package reportsnapshot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/projecthealth"
)

func TestFreezeHealthPrivacyAndMetricParity(t *testing.T) {
	in := testInput()
	member := testID(5)
	update := in.AsOf.Add(-8 * 24 * time.Hour)
	h := projecthealth.Input{Project: projecthealth.Project{WorkspaceID: in.WorkspaceID, ID: *in.ProjectID, Revision: 1, Status: "in_progress", LeadType: "member", LeadID: member, DueDate: ptr("2026-11-12")}, Now: in.AsOf, Timezone: ptr("UTC"), References: []projecthealth.Reference{{Kind: "member", ID: member, Valid: true, Facts: map[string]any{"runtime_id": "hidden-runtime", "roster": "hidden-roster"}}}, LatestUpdateAt: &update}
	statuses := []string{"todo", "blocked", "done", "cancelled"}
	for i := range in.Facts.Issues {
		f := &in.Facts.Issues[i]
		f.Status = statuses[i]
		f.AssigneeType = "member"
		f.AssigneeID = member
		f.DueDate = ptr("2026-11-12")
		h.Issues = append(h.Issues, projecthealth.Issue{ID: f.ID, Status: f.Status, AssigneeType: member, AssigneeID: member, DueDate: f.DueDate})
	}
	// Keep the baseline exact same allowed input, including the current report.
	for i := range h.Issues {
		h.Issues[i].AssigneeType = "member"
	}
	baseline, e := projecthealth.Compute(h)
	if e != nil {
		t.Fatal(e)
	}
	frozen := FreezeHealth(h)
	raw, _ := CanonicalJSON(frozen)
	if strings.Contains(string(raw), "hidden-") || strings.Contains(string(raw), "runtime_id") || strings.Contains(string(raw), `"WorkspaceID"`) {
		t.Fatalf("private or unstable JSON: %s", raw)
	}
	in.Facts.Health = &frozen
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	counts := baseline.Statistics.Counts
	for name, value := range map[string]*int64{"total": counts.Total, "completed": counts.Completed, "cancelled": counts.Cancelled, "open": counts.Open, "blocked": counts.Blocked, "overdue": counts.Overdue, "unassigned": counts.Unassigned, "in_review": counts.InReview, "risk_union": counts.RiskUnion, "execution_environment_unavailable": counts.ExecutionEnvironmentUnavailable} {
		integer(t, metric(t, s, "formal_stock."+name), *value)
	}
	for id, want := range map[string]any{"project.overdue": baseline.Statistics.ProjectOverdue, "project.lead_valid": baseline.Statistics.LeadValid, "project.health": baseline.Statistics.Health, "project.latest_update_at": baseline.Statistics.LatestUpdateAt, "project.progress_age_days": baseline.Statistics.ProgressAgeDays} {
		m := metric(t, s, id)
		expected, _ := json.Marshal(want)
		if string(m.Value) != string(expected) || m.Quality != "exact" {
			t.Fatalf("%s = %s/%s want %s", id, m.Value, m.Quality, expected)
		}
	}
	if m := metric(t, s, "project.latest_update_at"); m.Coverage.Unit != "required_dependency" || *m.Coverage.Eligible != 1 {
		t.Fatal("project timestamp used issue denominator")
	}
	// Nil from an exhaustive empty update source is exact absence, not unknown.
	in.Facts.Health.LatestUpdateAt = nil
	s, e = Build(in)
	if e != nil {
		t.Fatal(e)
	}
	m := metric(t, s, "project.latest_update_at")
	if m.Quality != "exact" || string(m.Value) != "null" {
		t.Fatal("exact absence lost")
	}
}
