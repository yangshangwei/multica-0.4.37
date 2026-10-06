package iteration

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// BuildSnapshot consumes an already coherent, resolved history. Closure calls
// it before releasing memberships. It does not query live entities or persist.
func BuildSnapshot(history History, operationID, endType, reason string, logicalEndedAt, processedAt time.Time, destinations []Destination) (Snapshot, error) {
	if history.Snapshot != nil || history.Iteration.Status != "active" || history.Iteration.StartedAt == nil || logicalEndedAt.Before(*history.Iteration.StartedAt) || processedAt.Before(logicalEndedAt) || !history.Statistics.CalculatedAt.Equal(logicalEndedAt) {
		return Snapshot{}, errors.New("snapshot requires active history calculated at logical end")
	}
	if !validHistoryID(operationID) || (endType != "completed" && endType != "cancelled") || strings.TrimSpace(reason) == "" {
		return Snapshot{}, errors.New("invalid snapshot operation")
	}
	snapshot := Snapshot{SchemaVersion: SchemaVersion, WorkspaceID: history.Iteration.WorkspaceID, IterationID: history.Iteration.ID, OperationID: operationID, EndType: endType, Reason: reason, LogicalEndedAt: logicalEndedAt.UTC(), ProcessedAt: processedAt.UTC(), Original: sortedHistoricalIssues(history.Original), Scope: sortedHistoricalIssues(history.Scope), Events: append([]Event{}, history.Events...), Statistics: history.Statistics, Destinations: append([]Destination{}, destinations...)}
	sort.Slice(snapshot.Destinations, func(i, j int) bool { return snapshot.Destinations[i].IssueID < snapshot.Destinations[j].IssueID })
	if err := validateSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	if err := validateSnapshotChart(snapshot, *history.Iteration.StartedAt, history.Iteration.Timezone); err != nil {
		return Snapshot{}, err
	}
	// Deep ownership includes nested nullable display values and raw event JSON.
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	var result Snapshot
	if err = json.Unmarshal(raw, &result); err != nil {
		return Snapshot{}, err
	}
	return result, nil
}

func validateSnapshotChart(snapshot Snapshot, startedAt time.Time, timezone string) error {
	original := make([]ScopeIssue, 0, len(snapshot.Original))
	for _, issue := range snapshot.Original {
		original = append(original, ScopeIssue{IssueID: issue.IssueID, StatusCategory: issue.StatusCategory})
	}
	changes, err := historyChanges(true, snapshot.Events)
	if err != nil {
		return err
	}
	chart, err := BuildChart(original, changes, startedAt, snapshot.LogicalEndedAt, timezone)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(chart, snapshot.Statistics.Chart) {
		return errors.New("snapshot chart disagrees with frozen facts")
	}
	return nil
}

// DecodeSnapshot validates stored identity and completeness. Closed reads must
// fail closed when a payload is missing/damaged, never rebuild it from live rows.
func DecodeSnapshot(row db.IterationSnapshot) (Snapshot, error) {
	var value Snapshot
	if err := requireJSONFields(row.Body, "schema_version", "workspace_id", "iteration_id", "operation_id", "end_type", "reason", "logical_ended_at", "processed_at", "original", "scope", "events", "statistics", "destinations"); err != nil {
		return value, err
	}
	if err := requireJSONNonNull(row.Body, "schema_version", "workspace_id", "iteration_id", "operation_id", "end_type", "reason", "logical_ended_at", "processed_at"); err != nil {
		return value, err
	}
	if err := json.Unmarshal(row.Body, &value); err != nil {
		return value, err
	}
	if row.SchemaVersion != SchemaVersion || value.WorkspaceID != historyUUID(row.WorkspaceID) || value.IterationID != historyUUID(row.IterationID) || value.OperationID != historyUUID(row.OperationID) {
		return value, errors.New("stored snapshot identity mismatch")
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(row.Body, &body); err != nil {
		return value, err
	}
	if err := requireJSONFields(body["statistics"], "original", "current", "cancelled", "effective", "completed", "original_completed", "remaining", "added_unique", "removed_events", "reentry_events", "cancel_events", "reopen_events", "started", "initial_effective", "net_effective_change", "net_effective_change_ratio", "effective_ratio", "original_ratio", "chart", "calculated_at"); err != nil {
		return value, err
	}
	if err := requireJSONNonNull(body["statistics"], "original", "current", "cancelled", "effective", "completed", "original_completed", "remaining", "added_unique", "removed_events", "reentry_events", "cancel_events", "reopen_events", "started", "initial_effective", "net_effective_change", "calculated_at"); err != nil {
		return value, err
	}
	for _, field := range []string{"original", "scope"} {
		var items []json.RawMessage
		if err := json.Unmarshal(body[field], &items); err != nil {
			return value, err
		}
		for _, item := range items {
			if err := requireHistoricalIssueJSON(item); err != nil {
				return value, err
			}
		}
	}
	var destinations []json.RawMessage
	if err := json.Unmarshal(body["destinations"], &destinations); err != nil {
		return value, err
	}
	for _, destination := range destinations {
		if err := requireJSONFields(destination, "issue_id", "target_iteration_id", "rollover_count_before", "rollover_count_after"); err != nil {
			return value, err
		}
		// Only target_iteration_id is nullable. The typed destination decode
		// above rejects noninteger tokens; validateSnapshot enforces DB bounds.
		if err := requireJSONNonNull(destination, "issue_id", "rollover_count_before", "rollover_count_after"); err != nil {
			return value, err
		}
	}
	return value, validateSnapshot(value)
}

func requireHistoricalIssueJSON(raw []byte) error {
	if err := requireJSONFields(raw, "issue_id", "identifier", "title", "project_id", "project_name", "assignee_type", "assignee_id", "assignee_name", "status_key", "status_category", "was_completed_at_start", "rollover_count"); err != nil {
		return err
	}
	return requireJSONNonNull(raw, "issue_id", "identifier", "title", "status_key", "status_category", "was_completed_at_start", "rollover_count")
}

func requireJSONNonNull(raw []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return errors.New("stored historical object is malformed")
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok || string(value) == "null" {
			return fmt.Errorf("stored historical field %s must not be null", field)
		}
	}
	return nil
}

func requireJSONFields(raw []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return errors.New("stored historical object is malformed")
	}
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("stored historical object missing %s", field)
		}
	}
	return nil
}

func validateSnapshot(value Snapshot) error {
	if value.SchemaVersion != SchemaVersion || !validHistoryID(value.WorkspaceID) || !validHistoryID(value.IterationID) || !validHistoryID(value.OperationID) || (value.EndType != "completed" && value.EndType != "cancelled") || strings.TrimSpace(value.Reason) == "" || value.LogicalEndedAt.IsZero() || value.ProcessedAt.Before(value.LogicalEndedAt) {
		return errors.New("invalid snapshot header")
	}
	if value.Original == nil || value.Scope == nil || value.Events == nil || value.Destinations == nil || value.Statistics.Chart == nil {
		return errors.New("snapshot collections must be present")
	}
	originals := map[string]HistoricalIssue{}
	scope := map[string]HistoricalIssue{}
	for _, set := range []struct {
		items []HistoricalIssue
		byID  map[string]HistoricalIssue
	}{{value.Original, originals}, {value.Scope, scope}} {
		for _, item := range set.items {
			if err := validateHistoricalIssue(item); err != nil {
				return err
			}
			if _, ok := set.byID[item.IssueID]; ok {
				return errors.New("duplicate snapshot issue")
			}
			set.byID[item.IssueID] = item
		}
	}
	lastSequence := int64(0)
	for _, event := range value.Events {
		if err := validateHistoryEvent(event, value.IterationID); err != nil {
			return err
		}
		if event.Sequence <= lastSequence || event.OccurredAt.After(value.LogicalEndedAt) {
			return errors.New("snapshot event order or cutoff invalid")
		}
		lastSequence = event.Sequence
	}
	counts := value.Statistics.ScopeStatistics
	if !value.Statistics.CalculatedAt.Equal(value.LogicalEndedAt) || counts.Original != len(originals) || counts.Current != len(scope) || counts.Cancelled < 0 || counts.Effective < 0 || counts.Completed < 0 || counts.OriginalCompleted < 0 || counts.Remaining < 0 || counts.Started < 0 || counts.InitialEffective < 0 || counts.AddedUnique < 0 || counts.RemovedEvents < 0 || counts.ReentryEvents < 0 || counts.CancelEvents < 0 || counts.ReopenEvents < 0 {
		return errors.New("snapshot statistics invalid")
	}
	baseline := map[string]IssueFacts{}
	for _, event := range value.Events {
		if event.Kind == "baseline" {
			facts, err := historyEventIssueFacts(event)
			if err != nil {
				return err
			}
			baseline[*event.IssueID] = facts
		}
	}
	canonicalOriginal := make([]ScopeIssue, 0, len(originals))
	for id, item := range originals {
		if item.WasCompletedAtStart != (item.StatusCategory == "done") {
			return errors.New("snapshot original completion flag invalid")
		}
		facts, ok := baseline[id]
		if !ok || facts.StatusCategory != item.StatusCategory {
			return errors.New("snapshot missing original baseline")
		}
		canonicalOriginal = append(canonicalOriginal, ScopeIssue{IssueID: id, StatusCategory: item.StatusCategory, HasStarted: facts.HasStarted})
	}
	changes, err := historyChanges(true, value.Events)
	if err != nil {
		return err
	}
	canonical, err := CalculateStatistics(canonicalOriginal, changes)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(counts, canonical) {
		return errors.New("snapshot statistics disagree with frozen facts")
	}
	state, err := newScopeState(canonicalOriginal)
	if err != nil {
		return err
	}
	for _, change := range changes {
		state.apply(change)
	}
	for id, item := range scope {
		fact, ok := state.current[id]
		if !ok || fact.StatusCategory != item.StatusCategory || item.WasCompletedAtStart != originals[id].WasCompletedAtStart {
			return errors.New("snapshot scope disagrees with frozen events")
		}
	}
	previous := ""
	for _, point := range value.Statistics.Chart {
		if _, err := ParseDate(point.Date); err != nil {
			return err
		}
		if point.Date <= previous || point.Effective < 0 || point.Completed < 0 || point.Completed > point.Effective || point.Original != counts.Original {
			return errors.New("snapshot chart invalid")
		}
		previous = point.Date
	}
	seen := map[string]bool{}
	if len(value.Destinations) != len(scope) {
		return errors.New("snapshot destinations must cover the complete scope")
	}
	for _, destination := range value.Destinations {
		item, ok := scope[destination.IssueID]
		if !ok || seen[destination.IssueID] || destination.RolloverCountBefore != item.RolloverCount || destination.RolloverCountAfter < destination.RolloverCountBefore || destination.RolloverCountAfter > 2147483647 {
			return errors.New("invalid snapshot destination")
		}
		if destination.TargetIterationID != nil && (!validHistoryID(*destination.TargetIterationID) || *destination.TargetIterationID == value.IterationID) {
			return errors.New("invalid snapshot destination target")
		}
		if destination.TargetIterationID == nil {
			if destination.RolloverCountAfter != destination.RolloverCountBefore {
				return errors.New("released issue must retain rollover count")
			}
		} else if item.StatusCategory == "done" || item.StatusCategory == "cancelled" || destination.RolloverCountAfter != destination.RolloverCountBefore+1 {
			return errors.New("rollover requires nonterminal issue and one increment")
		}
		seen[destination.IssueID] = true
	}
	return nil
}
