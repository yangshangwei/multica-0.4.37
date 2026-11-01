package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Constructed by the authenticated handler; never decoded from a request body.
// TokenHash is transient and is not stored in knowledge delivery metadata.
type TaskKnowledgeCaller struct {
	RuntimeID       pgtype.UUID
	Kind, TokenHash string
}

func taskKnowledgeError(err error) error {
	if err == nil {
		return nil
	}
	var domain *CollaborationError
	if errors.As(err, &domain) {
		return err
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && (pgerr.Code == "55P03" || pgerr.Code == "40001" || pgerr.Code == "40P01") {
		return knowledgeFailure(503, "knowledge_busy")
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, auth.ErrPasswordSession) || errors.Is(err, ErrManagedRuntimeSource) {
		return knowledgeFailure(403, "knowledge_authorization_denied")
	}
	return knowledgeFailure(503, "knowledge_unavailable")
}

func lockTaskKnowledgeCredential(ctx context.Context, q *db.Queries, caller TaskKnowledgeCaller, task db.AgentTaskQueue, r db.AgentRuntime) error {
	if caller.RuntimeID != r.ID || caller.TokenHash == "" {
		return knowledgeFailure(403, "knowledge_authorization_denied")
	}
	var user pgtype.UUID
	var version int64
	switch caller.Kind {
	case "daemon_token":
		token, err := q.LockTaskKnowledgeDaemonCredential(ctx, caller.TokenHash)
		if err != nil {
			return err
		}
		if token.WorkspaceID != r.WorkspaceID || token.DaemonID != r.DaemonID.String || !token.ExpiresAt.Time.After(time.Now()) {
			return knowledgeFailure(403, "knowledge_authorization_denied")
		}
		user, version = token.UserID, token.AuthVersion
	case "task_token":
		token, err := q.LockTaskKnowledgeTaskCredential(ctx, caller.TokenHash)
		if err != nil {
			return err
		}
		if token.TaskID != task.ID || token.AgentID != task.AgentID || token.WorkspaceID != r.WorkspaceID || !token.ExpiresAt.Time.After(time.Now()) {
			return knowledgeFailure(403, "knowledge_authorization_denied")
		}
		user, version = token.UserID, token.AuthVersion
	default:
		return knowledgeFailure(403, "knowledge_authorization_denied")
	}
	if user != r.OwnerID || !user.Valid {
		return knowledgeFailure(403, "knowledge_authorization_denied")
	}
	if auth.PasswordMode() {
		_, err := auth.CheckPasswordVersion(ctx, q, util.UUIDToString(user), version)
		return err
	}
	return nil
}

func taskKnowledgeManifestUnion(a, b KnowledgeManifest) (KnowledgeManifest, error) {
	out := KnowledgeManifest{ProtocolVersion: 1, Entries: append([]KnowledgeManifestEntry{}, a.Entries...)}
	for _, entry := range b.Entries {
		if !slices.Contains(out.Entries, entry) {
			out.Entries = append(out.Entries, entry)
		}
	}
	if len(out.Entries) > 32 {
		return out, knowledgeFailure(409, "knowledge_context_changed")
	}
	slices.SortFunc(out.Entries, func(a, b KnowledgeManifestEntry) int {
		if a.CandidateID < b.CandidateID {
			return -1
		}
		if a.CandidateID > b.CandidateID {
			return 1
		}
		return 0
	})
	return out, nil
}

// Each invocation owns an independent transaction. All hints are metadata-only;
// no body escapes until commit. Do not replace this with runInTx: that helper
// acquires the managed namespace before the source graph used here.
func (s *TaskService) taskKnowledgeTransaction(ctx context.Context, caller TaskKnowledgeCaller, taskID pgtype.UUID, exact *KnowledgeManifest, fn func(*db.Queries, db.AgentTaskQueue, KnowledgeConsumerFacts, *taskKnowledgeDelivery, *taskKnowledgeClaim, KnowledgeReadLockHints) error) error {
	if !s.KnowledgeContextEnabled {
		return knowledgeFailure(409, "knowledge_invocation_unsupported")
	}
	if s.TxStarter == nil {
		return knowledgeFailure(503, "knowledge_unavailable")
	}
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return taskKnowledgeError(err)
	}
	f, r, _, err := s.taskKnowledgeFacts(ctx, s.Queries, task)
	if err != nil {
		return taskKnowledgeError(err)
	}
	d, err := decodeTaskKnowledge(task.KnowledgeContextVersion, task.KnowledgeDelivery)
	if err != nil {
		return err
	}
	c, err := taskKnowledgeClaimAt(d, task.ClaimGeneration)
	if err != nil {
		return err
	}
	if !c.Descriptor.PreparationRequired || caller.RuntimeID != task.RuntimeID {
		return knowledgeFailure(403, "knowledge_authorization_denied")
	}
	_, managed, history, err := s.taskKnowledgeExposure(ctx, s.Queries, task, c.Descriptor.Session, true)
	if err != nil {
		return taskKnowledgeError(err)
	}
	resolver := &CollaborationService{}
	hints, err := resolver.CollectKnowledgeReadLockHints(ctx, s.Queries, f, exact)
	if err != nil {
		return taskKnowledgeError(err)
	}
	if managed {
		extra, err := resolver.CollectKnowledgeReadLockHints(ctx, s.Queries, f, &history)
		if err != nil {
			return taskKnowledgeError(err)
		}
		for _, source := range extra.Sources {
			found := -1
			for i := range hints.Sources {
				if hints.Sources[i].IssueID == source.IssueID {
					found = i
					break
				}
			}
			if found < 0 {
				hints.Sources = append(hints.Sources, source)
			} else {
				for _, ref := range source.Sources {
					if !slices.Contains(hints.Sources[found].Sources, ref) {
						hints.Sources[found].Sources = append(hints.Sources[found].Sources, ref)
					}
				}
			}
		}
		for _, id := range extra.CandidateIDs {
			if !slices.Contains(hints.CandidateIDs, id) {
				hints.CandidateIDs = append(hints.CandidateIDs, id)
			}
		}
		slices.SortFunc(hints.Sources, func(a, b KnowledgeSourceLockHint) int { return knowledgeCompareUUID(a.IssueID, b.IssueID) })
		slices.SortFunc(hints.CandidateIDs, knowledgeCompareUUID)
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return taskKnowledgeError(err)
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	users := append([]pgtype.UUID{f.RuntimeOwnerUserID}, f.InputAuthorUserIDs...)
	if f.OriginatorUserID.Valid {
		users = append(users, f.OriginatorUserID)
	}
	slices.SortFunc(users, knowledgeCompareUUID)
	users = slices.Compact(users)
	accounts, err := q.LockKnowledgeUsers(ctx, users)
	if err != nil {
		return taskKnowledgeError(err)
	}
	if len(accounts) != len(users) {
		return knowledgeFailure(403, "knowledge_authorization_denied")
	}
	for _, u := range accounts {
		if u.DisabledAt.Valid {
			return knowledgeFailure(403, "knowledge_authorization_denied")
		}
	}
	if err = lockTaskKnowledgeCredential(ctx, q, caller, task, r); err != nil {
		return taskKnowledgeError(err)
	}
	workspaces := []pgtype.UUID{f.WorkspaceID}
	for _, source := range hints.Sources {
		workspaces = append(workspaces, source.WorkspaceID)
	}
	slices.SortFunc(workspaces, knowledgeCompareUUID)
	workspaces = slices.Compact(workspaces)
	for _, ws := range workspaces {
		if _, err = q.LockKnowledgeWorkspace(ctx, ws); err != nil {
			return taskKnowledgeError(err)
		}
		for _, user := range users {
			locked, err := q.TryLockKnowledgeMemberFence(ctx, db.TryLockKnowledgeMemberFenceParams{WorkspaceID: ws, UserID: user})
			if err != nil {
				return taskKnowledgeError(err)
			}
			if !locked {
				return knowledgeFailure(503, "knowledge_busy")
			}
			if _, err = q.LockKnowledgeMember(ctx, db.LockKnowledgeMemberParams{WorkspaceID: ws, UserID: user}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return taskKnowledgeError(err)
			}
		}
	}
	if err = resolver.LockKnowledgeReadTargets(ctx, q, hints); err != nil {
		return taskKnowledgeError(err)
	}
	if f.ChatSessionID.Valid {
		if _, err = q.LockTaskKnowledgeChat(ctx, f.ChatSessionID); err != nil {
			return taskKnowledgeError(err)
		}
	}
	// The ordinary consumer's current project/scope must remain stable too;
	// it need not itself be one of the collaboration knowledge sources.
	issues := []pgtype.UUID{}
	if task.IssueID.Valid {
		issues = append(issues, task.IssueID)
	}
	for _, source := range hints.Sources {
		issues = append(issues, source.IssueID)
	}
	slices.SortFunc(issues, knowledgeCompareUUID)
	issues = slices.Compact(issues)
	for _, id := range issues {
		if _, err = q.LockKnowledgeSourceIssue(ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) && id != task.IssueID {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			return taskKnowledgeError(err)
		}
	}
	if task.AutopilotRunID.Valid {
		run, err := q.GetAutopilotRun(ctx, task.AutopilotRunID)
		if err != nil {
			return taskKnowledgeError(err)
		}
		if _, err = q.LockTaskKnowledgeAutopilot(ctx, run.AutopilotID); err != nil {
			return taskKnowledgeError(err)
		}
		if _, err = q.LockTaskKnowledgeAutopilotRun(ctx, task.AutopilotRunID); err != nil {
			return taskKnowledgeError(err)
		}
	}
	if err = resolver.LockKnowledgeReadSources(ctx, q, hints); err != nil {
		return taskKnowledgeError(err)
	}
	if r.DaemonID.Valid {
		locked, err := q.TryTaskKnowledgeInstallationFence(ctx, installationNamespace(r.WorkspaceID, r.DaemonID.String))
		if err != nil {
			return taskKnowledgeError(err)
		}
		if !locked {
			return knowledgeFailure(503, "knowledge_busy")
		}
	}
	currentRuntime, err := q.LockTaskKnowledgeRuntime(ctx, task.RuntimeID)
	if err != nil {
		return taskKnowledgeError(err)
	}
	binding, err := ValidateManagedRuntime(ctx, s.runtimeLookup(q), currentRuntime)
	if err != nil {
		return taskKnowledgeError(err)
	}
	if binding.ID.Valid {
		if _, err = q.LockTaskKnowledgeInstallation(ctx, binding.InstallationID); err != nil {
			return taskKnowledgeError(err)
		}
	}
	currentAgent, err := q.LockTaskKnowledgeAgent(ctx, task.AgentID)
	if err != nil {
		return taskKnowledgeError(err)
	}
	current, err := q.LockTaskKnowledgeTask(ctx, task.ID)
	if err != nil {
		return taskKnowledgeError(err)
	}
	if current.ClaimGeneration != task.ClaimGeneration || current.RuntimeID != task.RuntimeID || current.OriginatorUserID != task.OriginatorUserID || !current.DispatchedAt.Time.Equal(task.DispatchedAt.Time) || !bytes.Equal(current.Context, task.Context) || (current.Status != "dispatched" && current.Status != "running") {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	if err = ValidateManagedTaskBinding(ctx, current); err != nil {
		return taskKnowledgeError(err)
	}
	currentFacts, _, _, err := s.taskKnowledgeFacts(ctx, q, current)
	if err != nil {
		return taskKnowledgeError(err)
	}
	if !reflect.DeepEqual(f, currentFacts) {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	d, err = decodeTaskKnowledge(current.KnowledgeContextVersion, current.KnowledgeDelivery)
	if err != nil {
		return err
	}
	c, err = taskKnowledgeClaimAt(d, current.ClaimGeneration)
	if err != nil {
		return err
	}
	if currentRuntime.ProfileID.Valid || currentRuntime.Provider != "claude" || currentRuntime.Provider != c.Descriptor.Provider || currentRuntime.RuntimeMode != "local" || currentRuntime.RuntimeMode != c.Descriptor.Mode {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	digest, err := taskKnowledgeConfigurationDigest(currentRuntime, currentAgent)
	if err != nil {
		return taskKnowledgeError(err)
	}
	if digest != c.ConfigurationDigest {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	actualBinding, actualManaged, actualHistory, err := s.taskKnowledgeExposure(ctx, q, current, c.Descriptor.Session, true)
	if err != nil {
		return taskKnowledgeError(err)
	}
	if actualManaged != managed || !reflect.DeepEqual(history, actualHistory) || managed && (actualBinding == nil || *actualBinding != c.Descriptor.Session) {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	if managed {
		ok, _, err := resolver.RevalidateKnowledgeManifest(ctx, q, f, history)
		if err != nil {
			return taskKnowledgeError(err)
		}
		if !ok {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
	}
	if err = fn(q, current, f, d, c, hints); err != nil {
		return taskKnowledgeError(err)
	}
	return taskKnowledgeError(tx.Commit(ctx))
}

func checkTaskKnowledgeIdentity(task db.AgentTaskQueue, generation, dispatched string) error {
	if strconv.FormatInt(task.ClaimGeneration, 10) != generation || task.DispatchedAt.Time.UTC().Format(time.RFC3339Nano) != dispatched {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	return nil
}

func (s *TaskService) PrepareTaskKnowledge(ctx context.Context, caller TaskKnowledgeCaller, taskID pgtype.UUID, req protocol.TaskKnowledgePrepareRequest) (protocol.TaskKnowledgePrepareResponse, error) {
	var out protocol.TaskKnowledgePrepareResponse
	if protocol.ValidateTaskKnowledgeRequest(req.ProtocolVersion, req.RequestID, req.ClaimGeneration, req.DispatchedAt) != nil || protocol.ValidateTaskKnowledgeSession(req.Session) != nil || !protocol.ValidTaskKnowledgeDigest(req.InvocationFingerprint) {
		return out, knowledgeFailure(400, "invalid_knowledge_request")
	}
	hash, err := protocol.TaskKnowledgeRequestHash(req)
	if err != nil {
		return out, err
	}
	var exact *KnowledgeManifest
	if task, e := s.Queries.GetAgentTask(ctx, taskID); e == nil {
		if d, e := decodeTaskKnowledge(task.KnowledgeContextVersion, task.KnowledgeDelivery); e == nil {
			if c, e := taskKnowledgeClaimAt(d, task.ClaimGeneration); e == nil && c.Prepared != nil {
				m := c.Prepared.Manifest
				exact = &m
			}
		}
	}
	err = s.taskKnowledgeTransaction(ctx, caller, taskID, exact, func(q *db.Queries, task db.AgentTaskQueue, f KnowledgeConsumerFacts, d *taskKnowledgeDelivery, c *taskKnowledgeClaim, hints KnowledgeReadLockHints) error {
		if err := checkTaskKnowledgeIdentity(task, req.ClaimGeneration, req.DispatchedAt); err != nil {
			return err
		}
		if req.Session != c.Descriptor.Session {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		resolver := &CollaborationService{}
		var contributions []protocol.CollaborationKnowledgeContribution
		if p := c.Prepared; p != nil {
			if exact == nil {
				return knowledgeFailure(503, "knowledge_busy")
			}
			if !sameTaskKnowledgeManifest(*exact, p.Manifest) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			if p.RequestID != req.RequestID || p.RequestHash != hash {
				return knowledgeFailure(409, "idempotency_conflict")
			}
			ok, _, err := resolver.RevalidateKnowledgeManifest(ctx, q, f, p.Manifest)
			if err != nil {
				return err
			}
			if !ok {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			contributions = []protocol.CollaborationKnowledgeContribution{}
			for _, entry := range p.Manifest.Entries {
				row, err := q.GetKnowledgeCandidate(ctx, util.MustParseUUID(entry.CandidateID))
				if err != nil {
					return err
				}
				item, err := knowledgeContribution(row)
				if err != nil {
					return err
				}
				contributions = append(contributions, item)
			}
		} else {
			if len(c.Attempts) != 0 {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			resolved, err := resolver.ResolveApprovedKnowledge(ctx, q, f)
			if err != nil {
				return err
			}
			if err = ValidateKnowledgeResolutionLocks(resolved, hints); err != nil {
				return err
			}
			_, _, history, err := s.taskKnowledgeExposure(ctx, q, task, c.Descriptor.Session, true)
			if err != nil {
				return err
			}
			if _, err = taskKnowledgeManifestUnion(history, resolved.Manifest); err != nil {
				return err
			}
			c.Prepared = &taskKnowledgePreparation{ID: uuid.NewString(), RequestID: req.RequestID, RequestHash: hash, InvocationFingerprint: req.InvocationFingerprint, Manifest: resolved.Manifest, ManifestSHA256: resolved.ManifestSHA256, Targets: resolved.Targets, DisclosedAt: time.Now().UTC()}
			c.Phase = "disclosed"
			contributions = resolved.Contributions
			raw, err := encodeTaskKnowledge(d)
			if err != nil {
				return err
			}
			if _, err = q.SetTaskKnowledgeDelivery(ctx, db.SetTaskKnowledgeDeliveryParams{TaskID: task.ID, ClaimGeneration: task.ClaimGeneration, Delivery: raw}); err != nil {
				return err
			}
		}
		p := c.Prepared
		if err := ValidateKnowledgeResolutionLocks(KnowledgeResolution{Contributions: contributions, Manifest: p.Manifest, ManifestSHA256: p.ManifestSHA256, Targets: p.Targets}, hints); err != nil {
			return err
		}
		out = protocol.TaskKnowledgePrepareResponse{ProtocolVersion: 1, PrepareID: p.ID, InvocationFingerprint: p.InvocationFingerprint, Manifest: p.Manifest, ManifestSHA256: p.ManifestSHA256, Contributions: contributions, Targets: p.Targets, Session: c.Descriptor.Session}
		material, wire, err := protocol.CollaborationKnowledgeSizes(out)
		if err != nil || material > protocol.CollaborationKnowledgeMaxRuntimeBytes || wire > protocol.CollaborationKnowledgeMaxRuntimeJSONBytes {
			return knowledgeFailure(503, "knowledge_unavailable")
		}
		return nil
	})
	if err != nil {
		return protocol.TaskKnowledgePrepareResponse{}, err
	}
	return out, nil
}

func (s *TaskService) AuthorizeTaskKnowledgeLaunch(ctx context.Context, caller TaskKnowledgeCaller, taskID pgtype.UUID, req protocol.TaskKnowledgeAuthorizeRequest) (protocol.TaskKnowledgeAuthorization, error) {
	var out protocol.TaskKnowledgeAuthorization
	if protocol.ValidateTaskKnowledgeRequest(req.ProtocolVersion, req.RequestID, req.ClaimGeneration, req.DispatchedAt) != nil || protocol.ValidateTaskKnowledgeSession(req.Session) != nil || !protocol.ValidTaskKnowledgeDigest(req.PromptSHA256) || !protocol.ValidTaskKnowledgeDigest(req.ManifestSHA256) || !protocol.ValidTaskKnowledgeDigest(req.InvocationFingerprint) || req.AttemptSequence < 1 || req.AttemptSequence > 32 {
		return out, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if _, err := uuid.Parse(req.AttemptNonce); err != nil {
		return out, knowledgeFailure(400, "invalid_knowledge_request")
	}
	if !s.KnowledgeContextEnabled {
		return out, knowledgeFailure(409, "knowledge_invocation_unsupported")
	}
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return out, taskKnowledgeError(err)
	}
	d, err := decodeTaskKnowledge(task.KnowledgeContextVersion, task.KnowledgeDelivery)
	if err != nil {
		return out, err
	}
	c, err := taskKnowledgeClaimAt(d, task.ClaimGeneration)
	if err != nil || c.Prepared == nil {
		return out, knowledgeFailure(409, "knowledge_context_changed")
	}
	exact := c.Prepared.Manifest
	hash, err := protocol.TaskKnowledgeRequestHash(req)
	if err != nil {
		return out, err
	}
	err = s.taskKnowledgeTransaction(ctx, caller, taskID, &exact, func(q *db.Queries, task db.AgentTaskQueue, f KnowledgeConsumerFacts, d *taskKnowledgeDelivery, c *taskKnowledgeClaim, hints KnowledgeReadLockHints) error {
		if err := checkTaskKnowledgeIdentity(task, req.ClaimGeneration, req.DispatchedAt); err != nil {
			return err
		}
		p := c.Prepared
		if p == nil || p.ID != req.PrepareID || p.ManifestSHA256 != req.ManifestSHA256 || p.InvocationFingerprint != req.InvocationFingerprint || req.Session != c.Descriptor.Session {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		ok, _, err := (&CollaborationService{}).RevalidateKnowledgeManifest(ctx, q, f, p.Manifest)
		if err != nil {
			return err
		}
		if !ok {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		for _, attempt := range c.Attempts {
			if attempt.RequestID == req.RequestID || attempt.Authorization.AttemptSequence == req.AttemptSequence {
				if attempt.RequestID != req.RequestID || attempt.RequestHash != hash {
					return knowledgeFailure(409, "idempotency_conflict")
				}
				out = attempt.Authorization
				return nil
			}
		}
		if req.AttemptSequence != int64(len(c.Attempts)+1) {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		for _, attempt := range c.Attempts {
			if !attempt.Terminated || attempt.Refused {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			if req.Session.NativeSessionID == "" && attempt.BoundSessionID != "" {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
		}
		out = protocol.TaskKnowledgeAuthorization{ProtocolVersion: 1, PrepareID: p.ID, AttemptSequence: req.AttemptSequence, AttemptNonce: req.AttemptNonce, ManifestSHA256: p.ManifestSHA256, InvocationFingerprint: p.InvocationFingerprint, PromptSHA256: req.PromptSHA256, Session: req.Session}
		c.Attempts = append(c.Attempts, taskKnowledgeAttempt{RequestID: req.RequestID, RequestHash: hash, Authorization: out, Observations: []protocol.TaskKnowledgeObservation{}})
		raw, err := encodeTaskKnowledge(d)
		if err != nil {
			return err
		}
		_, err = q.SetTaskKnowledgeDelivery(ctx, db.SetTaskKnowledgeDeliveryParams{TaskID: task.ID, ClaimGeneration: task.ClaimGeneration, Delivery: raw})
		return err
	})
	if err != nil {
		return protocol.TaskKnowledgeAuthorization{}, err
	}
	return out, nil
}

// Compare canonical metadata, not order-sensitive marshaled history hints.
func sameTaskKnowledgeManifest(a, b KnowledgeManifest) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(aa, bb)
}
