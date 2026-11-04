package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const collaborationKnowledgeClaimKey = "knowledge:claim:v1"
const collaborationKnowledgePrepareKey = "knowledge:prepare:v1"

type collaborationKnowledgePreparation struct {
	Request        protocol.CollaborationRunKnowledgePrepare `json:"request"`
	Manifest       KnowledgeManifest                         `json:"manifest"`
	ManifestSHA256 string                                    `json:"manifest_sha256"`
	Targets        []KnowledgeTargetRevision                 `json:"targets"`
}

func loadCollaborationKnowledge[T any](ctx context.Context, q *db.Queries, run pgtype.UUID, key, kind string) (T, error) {
	var out T
	e, err := q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: key})
	if err != nil {
		return out, err
	}
	if e.Kind != kind || decodeCollaborationPreparationJSON(e.Payload, &out) != nil {
		return out, knowledgeFailure(503, "knowledge_unavailable")
	}
	return out, nil
}

func storeCollaborationKnowledge(ctx context.Context, q *db.Queries, run, issue pgtype.UUID, key, kind string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 64<<10 {
		return knowledgeFailure(503, "knowledge_unavailable")
	}
	_, err = q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: issue, RunID: run, EventID: key, Kind: kind, Payload: raw})
	return err
}

func (s *CollaborationService) claimRuntimeKnowledge(ctx context.Context, r *collaborationRuntimeState, p protocol.CollaborationRunClaimRequest, claim protocol.CollaborationRunClaimResponse, freezing bool) (*protocol.CollaborationRunKnowledgeClaim, error) {
	old, err := loadCollaborationKnowledge[protocol.CollaborationRunKnowledgeClaim](ctx, r.q, r.run.ID, collaborationKnowledgeClaimKey, "knowledge_claim")
	if err == nil {
		if p.KnowledgeVersion != 1 || !s.KnowledgeContextEnabled || old.Identity != claim.Identity || claim.Preparation == nil || claim.Preparation.Host == nil || old.ConfigurationDigest != claim.Preparation.Host.ConfigurationDigest {
			return nil, knowledgeFailure(409, "knowledge_context_changed")
		}
		return &old, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if p.KnowledgeVersion != 0 && p.KnowledgeVersion != 1 {
		return nil, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if !freezing || !s.KnowledgeContextEnabled || p.KnowledgeVersion == 0 || claim.ProtocolVersion != protocol.CollaborationHostRuntimeProtocolVersion || claim.Preparation == nil || claim.Preparation.Host == nil || !agent.KnowledgeLifecycleSupported(claim.Preparation.Provider.Provider, claim.Preparation.Host.ProviderMode, true) || claim.Preparation.Host.ProviderMode != "local" {
		return nil, nil
	}
	runtime, _, err := s.authorizeCollaborationDaemon(ctx, r.q, r.run.RuntimeID)
	if err != nil {
		return nil, err
	}
	if runtime.ProfileID.Valid || runtime.Provider != claim.Preparation.Provider.Provider || !agent.KnowledgeLifecycleSupported(runtime.Provider, runtime.RuntimeMode, true) || runtime.RuntimeMode != "local" {
		return nil, nil
	}
	d := protocol.CollaborationRunKnowledgeClaim{ProtocolVersion: 1, Identity: claim.Identity, ConfigurationDigest: claim.Preparation.Host.ConfigurationDigest, Namespace: claim.Identity.RunID}
	if err = storeCollaborationKnowledge(ctx, r.q, r.run.ID, r.access.Issue.ID, collaborationKnowledgeClaimKey, "knowledge_claim", d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *CollaborationService) collaborationKnowledgeFacts(ctx context.Context, q *db.Queries, run db.CollaborationRun) (KnowledgeConsumerFacts, error) {
	f := KnowledgeConsumerFacts{Supported: true, Surface: "collaboration", Audience: "collaboration", AgentID: run.AgentID, RuntimeID: run.RuntimeID, RuntimeOwnerUserID: run.ResourceOwnerUserID}
	session, err := q.GetCollaborationSessionByID(ctx, run.SessionID)
	if err != nil {
		return f, err
	}
	issue, err := q.GetIssue(ctx, session.IssueID)
	if err != nil {
		return f, err
	}
	f.CollaborationIssueID, f.WorkspaceID, f.ProjectID = issue.ID, issue.WorkspaceID, issue.ProjectID
	inputs, err := decodeCollaborationRoster[protocol.CollaborationRunInput](run.PromptSnapshot)
	if err != nil || len(inputs) == 0 || len(inputs) > 1024 {
		return f, knowledgeFailure(409, "knowledge_context_changed")
	}
	for _, input := range inputs {
		id, err := util.ParseUUID(input.AuthorUserID)
		if err != nil {
			return f, knowledgeFailure(409, "knowledge_context_changed")
		}
		f.InputAuthorUserIDs = append(f.InputAuthorUserIDs, id)
	}
	f.OriginatorUserID = f.InputAuthorUserIDs[0]
	return f, nil
}

// This graph deliberately does not call lockCollaborationRuntimeRun. Source
// writers take issue UPDATE; our SHARE NOWAIT locks freeze their state while
// currentAdmission runs under prelocked commands/grants/group runs. Its resource
// helpers are already exclusively NOWAIT/try-lock readers. A busy result rolls
// the whole graph back and runCollaborationKnowledgeMutation re-runs it; never
// turn the namespace, member or status fence into a blocking wait.
func (s *CollaborationService) lockCollaborationKnowledge(ctx context.Context, tx pgx.Tx, runID pgtype.UUID, tokenHash string, exact *KnowledgeManifest) (*collaborationRuntimeState, KnowledgeConsumerFacts, KnowledgeReadLockHints, error) {
	q := s.Queries.WithTx(tx)
	var f KnowledgeConsumerFacts
	var hints KnowledgeReadLockHints
	fail := func(err error) (*collaborationRuntimeState, KnowledgeConsumerFacts, KnowledgeReadLockHints, error) {
		return nil, f, hints, err
	}
	if !s.KnowledgeContextEnabled {
		return fail(knowledgeFailure(409, "knowledge_invocation_unsupported"))
	}
	run, err := q.GetCollaborationRuntimeRun(ctx, runID)
	if err != nil {
		return fail(err)
	}
	f, err = s.collaborationKnowledgeFacts(ctx, q, run)
	if err != nil {
		return fail(err)
	}
	hints, err = s.CollectKnowledgeReadLockHints(ctx, q, f, exact)
	if err != nil {
		return fail(err)
	}
	users, err := q.GetCollaborationWorkflowUserIDs(ctx, db.GetCollaborationWorkflowUserIDsParams{IssueID: f.CollaborationIssueID, ActorID: f.RuntimeOwnerUserID})
	if err != nil {
		return fail(err)
	}
	audience, err := q.ListKnowledgeAudienceUsers(ctx, f.CollaborationIssueID)
	if err != nil {
		return fail(err)
	}
	if len(audience) > 1024 {
		return fail(knowledgeFailure(409, "knowledge_context_changed"))
	}
	users = append(users, audience...)
	users = append(users, f.InputAuthorUserIDs...)
	slices.SortFunc(users, knowledgeCompareUUID)
	users = slices.Compact(users)
	accounts, err := q.LockKnowledgeUsers(ctx, users)
	if err != nil {
		return fail(err)
	}
	if len(accounts) != len(users) {
		return fail(knowledgeFailure(403, "knowledge_authorization_denied"))
	}
	for _, account := range accounts {
		if account.DisabledAt.Valid {
			return fail(knowledgeFailure(403, "knowledge_authorization_denied"))
		}
	}
	credential, err := q.LockTaskKnowledgeDaemonCredential(ctx, tokenHash)
	if err != nil {
		return fail(err)
	}
	if credential.UserID != f.RuntimeOwnerUserID || !credential.ExpiresAt.Time.After(time.Now()) {
		return fail(knowledgeFailure(403, "knowledge_authorization_denied"))
	}
	workspaces := []pgtype.UUID{f.WorkspaceID, run.ExecutionWorkspaceID}
	for _, source := range hints.Sources {
		workspaces = append(workspaces, source.WorkspaceID)
	}
	slices.SortFunc(workspaces, knowledgeCompareUUID)
	workspaces = slices.Compact(workspaces)
	for _, ws := range workspaces {
		if _, err = q.LockKnowledgeWorkspace(ctx, ws); err != nil {
			return fail(err)
		}
		for _, user := range users {
			locked, e := q.TryLockKnowledgeMemberFence(ctx, db.TryLockKnowledgeMemberFenceParams{WorkspaceID: ws, UserID: user})
			if e != nil {
				return fail(e)
			}
			if !locked {
				return fail(knowledgeFailure(503, "knowledge_busy"))
			}
			if _, e = q.LockKnowledgeMember(ctx, db.LockKnowledgeMemberParams{WorkspaceID: ws, UserID: user}); e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return fail(e)
			}
		}
	}
	if err = s.LockKnowledgeReadTargets(ctx, q, hints); err != nil {
		return fail(err)
	}
	locked, err := q.TryCollaborationKnowledgeStatusFence(ctx, f.WorkspaceID)
	if err != nil {
		return fail(err)
	}
	if !locked {
		return fail(knowledgeFailure(503, "knowledge_busy"))
	}
	issues := []pgtype.UUID{f.CollaborationIssueID}
	for _, source := range hints.Sources {
		issues = append(issues, source.IssueID)
	}
	slices.SortFunc(issues, knowledgeCompareUUID)
	issues = slices.Compact(issues)
	for _, id := range issues {
		if _, err = q.LockKnowledgeSourceIssue(ctx, id); err != nil {
			return fail(err)
		}
	}
	if err = s.LockKnowledgeReadSources(ctx, q, hints); err != nil {
		return fail(err)
	}
	state, err := q.LockKnowledgeSourceCollaboration(ctx, f.CollaborationIssueID)
	if err != nil {
		return fail(err)
	}
	session, err := q.LockKnowledgeSourceSession(ctx, f.CollaborationIssueID)
	if err != nil {
		return fail(err)
	}
	if session.ID != run.SessionID {
		return fail(knowledgeFailure(409, "knowledge_context_changed"))
	}
	currentAudience, err := q.ListKnowledgeAudienceUsers(ctx, f.CollaborationIssueID)
	if err != nil {
		return fail(err)
	}
	if !slices.Equal(audience, currentAudience) {
		return fail(knowledgeFailure(409, "knowledge_context_changed"))
	}
	if _, err = q.LockCollaborationKnowledgeCommands(ctx, session.ID); err != nil {
		return fail(err)
	}
	if _, err = q.LockCollaborationKnowledgeGrants(ctx, session.ID); err != nil {
		return fail(err)
	}
	if _, err = q.LockCollaborationKnowledgeGroupRuns(ctx, run.GroupID); err != nil {
		return fail(err)
	}
	runtime, err := q.LockTaskKnowledgeRuntime(ctx, run.RuntimeID)
	if err != nil {
		return fail(err)
	}
	locked, err = q.TryTaskKnowledgeInstallationFence(ctx, installationNamespace(runtime.WorkspaceID, runtime.DaemonID.String))
	if err != nil {
		return fail(err)
	}
	if !locked {
		return fail(knowledgeFailure(503, "knowledge_busy"))
	}
	if _, err = q.TryLockCollaborationAuthorityBinding(ctx, run.BindingID); err != nil {
		return fail(err)
	}
	if _, err = q.LockTaskKnowledgeInstallation(ctx, run.InstallationID); err != nil {
		return fail(err)
	}
	currentRuntime, binding, err := s.authorizeCollaborationDaemon(ctx, q, run.RuntimeID)
	if err != nil {
		return fail(err)
	}
	if currentRuntime.OwnerID != f.RuntimeOwnerUserID || binding.ID != run.BindingID || binding.BindingEpoch != run.BindingEpoch || credential.WorkspaceID != runtime.WorkspaceID || credential.DaemonID != runtime.DaemonID.String || credential.AuthVersion < 1 {
		return fail(knowledgeFailure(403, "knowledge_authorization_denied"))
	}
	if _, err = auth.CheckPasswordVersion(ctx, q, util.UUIDToString(credential.UserID), credential.AuthVersion); err != nil {
		return fail(err)
	}
	if _, err = q.LockTaskKnowledgeAgent(ctx, run.AgentID); err != nil {
		return fail(err)
	}
	current, err := q.GetCollaborationRuntimeRun(ctx, runID)
	if err != nil {
		return fail(err)
	}
	currentFacts, err := s.collaborationKnowledgeFacts(ctx, q, current)
	if err != nil {
		return fail(err)
	}
	if !reflect.DeepEqual(f, currentFacts) || current.RuntimeID != run.RuntimeID || current.ClaimGeneration != run.ClaimGeneration || current.LaunchNonce != run.LaunchNonce {
		return fail(knowledgeFailure(409, "knowledge_context_changed"))
	}
	issue, err := q.GetIssue(ctx, f.CollaborationIssueID)
	if err != nil {
		return fail(err)
	}
	t := &collaborationQueueTx{q: q, session: session, access: CollaborationIssueAccess{Issue: issue, SessionID: session.ID, LifecycleEpoch: state.LifecycleEpoch, Revision: state.Revision, AuthorityEpoch: state.AuthorityEpoch, EventSequence: session.EventSeq, InformationRequestID: state.ActiveInformationRequestID}}
	t.gate, err = q.LockCollaborationKnowledgeGate(ctx)
	if err != nil {
		return fail(err)
	}
	access, err := q.GetCollaborationAccess(ctx, db.GetCollaborationAccessParams{ViewerID: f.RuntimeOwnerUserID, IssueID: issue.ID})
	if err != nil {
		return fail(err)
	}
	t.access.StatusCategory = access.StatusCategory
	t.channels, err = q.ListCollaborationChannels(ctx, session.ID)
	if err != nil {
		return fail(err)
	}
	t.targets, err = q.ListCollaborationTargets(ctx, session.ID)
	if err != nil {
		return fail(err)
	}
	r := &collaborationRuntimeState{collaborationQueueTx: t, run: current}
	r.group, err = q.GetCollaborationExecutionGroup(ctx, db.GetCollaborationExecutionGroupParams{SessionID: session.ID, ID: run.GroupID})
	if err != nil {
		return fail(err)
	}
	r.batch, err = q.GetCollaborationBatch(ctx, run.BatchID)
	if err != nil {
		return fail(err)
	}
	r.channel, err = q.GetCollaborationChannel(ctx, run.ChannelID)
	if err != nil {
		return fail(err)
	}
	var found bool
	r.target, found = t.target(run.TargetID)
	if !found {
		return fail(knowledgeFailure(409, "knowledge_context_changed"))
	}
	r.grant, err = q.GetCollaborationGrant(ctx, run.GrantID)
	if err != nil {
		return fail(err)
	}
	r.inputs, err = q.ListCollaborationBatchInputs(ctx, run.BatchID)
	if err != nil {
		return fail(err)
	}
	if err = r.currentAdmission(ctx); err != nil {
		return fail(err)
	}
	if err = r.requireReservation(ctx); err != nil {
		return fail(err)
	}
	term, err := q.GetCollaborationTermination(ctx, runID)
	if err != nil {
		return fail(err)
	}
	if term.ConfirmedAt.Valid || term.RevokedAt.Valid || term.StopRequestedAt.Valid || r.run.SealedAttemptSeq.Valid || !t.gate.HostExecutionEnabled {
		return fail(knowledgeFailure(409, "knowledge_context_changed"))
	}
	d, err := loadCollaborationKnowledge[protocol.CollaborationRunKnowledgeClaim](ctx, q, runID, collaborationKnowledgeClaimKey, "knowledge_claim")
	if err != nil {
		return fail(err)
	}
	preparation, err := s.readRuntimePreparation(ctx, r)
	if err != nil {
		return fail(err)
	}
	if d.Identity != collaborationRunIdentity(current, issue.ID) || d.Namespace != util.UUIDToString(runID) || d.ProtocolVersion != 1 || preparation.Host == nil || d.ConfigurationDigest != preparation.Host.ConfigurationDigest || runtime.Provider != preparation.Provider.Provider || !agent.KnowledgeLifecycleSupported(runtime.Provider, runtime.RuntimeMode, true) || runtime.RuntimeMode != "local" || runtime.ProfileID.Valid {
		return fail(knowledgeFailure(409, "knowledge_context_changed"))
	}
	return r, f, hints, nil
}

// runCollaborationKnowledgeMutation re-runs fn in a fresh transaction while it
// reports a busy result, either a fence try-lock miss or NOWAIT exhaustion in
// collaborationRuntimeMutation. A commit error is returned unretried: its
// outcome is unknown, and only the original request identity may reconcile it.
func runCollaborationKnowledgeMutation[T any](ctx context.Context, s *CollaborationService, fn func(pgx.Tx) (T, error)) (T, error) {
	var out T
	var commit *collaborationRuntimeCommitError
	err := retryKnowledgeBusy(ctx, func() error {
		var e error
		out, e = collaborationRuntimeMutation(ctx, s, fn)
		if errors.As(e, &commit) {
			return nil
		}
		return taskKnowledgeError(e)
	})
	if commit != nil {
		var empty T
		return empty, taskKnowledgeError(commit)
	}
	return out, err
}

func (s *CollaborationService) PrepareRuntimeKnowledge(ctx context.Context, runID pgtype.UUID, tokenHash string, p protocol.CollaborationRunKnowledgePrepare) (protocol.CollaborationRunKnowledgeBundle, error) {
	var empty protocol.CollaborationRunKnowledgeBundle
	if protocol.ValidateCollaborationRunKnowledgePrepare(p) != nil || p.Identity.RunID != util.UUIDToString(runID) {
		return empty, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if !s.KnowledgeContextEnabled {
		return empty, knowledgeFailure(409, "knowledge_invocation_unsupported")
	}
	out, err := runCollaborationKnowledgeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRunKnowledgeBundle, error) {
		q := s.Queries.WithTx(tx)
		prior, e := loadCollaborationKnowledge[collaborationKnowledgePreparation](ctx, q, runID, collaborationKnowledgePrepareKey, "knowledge_preparation")
		var exact *KnowledgeManifest
		if e == nil {
			exact = &prior.Manifest
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return empty, e
		}
		r, f, hints, e := s.lockCollaborationKnowledge(ctx, tx, runID, tokenHash, exact)
		if e != nil {
			return empty, e
		}
		if p.Identity != collaborationRunIdentity(r.run, r.access.Issue.ID) {
			return empty, knowledgeFailure(409, "knowledge_context_changed")
		}
		savedHandoff, e := loadStoredHandoff(ctx, q, runID, p.HandoffID)
		if e != nil {
			return empty, e
		}
		if savedHandoff.Identity != p.Identity {
			return empty, knowledgeFailure(409, "knowledge_context_changed")
		}
		if _, e = restoreStoredHandoff(ctx, q, r.access, r.run, savedHandoff); e != nil {
			return empty, e
		}
		// Re-read under the run lock: two original request attempts cannot both
		// create a different private snapshot after racing during hint discovery.
		stored, e := loadCollaborationKnowledge[collaborationKnowledgePreparation](ctx, q, runID, collaborationKnowledgePrepareKey, "knowledge_preparation")
		resolution := KnowledgeResolution{}
		if e == nil {
			if stored.Request != p {
				return empty, knowledgeFailure(409, "knowledge_context_changed")
			}
			valid, _, e := s.RevalidateKnowledgeManifest(ctx, q, f, stored.Manifest)
			if e != nil {
				return empty, e
			}
			if !valid {
				return empty, knowledgeFailure(409, "knowledge_context_changed")
			}
			resolution = KnowledgeResolution{Manifest: stored.Manifest, ManifestSHA256: stored.ManifestSHA256, Targets: stored.Targets, Contributions: []protocol.CollaborationKnowledgeContribution{}}
			for _, entry := range stored.Manifest.Entries {
				row, e := q.GetKnowledgeCandidate(ctx, util.MustParseUUID(entry.CandidateID))
				if e != nil {
					return empty, e
				}
				c, e := knowledgeContribution(row)
				if e != nil {
					return empty, e
				}
				resolution.Contributions = append(resolution.Contributions, c)
			}
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return empty, e
		} else {
			resolution, e = s.ResolveApprovedKnowledge(ctx, q, f)
			if e != nil {
				return empty, e
			}
			stored = collaborationKnowledgePreparation{Request: p, Manifest: resolution.Manifest, ManifestSHA256: resolution.ManifestSHA256, Targets: resolution.Targets}
			if e = storeCollaborationKnowledge(ctx, q, runID, r.access.Issue.ID, collaborationKnowledgePrepareKey, "knowledge_preparation", stored); e != nil {
				return empty, e
			}
		}
		if e = ValidateKnowledgeResolutionLocks(resolution, hints); e != nil {
			return empty, e
		}
		out := protocol.CollaborationRunKnowledgeBundle{CollaborationRunKnowledgePrepare: p, Manifest: resolution.Manifest, ManifestSHA256: resolution.ManifestSHA256, Contributions: resolution.Contributions, Targets: resolution.Targets}
		if e = protocol.ValidateCollaborationRunKnowledgeBundle(out, 0); e != nil {
			return empty, knowledgeFailure(503, "knowledge_unavailable")
		}
		return out, nil
	})
	if err != nil {
		return empty, taskKnowledgeError(err)
	}
	return out, nil
}

func (s *CollaborationService) AuthorizeRuntimeKnowledge(ctx context.Context, runID pgtype.UUID, tokenHash string, p protocol.CollaborationRunKnowledgeAuthorization) (protocol.CollaborationRunKnowledgeAuthorization, error) {
	var empty protocol.CollaborationRunKnowledgeAuthorization
	seq, valid := collaborationRuntimeVersion(p.AttemptSeq, false)
	if protocol.ValidateCollaborationRunKnowledgePrepare(p.CollaborationRunKnowledgePrepare) != nil || !valid || seq > protocol.CollaborationHostMaxLaunches || !protocol.HandoffDigest(p.PromptSHA256) || !protocol.HandoffDigest(p.ManifestSHA256) {
		return empty, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if _, err := util.ParseUUID(p.AttemptNonce); err != nil {
		return empty, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if !s.KnowledgeContextEnabled {
		return empty, knowledgeFailure(409, "knowledge_invocation_unsupported")
	}
	out, err := runCollaborationKnowledgeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRunKnowledgeAuthorization, error) {
		q := s.Queries.WithTx(tx)
		saved, e := loadCollaborationKnowledge[collaborationKnowledgePreparation](ctx, q, runID, collaborationKnowledgePrepareKey, "knowledge_preparation")
		if e != nil {
			return empty, e
		}
		if saved.Request != p.CollaborationRunKnowledgePrepare || saved.ManifestSHA256 != p.ManifestSHA256 {
			return empty, knowledgeFailure(409, "knowledge_context_changed")
		}
		r, f, hints, e := s.lockCollaborationKnowledge(ctx, tx, runID, tokenHash, &saved.Manifest)
		if e != nil {
			return empty, e
		}
		if p.Identity != collaborationRunIdentity(r.run, r.access.Issue.ID) || r.run.AuthorizedAttemptSeq != seq {
			return empty, knowledgeFailure(409, "knowledge_context_changed")
		}
		attempt, e := q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: runID, AttemptSeq: seq})
		if e != nil {
			return empty, e
		}
		var descriptor protocol.CollaborationLaunchDescriptor
		if attempt.AttemptNonce != p.AttemptNonce || !attempt.AuthorizedAt.Valid || decodeCollaborationPreparationJSON(attempt.LaunchDescriptor, &descriptor) != nil || descriptor.Kind != "provider" {
			return empty, knowledgeFailure(409, "knowledge_context_changed")
		}
		binding, e := loadCollaborationKnowledge[protocol.CollaborationHandoffBinding](ctx, q, runID, "handoff_bind:"+p.AttemptNonce, "handoff_binding")
		if e != nil {
			return empty, e
		}
		if binding.SnapshotID != p.HandoffID || binding.PromptSHA256 != p.PromptSHA256 || binding.AttemptSeq != p.AttemptSeq || binding.AttemptNonce != p.AttemptNonce {
			return empty, knowledgeFailure(409, "knowledge_context_changed")
		}
		valid, _, e := s.RevalidateKnowledgeManifest(ctx, q, f, saved.Manifest)
		if e != nil {
			return empty, e
		}
		if !valid {
			return empty, knowledgeFailure(409, "knowledge_context_changed")
		}
		for _, entry := range saved.Manifest.Entries {
			id, e := util.ParseUUID(entry.CandidateID)
			if e != nil || !slices.Contains(hints.CandidateIDs, id) {
				return empty, knowledgeFailure(409, "knowledge_context_changed")
			}
		}
		key := "knowledge:launch:" + strconv.FormatInt(seq, 10)
		old, e := loadCollaborationKnowledge[protocol.CollaborationRunKnowledgeAuthorization](ctx, q, runID, key, "knowledge_authorization")
		if e == nil {
			if old != p {
				return empty, knowledgeFailure(409, "knowledge_context_changed")
			}
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return empty, e
		} else if e = storeCollaborationKnowledge(ctx, q, runID, r.access.Issue.ID, key, "knowledge_authorization", p); e != nil {
			return empty, e
		}
		return p, nil
	})
	if err != nil {
		return empty, taskKnowledgeError(err)
	}
	return out, nil
}

func (s *CollaborationService) RuntimeKnowledgeStatus(ctx context.Context, runID pgtype.UUID, token string) (protocol.CollaborationRunKnowledgeStatus, error) {
	var out protocol.CollaborationRunKnowledgeStatus
	term, err := authorizeCollaborationTermination(ctx, s.Queries, runID, token)
	if err != nil {
		return out, err
	}
	identity, err := collaborationFrozenIdentity(term)
	if err != nil {
		return out, err
	}
	out = protocol.CollaborationRunKnowledgeStatus{ProtocolVersion: 1, Identity: identity}
	stored, err := loadCollaborationKnowledge[collaborationKnowledgePreparation](ctx, s.Queries, runID, collaborationKnowledgePrepareKey, "knowledge_preparation")
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if stored.Request.Identity != identity {
		return out, knowledgeFailure(409, "knowledge_context_changed")
	}
	out.Prepared, out.RequestID = true, stored.Request.RequestID
	return out, nil
}
