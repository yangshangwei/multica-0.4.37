package iteration

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"testing"
)

func TestHistorySnapshotLegacyPriorityLabelsRemainUnknown(t *testing.T) {
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
	var body map[string]any
	if err = json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"original", "scope"} {
		for _, entry := range body[key].([]any) {
			item := entry.(map[string]any)
			delete(item, "priority")
			delete(item, "labels")
		}
	}
	events := body["events"].([]any)
	baseline := events[1].(map[string]any)["after_facts"].(map[string]any)
	delete(baseline, "priority")
	delete(baseline, "labels")
	raw, err = json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	row := db.IterationSnapshot{WorkspaceID: pgtype.UUID{Bytes: uuid.MustParse(snapshot.WorkspaceID), Valid: true}, IterationID: pgtype.UUID{Bytes: uuid.MustParse(snapshot.IterationID), Valid: true}, OperationID: pgtype.UUID{Bytes: uuid.MustParse(snapshot.OperationID), Valid: true}, SchemaVersion: 1, Body: raw}
	decoded, err := DecodeSnapshot(row)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range append(decoded.Original, decoded.Scope...) {
		if item.Priority != nil || item.Labels != nil {
			t.Fatalf("legacy facts invented: %+v", item)
		}
	}
}
