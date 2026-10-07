package iteration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// OriginalFacts is written once at start, including complete display identity.
// Participation execution evidence is distinct from the public display DTO.
type OriginalFacts struct {
	HistoricalIssue
	HasStarted bool `json:"has_started"`
}

type History struct {
	Iteration  Iteration
	Statistics Statistics
	Original   []HistoricalIssue
	Scope      []HistoricalIssue
	Events     []Event
	Snapshot   *Snapshot
}

func historyUUID(id pgtype.UUID) string { return uuid.UUID(id.Bytes).String() }
func historyNullableUUID(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	v := historyUUID(id)
	return &v
}
func historyText(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
func historyTime(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func IterationFromRow(row db.Iteration) Iteration {
	return Iteration{ID: historyUUID(row.ID), WorkspaceID: historyUUID(row.WorkspaceID), Name: row.Name, Description: historyText(row.Description), CoordinatorUserID: historyNullableUUID(row.CoordinatorUserID), Status: row.Status, Mode: row.Mode, StartDate: row.StartDate.Time.Format(time.DateOnly), EndDate: row.EndDate.Time.Format(time.DateOnly), Timezone: row.Timezone, Revision: row.Revision, ScopeRevision: row.ScopeRevision, StartedAt: historyTime(row.StartedAt), LogicalEndedAt: historyTime(row.LogicalEndedAt), ProcessedAt: historyTime(row.ProcessedAt)}
}

// LoadHistory must run inside a coherent read transaction or under the caller's
// I1 writer fence. It reads the complete event stream, independently of UI pages.
func LoadHistory(ctx context.Context, tx pgx.Tx, ws, id pgtype.UUID, calculatedAt time.Time) (History, error) {
	q := db.New(tx)
	row, err := q.GetIteration(ctx, db.GetIterationParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return History{}, &OperationError{Status: 404, Code: "iteration_not_found", Message: "Iteration not found"}
	}
	if err != nil {
		return History{}, err
	}
	h := History{Iteration: IterationFromRow(row), Original: []HistoricalIssue{}, Scope: []HistoricalIssue{}, Events: []Event{}}
	if row.Status == "completed" || (row.Status == "cancelled" && row.StartedAt.Valid) {
		stored, e := q.GetIterationSnapshot(ctx, db.GetIterationSnapshotParams{WorkspaceID: ws, IterationID: id})
		if e != nil {
			return History{}, fmt.Errorf("read frozen iteration snapshot: %w", e)
		}
		snapshot, e := DecodeSnapshot(stored)
		if e != nil {
			return History{}, e
		}
		if snapshot.EndType != row.Status || !row.LogicalEndedAt.Valid || !snapshot.LogicalEndedAt.Equal(row.LogicalEndedAt.Time) || !row.ProcessedAt.Valid || !snapshot.ProcessedAt.Equal(row.ProcessedAt.Time) {
			return History{}, errors.New("snapshot does not match closed iteration")
		}
		if !row.StartedAt.Valid {
			return History{}, errors.New("closed snapshot has no iteration start")
		}
		if err = validateSnapshotChart(snapshot, row.StartedAt.Time, row.Timezone); err != nil {
			return History{}, err
		}
		h.Snapshot = &snapshot
		h.Original = snapshot.Original
		h.Scope = snapshot.Scope
		h.Events = snapshot.Events
		h.Statistics = snapshot.Statistics
		return h, nil
	}
	if row.Status != "planned" && row.Status != "active" && row.Status != "cancelled" {
		return History{}, errors.New("unknown iteration status")
	}
	if (row.Status == "active") != row.StartedAt.Valid {
		return History{}, errors.New("iteration start facts inconsistent")
	}
	participations, err := q.ListIterationParticipations(ctx, db.ListIterationParticipationsParams{WorkspaceID: ws, IterationID: id})
	if err != nil {
		return History{}, err
	}
	h.Events, err = LoadHistoryEvents(ctx, tx, ws, id)
	if err != nil {
		return History{}, err
	}
	// A wall-clock correction cannot place a committed logical event in the
	// future of its own projection. Raw clock samples remain on each event.
	if row.StartedAt.Valid && calculatedAt.Before(row.StartedAt.Time) {
		calculatedAt = row.StartedAt.Time
	}
	for _, event := range h.Events {
		if calculatedAt.Before(event.OccurredAt) {
			calculatedAt = event.OccurredAt
		}
	}
	issues, err := q.ListIterationCurrentIssues(ctx, db.ListIterationCurrentIssuesParams{WorkspaceID: ws, CurrentIterationID: id})
	if err != nil {
		return History{}, err
	}
	h.Scope, err = CaptureHistoricalIssues(ctx, tx, ws, issues, false)
	if err != nil {
		return History{}, err
	}
	original, changes, err := historyScopeFacts(row, participations, h.Events)
	if err != nil {
		return History{}, err
	}
	originalByID := map[string]HistoricalIssue{}
	participationByID := map[string]db.IterationParticipation{}
	for _, p := range participations {
		participationByID[historyUUID(p.IssueID)] = p
		if p.InOriginal {
			facts, decodeErr := decodeOriginalFacts(p.OriginalFacts)
			if decodeErr != nil {
				return History{}, decodeErr
			}
			h.Original = append(h.Original, facts.HistoricalIssue)
			originalByID[facts.IssueID] = facts.HistoricalIssue
		}
	}
	state, err := newScopeState(original)
	if err != nil {
		return History{}, err
	}
	for _, change := range changes {
		state.apply(change)
	}
	if !row.StartedAt.Valid {
		state.current = map[string]ScopeIssue{}
	}
	currentCount := 0
	for _, p := range participations {
		if p.CurrentJoinedAt.Valid {
			currentCount++
		}
	}
	if currentCount != len(h.Scope) {
		return History{}, errors.New("current issue and participation sets disagree")
	}
	for i, item := range h.Scope {
		p, ok := participationByID[item.IssueID]
		if !ok || !p.CurrentJoinedAt.Valid {
			return History{}, errors.New("current issue has no participation")
		}
		h.Scope[i].WasCompletedAtStart = originalByID[item.IssueID].WasCompletedAtStart
		if row.StartedAt.Valid {
			factual, ok := state.current[item.IssueID]
			if !ok || factual.StatusCategory != item.StatusCategory || factual.HasStarted != p.HasStartedCurrentParticipation {
				return History{}, errors.New("current issue disagrees with persisted history")
			}
		} else {
			state.current[item.IssueID] = ScopeIssue{IssueID: item.IssueID, StatusCategory: item.StatusCategory, HasStarted: p.HasStartedCurrentParticipation || startedCategory(item.StatusCategory)}
		}
	}
	if len(state.current) != len(h.Scope) {
		return History{}, errors.New("event stream current scope disagrees with live scope")
	}
	counts := state.statistics()
	chart := []ScopeChartPoint{}
	if row.StartedAt.Valid {
		counts, err = CalculateStatistics(original, changes)
		if err != nil {
			return History{}, err
		}
		chart, err = BuildChart(original, changes, row.StartedAt.Time, calculatedAt, row.Timezone)
		if err != nil {
			return History{}, err
		}
	}
	h.Statistics = Statistics{ScopeStatistics: counts, Chart: chart, CalculatedAt: calculatedAt.UTC()}
	return h, nil
}

// LoadHistoryEvents reads the append-only audit stream for an already resolved,
// authorized iteration. Closed metadata corrections stay visible here without
// changing Snapshot.Events, the frozen scope, or historical statistics.
func LoadHistoryEvents(ctx context.Context, tx pgx.Tx, ws, id pgtype.UUID) ([]Event, error) {
	rows, err := db.New(tx).ListAllIterationEvents(ctx, db.ListAllIterationEventsParams{WorkspaceID: ws, IterationID: id})
	if err != nil {
		return nil, err
	}
	items := make([]Event, 0, len(rows))
	for _, event := range rows {
		value := Event{ID: historyUUID(event.ID), Sequence: event.Sequence, IterationID: historyUUID(event.IterationID), IssueID: historyNullableUUID(event.IssueID), OperationID: historyUUID(event.OperationID), Kind: event.Kind, Actor: event.Actor, OccurredAt: event.OccurredAt.Time.UTC(), SampledAt: event.SampledAt.Time.UTC(), BeforeFacts: event.BeforeFacts, AfterFacts: event.AfterFacts, Reason: historyText(event.Reason)}
		if err = validateHistoryEvent(value, historyUUID(id)); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, nil
}

func historyScopeFacts(row db.Iteration, participants []db.IterationParticipation, events []Event) ([]ScopeIssue, []ScopeChange, error) {
	original := []ScopeIssue{}
	for _, p := range participants {
		if !p.InOriginal {
			continue
		}
		if !row.StartedAt.Valid {
			return nil, nil, errors.New("unstarted iteration has original commitment")
		}
		facts, err := decodeOriginalFacts(p.OriginalFacts)
		if err != nil {
			return nil, nil, err
		}
		if err := validateHistoricalIssue(facts.HistoricalIssue); err != nil {
			return nil, nil, err
		}
		if facts.IssueID != historyUUID(p.IssueID) {
			return nil, nil, errors.New("original issue identity mismatch")
		}
		original = append(original, ScopeIssue{IssueID: facts.IssueID, StatusCategory: facts.StatusCategory, HasStarted: facts.HasStarted})
	}
	changes, err := historyChanges(row.StartedAt.Valid, events)
	return original, changes, err
}

func decodeOriginalFacts(raw []byte) (OriginalFacts, error) {
	var facts OriginalFacts
	if err := requireHistoricalIssueJSON(raw); err != nil {
		return facts, err
	}
	if err := requireJSONNonNull(raw, "has_started"); err != nil {
		return facts, err
	}
	if err := json.Unmarshal(raw, &facts); err != nil {
		return facts, err
	}
	if facts.WasCompletedAtStart != (facts.StatusCategory == "done") {
		return facts, errors.New("original completion flag disagrees with status")
	}
	return facts, validateHistoricalIssue(facts.HistoricalIssue)
}

func historyChanges(started bool, events []Event) ([]ScopeChange, error) {
	changes := []ScopeChange{}
	startSequence := int64(0)
	for _, event := range events {
		if event.Kind == "baseline" {
			if _, err := historyEventIssueFacts(event); err != nil {
				return nil, err
			}
		}
		if event.Kind == "start" {
			if startSequence != 0 || event.IssueID != nil {
				return nil, errors.New("invalid iteration start marker")
			}
			startSequence = event.Sequence
		}
	}
	if !started {
		return changes, nil
	}
	if startSequence == 0 {
		return nil, errors.New("started iteration has no start marker")
	}
	for _, event := range events {
		if event.Sequence <= startSequence {
			continue
		}
		switch event.Kind {
		case "baseline", "planned_activity", "edit", "edited", "create", "created", "date_edit", "end", "completed", "cancelled", "cancel_planned":
			continue
		case "join", "reenter", "leave", "delete", "issue_changed", "status", "cancel", "reopen", "execution_started":
		default:
			return nil, fmt.Errorf("unknown iteration event kind %q", event.Kind)
		}
		if event.IssueID == nil {
			return nil, errors.New("scope event requires an issue")
		}
		change := ScopeChange{Sequence: event.Sequence, OccurredAt: event.OccurredAt, IssueID: *event.IssueID, ExecutionStarted: event.Kind == "execution_started"}
		if event.Kind != "leave" && event.Kind != "delete" {
			facts, err := historyEventIssueFacts(event)
			if err != nil {
				return nil, err
			}
			change.After = &ScopeIssue{IssueID: *event.IssueID, StatusCategory: facts.StatusCategory, HasStarted: facts.HasStarted}
		} else if string(event.AfterFacts) != "null" {
			return nil, errors.New("leave event retains after facts")
		}
		changes = append(changes, change)
	}
	return orderedChanges(changes)
}

func historyEventIssueFacts(event Event) (IssueFacts, error) {
	var facts IssueFacts
	if event.IssueID == nil {
		return facts, errors.New("scope event requires an issue")
	}
	if err := json.Unmarshal(event.AfterFacts, &facts); err != nil {
		return facts, err
	}
	if !facts.IssueID.Valid || historyUUID(facts.IssueID) != *event.IssueID {
		return facts, errors.New("scope event issue identity mismatch")
	}
	if !validScopeIssue(ScopeIssue{IssueID: *event.IssueID, StatusCategory: facts.StatusCategory}) {
		return facts, errors.New("unknown scope status facts")
	}
	return facts, nil
}

func validateHistoricalIssue(item HistoricalIssue) error {
	seenLabels := map[string]bool{}
	for _, label := range item.Labels {
		if !validHistoryID(label.ID) || label.Name == "" || seenLabels[label.ID] {
			return errors.New("invalid historical label")
		}
		seenLabels[label.ID] = true
	}
	if !validHistoryID(item.IssueID) || item.Identifier == "" || item.Title == "" || item.StatusKey == "" || item.RolloverCount < 0 || item.RolloverCount > 2147483647 || !validScopeIssue(ScopeIssue{IssueID: item.IssueID, StatusCategory: item.StatusCategory}) {
		return errors.New("invalid historical issue")
	}
	for _, id := range []*string{item.ProjectID, item.AssigneeID} {
		if id != nil && !validHistoryID(*id) {
			return errors.New("invalid historical reference")
		}
	}
	if (item.AssigneeID == nil) != (item.AssigneeType == nil) {
		return errors.New("incomplete historical assignee")
	}
	if (item.ProjectName != nil && item.ProjectID == nil) || (item.AssigneeName != nil && item.AssigneeID == nil) {
		return errors.New("historical display name has no reference identity")
	}
	if item.AssigneeType != nil {
		switch *item.AssigneeType {
		case "member", "agent", "squad":
		default:
			return errors.New("unknown historical assignee type")
		}
	}
	return nil
}
func validHistoryID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
func validateHistoryEvent(event Event, iterationID string) error {
	if !validHistoryID(event.ID) || event.IterationID != iterationID || !validHistoryID(event.OperationID) || event.Sequence < 1 || event.Sequence > 9007199254740991 || event.Kind == "" || event.OccurredAt.IsZero() || event.SampledAt.IsZero() {
		return errors.New("invalid stored iteration event")
	}
	if event.IssueID != nil && !validHistoryID(*event.IssueID) {
		return errors.New("invalid event issue ID")
	}
	op, _ := uuid.Parse(event.OperationID)
	if err := validateIssueRecordIdentity(event.Actor, pgtype.UUID{Bytes: op, Valid: true}); err != nil {
		return err
	}
	if !json.Valid(event.BeforeFacts) || !json.Valid(event.AfterFacts) {
		return errors.New("invalid event facts")
	}
	return nil
}

func sortedHistoricalIssues(items []HistoricalIssue) []HistoricalIssue {
	result := append([]HistoricalIssue{}, items...)
	sort.Slice(result, func(i, j int) bool { return result[i].IssueID < result[j].IssueID })
	return result
}
