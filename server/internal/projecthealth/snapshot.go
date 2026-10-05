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
	Statistics  StatisticsSnapshot
	Risks       map[string][]string
	Categories  map[string]string
	StatusNames map[string]string
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

func Compute(in Input) (Collection, error) {
	zone := "UTC"
	if in.Timezone != nil {
		zone = *in.Timezone
	}
	loc, err := time.LoadLocation(zone)
	if err != nil || zone == "" || zone == "Local" {
		return Collection{}, fmt.Errorf("invalid planning timezone %q", zone)
	}
	day := in.Now.In(loc).Format(time.DateOnly)
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
	categories := statusCategories(statuses)
	reference := map[string]Reference{}
	for _, r := range refs {
		reference[r.Kind+":"+r.ID] = r
	}
	valid := func(kind, id string) bool { return kind != "" && id != "" && reference[kind+":"+id].Valid }
	byStatus := map[string]int64{}
	for _, issue := range issues {
		byStatus[issue.Status]++
	}
	counts := countScope(byStatus, categories)
	counts.Blocked, counts.Overdue, counts.Unassigned, counts.InReview, counts.RiskUnion, counts.ExecutionEnvironmentUnavailable = ptr(int64(0)), ptr(int64(0)), ptr(int64(0)), ptr(int64(0)), ptr(int64(0)), ptr(int64(0))
	out := Collection{Risks: map[string][]string{"blocked": {}, "overdue": {}, "unassigned": {}, "in_review": {}}, Categories: categories}
	seen := map[string]bool{}
	for _, i := range issues {
		if seen[i.ID] {
			return Collection{}, fmt.Errorf("duplicate formal issue %s", i.ID)
		}
		seen[i.ID] = true
		category := categories[i.Status]
		if category == "done" || category == "cancelled" {
			continue
		}
		hits := map[string]bool{"blocked": category == "blocked", "overdue": i.DueDate != nil && *i.DueDate < day, "unassigned": !valid(i.AssigneeType, i.AssigneeID), "in_review": category == "in_review"}
		risk := false
		for signal, hit := range hits {
			if hit {
				out.Risks[signal] = append(out.Risks[signal], i.ID)
				risk = true
			}
		}
		if risk {
			*counts.RiskUnion++
		}
		if valid(i.AssigneeType, i.AssigneeID) && reference[i.AssigneeType+":"+i.AssigneeID].EnvironmentUnavailable {
			*counts.ExecutionEnvironmentUnavailable++
		}
	}
	*counts.Blocked = int64(len(out.Risks["blocked"]))
	*counts.Overdue = int64(len(out.Risks["overdue"]))
	*counts.Unassigned = int64(len(out.Risks["unassigned"]))
	*counts.InReview = int64(len(out.Risks["in_review"]))
	ended := in.Project.Status == "completed" || in.Project.Status == "cancelled"
	overdue := in.Project.DueDate != nil && *in.Project.DueDate < day && !ended
	lead := valid(in.Project.LeadType, in.Project.LeadID)
	snapshot := StatisticsSnapshot{WorkspaceID: in.Project.WorkspaceID, ProjectID: in.Project.ID, ProjectRevision: in.Project.Revision, CalculatedAt: in.Now.UTC(), ReferenceDate: day, Timezone: zone, TimezoneConfigured: in.Timezone != nil, Complete: *counts.UnknownStatus == 0, IncompleteReasons: []string{}, Counts: counts, ProjectOverdue: &overdue, LeadValid: &lead, LatestUpdateAt: in.LatestUpdateAt, Reasons: []string{}, Health: "clear"}
	if *counts.Total > 0 {
		snapshot.ClosureRatio = ptr(float64(*counts.Completed+*counts.Cancelled) / float64(*counts.Total))
	}
	anchor := in.LatestUpdateAt
	if anchor != nil {
		age := calendarDays(*anchor, in.Now, loc)
		snapshot.ProgressAgeDays = &age
	} else {
		anchor = in.Project.InProgressSince
	}
	stale := in.Project.Status == "in_progress" && anchor != nil && calendarDays(*anchor, in.Now, loc) >= 7
	for _, reason := range []struct {
		hit  bool
		code string
	}{
		{*counts.Blocked > 0, "blocked_issues"}, {*counts.Overdue > 0, "overdue_issues"}, {overdue, "project_overdue"}, {*counts.Unassigned > 0, "unassigned_issues"}, {*counts.InReview > 0, "in_review_issues"}, {!lead, "invalid_project_lead"}, {stale, "stale_progress"}, {ended && *counts.Open > 0, "ended_project_open_issues"}, {*counts.ExecutionEnvironmentUnavailable > 0, "execution_environment_unavailable"},
	} {
		if reason.hit {
			snapshot.Reasons = append(snapshot.Reasons, reason.code)
		}
	}
	switch {
	case !snapshot.Complete:
		snapshot.Health = "unavailable"
		snapshot.IncompleteReasons = append(snapshot.IncompleteReasons, "unknown_status")
	case *counts.Total == 0:
		snapshot.Health = "empty"
	case *counts.Blocked > 0 || *counts.Overdue > 0 || overdue:
		snapshot.Health = "risk"
	case *counts.Unassigned > 0 || *counts.InReview > 0 || !lead || stale:
		snapshot.Health = "attention"
	}
	canonical, err := CanonicalJSON(struct {
		Version        int
		Project        Project
		ReferenceDate  string
		Timezone       *string
		Statuses       []Status
		Issues         []Issue
		References     []Reference
		LatestUpdateAt *time.Time
	}{1, in.Project, day, in.Timezone, statuses, issues, refs, in.LatestUpdateAt})
	if err != nil {
		return Collection{}, err
	}
	digest := sha256.Sum256(canonical)
	snapshot.SnapshotVersion = hex.EncodeToString(digest[:])
	out.Statistics = snapshot
	return out, nil
}

func ptr[T any](v T) *T { return &v }
func calendarDays(from, to time.Time, loc *time.Location) int {
	date := func(t time.Time) time.Time {
		y, m, d := t.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	days := int(date(to).Sub(date(from)) / (24 * time.Hour))
	if days < 0 {
		return 0
	}
	return days
}
