package iteration

import (
	"reflect"
	"testing"
	"time"
)

func scopeIssue(id, status string) ScopeIssue { return ScopeIssue{IssueID: id, StatusCategory: status} }
func change(seq int64, id, status string) ScopeChange {
	c := ScopeChange{Sequence: seq, IssueID: id}
	if status != "" {
		v := scopeIssue(id, status)
		c.After = &v
	}
	return c
}

func TestStatisticsCanonicalExample(t *testing.T) {
	var original []ScopeIssue
	for _, id := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		original = append(original, scopeIssue(id, "todo"))
	}
	changes := []ScopeChange{change(1, "A", "done"), change(2, "B", "done"), change(3, "C", "done"), change(4, "I", "todo"), change(5, "J", "todo"), change(6, "E", "cancelled"), change(7, "F", ""), change(8, "G", "done"), change(9, "I", "done")}
	s, err := CalculateStatistics(original, changes)
	if err != nil {
		t.Fatal(err)
	}
	if s.Original != 8 || s.Current != 9 || s.Cancelled != 1 || s.Effective != 8 || s.Completed != 5 || s.OriginalCompleted != 4 || s.Remaining != 3 || s.AddedUnique != 2 || s.RemovedEvents != 1 || s.CancelEvents != 1 || s.NetEffectiveChange != 0 {
		t.Fatalf("wrong A-J statistics: %+v", s)
	}
	if s.EffectiveRatio == nil || *s.EffectiveRatio != .625 || s.OriginalRatio == nil || *s.OriginalRatio != .5 {
		t.Fatalf("wrong ratios: %+v", s)
	}
	// F's later external completion is not part of this iteration's event input.
	if s.OriginalCompleted != 4 {
		t.Fatal("left issue contributed completion")
	}
}

func TestStatisticsMembershipAndParticipation(t *testing.T) {
	original := []ScopeIssue{scopeIssue("A", "done"), scopeIssue("A", "done"), scopeIssue("B", "blocked")}
	changes := []ScopeChange{change(1, "I", "blocked"), change(2, "I", ""), change(3, "I", "todo"), change(4, "A", ""), change(5, "A", "todo"), change(6, "B", "blocked"), change(7, "I", "in_progress"), change(8, "I", "cancelled"), change(9, "I", "todo")}
	changes[5].ExecutionStarted = true
	s, err := CalculateStatistics(original, changes)
	if err != nil {
		t.Fatal(err)
	}
	if s.Original != 2 || s.AddedUnique != 1 || s.ReentryEvents != 2 || s.RemovedEvents != 2 || s.Started != 2 || s.Completed != 0 || s.CancelEvents != 1 || s.ReopenEvents != 1 {
		t.Fatalf("wrong participation: %+v", s)
	}
	if s.Current != s.Original+s.AddedUnique+s.ReentryEvents-s.RemovedEvents {
		t.Fatal("scope identity failed")
	}
	changes = append(changes, change(10, "B", ""), change(11, "B", "blocked"), change(12, "I", ""), change(13, "I", "blocked"))
	s, err = CalculateStatistics(original, changes)
	if err != nil || s.Started != 0 {
		t.Fatalf("reentry must reset started: %+v, %v", s, err)
	}
}

func TestStatisticsEmptyCancelledNoOpsAndReopen(t *testing.T) {
	for _, original := range [][]ScopeIssue{nil, {scopeIssue("A", "cancelled")}} {
		s, err := CalculateStatistics(original, nil)
		if err != nil || s.EffectiveRatio != nil || s.NetEffectiveChangeRatio != nil {
			t.Fatalf("empty denominator: %+v %v", s, err)
		}
		if len(original) == 0 && s.OriginalRatio != nil {
			t.Fatal("empty original ratio is not null")
		}
	}
	s, err := CalculateStatistics(nil, []ScopeChange{change(1, "A", "todo"), change(2, "A", "todo"), change(3, "A", "done"), change(4, "A", "todo"), change(5, "A", ""), change(6, "A", ""), change(7, "A", "done")})
	if err != nil || s.AddedUnique != 1 || s.RemovedEvents != 1 || s.ReentryEvents != 1 || s.ReopenEvents != 1 || s.Completed != 1 {
		t.Fatalf("no-op and reopen: %+v %v", s, err)
	}
}

func TestStatisticsRejectUnknownFacts(t *testing.T) {
	for _, cs := range [][]ScopeChange{{change(1, "A", "mystery")}, {change(1, "A", "todo"), change(1, "B", "todo")}} {
		if _, err := CalculateStatistics(nil, cs); err == nil {
			t.Fatal("invalid facts accepted")
		}
	}
}

func TestChartDayBoundaryAndClockRollback(t *testing.T) {
	start := time.Date(2026, 10, 5, 23, 0, 0, 0, time.UTC)
	changes := []ScopeChange{change(2, "A", "todo"), change(1, "A", "done")}
	changes[0].OccurredAt = start.Add(30 * time.Minute) // Clock moved back after sequence 1.
	changes[1].OccurredAt = start.Add(time.Hour)        // Exactly next day: excluded from October 5.
	chart, err := BuildChart([]ScopeIssue{scopeIssue("A", "todo")}, changes, start, start.Add(49*time.Hour), "UTC")
	if err != nil {
		t.Fatal(err)
	}
	want := []ScopeChartPoint{{Date: "2026-10-05", Effective: 1, Completed: 0, Original: 1}, {Date: "2026-10-06", Effective: 1, Completed: 0, Original: 1}, {Date: "2026-10-07", Effective: 1, Completed: 0, Original: 1}, {Date: "2026-10-08", Effective: 1, Completed: 0, Original: 1}}
	if !reflect.DeepEqual(chart, want) {
		t.Fatalf("chart: %+v", chart)
	}
	// A single change at the boundary belongs to the next date.
	chart, err = BuildChart([]ScopeIssue{scopeIssue("A", "todo")}, changes[1:], start, start.Add(time.Hour), "UTC")
	if err != nil || len(chart) != 2 || chart[0].Completed != 0 || chart[1].Completed != 1 {
		t.Fatalf("midnight chart: %+v %v", chart, err)
	}
}

func TestChartSkipsNonexistentDateAndStopsAtCutoff(t *testing.T) {
	start := time.Date(2011, 12, 29, 10, 0, 0, 0, time.UTC)
	end := time.Date(2011, 12, 31, 9, 0, 0, 0, time.UTC)
	first := change(1, "A", "done")
	first.OccurredAt = time.Date(2011, 12, 30, 10, 0, 0, 0, time.UTC)
	future := change(2, "A", "")
	future.OccurredAt = end.Add(2 * time.Hour)
	chart, err := BuildChart([]ScopeIssue{scopeIssue("A", "todo")}, []ScopeChange{first, future}, start, end, "Pacific/Apia")
	if err != nil {
		t.Fatal(err)
	}
	want := []ScopeChartPoint{{Date: "2011-12-29", Effective: 1, Completed: 0, Original: 1}, {Date: "2011-12-31", Effective: 1, Completed: 1, Original: 1}}
	if !reflect.DeepEqual(chart, want) {
		t.Fatalf("skipped calendar day/cutoff: %+v", chart)
	}
}

func TestStatisticsInputsRemainImmutable(t *testing.T) {
	original := []ScopeIssue{scopeIssue("A", "in_progress")}
	late := change(2, "A", "done")
	late.OccurredAt = time.Date(2026, 10, 6, 1, 0, 0, 0, time.UTC)
	early := change(1, "A", "todo")
	early.OccurredAt = time.Date(2026, 10, 6, 2, 0, 0, 0, time.UTC)
	changes := []ScopeChange{late, early}
	originalBefore := append([]ScopeIssue(nil), original...)
	changesBefore := append([]ScopeChange(nil), changes...)
	if _, err := CalculateStatistics(original, changes); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildChart(original, changes, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), early.OccurredAt, "UTC"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, originalBefore) || !reflect.DeepEqual(changes, changesBefore) {
		t.Fatal("statistics mutated source facts")
	}
	if late.After.HasStarted || early.After.HasStarted {
		t.Fatal("statistics changed event participation evidence")
	}
}

func TestStatisticsDeletionPreservesOriginalAndNullRatiosJSON(t *testing.T) {
	s, err := CalculateStatistics([]ScopeIssue{scopeIssue("A", "done")}, []ScopeChange{change(1, "A", "")})
	if err != nil || s.Original != 1 || s.OriginalCompleted != 0 || s.EffectiveRatio != nil || s.OriginalRatio == nil || *s.OriginalRatio != 0 {
		t.Fatalf("deleted original: %+v %v", s, err)
	}
}

func TestStatisticsReopenRequiresReturnToNonterminal(t *testing.T) {
	// Terminal-to-terminal transitions change cancellation/completion facts;
	// reopening means returning from done/cancelled to active work.
	s, err := CalculateStatistics([]ScopeIssue{scopeIssue("A", "done")}, []ScopeChange{change(1, "A", "cancelled"), change(2, "A", "done"), change(3, "A", "blocked"), change(4, "A", "cancelled"), change(5, "A", "todo")})
	if err != nil || s.CancelEvents != 2 || s.ReopenEvents != 2 || s.Started != 1 {
		t.Fatalf("terminal/reopen facts: %+v %v", s, err)
	}
}
