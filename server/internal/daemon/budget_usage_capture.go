package daemon

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/multica-ai/multica/server/pkg/agent"
)

// Capturing the provider invocations of an attempt (budget usage protocol v1).
//
// executeAndDrain is the one place that launches the provider process of a
// task, and the only place that knows whether the Result it hands back is the
// backend's own or one it made up on its way out. It says so through an
// origin holder in the context. executeAndDrainRecorded reads the holder and
// writes the invocation into the record of the attempt.

// budgetUsageResultSource says where the Result of one executeAndDrain call
// came from.
type budgetUsageResultSource int

const (
	// budgetUsageResultMadeUp is a Result executeAndDrain built itself: the run
	// was stopped before the backend delivered one, or the backend closed its
	// Result channel without a value. It is the zero value, so a return path
	// that sets no mark counts as made up.
	budgetUsageResultMadeUp budgetUsageResultSource = iota
	// budgetUsageResultFromBackend is the backend's own Result, whatever
	// executeAndDrain changed about its status afterwards.
	budgetUsageResultFromBackend
	// budgetUsageResultFromFailedStart means backend.Execute returned an
	// error: no provider run exists.
	budgetUsageResultFromFailedStart
)

// budgetUsageResultOrigin is the holder executeAndDrain marks. It is written
// by the goroutine that runs executeAndDrain and read by its caller after the
// call returned.
type budgetUsageResultOrigin struct {
	source budgetUsageResultSource
}

type budgetUsageResultOriginKey struct{}

// withBudgetUsageResultOrigin asks the executeAndDrain call that receives the
// context to mark where its Result came from.
func withBudgetUsageResultOrigin(ctx context.Context, origin *budgetUsageResultOrigin) context.Context {
	return context.WithValue(ctx, budgetUsageResultOriginKey{}, origin)
}

func budgetUsageResultOriginFrom(ctx context.Context) *budgetUsageResultOrigin {
	origin, _ := ctx.Value(budgetUsageResultOriginKey{}).(*budgetUsageResultOrigin)
	return origin
}

// markBudgetUsageStartFailed marks that backend.Execute returned an error.
// Without a holder in the context it does nothing.
func markBudgetUsageStartFailed(ctx context.Context) {
	if origin := budgetUsageResultOriginFrom(ctx); origin != nil {
		origin.source = budgetUsageResultFromFailedStart
	}
}

// markBudgetUsageBackendResult marks that the Result channel of the backend
// was read. delivered is false when the channel was closed without a value:
// the Result in hand is then an empty one the backend never sent. Without a
// holder in the context it does nothing.
func markBudgetUsageBackendResult(ctx context.Context, delivered bool) {
	if !delivered {
		return
	}
	if origin := budgetUsageResultOriginFrom(ctx); origin != nil {
		origin.source = budgetUsageResultFromBackend
	}
}

// budgetUsageInvocationOutcome says how an invocation ended and what it used,
// from where the Result came from and the status that was handed back.
//
// A launch that could not start returned and used nothing. The backend's own
// Result ended the invocation by itself when it says completed or failed;
// with any other status the run was cut short, and what the backend counted
// until then is still its usage. A Result executeAndDrain made up carries no
// usage of the provider.
func budgetUsageInvocationOutcome(source budgetUsageResultSource, result agent.Result) (string, map[string]agent.TokenUsage) {
	switch source {
	case budgetUsageResultFromFailedStart:
		return budgetUsageEndedReturned, nil
	case budgetUsageResultFromBackend:
		if result.Status == "completed" || result.Status == "failed" {
			return budgetUsageEndedReturned, result.Usage
		}
		return budgetUsageEndedInterrupted, result.Usage
	default:
		return budgetUsageEndedInterrupted, nil
	}
}

// executeAndDrainRecorded is executeAndDrain for a provider launch of a
// claimed task. When the context carries the usage report record of a budget
// attempt, the launch is written to it as one invocation: the intent before
// the provider is started, and afterwards how it ended together with the
// usage of this launch alone. Without a record it is executeAndDrain.
func (d *Daemon) executeAndDrainRecorded(ctx context.Context, backend agent.Backend, prompt string, opts agent.ExecOptions, taskLog *slog.Logger, taskID, codexHome string, msgSeq *atomic.Int32) (agent.Result, int32, error) {
	attempt := budgetUsageAttemptFrom(ctx)
	if attempt == nil {
		return d.executeAndDrain(ctx, backend, prompt, opts, taskLog, taskID, codexHome, msgSeq)
	}
	// A launch counts as fresh only when it was given no session to resume and
	// no argument beyond the daemon's own. The arguments are judged as they
	// were passed to this launch, not by what an adapter keeps of them.
	resumed := opts.ResumeSessionID != "" || len(opts.ExtraArgs) > 0 || len(opts.CustomArgs) > 0
	invocationID := attempt.beginInvocation(resumed)
	origin := &budgetUsageResultOrigin{}
	result, tools, err := d.executeAndDrain(withBudgetUsageResultOrigin(ctx, origin), backend, prompt, opts, taskLog, taskID, codexHome, msgSeq)
	ended, usage := budgetUsageInvocationOutcome(origin.source, result)
	attempt.endInvocation(invocationID, ended, usage)
	return result, tools, err
}

// beginBudgetUsageAttempt creates the usage report record of a claimed task,
// right before its run. It returns nil when this process has no journal, when
// the task carries no budget attempt id, or when the record cannot be
// created; the task then runs exactly as it does without the protocol.
func (d *Daemon) beginBudgetUsageAttempt(task Task, provider string, taskLog *slog.Logger) *budgetUsageAttempt {
	if d.budgetUsage == nil || task.BudgetAttemptID == "" {
		return nil
	}
	_, customProfileCommand := d.customProfileLaunchForRuntime(task.RuntimeID)
	attempt, err := d.budgetUsage.begin(budgetUsageSeed{
		AttemptID:         task.BudgetAttemptID,
		WorkspaceID:       task.WorkspaceID,
		TaskID:            task.ID,
		Adapter:           budgetUsageAdapterName(provider, customProfileCommand),
		NormalizerVersion: agent.UsageNormalizerVersion(provider),
	}, taskLog)
	if err != nil {
		// The attempt id is whatever the claim carried; it is named only once
		// it is known to be an id.
		attrs := []any{"error", err}
		if managedUUID.MatchString(task.BudgetAttemptID) {
			attrs = append(attrs, "attempt_id", task.BudgetAttemptID)
		}
		taskLog.Warn("budget usage: no usage report record for this run, running the task without the budget usage protocol", attrs...)
		return nil
	}
	return attempt
}
