package budget

import (
	"encoding/json"
	"errors"
	"testing"
)

func usdRate(t *testing.T, usdNumerator, usdDenominator, perTokens int64) Rate {
	t.Helper()
	r, err := USDRate(usdNumerator, usdDenominator, perTokens)
	if err != nil {
		t.Fatalf("USDRate(%d, %d, %d): %v", usdNumerator, usdDenominator, perTokens, err)
	}
	return r
}

// priceFixture: input 3 USD, output 15 USD, cache read 0.30 USD and cache
// write 3.75 USD per million tokens.
func priceFixture(t *testing.T) PriceSnapshot {
	t.Helper()
	return PriceSnapshot{
		Provider: "anthropic", Model: "claude-fable-5-2", NativeModel: "claude-fable-5-2-20261101",
		AdapterVersion: "claude-adapter-v1", NormalizerVersion: "normalizer-v1",
		PriceVersion: "2026-11-01", Source: "fixture", EffectiveFrom: "2026-11-01T00:00:00Z",
		Currency: "USD", RoundingVersion: RoundingHalfUpV1,
		Input:      usdRate(t, 3, 1, 1_000_000),
		Output:     usdRate(t, 15, 1, 1_000_000),
		CacheRead:  usdRate(t, 3, 10, 1_000_000),
		CacheWrite: usdRate(t, 15, 4, 1_000_000),
	}
}

const priceFixtureJSON = `{"provider":"anthropic","model":"claude-fable-5-2","native_model":"claude-fable-5-2-20261101","adapter_version":"claude-adapter-v1","normalizer_version":"normalizer-v1","price_version":"2026-11-01","source":"fixture","effective_from":"2026-11-01T00:00:00Z","effective_until":"","currency":"USD","rounding_version":"half-up-v1","input":{"ticks":"30000000000","per_tokens":"1000000"},"output":{"ticks":"150000000000","per_tokens":"1000000"},"cache_read":{"ticks":"30000000000","per_tokens":"10000000"},"cache_write":{"ticks":"150000000000","per_tokens":"4000000"}}`

// SHA-256 of "budget-price-snapshot-v1\n" + priceFixtureJSON, computed outside Go.
const priceFixtureDigest = "af470d20e36503942d9de270aa383e7912ef17f733752250007c61a4b76cb8b7"

func TestUSDRateIsAnIntegerFraction(t *testing.T) {
	if got := usdRate(t, 3, 1, 1_000_000); got != (Rate{Ticks: 30_000_000_000, PerTokens: 1_000_000}) {
		t.Fatalf("3 USD per million = %+v", got)
	}
	if got := usdRate(t, 1, 3, 1_000_000); got != (Rate{Ticks: 10_000_000_000, PerTokens: 3_000_000}) {
		t.Fatalf("1/3 USD per million = %+v", got)
	}
	for _, in := range [][3]int64{{-1, 1, 1}, {1, 0, 1}, {1, -1, 1}, {1, 1, 0}, {1, 1, -1}} {
		if _, err := USDRate(in[0], in[1], in[2]); !errors.Is(err, ErrInvalidPrice) {
			t.Errorf("USDRate%v error = %v, want ErrInvalidPrice", in, err)
		}
	}
	// 922337204 USD is 9223372040000000000 ticks, above the 64-bit range.
	if _, err := USDRate(922_337_204, 1, 1); !errors.Is(err, ErrOverflow) {
		t.Fatalf("overflowing numerator error = %v", err)
	}
	if _, err := USDRate(1, 3_037_000_500, 3_037_000_500); !errors.Is(err, ErrOverflow) {
		t.Fatalf("overflowing denominator error = %v", err)
	}
}

// B23: 1,000,001 input tokens at 3 USD per 1,000,000 tokens is exactly
// 3.000003 USD = 30,000,030,000 ticks.
func TestB23EstimateExactIntegerNeedsNoRounding(t *testing.T) {
	got, err := priceFixture(t).Estimate(TokenCounts{Input: 1_000_001}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ticks != 30_000_030_000 || got.ReserveTicks != 30_000_030_000 || !got.Exact {
		t.Fatalf("estimate = %+v", got)
	}
	if got.ExactNumerator != "30000030000" || got.ExactDenominator != "1" {
		t.Fatalf("exact value = %s/%s", got.ExactNumerator, got.ExactDenominator)
	}
}

// B23: one token at 1/3 USD per 1,000,000 tokens is 10000/3 = 3333.33... ticks.
// The reservation rounds up to 3334; the displayed estimate follows the frozen
// rounding version.
func TestB23EstimateFractionRoundsUpForReservation(t *testing.T) {
	s := priceFixture(t)
	s.Input = usdRate(t, 1, 3, 1_000_000)
	got, err := s.Estimate(TokenCounts{Input: 1}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ReserveTicks != 3334 || got.Ticks != 3333 || got.Exact {
		t.Fatalf("half-up estimate = %+v", got)
	}
	if got.ExactNumerator != "10000" || got.ExactDenominator != "3" || got.RoundingVersion != "half-up-v1" {
		t.Fatalf("exact value = %s/%s under %s", got.ExactNumerator, got.ExactDenominator, got.RoundingVersion)
	}

	// Replay: the same frozen snapshot, restored from its stored form, gives
	// the same estimate field for field.
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored PriceSnapshot
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	again, err := restored.Estimate(TokenCounts{Input: 1}, "")
	if err != nil || again != got {
		t.Fatalf("replayed estimate = %+v, %v; want %+v", again, err, got)
	}

	ceil := s
	ceil.RoundingVersion = RoundingCeilV1
	up, err := ceil.Estimate(TokenCounts{Input: 1}, "")
	if err != nil || up.Ticks != 3334 || up.ReserveTicks != 3334 || up.RoundingVersion != "ceil-v1" {
		t.Fatalf("ceil estimate = %+v, %v", up, err)
	}
}

func TestEstimateHalfUpTie(t *testing.T) {
	s := priceFixture(t)
	s.Input = Rate{Ticks: 1, PerTokens: 2}
	for _, tc := range []struct {
		tokens         Count
		ticks, reserve Ticks
		exact          bool
	}{
		{1, 1, 1, false}, // 1/2 rounds half up to 1
		{3, 2, 2, false}, // 3/2 rounds half up to 2
		{4, 2, 2, true},
		{0, 0, 0, true},
	} {
		got, err := s.Estimate(TokenCounts{Input: tc.tokens}, "")
		if err != nil || got.Ticks != tc.ticks || got.ReserveTicks != tc.reserve || got.Exact != tc.exact {
			t.Errorf("%d tokens = %+v, %v; want %d/%d exact=%v", tc.tokens, got, err, tc.ticks, tc.reserve, tc.exact)
		}
	}
}

// Dimensions are summed exactly and rounded once: 1/3 + 1/6 = 1/2 tick.
// Rounding each dimension first would give 0 (half up) or 2 (ceiling).
func TestEstimateSumsDimensionsBeforeRoundingOnce(t *testing.T) {
	s := priceFixture(t)
	s.Input = Rate{Ticks: 1, PerTokens: 3}
	s.Output = Rate{Ticks: 1, PerTokens: 6}
	got, err := s.Estimate(TokenCounts{Input: 1, Output: 1}, "")
	if err != nil || got.Ticks != 1 || got.ReserveTicks != 1 || got.ExactNumerator != "1" || got.ExactDenominator != "2" {
		t.Fatalf("estimate = %+v, %v", got, err)
	}
}

// 1000 input (0.003 USD) + 200 output (0.003 USD) + 5000 cache read
// (0.0015 USD) + 400 cache write (0.0015 USD) = 0.009 USD = 90,000,000 ticks.
func TestEstimateFourDimensions(t *testing.T) {
	got, err := priceFixture(t).Estimate(TokenCounts{Input: 1000, Output: 200, CacheRead: 5000, CacheWrite: 400}, "")
	if err != nil || got.Ticks != 90_000_000 || got.ReserveTicks != 90_000_000 || !got.Exact {
		t.Fatalf("estimate = %+v, %v", got, err)
	}
}

func TestEstimateOverflowIsAnErrorNotWraparound(t *testing.T) {
	s := priceFixture(t)
	s.Input = Rate{Ticks: 9_223_372_036_854_775_807, PerTokens: 1}
	if got, err := s.Estimate(TokenCounts{Input: 2}, ""); !errors.Is(err, ErrOverflow) || got != (Estimate{}) {
		t.Fatalf("estimate = %+v, %v; want ErrOverflow", got, err)
	}
	// The largest representable result is still accepted.
	if got, err := s.Estimate(TokenCounts{Input: 1}, ""); err != nil || got.Ticks != 9_223_372_036_854_775_807 {
		t.Fatalf("boundary estimate = %+v, %v", got, err)
	}
	// Half a tick above the bound cannot be rounded into range.
	s.Output = Rate{Ticks: 1, PerTokens: 2}
	if _, err := s.Estimate(TokenCounts{Input: 1, Output: 1}, ""); !errors.Is(err, ErrOverflow) {
		t.Fatalf("rounded overflow error = %v", err)
	}
}

func TestPriceSnapshotValidation(t *testing.T) {
	if err := priceFixture(t).Validate(); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		mutate func(*PriceSnapshot)
		want   error
	}{
		"non-USD currency":     {func(s *PriceSnapshot) { s.Currency = "EUR" }, ErrUnsupportedCurrency},
		"missing currency":     {func(s *PriceSnapshot) { s.Currency = "" }, ErrUnsupportedCurrency},
		"missing provider":     {func(s *PriceSnapshot) { s.Provider = "" }, ErrInvalidPrice},
		"missing model":        {func(s *PriceSnapshot) { s.Model = "" }, ErrInvalidPrice},
		"missing adapter":      {func(s *PriceSnapshot) { s.AdapterVersion = "" }, ErrInvalidPrice},
		"missing normalizer":   {func(s *PriceSnapshot) { s.NormalizerVersion = "" }, ErrInvalidPrice},
		"missing version":      {func(s *PriceSnapshot) { s.PriceVersion = "" }, ErrInvalidPrice},
		"unknown rounding":     {func(s *PriceSnapshot) { s.RoundingVersion = "bankers-v9" }, ErrInvalidPrice},
		"zero denominator":     {func(s *PriceSnapshot) { s.Output.PerTokens = 0 }, ErrInvalidPrice},
		"negative denominator": {func(s *PriceSnapshot) { s.CacheRead.PerTokens = -1 }, ErrInvalidPrice},
		"negative rate":        {func(s *PriceSnapshot) { s.CacheWrite.Ticks = -1 }, ErrNegativeAmount},
	} {
		s := priceFixture(t)
		tc.mutate(&s)
		if err := s.Validate(); !errors.Is(err, tc.want) {
			t.Errorf("%s: Validate = %v, want %v", name, err, tc.want)
		}
		if got, err := s.Estimate(TokenCounts{Input: 1}, ""); !errors.Is(err, tc.want) || got != (Estimate{}) {
			t.Errorf("%s: Estimate = %+v, %v, want %v", name, got, err, tc.want)
		}
	}
	if _, err := priceFixture(t).Estimate(TokenCounts{Output: -1}, ""); !errors.Is(err, ErrNegativeAmount) {
		t.Fatalf("negative tokens error = %v", err)
	}
}

func TestPriceSnapshotDigestBindsEveryField(t *testing.T) {
	s := priceFixture(t)
	raw, err := json.Marshal(s)
	if err != nil || string(raw) != priceFixtureJSON {
		t.Fatalf("stored form = %s, %v", raw, err)
	}
	got, err := s.Digest()
	if err != nil || got != priceFixtureDigest {
		t.Fatalf("digest = %q, %v; want %q", got, err, priceFixtureDigest)
	}
	for name, mutate := range map[string]func(*PriceSnapshot){
		"provider":    func(s *PriceSnapshot) { s.Provider = "openai" },
		"model":       func(s *PriceSnapshot) { s.Model = "other" },
		"native":      func(s *PriceSnapshot) { s.NativeModel = "other" },
		"adapter":     func(s *PriceSnapshot) { s.AdapterVersion = "claude-adapter-v2" },
		"normalizer":  func(s *PriceSnapshot) { s.NormalizerVersion = "normalizer-v2" },
		"version":     func(s *PriceSnapshot) { s.PriceVersion = "2026-12-01" },
		"source":      func(s *PriceSnapshot) { s.Source = "other" },
		"from":        func(s *PriceSnapshot) { s.EffectiveFrom = "2026-12-01T00:00:00Z" },
		"until":       func(s *PriceSnapshot) { s.EffectiveUntil = "2027-01-01T00:00:00Z" },
		"rounding":    func(s *PriceSnapshot) { s.RoundingVersion = RoundingCeilV1 },
		"input":       func(s *PriceSnapshot) { s.Input.Ticks++ },
		"output":      func(s *PriceSnapshot) { s.Output.PerTokens++ },
		"cache read":  func(s *PriceSnapshot) { s.CacheRead.Ticks++ },
		"cache write": func(s *PriceSnapshot) { s.CacheWrite.PerTokens++ },
	} {
		changed := priceFixture(t)
		mutate(&changed)
		if d, err := changed.Digest(); err != nil || d == priceFixtureDigest || len(d) != 64 {
			t.Errorf("%s: digest = %q, %v", name, d, err)
		}
	}
}

// B24 (pure half): a cheaper price version named by the caller never replaces
// the server's frozen snapshot, and a later price table does not change what
// the old snapshot computes.
func TestB24CallerClaimedPriceVersionNeverOverridesSnapshot(t *testing.T) {
	frozen := priceFixture(t)
	cheaper := priceFixture(t)
	cheaper.PriceVersion = "2026-12-01-cheaper"
	cheaper.Input = usdRate(t, 1, 1, 1_000_000)

	got, err := frozen.Estimate(TokenCounts{Input: 1_000_001}, "2026-12-01-cheaper")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ticks != 30_000_030_000 || got.PriceVersion != "2026-11-01" || got.PriceDigest != priceFixtureDigest || !got.ClaimedPriceVersionIgnored {
		t.Fatalf("estimate with a claimed version = %+v", got)
	}
	same, err := frozen.Estimate(TokenCounts{Input: 1_000_001}, "2026-11-01")
	if err != nil || same.Ticks != 30_000_030_000 || same.ClaimedPriceVersionIgnored {
		t.Fatalf("estimate with the matching version = %+v, %v", same, err)
	}
	// The newer table exists and is cheaper (1 USD per million = 10,000,010,000
	// ticks), and the frozen snapshot still replays its original value.
	if now, err := cheaper.Estimate(TokenCounts{Input: 1_000_001}, ""); err != nil || now.Ticks != 10_000_010_000 {
		t.Fatalf("newer table = %+v, %v", now, err)
	}
	if again, err := frozen.Estimate(TokenCounts{Input: 1_000_001}, ""); err != nil || again.Ticks != 30_000_030_000 || again.PriceDigest != priceFixtureDigest {
		t.Fatalf("frozen replay = %+v, %v", again, err)
	}
}
