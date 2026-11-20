package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStageExecutionFixtureUsesObservationClock(t *testing.T) {
	observedAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	target := StageExecutionTarget{ConditionalVersion: "initial"}
	source := &stageExecutionSourceFixture{
		key:    []byte("synthetic-observation-clock-signing-key"),
		target: target,
		observationClock: func(context.Context) (time.Time, error) {
			return observedAt, nil
		},
	}
	invocation := StageExecutionInvocation{
		IntentID:           "fixture-intent",
		ExternalRequestKey: "fixture-request",
		Target:             target,
		TargetDigest:       strings.Repeat("b", 64),
		ArtifactDigest:     strings.Repeat("c", 64),
	}
	proof, err := source.Execute(t.Context(), invocation)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := source.decode(proof, "execution")
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Observation.ObservedAt.Equal(observedAt) {
		t.Fatalf("synthetic proof used host time instead of the supplied observation clock: got %s, want %s", decoded.Observation.ObservedAt, observedAt)
	}
	if !stageObservationValid(invocation, decoded.Observation, observedAt) {
		t.Fatal("proof from the authoritative observation clock was rejected")
	}
	if stageObservationValid(invocation, decoded.Observation, observedAt.Add(-time.Millisecond)) {
		t.Fatal("future proof must remain rejected by the production validation")
	}
}

func TestStageExecutionFixturePropagatesObservationClockFailure(t *testing.T) {
	want := errors.New("synthetic clock unavailable")
	source := &stageExecutionSourceFixture{
		observationClock: func(context.Context) (time.Time, error) {
			return time.Time{}, want
		},
	}
	proof, err := source.Execute(t.Context(), StageExecutionInvocation{})
	if !errors.Is(err, want) || len(proof.Payload) != 0 || len(proof.Authentication) != 0 {
		t.Fatalf("clock failure must not become a signed observation: proof=%+v err=%v", proof, err)
	}
}
