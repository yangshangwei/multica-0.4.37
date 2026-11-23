package reportsnapshot

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func publicFixture(t *testing.T) (Snapshot, PublicSnapshot) {
	t.Helper()
	in := testInput()
	member := "11111111-1111-4111-8111-111111111111"
	in.CreatedBy = "22222222-2222-4222-8222-222222222222"
	in.RequestKey = "33333333-3333-4333-8333-333333333333"
	in.ReadCut = "private-read-cut"
	in.Facts.Health.Project.LeadType, in.Facts.Health.Project.LeadID = "member", member
	in.Facts.Health.References = []ReferenceFact{{Kind: "member", ID: member, Valid: true, EnvironmentUnavailable: true}}
	for i := range in.Facts.Issues {
		in.Facts.Issues[i].AssigneeType, in.Facts.Issues[i].AssigneeID = "member", member
		in.Facts.Issues[i].OriginID = &member
		in.Facts.Issues[i].OriginType = "private-origin"
	}
	for i := range in.Facts.Sources {
		in.Facts.Sources[i].Cursor = ptr("private-cursor")
		in.Facts.Sources[i].Watermark = ptr("private-watermark")
		in.Facts.Sources[i].Identities = []SourceIdentity{{ID: "private-source", Revision: "private-version"}}
	}
	s, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	return s, p
}

func publicMetric(t *testing.T, snapshot PublicSnapshot, id string) Metric {
	t.Helper()
	m := findMetric(snapshot.Metrics, id)
	if m.ID == "" {
		t.Fatalf("missing public metric %s", id)
	}
	return m
}

func TestPublicSnapshotUnknownAndNoProjectStayExplicit(t *testing.T) {
	in := testInput()
	for i := range in.Facts.Issues {
		if in.Facts.Issues[i].ID != in.ReportIssueID {
			in.Facts.Issues[i].Classification = "unknown"
			in.Facts.Issues[i].HistoryComplete = false
		}
	}
	for i := range in.Facts.Sources {
		if in.Facts.Sources[i].Kind == "references" || in.Facts.Sources[i].Kind == "updates" {
			in.Facts.Sources[i].ReadComplete = false
		}
		if in.Facts.Sources[i].Kind == "activities" {
			in.Facts.Sources[i].ProducerComplete = false
		}
	}
	in.Facts.Gaps = []Gap{{Code: "private-diagnostic", Source: "references"}}
	s, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	integer(t, publicMetric(t, p, "formal_stock.total"), 4)
	for _, id := range []string{"stock.total", "window.closed", "project.lead_valid", "project.latest_update_at"} {
		m := publicMetric(t, p, id)
		if string(m.Value) != "null" || m.Quality != "unknown" {
			t.Fatalf("%s guessed from incomplete facts: %+v", id, m)
		}
	}
	if p.Facts.Issues[0].AssignmentValid != nil || p.Facts.Project.LeadValid != nil || p.Facts.Gaps[0].Code != "source_evidence_unavailable" {
		t.Fatal("unknown validity or private diagnostic was exposed")
	}
	if err := ValidatePublic(p); err != nil {
		t.Fatal(err)
	}
	in = testInput()
	in.ProjectID, in.Facts.Health, in.Facts.Sources = nil, nil, nil
	in.Facts.Issues = in.Facts.Issues[:1]
	in.Facts.Issues[0].ProjectID = nil
	s, err = Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range p.Metrics {
		if m.Quality != "not_applicable" || string(m.Value) != "null" {
			t.Fatalf("no project became project zero: %+v", m)
		}
	}
}

func TestPublicSnapshotReplayKeepsCalendarAndIgnoresArrayOrder(t *testing.T) {
	in := testInput()
	in.Window = Window{Start: at("2026-03-08T05:00:00Z"), End: at("2026-03-09T04:00:00Z"), Timezone: "America/New_York", PeriodMode: "calendar_day"}
	in.AsOf = at("2026-03-09T05:00:00Z")
	in.Facts.Health.LatestUpdateAt = ptr(at("2026-03-01T04:00:00Z"))
	in.Facts.Health.Project.InProgressSince = ptr(at("2026-03-01T03:00:00Z"))
	for i := range in.Facts.Issues {
		in.Facts.Issues[i].CreatedAt = at("2026-03-01T00:00:00Z")
		in.Facts.Issues[i].BaselineAt = ptr(at("2026-03-07T00:00:00Z"))
	}
	s, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	integer(t, publicMetric(t, p, "project.progress_age_days"), 9)
	p.Window.Timezone = "Historic/NoLongerInTzdb"
	for i, j := 0, len(p.Facts.Issues)-1; i < j; i, j = i+1, j-1 {
		p.Facts.Issues[i], p.Facts.Issues[j] = p.Facts.Issues[j], p.Facts.Issues[i]
	}
	metrics, err := ReplayPublic(p.PublicInput)
	if err != nil {
		t.Fatalf("public replay depended on current tzdb: %v", err)
	}
	got, _ := CanonicalJSON(metrics)
	want, _ := CanonicalJSON(p.Metrics)
	if !bytes.Equal(got, want) {
		t.Fatal("public-only replay changed dates or array-order semantics")
	}
}

func TestPublicSnapshotOnlyProjectsProvenLifecycleFacts(t *testing.T) {
	in := testInput()
	issue := &in.Facts.Issues[1]
	issue.Status, issue.BaselineStatus, issue.BaselineCategory = "cancelled", "done", "done"
	when := in.Window.Start.Add(time.Hour)
	in.Facts.Activities = []ActivityFact{{ID: "55555555-5555-4555-8555-555555555555", IssueID: issue.ID, Action: "status_changed", RecordedAt: when, BusinessAt: &when, Sequence: ptr(int64(1)), FromStatus: "done", ToStatus: "cancelled", FromCategory: "done", ToCategory: "cancelled", ProjectID: in.ProjectID, ProjectKnown: true, SourceVersion: "private-adapter-instance"}}
	s, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	integer(t, publicMetric(t, p, "window.closed"), 0)
	integer(t, publicMetric(t, p, "window.cancelled"), 1)
	raw, _ := CanonicalJSON(p)
	if len(p.Facts.Activities) != 1 || p.Facts.Activities[0].FactID != "event-1" || bytes.Contains(raw, []byte(in.Facts.Activities[0].ID)) || bytes.Contains(raw, []byte("private-adapter")) {
		t.Fatal("event provenance was exposed or required transition missing")
	}
	in.Facts.Activities[0].Sequence = nil
	s, err = Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Facts.Activities) != 0 || publicMetric(t, p, "window.cancelled").Quality == "exact" {
		t.Fatal("unproven raw history was projected as usable facts")
	}
}

func TestPublicSnapshotContainsOnlyReplayFacts(t *testing.T) {
	internal, public := publicFixture(t)
	raw, err := CanonicalJSON(public)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"11111111-1111-4111-8111-111111111111", internal.CreatedBy, internal.RequestKey, "private-", "created_by", "request_key", "read_cut", "origin_id", "origin_type", "assignee_id", "assignee_type", "lead_id", "lead_type", `"references":`, "snapshot_digest", internal.SnapshotDigest, internal.FactsDigest} {
		if bytes.Contains(raw, []byte(private)) {
			t.Fatalf("public snapshot disclosed %q", private)
		}
	}
	metrics, err := ReplayPublic(public.PublicInput)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := CanonicalJSON(internal.Metrics)
	got, _ := CanonicalJSON(metrics)
	if !bytes.Equal(got, want) {
		t.Fatalf("public-only replay differs: %s != %s", got, want)
	}
	decoded, err := DecodePublic(raw)
	if err != nil || decoded.ProjectionDigest != public.ProjectionDigest {
		t.Fatalf("public round trip failed: %v", err)
	}
	if *public.Facts.Issues[0].AssignmentValid != true || *public.Facts.Issues[0].EnvironmentUnavailable != true || *public.Facts.Project.LeadValid != true {
		t.Fatal("joined validity facts were not retained")
	}
}

func TestPublicSnapshotStrictSchemaAndIndependentValidation(t *testing.T) {
	_, public := publicFixture(t)
	raw, _ := CanonicalJSON(public)
	for _, bad := range []string{
		strings.Replace(string(raw), `"public_projection_version":1`, `"public_projection_version":1,"public_projection_version":1`, 1),
		strings.Replace(string(raw), `"public_projection_version":1`, `"Public_Projection_Version":1`, 1),
		strings.Replace(string(raw), `"public_projection_version":1`, `"public_projection_version":2`, 1),
		strings.TrimSuffix(string(raw), "}") + `,"created_by":"private"}`,
		strings.TrimSuffix(string(raw), "}") + `,"request_key":"private"}`,
		string(raw) + ` {}`,
	} {
		if _, err := DecodePublic([]byte(bad)); err == nil {
			t.Fatal("malformed or internal public field accepted")
		}
	}
	public.Metrics[0].Value = json.RawMessage("999")
	if err := ValidatePublic(public); err == nil {
		t.Fatal("result tampering accepted")
	}
	_, public = publicFixture(t)
	public.Facts.Issues[0].ID = "44444444-4444-4444-8444-444444444444"
	if err := ValidatePublic(public); err == nil {
		t.Fatal("cohort tampering accepted")
	}
	if _, err := DecodePublic(make([]byte, MaxSnapshotBytes+1)); err == nil {
		t.Fatal("oversized public input accepted")
	}
}

func TestPublicReplayNullableValidityCannotInventKnownFalse(t *testing.T) {
	_, public := publicFixture(t)
	public.Facts.Project.LeadValid = nil
	public.Facts.Issues[1].AssignmentValid = nil
	public.Facts.Issues[1].EnvironmentUnavailable = nil
	metrics, err := ReplayPublic(public.PublicInput)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"project.lead_valid", "project.health", "formal_stock.unassigned"} {
		metric := findMetric(metrics, id)
		if metric.Quality == "exact" || string(metric.Value) != "null" {
			t.Fatalf("nullable fact defaulted for %s: %+v", id, metric)
		}
	}
	integer(t, findMetric(metrics, "formal_stock.total"), 4)
}

func TestPublicProjectionSanitizesGapMetadataWithoutBroadeningImpact(t *testing.T) {
	in := testInput()
	in.Facts.Gaps = []Gap{{Code: "private-sql-error", Source: "private-query", MetricIDs: []string{"project.health", "private-metric-name"}}, {Code: "private-irrelevant", Source: "private-source"}}
	s, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := CanonicalJSON(p)
	if bytes.Contains(raw, []byte("private-")) {
		t.Fatal("gap metadata disclosed private source details")
	}
	integer(t, publicMetric(t, p, "formal_stock.total"), 4)
	if publicMetric(t, p, "project.health").Quality != "unknown" {
		t.Fatal("sanitization lost the affected public dependency")
	}
}

func TestPublicReplayRejectsUnsupportedProjectAndSourceContracts(t *testing.T) {
	_, public := publicFixture(t)
	public.Facts.Project.Status = "future_status"
	if _, err := ReplayPublic(public.PublicInput); err == nil {
		t.Fatal("unknown project state entered health calculation")
	}
	_, public = publicFixture(t)
	public.Facts.Sources[0].Version = "private-source-instance"
	if _, err := ReplayPublic(public.PublicInput); err == nil {
		t.Fatal("unrecognized source version accepted")
	}
}

func TestKnownCandidateGapPreservesCoverageDenominator(t *testing.T) {
	in := testInput()
	in.Facts.Gaps = []Gap{{Code: "source_unavailable", Source: "issues", IssueIDs: []string{in.Facts.Issues[1].ID}}}
	s, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id                              string
		included, excluded, denominator int64
	}{{"formal_stock.total", 3, 0, 4}, {"stock.total", 2, 1, 3}} {
		m := publicMetric(t, p, tc.id)
		c := m.Coverage
		if c.Eligible == nil || *c.Eligible != 4 || c.Included != tc.included || c.Excluded != tc.excluded || c.Unknown.Count == nil || *c.Unknown.Count != 1 || c.Unknown.KnownCount != 1 || c.Unknown.ExtentUnknown || c.Ratio == nil || c.Ratio.Numerator != tc.included || c.Ratio.Denominator != tc.denominator || string(m.Value) != "null" {
			t.Fatalf("known candidate became unknown tail: %+v", m)
		}
	}
	in.Facts.Gaps[0].IssueIDs = nil
	s, err = Build(in)
	if err != nil {
		t.Fatal(err)
	}
	m := metric(t, s, "formal_stock.total")
	if m.Coverage.Eligible != nil || m.Coverage.Unknown.Count != nil || m.Coverage.Ratio != nil || !m.Coverage.Unknown.ExtentUnknown {
		t.Fatal("global unavailable source invented a known extent")
	}
}

func TestWindowCreationUsesBirthEvidenceDespiteStatusHistoryGap(t *testing.T) {
	in := testInput()
	in.Facts.Issues[1].CreatedAt = in.Window.Start
	in.Facts.Gaps = []Gap{{Code: "producer_history_unproven", Source: "activities"}}
	s, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ProjectPublic(s)
	if err != nil {
		t.Fatal(err)
	}
	integer(t, publicMetric(t, p, "window.created"), 1)
	if publicMetric(t, p, "window.closed").Quality == "exact" {
		t.Fatal("birth evidence was incorrectly used as status history")
	}
	in.Facts.Gaps = append(in.Facts.Gaps, Gap{Code: "source_unavailable", Source: "issues", IssueIDs: []string{in.Facts.Issues[1].ID}})
	s, err = Build(in)
	if err != nil {
		t.Fatal(err)
	}
	m := metric(t, s, "window.created")
	if m.Coverage.Eligible == nil || *m.Coverage.Eligible != 4 || m.Coverage.Unknown.Count == nil || *m.Coverage.Unknown.Count != 1 || string(m.Value) != "null" {
		t.Fatalf("missing known birth fact did not remain a bounded unknown: %+v", m)
	}
}

func TestPublicProjectReasonsPreserveHealthPriority(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      string
		environment bool
		empty       bool
		due         *string
		wantHealth  string
		wantReasons []string
	}{
		{"known clear", "planned", false, false, nil, "clear", []string{}},
		{"environment reason", "planned", true, false, nil, "clear", []string{"execution_environment_unavailable"}},
		{"ended open reason", "completed", false, false, nil, "clear", []string{"ended_project_open_issues"}},
		{"empty precedes overdue", "planned", false, true, ptr("2026-11-12"), "empty", []string{"project_overdue"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := testInput()
			member := "11111111-1111-4111-8111-111111111111"
			in.Facts.Health.Project.Status, in.Facts.Health.Project.DueDate = tc.status, tc.due
			in.Facts.Health.Project.LeadType, in.Facts.Health.Project.LeadID = "member", member
			in.Facts.Health.References = []ReferenceFact{{Kind: "member", ID: member, Valid: true, EnvironmentUnavailable: tc.environment}}
			for i := range in.Facts.Issues {
				in.Facts.Issues[i].AssigneeType, in.Facts.Issues[i].AssigneeID = "member", member
				if tc.empty {
					in.Facts.Issues[i].AdmissionStatus = "pending"
				}
			}
			s, err := Build(in)
			if err != nil {
				t.Fatal(err)
			}
			public, err := ProjectPublic(s)
			if err != nil {
				t.Fatal(err)
			}
			if health := publicMetric(t, public, "project.health"); health.Quality != "exact" || string(health.Value) != `"`+tc.wantHealth+`"` || !reflect.DeepEqual(public.ProjectReasons, tc.wantReasons) {
				t.Fatalf("health/reasons = %s / %#v, want %s / %#v", publicMetric(t, public, "project.health").Value, public.ProjectReasons, tc.wantHealth, tc.wantReasons)
			}
			if err := ValidatePublic(public); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPublicProjectReasonsUnknownAndNotApplicableAreNull(t *testing.T) {
	for _, noProject := range []bool{false, true} {
		in := testInput()
		if noProject {
			in.ProjectID, in.Facts.Health, in.Facts.Sources = nil, nil, nil
			in.Facts.Issues = in.Facts.Issues[:1]
			in.Facts.Issues[0].ProjectID = nil
		} else {
			in.Facts.Gaps = []Gap{{Code: "source_unavailable", Source: "references"}}
		}
		s, err := Build(in)
		if err != nil {
			t.Fatal(err)
		}
		public, err := ProjectPublic(s)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := CanonicalJSON(public)
		if err != nil || public.ProjectReasons != nil || !bytes.Contains(raw, []byte(`"project_reasons":null`)) {
			t.Fatalf("unknown reasons became an empty known set: %s / %v", raw, err)
		}
		public.ProjectReasons = []string{}
		if err := ValidatePublic(public); err == nil {
			t.Fatal("unknown reasons accepted as known empty")
		}
	}
}

func TestPublicProjectReasonsAreRecomputedAndDigestBound(t *testing.T) {
	_, public := publicFixture(t)
	oldResult, oldProjection := public.ResultDigest, public.ProjectionDigest
	oldMetrics, _ := CanonicalJSON(public.Metrics)
	public.ProjectReasons = []string{"private-freeform-reason"}
	if err := ValidatePublic(public); err == nil {
		t.Fatal("unproven reason accepted")
	}
	raw, err := CanonicalJSON(public)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePublic(raw); err == nil {
		t.Fatal("tampered reason decoded")
	}
	// Ending this otherwise clear project adds a reason without changing any
	// metric value. The result digest must therefore bind reasons independently.
	public.Facts.Project.Status = "completed"
	fresh, err := buildPublic(public.PublicInput)
	if err != nil {
		t.Fatal(err)
	}
	newMetrics, _ := CanonicalJSON(fresh.Metrics)
	if !bytes.Equal(oldMetrics, newMetrics) || fresh.ResultDigest == oldResult || fresh.ProjectionDigest == oldProjection || !reflect.DeepEqual(fresh.ProjectReasons, []string{"ended_project_open_issues", "execution_environment_unavailable"}) {
		t.Fatal("facts-driven reason change was not independently sealed")
	}
}
