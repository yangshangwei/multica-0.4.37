package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationExecutionViewRetainsUnresolvedLegacyIdentity(t *testing.T) {
	session := db.CollaborationSession{ID: util.MustParseUUID(uuid.NewString()), IssueID: util.MustParseUUID(uuid.NewString()),
		QueueState: "paused", ControlVersion: 1, TargetSetVersion: 1, ActiveRunID: util.MustParseUUID(uuid.NewString())}
	command := db.CollaborationCommand{ID: util.MustParseUUID(uuid.NewString()), SessionID: session.ID, LegacySessionID: session.ID, Seq: 1, LegacyUnresolved: true}
	run := db.CollaborationRun{ID: session.ActiveRunID, SessionID: session.ID, LegacySessionID: session.ID,
		CommandID: command.ID, LegacyCommandID: command.ID, LegacyUnresolved: true, ProgressState: "ending", ProofState: "unknown"}
	view, err := projectCollaborationExecution(session, nil, []db.CollaborationCommand{command}, nil, []db.CollaborationRun{run})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Commands) != 0 || len(view.Runs) != 0 || len(view.Targets) != 0 || len(view.LegacyUnresolved.CommandIDs) != 1 || len(view.LegacyUnresolved.RunIDs) != 1 ||
		view.LegacyUnresolved.CommandIDs[0] != util.UUIDToString(command.ID) || view.LegacyUnresolved.RunIDs[0] != util.UUIDToString(run.ID) ||
		view.LegacyUnresolved.ActiveRunID == nil || *view.LegacyUnresolved.ActiveRunID != util.UUIDToString(run.ID) || view.Availability != "unavailable" {
		t.Fatalf("legacy work was hidden or given an invented target: %+v", view)
	}
	if _, err := projectCollaborationExecution(session, nil, []db.CollaborationCommand{command}, nil, nil); err == nil {
		t.Fatal("missing legacy active run must not become an idle projection")
	}
	run.SessionID = util.MustParseUUID(uuid.NewString())
	if _, err := projectCollaborationExecution(session, nil, []db.CollaborationCommand{command}, nil, []db.CollaborationRun{run}); err == nil {
		t.Fatal("foreign legacy work entered this issue projection")
	}
}
