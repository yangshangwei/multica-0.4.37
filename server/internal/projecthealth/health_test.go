package projecthealth

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func sampleInput() Input {
	return Input{Project: Project{WorkspaceID: "ws", ID: "project", Revision: 1, Status: "planned", LeadType: "member", LeadID: "member"}, Now: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC), References: []Reference{{Kind: "member", ID: "member", Valid: true}}}
}
func issue(id, status string) Issue {
	return Issue{ID: id, Status: status, AssigneeType: "member", AssigneeID: "member"}
}
func str(v string) *string { return &v }
func checkCount(t *testing.T, name string, got *int64, want int64) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s: got %v, want %d", name, got, want)
	}
}

func TestGoldenCompletionCancellationAndEmpty(t *testing.T) {
	in := sampleInput()
	for n, s := range []string{"done", "done", "done", "done", "done", "done", "cancelled", "cancelled", "todo", "todo"} {
		in.Issues = append(in.Issues, issue(string(rune('a'+n)), s))
	}
	out, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	s := out.Statistics
	checkCount(t, "total", s.Counts.Total, 10)
	checkCount(t, "completed", s.Counts.Completed, 6)
	checkCount(t, "cancelled", s.Counts.Cancelled, 2)
	checkCount(t, "open", s.Counts.Open, 2)
	if s.ClosureRatio == nil || *s.ClosureRatio != 0.8 || s.Health != "clear" || !s.Complete {
		t.Fatalf("golden: %+v", s)
	}
	in.Issues = nil
	in.Project.DueDate = str("2026-03-09")
	out, err = Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Statistics.ClosureRatio != nil || out.Statistics.Health != "empty" || !*out.Statistics.ProjectOverdue {
		t.Fatalf("empty project hides overdue or invents percentage: %+v", out)
	}
	in.Issues = []Issue{issue("a", "cancelled"), issue("b", "cancelled")}
	out, err = Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	checkCount(t, "cancelled only", out.Statistics.Counts.Completed, 0)
	if *out.Statistics.ClosureRatio != 1 || in.Project.Status != "planned" {
		t.Fatalf("cancelled scope: %+v", out)
	}
}

func TestRiskOverlapUnknownAndArchivedCategory(t *testing.T) {
	in := sampleInput()
	in.Statuses = []Status{{Key: "waiting_customer", Category: "in_review"}}
	overlap := issue("a", "blocked")
	overlap.DueDate = str("2026-03-09")
	overlap.AssigneeID = "departed"
	today := issue("b", "todo")
	today.DueDate = str("2026-03-10")
	done := issue("c", "done")
	done.DueDate = str("2026-03-08")
	in.Issues = []Issue{overlap, today, done, issue("d", "waiting_customer")}
	out, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]*int64{"blocked": out.Statistics.Counts.Blocked, "overdue": out.Statistics.Counts.Overdue, "unassigned": out.Statistics.Counts.Unassigned, "review": out.Statistics.Counts.InReview} {
		checkCount(t, name, got, 1)
	}
	checkCount(t, "union", out.Statistics.Counts.RiskUnion, 2)
	for _, signal := range []string{"blocked", "overdue", "unassigned"} {
		if strings.Join(out.Risks[signal], ",") != "a" {
			t.Fatalf("%s: %v", signal, out.Risks)
		}
	}
	in.Issues = append(in.Issues, issue("e", "future_unknown"))
	out, err = Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	checkCount(t, "unknown retained", out.Statistics.Counts.Total, 5)
	checkCount(t, "unknown open", out.Statistics.Counts.Open, 4)
	checkCount(t, "unknown", out.Statistics.Counts.UnknownStatus, 1)
	if out.Statistics.Complete || out.Statistics.Health != "unavailable" {
		t.Fatalf("unknown presented as healthy: %+v", out)
	}
}

func TestCalendarDaysAcrossDSTAndLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name, now, since string
		wantAge          int
		stale            bool
	}{
		{"spring seven dates less than 168h", "2026-03-10T12:00:00-04:00", "2026-03-03T12:00:00-05:00", 7, true},
		{"spring six dates", "2026-03-09T12:00:00-04:00", "2026-03-03T12:00:00-05:00", 6, false},
		{"fall seven dates more than 168h", "2026-11-03T12:00:00-05:00", "2026-10-27T12:00:00-04:00", 7, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := sampleInput()
			in.Timezone = str("America/New_York")
			in.Now, _ = time.Parse(time.RFC3339, tc.now)
			since, _ := time.Parse(time.RFC3339, tc.since)
			in.Project.InProgressSince = &since
			in.Project.Status = "in_progress"
			in.Issues = []Issue{issue("a", "todo")}
			out, err := Compute(in)
			if err != nil {
				t.Fatal(err)
			}
			if out.Statistics.ProgressAgeDays != nil || (out.Statistics.Health == "attention") != tc.stale {
				t.Fatalf("unpublished clock: %+v", out.Statistics)
			}
			in.LatestUpdateAt = &since
			out, err = Compute(in)
			if err != nil {
				t.Fatal(err)
			}
			if out.Statistics.ProgressAgeDays == nil || *out.Statistics.ProgressAgeDays != tc.wantAge {
				t.Fatalf("calendar age: %+v", out.Statistics)
			}
			in.Project.Status = "completed"
			in.Project.DueDate = str("2026-01-01")
			out, err = Compute(in)
			if err != nil {
				t.Fatal(err)
			}
			if *out.Statistics.ProjectOverdue || out.Statistics.Health != "clear" || !slices.Contains(out.Statistics.Reasons, "ended_project_open_issues") {
				t.Fatalf("independent project state: %+v", out.Statistics)
			}
		})
	}
}

func TestDigestUsesInputsAndExcludesCalculationClock(t *testing.T) {
	in := sampleInput()
	in.Issues = []Issue{issue("a", "todo"), issue("b", "done")}
	a, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	in.Now = in.Now.Add(time.Hour)
	in.Issues[0], in.Issues[1] = in.Issues[1], in.Issues[0]
	b, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Statistics.SnapshotVersion != b.Statistics.SnapshotVersion {
		t.Fatal("order or calculation time changed version")
	}
	in.Issues[0].ID = "c"
	c, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Statistics.SnapshotVersion == c.Statistics.SnapshotVersion {
		t.Fatal("equal totals hid changed member identity")
	}
	in.Timezone = str("UTC")
	d, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if c.Statistics.SnapshotVersion == d.Statistics.SnapshotVersion {
		t.Fatal("explicit timezone configuration not represented")
	}
	data, err := json.Marshal(d.Statistics)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"statistics_snapshot", "latest_acceptance", "description_snapshot", "current_revision"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("recursive snapshot: %s", data)
		}
	}
}

func TestOfflineReferencesRemainAssigned(t *testing.T) {
	in := sampleInput()
	in.References = append(in.References, Reference{Kind: "agent", ID: "offline", Valid: true, EnvironmentUnavailable: true})
	a := issue("a", "todo")
	a.AssigneeType = "agent"
	a.AssigneeID = "offline"
	in.Issues = []Issue{a}
	out, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	checkCount(t, "assigned offline", out.Statistics.Counts.Unassigned, 0)
	checkCount(t, "environment", out.Statistics.Counts.ExecutionEnvironmentUnavailable, 1)
}

func TestCanonicalJSONPreservesLargeIntegersAndSortsNestedKeys(t *testing.T) {
	got, err := CanonicalJSON(map[string]any{"z": int64(9007199254740993), "a": map[string]any{"z": 2, "a": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"a\":{\"a\":1,\"z\":2},\"z\":9007199254740993}" {
		t.Fatalf("canonical: %s", got)
	}
}

func TestTimezoneDayAndSemanticInputChanges(t *testing.T) {
	in := sampleInput()
	in.Now = time.Date(2026, 3, 9, 18, 0, 0, 0, time.UTC)
	in.Timezone = str("Asia/Shanghai")
	task := issue("a", "todo")
	task.DueDate = str("2026-03-09")
	in.Issues = []Issue{task}
	original, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	if original.Statistics.ReferenceDate != "2026-03-10" {
		t.Fatalf("workspace day: %s", original.Statistics.ReferenceDate)
	}
	checkCount(t, "workspace overdue", original.Statistics.Counts.Overdue, 1)
	for _, mutate := range []func(*Input){
		func(v *Input) { v.Issues[0].DueDate = str("2026-03-08") },
		func(v *Input) { v.Issues[0].AssigneeID = "another-member" },
		func(v *Input) {
			v.References[0].Facts = map[string]any{"member_exists": true, "membership_generation": 2}
		},
		func(v *Input) { v.Project.Revision++ },
		func(v *Input) { v.Statuses = []Status{{Key: "custom", Category: "done"}} },
	} {
		candidate := in
		candidate.Issues = append([]Issue(nil), in.Issues...)
		candidate.References = append([]Reference(nil), in.References...)
		mutate(&candidate)
		changed, e := Compute(candidate)
		if e != nil {
			t.Fatal(e)
		}
		if original.Statistics.SnapshotVersion == changed.Statistics.SnapshotVersion {
			t.Fatal("underlying input change kept old version")
		}
	}
}

func TestPlanningTimezoneDefaultsAndShanghaiMidnight(t *testing.T) {
	for _, tc := range []struct {
		name     string
		zone     *string
		now      string
		wantZone string
		wantDay  string
		wantAge  int
		overdue  int64
	}{
		{"default before midnight", nil, "2026-03-09T15:59:59Z", "Asia/Shanghai", "2026-03-09", 6, 0},
		{"default at midnight", nil, "2026-03-09T16:00:00Z", "Asia/Shanghai", "2026-03-10", 7, 1},
		{"explicit UTC", str("UTC"), "2026-03-09T16:00:00Z", "UTC", "2026-03-09", 6, 0},
		{"explicit New York", str("America/New_York"), "2026-03-09T16:00:00Z", "America/New_York", "2026-03-09", 6, 0},
		{"explicit Shanghai", str("Asia/Shanghai"), "2026-03-09T16:00:00Z", "Asia/Shanghai", "2026-03-10", 7, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := sampleInput()
			in.Timezone = tc.zone
			var err error
			in.Now, err = time.Parse(time.RFC3339, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			in.Project.Status = "in_progress"
			in.Project.DueDate = str("2026-03-09")
			since := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
			in.LatestUpdateAt = &since
			due := issue("a", "todo")
			due.DueDate = str("2026-03-09")
			in.Issues = []Issue{due}
			out, err := Compute(in)
			if err != nil {
				t.Fatal(err)
			}
			s := out.Statistics
			if s.Timezone != tc.wantZone || s.TimezoneConfigured != (tc.zone != nil) || s.ReferenceDate != tc.wantDay {
				t.Fatalf("planning timezone: %+v", s)
			}
			checkCount(t, "overdue", s.Counts.Overdue, tc.overdue)
			if s.ProjectOverdue == nil || *s.ProjectOverdue != (tc.overdue == 1) || s.ProgressAgeDays == nil || *s.ProgressAgeDays != tc.wantAge || slices.Contains(s.Reasons, "stale_progress") != (tc.wantAge >= 7) {
				t.Fatalf("planning calendar boundaries: %+v", s)
			}
		})
	}
}

func TestDefaultTimezoneInvalidatesLegacyUTCSnapshot(t *testing.T) {
	updated := time.Date(2026, 3, 3, 18, 0, 0, 0, time.UTC)
	in := Input{
		Project:        Project{WorkspaceID: "ws", ID: "project", Revision: 1, Status: "in_progress"},
		Now:            time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC),
		LatestUpdateAt: &updated,
	}
	out, err := Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	// The UTC-default implementation used this version for the same input and
	// reference date, but counted seven calendar days since the last update.
	const legacyUTCVersion = "d90e6cc71a95fb8b4c9a5887bc0da793d0234fabd8b5d5b23ecf12a474f24af8"
	if out.Statistics.SnapshotVersion == legacyUTCVersion {
		t.Fatal("changed default timezone retained an outdated preview version")
	}
	if out.Statistics.ReferenceDate != "2026-03-10" || out.Statistics.ProgressAgeDays == nil || *out.Statistics.ProgressAgeDays != 6 {
		t.Fatalf("unexpected Shanghai calendar: %+v", out.Statistics)
	}
}

func TestBatchScopeCountsMatchHealthClassification(t *testing.T) {
	categories := statusCategories([]Status{{Key: "delivered", Category: "done"}, {Key: "abandoned", Category: "cancelled"}})
	counts := countScope(map[string]int64{"delivered": 6, "abandoned": 2, "todo": 1, "future": 1}, categories)
	checkCount(t, "total", counts.Total, 10)
	checkCount(t, "done", counts.Completed, 6)
	checkCount(t, "cancelled", counts.Cancelled, 2)
	checkCount(t, "open", counts.Open, 2)
	checkCount(t, "unknown", counts.UnknownStatus, 1)
	if counts.Blocked != nil || counts.Overdue != nil {
		t.Fatal("count-only projection invented uncollected risk values")
	}
	empty := countScope(nil, categories)
	checkCount(t, "empty scope", empty.Total, 0)
}
