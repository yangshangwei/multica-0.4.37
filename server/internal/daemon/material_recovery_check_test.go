package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type materialAbsentNoteReader struct{ noteErr, fileErr error }

func (r materialAbsentNoteReader) FindReview(context.Context, string, string) (int64, error) {
	return 1, nil
}
func (r materialAbsentNoteReader) Observe(context.Context, int64) (materialgit.Review, error) {
	return materialgit.Review{Number: 1, Branch: "candidate", BaseBranch: "main", HeadSHA: strings.Repeat("a", 40), State: "open"}, nil
}
func (r materialAbsentNoteReader) ObserveDecision(context.Context, int64, string, string) (string, error) {
	return "", r.noteErr
}
func (r materialAbsentNoteReader) ReadFile(context.Context, string, string) (string, error) {
	return "actual remote content", r.fileErr
}

func TestMaterialReviewSyncRecoveryRequiresCompleteAbsence(t *testing.T) {
	e := protocol.MaterialExecution{Action: "observe", ReconcileAction: "review_sync", SourceBranch: "candidate", BaseBranch: "main", ReviewNumber: 1, AuditNote: &protocol.MaterialAuditNote{Marker: "decision", Body: "decision note"}, Files: []protocol.MaterialFile{{Path: "prd.md"}}}
	r := materialAbsentNoteReader{noteErr: &materialgit.Error{Code: "decision_note_missing"}}
	o, err := readMaterialObservation(t.Context(), e, protocol.MaterialClaimBinding{}, r)
	if err != nil || !o.SourceComplete || o.Outcome != "not_found" || o.AuditNote != nil || len(o.Files) != 1 || o.Files[0].Content != "actual remote content" {
		t.Fatalf("complete missing-note proof: %+v %v", o, err)
	}
	r.fileErr = errors.New("unreadable blob")
	if _, err := readMaterialObservation(t.Context(), e, protocol.MaterialClaimBinding{}, r); err == nil {
		t.Fatal("missing bytes accepted as absence")
	}
	r.fileErr = nil
	r.noteErr = &materialgit.Error{Code: "decision_note_conflict"}
	if _, err := readMaterialObservation(t.Context(), e, protocol.MaterialClaimBinding{}, r); err == nil {
		t.Fatal("conflicting note treated as absent")
	}
	r.noteErr = &materialgit.Error{Code: "decision_note_missing"}
	e.Action = "review_sync"
	if _, err := readMaterialObservation(t.Context(), e, protocol.MaterialClaimBinding{}, r); err == nil {
		t.Fatal("mutation accepted missing note")
	}
}
