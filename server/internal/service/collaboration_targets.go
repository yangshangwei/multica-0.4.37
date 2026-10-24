package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationRegisterTargetParams struct {
	CollaborationQueueRequest
	Kind                     collaboration.TargetKind
	ResourceID               pgtype.UUID
	ExpectedTargetSetVersion int64
	Roster                   []collaboration.TargetResource
}

type CollaborationRevokeGrantParams struct {
	CollaborationQueueRequest
	GrantID            pgtype.UUID
	ExpectedGrantEpoch int64
}

// RevokeGrant withdraws only the actual actor's retained grant. Stopping its
// dependent group does not cancel other participants' waiting root inputs.
func (s *CollaborationService) RevokeGrant(ctx context.Context, p CollaborationRevokeGrantParams) (CollaborationQueueReceipt, error) {
	if !p.GrantID.Valid || p.ExpectedGrantEpoch < 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_grant_revocation")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "revoke_grant", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		grants, err := t.q.LockCollaborationOwnerGrants(ctx, db.LockCollaborationOwnerGrantsParams{IssueID: p.IssueID, OwnerUserID: t.actor})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		var selected db.CollaborationGrant
		for _, grant := range grants {
			if grant.ID == p.GrantID {
				selected = grant
				break
			}
		}
		if !selected.ID.Valid {
			return t.receipt("not_found", http.StatusNotFound), nil
		}
		if selected.Epoch != p.ExpectedGrantEpoch {
			return t.receipt("stale_grant", http.StatusConflict), nil
		}
		if selected.RevokedAt.Valid {
			return t.receipt("grant_already_revoked", http.StatusOK), nil
		}
		if _, err = t.q.RevokeCollaborationGrant(ctx, db.RevokeCollaborationGrantParams{GrantID: selected.ID, IssueID: p.IssueID, Reason: "owner_revoked"}); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		stopped, err := t.stopInvalidatedWork(ctx, pgtype.UUID{}, 0, []db.CollaborationGrant{selected}, "owner_revoked")
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		r := t.receipt("grant_revoked", http.StatusOK)
		if stopped != nil {
			r = *stopped
			r.Code = "grant_revoked"
		}
		return r, t.event(ctx, "grant_revoked", r)
	})
}

// RegisterTarget records a pending configuration. It never grants consent,
// enrolls an owner, or resumes either control scope. The public target/consent
// API is independent of the still-unavailable shared launch protocol.
func (s *CollaborationService) RegisterTarget(ctx context.Context, p CollaborationRegisterTargetParams) (CollaborationQueueReceipt, error) {
	if p.ExpectedTargetSetVersion < 1 || !p.ResourceID.Valid || !validCollaborationRoster(p.Kind, p.ResourceID, p.Roster) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_target_configuration")
	}
	p.Roster = append([]collaboration.TargetResource(nil), p.Roster...)
	sort.Slice(p.Roster, func(i, j int) bool { return p.Roster[i].AgentID < p.Roster[j].AgentID })
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "register_target", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		return t.registerTarget(ctx, p)
	})
}

// Both direct registration and adoption borrow this exact primitive. Only the
// queue transaction owner may commit the target, channel, event and operation.
func (t *collaborationQueueTx) registerTarget(ctx context.Context, p CollaborationRegisterTargetParams) (CollaborationQueueReceipt, error) {
	if !t.access.Capabilities.Invite {
		return t.receipt("forbidden", http.StatusForbidden), nil
	}
	if !t.access.Capabilities.SubmitCommand {
		return t.receipt("issue_closed", http.StatusConflict), nil
	}
	if t.session.TargetSetVersion != p.ExpectedTargetSetVersion {
		return t.receipt("stale_scope", http.StatusConflict), nil
	}
	for _, existing := range t.targets {
		if existing.Kind == string(p.Kind) && existing.ResourceID == p.ResourceID {
			return t.targetReceipt("target_exists", http.StatusConflict, existing), nil
		}
	}
	if code, err := t.validateResourceOwners(ctx, p.Roster); code != "" || err != nil {
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		return t.receipt(code, http.StatusConflict), nil
	}
	if err := lockCollaborationResources(ctx, t.q, p.Kind, p.ResourceID, p.Roster); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	configuration, code, err := collaborationConfigurationSnapshot(ctx, t.q, p.Kind, p.ResourceID, p.Roster)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if code != "" {
		return t.receipt(code, http.StatusConflict), nil
	}
	roster, err := json.Marshal(p.Roster)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	target, err := t.q.CreateConsentedCollaborationTarget(ctx, db.CreateConsentedCollaborationTargetParams{SessionID: t.session.ID,
		Kind: string(p.Kind), ResourceID: p.ResourceID, Roster: roster, ConfigurationFingerprint: pgtype.Text{String: configuration.Fingerprint, Valid: true}})
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	t.targets = append(t.targets, target)
	if _, err = t.ensureTargetChannel(ctx, target); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	target, _ = t.target(target.ID)
	t.session, err = t.q.AdvanceCollaborationTargetSet(ctx, t.session.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	r := t.targetReceipt("target_registered", http.StatusCreated, target)
	return r, t.event(ctx, "target_registered", r)
}

func validCollaborationRoster(kind collaboration.TargetKind, resourceID pgtype.UUID, roster []collaboration.TargetResource) bool {
	if len(roster) == 0 || len(roster) > 256 {
		return false
	}
	leaders := 0
	seen := make(map[pgtype.UUID]bool, len(roster))
	for _, resource := range roster {
		agent, err := util.ParseUUID(resource.AgentID)
		if err != nil || seen[agent] {
			return false
		}
		seen[agent] = true
		for _, id := range []string{resource.RuntimeID, resource.ExecutionWorkspaceID, resource.OwnerUserID} {
			if _, err := util.ParseUUID(id); err != nil {
				return false
			}
		}
		switch kind {
		case collaboration.TargetAgent:
			if len(roster) != 1 || resource.Role != "agent" || agent != resourceID {
				return false
			}
		case collaboration.TargetSquad:
			if resource.Role == "leader" {
				leaders++
			} else if resource.Role != "member" {
				return false
			}
		default:
			return false
		}
	}
	return kind == collaboration.TargetAgent || leaders == 1
}

func decodeCollaborationRoster[T any](data []byte) ([]T, error) {
	var roster []T
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&roster); err != nil {
		return nil, errors.New("invalid collaboration resource roster")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("invalid collaboration resource roster trailing data")
	}
	return roster, nil
}

func (t *collaborationQueueTx) target(id pgtype.UUID) (db.CollaborationTarget, bool) {
	for _, target := range t.targets {
		if target.ID == id {
			return target, true
		}
	}
	return db.CollaborationTarget{}, false
}

func (t *collaborationQueueTx) replaceTarget(target db.CollaborationTarget) {
	for i := range t.targets {
		if t.targets[i].ID == target.ID {
			t.targets[i] = target
			return
		}
	}
}

func collaborationTargetHoldReason(target db.CollaborationTarget) *collaboration.TargetHoldReason {
	if !target.HoldReason.Valid {
		return nil
	}
	reason := collaboration.TargetHoldReason(target.HoldReason.String)
	return &reason
}

// setTargetHold borrows the source/channel/target locks. A repeated reason is
// inert unless a new consent requires a fresh observation fence. It never
// changes channel controls, target epochs, grants or retained execution proof.
func (t *collaborationQueueTx) setTargetHold(ctx context.Context, target db.CollaborationTarget, reason collaboration.TargetHoldReason, advanceVersion bool) (db.CollaborationTarget, error) {
	if target.HoldVersion < 1 || reason != "" && !reason.Valid() {
		return db.CollaborationTarget{}, invalidCollaborationLedger("invalid target hold")
	}
	hold := pgtype.Text{String: string(reason), Valid: reason != ""}
	if target.HoldReason == hold && !advanceVersion {
		return target, nil
	}
	updated, err := t.q.SetCollaborationTargetHold(ctx, db.SetCollaborationTargetHoldParams{
		SessionID: t.session.ID, TargetID: target.ID, HoldReason: hold, AdvanceVersion: advanceVersion,
	})
	if err == nil {
		t.replaceTarget(updated)
	}
	return updated, err
}

func collaborationTargetHoldResult(target db.CollaborationTarget, code string) CollaborationTargetHoldResult {
	return CollaborationTargetHoldResult{TargetID: target.ID, ChannelID: target.ChannelID, Code: code,
		HoldReason: collaborationTargetHoldReason(target), HoldVersion: target.HoldVersion}
}

func (t *collaborationQueueTx) targetReceipt(code string, status int, target db.CollaborationTarget) CollaborationQueueReceipt {
	r := t.receipt(code, status)
	r.Scope, r.TargetID, r.GroupID = collaboration.ControlTarget, target.ID, target.ActiveGroupID
	r.ChannelID = target.ChannelID
	r.QueueState, r.ControlVersion = target.QueueState, target.ControlVersion
	return r
}

func collaborationResourceMatchesGrant(resource collaboration.TargetResource, grant db.CollaborationGrant) bool {
	return resource.Role == grant.Role.String && resource.AgentID == util.UUIDToString(grant.AgentID) &&
		resource.RuntimeID == util.UUIDToString(grant.RuntimeID) && resource.OwnerUserID == util.UUIDToString(grant.OwnerUserID) &&
		resource.ExecutionWorkspaceID == util.UUIDToString(grant.ExecutionWorkspaceID)
}

// Input authorization is narrower than runtime admission: the complete current
// roster needs owner consent, but offline/paused resources may retain waiting
// inputs. No check here is sufficient to claim, launch or resume execution.
func (t *collaborationQueueTx) authorizeTargetInput(ctx context.Context, target db.CollaborationTarget) (string, error) {
	return lockAndAuthorizeCollaborationTargetInput(ctx, t.q, t.access.Issue.ID, target)
}

var errInvalidCollaborationLedger = errors.New("invalid collaboration execution ledger")

func invalidCollaborationLedger(detail string) error {
	return fmt.Errorf("%w: %s", errInvalidCollaborationLedger, detail)
}
