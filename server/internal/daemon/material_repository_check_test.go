package daemon

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/internal/materialrepo"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type repositoryInspectionRetryReader struct {
	materialAbsentNoteReader
	resolved bool
}

func (r *repositoryInspectionRetryReader) ResolveBranch(context.Context, string) (string, error) {
	r.resolved = true
	return strings.Repeat("a", 40), nil
}
func (r *repositoryInspectionRetryReader) Snapshot(_ context.Context, commit string, _ []string) (materialrepo.Snapshot, error) {
	return materialrepo.Snapshot{Complete: true, Commit: commit, Entries: []materialrepo.Entry{}}, nil
}

var _ materialgit.RepositoryObserver = (*repositoryInspectionRetryReader)(nil)

func TestMaterialRepositoryInspectionRecoveryReadsBase(t *testing.T) {
	r := &repositoryInspectionRetryReader{}
	e := protocol.MaterialExecution{TargetKind: "repository_initialization", Action: "observe", ReconcileAction: "inspect_repository", BaseBranch: "main", RepositoryExecution: &protocol.MaterialRepositoryExecution{SystemID: "repository"}}
	o, err := readRepositoryObservation(t.Context(), e, protocol.MaterialClaimBinding{}, r)
	var s materialrepo.Snapshot
	if err != nil || json.Unmarshal(o.RepositorySnapshot, &s) != nil || !s.Complete || !r.resolved || o.Review != nil {
		t.Fatalf("inspection retry tried to discover nonexistent review: %+v %v", o, err)
	}
}
