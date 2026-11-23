package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Only verified identity metadata is retained. PATHash is a non-bearer lookup
// fingerprint, never a token; sqlc and every public DTO exclude this column.
type stageSignerCredential struct {
	Version         int                   `json:"version"`
	Kind            auth.CredentialKind   `json:"kind"`
	ExpiresAt       *time.Time            `json:"expires_at"`
	PATID           string                `json:"pat_id,omitempty"`
	PATHash         string                `json:"pat_hash,omitempty"`
	SourceMemberID  string                `json:"source_member_id,omitempty"`
	PasswordSession *auth.PasswordSession `json:"password_session,omitempty"`
}

func stageCredentialFailure(err error) error {
	var denied ArtifactError
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, auth.ErrPasswordSession) || (errors.As(err, &denied) && denied.Status == http.StatusForbidden) {
		return collaborationError(http.StatusForbidden, "forbidden")
	}
	return err
}

func freezeStageSigner(ctx context.Context, tx pgx.Tx, workspace pgtype.UUID) (stageSignerCredential, []byte, error) {
	c := completionRequestCredential(ctx)
	if kind, _, err := c.Actor(); err != nil || kind != "human" {
		return stageSignerCredential{}, nil, artifactDeny()
	}
	if err := CheckArtifactCredentialInTx(ctx, tx, c); err != nil {
		return stageSignerCredential{}, nil, stageCredentialFailure(err)
	}
	s := stageSignerCredential{Version: 1, Kind: c.Principal.Kind, PasswordSession: c.PasswordSession}
	// Membership existence alone cannot distinguish removal followed by a new
	// grant. Freeze the original server-created member row under its live lock.
	members, err := db.New(tx).LockArtifactMembers(ctx, db.LockArtifactMembersParams{WorkspaceID: workspace, UserIds: []pgtype.UUID{util.MustParseUUID(c.Principal.UserID)}})
	if err != nil {
		return s, nil, err
	}
	if len(members) == 1 {
		s.SourceMemberID = util.UUIDToString(members[0].ID)
	}
	if c.Principal.Kind == auth.CredentialPAT {
		row, err := db.New(tx).LockArtifactPAT(ctx, db.LockArtifactPATParams{TokenHash: c.Principal.PATTokenHash, UserID: util.MustParseUUID(c.Principal.UserID)})
		if err != nil {
			return s, nil, err
		}
		s.PATID, s.PATHash = util.UUIDToString(row.ID), row.TokenHash
		if row.ExpiresAt.Valid {
			v := row.ExpiresAt.Time.UTC()
			s.ExpiresAt = &v
		}
	} else {
		v := c.Principal.ExpiresAt.UTC()
		s.ExpiresAt = &v
	}
	raw, err := json.Marshal(s)
	return s, raw, err
}

func (s stageSignerCredential) credential(user string) ArtifactCredentialSnapshot {
	c := ArtifactCredentialSnapshot{Principal: auth.AuthenticatedPrincipal{UserID: user, Kind: s.Kind, PATTokenHash: s.PATHash}, PasswordSession: s.PasswordSession}
	if s.ExpiresAt != nil {
		c.Principal.ExpiresAt = *s.ExpiresAt
	}
	return c
}

func stageRoleHolder(role *db.CollaborationStageRole, step string) (pgtype.UUID, string) {
	if role == nil {
		return pgtype.UUID{}, ""
	}
	switch step {
	case "code_review":
		return role.CodeReviewerUserID, "code_reviewer"
	case "qa":
		return role.QaApproverUserID, "qa_approver"
	case "release", "rollback":
		return role.ReleaseApproverUserID, "release_approver"
	default:
		return pgtype.UUID{}, ""
	}
}

// Read hints use the same retained issuer identity as consumption. No consumer
// credential is ever substituted for an expired or revoked signer credential.
func stageSignerCurrent(ctx context.Context, q *db.Queries, role *db.CollaborationStageRole, d db.CollaborationStageDecision, now time.Time) (bool, stageSignerCredential, error) {
	var signer stageSignerCredential
	holder, roleName := stageRoleHolder(role, d.Step)
	if role == nil || holder != d.ActorUserID || roleName != d.Role || role.Revision != d.RoleRevision {
		return false, signer, nil
	}
	if json.Unmarshal(d.SignerCredential, &signer) != nil || signer.Version != 1 {
		return false, signer, nil
	}
	if string(signer.Kind) != d.CredentialKind {
		return false, signer, nil
	}
	if signer.Kind != auth.CredentialJWT && signer.Kind != auth.CredentialPAT {
		return false, signer, nil
	}
	if signer.Kind == auth.CredentialJWT && signer.ExpiresAt == nil {
		return false, signer, nil
	}
	if signer.ExpiresAt != nil && !now.Before(*signer.ExpiresAt) {
		return false, signer, nil
	}
	user, err := q.GetUser(ctx, d.ActorUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, signer, nil
	}
	if err != nil {
		return false, signer, err
	}
	if user.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(user.ID), user.Email.String) {
		return false, signer, nil
	}
	p, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: d.IssueID, UserID: d.ActorUserID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, signer, nil
	}
	if err != nil {
		return false, signer, err
	}
	if p.RevokedAt.Valid || p.Epoch != d.ParticipantEpoch {
		return false, signer, nil
	}
	account, err := q.GetCollaborationAccountFence(ctx, d.ActorUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, signer, nil
	}
	if err != nil {
		return false, signer, err
	}
	if account.Epoch != d.AccountEpoch {
		return false, signer, nil
	}
	if d.Decision == "approved" {
		member, memberErr := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: d.ActorUserID, WorkspaceID: d.WorkspaceID})
		err = memberErr
		if errors.Is(err, pgx.ErrNoRows) {
			return false, signer, nil
		}
		if err != nil {
			return false, signer, err
		}
		if !stageUUID(signer.SourceMemberID) || util.UUIDToString(member.ID) != signer.SourceMemberID {
			return false, signer, nil
		}
	}
	if signer.Kind == auth.CredentialPAT {
		if !stageUUID(signer.PATID) || signer.PATHash == "" {
			return false, signer, nil
		}
		token, err := q.GetCollaborationStageSignerPAT(ctx, db.GetCollaborationStageSignerPATParams{ID: util.MustParseUUID(signer.PATID), UserID: d.ActorUserID, TokenHash: signer.PATHash})
		if errors.Is(err, pgx.ErrNoRows) {
			return false, signer, nil
		}
		if err != nil {
			return false, signer, err
		}
		if token.ExpiresAt.Valid && (signer.ExpiresAt == nil || token.ExpiresAt.Time.Before(*signer.ExpiresAt)) {
			expiry := token.ExpiresAt.Time
			signer.ExpiresAt = &expiry
		}
	}
	if signer.PasswordSession != nil {
		session := *signer.PasswordSession
		if session.UserID != util.UUIDToString(d.ActorUserID) || session.Setup || session.Change {
			return false, signer, nil
		}
		if _, err = auth.CheckPasswordSessionVersion(ctx, q, session); err != nil {
			if errors.Is(err, auth.ErrPasswordSession) || errors.Is(err, pgx.ErrNoRows) {
				return false, signer, nil
			}
			return false, signer, err
		}
	} else if auth.PasswordMode() {
		return false, signer, nil
	}
	return true, signer, nil
}

func stageMaxExpiry(now time.Time, ci CollaborationStageObservedCI, credentialExpiry *time.Time, content stageCodeContent) time.Time {
	maximum := now.Add(stageApprovalHorizon)
	if ci.ObservedAt != nil && ci.ObservedAt.Add(stageCIHorizon).Before(maximum) {
		maximum = ci.ObservedAt.Add(stageCIHorizon)
	}
	if credentialExpiry != nil && credentialExpiry.Before(maximum) {
		maximum = *credentialExpiry
	}
	if a := content.Source.ArtifactObservation; a != nil && a.ValidUntil != nil && a.ValidUntil.Before(maximum) {
		maximum = *a.ValidUntil
	}
	return maximum.UTC().Truncate(time.Microsecond)
}

// Consume retained signer identities only after slow evidence observers finish.
// The returned database time also bounds time-sensitive evidence at the writer.
func lockStageFinalSigners(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, state *collaborationStageState, decisions []db.CollaborationStageDecision) ([]stageSignerCredential, string, error) {
	q := db.New(tx)
	signers := make([]stageSignerCredential, 0, len(decisions))
	for _, d := range decisions {
		now, err := q.ArtifactExecutionDatabaseTime(ctx)
		if err != nil {
			return nil, "", err
		}
		if d.ExpiresAt.Valid && !now.Time.Before(d.ExpiresAt.Time) {
			return nil, "stage_decision_expired", nil
		}
		if d.LifecycleEpoch != access.LifecycleEpoch {
			return nil, "stage_evidence_changed", nil
		}
		if d.RoleRevision != state.task.RoleRevision {
			return nil, "stage_roles_changed", nil
		}
		if d.PolicyRevision != state.policy.Revision {
			return nil, "stage_policy_changed", nil
		}
		current, signer, err := stageSignerCurrent(ctx, q, state.role, d, now.Time)
		if err != nil {
			return nil, "", err
		}
		if !current {
			return nil, "stage_signer_changed", nil
		}
		if d.Decision == "approved" {
			members, err := q.LockArtifactMembers(ctx, db.LockArtifactMembersParams{WorkspaceID: access.Issue.WorkspaceID, UserIds: []pgtype.UUID{d.ActorUserID}})
			if err != nil {
				return nil, "", err
			}
			if len(members) != 1 || util.UUIDToString(members[0].ID) != signer.SourceMemberID {
				return nil, "stage_signer_changed", nil
			}
		}
		if err = CheckArtifactCredentialInTx(ctx, tx, signer.credential(util.UUIDToString(d.ActorUserID))); err != nil {
			var denied *CollaborationError
			if errors.As(stageCredentialFailure(err), &denied) && denied.Code == "forbidden" {
				return nil, "stage_signer_changed", nil
			}
			return nil, "", err
		}
		if d.Decision == "approved" {
			candidate, err := q.GetCollaborationStageCodeCandidate(ctx, db.GetCollaborationStageCodeCandidateParams{IssueID: d.IssueID, CandidateID: d.CandidateID})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, "stage_signer_changed", nil
			}
			if err != nil {
				return nil, "", err
			}
			content, err := decodeStageCode(candidate)
			if err != nil {
				return nil, "", err
			}
			if err = stageArtifactSourceReadAs(ctx, tx, access, content, signer.credential(util.UUIDToString(d.ActorUserID))); err != nil {
				if collaborationErrorCodeForReport(err, "not_found") {
					return nil, "stage_signer_changed", nil
				}
				return nil, "", err
			}
		}
		signers = append(signers, signer)
	}
	return signers, "", nil
}

func checkStageFinalAuthority(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, state *collaborationStageState, decisions []db.CollaborationStageDecision) (time.Time, string, error) {
	now, _, code, err := checkStageFinalAuthorityWithFinalCheck(ctx, tx, access, state, decisions, nil)
	return now, code, err
}

func checkStageFinalAuthorityWithFinalCheck(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, state *collaborationStageState, decisions []db.CollaborationStageDecision, finalCheck func() (time.Time, error)) (time.Time, time.Time, string, error) {
	q := db.New(tx)
	signers, code, err := lockStageFinalSigners(ctx, tx, access, state, decisions)
	if err != nil || code != "" {
		return time.Time{}, time.Time{}, code, err
	}
	caller := completionRequestCredential(ctx)
	if err := CheckArtifactCredentialInTx(ctx, tx, caller); err != nil {
		var denied *CollaborationError
		if errors.As(stageCredentialFailure(err), &denied) && denied.Code == "forbidden" {
			return time.Time{}, time.Time{}, "forbidden", nil
		}
		return time.Time{}, time.Time{}, "", err
	}
	var callerExpiry *time.Time
	if caller.Principal.Kind == auth.CredentialJWT {
		v := caller.Principal.ExpiresAt
		callerExpiry = &v
	} else {
		token, err := q.LockArtifactPAT(ctx, db.LockArtifactPATParams{TokenHash: caller.Principal.PATTokenHash, UserID: util.MustParseUUID(caller.Principal.UserID)})
		if err != nil {
			return time.Time{}, time.Time{}, "", err
		}
		if token.ExpiresAt.Valid {
			v := token.ExpiresAt.Time
			callerExpiry = &v
		}
	}
	var extraExpiry time.Time
	if finalCheck != nil {
		if extraExpiry, err = finalCheck(); err != nil {
			return time.Time{}, time.Time{}, "", err
		}
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	if callerExpiry != nil && !now.Time.Before(*callerExpiry) {
		return now.Time, time.Time{}, "forbidden", nil
	}
	if !extraExpiry.IsZero() && !now.Time.Before(extraExpiry) {
		return now.Time, time.Time{}, "stage_evidence_changed", nil
	}
	expires := extraExpiry
	if callerExpiry != nil {
		if expires.IsZero() || callerExpiry.Before(expires) {
			expires = *callerExpiry
		}
	}
	for index, decision := range decisions {
		if decision.ExpiresAt.Valid && !now.Time.Before(decision.ExpiresAt.Time) {
			return now.Time, time.Time{}, "stage_decision_expired", nil
		}
		if signers[index].ExpiresAt != nil && !now.Time.Before(*signers[index].ExpiresAt) {
			return now.Time, time.Time{}, "stage_signer_changed", nil
		}
		if decision.ExpiresAt.Valid && (expires.IsZero() || decision.ExpiresAt.Time.Before(expires)) {
			expires = decision.ExpiresAt.Time
		}
		if signers[index].ExpiresAt != nil && (expires.IsZero() || signers[index].ExpiresAt.Before(expires)) {
			expires = *signers[index].ExpiresAt
		}
	}
	return now.Time, expires, "", nil
}
