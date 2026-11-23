package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"os"
	"strings"
	"testing"
)

func usageFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../pkg/protocol/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestIssueBudgetUsageDecodeFrozenFixtures(t *testing.T) {
	for _, tc := range []struct {
		name, digest string
		invocations  int
	}{
		{"budget_usage_v1_batch.json", "0d76f057284362e657cb20e1e864b66883f8e4f553e15ac540ae0d2439db7dc0", 3},
		{"budget_usage_v1_batch_empty.json", "bfe8d8baf7b275a45c9bb28251a5f86b7f2726dba691c55e4de6de944b63cdf8", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch, err := DecodeIssueBudgetUsage(usageFixture(t, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			if batch.PayloadDigest != tc.digest || len(batch.Invocations) != tc.invocations {
				t.Fatalf("batch digest/size = %s/%d", batch.PayloadDigest, len(batch.Invocations))
			}
		})
	}
}

func TestIssueBudgetUsageDecodeRejects(t *testing.T) {
	base := string(usageFixture(t, "budget_usage_v1_batch.json"))
	cases := []struct{ name, from, to, path string }{
		{"unknown", `"batch_key":`, `"extra":1,"batch_key":`, "extra"},
		{"case", `"batch_key":`, `"Batch_Key":`, "Batch_Key"},
		{"duplicate", `"resumed":false`, `"resumed":false,"resumed":false`, "invocations[0].resumed"},
		{"missing", `"resumed":false,`, ``, "invocations[0].resumed"},
		{"null", `"resumed":false`, `"resumed":null`, "invocations[0].resumed"},
		{"boolean", `"resumed":false`, `"resumed":"false"`, "invocations[0].resumed"},
		{"protocol", `"protocol_version":1`, `"protocol_version":2`, "protocol_version"},
		{"protocol string", `"protocol_version":1`, `"protocol_version":"1"`, "protocol_version"},
		{"batch key", `"batch_key":"0195f3a2-0000-7000-8000-000000000002"`, `"batch_key":"short"`, "batch_key"},
		{"adapter", `"adapter":"claude"`, `"adapter":"Claude"`, "adapter"},
		{"epoch", `"producer_epoch":"0195f39e-0000-7000-8000-00000000b01d"`, `"producer_epoch":"bad epoch"`, "producer_epoch"},
		{"normalizer", `"normalizer_version":"1"`, `"normalizer_version":""`, "normalizer_version"},
		{"version", `"adapter_version":"2.1.34 (Claude Code)"`, `"adapter_version":""`, "adapter_version"},
		{"timestamp", `"reported_at":"2026-11-17T08:00:00Z"`, `"reported_at":"yesterday"`, "reported_at"},
		{"observed", `"observed_at":"2026-11-17T07:59:40Z"`, `"observed_at":"bad"`, "invocations[0].observed_at"},
		{"array null", `"invocations":[`, `"invocations":null,"unused":[`, "invocations"},
		{"usage null", `"usage":[`, `"usage":null,"unused":[`, "invocations[0].usage"},
		{"uuid uppercase", `00000000000a`, `00000000000A`, "invocations[0].invocation_id"},
		{"uuid duplicate", `00000000000b`, `00000000000a`, "invocations[1].invocation_id"},
		{"ended", `"ended":"returned"`, `"ended":"complete"`, "invocations[0].ended"},
		{"unknown usage", `"ended":"returned"`, `"ended":"unknown"`, "invocations[0].usage"},
		{"model empty", `"model":"<synthetic>"`, `"model":""`, "invocations[0].usage[0].model"},
		{"model long", `"model":"<synthetic>"`, `"model":"` + strings.Repeat("x", 121) + `"`, "invocations[0].usage[0].model"},
		{"model duplicate", `"model":"claude-fable-5"`, `"model":"<synthetic>"`, "invocations[0].usage[1].model"},
		{"decimal number", `"input":"0"`, `"input":0`, "invocations[0].usage[0].tokens.input"},
		{"decimal leading zero", `"input":"0"`, `"input":"00"`, "invocations[0].usage[0].tokens.input"},
		{"decimal overflow", `"input":"0"`, `"input":"9223372036854775808"`, "invocations[0].usage[0].tokens.input"},
		{"cost zero", `"cost_ticks":"81000000"`, `"cost_ticks":"0"`, "invocations[0].usage[0].cost_ticks"},
		{"cost null", `"cost_ticks":"81000000"`, `"cost_ticks":null`, "invocations[0].usage[0].cost_ticks"},
		{"empty item", `,"cost_ticks":"81000000"`, ``, "invocations[0].usage[0]"},
		{"control", `<synthetic>`, "bad" + string([]byte{'\\', 'u', '0', '0', '8', '5'}), "invocations[0].usage[0].model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := strings.Replace(base, tc.from, tc.to, 1)
			_, err := DecodeIssueBudgetUsage([]byte(raw))
			if err == nil || !strings.Contains(err.Error(), tc.path) {
				t.Fatalf("want error at %s, got %v", tc.path, err)
			}
		})
	}
	for _, raw := range [][]byte{[]byte("null"), []byte("[]"), []byte(base + "{}"), append([]byte(base), 0xff), bytes.Repeat([]byte{' '}, IssueBudgetUsageMaxBytes+1)} {
		if _, err := DecodeIssueBudgetUsage(raw); err == nil {
			t.Fatal("accepted invalid root/body")
		}
	}
	for _, field := range []string{"mode", "sequence", "baseline_event_id", "overlap", "supersedes", "currency", "zero_basis", "price_version", "proof"} {
		t.Run("forbidden/"+field, func(t *testing.T) {
			raw := strings.Replace(base, `"model":"<synthetic>"`, `"`+field+`":true,"model":"<synthetic>"`, 1)
			if _, err := DecodeIssueBudgetUsage([]byte(raw)); err == nil || !strings.Contains(err.Error(), "invocations[0].usage[0]."+field) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestIssueBudgetUsageDecodeBoundaries(t *testing.T) {
	b := usageBatch(t)
	b.Invocations = b.Invocations[:1]
	b.Invocations[0].Usage = b.Invocations[0].Usage[1:]
	for _, model := range []string{strings.Repeat("x", 120), "provider/model/<&é>"} {
		b.Invocations[0].Usage[0].Model = model
		b.ReportedAt = "2026-11-17T08:00:00.123456+08:30"
		b.Invocations[0].ObservedAt = "2026-11-17T07:59:00.1Z"
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeIssueBudgetUsage(raw); err != nil {
			t.Fatal(err)
		}
	}
	b = usageBatch(t)
	inv := b.Invocations[0]
	b.Invocations = nil
	for i := 0; i < 33; i++ {
		copy := inv
		copy.InvocationID = fmt.Sprintf("%08x-1111-4111-8111-111111111111", i)
		b.Invocations = append(b.Invocations, copy)
	}
	raw, _ := json.Marshal(b)
	if _, err := DecodeIssueBudgetUsage(raw); err == nil {
		t.Fatal("accepted 33 invocations")
	}
	b.Invocations = b.Invocations[:32]
	raw, _ = json.Marshal(b)
	if _, err := DecodeIssueBudgetUsage(raw); err != nil {
		t.Fatal(err)
	}
	b.Invocations = b.Invocations[:1]
	b.Invocations[0].Usage = nil
	for i := 0; i < 257; i++ {
		b.Invocations[0].Usage = append(b.Invocations[0].Usage, issueBudgetUsageItem{Model: fmt.Sprint(i), Tokens: budget.TokenCounts{Input: 1}})
	}
	raw, _ = json.Marshal(b)
	if _, err := DecodeIssueBudgetUsage(raw); err == nil {
		t.Fatal("accepted 257 items")
	}
	b.Invocations[0].Usage = b.Invocations[0].Usage[:256]
	raw, _ = json.Marshal(b)
	if _, err := DecodeIssueBudgetUsage(raw); err != nil {
		t.Fatal(err)
	}
	raw = append(raw, bytes.Repeat([]byte{' '}, IssueBudgetUsageMaxBytes-len(raw))...)
	if _, err := DecodeIssueBudgetUsage(raw); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeIssueBudgetUsage(append(raw, ' ')); !errors.Is(err, ErrIssueBudgetUsageTooLarge) {
		t.Fatalf("size overflow = %v", err)
	}
}

func TestIssueBudgetUsageIntentGolden(t *testing.T) {
	attempt := db.IssueBudgetAttempt{ID: util.MustParseUUID(usageAttemptID), WorkspaceID: util.MustParseUUID("11111111-1111-4111-8111-111111111111")}
	// Independently calculated using Python hashlib over the specified five-key JSON.
	if got := issueBudgetUsageIntent(attempt, strings.Repeat("a", 64)); got != "45030cf08af04f7ac9156836566909b2339e59b045c404d9192fb707755a2bb7" {
		t.Fatalf("intent = %s", got)
	}
}

func TestIssueBudgetUsageDecodeEveryRequiredFieldAndObjectKey(t *testing.T) {
	raw := usageFixture(t, "budget_usage_v1_batch.json")
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	invocation := value["invocations"].([]any)[0].(map[string]any)
	item := invocation["usage"].([]any)[0].(map[string]any)
	tokens := item["tokens"].(map[string]any)
	for _, level := range []struct {
		path     string
		object   map[string]any
		optional string
	}{
		{"", value, "adapter_version"}, {"invocations[0].", invocation, ""}, {"invocations[0].usage[0].", item, "cost_ticks"}, {"invocations[0].usage[0].tokens.", tokens, ""},
	} {
		keys := make([]string, 0, len(level.object))
		for key := range level.object {
			keys = append(keys, key)
		}
		for _, key := range keys {
			original := level.object[key]
			for _, kind := range []string{"missing", "null"} {
				if kind == "missing" && key == level.optional {
					continue
				}
				t.Run(level.path+key+"/"+kind, func(t *testing.T) {
					if kind == "missing" {
						delete(level.object, key)
					} else {
						level.object[key] = nil
					}
					mutated, err := json.Marshal(value)
					level.object[key] = original
					if err != nil {
						t.Fatal(err)
					}
					_, err = DecodeIssueBudgetUsage(mutated)
					if err == nil || !strings.Contains(err.Error(), level.path+key) {
						t.Fatalf("got %v", err)
					}
				})
			}
		}
		// A case variant or duplicate must never be accepted by the second decode.
		for _, key := range []string{"unexpected", "Input", "Model", "Resumed", "Protocol_Version"} {
			level.object[key] = true
			mutated, _ := json.Marshal(value)
			delete(level.object, key)
			if _, err := DecodeIssueBudgetUsage(mutated); err == nil {
				t.Fatalf("accepted unknown key %s%s", level.path, key)
			}
		}
	}
}

func TestIssueBudgetUsageDecodeStringAndDecimalLimits(t *testing.T) {
	original := string(usageFixture(t, "budget_usage_v1_batch.json"))
	for _, tc := range []struct{ old, value, path string }{
		{`"adapter":"claude"`, `"adapter":"` + strings.Repeat("a", 63) + `"`, "adapter"},
		{`"normalizer_version":"1"`, `"normalizer_version":"` + strings.Repeat("x", 64) + `"`, "normalizer_version"},
		{`"adapter_version":"2.1.34 (Claude Code)"`, `"adapter_version":"` + strings.Repeat("v", 120) + `"`, "adapter_version"},
	} {
		mutated := strings.Replace(original, tc.old, tc.value, 1)
		if _, err := DecodeIssueBudgetUsage([]byte(mutated)); err != nil {
			t.Fatal(err)
		}
		over := strings.TrimSuffix(tc.value, `"`) + `x"`
		if _, err := DecodeIssueBudgetUsage([]byte(strings.Replace(original, tc.old, over, 1))); err == nil || !strings.Contains(err.Error(), tc.path) {
			t.Fatalf("over limit = %v", err)
		}
	}
	for _, decimal := range []string{"-1", "+1", "1.0", "1e1", " 1", "01", "", "9223372036854775808"} {
		for _, field := range []string{"input", "output", "cache_read", "cache_write", "cost_ticks"} {
			old := `"` + field + `":"0"`
			if field == "cost_ticks" {
				old = `"cost_ticks":"81000000"`
			}
			if _, err := DecodeIssueBudgetUsage([]byte(strings.Replace(original, old, `"`+field+`":"`+decimal+`"`, 1))); err == nil || !strings.Contains(err.Error(), field) {
				t.Fatalf("accepted %s=%q: %v", field, decimal, err)
			}
		}
	}
}
