package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationReviewLateProofRejectsContradictorySettledRun(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "retain contradictory historical evidence")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.StopQueue(f.ctx, f.stopParams(run, queueTarget(t, f.collaborationQueueFixture, f.targetID).ControlVersion)); err != nil {
		t.Fatal(err)
	}
	// A late supervisor proof must validate retained evidence even after the
	// group was settled. Keep a reservation while simulating contradictory data.
	f.fx.Exec(t, `UPDATE collaboration_run SET progress_state='running' WHERE id=$1`, runID)
	f.fx.Exec(t, `UPDATE collaboration_runtime_reservation SET released_at=NULL WHERE run_id=$1`, runID)
	proof := protocol.CollaborationRunProof{
		ReceiptID: uuid.NewString(), Identity: claim.Identity,
		LastAuthorizedAttemptSeq: "0", SealedAttemptSeq: "0",
		State: "never_launched_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
	}
	_, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof)
	if err == nil {
		t.Fatal("late proof accepted a historical run that still claims running")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND confirmed_at IS NULL`, runID) != 1 {
		t.Fatal("contradictory historical proof released capacity or committed its receipt")
	}
}

func TestCollaborationReviewProjectionRequiresCompleteNonlaunchEvidence(t *testing.T) {
	for _, scenario := range []string{"started", "missing_seal", "incomplete_seal"} {
		t.Run(scenario, func(t *testing.T) {
			f := newExecutionProjectionFixture()
			f.settle("never_launched_proven")
			f.runs[0].SealedAttemptSeq = pgtype.Int8{Int64: 0, Valid: true}
			f.commands[0].State, f.commands[0].GroupID, f.commands[0].BatchID = "waiting", pgtype.UUID{}, util.MustParseUUID(uuid.NewString())
			if _, err := f.project(); err != nil {
				t.Fatalf("complete nonlaunch fixture is unavailable: %v", err)
			}
			switch scenario {
			case "started":
				f.runs[0].StartedAt = f.runs[0].SettledAt
			case "missing_seal":
				f.runs[0].SealedAttemptSeq = pgtype.Int8{}
			case "incomplete_seal":
				f.runs[0].AuthorizedAttemptSeq = 1
			}
			if _, err := f.project(); err == nil {
				t.Fatalf("returned input projected as safe with %s nonlaunch evidence", scenario)
			}
		})
	}
}
