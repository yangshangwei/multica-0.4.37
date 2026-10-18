package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type CollaborationRuntimeCommentParams struct {
	CommentID pgtype.UUID `json:"comment_id"`
	ParentID  pgtype.UUID `json:"parent_id"`
	Content   string      `json:"content"`
}

type CollaborationRuntimeCommentReceipt struct {
	CommentID         pgtype.UUID   `json:"comment_id"`
	RunIDs            []pgtype.UUID `json:"run_ids"`
	ContinuationRunID pgtype.UUID   `json:"continuation_run_id"`
}

type CollaborationRuntimeMember struct {
	AgentID pgtype.UUID `json:"agent_id"`
	Name    string      `json:"name"`
	Role    string      `json:"role"`
}

type CollaborationRuntimeComment struct {
	ID         pgtype.UUID `json:"id"`
	AuthorType string      `json:"author_type"`
	AuthorID   pgtype.UUID `json:"author_id"`
	Content    string      `json:"content"`
	ParentID   pgtype.UUID `json:"parent_id"`
	CreatedAt  time.Time   `json:"created_at"`
}

type CollaborationRuntimeContext struct {
	Identity        protocol.CollaborationRunIdentity `json:"identity"`
	IssueID         pgtype.UUID                       `json:"issue_id"`
	Title           string                            `json:"title"`
	Description     string                            `json:"description"`
	AgentID         pgtype.UUID                       `json:"agent_id"`
	SquadID         pgtype.UUID                       `json:"squad_id"`
	StepKind        string                            `json:"step_kind"`
	Roster          []CollaborationRuntimeMember      `json:"roster"`
	Inputs          []protocol.CollaborationRunInput  `json:"inputs"`
	Comments        []CollaborationRuntimeComment     `json:"comments"`
	DispatchComment string                            `json:"dispatch_comment"`
}

// This lookup never turns the runtime owner's identity into human authority.
// The complete current predicate is checked again inside every business call.
func (s *CollaborationService) ResolveRuntimeBusinessRun(ctx context.Context, token string) (db.CollaborationRun, error) {
	var out db.CollaborationRun
	if !strings.HasPrefix(token, CollaborationBusinessTokenPrefix) || len(token) != 68 {
		return out, collaborationError(http.StatusForbidden, "business_capability_required")
	}
	id, err := s.Queries.GetCollaborationBusinessRunByHash(ctx, collaborationCapabilityHash(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, collaborationError(http.StatusForbidden, "business_capability_required")
	}
	if err != nil {
		return out, err
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (db.CollaborationRun, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, id, token)
		if err != nil {
			return out, err
		}
		return r.run, nil
	})
}

func (s *CollaborationService) lockRuntimeBusiness(ctx context.Context, tx pgx.Tx, runID pgtype.UUID, token string) (*collaborationRuntimeState, error) {
	if !strings.HasPrefix(token, CollaborationBusinessTokenPrefix) || len(token) != 68 {
		return nil, collaborationError(http.StatusForbidden, "business_capability_required")
	}
	r, err := s.lockCollaborationRuntimeRun(ctx, tx, runID, true, false)
	if err != nil {
		return nil, err
	}
	capability, err := r.q.GetCollaborationBusinessCapabilityByHash(ctx, db.GetCollaborationBusinessCapabilityByHashParams{RunID: runID, TokenHash: collaborationCapabilityHash(token)})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (capability.RevokedAt.Valid || capability.AttemptSeq != r.run.AuthorizedAttemptSeq || !collaborationCapabilityMatches(token, CollaborationBusinessTokenPrefix, capability.TokenHash)) {
		return nil, collaborationError(http.StatusForbidden, "business_capability_required")
	}
	if err != nil {
		return nil, err
	}
	if err = r.currentAdmission(ctx); err != nil {
		return nil, err
	}
	if err = r.requireReservation(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *collaborationRuntimeState) requireReservation(ctx context.Context) error {
	reservation, err := r.q.GetCollaborationRuntimeReservation(ctx, r.run.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return collaborationError(http.StatusConflict, "runtime_reservation_required")
	}
	if err != nil {
		return err
	}
	if reservation.ReleasedAt.Valid || reservation.RuntimeID != r.run.RuntimeID || reservation.AgentID != r.run.AgentID || reservation.BindingID != r.run.BindingID ||
		reservation.InstallationID != r.run.InstallationID || reservation.ExecutionWorkspaceID != r.run.ExecutionWorkspaceID {
		return invalidCollaborationLedger("runtime reservation identity mismatch")
	}
	return nil
}

func (r *collaborationRuntimeState) reserveRun(ctx context.Context) error {
	if _, err := r.q.GetCollaborationRuntimeReservation(ctx, r.run.ID); err == nil {
		return r.requireReservation(ctx)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	runtime, err := (RuntimeLookup{Queries: r.q, Source: "task"}).Get(ctx, r.run.RuntimeID)
	if err != nil {
		return err
	}
	if runtime.Status != "online" {
		return collaborationError(http.StatusConflict, "runtime_offline")
	}
	occupied, err := r.q.CollaborationRuntimeNamespaceOccupied(ctx, db.CollaborationRuntimeNamespaceOccupiedParams{WorkspaceID: runtime.WorkspaceID, DaemonID: runtime.DaemonID.String})
	if err != nil {
		return err
	}
	if occupied {
		return collaborationError(http.StatusConflict, "runtime_busy")
	}
	return r.q.CreateCollaborationRuntimeReservation(ctx, db.CreateCollaborationRuntimeReservationParams{RunID: r.run.ID, ExecutionWorkspaceID: runtime.WorkspaceID, AgentID: r.run.AgentID, RuntimeID: runtime.ID, DaemonID: runtime.DaemonID.String, InstallationID: r.run.InstallationID, BindingID: r.run.BindingID})
}

func (s *CollaborationService) RuntimeContext(ctx context.Context, runID pgtype.UUID, token string) (CollaborationRuntimeContext, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (CollaborationRuntimeContext, error) {
		var out CollaborationRuntimeContext
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return out, err
		}
		out = CollaborationRuntimeContext{Identity: collaborationRunIdentity(r.run, r.access.Issue.ID), IssueID: r.access.Issue.ID, Title: r.access.Issue.Title,
			Description: r.access.Issue.Description.String, AgentID: r.run.AgentID, StepKind: r.run.StepKind.String, Roster: []CollaborationRuntimeMember{}, Comments: []CollaborationRuntimeComment{}}
		if r.target.Kind == "squad" {
			out.SquadID = r.target.ResourceID
		}
		roster, err := decodeCollaborationRoster[collaboration.GroupResource](r.group.Roster)
		if err != nil {
			return out, err
		}
		for _, resource := range roster {
			id, err := util.ParseUUID(resource.AgentID)
			if err != nil {
				return out, invalidCollaborationLedger("invalid roster agent")
			}
			agent, err := r.q.GetAgent(ctx, id)
			if err != nil {
				return out, err
			}
			out.Roster = append(out.Roster, CollaborationRuntimeMember{AgentID: id, Name: agent.Name, Role: resource.Role})
		}
		if r.run.PromptSnapshot == nil {
			return out, invalidCollaborationLedger("business run has no claimed prompt")
		}
		out.Inputs, err = decodeCollaborationRoster[protocol.CollaborationRunInput](r.run.PromptSnapshot)
		if err != nil {
			return out, err
		}
		comments, err := r.q.ListCollaborationRuntimeComments(ctx, r.access.Issue.ID)
		if err != nil {
			return out, err
		}
		for _, c := range comments {
			out.Comments = append(out.Comments, CollaborationRuntimeComment{ID: c.ID, AuthorType: c.AuthorType, AuthorID: c.AuthorID, Content: c.Content, ParentID: c.ParentID, CreatedAt: c.CreatedAt.Time})
		}
		delegations, err := r.q.LockCollaborationGroupDelegations(ctx, r.group.ID)
		if err != nil {
			return out, err
		}
		for _, d := range delegations {
			if d.RunID != runID {
				continue
			}
			c, err := r.q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{IssueID: r.access.Issue.ID, WorkspaceID: r.access.Issue.WorkspaceID, CommentID: d.SourceCommentID})
			if err != nil {
				return out, err
			}
			out.DispatchComment = c.Content
		}
		return out, nil
	})
}

func (s *CollaborationService) CreateRuntimeComment(ctx context.Context, runID pgtype.UUID, token string, p CollaborationRuntimeCommentParams) (CollaborationRuntimeCommentReceipt, error) {
	var empty CollaborationRuntimeCommentReceipt
	if !p.CommentID.Valid {
		return empty, collaborationError(http.StatusBadRequest, "invalid_comment_request")
	}
	if err := validateCollaborationCommentInput(CollaborationCommentInput{Content: p.Content, ParentID: p.ParentID}); err != nil {
		return empty, err
	}
	p.Content = util.SanitizeTextForPostgres(p.Content)
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (CollaborationRuntimeCommentReceipt, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return empty, err
		}
		eventID := "comment:" + util.UUIDToString(p.CommentID)
		prior, err := r.q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: runID, ProducerEventID: eventID})
		if err == nil {
			var saved struct {
				Input   CollaborationRuntimeCommentParams  `json:"input"`
				Receipt CollaborationRuntimeCommentReceipt `json:"receipt"`
			}
			if prior.Kind != "comment" || json.Unmarshal(prior.Payload, &saved) != nil || saved.Input != p {
				return empty, collaborationError(http.StatusConflict, "comment_conflict")
			}
			return saved.Receipt, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return empty, err
		}
		if err = validateCollaborationCommentParent(ctx, r.q, r.access, p.ParentID); err != nil {
			return empty, err
		}
		comment, err := r.q.CreateCollaborationRuntimeComment(ctx, db.CreateCollaborationRuntimeCommentParams{ID: p.CommentID, IssueID: r.access.Issue.ID, WorkspaceID: r.access.Issue.WorkspaceID, AuthorID: r.run.AgentID, ParentID: p.ParentID, Content: p.Content})
		if err != nil {
			return empty, err
		}
		out := CollaborationRuntimeCommentReceipt{CommentID: comment.ID, RunIDs: []pgtype.UUID{}}
		mentions := parseCollaborationMentions(p.Content)
		if r.target.Kind == "squad" {
			roster, err := decodeCollaborationRoster[collaboration.GroupResource](r.group.Roster)
			if err != nil {
				return empty, err
			}
			for _, mention := range mentions.Targets {
				if mention.Kind != collaboration.TargetAgent {
					continue
				}
				for _, resource := range roster {
					if resource.AgentID != util.UUIDToString(mention.ResourceID) {
						continue
					}
					if resource.Role == "leader" {
						out.ContinuationRunID, err = r.queueContinuation(ctx, r.run.ID)
					} else {
						var run db.CollaborationRun
						run, err = r.createDerivedRun(ctx, resource, r.run.ID, "member", "comment:"+util.UUIDToString(comment.ID)+":"+resource.AgentID)
						if err == nil {
							out.RunIDs = append(out.RunIDs, run.ID)
							err = r.q.CreateCollaborationCommentDelegation(ctx, db.CreateCollaborationCommentDelegationParams{GroupID: r.group.ID, ParentRunID: r.run.ID, RequestKey: util.UUIDToString(comment.ID) + ":" + resource.AgentID, PayloadHash: collaborationCapabilityHash(p.Content), MemberAgentID: run.AgentID, SourceCommentID: comment.ID, RunID: run.ID})
						}
					}
					if err != nil {
						return empty, err
					}
				}
			}
			if r.run.StepKind.String == "member" && !out.ContinuationRunID.Valid {
				out.ContinuationRunID, err = r.queueContinuation(ctx, r.run.ID)
				if err != nil {
					return empty, err
				}
			}
		}
		payload, err := json.Marshal(struct {
			Input   CollaborationRuntimeCommentParams  `json:"input"`
			Receipt CollaborationRuntimeCommentReceipt `json:"receipt"`
		}{p, out})
		if err != nil {
			return empty, err
		}
		if _, err = r.q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: runID, EventID: eventID, Kind: "comment", Payload: payload}); err != nil {
			return empty, err
		}
		seq, err := r.q.AdvanceCollaborationEventSequence(ctx, r.access.Issue.ID)
		if err != nil {
			return empty, err
		}
		public, err := json.Marshal(map[string]any{"comment_id": comment.ID, "run_id": runID, "group_id": r.group.ID, "author_type": "agent", "author_id": r.run.AgentID})
		if err != nil {
			return empty, err
		}
		if _, err = r.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: r.access.Issue.ID, EventSeq: seq, Kind: "runtime_comment_created", Payload: public}); err != nil {
			return empty, err
		}
		participants, err := collaborationMentionParticipants(ctx, r.q, r.access)
		if err != nil {
			return empty, err
		}
		recipients := []CollaborationCommentRecipient{}
		for _, person := range participants {
			if mentions.All || mentions.People[person.UserID] {
				recipients = append(recipients, CollaborationCommentRecipient{Recipient: person, IsMention: true})
			}
		}
		if err = writeCollaborationCommentNotifications(ctx, r.q, r.access.Issue.ID, comment.ID, seq, "mention_added", "comment", recipients); err != nil {
			return empty, err
		}
		return out, nil
	})
}

func (r *collaborationRuntimeState) createDerivedRun(ctx context.Context, resource collaboration.GroupResource, parent pgtype.UUID, kind, key string) (db.CollaborationRun, error) {
	var empty db.CollaborationRun
	id, err := util.ParseUUID(resource.GrantID)
	if err != nil {
		return empty, invalidCollaborationLedger("invalid derived resource grant")
	}
	grant, err := r.q.GetCollaborationGrant(ctx, id)
	if err != nil {
		return empty, err
	}
	if grant.RevokedAt.Valid || grant.Epoch != resource.GrantEpoch || grant.TargetID != r.group.TargetID || !collaborationResourceMatchesGrant(resource.TargetResource, grant) {
		return empty, collaborationError(http.StatusConflict, "stale_authority")
	}
	runtime, err := (RuntimeLookup{Queries: r.q, Source: "task"}).Get(ctx, grant.RuntimeID)
	if err != nil {
		return empty, err
	}
	run, err := r.q.CreateCollaborationDerivedRun(ctx, db.CreateCollaborationDerivedRunParams{SessionID: r.session.ID, TargetID: r.group.TargetID, GroupID: r.group.ID, CommandID: r.group.CommandID, ParentRunID: parent, StepKey: key, StepKind: kind,
		AgentID: grant.AgentID, TargetEpoch: r.group.TargetEpoch, ConfigVersion: r.group.ConfigVersion, GrantID: grant.ID, GrantEpoch: grant.Epoch, ResourceEpoch: runtime.CollaborationConfigEpoch, ExecutionWorkspaceID: grant.ExecutionWorkspaceID,
		RuntimeID: grant.RuntimeID, InstallationID: grant.InstallationID, BindingID: grant.BindingID, BindingEpoch: grant.BindingEpoch, ChannelID: r.group.ChannelID, BatchID: r.group.BatchID, DeploymentID: r.run.DeploymentID, ResourceOwnerUserID: grant.OwnerUserID})
	if err != nil {
		return empty, err
	}
	if err = recordCollaborationExecutionChange(ctx, r.q, r.access.Issue.ID, r.group.ID, run.ID, "runtime_prepared"); err != nil {
		return empty, err
	}
	return run, nil
}

func (r *collaborationRuntimeState) queueContinuation(ctx context.Context, parent pgtype.UUID) (pgtype.UUID, error) {
	queued, err := r.q.GetCollaborationQueuedContinuation(ctx, r.group.ID)
	if err == nil {
		return queued.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, err
	}
	roster, err := decodeCollaborationRoster[collaboration.GroupResource](r.group.Roster)
	if err != nil {
		return pgtype.UUID{}, err
	}
	for _, resource := range roster {
		if resource.Role != "leader" {
			continue
		}
		run, err := r.createDerivedRun(ctx, resource, parent, "continuation", "continuation:"+uuid.NewString())
		return run.ID, err
	}
	return pgtype.UUID{}, invalidCollaborationLedger("squad has no frozen leader")
}

// Existing squad activity is business evidence, independent of supervisor proof.
// action/no_action finish this turn successfully; failed records failure. The
// final leader turn decides the group, while member failure only wakes it.
func (s *CollaborationService) RuntimeSquadActivity(ctx context.Context, runID pgtype.UUID, token, eventID, outcome, reason string) error {
	if len(eventID) == 0 || len(eventID) > 128 || len(reason) > 64<<10 || (outcome != "action" && outcome != "no_action" && outcome != "failed") {
		return collaborationError(http.StatusBadRequest, "invalid_squad_activity")
	}
	_, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (bool, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return false, err
		}
		if r.target.Kind != "squad" {
			return false, collaborationError(http.StatusConflict, "squad_activity_unavailable")
		}
		payload, err := json.Marshal(map[string]string{"outcome": outcome, "reason": util.SanitizeTextForPostgres(reason)})
		if err != nil {
			return false, err
		}
		prior, err := r.q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: runID, ProducerEventID: eventID})
		if err == nil {
			var saved map[string]string
			if prior.Kind != "squad_activity" || json.Unmarshal(prior.Payload, &saved) != nil || saved["outcome"] != outcome || saved["reason"] != util.SanitizeTextForPostgres(reason) {
				return false, collaborationError(http.StatusConflict, "squad_activity_conflict")
			}
			return true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}
		if r.run.BusinessOutcome.Valid {
			return false, collaborationError(http.StatusConflict, "outcome_already_recorded")
		}
		if _, err = r.q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: runID, EventID: eventID, Kind: "squad_activity", Payload: payload}); err != nil {
			return false, err
		}
		business := protocol.CollaborationBusinessOutcomeSuccess
		if outcome == "failed" {
			business = protocol.CollaborationBusinessOutcomeFailure
		}
		if _, err = r.q.RecordCollaborationRuntimeOutcome(ctx, db.RecordCollaborationRuntimeOutcomeParams{RunID: runID, Outcome: business}); err != nil {
			return false, err
		}
		return true, recordCollaborationExecutionChange(ctx, r.q, r.access.Issue.ID, r.group.ID, runID, "runtime_outcome_recorded")
	})
	return err
}

// Proof records wake intent without needing current accounts or resources.
// Polling performs current admission and creates at most one queued leader run.
func (s *CollaborationService) drainSquadWakeup(ctx context.Context, parent pgtype.UUID) error {
	_, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (bool, error) {
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, parent, true, false)
		if err != nil {
			return false, err
		}
		if !r.group.WakePending {
			return true, nil
		}
		if err = r.currentGroupAdmission(ctx); err != nil {
			return false, err
		}
		if _, err = r.queueContinuation(ctx, parent); err != nil {
			return false, err
		}
		return true, r.q.ClearCollaborationSquadWakeup(ctx, r.group.ID)
	})
	return err
}
