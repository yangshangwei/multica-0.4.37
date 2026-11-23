package service

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationHostManagedTerminationPredicate(t *testing.T) {
	ended := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	base := db.CollaborationRun{Containment: pgtype.Text{String: protocol.CollaborationContainmentHostProcessSupervision, Valid: true},
		ProofState: "host_exit_observed", ProgressState: "settled", SettledAt: ended, AuthorizedAttemptSeq: 2, SealedAttemptSeq: pgtype.Int8{Int64: 2, Valid: true}}
	if !collaborationRunProvenEnded(base) {
		t.Fatal("observed, sealed host execution with missing business result did not end")
	}
	for name, mutate := range map[string]func(*db.CollaborationRun){
		"wrong mode": func(r *db.CollaborationRun) {
			r.Containment.String = protocol.CollaborationContainmentLinuxPIDNamespace
		},
		"unknown mode":      func(r *db.CollaborationRun) { r.Containment.String = "future" },
		"host strong proof": func(r *db.CollaborationRun) { r.ProofState = "exited_proven" },
		"success only": func(r *db.CollaborationRun) {
			r.ProofState = "unknown"
			r.BusinessOutcome = pgtype.Text{String: "success", Valid: true}
		},
		"unsealed latest launch": func(r *db.CollaborationRun) { r.SealedAttemptSeq.Int64 = 1 },
		"invented launch":        func(r *db.CollaborationRun) { r.SealedAttemptSeq.Int64 = 3 },
		"still running":          func(r *db.CollaborationRun) { r.ProgressState = "running" },
		"no observed launch":     func(r *db.CollaborationRun) { r.AuthorizedAttemptSeq = 0; r.SealedAttemptSeq.Int64 = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			run := base
			mutate(&run)
			if collaborationRunProvenEnded(run) {
				t.Fatal("incompatible/unknown host state released its barrier")
			}
		})
	}
}
