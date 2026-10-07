package service

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"testing"
)

func TestIterationOriginalPriorityLabelsAndStalePreview(t *testing.T) {
	fx, s := lifecycleFixture(t)
	source := lifecycleCreate(t, fx, s)
	issue := fx.Issue(t, "Original fields", testutil.Cols{"priority": "high"})
	label := fx.Insert(t, "issue_label", testutil.Cols{"workspace_id": fx.WorkspaceID, "name": "Original label", "color": "#123456", "resource_type": "issue"})
	fx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": issue, "label_id": label}, "issue_id=$1 AND label_id=$2", issue, label)
	lifecycleMove(t, fx, s, issue, &source)
	start := lifecycleDraft(t, fx, source, "start")
	start.Start = &iteration.StartDraft{TargetID: source, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
	lifecycleApply(t, fx, s, start)
	var original iteration.OriginalFacts
	var raw []byte
	fx.QueryRow(t, "SELECT original_facts FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2", source, issue).Scan(&raw)
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	if original.Priority == nil || *original.Priority != "high" || len(original.Labels) != 1 || original.Labels[0].Name != "Original label" {
		t.Fatalf("start capture=%+v", original)
	}
	draft := closureDraft(t, fx, source, issue, "end", nil)
	ws, actor := lifecycleIDs(fx)
	preview, err := s.Preview(t.Context(), ws, actor, draft, lifecycleAuth)
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "UPDATE issue_label SET name='Changed label' WHERE id=$1", label)
	_, err = s.Apply(t.Context(), ws, actor, ApplyIterationInput{RequestID: uuid.NewString(), Draft: draft, PreviewHash: preview.PreviewHash}, lifecycleAuth)
	var conflict *iteration.OperationError
	if !errors.As(err, &conflict) || conflict.Status != 409 {
		t.Fatalf("expected stale409, got %v", err)
	}
	fx.Exec(t, "UPDATE issue SET priority='low' WHERE id=$1", issue)
	lifecycleApply(t, fx, s, draft)
	var snapshot iteration.Snapshot
	fx.QueryRow(t, "SELECT body FROM iteration_snapshot WHERE iteration_id=$1", source).Scan(&raw)
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Original) != 1 || len(snapshot.Scope) != 1 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	o, current := snapshot.Original[0], snapshot.Scope[0]
	if o.Priority == nil || *o.Priority != "high" || len(o.Labels) != 1 || o.Labels[0].Name != "Original label" {
		t.Fatalf("original changed: %+v", o)
	}
	if current.Priority == nil || *current.Priority != "low" || len(current.Labels) != 1 || current.Labels[0].Name != "Changed label" {
		t.Fatalf("closure capture: %+v", current)
	}
}
