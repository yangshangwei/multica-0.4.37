package service

import (
	"bytes"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/reportsnapshot"
)

func reportAuditFixture(t *testing.T, pageSize int) (reportsnapshot.Snapshot, reportSnapshotAudit) {
	t.Helper()
	project, report, other := "10000000-0000-4000-8000-000000000001", "20000000-0000-4000-8000-000000000001", "30000000-0000-4000-8000-000000000001"
	start := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	in := reportsnapshot.Input{Identity: reportsnapshot.Identity{SnapshotID: "40000000-0000-4000-8000-000000000001", WorkspaceID: "50000000-0000-4000-8000-000000000001", ReportIssueID: report, ProjectID: &project, ReportRevision: 1, Sequence: 1, CreatedBy: "60000000-0000-4000-8000-000000000001", RequestKey: "70000000-0000-4000-8000-000000000001"}, Window: reportsnapshot.Window{Start: start, End: start.Add(24 * time.Hour), Timezone: "UTC", PeriodMode: "rolling_24h"}, AsOf: start.Add(25 * time.Hour), ReadCut: "private-fixed-cut"}
	in.Facts.Health = &reportsnapshot.HealthFacts{Project: reportsnapshot.ProjectFact{WorkspaceID: in.WorkspaceID, ID: project, Revision: 1, Status: "planned"}, Statuses: []reportsnapshot.StatusFact{}, References: []reportsnapshot.ReferenceFact{}}
	in.Facts.Issues = []reportsnapshot.IssueFact{{ID: report, Revision: 1, ProjectID: &project, Status: "todo", AdmissionStatus: "not_required", CreatedAt: start.Add(-24 * time.Hour), Classification: "report", ClassificationReason: "current_report"}, {ID: other, Revision: 1, ProjectID: &project, Status: "done", AdmissionStatus: "not_required", CreatedAt: start.Add(-24 * time.Hour), Classification: "unknown", ClassificationReason: "report_classification_unproven"}}
	audit := reportSnapshotAudit{Version: 1, CutRef: in.ReadCut, Classifications: []reportSnapshotClassificationProof{{IssueID: report, IssueRevision: 1, Kind: "current_report"}, {IssueID: other, IssueRevision: 1, Kind: "unknown"}}}
	orders := map[string]string{"project": "id ASC", "issues": "id ASC", "status_catalog": "key ASC", "references": "kind,id ASC", "updates": "published_at DESC,id DESC", "activities": "created_at ASC,id ASC"}
	for _, kind := range []string{"project", "issues", "status_catalog", "references", "updates", "activities"} {
		count := 0
		if kind == "project" {
			count = 1
		}
		if kind == "issues" {
			count = 2
		}
		in.Facts.Sources = append(in.Facts.Sources, reportsnapshot.SourceFact{Kind: kind, Version: "fixture-v1", Ordering: orders[kind], ReadComplete: true, ProducerComplete: kind != "activities", Count: reportCount(count), Identities: []reportsnapshot.SourceIdentity{}})
	}
	for i := range in.Facts.Sources {
		source := &in.Facts.Sources[i]
		if source.Kind == "issues" {
			for _, issue := range in.Facts.Issues {
				source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: issue.ID, Revision: "1"})
			}
			for _, proof := range audit.Classifications {
				revision, err := reportContentRevision(proof)
				if err != nil {
					t.Fatal(err)
				}
				source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: "classification:" + proof.IssueID, Revision: revision})
			}
		} else if source.Kind == "project" {
			source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: project, Revision: "1"})
		}
	}
	expected, err := reportAuditExpectedRows(in, audit)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range in.Facts.Sources {
		entry := reportSnapshotSourceAudit{Kind: source.Kind, Ordering: source.Ordering, Termination: "selection_complete", Pages: []reportSnapshotReadPage{}, RootBoundIDs: []string{}}
		rows := []reportSnapshotAuditRow{}
		for _, row := range expected[source.Kind] {
			rows = append(rows, row)
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
		if source.Kind == "issues" {
			entry.Termination = "eof"
			lower := "00000000-0000-0000-0000-000000000000"
			for offset := 0; ; {
				end := min(offset+pageSize, len(rows))
				part := append([]reportSnapshotAuditRow{}, rows[offset:end]...)
				page, err := reportAuditPage(lower, pageSize, part, len(part) < pageSize)
				if err != nil {
					t.Fatal(err)
				}
				entry.Pages = append(entry.Pages, page)
				if page.Exhausted {
					break
				}
				lower, offset = page.UpperInclusive, end
			}
		} else {
			page, err := reportAuditPage("", 0, rows, true)
			if err != nil {
				t.Fatal(err)
			}
			entry.Pages = append(entry.Pages, page)
		}
		audit.Sources = append(audit.Sources, entry)
	}
	snapshot, err := reportsnapshot.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, audit
}

func TestReportSnapshotCollectionAuditPageSizeDoesNotChangeFacts(t *testing.T) {
	first, a := reportAuditFixture(t, 1)
	second, b := reportAuditFixture(t, 2)
	if err := validateReportSnapshotAudit(a, first.Input); err != nil {
		t.Fatal(err)
	}
	if err := validateReportSnapshotAudit(b, second.Input); err != nil {
		t.Fatal(err)
	}
	aa, _ := reportDigest(a)
	bb, _ := reportDigest(b)
	if aa == bb || first.FactsDigest != second.FactsDigest || first.ResultDigest != second.ResultDigest || first.SnapshotDigest != second.SnapshotDigest {
		t.Fatal("transport pages contaminated immutable computational identity")
	}
}

func TestReportSnapshotCollectionAuditRejectsCutPageAndProvenanceDrift(t *testing.T) {
	for _, change := range []struct {
		name   string
		mutate func(*reportSnapshotAudit)
	}{
		{"cut", func(a *reportSnapshotAudit) { a.CutRef = "other-cut" }},
		{"page hash", func(a *reportSnapshotAudit) { a.Sources[0].Pages[0].PageDigest = "bad" }},
		{"missing EOF", func(a *reportSnapshotAudit) { a.Sources[1].Pages = a.Sources[1].Pages[:1] }},
		{"page boundary", func(a *reportSnapshotAudit) { a.Sources[1].Pages[1].LowerExclusive = "other-key" }},
		{"classification", func(a *reportSnapshotAudit) { a.Classifications[1].Kind = "builtin_report" }},
		{"missing source", func(a *reportSnapshotAudit) { a.Sources = a.Sources[:len(a.Sources)-1] }},
	} {
		t.Run(change.name, func(t *testing.T) {
			snapshot, audit := reportAuditFixture(t, 1)
			change.mutate(&audit)
			if err := validateReportSnapshotAudit(audit, snapshot.Input); err == nil {
				t.Fatal("inconsistent audit accepted")
			}
		})
	}
}

func TestReportSnapshotCollectionAuditHasNoPublicProjection(t *testing.T) {
	snapshot, audit := reportAuditFixture(t, 2)
	raw, err := json.Marshal(audit)
	if err != nil || len(raw) == 0 {
		t.Fatal(err)
	}
	public, err := reportsnapshot.ProjectPublic(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if public.Facts.Sources[0].ProofVisibility == "" {
		t.Fatal("public source capability state missing")
	}
	if err := reportsnapshot.ValidatePublic(public); err != nil {
		t.Fatal(err)
	}
	publicBytes, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{audit.CutRef, "page_digest", "lower_exclusive", "linked_run_id", "classifications", "cut_ref"} {
		if bytes.Contains(publicBytes, []byte(private)) {
			t.Fatalf("public projection disclosed private audit %q", private)
		}
	}
}
