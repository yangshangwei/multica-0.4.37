//go:build agentintegration || knowledgeintegration

package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
)

// Receipt recovery cannot erase an execution or persistence error joined with
// a transport failure. Every leaf must belong to the recoverable receipt path.
func recordingReceiptRecoveryError(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !recordingReceiptRecoveryError(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return recordingReceiptRecoveryError(wrapped.Unwrap())
	}
	return err == context.DeadlineExceeded || err == collaborationrun.ErrTransport
}

func TestRecordingReceiptRecoveryRequiresOnlyTransientErrors(t *testing.T) {
	permanent := errors.New("provider or session persistence failed")
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline", context.DeadlineExceeded, true},
		{"transport", collaborationrun.ErrTransport, true},
		{"wrapped_transport", fmt.Errorf("receipt: %w", collaborationrun.ErrTransport), true},
		{"joined_transient", errors.Join(context.DeadlineExceeded, collaborationrun.ErrTransport), true},
		{"permanent", permanent, false},
		{"cancelled", context.Canceled, false},
		{"provider_and_transport", errors.Join(permanent, collaborationrun.ErrTransport), false},
		{"protocol_and_deadline", errors.Join(collaborationrun.ErrProtocol, context.DeadlineExceeded), false},
		{"nested_mixed", fmt.Errorf("outer: %w", errors.Join(context.DeadlineExceeded, fmt.Errorf("session: %w", permanent))), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := recordingReceiptRecoveryError(tc.err); got != tc.want {
				t.Fatalf("recover=%t, want %t; non-receipt errors must remain failures", got, tc.want)
			}
		})
	}
}
