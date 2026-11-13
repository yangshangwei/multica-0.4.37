package reportsnapshot

import (
	"encoding/json"
	"sort"

	"github.com/multica-ai/multica/server/internal/projecthealth"
)

type metricContext struct {
	input           PublicInput
	sources         map[string]PublicSource
	categories      map[string]string
	health          projecthealth.Collection
	healthAvailable bool
}

func computeMetrics(in Input) ([]Metric, error) {
	return computePublicMetrics(publicInputFromInternal(in))
}

func computePublicMetrics(in PublicInput) ([]Metric, error) {
	metrics, _, err := computePublicResult(in)
	return metrics, err
}

func computePublicResult(in PublicInput) ([]Metric, []string, error) {
	c := metricContext{input: in, sources: map[string]PublicSource{}, categories: map[string]string{}}
	for _, source := range in.Facts.Sources {
		c.sources[source.Kind] = source
	}
	for _, issue := range in.Facts.Issues {
		c.categories[issue.Status] = issue.Category
	}
	if p := in.Facts.Project; p != nil {
		signals := projecthealth.SignalInput{ProjectStatus: p.Status, ProjectDueDate: p.DueDate, LeadValid: boolValue(p.LeadValid), InProgressSince: p.InProgressSince, LatestUpdateAt: p.LatestUpdateAt, Calendar: publicCalendar(in)}
		for _, issue := range in.Facts.Issues {
			if isFormal(issue) {
				signals.Issues = append(signals.Issues, projecthealth.IssueSignal{ID: issue.ID, Status: issue.Status, Category: issue.Category, DueDate: issue.DueDate, AssignmentValid: boolValue(issue.AssignmentValid), EnvironmentUnavailable: boolValue(issue.EnvironmentUnavailable)})
			}
		}
		h, err := projecthealth.ComputeSignals(signals)
		if err != nil {
			return nil, nil, err
		}
		c.health, c.healthAvailable = h, true
	}
	out := []Metric{}
	names := []string{"total", "completed", "cancelled", "open", "unknown_status", "blocked", "overdue", "unassigned", "in_review", "risk_union", "execution_environment_unavailable"}
	for _, formal := range []bool{true, false} {
		prefix := "stock."
		if formal {
			prefix = "formal_stock."
		}
		for _, name := range names {
			out = append(out, c.stockMetric(prefix+name, name, formal))
		}
		out = append(out, c.closureMetric(prefix+"closure_ratio", formal))
	}
	out = append(out, c.projectMetrics()...)
	for _, name := range []string{"created", "completed", "cancelled", "closed", "reopened", "end_open", "end_in_progress", "end_blocked"} {
		out = append(out, c.windowMetric("window."+name, name))
	}
	created := findMetric(out, "window.created")
	closed := findMetric(out, "window.closed")
	net := Metric{ID: "window.net_created_minus_closed", ValueKind: "integer", Value: json.RawMessage("null"), Quality: "unknown", VerifiedIDs: []string{}, Coverage: dependencyCoverage([]string{"window.created", "window.closed"}, map[string]bool{"window.created": created.Quality == "exact", "window.closed": closed.Quality == "exact"})}
	if in.ProjectID == nil {
		net.Quality = "not_applicable"
		net.Coverage = emptyCoverage("required_dependency")
	} else if created.Quality == "exact" && closed.Quality == "exact" {
		var a, b int64
		_ = json.Unmarshal(created.Value, &a)
		_ = json.Unmarshal(closed.Value, &b)
		net.Value = jsonValue(a - b)
		net.Quality = "exact"
	}
	out = append(out, net)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	var reasons []string
	if findMetric(out, "project.health").Quality == "exact" {
		reasons = append([]string{}, c.health.Statistics.Reasons...)
		for _, reason := range reasons {
			if !contains([]string{"blocked_issues", "overdue_issues", "project_overdue", "unassigned_issues", "in_review_issues", "invalid_project_lead", "stale_progress", "ended_project_open_issues", "execution_environment_unavailable"}, reason) {
				return nil, nil, ErrUnsupported
			}
		}
	}
	return out, reasons, nil
}

func isFormal(i PublicIssueFact) bool {
	return i.AdmissionStatus == "not_required" || i.AdmissionStatus == "accepted"
}
func terminal(s string) bool              { return s == "done" || s == "cancelled" }
func jsonValue(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
func findMetric(in []Metric, id string) Metric {
	for _, m := range in {
		if m.ID == id {
			return m
		}
	}
	return Metric{}
}
func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func (c metricContext) gapAffects(source, metric, issueID string) bool {
	for _, g := range c.input.Facts.Gaps {
		if len(g.MetricIDs) > 0 && !contains(g.MetricIDs, metric) && !contains(g.MetricIDs, "*") {
			continue
		}
		if len(g.MetricIDs) == 0 && g.Source != "" && g.Source != source {
			continue
		}
		if len(g.IssueIDs) > 0 && issueID != "" && !contains(g.IssueIDs, issueID) {
			continue
		}
		return true
	}
	return false
}
func (c metricContext) available(source, metric, issueID string) bool {
	if source == "calendar" {
		return c.input.TimeBasis != nil && !c.gapAffects(source, metric, issueID)
	}
	s, ok := c.sources[source]
	return ok && s.ReadComplete && !c.gapAffects(source, metric, issueID)
}

func emptyCoverage(unit string) Coverage {
	return Coverage{Version: CoverageVersion, Unit: unit, Eligible: ptr(int64(0)), Unknown: UnknownCoverage{Count: ptr(int64(0)), Reasons: []string{}}, IncludedIDs: []string{}, ExcludedIDs: []string{}, UnknownIDs: []string{}}
}
func coverage(unit string, included, excluded, unknown []string, extentUnknown bool, reasons []string) Coverage {
	included = normalizeStrings(included)
	excluded = normalizeStrings(excluded)
	unknown = normalizeStrings(unknown)
	n := int64(len(included) + len(excluded) + len(unknown))
	u := int64(len(unknown))
	out := Coverage{Version: CoverageVersion, Unit: unit, Eligible: &n, Included: int64(len(included)), Excluded: int64(len(excluded)), Unknown: UnknownCoverage{Count: &u, KnownCount: u, ExtentUnknown: extentUnknown, Reasons: normalizeStrings(reasons)}, ObservedCandidateCount: n, IncludedIDs: included, ExcludedIDs: excluded, UnknownIDs: unknown}
	if extentUnknown {
		out.Eligible = nil
		out.Unknown.Count = nil
	} else if n-out.Excluded > 0 {
		out.Ratio = &Rational{Numerator: out.Included, Denominator: n - out.Excluded}
	}
	return out
}
func dependencyCoverage(names []string, known map[string]bool) Coverage {
	included, unknown := []string{}, []string{}
	for _, n := range names {
		if known[n] {
			included = append(included, n)
		} else {
			unknown = append(unknown, n)
		}
	}
	reasons := []string{}
	if len(unknown) > 0 {
		reasons = append(reasons, "required_dependency_unavailable")
	}
	return coverage("required_dependency", included, nil, unknown, false, reasons)
}

func (c metricContext) countMetric(id string, formal bool, known func(PublicIssueFact) bool, hit func(PublicIssueFact) bool) Metric {
	m := Metric{ID: id, ValueKind: "integer", Value: json.RawMessage("null"), Quality: "unknown", VerifiedIDs: []string{}, Coverage: emptyCoverage("issue")}
	if c.input.ProjectID == nil {
		m.Quality = "not_applicable"
		return m
	}
	included, excluded, unknown := []string{}, []string{}, []string{}
	hits := []string{}
	for _, i := range c.input.Facts.Issues {
		switch {
		case !isFormal(i) || !formal && i.Classification == "report":
			excluded = append(excluded, i.ID)
		case !formal && i.Classification == "unknown" || c.gapAffects("issues", id, i.ID) || !known(i):
			unknown = append(unknown, i.ID)
		default:
			included = append(included, i.ID)
			if hit(i) {
				hits = append(hits, i.ID)
			}
		}
	}
	extent := !c.cohortComplete(id)
	reasons := []string{}
	if extent {
		reasons = append(reasons, "candidate_extent_unknown")
	}
	if len(unknown) > 0 {
		reasons = append(reasons, "required_issue_facts_unknown")
	}
	m.Coverage = coverage("issue", included, excluded, unknown, extent, reasons)
	m.VerifiedIDs = normalizeStrings(hits)
	n := int64(len(hits))
	if !extent && len(unknown) == 0 {
		m.Value = jsonValue(n)
		m.Quality = "exact"
	} else {
		m.LowerBound = &n
		if n > 0 {
			m.Quality = "partial"
		}
	}
	return m
}

// Enumeration and a candidate's metric facts are different completeness
// claims. A named missing candidate does not imply an unbounded unseen tail.
func (c metricContext) cohortComplete(metric string) bool {
	source, exists := c.sources["issues"]
	if !exists || !source.ReadComplete {
		return false
	}
	for _, gap := range c.input.Facts.Gaps {
		if len(gap.IssueIDs) > 0 || gap.Source != "" && gap.Source != "issues" {
			continue
		}
		if len(gap.MetricIDs) > 0 && !contains(gap.MetricIDs, metric) && !contains(gap.MetricIDs, "*") {
			continue
		}
		switch gap.Code {
		case "scan_limit", "source_unavailable", "source_count_mismatch", "cohort_incomplete":
			return false
		}
	}
	return true
}

func (c metricContext) stockMetric(id, name string, formal bool) Metric {
	known := func(i PublicIssueFact) bool {
		if c.gapAffects("issues", id, i.ID) {
			return false
		}
		if name == "total" {
			return true
		}
		if !c.available("status_catalog", id, i.ID) || !c.healthAvailable {
			return false
		}
		if name != "unknown_status" && !validCategory(c.categories[i.Status]) {
			return false
		}
		if name == "unassigned" || name == "risk_union" || name == "execution_environment_unavailable" {
			return c.available("references", id, i.ID) && i.AssignmentValid != nil && i.EnvironmentUnavailable != nil
		}
		return true
	}
	hit := func(i PublicIssueFact) bool {
		category := c.categories[i.Status]
		switch name {
		case "total":
			return true
		case "completed":
			return category == "done"
		case "cancelled":
			return category == "cancelled"
		case "open":
			return validCategory(category) && !terminal(category)
		case "unknown_status":
			return !validCategory(category)
		case "risk_union":
			for _, signal := range []string{"blocked", "overdue", "unassigned", "in_review"} {
				if contains(c.health.Risks[signal], i.ID) {
					return true
				}
			}
			return false
		case "execution_environment_unavailable":
			return contains(c.health.EnvironmentUnavailableIDs, i.ID)
		default:
			return contains(c.health.Risks[name], i.ID)
		}
	}
	return c.countMetric(id, formal, known, hit)
}

func (c metricContext) closureMetric(id string, formal bool) Metric {
	denominator := c.stockMetric(id, "total", formal)
	numerator := c.countMetric(id, formal, func(i PublicIssueFact) bool {
		return c.available("status_catalog", id, i.ID) && validCategory(c.categories[i.Status])
	}, func(i PublicIssueFact) bool { return terminal(c.categories[i.Status]) })
	out := numerator
	out.ValueKind = "rational"
	out.LowerBound = nil
	out.Value = json.RawMessage("null")
	if numerator.Quality == "exact" && denominator.Quality == "exact" {
		var n, d int64
		_ = json.Unmarshal(numerator.Value, &n)
		_ = json.Unmarshal(denominator.Value, &d)
		if d > 0 {
			out.Value = jsonValue(Rational{Numerator: n, Denominator: d})
		}
		out.Quality = "exact"
	}
	return out
}

func (c metricContext) projectMetric(id, kind string, deps []string, value any, extraKnown bool) Metric {
	out := Metric{ID: id, ValueKind: kind, Value: json.RawMessage("null"), Quality: "unknown", VerifiedIDs: []string{}, Coverage: emptyCoverage("required_dependency")}
	if c.input.ProjectID == nil {
		out.Quality = "not_applicable"
		return out
	}
	known := map[string]bool{}
	for _, dep := range deps {
		known[dep] = c.healthAvailable && c.available(dep, id, "")
		if dep == "references" {
			known[dep] = known[dep] && c.input.Facts.Project != nil && c.input.Facts.Project.LeadValid != nil
			if id == "project.health" {
				for _, issue := range c.input.Facts.Issues {
					if isFormal(issue) && (issue.AssignmentValid == nil || issue.EnvironmentUnavailable == nil) {
						known[dep] = false
					}
				}
			}
		}
	}
	if !extraKnown && len(deps) > 0 {
		known[deps[0]] = false
	}
	out.Coverage = dependencyCoverage(deps, known)
	if out.Coverage.Unknown.KnownCount == 0 {
		out.Quality = "exact"
		out.Value = jsonValue(value)
	}
	return out
}
func (c metricContext) projectMetrics() []Metric {
	s := c.health.Statistics
	return []Metric{
		c.projectMetric("project.overdue", "boolean", []string{"project", "calendar"}, s.ProjectOverdue, true),
		c.projectMetric("project.lead_valid", "boolean", []string{"project", "references"}, s.LeadValid, true),
		c.projectMetric("project.latest_update_at", "timestamp", []string{"updates"}, s.LatestUpdateAt, true),
		c.projectMetric("project.progress_age_days", "integer", []string{"updates", "calendar"}, s.ProgressAgeDays, true),
		c.projectMetric("project.health", "enum", []string{"status_catalog", "project", "issues", "references", "updates", "calendar"}, s.Health, s.Complete),
	}
}

type lifecycleFacts struct {
	known                                  bool
	completed, cancelled, closed, reopened bool
	endCategory                            string
}

func (c metricContext) lifecycle(i PublicIssueFact, metric string) lifecycleFacts {
	unknown := lifecycleFacts{}
	source, ok := c.sources["activities"]
	if !ok || !source.ProducerComplete || !c.available("activities", metric, i.ID) || !i.HistoryComplete || i.BaselineAt == nil || i.BaselineAt.After(c.input.Window.Start) || !validCategory(i.BaselineCategory) || i.BaselineStatus == "" {
		return unknown
	}
	acts := []PublicActivity{}
	for _, a := range c.input.Facts.Activities {
		if a.IssueID == i.ID {
			acts = append(acts, a)
		}
	}
	for _, a := range acts {
		if !sameID(a.ProjectID, c.input.ProjectID) || !validCategory(a.FromCategory) || !validCategory(a.ToCategory) {
			return unknown
		}
	}
	sort.Slice(acts, func(a, b int) bool { return acts[a].Sequence < acts[b].Sequence })
	out := lifecycleFacts{known: true, endCategory: i.BaselineCategory}
	status, category := i.BaselineStatus, i.BaselineCategory
	previous := int64(0)
	last := *i.BaselineAt
	for _, a := range acts {
		if a.Sequence <= previous || a.BusinessAt.Before(last) || a.BusinessAt.Before(*i.BaselineAt) || a.BusinessAt.After(c.input.AsOf) || a.FromStatus != status || a.FromCategory != category {
			return unknown
		}
		previous = a.Sequence
		last = a.BusinessAt
		status = a.ToStatus
		category = a.ToCategory
		if a.BusinessAt.Before(c.input.Window.End) {
			out.endCategory = category
		}
		if a.BusinessAt.Before(c.input.Window.Start) || !a.BusinessAt.Before(c.input.Window.End) {
			continue
		}
		if a.FromCategory != "done" && a.ToCategory == "done" {
			out.completed = true
		}
		if a.FromCategory != "cancelled" && a.ToCategory == "cancelled" {
			out.cancelled = true
		}
		if !terminal(a.FromCategory) && terminal(a.ToCategory) {
			out.closed = true
		}
		if terminal(a.FromCategory) && !terminal(a.ToCategory) {
			out.reopened = true
		}
	}
	if status != i.Status {
		return unknown
	}
	return out
}

func (c metricContext) windowMetric(id, name string) Metric {
	lives := map[string]lifecycleFacts{}
	known := func(i PublicIssueFact) bool {
		if name == "created" {
			return i.CreationProjectKnown
		}
		if c.gapAffects("activities", id, i.ID) {
			return false
		}
		value := c.lifecycle(i, id)
		lives[i.ID] = value
		return value.known
	}
	hit := func(i PublicIssueFact) bool {
		if name == "created" {
			return !i.CreatedAt.Before(c.input.Window.Start) && i.CreatedAt.Before(c.input.Window.End)
		}
		v := lives[i.ID]
		switch name {
		case "completed":
			return v.completed
		case "cancelled":
			return v.cancelled
		case "closed":
			return v.closed
		case "reopened":
			return v.reopened
		case "end_open":
			return !terminal(v.endCategory)
		case "end_in_progress":
			return v.endCategory == "in_progress"
		case "end_blocked":
			return v.endCategory == "blocked"
		}
		return false
	}
	return c.countMetric(id, false, known, hit)
}

// Keep the set of public metric IDs visible to gap validation and consumers.
func MetricByID(s Snapshot, id string) (Metric, bool) {
	m := findMetric(s.Metrics, id)
	return m, m.ID != ""
}
