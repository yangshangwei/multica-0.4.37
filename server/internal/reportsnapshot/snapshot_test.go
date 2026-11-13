package reportsnapshot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testID(n int) string {
	return "00000000-0000-4000-8000-" + strings.Repeat("0", 11) + string(rune('0'+n))
}
func at(s string) time.Time {
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil {
		panic(e)
	}
	return t
}
func testInput() Input {
	p := testID(2)
	count := int64(4)
	in := Input{Identity: Identity{SnapshotID: testID(1), WorkspaceID: testID(3), ReportIssueID: testID(4), ProjectID: &p, ReportRevision: 1, Sequence: 1, CreatedBy: testID(5), RequestKey: testID(6)}, Window: Window{Start: at("2026-11-12T00:00:00Z"), End: at("2026-11-13T00:00:00Z"), Timezone: "UTC", PeriodMode: "explicit"}, AsOf: at("2026-11-13T01:00:00Z"), ReadCut: "test-cut"}
	in.Facts.Health = &HealthFacts{Project: ProjectFact{WorkspaceID: in.WorkspaceID, ID: p, Revision: 1, Status: "in_progress"}}
	for _, id := range []string{testID(4), testID(7), testID(8), testID(9)} {
		in.Facts.Issues = append(in.Facts.Issues, IssueFact{ID: id, Revision: 1, ProjectID: &p, Status: "todo", AdmissionStatus: "not_required", CreatedAt: at("2026-11-01T00:00:00Z"), Classification: "business", ClassificationReason: "verified-fixture", CreationProjectKnown: true, HistoryComplete: true, BaselineAt: ptr(at("2026-11-11T00:00:00Z")), BaselineStatus: "todo", BaselineCategory: "todo"})
	}
	for _, kind := range []string{"issues", "status_catalog", "references", "project", "updates", "activities"} {
		in.Facts.Sources = append(in.Facts.Sources, SourceFact{Kind: kind, Version: "fixture-v1", Ordering: "id", ReadComplete: true, ProducerComplete: true})
	}
	in.Facts.Sources[0].Count = &count
	return in
}
func metric(t *testing.T, s Snapshot, id string) Metric {
	t.Helper()
	for _, m := range s.Metrics {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("missing metric %s", id)
	return Metric{}
}
func integer(t *testing.T, m Metric, n int64) {
	t.Helper()
	var got int64
	if e := json.Unmarshal(m.Value, &got); e != nil || got != n || m.Quality != "exact" {
		t.Fatalf("%s = %s/%s, want exact %d", m.ID, m.Value, m.Quality, n)
	}
}

func TestSnapshotLifecycleAndHalfOpenWindow(t *testing.T) {
	in := testInput()
	a, b, c := testID(7), testID(8), testID(9)
	for i := range in.Facts.Issues {
		switch in.Facts.Issues[i].ID {
		case a:
			in.Facts.Issues[i].Status = "done"
		case b:
			in.Facts.Issues[i].Status = "cancelled"
		case c:
			in.Facts.Issues[i].CreatedAt = in.Window.Start
			in.Facts.Issues[i].BaselineAt = ptr(in.Window.Start)
			in.Facts.Issues[i].Status = "done"
		}
	}
	add := func(id, issue, from, to string, seq int64, when string) {
		business := at(when)
		in.Facts.Activities = append(in.Facts.Activities, ActivityFact{ID: id, IssueID: issue, Action: "status_changed", RecordedAt: business, BusinessAt: &business, Sequence: &seq, FromStatus: from, ToStatus: to, FromCategory: from, ToCategory: to, ProjectID: in.ProjectID, ProjectKnown: true, SourceVersion: "fixture-v1"})
	}
	add("00000000-0000-4000-9000-000000000001", a, "todo", "done", 1, "2026-11-12T01:00:00Z")
	add("00000000-0000-4000-9000-000000000002", a, "done", "todo", 2, "2026-11-12T02:00:00Z")
	add("00000000-0000-4000-9000-000000000003", a, "todo", "done", 3, "2026-11-12T03:00:00Z")
	add("00000000-0000-4000-9000-000000000004", b, "todo", "cancelled", 1, "2026-11-12T01:00:00Z")
	add("00000000-0000-4000-9000-000000000005", c, "todo", "done", 1, "2026-11-13T00:00:00Z")
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	for id, n := range map[string]int64{"stock.total": 3, "window.created": 1, "window.completed": 1, "window.cancelled": 1, "window.closed": 2, "window.reopened": 1, "window.net_created_minus_closed": -1} {
		integer(t, metric(t, s, id), n)
	}
	if got := metric(t, s, "window.closed").VerifiedIDs; len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("closed IDs %v", got)
	}
}

func TestSnapshotUnknownIsNotZeroOrFalse(t *testing.T) {
	in := testInput()
	in.Facts.Issues[1].Classification = "unknown"
	in.Facts.Issues[2].HistoryComplete = false
	for i := range in.Facts.Sources {
		if in.Facts.Sources[i].Kind == "updates" {
			in.Facts.Sources[i].ReadComplete = false
		}
	}
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	m := metric(t, s, "stock.total")
	if string(m.Value) != "null" || m.Coverage.Unknown.Count == nil || *m.Coverage.Unknown.Count != 1 {
		t.Fatalf("unknown total %+v", m)
	}
	if m = metric(t, s, "project.latest_update_at"); m.Quality != "unknown" || string(m.Value) != "null" || m.Coverage.Unit != "required_dependency" {
		t.Fatalf("unknown project time %+v", m)
	}
	if m = metric(t, s, "project.overdue"); m.Quality != "exact" || string(m.Value) != "false" {
		t.Fatalf("independent project boolean %+v", m)
	}
	if s.Complete {
		t.Fatal("unknown source marked complete")
	}
}

func TestSnapshotDigestsAndFrozenReplay(t *testing.T) {
	in := testInput()
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	in.SnapshotID = "00000000-0000-4000-8000-000000000011"
	in.Sequence = 2
	for i, j := 0, len(in.Facts.Issues)-1; i < j; i, j = i+1, j-1 {
		in.Facts.Issues[i], in.Facts.Issues[j] = in.Facts.Issues[j], in.Facts.Issues[i]
	}
	in.Facts.Issues = append(in.Facts.Issues, in.Facts.Issues[0])
	other, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	if other.FactsDigest != s.FactsDigest || other.ResultDigest != s.ResultDigest || other.SnapshotDigest == s.SnapshotDigest {
		t.Fatal("canonical identity/ordering boundary not preserved")
	}
	raw, e := CanonicalJSON(s)
	if e != nil {
		t.Fatal(e)
	}
	read, e := Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	if read.SnapshotDigest != s.SnapshotDigest {
		t.Fatal("replay changed digest")
	}
	s.Metrics[0].Value = json.RawMessage("999")
	if e := Validate(s); e == nil {
		t.Fatal("tampered result accepted")
	}
	if _, e := Decode(append(raw, []byte(" {}")...)); e == nil {
		t.Fatal("trailing JSON accepted")
	}
	duplicate := strings.Replace(string(raw), `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1)
	if _, e := Decode([]byte(duplicate)); e == nil {
		t.Fatal("duplicate key accepted")
	}
}

func TestSnapshotDSTWindowAndSavedOffsets(t *testing.T) {
	in := testInput()
	in.Window = Window{Start: at("2026-03-08T05:00:00Z"), End: at("2026-03-09T04:00:00Z"), Timezone: "America/New_York", PeriodMode: "calendar_day"}
	in.AsOf = at("2026-03-09T05:00:00Z")
	for i := range in.Facts.Issues {
		in.Facts.Issues[i].CreatedAt = at("2026-03-01T00:00:00Z")
		in.Facts.Issues[i].BaselineAt = ptr(at("2026-03-07T00:00:00Z"))
	}
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	if s.TimeBasis.StartOffsetSeconds != -18000 || s.TimeBasis.EndOffsetSeconds != -14400 {
		t.Fatalf("offsets %+v", s.TimeBasis)
	}
	// Once offsets are resolved, validation uses retained civil-time facts.
	s.Window.Timezone = "Historic/ZoneRemovedFromCurrentTzdb"
	replay, e := Build(s.Input)
	if e != nil {
		t.Fatalf("saved offsets reparsed against current tzdb: %v", e)
	}
	if replay.ReferenceDate != s.ReferenceDate {
		t.Fatal("reference date changed")
	}
	in.Window.PeriodMode = "rolling_24h"
	if _, e = Build(in); e == nil {
		t.Fatal("23h calendar day accepted as rolling24h")
	}
}

func TestSnapshotNoProjectAndMissingSource(t *testing.T) {
	in := testInput()
	in.ProjectID = nil
	in.Facts.Health = nil
	in.Facts.Issues = in.Facts.Issues[:1]
	in.Facts.Issues[0].ProjectID = nil
	in.Facts.Sources = nil
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	if m := metric(t, s, "stock.total"); m.Quality != "not_applicable" || string(m.Value) != "null" {
		t.Fatalf("no project projected as zero %+v", m)
	}
	in = testInput()
	in.Facts.Sources = nil
	s, e = Build(in)
	if e != nil {
		t.Fatal(e)
	}
	m := metric(t, s, "stock.total")
	if m.Coverage.Eligible != nil || m.Coverage.Unknown.Count != nil || !m.Coverage.Unknown.ExtentUnknown || string(m.Value) != "null" {
		t.Fatalf("missing source became complete %+v", m)
	}
}

func TestSnapshotSourceGapAndPayloadIdentity(t *testing.T) {
	in := testInput()
	first, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range first.Facts.Sources {
		if len(s.ContentDigest) != 64 {
			t.Fatalf("missing source identity %+v", s)
		}
	}
	for i := range in.Facts.Sources {
		if in.Facts.Sources[i].Kind == "issues" {
			in.Facts.Sources[i].ReadComplete = false
		}
	}
	partial, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	m := metric(t, partial, "formal_stock.total")
	if string(m.Value) != "null" || m.Coverage.Eligible != nil || m.Coverage.Unknown.Count != nil || m.Coverage.Unknown.KnownCount != 0 || !m.Coverage.Unknown.ExtentUnknown {
		t.Fatalf("unbounded tail %+v", m)
	}
	var oldDigest, newDigest string
	for _, s := range first.Facts.Sources {
		if s.Kind == "issues" {
			oldDigest = s.ContentDigest
		}
	}
	for _, s := range partial.Facts.Sources {
		if s.Kind == "issues" {
			newDigest = s.ContentDigest
		}
	}
	if oldDigest == newDigest {
		t.Fatal("source completeness change not bound to source digest")
	}
	// Read-complete source count mismatch becomes a gap, not a complete count.
	in = testInput()
	for i := range in.Facts.Sources {
		if in.Facts.Sources[i].Kind == "issues" {
			in.Facts.Sources[i].Count = ptr(int64(5))
		}
	}
	mismatch, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	if mismatch.Complete || len(mismatch.Facts.Gaps) == 0 || metric(t, mismatch, "stock.total").Quality == "exact" {
		t.Fatal("source count mismatch hidden")
	}
}

func TestSnapshotProductionHistoryAndClassificationRemainUnknown(t *testing.T) {
	in := testInput()
	for i := range in.Facts.Issues {
		if in.Facts.Issues[i].ID != in.ReportIssueID {
			in.Facts.Issues[i].Classification = "unknown"
			in.Facts.Issues[i].HistoryComplete = false
			in.Facts.Issues[i].CreationProjectKnown = false
		}
	}
	for i := range in.Facts.Sources {
		if in.Facts.Sources[i].Kind == "activities" {
			in.Facts.Sources[i].ProducerComplete = false
		}
	}
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	integer(t, metric(t, s, "formal_stock.total"), 4)
	for _, id := range []string{"stock.total", "window.created", "window.closed", "window.reopened"} {
		m := metric(t, s, id)
		if m.Quality != "unknown" || string(m.Value) != "null" || m.Coverage.Unknown.KnownCount != 3 {
			t.Fatalf("%s overclaims %+v", id, m)
		}
	}
}

func TestSnapshotTerminalSwitchDoesNotCloseAgain(t *testing.T) {
	in := testInput()
	issue := &in.Facts.Issues[1]
	issue.Status = "cancelled"
	issue.BaselineStatus = "done"
	issue.BaselineCategory = "done"
	when := in.Window.Start.Add(time.Hour)
	seq := int64(1)
	in.Facts.Activities = []ActivityFact{{ID: "00000000-0000-4000-9000-000000000001", IssueID: issue.ID, Action: "status_changed", RecordedAt: when, BusinessAt: &when, Sequence: &seq, FromStatus: "done", ToStatus: "cancelled", FromCategory: "done", ToCategory: "cancelled", ProjectID: in.ProjectID, ProjectKnown: true, SourceVersion: "fixture-v1"}}
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	integer(t, metric(t, s, "window.closed"), 0)
	integer(t, metric(t, s, "window.cancelled"), 1)
	in.Facts.Activities[0].Sequence = nil
	s, e = Build(in)
	if e != nil {
		t.Fatal(e)
	}
	if m := metric(t, s, "window.closed"); m.Quality == "exact" {
		t.Fatal("timestamp used as causal ordering")
	}
}

func TestSnapshotConflictsAndMalformedSealsRefuse(t *testing.T) {
	in := testInput()
	conflict := in.Facts.Issues[1]
	conflict.Status = "done"
	in.Facts.Issues = append(in.Facts.Issues, conflict)
	if _, e := Build(in); e == nil {
		t.Fatal("same identity revision with conflicting payload accepted")
	}
	in = testInput()
	in.ProjectID = ptr(testID(1))
	if _, e := Build(in); e == nil {
		t.Fatal("cross-project input accepted")
	}
	in = testInput()
	in.Facts.Issues[1].AdmissionStatus = "future_unknown"
	if _, e := Build(in); e == nil {
		t.Fatal("unknown admission was excluded")
	}
	in = testInput()
	in.Facts.Health = nil
	s, e := Build(in)
	if e != nil {
		t.Fatal(e)
	}
	if m := metric(t, s, "project.health"); m.Quality != "unknown" || string(m.Value) != "null" {
		t.Fatal("missing health was defaulted")
	}
	good, e := Build(testInput())
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := CanonicalJSON(good)
	for _, broken := range []string{strings.Replace(string(raw), `"schema_version":1`, `"schema_version":2`, 1), strings.Replace(string(raw), `"schema_version":1`, `"Schema_Version":1`, 1), strings.Replace(string(raw), `"schema_version":1`, `"schema_version":1,"extra":true`, 1)} {
		if _, e := Decode([]byte(broken)); e == nil {
			t.Fatal("unsupported/alias/extra schema accepted")
		}
	}
	if _, e := Decode(make([]byte, MaxSnapshotBytes+1)); e == nil {
		t.Fatal("oversize input accepted")
	}
}
