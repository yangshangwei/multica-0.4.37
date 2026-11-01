package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const maxTaskKnowledgeClaims = 32

// This document is private server metadata. It never contains contributions,
// credentials or a Provider transcript and is never a public task projection.
type taskKnowledgeDelivery struct {
	Version   int                  `json:"version"`
	ScopeKind string               `json:"scope_kind"`
	ScopeID   string               `json:"scope_id"`
	Claims    []taskKnowledgeClaim `json:"claims"`
}
type taskKnowledgeClaim struct {
	Phase               string                                `json:"phase"`
	DaemonID            string                                `json:"daemon_id"`
	RuntimeOwnerUserID  string                                `json:"runtime_owner_user_id"`
	Descriptor          protocol.TaskKnowledgeClaimDescriptor `json:"descriptor"`
	ConfigurationDigest string                                `json:"configuration_digest"`
	Prepared            *taskKnowledgePreparation             `json:"prepared,omitempty"`
	Attempts            []taskKnowledgeAttempt                `json:"attempts"`
}
type taskKnowledgePreparation struct {
	ID                    string                                 `json:"id"`
	RequestID             string                                 `json:"request_id"`
	RequestHash           string                                 `json:"request_hash"`
	InvocationFingerprint string                                 `json:"invocation_fingerprint"`
	Manifest              protocol.TaskKnowledgeManifest         `json:"manifest"`
	ManifestSHA256        string                                 `json:"manifest_sha256"`
	Targets               []protocol.TaskKnowledgeTargetRevision `json:"targets"`
	DisclosedAt           time.Time                              `json:"disclosed_at"`
}
type taskKnowledgeAttempt struct {
	RequestID             string                              `json:"request_id"`
	RequestHash           string                              `json:"request_hash"`
	Authorization         protocol.TaskKnowledgeAuthorization `json:"authorization"`
	Observations          []protocol.TaskKnowledgeObservation `json:"observations"`
	BoundSessionID        string                              `json:"bound_session_id,omitempty"`
	Started               bool                                `json:"started"`
	Terminated            bool                                `json:"terminated"`
	Refused               bool                                `json:"refused"`
	RefusedObservationIDs []string                            `json:"refused_observation_ids,omitempty"`
}

func decodeTaskKnowledge(version pgtype.Int4, raw []byte) (*taskKnowledgeDelivery, error) {
	if !version.Valid && (len(raw) == 0 || bytes.Equal(raw, []byte("null"))) {
		return nil, nil
	}
	if !version.Valid || version.Int32 != 1 || len(raw) == 0 || len(raw) > protocol.CollaborationKnowledgeMaxDeliveryBytes {
		return nil, knowledgeFailure(409, "knowledge_context_changed")
	}
	var d taskKnowledgeDelivery
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return nil, knowledgeFailure(409, "knowledge_context_changed")
	}
	if decoder.Decode(new(any)) != io.EOF || d.Version != 1 || d.ScopeID == "" || len(d.Claims) == 0 || len(d.Claims) > maxTaskKnowledgeClaims {
		return nil, knowledgeFailure(409, "knowledge_context_changed")
	}
	seen := map[string]bool{}
	for _, claim := range d.Claims {
		if !protocol.ValidTaskKnowledgeIdentity(claim.DaemonID) {
			return nil, knowledgeFailure(409, "knowledge_context_changed")
		}
		if _, err := uuid.Parse(claim.RuntimeOwnerUserID); err != nil {
			return nil, knowledgeFailure(409, "knowledge_context_changed")
		}
		if claim.Phase != "claimed" && claim.Phase != "disclosed" || claim.Phase == "claimed" && (claim.Prepared != nil || len(claim.Attempts) != 0) || claim.Phase == "disclosed" && claim.Prepared == nil {
			return nil, knowledgeFailure(409, "knowledge_context_changed")
		}
		c := claim.Descriptor
		if protocol.ValidateTaskKnowledgeRequest(c.ProtocolVersion, c.TaskID, c.ClaimGeneration, c.DispatchedAt) != nil || c.Provider != "claude" || c.Mode != "local" || !c.PreparationRequired || c.RuntimeID != c.Session.RuntimeID || c.Provider != c.Session.Provider || c.ConfigurationID != c.Session.ConfigurationID || seen[c.ClaimGeneration] || !protocol.ValidTaskKnowledgeDigest(claim.ConfigurationDigest) ||
			protocol.ValidateTaskKnowledgeSession(c.Session) != nil || c.Session.ScopeID != d.ScopeID || c.Session.ScopeKind != d.ScopeKind || len(claim.Attempts) > 32 || claim.Attempts == nil {
			return nil, knowledgeFailure(409, "knowledge_context_changed")
		}
		seen[c.ClaimGeneration] = true
		if p := claim.Prepared; p != nil {
			hash, err := protocol.TaskKnowledgeRequestHash(p.Manifest)
			_, idErr := uuid.Parse(p.ID)
			_, requestErr := uuid.Parse(p.RequestID)
			if err != nil || idErr != nil || requestErr != nil || !protocol.ValidTaskKnowledgeDigest(p.RequestHash) || p.Manifest.ProtocolVersion != 1 || p.Manifest.Entries == nil || len(p.Manifest.Entries) > 32 || hash != p.ManifestSHA256 || !protocol.ValidTaskKnowledgeDigest(p.InvocationFingerprint) || p.DisclosedAt.IsZero() {
				return nil, knowledgeFailure(409, "knowledge_context_changed")
			}
		}
		attemptKeys := map[string]bool{}
		for i, a := range claim.Attempts {
			_, nonceErr := uuid.Parse(a.Authorization.AttemptNonce)
			_, requestErr := uuid.Parse(a.RequestID)
			if claim.Prepared == nil || nonceErr != nil || requestErr != nil || attemptKeys[a.RequestID] || attemptKeys[a.Authorization.AttemptNonce] || !protocol.ValidTaskKnowledgeDigest(a.RequestHash) || a.Authorization.ProtocolVersion != 1 || a.Authorization.ManifestSHA256 != claim.Prepared.ManifestSHA256 || a.Authorization.InvocationFingerprint != claim.Prepared.InvocationFingerprint || !protocol.ValidTaskKnowledgeDigest(a.Authorization.PromptSHA256) || a.Authorization.AttemptSequence != int64(i+1) || a.Authorization.PrepareID != claim.Prepared.ID || a.Authorization.Session != c.Session || len(a.Observations) > 8 || a.Observations == nil || !validTaskKnowledgeObservations(c, a) {
				return nil, knowledgeFailure(409, "knowledge_context_changed")
			}
			attemptKeys[a.RequestID] = true
			attemptKeys[a.Authorization.AttemptNonce] = true
		}
	}
	return &d, nil
}

func validTaskKnowledgeObservations(claim protocol.TaskKnowledgeClaimDescriptor, attempt taskKnowledgeAttempt) bool {
	started, terminated, refused := false, false, false
	bound := ""
	denied := []string{}
	seen := map[string]bool{}
	for _, o := range attempt.Observations {
		if protocol.ValidateTaskKnowledgeRequest(o.ProtocolVersion, o.RequestID, o.ClaimGeneration, o.DispatchedAt) != nil || o.ClaimGeneration != claim.ClaimGeneration || o.DispatchedAt != claim.DispatchedAt || o.PrepareID != attempt.Authorization.PrepareID || o.AttemptNonce != attempt.Authorization.AttemptNonce || o.AttemptSequence != attempt.Authorization.AttemptSequence || seen[o.RequestID] {
			return false
		}
		seen[o.RequestID] = true
		switch o.Kind {
		case "started":
			if started || terminated || o.ProcessID <= 0 || !protocol.ValidTaskKnowledgeIdentity(o.ProcessInstanceID) {
				return false
			}
			started = true
		case "native_session":
			if !started || terminated || !protocol.ValidTaskKnowledgeIdentity(o.NativeSessionID) || (o.Source != "system" && o.Source != "result") {
				return false
			}
			if refused || claim.Session.NativeSessionID != "" && claim.Session.NativeSessionID != o.NativeSessionID || bound != "" && bound != o.NativeSessionID {
				refused = true
				denied = append(denied, o.RequestID)
			} else {
				bound = o.NativeSessionID
			}
		case "terminated":
			if terminated {
				return false
			}
			switch o.TerminationKind {
			case "not_started":
				if started || o.WaitObserved {
					return false
				}
				terminated = true
			case "managed_launch_ended":
				if !started || !o.WaitObserved {
					return false
				}
				terminated = true
			case "unknown":
				refused = true
			default:
				return false
			}
		default:
			return false
		}
	}
	return started == attempt.Started && terminated == attempt.Terminated && refused == attempt.Refused && bound == attempt.BoundSessionID && slices.Equal(denied, attempt.RefusedObservationIDs)
}

func encodeTaskKnowledge(d *taskKnowledgeDelivery) ([]byte, error) {
	raw, err := json.Marshal(d)
	if err != nil || len(raw) > protocol.CollaborationKnowledgeMaxDeliveryBytes {
		return nil, knowledgeFailure(409, "knowledge_context_changed")
	}
	if _, err = decodeTaskKnowledge(pgtype.Int4{Int32: 1, Valid: true}, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func taskKnowledgeClaimAt(d *taskKnowledgeDelivery, generation int64) (*taskKnowledgeClaim, error) {
	if d != nil {
		for i := range d.Claims {
			if d.Claims[i].Descriptor.ClaimGeneration == strconv.FormatInt(generation, 10) {
				return &d.Claims[i], nil
			}
		}
	}
	return nil, knowledgeFailure(409, "knowledge_context_changed")
}

func (s *TaskService) taskKnowledgeFacts(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) (KnowledgeConsumerFacts, db.AgentRuntime, db.Agent, error) {
	r, err := s.runtimeLookup(q).Get(ctx, task.RuntimeID)
	if err != nil {
		return KnowledgeConsumerFacts{}, r, db.Agent{}, err
	}
	a, err := q.GetAgent(ctx, task.AgentID)
	if err != nil {
		return KnowledgeConsumerFacts{}, r, a, err
	}
	f := KnowledgeConsumerFacts{Supported: true, WorkspaceID: r.WorkspaceID, RuntimeOwnerUserID: r.OwnerID, RuntimeID: r.ID, AgentID: a.ID, TaskID: task.ID, OriginatorUserID: task.OriginatorUserID, Audience: "workspace"}
	if a.RuntimeID != r.ID || a.WorkspaceID != r.WorkspaceID || a.ArchivedAt.Valid {
		return f, r, a, knowledgeFailure(403, "knowledge_authorization_denied")
	}
	switch {
	case task.ChatSessionID.Valid:
		f.Surface, f.Audience, f.ChatSessionID = "chat", "private_chat", task.ChatSessionID
		chat, e := q.GetChatSession(ctx, task.ChatSessionID)
		if e != nil {
			return f, r, a, e
		}
		if chat.WorkspaceID != r.WorkspaceID || chat.AgentID != task.AgentID {
			return f, r, a, knowledgeFailure(403, "knowledge_authorization_denied")
		}
		f.ProjectID = chat.ProjectID
	case task.IssueID.Valid:
		f.Surface = "issue"
		issue, e := q.GetIssue(ctx, task.IssueID)
		if e != nil {
			return f, r, a, e
		}
		if issue.WorkspaceID != r.WorkspaceID {
			return f, r, a, knowledgeFailure(403, "knowledge_authorization_denied")
		}
		f.ProjectID = issue.ProjectID
	case task.AutopilotRunID.Valid:
		f.Surface = "autopilot"
		run, e := q.GetAutopilotRun(ctx, task.AutopilotRunID)
		if e != nil {
			return f, r, a, e
		}
		auto, e := q.GetAutopilot(ctx, run.AutopilotID)
		if e != nil {
			return f, r, a, e
		}
		if auto.WorkspaceID != r.WorkspaceID {
			return f, r, a, knowledgeFailure(403, "knowledge_authorization_denied")
		}
		f.ProjectID = auto.ProjectID
	default:
		var qc QuickCreateContext
		if json.Unmarshal(task.Context, &qc) != nil || qc.Type != QuickCreateContextType || qc.WorkspaceID != util.UUIDToString(r.WorkspaceID) {
			return f, r, a, knowledgeFailure(403, "knowledge_authorization_denied")
		}
		f.Surface = "quick_create"
		if qc.ProjectID != "" {
			f.ProjectID, err = util.ParseUUID(qc.ProjectID)
			if err != nil {
				return f, r, a, knowledgeFailure(403, "knowledge_authorization_denied")
			}
		}
	}
	if _, e := q.GetChannelTaskDelivery(ctx, task.ID); e == nil {
		f.Audience = "unavailable"
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return f, r, a, e
	}
	comments := append([]pgtype.UUID{}, task.CoalescedCommentIds...)
	if task.TriggerCommentID.Valid {
		comments = append(comments, task.TriggerCommentID)
	}
	for _, id := range comments {
		comment, err := q.GetComment(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			f.Audience = "unavailable"
			continue
		}
		if err != nil {
			return f, r, a, err
		}
		if comment.AuthorType != "member" || !comment.AuthorID.Valid {
			f.Audience = "unavailable"
			continue
		}
		f.InputAuthorUserIDs = append(f.InputAuthorUserIDs, comment.AuthorID)
	}
	slices.SortFunc(f.InputAuthorUserIDs, knowledgeCompareUUID)
	f.InputAuthorUserIDs = slices.Compact(f.InputAuthorUserIDs)
	return f, r, a, nil
}

func taskKnowledgeScope(task db.AgentTaskQueue, f KnowledgeConsumerFacts) (string, string) {
	switch f.Surface {
	case "issue":
		return "issue", util.UUIDToString(task.IssueID)
	case "chat":
		return "chat", util.UUIDToString(task.ChatSessionID)
	case "autopilot":
		return "autopilot", util.UUIDToString(task.AutopilotRunID)
	default:
		return "quick_create", util.UUIDToString(task.ID)
	}
}

func taskKnowledgeConfigurationDigest(r db.AgentRuntime, a db.Agent) (string, error) {
	return protocol.TaskKnowledgeRequestHash(struct {
		RuntimeID                        pgtype.UUID
		OwnerID                          pgtype.UUID
		Provider                         string
		Mode                             string
		DaemonID                         string
		ProfileID                        pgtype.UUID
		RuntimeEpoch                     int64
		AgentID                          pgtype.UUID
		AgentOwner                       pgtype.UUID
		Instructions                     string
		Config, Args, Env, MCP, Disabled []byte
		Model, Thinking, Tier            pgtype.Text
	}{r.ID, r.OwnerID, r.Provider, r.RuntimeMode, r.DaemonID.String, r.ProfileID, r.CollaborationConfigEpoch, a.ID, a.OwnerID, a.Instructions, a.RuntimeConfig, a.CustomArgs, a.CustomEnv, a.McpConfig, a.DisabledRuntimeSkills, a.Model, a.ThinkingLevel, a.ServiceTier})
}

func taskKnowledgeInvocationSupported(inv *protocol.TaskKnowledgeInvocation, r db.AgentRuntime, a db.Agent) bool {
	if inv == nil || inv.ProtocolVersion != 1 || !inv.Builtin || r.ProfileID.Valid || !r.OwnerID.Valid || !r.DaemonID.Valid || !protocol.ValidTaskKnowledgeIdentity(r.DaemonID.String) || r.Provider != inv.Provider || r.RuntimeMode != "local" || inv.Mode != "local" || !protocol.ValidTaskKnowledgeIdentity(inv.ConfigurationID) || len(inv.ConfigurationID) > 256 {
		return false
	}
	if !agent.KnowledgeLifecycleSupported(inv.Provider, inv.Mode, inv.Builtin) || validateTaskKnowledgeOrdinaryArgs(inv.Provider, inv.ExtraArgs) != nil {
		return false
	}
	var args []string
	if len(a.CustomArgs) > 0 && !bytes.Equal(a.CustomArgs, []byte("null")) && json.Unmarshal(a.CustomArgs, &args) != nil {
		return false
	}
	return validateTaskKnowledgeOrdinaryArgs(inv.Provider, args) == nil
}

func validateTaskKnowledgeOrdinaryArgs(provider string, args []string) error {
	// C4's intentionally non-reusable collaboration namespace can use this
	// flag. Ordinary knowledge still binds native pointers for future resume.
	for _, arg := range args {
		if name, _, _ := strings.Cut(arg, "="); name == "--no-session-persistence" {
			return knowledgeFailure(409, "knowledge_invocation_unsupported")
		}
	}
	return agent.ValidateKnowledgeSessionArgs(provider, args)
}

// BuildTaskKnowledgeClaim checks the finally chosen native session, including
// old-client claims. It discloses no contribution and does not finalize a claim.
func (s *TaskService) BuildTaskKnowledgeClaim(ctx context.Context, task db.AgentTaskQueue, inv *protocol.TaskKnowledgeInvocation, chosen string) (*protocol.TaskKnowledgeClaimDescriptor, string, error) {
	r, err := s.runtimeLookup(s.Queries).Get(ctx, task.RuntimeID)
	if err != nil {
		return nil, "", err
	}
	a, err := s.Queries.GetAgent(ctx, task.AgentID)
	if err != nil {
		return nil, "", err
	}
	if inv != nil && inv.ProtocolVersion == 1 && r.Provider == "claude" {
		var custom []string
		if len(a.CustomArgs) > 0 && !bytes.Equal(a.CustomArgs, []byte("null")) {
			if json.Unmarshal(a.CustomArgs, &custom) != nil {
				return nil, "", knowledgeFailure(409, "knowledge_invocation_unsupported")
			}
		}
		// Session selectors are refused whatever the gate: transcripts written
		// while it was on stay on disk and an explicit selector can reach them.
		// Subcommand operands and a bare "--" are refused with them, because
		// they stop the managed argv from governing the process.
		// Other options outside the negotiation allowlist (and
		// --no-session-persistence) only make the invocation unsupported below,
		// where the history guard chooses base execution or refusal.
		if agent.KnowledgeSessionSelectorPresent("claude", inv.ExtraArgs) || agent.KnowledgeSessionSelectorPresent("claude", custom) {
			return nil, "", knowledgeFailure(409, "knowledge_invocation_unsupported")
		}
	}
	generic := false
	if !task.IssueID.Valid && !task.ChatSessionID.Valid && !task.AutopilotRunID.Valid {
		var quick QuickCreateContext
		generic = json.Unmarshal(task.Context, &quick) != nil || quick.Type != QuickCreateContextType
	}
	supported := !generic && s.KnowledgeContextEnabled && taskKnowledgeInvocationSupported(inv, r, a)
	if !supported {
		// A reclaimed row with retained negotiation cannot be delivered to an
		// unfenced client. New unrelated legacy tasks can remain ordinary when the
		// complete relevant namespace proves no private managed exposure.
		if task.KnowledgeContextVersion.Valid {
			return nil, "", knowledgeFailure(409, "knowledge_invocation_unsupported")
		}
		exposed, err := s.taskKnowledgeRuntimeExposure(ctx, r)
		if err != nil {
			return nil, "", err
		}
		if exposed {
			return nil, "", knowledgeFailure(409, "knowledge_invocation_unsupported")
		}
		return nil, chosen, nil
	}
	f, r, a, err := s.taskKnowledgeFacts(ctx, s.Queries, task)
	if err != nil {
		return nil, "", err
	}
	if !taskKnowledgeInvocationSupported(inv, r, a) {
		return nil, "", knowledgeFailure(409, "knowledge_context_changed")
	}
	kind, id := taskKnowledgeScope(task, f)
	binding := protocol.TaskKnowledgeSessionBinding{Provider: r.Provider, RuntimeID: util.UUIDToString(r.ID), AgentID: util.UUIDToString(a.ID), ConfigurationID: inv.ConfigurationID, ScopeKind: kind, ScopeID: id, Generation: uuid.NewString()}
	if chosen != "" {
		existing, managed, err := s.taskKnowledgeResume(ctx, task, f, binding, chosen, true)
		if err != nil {
			return nil, "", err
		}
		if existing != nil {
			binding = *existing
		} else if managed {
			chosen = ""
		} else {
			binding.NativeSessionID = chosen
		}
	}
	d := &protocol.TaskKnowledgeClaimDescriptor{ProtocolVersion: 1, TaskID: util.UUIDToString(task.ID), RuntimeID: util.UUIDToString(task.RuntimeID), ClaimGeneration: strconv.FormatInt(task.ClaimGeneration, 10), DispatchedAt: task.DispatchedAt.Time.UTC().Format(time.RFC3339Nano), Provider: r.Provider, Mode: inv.Mode, ConfigurationID: inv.ConfigurationID, PreparationRequired: true, Session: binding}
	return d, chosen, nil
}

func (s *TaskService) taskKnowledgeRuntimeExposure(ctx context.Context, r db.AgentRuntime) (bool, error) {
	rows, err := s.Queries.ListTaskKnowledgeRuntimeHistory(ctx, db.ListTaskKnowledgeRuntimeHistoryParams{RuntimeID: r.ID, DaemonID: r.DaemonID.String, Provider: r.Provider, OwnerID: util.UUIDToString(r.OwnerID)})
	if err != nil {
		return false, err
	}
	if len(rows) > 128 {
		return false, knowledgeFailure(409, "knowledge_context_changed")
	}
	for _, row := range rows {
		d, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
		if err != nil {
			return false, err
		}
		if d == nil {
			continue
		}
		for _, c := range d.Claims {
			if c.Descriptor.TaskID != util.UUIDToString(row.ID) || c.Descriptor.Session.AgentID != util.UUIDToString(row.AgentID) {
				return false, knowledgeFailure(409, "knowledge_context_changed")
			}
			if c.Prepared != nil && len(c.Prepared.Manifest.Entries) > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *TaskService) taskKnowledgeResume(ctx context.Context, task db.AgentTaskQueue, f KnowledgeConsumerFacts, want protocol.TaskKnowledgeSessionBinding, native string, supported bool) (*protocol.TaskKnowledgeSessionBinding, bool, error) {
	want.NativeSessionID = native
	binding, managed, manifest, err := s.taskKnowledgeExposure(ctx, s.Queries, task, want, supported)
	if err != nil {
		return nil, managed, err
	}
	if managed && binding != nil {
		ok, _, err := (&CollaborationService{}).RevalidateKnowledgeManifest(ctx, s.Queries, f, manifest)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, nil
		}
	}
	return binding, managed, nil
}

func (s *TaskService) taskKnowledgeExposure(ctx context.Context, q *db.Queries, task db.AgentTaskQueue, want protocol.TaskKnowledgeSessionBinding, supported bool) (*protocol.TaskKnowledgeSessionBinding, bool, KnowledgeManifest, error) {
	manifest := KnowledgeManifest{ProtocolVersion: 1, Entries: []KnowledgeManifestEntry{}}
	if want.NativeSessionID == "" {
		return nil, false, manifest, nil
	}
	runtime, err := s.runtimeLookup(q).Get(ctx, task.RuntimeID)
	if err != nil {
		return nil, false, manifest, err
	}
	owner, err := q.GetAgent(ctx, task.AgentID)
	if err != nil {
		return nil, false, manifest, err
	}
	configuration, err := taskKnowledgeConfigurationDigest(runtime, owner)
	if err != nil {
		return nil, false, manifest, err
	}
	rows, err := q.ListTaskKnowledgeHistory(ctx, db.ListTaskKnowledgeHistoryParams{AgentID: task.AgentID, IssueID: task.IssueID, ChatSessionID: task.ChatSessionID, ScopeID: want.ScopeID, NativeSessionID: want.NativeSessionID})
	if err != nil {
		return nil, false, manifest, err
	}
	if len(rows) > 128 {
		return nil, true, manifest, knowledgeFailure(409, "knowledge_context_changed")
	}
	var binding *protocol.TaskKnowledgeSessionBinding
	managed := false
	for _, row := range rows {
		d, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
		if err != nil {
			return nil, true, manifest, err
		}
		if d == nil {
			continue
		}
		for _, c := range d.Claims {
			b := c.Descriptor.Session
			matches := b.NativeSessionID == want.NativeSessionID
			for _, attempt := range c.Attempts {
				if supported && attempt.Refused && (attempt.BoundSessionID == want.NativeSessionID || b.NativeSessionID == want.NativeSessionID) {
					return nil, true, manifest, nil
				}
				if attempt.BoundSessionID == want.NativeSessionID {
					matches = true
					b.NativeSessionID = want.NativeSessionID
				}
				for _, observation := range attempt.Observations {
					if observation.NativeSessionID == want.NativeSessionID && attempt.BoundSessionID != want.NativeSessionID {
						return nil, true, manifest, knowledgeFailure(409, "knowledge_context_changed")
					}
				}
			}
			if !matches {
				if row.SessionID.String == want.NativeSessionID && c.Prepared != nil {
					return nil, true, manifest, knowledgeFailure(409, "knowledge_context_changed")
				}
				continue
			}
			if c.Prepared == nil {
				continue
			}
			if len(c.Prepared.Manifest.Entries) == 0 && !supported {
				continue
			}
			managed = true
			if !supported {
				return nil, true, manifest, knowledgeFailure(409, "knowledge_invocation_unsupported")
			}
			if c.ConfigurationDigest != configuration {
				return nil, true, manifest, nil
			}
			if b.Provider != want.Provider || b.RuntimeID != want.RuntimeID || b.AgentID != want.AgentID || b.ConfigurationID != want.ConfigurationID || b.ScopeKind != want.ScopeKind || b.ScopeID != want.ScopeID {
				return nil, true, manifest, nil
			}
			if binding != nil && *binding != b {
				return nil, true, manifest, nil
			}
			copy := b
			binding = &copy
			for _, entry := range c.Prepared.Manifest.Entries {
				if !slices.Contains(manifest.Entries, entry) {
					manifest.Entries = append(manifest.Entries, entry)
				}
			}
			if len(manifest.Entries) > 32 {
				return nil, true, manifest, nil
			}
		}
	}
	slices.SortFunc(manifest.Entries, func(a, b KnowledgeManifestEntry) int {
		if a.CandidateID < b.CandidateID {
			return -1
		}
		if a.CandidateID > b.CandidateID {
			return 1
		}
		return 0
	})
	return binding, managed, manifest, nil
}

// Finalization holds only the pre-existing runtime/task locks. No source,
// membership or workspace authorization graph is imported into this writer.
func (s *TaskService) finalizeTaskKnowledgeClaim(ctx context.Context, q *db.Queries, task db.AgentTaskQueue, descriptor *protocol.TaskKnowledgeClaimDescriptor) error {
	if descriptor == nil {
		return nil
	}
	if !s.KnowledgeContextEnabled {
		return knowledgeFailure(409, "knowledge_invocation_unsupported")
	}
	if descriptor.TaskID != util.UUIDToString(task.ID) || descriptor.RuntimeID != util.UUIDToString(task.RuntimeID) || descriptor.ClaimGeneration != strconv.FormatInt(task.ClaimGeneration, 10) || descriptor.DispatchedAt != task.DispatchedAt.Time.UTC().Format(time.RFC3339Nano) || protocol.ValidateTaskKnowledgeSession(descriptor.Session) != nil {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	d, err := decodeTaskKnowledge(task.KnowledgeContextVersion, task.KnowledgeDelivery)
	if err != nil {
		return err
	}
	if d == nil {
		d = &taskKnowledgeDelivery{Version: 1, ScopeKind: descriptor.Session.ScopeKind, ScopeID: descriptor.Session.ScopeID, Claims: []taskKnowledgeClaim{}}
	}
	if c, e := taskKnowledgeClaimAt(d, task.ClaimGeneration); e == nil {
		if c.Descriptor != *descriptor {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		return nil
	}
	if len(d.Claims) >= maxTaskKnowledgeClaims {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	r, err := s.runtimeLookup(q).Get(ctx, task.RuntimeID)
	if err != nil {
		return err
	}
	a, err := q.GetAgent(ctx, task.AgentID)
	if err != nil {
		return err
	}
	if r.ProfileID.Valid || r.Provider != "claude" || r.RuntimeMode != "local" || descriptor.Provider != r.Provider || descriptor.Mode != r.RuntimeMode || descriptor.Session.Provider != r.Provider || descriptor.Session.AgentID != util.UUIDToString(task.AgentID) || descriptor.Session.RuntimeID != util.UUIDToString(r.ID) || descriptor.Session.ConfigurationID != descriptor.ConfigurationID || !descriptor.PreparationRequired {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	kind, id := taskKnowledgeScope(task, KnowledgeConsumerFacts{Surface: descriptor.Session.ScopeKind})
	if kind != descriptor.Session.ScopeKind || id != descriptor.Session.ScopeID {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	digest, err := taskKnowledgeConfigurationDigest(r, a)
	if err != nil {
		return err
	}
	d.Claims = append(d.Claims, taskKnowledgeClaim{Phase: "claimed", DaemonID: r.DaemonID.String, RuntimeOwnerUserID: util.UUIDToString(r.OwnerID), Descriptor: *descriptor, ConfigurationDigest: digest, Attempts: []taskKnowledgeAttempt{}})
	raw, err := encodeTaskKnowledge(d)
	if err != nil {
		return err
	}
	_, err = q.SetTaskKnowledgeDelivery(ctx, db.SetTaskKnowledgeDeliveryParams{TaskID: task.ID, ClaimGeneration: task.ClaimGeneration, Delivery: raw})
	return err
}
