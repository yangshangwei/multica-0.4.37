package iteration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func snapshotHistoryFixture(t *testing.T) History {
	t.Helper()
	start := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	ws, id, issue := uuid.NewString(), uuid.NewString(), uuid.NewString()
	name := "Frozen project"
	project := uuid.NewString()
	item := HistoricalIssue{IssueID: issue, Identifier: "HG-1", Title: "Commitment", ProjectID: &project, ProjectName: &name, StatusKey: "todo", StatusCategory: "todo"}
	facts, err := json.Marshal(OriginalFacts{HistoricalIssue: item})
	if err != nil {
		t.Fatal(err)
	}
	events := []Event{{ID: uuid.NewString(), IterationID: id, OperationID: uuid.NewString(), Sequence: 1, Kind: "start", Actor: json.RawMessage(`{"type":"system","id":null,"user_id":null,"source":"test"}`), BeforeFacts: json.RawMessage("null"), AfterFacts: json.RawMessage("null"), OccurredAt: start, SampledAt: start}, {ID: uuid.NewString(), IterationID: id, IssueID: &issue, OperationID: uuid.NewString(), Sequence: 2, Kind: "baseline", Actor: json.RawMessage(`{"type":"system","id":null,"user_id":null,"source":"test"}`), BeforeFacts: json.RawMessage("null"), AfterFacts: facts, OccurredAt: start, SampledAt: start}}
	stats, err := CalculateStatistics([]ScopeIssue{{IssueID: issue, StatusCategory: "todo"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	chart, err := BuildChart([]ScopeIssue{{IssueID: issue, StatusCategory: "todo"}}, nil, start, start, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	return History{Iteration: Iteration{ID: id, WorkspaceID: ws, Status: "active", Timezone: "UTC", StartedAt: &start}, Original: []HistoricalIssue{item}, Scope: []HistoricalIssue{item}, Events: events, Statistics: Statistics{ScopeStatistics: stats, Chart: chart, CalculatedAt: start}}
}

func TestHistorySnapshotOwnsFrozenPayload(t *testing.T) {
	h := snapshotHistoryFixture(t)
	now := h.Statistics.CalculatedAt
	snapshot, err := BuildSnapshot(h, uuid.NewString(), "completed", "Finished", now, now, []Destination{{IssueID: h.Scope[0].IssueID}})
	if err != nil {
		t.Fatal(err)
	}
	*h.Original[0].ProjectName = "Changed"
	h.Scope[0].Title = "Changed"
	h.Events[1].AfterFacts[0] = '!'
	h.Statistics.Chart[0].Effective = 99
	if *snapshot.Original[0].ProjectName != "Frozen project" || snapshot.Scope[0].Title != "Commitment" || !json.Valid(snapshot.Events[1].AfterFacts) || snapshot.Statistics.Chart[0].Effective != 1 {
		t.Fatal("snapshot retained mutable source storage")
	}
}

func TestHistorySnapshotRejectsMalformedStoredPayload(t *testing.T) {
	h := snapshotHistoryFixture(t)
	now := h.Statistics.CalculatedAt
	snapshot, err := BuildSnapshot(h, uuid.NewString(), "completed", "Finished", now, now, []Destination{{IssueID: h.Scope[0].IssueID}})
	if err != nil {
		t.Fatal(err)
	}
	encode := func(s Snapshot) db.IterationSnapshot {
		raw, e := json.Marshal(s)
		if e != nil {
			t.Fatal(e)
		}
		ws, _ := uuid.Parse(snapshot.WorkspaceID)
		id, _ := uuid.Parse(snapshot.IterationID)
		op, _ := uuid.Parse(snapshot.OperationID)
		return db.IterationSnapshot{WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true}, IterationID: pgtype.UUID{Bytes: id, Valid: true}, OperationID: pgtype.UUID{Bytes: op, Valid: true}, SchemaVersion: 1, Body: raw}
	}
	row := encode(snapshot)
	if _, err = DecodeSnapshot(row); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Snapshot){"identity": func(s *Snapshot) { s.WorkspaceID = uuid.NewString() }, "counter": func(s *Snapshot) { s.Statistics.Completed = 9 }, "status": func(s *Snapshot) { s.Scope[0].StatusCategory = "mystery" }, "reference": func(s *Snapshot) { invalid := "not-a-uuid"; s.Scope[0].ProjectID = &invalid }, "missing events": func(s *Snapshot) { s.Events = nil }, "missing chart": func(s *Snapshot) { s.Statistics.Chart = nil }, "event cutoff": func(s *Snapshot) { s.Events[1].OccurredAt = now.Add(time.Hour) }}
	for name, modify := range cases {
		t.Run(name, func(t *testing.T) {
			var bad Snapshot
			if err := json.Unmarshal(row.Body, &bad); err != nil {
				t.Fatal(err)
			}
			modify(&bad)
			if _, err := DecodeSnapshot(encode(bad)); err == nil {
				t.Fatal("malformed snapshot accepted")
			}
		})
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(row.Body, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "destinations")
	row.Body, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeSnapshot(row); err == nil {
		t.Fatal("missing required collection accepted")
	}
}

func TestHistorySnapshotRequiresCompleteValidDestinations(t *testing.T) {
	h := snapshotHistoryFixture(t)
	now := h.Statistics.CalculatedAt
	target := uuid.NewString()
	id := h.Scope[0].IssueID
	for name, destinations := range map[string][]Destination{
		"missing":                {},
		"duplicate":              {{IssueID: id}, {IssueID: id}},
		"released count changed": {{IssueID: id, RolloverCountAfter: 1}},
		"target count unchanged": {{IssueID: id, TargetIterationID: &target}},
		"self target":            {{IssueID: id, TargetIterationID: &h.Iteration.ID, RolloverCountAfter: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := BuildSnapshot(h, uuid.NewString(), "completed", "Finished", now, now, destinations); err == nil {
				t.Fatal("incomplete or invalid destinations accepted")
			}
		})
	}
	if _, err := BuildSnapshot(h, uuid.NewString(), "completed", "Finished", now, now, []Destination{{IssueID: id, TargetIterationID: &target, RolloverCountAfter: 1}}); err != nil {
		t.Fatal(err)
	}
	h.Original[0].StatusKey = "done"
	h.Original[0].StatusCategory = "done"
	h.Original[0].WasCompletedAtStart = true
	h.Scope[0] = h.Original[0]
	facts, err := json.Marshal(OriginalFacts{HistoricalIssue: h.Original[0], HasStarted: true})
	if err != nil {
		t.Fatal(err)
	}
	h.Events[1].AfterFacts = facts
	h.Statistics.ScopeStatistics, err = CalculateStatistics([]ScopeIssue{{IssueID: id, StatusCategory: "done"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	h.Statistics.Chart, err = BuildChart([]ScopeIssue{{IssueID: id, StatusCategory: "done"}}, nil, now, now, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = BuildSnapshot(h, uuid.NewString(), "completed", "Finished", now, now, []Destination{{IssueID: id, TargetIterationID: &target, RolloverCountAfter: 1}}); err == nil {
		t.Fatal("terminal issue received rollover target")
	}
}

func storedHistorySnapshotFixture(t *testing.T) db.IterationSnapshot {
	t.Helper()
	h := snapshotHistoryFixture(t)
	now := h.Statistics.CalculatedAt
	snapshot, err := BuildSnapshot(h, uuid.NewString(), "completed", "Finished", now, now, []Destination{{IssueID: h.Scope[0].IssueID}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	ws, _ := uuid.Parse(snapshot.WorkspaceID)
	id, _ := uuid.Parse(snapshot.IterationID)
	op, _ := uuid.Parse(snapshot.OperationID)
	return db.IterationSnapshot{WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true}, IterationID: pgtype.UUID{Bytes: id, Valid: true}, OperationID: pgtype.UUID{Bytes: op, Valid: true}, SchemaVersion: 1, Body: raw}
}

func TestHistorySnapshotDestinationRequiresExplicitFields(t *testing.T) {
	for _, field := range []string{"issue_id", "target_iteration_id", "rollover_count_before", "rollover_count_after"} {
		for _, malformed := range []string{"missing", "null", "fraction", "unsafe_integer"} {
			if field == "target_iteration_id" && malformed == "null" {
				continue
			}
			t.Run(field+"/"+malformed, func(t *testing.T) {
				row := storedHistorySnapshotFixture(t)
				var body map[string]json.RawMessage
				if err := json.Unmarshal(row.Body, &body); err != nil {
					t.Fatal(err)
				}
				var destinations []map[string]json.RawMessage
				if err := json.Unmarshal(body["destinations"], &destinations); err != nil {
					t.Fatal(err)
				}
				switch malformed {
				case "missing":
					delete(destinations[0], field)
				case "null":
					destinations[0][field] = json.RawMessage("null")
				case "fraction":
					destinations[0][field] = json.RawMessage("0.5")
				case "unsafe_integer":
					destinations[0][field] = json.RawMessage("9007199254740992")
				}
				var err error
				body["destinations"], err = json.Marshal(destinations)
				if err != nil {
					t.Fatal(err)
				}
				row.Body, err = json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = DecodeSnapshot(row); err == nil {
					t.Fatal("incomplete destination was treated as a known release")
				}
			})
		}
	}
	if _, err := DecodeSnapshot(storedHistorySnapshotFixture(t)); err != nil {
		t.Fatalf("explicit null release must remain valid: %v", err)
	}
}

func TestHistorySnapshotRejectsMismatchedBaselineIdentity(t *testing.T) {
	row := storedHistorySnapshotFixture(t)
	var snapshot Snapshot
	if err := json.Unmarshal(row.Body, &snapshot); err != nil {
		t.Fatal(err)
	}
	var facts OriginalFacts
	if err := json.Unmarshal(snapshot.Events[1].AfterFacts, &facts); err != nil {
		t.Fatal(err)
	}
	facts.IssueID = uuid.NewString()
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Events[1].AfterFacts = raw
	row.Body, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeSnapshot(row); err == nil {
		t.Fatal("foreign baseline facts accepted in stored snapshot")
	}
	if _, err = historyChanges(true, snapshot.Events); err == nil {
		t.Fatal("live projection ignored foreign baseline facts")
	}
}
