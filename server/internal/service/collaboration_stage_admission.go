package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Stage admission for ordinary issues. An ordinary issue is governed by its
// nearest collaboration ancestor; the gates apply when that root is
// stage-managed (an enabled project stage policy), at any depth below it.
// Every other issue, and every human manual creation, keeps its previous
// behaviour. A lookup failure is returned: the gate fails closed.

// stageAncestorDepth bounds every parent walk. It also ends a parent cycle.
const stageAncestorDepth = 64

func stageProposalRequired() error {
	return DeliveryCompletionError{Status: http.StatusConflict, Code: "stage_proposal_required"}
}

// stageGoverningRoot resolves the nearest collaboration ancestor of an issue,
// the issue itself included, and whether it is stage-managed. unresolved means
// the bounded walk was cut (a very deep chain or a parent cycle): machine paths
// must not treat that as "outside every managed root".
func stageGoverningRoot(ctx context.Context, q *db.Queries, issueID pgtype.UUID) (stageRoot, bool, error) {
	if !issueID.Valid {
		return stageRoot{}, false, nil
	}
	row, err := q.GetCollaborationStageGoverningRoot(ctx, db.GetCollaborationStageGoverningRootParams{IssueID: issueID, MaxDepth: stageAncestorDepth})
	if err != nil {
		return stageRoot{}, false, err
	}
	if row.Unresolved {
		return stageRoot{}, true, nil
	}
	if !row.RootIssueID.Valid || row.RootIssueID.Bytes == [16]byte{} {
		return stageRoot{}, false, nil
	}
	root, err := loadStageRoot(ctx, q, row.RootIssueID)
	return root, false, err
}

// checkStageMachineParent refuses a machine-driven child anywhere below a
// managed root. Decomposition below an approved child is decomposition too.
func checkStageMachineParent(ctx context.Context, q *db.Queries, parent pgtype.UUID) error {
	root, unresolved, err := stageGoverningRoot(ctx, q, parent)
	if err != nil {
		return err
	}
	if unresolved {
		return DeliveryCompletionError{Status: http.StatusConflict, Code: "stage_ancestry_unresolved"}
	}
	if root.managed {
		return stageProposalRequired()
	}
	return nil
}

// checkStageChildCreate runs inside IssueService.CreateInTx for every creation
// entry point. Machine-driven creation is classified from the verified
// credential and the server-resolved creator; a human keeps manual creation,
// which approves nothing and creates no proposal mapping.
func checkStageChildCreate(ctx context.Context, q *db.Queries, p IssueCreateParams, parent db.Issue) error {
	if !p.ParentIssueID.Valid {
		if p.stagePermit != nil {
			return errors.New("stage materialization requires its root as parent")
		}
		return nil
	}
	if p.stagePermit != nil {
		// The permit is bound to one root by the acceptance transaction.
		if p.stagePermit.root != parent.ID {
			return errors.New("stage materialization permit does not match the parent")
		}
		return nil
	}
	principal, authenticated := collaboration.PrincipalFromContext(ctx)
	if !collaboration.StageMachineCreation(p.CreatorType, principal, authenticated) {
		return nil
	}
	return checkStageMachineParent(ctx, q, parent.ID)
}

// CheckStageChildUpdateInTx runs inside the ordinary update transaction
// (Update, Move and Batch) after the issue row lock. A machine cannot attach an
// issue anywhere below a managed root; a human keeps that right. For a mapped child the
// mapping stays keyed by the child, so removing or replacing the parent does
// not erase it, and a content/project/parent change advances the child's
// semantic version while an ordinary status change does not.
func CheckStageChildUpdateInTx(ctx context.Context, tx pgx.Tx, before, candidate db.Issue, agentActor bool) error {
	q := db.New(tx)
	if candidate.ParentIssueID.Valid && candidate.ParentIssueID != before.ParentIssueID {
		principal, authenticated := collaboration.PrincipalFromContext(ctx)
		creator := "member"
		if agentActor {
			creator = "agent"
		}
		if collaboration.StageMachineCreation(creator, principal, authenticated) {
			if err := checkStageMachineParent(ctx, q, candidate.ParentIssueID); err != nil {
				return err
			}
		}
	}
	if candidate.Title == before.Title && candidate.Description == before.Description &&
		candidate.ProjectID == before.ProjectID && candidate.ParentIssueID == before.ParentIssueID {
		return nil
	}
	_, err := q.BumpCollaborationStageChildSemanticVersion(ctx, before.ID)
	return err
}

// stageChildExecution resolves the gate facts for one ordinary issue. The root
// comes from the persistent mapping when one exists, otherwise from the nearest
// collaboration ancestor of an agent-created issue.
func stageChildExecution(ctx context.Context, q *db.Queries, issueID pgtype.UUID) (collaboration.StageChildFacts, pgtype.UUID, error) {
	var facts collaboration.StageChildFacts
	if !issueID.Valid {
		return facts, pgtype.UUID{}, nil
	}
	mapping, err := q.GetCollaborationStageChild(ctx, issueID)
	mapped := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return facts, pgtype.UUID{}, err
	}
	machine := mapped && mapping.Origin == "materialized"
	var issue db.Issue
	if !machine {
		if issue, err = q.GetIssue(ctx, issueID); err != nil {
			return facts, pgtype.UUID{}, err
		}
		if issue.CreatorType != "agent" {
			return facts, pgtype.UUID{}, nil
		}
	}
	var root stageRoot
	if mapped {
		root, err = loadStageRoot(ctx, q, mapping.RootIssueID)
	} else {
		var unresolved bool
		if root, unresolved, err = stageGoverningRoot(ctx, q, issue.ParentIssueID); err == nil && unresolved {
			err = collaborationError(http.StatusConflict, "stage_ancestry_unresolved")
		}
	}
	if err != nil || !root.managed {
		return facts, pgtype.UUID{}, err
	}
	facts = collaboration.StageChildFacts{Managed: true, MachineOrigin: true, Mapped: mapped,
		ApprovedLifecycle: mapping.LifecycleEpoch, RootLifecycle: root.row.LifecycleEpoch}
	return facts, root.row.ID, nil
}

// checkStageChildExecution is the shared enqueue gate. It returns the managed
// root when the gate applies, so a transaction owner can fence it.
func checkStageChildExecution(ctx context.Context, q *db.Queries, issueID pgtype.UUID) (pgtype.UUID, error) {
	facts, root, err := stageChildExecution(ctx, q, issueID)
	if err != nil {
		return pgtype.UUID{}, err
	}
	if code := collaboration.StageChildExecutionRefusal(facts); code != "" {
		return root, collaborationError(http.StatusConflict, code)
	}
	return root, nil
}

// lockStageChildRoot is the pre-collection half of the start fence: before the
// child row lock, a gated child takes a shared lock on its root's lifecycle row
// (root before child). An ungated issue takes no lock.
func lockStageChildRoot(ctx context.Context, q *db.Queries, issueID pgtype.UUID) (pgtype.UUID, error) {
	_, root, err := stageChildExecution(ctx, q, issueID)
	if err != nil || !root.Valid {
		return pgtype.UUID{}, err
	}
	if _, err = q.LockCollaborationStageRootShared(ctx, root); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, err
	}
	return root, nil
}

// checkStageChildExecutionLocked decides inside the owner's transaction under a
// shared lock on the root lifecycle, so a reopen either committed before this
// decision or waits for this transaction. fenced names the root already locked
// before the child row; any other root is locked NOWAIT because the child row
// is held, and contention is returned as the lock error for the owner's retry.
func checkStageChildExecutionLocked(ctx context.Context, q *db.Queries, issueID, fenced pgtype.UUID) error {
	_, root, err := stageChildExecution(ctx, q, issueID)
	if err != nil || !root.Valid {
		return err
	}
	if root != fenced {
		if _, err = q.TryLockCollaborationStageRootShared(ctx, root); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	_, err = checkStageChildExecution(ctx, q, issueID)
	return err
}
