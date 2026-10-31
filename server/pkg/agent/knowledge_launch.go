package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	KnowledgeSessionFresh          = "fresh"
	KnowledgeSessionResume         = "resume"
	KnowledgeContextChanged        = "knowledge_context_changed"
	KnowledgeAuthorizationDenied   = "knowledge_authorization_denied"
	KnowledgeUnavailable           = "knowledge_unavailable"
	KnowledgeInvocationUnsupported = "knowledge_invocation_unsupported"
)

// KnowledgeSessionIntent is the selection authorized for one command. Generation
// is an opaque server-issued scope generation, never a claim generation or native
// ID. A fresh intent has no native ID until the adapter observes one.
type KnowledgeSessionIntent struct {
	Kind            string
	Generation      string
	NativeSessionID string
}

// KnowledgeLaunchIntent contains no private material or credentials. The caller
// resolves the full server binding; this leaf compares the chosen native ID and
// effective invocation to that admission. ConfigurationID is the caller's opaque
// namespace. InvocationFingerprint binds the app-managed executable and options,
// not unobservable changes to vendor/host configuration.
type KnowledgeLaunchIntent struct {
	ConfigurationID       string
	InvocationFingerprint string
	Session               KnowledgeSessionIntent
}

// LaunchAttempt is assigned once at the physical start gate. Callback closures
// and native-session observations retain this same identity after admission.
type LaunchAttempt struct {
	Sequence int64
	Nonce    string
}

// NativeSessionObservation is a raw adapter fact, separate from process start,
// termination and business success. MatchesIntent=false must be retained as an
// unapproved selection, never used to move the old session's exposure or pointer.
type NativeSessionObservation struct {
	Attempt                 LaunchAttempt
	Knowledge               KnowledgeLaunchIntent
	NativeSessionID         string
	PreviousNativeSessionID string
	Source                  string
	MatchesIntent           bool
}

// KnowledgeLaunchError is terminal for this logical input. Integration must
// exclude it from automatic resume, transport and task replay policies. Error
// text is deliberately content-free; the original cause remains inspectable.
type KnowledgeLaunchError struct {
	Reason string
	Err    error
}

func (e *KnowledgeLaunchError) Error() string { return e.Reason }
func (e *KnowledgeLaunchError) Unwrap() error { return e.Err }

func KnowledgeFailureReason(err error) string {
	var refusal *KnowledgeLaunchError
	if errors.As(err, &refusal) {
		return refusal.Reason
	}
	return ""
}

func knowledgeFailure(err error) error {
	if err == nil || KnowledgeFailureReason(err) != "" {
		return err
	}
	return &KnowledgeLaunchError{Reason: KnowledgeUnavailable, Err: err}
}

func knowledgeUnsupported() error {
	return &KnowledgeLaunchError{Reason: KnowledgeInvocationUnsupported, Err: ErrLaunchUnsupported}
}

// KnowledgeLifecycleSupported is intentionally narrower than host process
// supervision. A custom executable speaking the Claude protocol is not the
// built-in runtime. This allowlist is not vendor-version or rollout evidence.
func KnowledgeLifecycleSupported(provider, mode string, builtinRuntime bool) bool {
	return builtinRuntime && provider == "claude" && (mode == "" || mode == "local")
}

// ValidateKnowledgeSessionArgs validates extra/default arguments when selection
// must be provably controlled. Do not silently filter a selector from user input.
// Unknown flags/subcommands are refused: a denylist alone cannot establish the
// semantics of a new vendor switch. Ordinary legacy execution does not call it.
// This argument check alone does not certify a custom/remote executable.
func ValidateKnowledgeSessionArgs(provider string, args []string) error {
	if provider != "claude" || len(args) > 64 {
		return knowledgeUnsupported()
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) > 4096 || strings.ContainsAny(arg, "\x00\r\n") {
			return knowledgeUnsupported()
		}
		name, value, equals := strings.Cut(arg, "=")
		switch name {
		case "--verbose":
			if equals {
				return knowledgeUnsupported()
			}
		case "--model", "--max-turns", "--max-budget-usd":
			if !equals {
				i++
				if i >= len(args) {
					return knowledgeUnsupported()
				}
				value = args[i]
			}
			if value == "" || strings.HasPrefix(value, "-") || len(value) > 4096 || strings.ContainsAny(value, "\x00\r\n") {
				return knowledgeUnsupported()
			}
		default:
			return knowledgeUnsupported()
		}
	}
	return nil
}

// KnowledgeInvocationFingerprint can be computed before negotiation without
// starting a CLI. It includes every option used by buildClaudeArgs, the raw
// default/custom arguments and managed MCP bytes. Prompt, session selection,
// credentials and liveness settings are intentionally separate from this hash.
// The caller still owns runtime/account identity and external config admission.
func KnowledgeInvocationFingerprint(provider string, cfg Config, opts ExecOptions) (string, error) {
	if !KnowledgeLifecycleSupported(provider, opts.OpenclawMode, cfg.BuiltinRuntime) ||
		cfg.provider != "" && cfg.provider != provider || cfg.knowledgeLaunchPrefix || len(cfg.LaunchPrefix) != 0 ||
		!filepath.IsAbs(cfg.ExecutablePath) {
		return "", knowledgeUnsupported()
	}
	if err := ValidateKnowledgeSessionArgs(provider, opts.ExtraArgs); err != nil {
		return "", err
	}
	if err := ValidateKnowledgeSessionArgs(provider, opts.CustomArgs); err != nil {
		return "", err
	}
	// Marshal a fixed shape rather than ExecOptions, which also contains the
	// observer, the expected digest and fields unrelated to this adapter.
	raw, err := json.Marshal(struct {
		Version      int
		Provider     string
		Mode         string
		Executable   string
		CLIVersion   string
		Cwd          string
		Model        string
		MaxTurns     int
		Thinking     string
		ExtraArgs    []string
		CustomArgs   []string
		MCP          json.RawMessage
		SettingsPath string
	}{1, provider, opts.OpenclawMode, cfg.ExecutablePath, cfg.CLIVersion, opts.Cwd,
		opts.Model, opts.MaxTurns, opts.ThinkingLevel, opts.ExtraArgs, opts.CustomArgs,
		opts.McpConfig, opts.ClaudeSettingsPath})
	if err != nil {
		return "", knowledgeUnsupported()
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func validKnowledgeIdentity(value string) bool {
	return value != "" && len(value) <= 4096 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func validateKnowledgeIntent(opts ExecOptions) error {
	intent := opts.Knowledge
	if intent == nil || !validKnowledgeIdentity(intent.ConfigurationID) || !validKnowledgeIdentity(intent.Session.Generation) {
		return knowledgeUnsupported()
	}
	switch intent.Session.Kind {
	case KnowledgeSessionFresh:
		if intent.Session.NativeSessionID != "" || opts.ResumeSessionID != "" {
			return knowledgeUnsupported()
		}
	case KnowledgeSessionResume:
		if !validKnowledgeIdentity(intent.Session.NativeSessionID) || opts.ResumeSessionID != intent.Session.NativeSessionID {
			return knowledgeUnsupported()
		}
	default:
		return knowledgeUnsupported()
	}
	return nil
}

func withKnowledgeClaudeLifecycle(ctx context.Context, cfg Config, opts ExecOptions) (result context.Context, err error) {
	defer func() {
		if opts.Knowledge != nil && opts.Lifecycle != nil && err != nil {
			opts.Lifecycle.fail(err)
		}
	}()
	if opts.Knowledge != nil {
		if err := validateKnowledgeIntent(opts); err != nil {
			return nil, err
		}
		fingerprint, err := KnowledgeInvocationFingerprint(cfg.provider, cfg, opts)
		if err != nil {
			return nil, err
		}
		if fingerprint != opts.Knowledge.InvocationFingerprint {
			return nil, &KnowledgeLaunchError{Reason: KnowledgeContextChanged}
		}
	}
	return withExecutionLifecycle(ctx, opts, cfg.provider)
}

// observeNativeSession is called on the exact command before any lossy public
// status publication or requested-ID fallback. No lifecycle-wide last-session
// state exists: even two sequential commands have distinct observation state.
func (s *observedLaunch) observeNativeSession(nativeID, source string) error {
	s.nativeSessionMu.Lock()
	defer s.nativeSessionMu.Unlock()
	if err := s.lifecycle.Err(); err != nil {
		return err
	}
	if nativeID == "" || nativeID == s.nativeSessionID {
		return nil
	}
	if !validKnowledgeIdentity(nativeID) {
		// A malformed/unbounded field is not a native identity. Keep the
		// admitted generation uncertain rather than forwarding arbitrary
		// stream bytes through the metadata-only observation boundary.
		err := &KnowledgeLaunchError{Reason: KnowledgeContextChanged}
		s.lifecycle.fail(err)
		s.lifecycle.Stop()
		return err
	}
	intent := s.info.Knowledge
	matches := s.nativeSessionID == "" &&
		(intent.Session.Kind == KnowledgeSessionFresh || intent.Session.NativeSessionID == nativeID)
	observation := NativeSessionObservation{
		Attempt: s.info.Attempt, Knowledge: intent, NativeSessionID: nativeID,
		PreviousNativeSessionID: s.nativeSessionID, Source: source, MatchesIntent: matches,
	}
	// Report even a changed ID before refusing so persistence can retain the
	// uncertainty without relabeling the old session's history as this one.
	err := knowledgeFailure(s.callbacks.NativeSessionObserved(observation))
	if !matches {
		err = &KnowledgeLaunchError{Reason: KnowledgeContextChanged, Err: err}
	}
	if err != nil {
		s.lifecycle.fail(err)
		s.lifecycle.Stop()
		return err
	}
	s.nativeSessionID = nativeID
	return nil
}

// knowledgeCommandLaunch retains the admitted command state even after Wait
// releases the lifecycle registry entry. It is never resolved by session text.
func knowledgeCommandLaunch(cmd *exec.Cmd) *observedLaunch {
	if state, ok := observedRuntimeCommands.Load(cmd); ok && state.(*observedLaunch).info.Knowledge.ConfigurationID != "" {
		return state.(*observedLaunch)
	}
	return nil
}
