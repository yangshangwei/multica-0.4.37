package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const CollaborationBusinessTokenPrefix = "mcb_"
const CollaborationTerminationTokenPrefix = "mct_"

// Runtime support is independent of availability, capacity and human consent.
// M09 only enables the verified managed/Linux single-agent launch path.
func collaborationRuntimeTargetSupported(ctx context.Context, q *db.Queries, target db.CollaborationTarget) (string, error) {
	if target.Kind != string(collaboration.TargetAgent) {
		return "squad_runtime_unavailable", nil
	}
	grants, err := q.ListCollaborationTargetGrants(ctx, target.ID)
	if err != nil {
		return "", err
	}
	for _, grant := range grants {
		if grant.RevokedAt.Valid || grant.ConfigVersion.Int64 != target.ConfigVersion || grant.TargetEpoch.Int64 != target.TargetEpoch || grant.AgentID != target.ResourceID || grant.Role.String != "agent" {
			continue
		}
		if grant.IdentityKind != "managed" {
			return "runtime_upgrade_required", nil
		}
		installation, err := q.GetManagedInstallation(ctx, grant.InstallationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "runtime_upgrade_required", nil
		}
		if err != nil {
			return "", err
		}
		if installation.Lifecycle != "active" || installation.Os.String != "linux" || util.UUIDToString(installation.DeploymentID) != auth.ManagedDeploymentID() {
			return "runtime_upgrade_required", nil
		}
		return "", nil
	}
	return "grant_required", nil
}

func (s *CollaborationService) collaborationRuntimeLookup(q *db.Queries) RuntimeLookup {
	if s.Issues != nil {
		return s.Issues.runtimeLookup(q)
	}
	return RuntimeLookup{Queries: q, Source: "task"}
}

func (s *CollaborationService) authorizeCollaborationDaemon(ctx context.Context, q *db.Queries, runtimeID pgtype.UUID) (db.AgentRuntime, db.InstallationDaemonBinding, error) {
	var runtime db.AgentRuntime
	var binding db.InstallationDaemonBinding
	source, ok := auth.PasswordSessionFromContext(ctx)
	if !auth.PasswordMode() || !ok || source.Kind != "daemon_token" || source.BindingID == "" || source.BindingEpoch < 1 || !runtimeID.Valid {
		return runtime, binding, collaborationError(http.StatusForbidden, "managed_runtime_required")
	}
	var err error
	runtime, err = s.collaborationRuntimeLookup(q).Get(ctx, runtimeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return runtime, binding, collaborationError(http.StatusNotFound, "runtime_not_found")
	}
	if err != nil {
		return runtime, binding, err
	}
	binding, err = ValidateManagedRuntime(ctx, s.collaborationRuntimeLookup(q), runtime)
	if err != nil || !binding.ID.Valid {
		if err == nil || errors.Is(err, ErrManagedRuntimeSource) || errors.Is(err, auth.ErrPasswordSession) {
			err = collaborationError(http.StatusForbidden, "managed_runtime_required")
		}
		return runtime, binding, err
	}
	return runtime, binding, nil
}

// All current-authority writers lock users before the existing managed
// namespace fence, then the source aggregate. Proof writers deliberately skip
// users, grant and runtime authorization so resource cleanup cannot strand them.
func (s *CollaborationService) lockCollaborationRuntimeSource(ctx context.Context, tx pgx.Tx, sessionHint db.CollaborationSession, owner, runtimeID pgtype.UUID, current, daemon bool) (*collaborationQueueTx, error) {
	q := s.Queries.WithTx(tx)
	t := &collaborationQueueTx{q: q}
	issueHint, err := q.GetIssue(ctx, sessionHint.IssueID)
	if err != nil {
		return nil, err
	}
	idsParam := db.GetCollaborationWorkflowUserIDsParams{IssueID: sessionHint.IssueID, ActorID: owner}
	var ids []pgtype.UUID
	if current {
		ids, err = q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
		if err != nil {
			return nil, err
		}
		if _, err = q.LockCollaborationUsers(ctx, ids); err != nil {
			return nil, err
		}
		if daemon {
			runtime, _, err := s.authorizeCollaborationDaemon(ctx, q, runtimeID)
			if err != nil {
				return nil, err
			}
			if runtime.OwnerID != owner {
				return nil, collaborationError(http.StatusForbidden, "runtime_owner_changed")
			}
			if _, err = LockManagedRuntime(ctx, s.collaborationRuntimeLookup(q), runtime); err != nil {
				return nil, err
			}
		}
		t.gate, err = q.LockCollaborationGateShared(ctx)
		if err != nil {
			return nil, err
		}
		if err = q.LockIssueStatusCatalogShared(ctx, issueHint.WorkspaceID); err != nil {
			return nil, err
		}
	}
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, issueHint.WorkspaceID); err != nil {
		return nil, err
	}
	issue, err := q.LockCollaborationIssue(ctx, sessionHint.IssueID)
	if err != nil {
		return nil, err
	}
	if issue.WorkspaceID != issueHint.WorkspaceID {
		return nil, collaborationError(http.StatusConflict, "stale_source")
	}
	state, err := q.LockIssueCollaboration(ctx, issue.ID)
	if err != nil {
		return nil, err
	}
	t.session, err = q.LockCollaborationSession(ctx, issue.ID)
	if err != nil {
		return nil, err
	}
	if t.session.ID != sessionHint.ID {
		return nil, invalidCollaborationLedger("runtime session changed")
	}
	t.access = CollaborationIssueAccess{Issue: issue, SessionID: t.session.ID, LifecycleEpoch: state.LifecycleEpoch, Revision: state.Revision, QueueState: t.session.QueueState, ControlVersion: t.session.ControlVersion}
	if current {
		currentIDs, err := q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
		if err != nil {
			return nil, err
		}
		if !sameCollaborationUserIDs(ids, currentIDs) {
			return nil, &pgconn.PgError{Code: "40001", Message: "collaboration runtime authority set changed"}
		}
		access, err := q.GetCollaborationAccess(ctx, db.GetCollaborationAccessParams{ViewerID: owner, IssueID: issue.ID})
		if err != nil {
			return nil, err
		}
		t.access.StatusCategory = access.StatusCategory
	}
	t.channels, err = q.LockCollaborationChannels(ctx, t.session.ID)
	if err != nil {
		return nil, err
	}
	t.targets, err = q.LockCollaborationTargets(ctx, t.session.ID)
	if err != nil {
		return nil, err
	}
	if current {
		for _, target := range t.targets {
			if _, err = q.LockCollaborationTargetGrants(ctx, target.ID); err != nil {
				return nil, err
			}
		}
	}
	if _, err = q.LockCollaborationSessionCommands(ctx, t.session.ID); err != nil {
		return nil, err
	}
	if _, err = q.LockCollaborationBatches(ctx, t.session.ID); err != nil {
		return nil, err
	}
	return t, nil
}

type collaborationRuntimeState struct {
	*collaborationQueueTx
	run     db.CollaborationRun
	group   db.CollaborationExecutionGroup
	batch   db.CollaborationBatch
	channel db.CollaborationChannel
	target  db.CollaborationTarget
	grant   db.CollaborationGrant
	inputs  []db.CollaborationCommand
}

func (s *CollaborationService) lockCollaborationRuntimeRun(ctx context.Context, tx pgx.Tx, runID pgtype.UUID, current, daemon bool) (*collaborationRuntimeState, error) {
	q := s.Queries.WithTx(tx)
	hint, err := q.GetCollaborationRuntimeRun(ctx, runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, collaborationError(http.StatusNotFound, "run_not_found")
	}
	if err != nil {
		return nil, err
	}
	session, err := q.GetCollaborationSessionByID(ctx, hint.SessionID)
	if err != nil {
		return nil, err
	}
	t, err := s.lockCollaborationRuntimeSource(ctx, tx, session, hint.ResourceOwnerUserID, hint.RuntimeID, current, daemon)
	if err != nil {
		return nil, err
	}
	r := &collaborationRuntimeState{collaborationQueueTx: t}
	r.group, err = q.LockCollaborationRuntimeGroup(ctx, hint.GroupID)
	if err != nil {
		return nil, err
	}
	r.run, err = q.LockCollaborationRuntimeRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	var found bool
	r.target, found = t.target(r.run.TargetID)
	if !found {
		if current {
			return nil, invalidCollaborationLedger("runtime target missing")
		}
		// Proof authority is the retained group/run/termination identity. It
		// cannot disappear merely because a current target was removed.
		r.target = db.CollaborationTarget{ID: r.group.TargetID, ChannelID: r.group.ChannelID, GroupGeneration: r.group.Generation, ActiveGroupID: r.group.ID}
		if r.run.StepKind.String == "agent" {
			r.target.Kind = "agent"
		} else {
			r.target.Kind = "squad"
		}
	}
	r.batch, err = q.GetCollaborationBatch(ctx, r.run.BatchID)
	if err != nil {
		return nil, err
	}
	r.channel, err = q.GetCollaborationChannel(ctx, r.run.ChannelID)
	if err != nil {
		return nil, err
	}
	if r.run.SessionID != t.session.ID || r.group.SessionID != t.session.ID || r.group.ID != r.run.GroupID || r.group.CommandID != r.run.CommandID ||
		r.group.TargetID != r.target.ID || r.group.ChannelID != r.channel.ID || r.group.BatchID != r.batch.ID || r.batch.ChannelID != r.channel.ID ||
		r.batch.SessionID != t.session.ID || r.channel.SessionID != t.session.ID || r.target.ChannelID != r.channel.ID {
		return nil, invalidCollaborationLedger("runtime batch identity mismatch")
	}
	r.inputs, err = q.ListCollaborationBatchInputs(ctx, r.batch.ID)
	if err != nil {
		return nil, err
	}
	if current {
		r.grant, err = q.GetCollaborationGrant(ctx, r.run.GrantID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, collaborationError(http.StatusForbidden, "grant_required")
		}
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}

func collaborationCurrentBatchAuthors(ctx context.Context, q *db.Queries, issueID pgtype.UUID, lifecycle int64, inputs []db.CollaborationCommand, state string) (string, error) {
	count := 0
	for _, input := range inputs {
		if input.State != state {
			continue
		}
		count++
		if input.LifecycleEpoch != lifecycle {
			return "stale_authority", nil
		}
		participant, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: issueID, UserID: input.AuthorUserID})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (participant.RevokedAt.Valid || participant.Epoch != input.ParticipantEpoch) {
			return "stale_authority", nil
		}
		if err != nil {
			return "", err
		}
		fence, err := q.GetCollaborationAccountFence(ctx, input.AuthorUserID)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && fence.Epoch != input.AuthorAuthEpoch {
			return "stale_authority", nil
		}
		if err != nil {
			return "", err
		}
		eligible, err := collaborationRecipientEligible(ctx, q, input.AuthorUserID, pgtype.UUID{})
		if err != nil {
			return "", err
		}
		if !eligible {
			return "stale_authority", nil
		}
	}
	if count == 0 {
		return "batch_empty", nil
	}
	return "", nil
}

func (r *collaborationRuntimeState) currentAdmission(ctx context.Context) error {
	if !r.gate.Enabled {
		return collaborationError(http.StatusConflict, "collaboration_execution_unavailable")
	}
	if !collaboration.NonterminalCategory(r.access.StatusCategory) || (r.access.Issue.AdmissionStatus != "not_required" && r.access.Issue.AdmissionStatus != "accepted") {
		return collaborationError(http.StatusConflict, "issue_closed")
	}
	if r.target.HoldReason.Valid {
		return collaborationError(http.StatusConflict, "target_held")
	}
	if r.session.QueueState != "active" || r.channel.QueueState != "active" || r.target.QueueState != "active" || r.group.ExpansionSealedAt.Valid || r.group.ForcedDisposition.Valid ||
		r.group.SettledAt.Valid || r.run.SettledAt.Valid || r.run.ForcedDisposition.Valid || r.batch.State != "in_flight" || r.batch.GroupID != r.group.ID || r.channel.ActiveGroupID != r.group.ID {
		return collaborationError(http.StatusConflict, "execution_stopped")
	}
	if r.target.TargetEpoch != r.run.TargetEpoch.Int64 || r.target.ConfigVersion != r.run.ConfigVersion.Int64 || r.grant.Epoch != r.run.GrantEpoch ||
		r.grant.TargetID != r.target.ID || r.grant.RuntimeID != r.run.RuntimeID || r.grant.AgentID != r.run.AgentID || r.grant.BindingID != r.run.BindingID ||
		r.grant.BindingEpoch != r.run.BindingEpoch || r.grant.InstallationID != r.run.InstallationID || r.grant.OwnerUserID != r.run.ResourceOwnerUserID ||
		util.UUIDToString(r.run.DeploymentID) != auth.ManagedDeploymentID() {
		return collaborationError(http.StatusConflict, "stale_authority")
	}
	seen := map[pgtype.UUID]bool{}
	for _, input := range r.inputs {
		if input.State != "in_flight" {
			continue
		}
		target, ok := r.targetByID(input.TargetID)
		if !ok || target.ChannelID != r.channel.ID {
			return invalidCollaborationLedger("batch input target mismatch")
		}
		if target.HoldReason.Valid {
			return collaborationError(http.StatusConflict, "target_held")
		}
		if !seen[target.ID] {
			if code, err := r.authorizeTargetInput(ctx, target); err != nil {
				return err
			} else if code != "" {
				return collaborationError(http.StatusConflict, code)
			}
			seen[target.ID] = true
		}
	}
	if code, err := collaborationCurrentBatchAuthors(ctx, r.q, r.access.Issue.ID, r.access.LifecycleEpoch, r.inputs, "in_flight"); err != nil {
		return err
	} else if code != "" {
		return collaborationError(http.StatusConflict, code)
	}
	if code, err := collaborationRuntimeTargetSupported(ctx, r.q, r.target); err != nil {
		return err
	} else if code != "" {
		return collaborationError(http.StatusConflict, code)
	}
	installation, err := r.q.GetManagedInstallation(ctx, r.run.InstallationID)
	if err != nil {
		return err
	}
	if installation.Admission != "accepting" {
		return collaborationError(http.StatusConflict, "installation_stopped")
	}
	return nil
}

func (r *collaborationRuntimeState) targetByID(id pgtype.UUID) (db.CollaborationTarget, bool) {
	return r.collaborationQueueTx.target(id)
}

func collaborationRunIdentity(run db.CollaborationRun, issueID pgtype.UUID) protocol.CollaborationRunIdentity {
	return protocol.CollaborationRunIdentity{DeploymentID: util.UUIDToString(run.DeploymentID), InstallationID: util.UUIDToString(run.InstallationID), IssueID: util.UUIDToString(issueID),
		SessionID: util.UUIDToString(run.SessionID), TargetID: util.UUIDToString(run.TargetID), ChannelID: util.UUIDToString(run.ChannelID), BatchID: util.UUIDToString(run.BatchID),
		GroupID: util.UUIDToString(run.GroupID), RunID: util.UUIDToString(run.ID), Fence: strconv.FormatInt(run.Fence, 10), ClaimGeneration: strconv.FormatInt(run.ClaimGeneration, 10), LaunchNonce: util.UUIDToString(run.LaunchNonce)}
}

func collaborationRuntimeMutation[T any](ctx context.Context, s *CollaborationService, fn func(pgx.Tx) (T, error)) (T, error) {
	var empty T
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.TxStarter.Begin(ctx)
		if err != nil {
			return empty, err
		}
		result, err := fn(tx)
		if err == nil {
			if err = tx.Commit(ctx); err != nil {
				rollbackCollaborationQueue(ctx, tx)
				return empty, fmt.Errorf("commit collaboration runtime operation: %w", err)
			}
			return result, nil
		}
		rollbackCollaborationQueue(ctx, tx)
		var conflict *pgconn.PgError
		if !errors.As(err, &conflict) || (conflict.Code != "40001" && conflict.Code != "40P01" && conflict.Code != "55P03") {
			return empty, err
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(20+attempt*20) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return empty, ctx.Err()
			}
		}
	}
	return empty, collaborationError(http.StatusServiceUnavailable, "collaboration_busy")
}

func (s *CollaborationService) ListRuntimeRuns(ctx context.Context, runtimeID pgtype.UUID) (protocol.CollaborationRunListResponse, error) {
	result := protocol.CollaborationRunListResponse{RunIDs: []string{}}
	_, binding, err := s.authorizeCollaborationDaemon(ctx, s.Queries, runtimeID)
	if err != nil {
		return result, err
	}
	waiting, err := s.Queries.ListCollaborationRuntimeWaitingTargets(ctx, db.ListCollaborationRuntimeWaitingTargetsParams{RuntimeID: runtimeID, BindingID: binding.ID})
	if err != nil {
		return result, err
	}
	for _, candidate := range waiting {
		promotion, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (collaborationRuntimePromotion, error) {
			return s.promoteCollaborationRuntimeBatch(ctx, tx, candidate.SessionID, candidate.ID, runtimeID)
		})
		if err == nil && promotion.RefusalCode != "" {
			// The hold and audit committed. This remains a refused promotion,
			// while actual transaction errors never enter this path.
			err = collaborationError(http.StatusConflict, promotion.RefusalCode)
		}
		if err != nil {
			var denied *CollaborationError
			if errors.As(err, &denied) && denied.Status < 500 {
				continue
			}
			return result, err
		}
	}
	ids, err := s.Queries.ListCollaborationRuntimeCandidates(ctx, db.ListCollaborationRuntimeCandidatesParams{RuntimeID: runtimeID, BindingID: binding.ID})
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		result.RunIDs = append(result.RunIDs, util.UUIDToString(id))
	}
	return result, nil
}

// A promotion may commit a durable authority refusal without creating a run.
// Other failures remain errors so the generic transaction helper rolls back.
type collaborationRuntimePromotion struct {
	RunID       pgtype.UUID
	RefusalCode string
}

func (s *CollaborationService) promoteCollaborationRuntimeBatch(ctx context.Context, tx pgx.Tx, sessionID, targetID, runtimeID pgtype.UUID) (collaborationRuntimePromotion, error) {
	q := s.Queries.WithTx(tx)
	runtime, binding, err := s.authorizeCollaborationDaemon(ctx, q, runtimeID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	session, err := q.GetCollaborationSessionByID(ctx, sessionID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	t, err := s.lockCollaborationRuntimeSource(ctx, tx, session, runtime.OwnerID, runtimeID, true, true)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	deny := func(code string) (collaborationRuntimePromotion, error) {
		return collaborationRuntimePromotion{}, collaborationError(http.StatusConflict, code)
	}
	if !t.gate.Enabled {
		return deny("collaboration_execution_unavailable")
	}
	if t.session.QueueState != "active" || !collaboration.NonterminalCategory(t.access.StatusCategory) || (t.access.Issue.AdmissionStatus != "not_required" && t.access.Issue.AdmissionStatus != "accepted") {
		return deny("issue_paused")
	}
	target, ok := t.target(targetID)
	if !ok || target.QueueState != "active" {
		return deny("target_paused")
	}
	if target.HoldReason.Valid {
		return deny("target_held")
	}
	if code, err := t.authorizeTargetInput(ctx, target); err != nil {
		return collaborationRuntimePromotion{}, err
	} else if code != "" {
		return t.holdInvalidPromotion(ctx, target, code)
	}
	if code, err := collaborationRuntimeTargetSupported(ctx, q, target); err != nil {
		return collaborationRuntimePromotion{}, err
	} else if code != "" {
		return deny(code)
	}
	channel, err := q.GetCollaborationChannel(ctx, target.ChannelID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if channel.QueueState != "active" || channel.ActiveGroupID.Valid || channel.RootAgentID != target.ResourceID {
		return deny("channel_busy")
	}
	groups, err := q.LockCollaborationUnsettledGroups(ctx, t.session.ID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	for _, group := range groups {
		if group.ChannelID == channel.ID {
			return deny("stop_unconfirmed")
		}
	}
	runs, err := q.LockCollaborationUnsettledRuns(ctx, t.session.ID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	for _, run := range runs {
		if run.ChannelID == channel.ID {
			return deny("stop_unconfirmed")
		}
	}
	batches, err := q.LockCollaborationWaitingBatches(ctx, channel.ID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if len(batches) == 0 || batches[0].EffectiveTargetID != target.ID {
		return deny("batch_not_ready")
	}
	batch := batches[0]
	inputs, err := q.ListCollaborationBatchInputs(ctx, batch.ID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if code, err := collaborationCurrentBatchAuthors(ctx, q, t.access.Issue.ID, t.access.LifecycleEpoch, inputs, "waiting"); err != nil {
		return collaborationRuntimePromotion{}, err
	} else if code != "" {
		return deny(code)
	}
	wire, err := collaborationRunInputs(inputs)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if len(wire) > maxCollaborationBatchInputs || len(encoded) > maxCollaborationBatchBytes {
		return deny("batch_capacity_exceeded")
	}
	var root db.CollaborationCommand
	for _, input := range inputs {
		if input.State != "waiting" {
			continue
		}
		inputTarget, exists := t.target(input.TargetID)
		if !exists || inputTarget.ChannelID != channel.ID {
			return collaborationRuntimePromotion{}, invalidCollaborationLedger("waiting input channel mismatch")
		}
		if inputTarget.HoldReason.Valid {
			return deny("target_held")
		}
		if code, err := t.authorizeTargetInput(ctx, inputTarget); err != nil {
			return collaborationRuntimePromotion{}, err
		} else if code != "" {
			return t.holdInvalidPromotion(ctx, inputTarget, code)
		}
		if input.TargetID == target.ID && !root.ID.Valid {
			root = input
		}
	}
	if !root.ID.Valid {
		return deny("batch_empty")
	}
	grants, err := q.ListCollaborationTargetGrants(ctx, target.ID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	var grant db.CollaborationGrant
	for _, g := range grants {
		if !g.RevokedAt.Valid && g.ConfigVersion.Int64 == target.ConfigVersion && g.TargetEpoch.Int64 == target.TargetEpoch && g.AgentID == target.ResourceID && g.RuntimeID == runtimeID && g.BindingID == binding.ID {
			grant = g
			break
		}
	}
	if !grant.ID.Valid {
		return deny("grant_required")
	}
	installation, err := q.GetManagedInstallation(ctx, grant.InstallationID)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if installation.Admission != "accepting" {
		return deny("installation_stopped")
	}
	if runtime.Status != "online" {
		return deny("runtime_offline")
	}
	occupied, err := q.CollaborationRuntimeNamespaceOccupied(ctx, db.CollaborationRuntimeNamespaceOccupiedParams{WorkspaceID: runtime.WorkspaceID, DaemonID: runtime.DaemonID.String})
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if occupied {
		return deny("runtime_busy")
	}
	resource := collaboration.GroupResource{TargetResource: collaboration.TargetResource{Role: "agent", AgentID: util.UUIDToString(grant.AgentID), RuntimeID: util.UUIDToString(grant.RuntimeID), ExecutionWorkspaceID: util.UUIDToString(grant.ExecutionWorkspaceID), OwnerUserID: util.UUIDToString(grant.OwnerUserID)}, GrantID: util.UUIDToString(grant.ID), GrantEpoch: grant.Epoch}
	roster, err := json.Marshal([]collaboration.GroupResource{resource})
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	group, err := q.CreateCollaborationRuntimeGroup(ctx, db.CreateCollaborationRuntimeGroupParams{SessionID: t.session.ID, TargetID: target.ID, CommandID: root.ID, Generation: channel.GroupGeneration + 1, TargetEpoch: target.TargetEpoch, ConfigVersion: target.ConfigVersion, Roster: roster, ChannelID: channel.ID, BatchID: batch.ID})
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	deployment, err := util.ParseUUID(auth.ManagedDeploymentID())
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	run, err := q.CreateCollaborationRuntimeRun(ctx, db.CreateCollaborationRuntimeRunParams{SessionID: t.session.ID, TargetID: target.ID, GroupID: group.ID, CommandID: root.ID,
		AgentID: grant.AgentID, TargetEpoch: target.TargetEpoch, ConfigVersion: target.ConfigVersion, GrantID: grant.ID, GrantEpoch: grant.Epoch, ResourceEpoch: runtime.CollaborationConfigEpoch,
		ExecutionWorkspaceID: runtime.WorkspaceID, RuntimeID: runtime.ID, InstallationID: grant.InstallationID, BindingID: grant.BindingID, BindingEpoch: grant.BindingEpoch, ChannelID: channel.ID, BatchID: batch.ID, DeploymentID: deployment, ResourceOwnerUserID: runtime.OwnerID})
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if err = q.CreateCollaborationRuntimeReservation(ctx, db.CreateCollaborationRuntimeReservationParams{RunID: run.ID, ExecutionWorkspaceID: runtime.WorkspaceID, AgentID: grant.AgentID, RuntimeID: runtime.ID, DaemonID: runtime.DaemonID.String, InstallationID: grant.InstallationID, BindingID: grant.BindingID}); err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if _, err = q.StartCollaborationBatch(ctx, db.StartCollaborationBatchParams{BatchID: batch.ID, CommandID: root.ID, GroupID: group.ID}); err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if err = q.StartCollaborationBatchInputs(ctx, batch.ID); err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if _, err = q.AssignCollaborationChannelGroup(ctx, db.AssignCollaborationChannelGroupParams{ChannelID: channel.ID, GroupID: group.ID}); err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if _, err = q.MirrorCollaborationChannelControl(ctx, channel.ID); err != nil {
		return collaborationRuntimePromotion{}, err
	}
	return collaborationRuntimePromotion{RunID: run.ID}, nil
}

// Only authoritative resource/configuration failures may commit a new hold.
// Offline, capacity, gate and database failures never become permanent holds.
func (t *collaborationQueueTx) holdInvalidPromotion(ctx context.Context, target db.CollaborationTarget, code string) (collaborationRuntimePromotion, error) {
	var reason collaboration.TargetHoldReason
	switch code {
	case "grant_required":
		reason = collaboration.HoldGrantRequired
	case "target_configuration_changed":
		reason = collaboration.HoldConfigInvalid
	default:
		return collaborationRuntimePromotion{}, collaborationError(http.StatusConflict, code)
	}
	previous := target.HoldVersion
	target, err := t.setTargetHold(ctx, target, reason, false)
	if err != nil {
		return collaborationRuntimePromotion{}, err
	}
	if target.HoldVersion != previous {
		r := t.targetReceipt(code, http.StatusConflict, target)
		r.TargetHolds = append(r.TargetHolds, collaborationTargetHoldResult(target, string(reason)))
		if err = t.event(ctx, "target_hold_changed", r); err != nil {
			return collaborationRuntimePromotion{}, err
		}
	}
	return collaborationRuntimePromotion{RefusalCode: code}, nil
}
