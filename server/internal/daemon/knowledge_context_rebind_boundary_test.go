package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// This canonical boundary matrix does not launch the fixture executable. A
// cleared pointer alone cannot authorize a claimed native-session rebind.
func TestKnowledgeRebindClaimRequiresOwnReachabilityDecision(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Task, *agent.ExecOptions)
		valid  bool
	}{
		{name: "own recorded drop", valid: true},
		{name: "missing drop provenance", change: func(task *Task, _ *agent.ExecOptions) { task.droppedPriorSessionID = "" }},
		{name: "another session was dropped", change: func(task *Task, _ *agent.ExecOptions) { task.droppedPriorSessionID = "other-native" }},
		{name: "task still requests resume", change: func(task *Task, _ *agent.ExecOptions) { task.PriorSessionID = "claimed-native" }},
		{name: "options still request resume", change: func(_ *Task, opts *agent.ExecOptions) { opts.ResumeSessionID = "claimed-native" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			f.task.KnowledgeContext.Session.NativeSessionID = "claimed-native"
			f.task.droppedPriorSessionID = "claimed-native"
			opts := agent.ExecOptions{}
			if tc.change != nil {
				tc.change(&f.task, &opts)
			}
			dropped, err := validateKnowledgeClaim(f.task, "claude", f.task.KnowledgeContext.ConfigurationID, opts)
			if tc.valid {
				if err != nil || !dropped {
					t.Fatalf("own reachability decision was refused: dropped=%v err=%v", dropped, err)
				}
			} else if dropped || agent.KnowledgeFailureReason(err) != agent.KnowledgeContextChanged {
				t.Fatalf("unproven resume drop was accepted: dropped=%v err=%v", dropped, err)
			}
		})
	}
}

// Generation and native ID are the only fields that may change on a flagged
// preparation. Validate through the consumer so removing its call to the
// session predicate is detected too.
func TestKnowledgeRebindPreparationPreservesClaimIdentity(t *testing.T) {
	claimedGeneration := uuid.NewString()
	for _, tc := range []struct {
		name   string
		change func(*protocol.TaskKnowledgeSessionBinding)
	}{
		{name: "reuses claimed generation", change: func(got *protocol.TaskKnowledgeSessionBinding) { got.Generation = claimedGeneration }},
		{name: "invalid generation", change: func(got *protocol.TaskKnowledgeSessionBinding) { got.Generation = "not-a-uuid" }},
		{name: "retains native session", change: func(got *protocol.TaskKnowledgeSessionBinding) { got.NativeSessionID = "claimed-native" }},
		{name: "changes provider", change: func(got *protocol.TaskKnowledgeSessionBinding) { got.Provider = "codex" }},
		{name: "changes runtime", change: func(got *protocol.TaskKnowledgeSessionBinding) { got.RuntimeID = uuid.NewString() }},
		{name: "changes agent", change: func(got *protocol.TaskKnowledgeSessionBinding) { got.AgentID = uuid.NewString() }},
		{name: "changes configuration", change: func(got *protocol.TaskKnowledgeSessionBinding) {
			got.ConfigurationID = protocol.CollaborationKnowledgeHash("another configuration")
		}},
		{name: "changes scope kind", change: func(got *protocol.TaskKnowledgeSessionBinding) { got.ScopeKind = "chat" }},
		{name: "changes scope identity", change: func(got *protocol.TaskKnowledgeSessionBinding) { got.ScopeID = uuid.NewString() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			claimed := f.task.KnowledgeContext.Session
			claimed.Generation = claimedGeneration
			claimed.NativeSessionID = "claimed-native"
			fingerprint := protocol.CollaborationKnowledgeHash("actual invocation")
			prepared := f.prepare(protocol.TaskKnowledgePrepareRequest{Session: claimed, InvocationFingerprint: fingerprint})
			prepared.Session.Generation, prepared.Session.NativeSessionID = uuid.NewString(), ""
			if err := validateKnowledgePreparation(f.task, claimed, true, fingerprint, prepared); err != nil {
				t.Fatalf("unchanged fresh binding refused: %v", err)
			}
			if err := validateKnowledgePreparation(f.task, claimed, false, fingerprint, prepared); agent.KnowledgeFailureReason(err) != agent.KnowledgeContextChanged {
				t.Fatalf("an unflagged preparation substituted a fresh binding: %v", err)
			}
			tc.change(&prepared.Session)
			if err := validateKnowledgePreparation(f.task, claimed, true, fingerprint, prepared); agent.KnowledgeFailureReason(err) != agent.KnowledgeContextChanged {
				t.Fatalf("preparation changed a protected binding field: %v", err)
			}
		})
	}
}

func TestKnowledgePromptRebindUsesPreparedFreshIdentity(t *testing.T) {
	for _, stale := range []bool{false, true} {
		name := "fresh server binding"
		if stale {
			name = "echoed claimed binding"
		}
		t.Run(name, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			f.task.KnowledgeContext.Session.NativeSessionID = "claimed-native"
			f.task.droppedPriorSessionID = "claimed-native"
			f.ignoreResumeUnavailable = stale
			cfg := agent.Config{ExecutablePath: f.bin, BuiltinRuntime: true, Logger: f.d.logger}
			opts := agent.ExecOptions{Cwd: t.TempDir()}
			prompt, err := f.d.prepareKnowledgePrompt(context.Background(), f.task, "claude", cfg, &opts, nil)
			if opts.Lifecycle != nil {
				t.Cleanup(opts.Lifecycle.Stop)
			}
			if stale {
				if agent.KnowledgeFailureReason(err) != agent.KnowledgeContextChanged || prompt != "" || opts.Knowledge != nil {
					t.Fatalf("echoed binding reached prompt/intent construction: err=%v intent=%v", err, opts.Knowledge)
				}
			} else {
				if err != nil || prompt == "" || opts.Knowledge == nil || opts.Knowledge.Session.Kind != agent.KnowledgeSessionFresh ||
					opts.Knowledge.Session.Generation != f.freshGeneration || opts.Knowledge.Session.NativeSessionID != "" {
					t.Fatalf("launch intent did not use the prepared fresh binding: err=%v intent=%v", err, opts.Knowledge)
				}
			}
			calls := f.callsFor("prepare")
			if len(calls) != 1 || len(f.callsFor("authorize")) != 0 {
				t.Fatal("prompt preparation launched or retried business work")
			}
			var request protocol.TaskKnowledgePrepareRequest
			if err := json.Unmarshal(calls[0].body, &request); err != nil || !request.ResumeUnavailable || request.Session != f.task.KnowledgeContext.Session {
				t.Fatalf("prepare did not retain the original claimed binding: %v", err)
			}
			if _, err := os.Stat(f.argv); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("boundary test executed a Provider")
			}
		})
	}
}
