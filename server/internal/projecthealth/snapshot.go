// Package projecthealth owns the formal project scope and its computed facts.
// It does not change issues, execution, project state, or acceptance outcomes.
package projecthealth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/multica-ai/multica/server/internal/util"
)

type Counts struct {
	Total                           *int64 `json:"total"`
	Completed                       *int64 `json:"completed"`
	Cancelled                       *int64 `json:"cancelled"`
	Open                            *int64 `json:"open"`
	Blocked                         *int64 `json:"blocked"`
	Overdue                         *int64 `json:"overdue"`
	Unassigned                      *int64 `json:"unassigned"`
	InReview                        *int64 `json:"in_review"`
	RiskUnion                       *int64 `json:"risk_union"`
	UnknownStatus                   *int64 `json:"unknown_status"`
	ExecutionEnvironmentUnavailable *int64 `json:"execution_environment_unavailable"`
}

// StatisticsSnapshot deliberately contains no update, acceptance or snapshot.
type StatisticsSnapshot struct {
	WorkspaceID        string     `json:"workspace_id"`
	ProjectID          string     `json:"project_id"`
	ProjectRevision    int64      `json:"project_revision"`
	SnapshotVersion    string     `json:"snapshot_version"`
	CalculatedAt       time.Time  `json:"calculated_at"`
	ReferenceDate      string     `json:"reference_date"`
	Timezone           string     `json:"timezone"`
	TimezoneConfigured bool       `json:"timezone_configured"`
	Complete           bool       `json:"complete"`
	IncompleteReasons  []string   `json:"incomplete_reasons"`
	Counts             Counts     `json:"counts"`
	ClosureRatio       *float64   `json:"closure_ratio"`
	ProjectOverdue     *bool      `json:"project_overdue"`
	LeadValid          *bool      `json:"lead_valid"`
	LatestUpdateAt     *time.Time `json:"latest_update_at"`
	ProgressAgeDays    *int       `json:"progress_age_days"`
	Health             string     `json:"health"`
	Reasons            []string   `json:"reasons"`
}

type Project struct {
	WorkspaceID           string
	ID                    string
	Revision              int64
	Status                string
	DueDate               *string
	LeadType              string
	LeadID                string
	InProgressSince       *time.Time
	InProgressSinceSource string
}

type Issue struct {
	ID           string
	Status       string
	DueDate      *string
	AssigneeType string
	AssigneeID   string
}

type Status struct{ Key, Category string }

// Reference carries lifecycle validity independently from machine readiness.
// Facts contain only availability inputs, never labels or protected content.
type Reference struct {
	Kind                   string
	ID                     string
	Valid                  bool
	EnvironmentUnavailable bool
	Facts                  any
}

type Input struct {
	Project        Project
	Timezone       *string
	Now            time.Time
	Statuses       []Status
	Issues         []Issue
	References     []Reference
	LatestUpdateAt *time.Time
}

type Collection struct {
	EnvironmentUnavailableIDs []string
	Statistics                StatisticsSnapshot
	Risks                     map[string][]string
	Categories                map[string]string
	StatusNames               map[string]string
}

// CanonicalJSON sorts nested object keys and preserves JSON integer precision.
func CanonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var normalized any
	if err = decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err = encoder.Encode(normalized); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte{'\n'}), nil
}

// CalendarFacts is the resolved calendar input used by the health rules.
// Snapshot consumers retain these dates instead of reinterpreting old instants
// after a timezone database update.
type CalendarFacts struct {
	ReferenceDate       string
	LatestUpdateDate    *string
	InProgressSinceDate *string
}

func Compute(in Input) (Collection, error) {
	zone := effectiveTimezone(in)
	loc, err := time.LoadLocation(zone)
	if err != nil || zone == "" || zone == "Local" {
		return Collection{}, fmt.Errorf("invalid planning timezone %q", zone)
	}
	calendar := CalendarFacts{ReferenceDate: in.Now.In(loc).Format(time.DateOnly)}
	if in.LatestUpdateAt != nil {
		calendar.LatestUpdateDate = ptr(in.LatestUpdateAt.In(loc).Format(time.DateOnly))
	}
	if in.Project.InProgressSince != nil {
		calendar.InProgressSinceDate = ptr(in.Project.InProgressSince.In(loc).Format(time.DateOnly))
	}
	return ComputeWithCalendar(in, calendar)
}

// ComputeWithCalendar runs the same health rules using already resolved dates.
// The caller must bind these dates to its retained time provenance. This entry
// point deliberately does not load the current timezone database.
func ComputeWithCalendar(in Input, calendar CalendarFacts) (Collection, error) {
	zone := effectiveTimezone(in)
	if zone == "" || zone == "Local" || !validCalendarDate(calendar.ReferenceDate) ||
		(in.LatestUpdateAt == nil) != (calendar.LatestUpdateDate == nil) ||
		(in.Project.InProgressSince == nil) != (calendar.InProgressSinceDate == nil) {
		return Collection{}, fmt.Errorf("invalid frozen planning calendar")
	}
	for _, date := range []*string{calendar.LatestUpdateDate, calendar.InProgressSinceDate} {
		if date != nil && !validCalendarDate(*date) {
			return Collection{}, fmt.Errorf("invalid frozen planning date")
		}
	}
	day := calendar.ReferenceDate
	statuses := append([]Status(nil), in.Statuses...)
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Key < statuses[j].Key })
	issues := append([]Issue(nil), in.Issues...)
	sort.Slice(issues, func(i, j int) bool { return issues[i].ID < issues[j].ID })
	refs := append([]Reference(nil), in.References...)
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Kind != refs[j].Kind {
			return refs[i].Kind < refs[j].Kind
		}
		return refs[i].ID < refs[j].ID
	})
	categories := StatusCategories(statuses)
	reference := map[string]Reference{}
	for _, r := range refs {
		reference[r.Kind+":"+r.ID] = r
	}
	valid := func(kind, id string) bool { return kind != "" && id != "" && reference[kind+":"+id].Valid }
	signals := SignalInput{ProjectStatus: in.Project.Status, ProjectDueDate: in.Project.DueDate, LeadValid: valid(in.Project.LeadType, in.Project.LeadID), InProgressSince: in.Project.InProgressSince, LatestUpdateAt: in.LatestUpdateAt, Calendar: calendar}
	for _, issue := range issues {
		signals.Issues = append(signals.Issues, IssueSignal{ID: issue.ID, Status: issue.Status, Category: categories[issue.Status], DueDate: issue.DueDate, AssignmentValid: valid(issue.AssigneeType, issue.AssigneeID), EnvironmentUnavailable: reference[issue.AssigneeType+":"+issue.AssigneeID].EnvironmentUnavailable})
	}
	out, err := ComputeSignals(signals)
	if err != nil {
		return Collection{}, err
	}
	out.Categories = categories
	snapshot := out.Statistics
	snapshot.WorkspaceID, snapshot.ProjectID, snapshot.ProjectRevision = in.Project.WorkspaceID, in.Project.ID, in.Project.Revision
	snapshot.CalculatedAt, snapshot.Timezone, snapshot.TimezoneConfigured = in.Now.UTC(), zone, in.Timezone != nil
	canonical, err := CanonicalJSON(struct {
		Version           int
		Project           Project
		ReferenceDate     string
		Timezone          *string
		EffectiveTimezone string
		Statuses          []Status
		Issues            []Issue
		References        []Reference
		LatestUpdateAt    *time.Time
	}{1, in.Project, day, in.Timezone, zone, statuses, issues, refs, in.LatestUpdateAt})
	if err != nil {
		return Collection{}, err
	}
	digest := sha256.Sum256(canonical)
	snapshot.SnapshotVersion = hex.EncodeToString(digest[:])
	out.Statistics = snapshot
	return out, nil
}

func ptr[T any](v T) *T { return &v }

func effectiveTimezone(in Input) string {
	if in.Timezone != nil {
		return *in.Timezone
	}
	return util.DefaultPlanningTimezone
}

func validCalendarDate(s string) bool {
	value, err := time.Parse(time.DateOnly, s)
	return err == nil && value.Format(time.DateOnly) == s
}

func calendarDateDays(from, to string) int {
	a, _ := time.Parse(time.DateOnly, from)
	b, _ := time.Parse(time.DateOnly, to)
	days := int(b.Sub(a) / (24 * time.Hour))
	if days < 0 {
		return 0
	}
	return days
}
