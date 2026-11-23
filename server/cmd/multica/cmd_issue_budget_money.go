package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/multica-ai/multica/server/internal/budget"
)

// budgetUSDDecimals is the number of decimals of a dollar amount that one tick
// can hold: one tick is 1e-10 USD (budget.TicksPerUSD). A test fails if the
// two ever disagree.
const budgetUSDDecimals = 10

// The two wordings of an amount that is null. Neither is ever replaced by 0.
const (
	// budgetNotSet is for something a manager sets: the limit and the
	// declared reserve.
	budgetNotSet = "not set"
	// budgetUnknown is for everything that is measured.
	budgetUnknown = "unknown"
)

var (
	errBudgetAmountForm = errors.New("must be a decimal number of dollars such as 25 or 0.50 (digits and at most one point; " +
		"no sign, exponent, separator, space or currency symbol), or the word none")
	errBudgetAmountPrecision = errors.New("has more than 10 digits after the point; one tick is 0.0000000001 USD and an amount is never rounded")
	errBudgetAmountRange     = errors.New("is above the largest amount, 922337203.6854775807 USD")
)

// parseBudgetUSD reads an amount as it is typed on the command line: a decimal
// number of dollars, or the word none for "not set", which is returned as a nil
// amount and sent as JSON null.
//
// It checks the form only; whether an amount is allowed is the server's rule.
// The conversion moves digits: the fraction is padded to ten digits and the
// whole string is read as a number of ticks, so nothing is rounded and no
// floating point value exists at any step.
func parseBudgetUSD(input string) (*budget.Ticks, error) {
	if input == "none" {
		return nil, nil
	}
	whole, fraction, hasPoint := strings.Cut(input, ".")
	if !budgetASCIIDigits(whole) || (len(whole) > 1 && whole[0] == '0') || (hasPoint && !budgetASCIIDigits(fraction)) {
		return nil, errBudgetAmountForm
	}
	if len(fraction) > budgetUSDDecimals {
		return nil, errBudgetAmountPrecision
	}
	digits := strings.TrimLeft(whole+fraction+strings.Repeat("0", budgetUSDDecimals-len(fraction)), "0")
	if digits == "" {
		digits = "0"
	}
	// The digits are canonical by now, so the only refusal left is the range.
	value, err := budget.ParseDecimal(digits)
	if err != nil {
		return nil, errBudgetAmountRange
	}
	ticks := budget.Ticks(value)
	return &ticks, nil
}

// budgetASCIIDigits reports whether s is one or more of the digits 0 to 9.
func budgetASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// formatBudgetTicks prints an amount as dollars and as ticks. The fraction
// keeps every digit that is not a trailing zero and never fewer than two, so
// the dollars shown are exact: "25.00 USD (250000000000 ticks)".
func formatBudgetTicks(ticks budget.Ticks) string {
	value := int64(ticks)
	if value < 0 {
		// Amounts are never negative on the wire. This is not a number of
		// dollars and is not shown as one.
		return fmt.Sprintf("invalid amount (%d ticks)", value)
	}
	fraction := fmt.Sprintf("%0*d", budgetUSDDecimals, value%budget.TicksPerUSD)
	fraction = fraction[:2] + strings.TrimRight(fraction[2:], "0")
	return fmt.Sprintf("%d.%s %s (%d ticks)", value/budget.TicksPerUSD, fraction, budget.CurrencyUSD, value)
}

// formatBudgetAmount prints an amount that may be null. absent is the wording
// of the null: budgetNotSet or budgetUnknown.
func formatBudgetAmount(ticks *budget.Ticks, absent string) string {
	if ticks == nil {
		return absent
	}
	return formatBudgetTicks(*ticks)
}
