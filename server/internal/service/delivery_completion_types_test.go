package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const validCompletionAuthorizationJSON = `{"contract_version":"delivery-completion-enforcement-v1","source_workspace_id":"11111111-1111-4111-8111-111111111111","operation_id":"22222222-2222-4222-8222-222222222222","target":{"kind":"ordinary","issue_id":"33333333-3333-4333-8333-333333333333","expected_issue_revision":0,"expected_status":"in_review","target_status":"done","target_position":null},"project":{"id":"44444444-4444-4444-8444-444444444444","expected_revision":0,"expected_policy_revision":1},"evidence":{"issue_id":"33333333-3333-4333-8333-333333333333","expected_issue_revision":0,"approval_id":"55555555-5555-4555-8555-555555555555","expected_source_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_spec_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_artifact_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"consumer":{"kind":"human","user_id":"66666666-6666-4666-8666-666666666666"},"expires_at":"2026-11-11T01:00:00Z","reason":"accept exact candidate"}`

func TestDeliveryCompletionRequestStrictContract(t *testing.T) {
	good, err := ParseDeliveryCompletionAuthorization([]byte(validCompletionAuthorizationJSON))
	if err != nil || good.Target.ExpectedIssueRevision != 0 || good.Consumer.Kind != "human" {
		t.Fatalf("valid zero-revision contract rejected: %v", err)
	}
	cases := map[string]string{
		"duplicate_nested": strings.Replace(validCompletionAuthorizationJSON, `"kind":"human"`, `"kind":"human","kind":"human"`, 1),
		"case_alias":       strings.Replace(validCompletionAuthorizationJSON, `"operation_id"`, `"Operation_ID"`, 1),
		"missing_revision": strings.Replace(validCompletionAuthorizationJSON, `"expected_issue_revision":0,`, "", 1),
		"null_revision":    strings.Replace(validCompletionAuthorizationJSON, `"expected_issue_revision":0`, `"expected_issue_revision":null`, 1),
		"unknown_target":   strings.Replace(validCompletionAuthorizationJSON, `"target_position":null`, `"target_position":null,"submission_id":"55555555-5555-4555-8555-555555555555"`, 1),
		"consumer_variant": strings.Replace(validCompletionAuthorizationJSON, `"kind":"human"`, `"kind":"task"`, 1),
		"zero_uuid":        strings.Replace(validCompletionAuthorizationJSON, "22222222-2222-4222-8222-222222222222", "00000000-0000-0000-0000-000000000000", 1),
		"trailing":         validCompletionAuthorizationJSON + `{}`,
		"skip":             strings.TrimSuffix(validCompletionAuthorizationJSON, "}") + `,"skip":true}`,
		"date_offset":      strings.Replace(validCompletionAuthorizationJSON, "01:00:00Z", "01:00:00+08:00", 1),
		"nul_reason":       strings.Replace(validCompletionAuthorizationJSON, "accept exact candidate", `accept\u0000candidate`, 1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseDeliveryCompletionAuthorization([]byte(raw)); err == nil {
				t.Fatal("ambiguous or malformed acceptance request admitted")
			}
		})
	}
}
func TestDeliveryCompletionDigestBindsOriginalIntent(t *testing.T) {
	a, err := ParseDeliveryCompletionAuthorization([]byte(validCompletionAuthorizationJSON))
	if err != nil {
		t.Fatal(err)
	}
	first := a.Digest()
	for name, change := range map[string]func(*DeliveryCompletionAuthorizationRequest){
		"consumer": func(x *DeliveryCompletionAuthorizationRequest) {
			x.Consumer.UserID = "77777777-7777-4777-8777-777777777777"
		},
		"operation": func(x *DeliveryCompletionAuthorizationRequest) {
			x.OperationID = "88888888-8888-4888-8888-888888888888"
		},
		"revision": func(x *DeliveryCompletionAuthorizationRequest) { x.Target.ExpectedIssueRevision++ },
		"target":   func(x *DeliveryCompletionAuthorizationRequest) { x.Target.TargetStatus = "closed_custom" },
		"position": func(x *DeliveryCompletionAuthorizationRequest) { v := 2.0; x.Target.TargetPosition = &v },
	} {
		t.Run(name, func(t *testing.T) {
			x := a
			change(&x)
			if x.Digest() == first {
				t.Fatal("acceptance digest ignored changed intent")
			}
		})
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(validCompletionAuthorizationJSON), &fields); err != nil {
		t.Fatal(err)
	}
	ordered, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseDeliveryCompletionAuthorization(ordered)
	if err != nil || b.Digest() != first {
		t.Fatalf("JSON field order changed normalized identity: %v", err)
	}
}

func TestDeliveryCompletionTypedRequestRejectsNoncanonicalExpiry(t *testing.T) {
	request, err := ParseDeliveryCompletionAuthorization([]byte(validCompletionAuthorizationJSON))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		expiry time.Time
	}{
		{"nanoseconds", request.ExpiresAt.Add(time.Nanosecond)},
		{"non_utc", request.ExpiresAt.In(time.FixedZone("task-test-offset", 8*60*60))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := request
			candidate.ExpiresAt = tc.expiry
			if err := candidate.Validate(); err == nil {
				t.Fatal("typed service request can persist an expiry that cannot round-trip through PostgreSQL and strict recovery")
			}
		})
	}
	// The HTTP boundary deliberately normalizes UTC nanoseconds before validation.
	raw := strings.Replace(validCompletionAuthorizationJSON, "01:00:00Z", "01:00:00.123456789Z", 1)
	canonical, err := ParseDeliveryCompletionAuthorization([]byte(raw))
	if err != nil || canonical.ExpiresAt.Nanosecond() != 123456000 || canonical.Validate() != nil {
		t.Fatalf("HTTP expiry normalization changed: expiry=%v error=%v", canonical.ExpiresAt, err)
	}
}
