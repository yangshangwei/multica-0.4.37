package main

import (
	"math"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/budget"
)

// The command line takes dollars and the API takes ticks. The conversion moves
// digits; these tables are its whole contract (design 12.6).
func TestIssueBudgetParseUSD(t *testing.T) {
	accepted := []struct {
		input string
		ticks int64
	}{
		{"0", 0},
		{"0.0", 0},
		{"0.0000000000", 0},
		{"0.0000000001", 1},
		{"0.5", 5_000_000_000},
		{"0.50", 5_000_000_000},
		{"1", 10_000_000_000},
		{"7.35", 73_500_000_000},
		{"25", 250_000_000_000},
		{"25.00", 250_000_000_000},
		{"25.0000000000", 250_000_000_000},
		{"10.0000000009", 100_000_000_009},
		{"922337203", 9_223_372_030_000_000_000},
		{"922337203.6854775807", math.MaxInt64},
	}
	for _, tc := range accepted {
		got, err := parseBudgetUSD(tc.input)
		if err != nil || got == nil {
			t.Errorf("parseBudgetUSD(%q) = %v, %v; want %d ticks", tc.input, got, err, tc.ticks)
			continue
		}
		if int64(*got) != tc.ticks {
			t.Errorf("parseBudgetUSD(%q) = %d ticks, want %d", tc.input, int64(*got), tc.ticks)
		}
	}

	// The word none is "not set": no amount, and no error.
	if got, err := parseBudgetUSD("none"); err != nil || got != nil {
		t.Errorf("parseBudgetUSD(none) = %v, %v; want no amount and no error", got, err)
	}

	refused := map[string]error{
		// Above the largest int64 number of ticks.
		"922337203.6854775808":   errBudgetAmountRange,
		"922337204":              errBudgetAmountRange,
		"9223372036854775808":    errBudgetAmountRange,
		strings.Repeat("9", 400): errBudgetAmountRange,
		// More than ten digits after the point is an error, never a rounding,
		// even when the extra digits are zeros.
		"1.00000000001":  errBudgetAmountPrecision,
		"0.00000000000":  errBudgetAmountPrecision,
		"0.00000000009":  errBudgetAmountPrecision,
		"25.12345678901": errBudgetAmountPrecision,
		// Not a number of dollars in the accepted form.
		"-1": errBudgetAmountForm, "+1": errBudgetAmountForm, "-0": errBudgetAmountForm,
		"1e3": errBudgetAmountForm, "1E3": errBudgetAmountForm, "1e-3": errBudgetAmountForm,
		"1,000": errBudgetAmountForm, "1_000": errBudgetAmountForm, "1 000": errBudgetAmountForm,
		".5": errBudgetAmountForm, "5.": errBudgetAmountForm, ".": errBudgetAmountForm, "1.2.3": errBudgetAmountForm,
		"01": errBudgetAmountForm, "00": errBudgetAmountForm, "00.5": errBudgetAmountForm,
		"": errBudgetAmountForm, " ": errBudgetAmountForm, " 1": errBudgetAmountForm, "1 ": errBudgetAmountForm,
		"1\n": errBudgetAmountForm, "\t1": errBudgetAmountForm,
		"$1": errBudgetAmountForm, "1$": errBudgetAmountForm, "1USD": errBudgetAmountForm, "1 USD": errBudgetAmountForm,
		"None": errBudgetAmountForm, "NONE": errBudgetAmountForm, "none ": errBudgetAmountForm, " none": errBudgetAmountForm,
		"null": errBudgetAmountForm, "unset": errBudgetAmountForm, "unlimited": errBudgetAmountForm,
		"0x10": errBudgetAmountForm, "0b1": errBudgetAmountForm, "1f": errBudgetAmountForm,
		"NaN": errBudgetAmountForm, "Inf": errBudgetAmountForm, "+Inf": errBudgetAmountForm, "infinity": errBudgetAmountForm,
		"1.-5": errBudgetAmountForm, "1.+5": errBudgetAmountForm, "1.5e1": errBudgetAmountForm,
		// Digits that are not ASCII digits.
		"\uff11": errBudgetAmountForm, "1.\u0665": errBudgetAmountForm,
	}
	for input, want := range refused {
		got, err := parseBudgetUSD(input)
		if err != want || got != nil {
			t.Errorf("parseBudgetUSD(%q) = %v, %v; want the refusal %q", input, got, err, want)
		}
	}
}

// The scale the command line assumes is the scale of the metering package.
func TestIssueBudgetUSDDecimalsMatchTheTickScale(t *testing.T) {
	scale := int64(1)
	for i := 0; i < budgetUSDDecimals; i++ {
		scale *= 10
	}
	if scale != budget.TicksPerUSD {
		t.Fatalf("10^%d = %d, but one dollar is %d ticks", budgetUSDDecimals, scale, budget.TicksPerUSD)
	}
}

func TestIssueBudgetFormatTicks(t *testing.T) {
	for _, tc := range []struct {
		ticks int64
		want  string
	}{
		{0, "0.00 USD (0 ticks)"},
		{1, "0.0000000001 USD (1 ticks)"},
		{250_000_000, "0.025 USD (250000000 ticks)"},
		{1_200_000_000, "0.12 USD (1200000000 ticks)"},
		{5_000_000_000, "0.50 USD (5000000000 ticks)"},
		{10_000_000_000, "1.00 USD (10000000000 ticks)"},
		{10_000_000_001, "1.0000000001 USD (10000000001 ticks)"},
		{73_500_000_000, "7.35 USD (73500000000 ticks)"},
		{250_000_000_000, "25.00 USD (250000000000 ticks)"},
		{math.MaxInt64, "922337203.6854775807 USD (9223372036854775807 ticks)"},
	} {
		if got := formatBudgetTicks(budget.Ticks(tc.ticks)); got != tc.want {
			t.Errorf("formatBudgetTicks(%d) = %q, want %q", tc.ticks, got, tc.want)
		}
	}

	// A null amount has two wordings and neither of them is a number.
	if got := formatBudgetAmount(nil, budgetNotSet); got != "not set" {
		t.Errorf("a null managed amount prints %q, want %q", got, "not set")
	}
	if got := formatBudgetAmount(nil, budgetUnknown); got != "unknown" {
		t.Errorf("a null measured amount prints %q, want %q", got, "unknown")
	}
	zero := budget.Ticks(0)
	for _, absent := range []string{budgetNotSet, budgetUnknown} {
		if got := formatBudgetAmount(&zero, absent); got != "0.00 USD (0 ticks)" {
			t.Errorf("an amount of zero prints %q with the wording %q; zero is a value, not an absence", got, absent)
		}
	}

	// Amounts are never negative on the wire. If one ever reaches the printer
	// it must not be dressed up as dollars.
	if got := formatBudgetTicks(budget.Ticks(-1)); strings.Contains(got, "USD") || !strings.Contains(got, "-1") {
		t.Errorf("a negative amount prints %q", got)
	}
}

// What the printer shows as dollars is what the parser reads back.
func TestIssueBudgetUSDTextRoundTrips(t *testing.T) {
	for _, ticks := range []int64{0, 1, 9, 10, 99, 9_999_999_999, 10_000_000_000, 10_000_000_001, 123_456_789_012, math.MaxInt64 - 1, math.MaxInt64} {
		text := formatBudgetTicks(budget.Ticks(ticks))
		dollars, _, found := strings.Cut(text, " USD (")
		if !found {
			t.Fatalf("formatBudgetTicks(%d) = %q has no dollar part", ticks, text)
		}
		got, err := parseBudgetUSD(dollars)
		if err != nil || got == nil || int64(*got) != ticks {
			t.Errorf("%d ticks print as %q, which reads back as %v, %v", ticks, dollars, got, err)
		}
	}
}
