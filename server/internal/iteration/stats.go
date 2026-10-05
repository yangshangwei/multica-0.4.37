package iteration

import (
	"fmt"
	"sort"
	"time"
)

// ScopeIssue contains already-resolved status facts, never display names.
// HasStarted represents execution evidence for the current participation.
type ScopeIssue struct {
	IssueID        string
	StatusCategory string
	HasStarted     bool
}

// ScopeChange is a post-start scope fact. After=nil releases membership;
// execution starts can be recorded even when the status category is unchanged.
// Callers must supply the complete event stream, excluding planned activity.
type ScopeChange struct {
	Sequence         int64
	OccurredAt       time.Time
	IssueID          string
	After            *ScopeIssue
	ExecutionStarted bool
}

type ScopeStatistics struct {
	Original                int      `json:"original"`
	Current                 int      `json:"current"`
	Cancelled               int      `json:"cancelled"`
	Effective               int      `json:"effective"`
	Completed               int      `json:"completed"`
	OriginalCompleted       int      `json:"original_completed"`
	Remaining               int      `json:"remaining"`
	AddedUnique             int      `json:"added_unique"`
	RemovedEvents           int      `json:"removed_events"`
	ReentryEvents           int      `json:"reentry_events"`
	CancelEvents            int      `json:"cancel_events"`
	ReopenEvents            int      `json:"reopen_events"`
	Started                 int      `json:"started"`
	InitialEffective        int      `json:"initial_effective"`
	NetEffectiveChange      int      `json:"net_effective_change"`
	NetEffectiveChangeRatio *float64 `json:"net_effective_change_ratio"`
	EffectiveRatio          *float64 `json:"effective_ratio"`
	OriginalRatio           *float64 `json:"original_ratio"`
}

type ScopeChartPoint struct {
	Date      string `json:"date"`
	Effective int    `json:"effective"`
	Completed int    `json:"completed"`
	Original  int    `json:"original"`
}

type scopeState struct {
	original map[string]ScopeIssue
	current  map[string]ScopeIssue
	seen     map[string]bool
	counts   ScopeStatistics
}

func validScopeIssue(issue ScopeIssue) bool {
	if issue.IssueID == "" {
		return false
	}
	switch issue.StatusCategory {
	case "backlog", "todo", "in_progress", "in_review", "blocked", "done", "cancelled":
		return true
	default:
		return false
	}
}

func startedCategory(category string) bool {
	return category == "in_progress" || category == "in_review" || category == "done"
}

func newScopeState(original []ScopeIssue) (*scopeState, error) {
	s := &scopeState{original: make(map[string]ScopeIssue), current: make(map[string]ScopeIssue), seen: make(map[string]bool)}
	for _, issue := range original {
		if !validScopeIssue(issue) {
			return nil, fmt.Errorf("invalid original issue facts for %q", issue.IssueID)
		}
		if old, ok := s.original[issue.IssueID]; ok {
			if old.StatusCategory != issue.StatusCategory || old.HasStarted != issue.HasStarted {
				return nil, fmt.Errorf("conflicting original facts for %q", issue.IssueID)
			}
			continue
		}
		s.original[issue.IssueID] = issue
		issue.HasStarted = issue.HasStarted || startedCategory(issue.StatusCategory)
		s.current[issue.IssueID] = issue
		s.seen[issue.IssueID] = true
		if issue.StatusCategory != "cancelled" {
			s.counts.InitialEffective++
		}
	}
	return s, nil
}

func orderedChanges(changes []ScopeChange) ([]ScopeChange, error) {
	result := append([]ScopeChange(nil), changes...)
	sort.Slice(result, func(i, j int) bool { return result[i].Sequence < result[j].Sequence })
	for i, c := range result {
		if c.Sequence <= 0 || (i > 0 && c.Sequence == result[i-1].Sequence) {
			return nil, fmt.Errorf("invalid or duplicate scope sequence %d", c.Sequence)
		}
		if c.IssueID == "" || (c.After != nil && (c.After.IssueID != c.IssueID || !validScopeIssue(*c.After))) {
			return nil, fmt.Errorf("invalid scope facts at sequence %d", c.Sequence)
		}
	}
	return result, nil
}

func (s *scopeState) apply(c ScopeChange) {
	before, present := s.current[c.IssueID]
	if c.After == nil {
		if present {
			delete(s.current, c.IssueID)
			s.counts.RemovedEvents++
		}
		return
	}
	after := *c.After
	after.HasStarted = after.HasStarted || c.ExecutionStarted || startedCategory(after.StatusCategory)
	if present {
		after.HasStarted = after.HasStarted || before.HasStarted
		if before.StatusCategory != "cancelled" && after.StatusCategory == "cancelled" {
			s.counts.CancelEvents++
		}
		if (before.StatusCategory == "cancelled" || before.StatusCategory == "done") && after.StatusCategory != "cancelled" && after.StatusCategory != "done" {
			s.counts.ReopenEvents++
		}
	} else {
		if s.seen[c.IssueID] {
			s.counts.ReentryEvents++
		} else {
			s.counts.AddedUnique++
			s.seen[c.IssueID] = true
		}
	}
	s.current[c.IssueID] = after
}

func ratio(n, d int) *float64 {
	if d == 0 {
		return nil
	}
	v := float64(n) / float64(d)
	return &v
}

func (s *scopeState) statistics() ScopeStatistics {
	result := s.counts
	result.Original = len(s.original)
	result.Current = len(s.current)
	for id, issue := range s.current {
		if issue.StatusCategory == "cancelled" {
			result.Cancelled++
			continue
		}
		result.Effective++
		if issue.HasStarted {
			result.Started++
		}
		if issue.StatusCategory == "done" {
			result.Completed++
			if _, ok := s.original[id]; ok {
				result.OriginalCompleted++
			}
		}
	}
	result.Remaining = result.Effective - result.Completed
	result.NetEffectiveChange = result.Effective - result.InitialEffective
	result.NetEffectiveChangeRatio = ratio(result.NetEffectiveChange, result.InitialEffective)
	result.EffectiveRatio = ratio(result.Completed, result.Effective)
	result.OriginalRatio = ratio(result.OriginalCompleted, result.Original)
	return result
}

// CalculateStatistics replays membership by sequence against the immutable
// original commitment. It does not consult current issues or mutate its input.
func CalculateStatistics(original []ScopeIssue, changes []ScopeChange) (ScopeStatistics, error) {
	state, err := newScopeState(original)
	if err != nil {
		return ScopeStatistics{}, err
	}
	ordered, err := orderedChanges(changes)
	if err != nil {
		return ScopeStatistics{}, err
	}
	for _, c := range ordered {
		state.apply(c)
	}
	return state.statistics(), nil
}

// BuildChart samples complete scope facts at each local day end, plus the
// current partial day. There are no points before actual start or after the
// supplied cutoff (the logical end for a frozen snapshot). Sequence is
// authoritative; a clock rollback cannot move a later event into an older day.
func BuildChart(original []ScopeIssue, changes []ScopeChange, startedAt, calculatedAt time.Time, timezone string) ([]ScopeChartPoint, error) {
	loc, err := ValidateTimezone(timezone)
	if err != nil {
		return nil, err
	}
	if startedAt.IsZero() || calculatedAt.Before(startedAt) {
		return nil, fmt.Errorf("invalid chart time range")
	}
	state, err := newScopeState(original)
	if err != nil {
		return nil, err
	}
	ordered, err := orderedChanges(changes)
	if err != nil {
		return nil, err
	}
	previous := startedAt
	for i := range ordered {
		if ordered[i].OccurredAt.IsZero() {
			return nil, fmt.Errorf("missing event time at sequence %d", ordered[i].Sequence)
		}
		if ordered[i].OccurredAt.Before(previous) {
			ordered[i].OccurredAt = previous
		}
		previous = ordered[i].OccurredAt
	}
	first, err := ParseDate(startedAt.In(loc).Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	last, err := ParseDate(calculatedAt.In(loc).Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	points := make([]ScopeChartPoint, 0)
	eventIndex := 0
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		label := day.Format(time.DateOnly)
		begin, err := DayBoundary(label, timezone)
		if err != nil {
			return nil, err
		}
		// A missing calendar date is unknown, not a fabricated flat/zero point.
		if begin.In(loc).Format(time.DateOnly) != label {
			continue
		}
		next, err := DayBoundary(day.AddDate(0, 0, 1).Format(time.DateOnly), timezone)
		if err != nil {
			return nil, err
		}
		for eventIndex < len(ordered) && ordered[eventIndex].OccurredAt.Before(next) && !ordered[eventIndex].OccurredAt.After(calculatedAt) {
			state.apply(ordered[eventIndex])
			eventIndex++
		}
		counts := state.statistics()
		points = append(points, ScopeChartPoint{Date: label, Effective: counts.Effective, Completed: counts.Completed, Original: counts.Original})
	}
	return points, nil
}
