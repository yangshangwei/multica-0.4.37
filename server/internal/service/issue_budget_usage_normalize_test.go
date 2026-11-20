package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
)

const usageAttemptID = "22222222-2222-4222-8222-222222222222"

func usagePrice(string) (metrics.ModelPrice, bool) {
	return metrics.ModelPrice{Provider: "test", Model: "test-model", InputPerM: 0.1, OutputPerM: 15, CacheReadPerM: 0.075, CacheWritePerM: 1.25}, true
}
func usageBatch(t *testing.T) issueBudgetUsageBatch {
	t.Helper()
	v, err := DecodeIssueBudgetUsage(usageFixture(t, "budget_usage_v1_batch.json"))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func usageLedger(t *testing.T) *budget.Ledger {
	t.Helper()
	l, _, err := budget.NewLedger().With(budget.Entry{ID: "reserve/" + usageAttemptID, Kind: budget.KindReserve, ReservationID: usageAttemptID, ReservedTicks: 100})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestIssueBudgetUsageEvidence(t *testing.T) {
	table := productionIssueBudgetUsageEvidence()
	if !reflect.DeepEqual(table, IssueBudgetUsageEvidenceTable{{"claude", "1"}: {FreshOwnUsage: true}}) {
		t.Fatalf("production evidence changed: %+v", table)
	}
	for _, key := range []IssueBudgetUsageEvidenceKey{{"claude", "2"}, {"codex", "1"}, {"custom-claude", "1"}} {
		if v, ok := table[key]; ok || v != (IssueBudgetUsageEvidence{}) {
			t.Fatalf("unexpected proof for %v", key)
		}
	}
}

func TestIssueBudgetUsagePriceSnapshot(t *testing.T) {
	price, _ := usagePrice("")
	snapshot, err := issueBudgetUsagePriceSnapshot(price, "native/<&é>", "claude/unknown", "1")
	if err != nil {
		t.Fatal(err)
	}
	// Independent Python hashlib vector over the hand-written canonical rates.
	if snapshot.PriceVersion != "metrics:efec33c3742ffc3e" {
		t.Fatalf("version = %s", snapshot.PriceVersion)
	}
	if snapshot.Input != (budget.Rate{Ticks: 10_000_000_000, PerTokens: 10_000_000}) || snapshot.CacheRead != (budget.Rate{Ticks: 750_000_000_000, PerTokens: 1_000_000_000}) || snapshot.CacheWrite != (budget.Rate{Ticks: 1_250_000_000_000, PerTokens: 100_000_000}) {
		t.Fatalf("rates = %+v", snapshot)
	}
	if snapshot.Source != "" || snapshot.EffectiveFrom != "" || snapshot.EffectiveUntil != "" || snapshot.NativeModel != "native/<&é>" || snapshot.RoundingVersion != "half-up-v1" || snapshot.Currency != "USD" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(append([]byte("budget-price-snapshot-v1\n"), raw...))
	digest, err := snapshot.Digest()
	if err != nil || digest != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest = %s, %v", digest, err)
	}
	price.Provider = "other"
	price.Model = "other"
	other, err := issueBudgetUsagePriceSnapshot(price, "another", "claude/99", "1")
	if err != nil || other.PriceVersion != snapshot.PriceVersion {
		t.Fatal("labels affected price version")
	}
	price.InputPerM = 0.2
	other, err = issueBudgetUsagePriceSnapshot(price, "another", "claude/99", "1")
	if err != nil || other.PriceVersion == snapshot.PriceVersion {
		t.Fatal("rate change did not affect version")
	}
	if r, err := issueBudgetUsageRate(0); err != nil || r != (budget.Rate{PerTokens: 1_000_000}) {
		t.Fatalf("zero rate = %+v, %v", r, err)
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), -0.1, math.MaxFloat64, math.SmallestNonzeroFloat64} {
		if _, err := issueBudgetUsageRate(v); err == nil {
			t.Fatalf("accepted unrepresentable rate %v", v)
		}
	}
	if _, ok := metrics.PriceForModelAlias("claude-fable-5"); !ok {
		t.Fatal("known model not priced")
	}
	if _, ok := metrics.PriceForModelAlias("unknown-f3-budget-model"); ok {
		t.Fatal("unknown model priced")
	}
}

func TestIssueBudgetUsageNormalizeCostAndScope(t *testing.T) {
	batch := usageBatch(t)
	batch.Invocations = batch.Invocations[:1]
	batch.Invocations[0].Usage = batch.Invocations[0].Usage[:1]
	supplied := budget.Ticks(7)
	for _, proven := range []bool{false, true} {
		for _, tc := range []struct {
			name        string
			cost        *budget.Ticks
			reported    bool
			source      IssueBudgetUsagePriceSource
			basis       budget.CostBasis
			amount      budget.Ticks
			hasSnapshot bool
		}{
			{"reported", &supplied, true, usagePrice, budget.CostReported, 7, false},
			{"adapter estimate", &supplied, false, func(string) (metrics.ModelPrice, bool) {
				t.Fatal("must not re-estimate supplied cost")
				return metrics.ModelPrice{}, false
			}, budget.CostEstimated, 7, false},
			{"server estimate", nil, false, usagePrice, budget.CostEstimated, 1000, true},
			{"no price", nil, false, nil, budget.CostAbsent, 0, false},
			{"zero estimate", nil, false, func(string) (metrics.ModelPrice, bool) {
				return metrics.ModelPrice{Provider: "test", Model: "free"}, true
			}, budget.CostEstimated, 0, true},
			{"bad rate", nil, false, func(string) (metrics.ModelPrice, bool) {
				return metrics.ModelPrice{Provider: "test", Model: "bad", InputPerM: math.NaN()}, true
			}, budget.CostAbsent, 0, false},
		} {
			t.Run(tc.name+map[bool]string{true: "/proven", false: "/unproven"}[proven], func(t *testing.T) {
				batch.Invocations[0].Usage[0].Tokens = budget.TokenCounts{Input: 1}
				batch.Invocations[0].Usage[0].CostTicks = tc.cost
				table := IssueBudgetUsageEvidenceTable{{"claude", "1"}: {FreshOwnUsage: proven, ReportedCost: tc.reported}}
				out := normalizeIssueBudgetUsage(batch, util.MustParseUUID(usageAttemptID), true, table, tc.source)
				if len(out.Entries) != 1 {
					t.Fatalf("entries = %d", len(out.Entries))
				}
				e := out.Entries[0]
				if e.Entry.Usage.Cost.Basis != tc.basis || e.Entry.Usage.Cost.Ticks != tc.amount || (e.PriceSnapshot != nil) != tc.hasSnapshot {
					t.Fatalf("normalized = %+v", e)
				}
				if _, _, err := budget.EncodeEntry(e.Entry); err != nil {
					t.Fatal(err)
				}
				next, receipts, err := usageLedger(t).With(e.Entry)
				if err != nil {
					t.Fatal(err)
				}
				if !proven {
					if receipts[0].Reason != budget.ReasonScopeAmbiguity || receipts[0].Disposition != budget.DispositionQuarantined || next.Projection().ActualKnown != 0 || next.Projection().Estimated != 0 {
						t.Fatalf("unproven counted: %+v", receipts)
					}
				} else {
					want := budget.CostStateUnknown
					if tc.basis == budget.CostReported {
						want = budget.CostStateActual
					}
					if tc.basis == budget.CostEstimated {
						want = budget.CostStateEstimated
					}
					if receipts[0].CostState != want {
						t.Fatalf("receipt = %+v", receipts[0])
					}
				}
			})
		}
	}
	batch.Invocations[0].Usage[0].CostTicks = nil
	batch.Invocations[0].Usage[0].Tokens.Input = budget.Count(math.MaxInt64)
	out := normalizeIssueBudgetUsage(batch, util.MustParseUUID(usageAttemptID), true, productionIssueBudgetUsageEvidence(), usagePrice)
	if out.Entries[0].Entry.Usage.Cost != (budget.Cost{}) || out.Entries[0].PriceSnapshot != nil {
		t.Fatal("overflow did not preserve absent cost")
	}
}

func TestIssueBudgetUsageProductionEvidenceConsequences(t *testing.T) {
	for _, tc := range []struct {
		adapter, version     string
		resumed, quarantined bool
	}{{"claude", "1", false, false}, {"claude", "1", true, true}, {"claude", "2", false, true}, {"custom-claude", "1", false, true}, {"codex", "1", false, true}} {
		t.Run(tc.adapter+tc.version+map[bool]string{true: "/resumed", false: "/fresh"}[tc.resumed], func(t *testing.T) {
			b := usageBatch(t)
			b.Adapter = tc.adapter
			b.NormalizerVersion = tc.version
			b.Invocations = b.Invocations[:1]
			b.Invocations[0].Resumed = tc.resumed
			rows := normalizeIssueBudgetUsage(b, util.MustParseUUID(usageAttemptID), true, productionIssueBudgetUsageEvidence(), usagePrice).Entries
			entries := []budget.Entry{}
			for _, row := range rows {
				entries = append(entries, row.Entry)
			}
			l, receipts, err := usageLedger(t).With(entries...)
			if err != nil {
				t.Fatal(err)
			}
			if l.Projection().ActualKnown != 0 {
				t.Fatal("production contributes actual cost")
			}
			if tc.quarantined {
				if l.Projection().Estimated != 0 || receipts[0].Reason != budget.ReasonScopeAmbiguity {
					t.Fatalf("unproven counted: %+v", receipts)
				}
			} else if l.Projection().Estimated == 0 {
				t.Fatal("fresh usage did not estimate")
			}
		})
	}
}

func TestIssueBudgetUsageNormalizeEnvelopeGapsAndFrozenPrice(t *testing.T) {
	batch := usageBatch(t)
	batch.AdapterVersion = ""
	batch.Invocations = append(batch.Invocations, issueBudgetUsageInvocation{InvocationID: "33333333-3333-4333-8333-333333333333", Ended: "returned", ObservedAt: batch.ReportedAt, Usage: []issueBudgetUsageItem{}})
	for _, hasReservation := range []bool{true, false} {
		out := normalizeIssueBudgetUsage(batch, util.MustParseUUID(usageAttemptID), hasReservation, productionIssueBudgetUsageEvidence(), usagePrice)
		if len(out.Entries) != 5 {
			t.Fatalf("entries = %d", len(out.Entries))
		}
		for i, row := range out.Entries {
			if row.AttemptID != util.MustParseUUID(usageAttemptID) || row.AdapterVersion != "claude/unknown" || (row.Entry.ReservationID != "") != hasReservation {
				t.Fatalf("row %d: %+v", i, row)
			}
			if _, _, err := budget.EncodeEntry(row.Entry); err != nil {
				t.Fatal(err)
			}
			if i >= 3 && (row.Entry.Kind != budget.KindGap || row.Completeness != "" || row.Entry.Usage != (budget.Usage{})) {
				t.Fatalf("gap %d: %+v", i, row)
			}
		}
		if out.Entries[2].Completeness != "partial" || out.Entries[0].Completeness != "final" {
			t.Fatal("completeness")
		}
		if !strings.HasPrefix(out.Entries[3].Entry.Detail, "outcome_unknown:") || !strings.HasPrefix(out.Entries[4].Entry.Detail, "no_usage:") {
			t.Fatal("gap details")
		}
	}
	old := normalizeIssueBudgetUsage(batch, util.MustParseUUID(usageAttemptID), true, productionIssueBudgetUsageEvidence(), usagePrice).Entries[1]
	frozen, _, err := budget.EncodeEntry(old.Entry)
	if err != nil {
		t.Fatal(err)
	}
	nextPrice := func(model string) (metrics.ModelPrice, bool) {
		p, ok := usagePrice(model)
		p.InputPerM = 2
		return p, ok
	}
	newer := normalizeIssueBudgetUsage(batch, util.MustParseUUID(usageAttemptID), true, productionIssueBudgetUsageEvidence(), nextPrice).Entries[1]
	decoded, err := budget.DecodeEntry(budget.EntryFormVersion, frozen)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != old.Entry || newer.Entry.Usage.Cost.PriceVersion == old.Entry.Usage.Cost.PriceVersion {
		t.Fatal("frozen price changed or new price reused version")
	}
	l, _, err := usageLedger(t).With(decoded)
	if err != nil || l.Projection().Estimated != old.Entry.Usage.Cost.Ticks {
		t.Fatalf("replay: %v", err)
	}
}

func TestIssueBudgetUsageLargestStoredEntry(t *testing.T) {
	b := usageBatch(t)
	b.Invocations = b.Invocations[:1]
	b.Invocations[0].Usage = b.Invocations[0].Usage[1:]
	item := &b.Invocations[0].Usage[0]
	item.Model = strings.Repeat("&", 120)
	item.Tokens = budget.TokenCounts{Input: math.MaxInt64, Output: math.MaxInt64, CacheRead: math.MaxInt64, CacheWrite: math.MaxInt64}
	maxPrice := func(string) (metrics.ModelPrice, bool) {
		// Exactly 1/4 tick per token in each dimension: four MaxInt64 counts
		// sum to exactly MaxInt64 ticks, without estimate overflow.
		return metrics.ModelPrice{Provider: "test", Model: "max", InputPerM: 0.000025, OutputPerM: 0.000025, CacheReadPerM: 0.000025, CacheWritePerM: 0.000025}, true
	}
	row := normalizeIssueBudgetUsage(b, util.MustParseUUID(usageAttemptID), true, productionIssueBudgetUsageEvidence(), maxPrice).Entries[0]
	if row.Entry.Usage.Cost.Ticks != math.MaxInt64 {
		t.Fatal("largest estimate did not reach the tick bound")
	}
	raw, _, err := budget.EncodeEntry(row.Entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.Entry.ID) != 200 || len(raw) > issueBudgetLedgerEntryMaxBytes || len(row.Entry.Usage.Cost.PriceVersion) != 24 {
		t.Fatalf("ID/stored/version length = %d/%d/%d", len(row.Entry.ID), len(raw), len(row.Entry.Usage.Cost.PriceVersion))
	}
}
