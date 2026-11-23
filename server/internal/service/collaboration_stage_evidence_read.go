package service

import (
	"context"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Only this read projection can construct the marker. Inherited Begin returns
// an unmarked nested transaction, so real writers retain their original fences.
type collaborationStageEvidenceReadTx struct {
	pgx.Tx
}

// Stage evidence changes no data, but M1's current-authority observer must hold
// permission row locks. Keep one snapshot without declaring it READ ONLY.
func (s *CollaborationService) readCollaborationStageEvidence(ctx context.Context, issueID pgtype.UUID, read func(pgx.Tx, *db.Queries, CollaborationIssueAccess) error) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		return err
	}
	readTx := collaborationStageEvidenceReadTx{Tx: tx}
	q := s.Queries.WithTx(readTx)
	access, err := s.ResolveIssueAccess(ctx, q, issueID)
	if err != nil {
		return err
	}
	if err = read(readTx, q, access); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Authorize the exact original frozen artifact, independently of whether its
// candidate, PR head or test outcome remains usable for a new stage approval.
func testReportSourceRead(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, event db.CollaborationRunEvent, report collaborationStoredTestReport) error {
	return testReportSourceReadAs(ctx, tx, access, event, report, completionRequestCredential(ctx))
}

func testReportSourceReadAs(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, event db.CollaborationRunEvent, report collaborationStoredTestReport, credential ArtifactCredentialSnapshot) error {
	if !access.SourceMember {
		return collaborationError(404, "not_found")
	}
	q := db.New(tx)
	frozen, err := loadFrozenTestSubject(ctx, q, event.RunID, access.Issue.ID, report.Identity)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(frozen.Subject, report.Report.Subject) {
		return collaborationError(503, "test_report_unavailable")
	}
	return testReportFrozenSourceReadAs(ctx, tx, access, frozen, credential)
}

func testReportFrozenSourceRead(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, frozen collaborationFrozenTestSubject) error {
	return testReportFrozenSourceReadAs(ctx, tx, access, frozen, completionRequestCredential(ctx))
}

func testReportFrozenSourceReadAs(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, frozen collaborationFrozenTestSubject, credential ArtifactCredentialSnapshot) error {
	return stageArtifactSourceReadAs(ctx, tx, access, frozen.Binding.Code, credential)
}

func stageArtifactSourceReadAs(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, content stageCodeContent, credential ArtifactCredentialSnapshot) error {
	q := db.New(tx)
	artifact := content.Baseline.Artifact
	if artifact == nil {
		return nil
	}
	child, err := q.GetIssue(ctx, util.MustParseUUID(content.Baseline.ChildIssueID))
	if stageArtifactUnavailable(err) || err == nil && child.WorkspaceID != access.Issue.WorkspaceID {
		return collaborationError(404, "not_found")
	}
	if err != nil {
		return err
	}
	execution, err := q.GetAgentArtifactExecutionByApproval(ctx, util.MustParseUUID(artifact.ApprovalID))
	if stageArtifactUnavailable(err) {
		return collaborationError(404, "not_found")
	}
	if err != nil {
		return err
	}
	artifactAccess := ArtifactAccess{Credential: credential, RequestedWorkspaceID: util.UUIDToString(access.Issue.WorkspaceID)}
	if _, readProjection := tx.(collaborationStageEvidenceReadTx); readProjection {
		execution, _, _, err = readDeliveryArtifactInTx(ctx, tx, artifactAccess, child, execution)
	} else {
		execution, _, _, err = lockDeliveryArtifactInTx(ctx, tx, artifactAccess, child, execution)
	}
	if stageArtifactUnavailable(err) {
		return collaborationError(404, "not_found")
	}
	if err != nil {
		return err
	}
	original := content.Source.ArtifactObservation
	if original == nil || execution.OutputArtifactDigest != artifact.ArtifactDigest || util.UUIDToString(execution.ExecutionID) != original.ExecutionID || execution.SpecDigest != original.SpecDigest || execution.ReceiptDigest != original.ReceiptDigest || execution.TaskGeneration != original.ProducerTaskGeneration {
		return collaborationError(404, "not_found")
	}
	err = CheckArtifactCredentialInTx(ctx, tx, credential)
	if stageArtifactUnavailable(err) {
		return collaborationError(404, "not_found")
	}
	return err
}
