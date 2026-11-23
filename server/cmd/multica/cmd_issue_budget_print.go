package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/budget"
)

// What stands in a table where an answer gives nothing. None of these is a
// number: a missing amount must not be read as an amount of zero.
const (
	budgetNoValue = "-"
	budgetUnsized = "unsized"
)

// budgetText returns a text of the server in a form that is safe to print. The
// tables are read on a terminal, and a text that came over the wire may hold
// anything: it is printed as it came only when every character of it can be
// seen, and as a quoted string otherwise.
func budgetText(s string) string {
	if s == "" || !utf8.ValidString(s) {
		return strconv.QuoteToGraphic(s)
	}
	for _, r := range s {
		if !unicode.IsGraphic(r) {
			return strconv.QuoteToGraphic(s)
		}
	}
	return s
}

func budgetWhole[T ~int64](n T) string { return strconv.FormatInt(int64(n), 10) }

func budgetYesNo(yes bool) string {
	if yes {
		return "yes"
	}
	return "no"
}

// budgetOrNone is a text that may be null in an answer.
func budgetOrNone(text *string) string {
	if text == nil {
		return budgetNoValue
	}
	return budgetText(*text)
}

// budgetLines is a table of two columns: a label, and what is known under it.
type budgetLines struct {
	rows [][2]string
}

func (l *budgetLines) add(label, value string) {
	l.rows = append(l.rows, [2]string{label, value})
}

// print writes the plain lines and the table under them.
func (l *budgetLines) print(w io.Writer, plain ...string) error {
	for _, line := range plain {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return printBudgetTable(w, func(row func(cells ...string)) {
		for _, line := range l.rows {
			row(line[0], line[1])
		}
	})
}

// printBudgetTable prints rows in aligned columns. The last cell of a row is
// not padded, so no line ends in spaces.
func printBudgetTable(w io.Writer, rows func(row func(cells ...string))) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	var failed error
	rows(func(cells ...string) {
		if _, err := fmt.Fprintln(table, strings.Join(cells, "\t")); err != nil && failed == nil {
			failed = err
		}
	})
	if failed != nil {
		return failed
	}
	return table.Flush()
}

// budgetStateText and budgetModeText add what a state or a mode means for the
// amounts next to it. A value this command does not know is printed as it came.
func budgetStateText(state string) string {
	if state == budgetStateTombstoned {
		return state + "; the root issue was deleted"
	}
	return budgetText(state)
}

func budgetModeText(mode string) string {
	if mode == budgetModeOff {
		return mode + "; nothing is recorded while observation is off"
	}
	return budgetText(mode)
}

// budgetReserveText is the reserve of one execution. It is a number a manager
// declared, so the line says what it is not.
func budgetReserveText(reserve *budget.Ticks) string {
	if reserve == nil {
		return budgetNotSet
	}
	return formatBudgetTicks(*reserve) + " for each execution; a declared reservation, not an estimate and not a cap"
}

// result adds the result of an operation: the policy of the budget root as the
// operation left it, and for an enable what the activation found.
func (l *budgetLines) result(result *budgetOperationResult) {
	l.add("OPERATION", budgetText(result.OperationID.value))
	l.add("KIND", budgetText(result.OperationKind.value))
	l.add("BUDGET ROOT", budgetText(result.BudgetRootID.value))
	l.add("ROOT ISSUE", budgetText(result.RootIssueID.value))
	l.add("STATE", budgetStateText(result.AdmissionState.value))
	l.add("MODE", budgetModeText(result.Mode.value))
	l.add("POLICY REVISION", budgetWhole(result.PolicyRevision.value))
	l.add("POLICY EPOCH", budgetWhole(result.PolicyEpoch.value))
	l.add("LIMIT", formatBudgetAmount(result.LimitTicks.pointer(), budgetNotSet))
	l.add("ATTEMPT RESERVE", budgetReserveText(result.AttemptReserveTicks.pointer()))
	l.add("REASON", budgetText(result.Reason.value))
	if result.OperationKind.value != budgetKindEnable {
		return
	}
	l.add("ACTIVATED AT", budgetText(result.ActivatedAt.value))
	// 12.9 leaves these two counts out when they are zero, so a count that is
	// not there is zero. It is the one place where that holds.
	bound := budgetWhole(result.MemberCount.value)
	if result.MemberCount.value > 0 {
		bound += ", the root issue included"
	}
	l.add("ISSUES BOUND", bound)
	inFlight := budgetWhole(result.InFlightGapCount.value)
	if result.InFlightGapCount.value > 0 {
		inFlight += "; already running when the root was enabled, so unknown from before activation and not a zero cost"
	}
	l.add("IN FLIGHT AT ACTIVATION", inFlight)
}

// printBudgetWritten prints the answer to a write that is committed. Its first
// line says whether this request wrote it or an earlier one with the same key.
func printBudgetWritten(w io.Writer, written *budgetWriteAnswer) error {
	first := "committed: the operation was written now"
	if written.Replayed.value {
		first = "committed: this request key had already committed this operation; nothing was written now"
	}
	var lines budgetLines
	lines.result(&written.Result.value)
	return lines.print(w, first)
}

func printBudgetOperationCommitted(w io.Writer, committed *budgetOperationCommitted) error {
	plain := []string{"committed: an operation is committed under this key"}
	if committed.Stale.value {
		plain = append(plain, "stale: the policy of this budget root has changed since this operation; what follows is the result of the operation, not the policy of today")
	}
	var lines budgetLines
	lines.result(&committed.Result.value)
	return lines.print(w, plain...)
}

// printBudgetOperationUnknown prints the other answer of the query by key. It
// is an answer and not an error, and it does not say that the write is lost.
func printBudgetOperationUnknown(w io.Writer) error {
	var lines budgetLines
	return lines.print(w,
		"unknown: no committed operation is visible under this key yet",
		"That is not a refusal and does not say that the write is lost.",
		"To go on, run the original command again exactly as it was: the same --request-key and the same flags.",
	)
}

// printBudgetShow prints a budget root line by line. binding is what the issue
// that was asked for has to do with the root, or nil when the root was asked
// for by its ID.
func printBudgetShow(w io.Writer, binding *budgetIssueBinding, shown *budgetShow) error {
	var lines budgetLines
	if binding != nil {
		lines.add("ISSUE", budgetText(binding.IssueID.value))
		lines.add("IS ROOT ISSUE", budgetYesNo(binding.IsRootIssue.value))
		lines.add("BIND SOURCE", budgetText(binding.BindSource.value))
		lines.add("BOUND AT", budgetText(binding.BoundAt.value))
	}
	lines.add("BUDGET ROOT", budgetText(shown.BudgetRootID.value))
	lines.add("ROOT ISSUE", budgetText(shown.RootIssueID.value))
	lines.add("STATE", budgetStateText(shown.AdmissionState.value))
	lines.add("MODE", budgetModeText(shown.Mode.value))
	lines.add("POLICY REVISION", budgetWhole(shown.PolicyRevision.value))
	lines.add("POLICY EPOCH", budgetWhole(shown.PolicyEpoch.value))
	lines.add("LEDGER REVISION", budgetWhole(shown.LedgerRevision.value))
	lines.add("ACTIVATED AT", budgetText(shown.ActivatedAt.value))
	settled := "not settled"
	if revision := shown.SettledAsOfRevision.pointer(); revision != nil {
		settled = budgetWhole(*revision)
	}
	lines.add("SETTLED AS OF REVISION", settled)
	lines.add("LIMIT", formatBudgetAmount(shown.LimitTicks.pointer(), budgetNotSet))
	lines.add("ATTEMPT RESERVE", budgetReserveText(shown.AttemptReserveTicks.pointer()))

	// A ledger that cannot be read gives no amount, also when the answer
	// carries one: what it holds is not a fact to decide on.
	readable := shown.LedgerState.value != budgetLedgerUnavailable
	ledger := budgetText(shown.LedgerState.value)
	if !readable {
		ledger += "; the ledger cannot be read, so every amount is unknown"
	}
	lines.add("LEDGER", ledger)
	var actual, estimated, reserved, hold, overrun *budget.Ticks
	if amounts := shown.Amounts.pointer(); amounts != nil && readable {
		actual, estimated = &amounts.ActualKnownTicks.value, &amounts.EstimatedTicks.value
		reserved, hold = &amounts.ReservedTicks.value, &amounts.UnknownHoldTicks.value
		overrun = &amounts.OverrunTicks.value
	}
	lines.add("ACTUAL KNOWN (A)", formatBudgetAmount(actual, budgetUnknown))
	lines.add("ESTIMATED (E)", formatBudgetAmount(estimated, budgetUnknown)+"; outside A and outside the limit")
	lines.add("RESERVED (R)", formatBudgetAmount(reserved, budgetUnknown))
	lines.add("UNKNOWN HOLD (H)", formatBudgetAmount(hold, budgetUnknown))
	lines.add("OVERRUN", formatBudgetAmount(overrun, budgetUnknown))

	// Each reason of an unknown has a line of its own; they are never one sum.
	for _, reason := range shown.Unknown.value.ByReason.value {
		lines.add("UNKNOWN", budgetText(reason.Reason.value)+": "+budgetWhole(reason.Count.value))
	}
	if len(shown.Unknown.value.ByReason.value) == 0 {
		lines.add("UNKNOWN", "none")
	}
	lines.add("UNKNOWN BEFORE ACTIVATION", budgetWhole(shown.Unknown.value.PreActivation.value))
	lines.add("TOTAL KNOWN", budgetYesNo(shown.TotalKnown.value))

	// The remaining amount is the server's. When it gives none, the line says
	// why as far as the answer tells, and never a number.
	remaining := shown.RemainingTicks.pointer()
	switch {
	case !readable:
		lines.add("REMAINING", budgetUnknown)
	case remaining != nil:
		lines.add("REMAINING", formatBudgetTicks(*remaining))
	case shown.LimitTicks.null:
		lines.add("REMAINING", budgetUnknown+"; no limit is set")
	case !shown.TotalKnown.value:
		lines.add("REMAINING", budgetUnknown+"; not everything is known")
	default:
		lines.add("REMAINING", budgetUnknown)
	}
	included := budgetUnknown
	if count := shown.Coverage.value.IncludedAttempts.pointer(); count != nil && readable {
		included = budgetWhole(*count)
	}
	lines.add("EXECUTIONS INCLUDED", included)
	for _, name := range shown.Coverage.value.NotApplicable.value {
		lines.add("NOT COUNTED", budgetText(name.value)+"; not counted, which is not a zero cost")
	}
	if len(shown.Coverage.value.NotApplicable.value) == 0 {
		lines.add("NOT COUNTED", "none")
	}
	return lines.print(w)
}

// printBudgetList prints one page of a list: the lines above the table, the
// table or that the page has no row, and the line of the next page.
func printBudgetList(w io.Writer, above []string, header []string, rows [][]string, empty, footer string) error {
	for _, line := range above {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintf(w, "%s\n%s\n", empty, footer)
		return err
	}
	err := printBudgetTable(w, func(row func(cells ...string)) {
		row(header...)
		for _, cells := range rows {
			row(cells...)
		}
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, footer)
	return err
}

func printBudgetObservations(w io.Writer, page *budgetObservationPage, footer string) error {
	header := []string{"ID", "CREATED AT", "KIND", "REASON", "COVERED", "ISSUE", "TASK", "CLAIM", "ATTEMPT", "DETAIL"}
	var rows [][]string
	for _, observation := range page.Observations.value {
		claim := budgetNoValue
		if generation := observation.ClaimGeneration.pointer(); generation != nil {
			claim = budgetWhole(*generation)
		}
		rows = append(rows, []string{
			budgetText(observation.ID.value),
			budgetText(observation.CreatedAt.value),
			budgetText(observation.Kind.value),
			budgetText(observation.Reason.value),
			budgetYesNo(observation.Covered.value),
			budgetOrNone(observation.IssueID.pointer()),
			budgetOrNone(observation.TaskID.pointer()),
			claim,
			budgetOrNone(observation.AttemptID.pointer()),
			budgetDetailText(observation.Detail.value),
		})
	}
	return printBudgetList(w, nil, header, rows, "no observations on this page", footer)
}

// budgetDetailText is the detail of an observation in one cell: the object as
// it came, without the space between its parts. Its keys are not interpreted.
func budgetDetailText(detail budgetJSONObject) string {
	var compact bytes.Buffer
	if err := json.Compact(&compact, detail); err != nil {
		return budgetText(string(detail))
	}
	return budgetText(compact.String())
}

func printBudgetAttempts(w io.Writer, page *budgetAttemptPage, footer string) error {
	ledger := page.LedgerState.value
	above := "ledger: " + budgetText(ledger)
	if ledger == budgetLedgerUnavailable {
		above += "; the ledger cannot be read, so what the reservations hold is unknown"
	}
	header := []string{"ATTEMPT", "CREATED AT", "STATE", "RESERVED", "RESERVATION", "ACTUAL KNOWN", "UNSPENT", "OVERRUN", "PROTOCOL", "CLAIM", "TASK", "ISSUE", "CONTEXT PARENT", "AGENT", "RUNTIME"}
	var rows [][]string
	for _, attempt := range page.Attempts.value {
		// An execution claimed by a daemon from before the budget protocol
		// has no version; that is a fact about the daemon, not a version 0.
		protocol := "old daemon"
		if version := attempt.ProtocolVersion.pointer(); version != nil {
			protocol = budgetWhole(*version)
		}
		cells := []string{
			budgetText(attempt.AttemptID.value),
			budgetText(attempt.CreatedAt.value),
			budgetText(attempt.State.value),
			formatBudgetAmount(attempt.ReservedTicks.pointer(), budgetUnsized),
		}
		cells = append(cells, budgetReservationCells(ledger, attempt.Reservation.pointer())...)
		rows = append(rows, append(cells,
			protocol,
			budgetWhole(attempt.ClaimGeneration.value),
			budgetText(attempt.TaskID.value),
			budgetOrNone(attempt.IssueID.pointer()),
			budgetOrNone(attempt.ContextParentIssueID.pointer()),
			budgetText(attempt.AgentID.value),
			budgetText(attempt.RuntimeID.value),
		))
	}
	return printBudgetList(w, []string{above}, header, rows, "no attempts on this page", footer)
}

// budgetReservationCells are the four cells of the reservation of an execution:
// its status, what is known to be spent, what is unspent and what ran over.
// "No reservation" is said only when a readable ledger says so. A reservation
// without a size holds 0 where a size would be, and that 0 is not an amount.
func budgetReservationCells(ledger string, reservation *budgetReservation) []string {
	switch {
	case ledger == budgetLedgerUnavailable, reservation == nil && ledger != budgetLedgerAvailable:
		return []string{budgetUnknown, budgetUnknown, budgetUnknown, budgetUnknown}
	case reservation == nil:
		return []string{"no reservation", budgetNoValue, budgetNoValue, budgetNoValue}
	}
	unspent, overrun := formatBudgetTicks(reservation.UnspentTicks.value), formatBudgetTicks(reservation.OverrunTicks.value)
	if reservation.Unsized.value {
		unspent, overrun = budgetUnsized, budgetUnsized
	}
	return []string{budgetText(reservation.Status.value), formatBudgetTicks(reservation.ActualKnownTicks.value), unspent, overrun}
}
