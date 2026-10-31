package collaborationrun

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const hostHandoffContinuityGuidance = "If a private provider conversation cannot be resumed, continue from this supplied startup snapshot without assuming missing private context."

func (c *Client) HandoffSnapshot(ctx context.Context, identity protocol.CollaborationRunIdentity, token string, p protocol.CollaborationHandoffSnapshotRequest) (protocol.CollaborationHandoffEnvelope, error) {
	var out protocol.CollaborationHandoffEnvelope
	if validateIdentity(identity) != nil || !validUUID(p.RequestID) || p.MemoryVersion != "" && !protocol.HandoffDecimal(p.MemoryVersion, true) {
		return out, ErrProtocol
	}
	if err := c.requestLimit(ctx, http.MethodPost, runPath(identity.RunID, "handoff/snapshots"), token, p, &out, protocol.CollaborationHandoffMaxJSONBytes); err != nil {
		return out, err
	}
	if out.RunID != identity.RunID || out.IssueID != identity.IssueID || out.SnapshotID != p.RequestID || out.Mode != "snapshot" || protocol.ValidateCollaborationHandoff(out) != nil {
		return out, ErrProtocol
	}
	return out, nil
}
func (c *Client) BindHandoff(ctx context.Context, identity protocol.CollaborationRunIdentity, token string, p protocol.CollaborationHandoffBinding) error {
	if validateIdentity(identity) != nil || !validUUID(p.SnapshotID) || !validDigest(p.PromptSHA256) || !validHostAttempt(p.AttemptSeq) || !validUUID(p.AttemptNonce) {
		return ErrProtocol
	}
	var out protocol.CollaborationHandoffBindingReceipt
	if err := c.request(ctx, http.MethodPost, runPath(identity.RunID, "handoff/bind"), token, p, &out); err != nil {
		return err
	}
	if out.ProtocolVersion != 1 || !out.Accepted || out.IssueID != identity.IssueID || out.RunID != identity.RunID || out.SnapshotID != p.SnapshotID || out.PromptSHA256 != p.PromptSHA256 || out.AttemptSeq != p.AttemptSeq || out.AttemptNonce != p.AttemptNonce {
		return ErrProtocol
	}
	return nil
}
func (r *Runner) hostHandoff(ctx context.Context, claim protocol.CollaborationRunClaimResponse, token string, memory *protocol.CollaborationRunMemory, previous *protocol.CollaborationHandoffEnvelope) (*protocol.CollaborationHandoffEnvelope, error) {
	if claim.HandoffVersion == 0 {
		return nil, nil
	}
	if claim.HandoffVersion != 1 {
		return nil, ErrProtocol
	}
	version := ""
	if memory != nil {
		version = memory.Version
	}
	id, err := uuid.Parse(claim.Identity.RunID)
	if err != nil {
		return nil, ErrProtocol
	}
	// The run identity is already durable. Neither a new launch nor a purged
	// memory base may mint a new logical snapshot and silently change its facts.
	request := protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewSHA1(id, []byte("multica:handoff-snapshot:v1")).String(), MemoryVersion: version}
	var out protocol.CollaborationHandoffEnvelope
	err = r.retryHandoff(ctx, func() error {
		var e error
		out, e = r.Client.HandoffSnapshot(ctx, claim.Identity, token, request)
		return e
	})
	if err != nil {
		return nil, err
	}
	if validateHostHandoff(claim, memory, out) != nil || previous != nil && !reflect.DeepEqual(*previous, out) {
		return nil, ErrProtocol
	}
	return &out, nil
}

func validateHostHandoff(claim protocol.CollaborationRunClaimResponse, memory *protocol.CollaborationRunMemory, handoff protocol.CollaborationHandoffEnvelope) error {
	if claim.HostContext == nil || protocol.ValidateCollaborationHandoff(handoff) != nil || handoff.IssueID != claim.Identity.IssueID || handoff.RunID != claim.Identity.RunID || handoff.Mode != "snapshot" {
		return ErrProtocol
	}
	parent := ""
	if claim.HostContext.ParentRunID != nil {
		parent = *claim.HostContext.ParentRunID
	}
	if handoff.Assignment.Role != claim.HostContext.Role || handoff.Assignment.StepKey != claim.HostContext.StepKey || handoff.Assignment.ParentRunID != parent {
		return ErrProtocol
	}
	if memory == nil {
		if handoff.Memory.Availability != "unsupported" || handoff.Memory.Version != "0" || handoff.Memory.SHA256 != "" {
			return ErrProtocol
		}
	} else if memory.Version == "0" {
		if handoff.Memory.Availability != "absent" || handoff.Memory.Version != "0" || handoff.Memory.SHA256 != "" {
			return ErrProtocol
		}
	} else if handoff.Memory.Availability != "available" || handoff.Memory.Version != memory.Version || handoff.Memory.SHA256 != protocol.CollaborationHandoffHash(memory.Content) {
		return ErrProtocol
	}
	for _, source := range handoff.Sources {
		if source.Trust == "admitted_assignment" && (claim.HostContext.Role != "member" || source.Kind != "delegation" || source.ID != handoff.Assignment.SourceID || source.RunID != parent) {
			return ErrProtocol
		}
	}
	if claim.HostContext.Role == "member" {
		if !validUUID(handoff.Assignment.DelegationID) || !validUUID(handoff.Assignment.SourceID) || parent == "" {
			return ErrProtocol
		}
		for _, source := range handoff.Sources {
			if source.Kind == "delegation" && source.ID == handoff.Assignment.SourceID && source.RunID == parent && source.Trust == "admitted_assignment" &&
				source.Availability == "available" && !source.Truncated && strings.TrimSpace(source.Content) != "" {
				// SHA256 describes the original source, while Content can be
				// redacted. The server revalidates the original before binding.
				return nil
			}
		}
		return ErrProtocol
	}
	return nil
}
func (r *Runner) retryHandoff(ctx context.Context, call func() error) error {
	attempts, backoff := r.PermitAttempts, r.PermitBackoff
	if attempts <= 0 {
		attempts = defaultPermitAttempts
	}
	if backoff <= 0 {
		backoff = defaultPermitBackoff
	}
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			timer := time.NewTimer(backoff << (i - 1))
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return errors.Join(err, ctx.Err())
			}
		}
		err = call()
		if err == nil {
			return nil
		}
		var refusal *HTTPError
		if !errors.Is(err, ErrTransport) && !(errors.As(err, &refusal) && (refusal.Status == 408 || refusal.Status == 429 || refusal.Status >= 500)) {
			return err
		}
	}
	return err
}
func (s *hostSupervisor) bindHandoff(ctx context.Context) error {
	s.mu.Lock()
	if s.local.record.Host.Stopped {
		s.mu.Unlock()
		return ErrSealed
	}
	snapshot, hash := s.handoffSnapshot, s.handoffPromptHash
	seq, nonce, token := s.local.record.AttemptSeq, s.local.record.AttemptNonce, s.local.record.Host.BusinessToken
	s.mu.Unlock()
	if snapshot == "" {
		if s.claim.HandoffVersion == 1 {
			return ErrProtocol
		}
		return nil
	}
	request := protocol.CollaborationHandoffBinding{SnapshotID: snapshot, PromptSHA256: hash, AttemptSeq: seq, AttemptNonce: nonce}
	return s.runner.retryHandoff(ctx, func() error { return s.runner.Client.BindHandoff(ctx, s.claim.Identity, token, request) })
}
func (s *hostSupervisor) setHandoff(handoff *protocol.CollaborationHandoffEnvelope, prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handoffSnapshot = ""
	s.handoffPromptHash = ""
	if handoff != nil {
		s.handoffSnapshot = handoff.SnapshotID
		s.handoffPromptHash = protocol.CollaborationHandoffHash(prompt)
	}
}
