package reportsnapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
)

// ProjectPublic is called when sealing, never as a fallback from a content GET.
// It checks the internal seal and freezes a separately verifiable public input.
func ProjectPublic(s Snapshot) (PublicSnapshot, error) {
	if err := Validate(s); err != nil {
		return PublicSnapshot{}, err
	}
	out, err := buildPublic(publicInputFromInternal(s.Input))
	if err != nil {
		return PublicSnapshot{}, err
	}
	want, _ := CanonicalJSON(s.Metrics)
	got, _ := CanonicalJSON(out.Metrics)
	if !bytes.Equal(want, got) {
		return PublicSnapshot{}, fmt.Errorf("%w: public metric projection mismatch", ErrInvalid)
	}
	return out, nil
}

// ReplayPublic requires only the saved public facts, not internal identities,
// a source service, the current time or the current timezone database.
func ReplayPublic(input PublicInput) ([]Metric, error) {
	in, err := normalizePublicInput(input)
	if err != nil {
		return nil, err
	}
	return computePublicMetrics(in)
}

func buildPublic(input PublicInput) (PublicSnapshot, error) {
	in, err := normalizePublicInput(input)
	if err != nil {
		return PublicSnapshot{}, err
	}
	metrics, reasons, err := computePublicResult(in)
	if err != nil {
		return PublicSnapshot{}, err
	}
	complete := true
	for _, metric := range metrics {
		if metric.Quality != "exact" && metric.Quality != "not_applicable" {
			complete = false
		}
	}
	state := "sealed_partial"
	if complete {
		state = "sealed_complete"
	}
	out := PublicSnapshot{PublicInput: in, FormalScope: "ordinary_formal_as_of", ProtocolVersion: ProtocolVersion, PublicProjectionVersion: PublicProjectionVersion, SemanticsVersion: SemanticsVersion, CoverageVersion: CoverageVersion, TimeRulesVersion: TimeRulesVersion, ReferenceDate: in.TimeBasis.ReferenceDate, State: state, Complete: complete, Metrics: metrics, ProjectReasons: reasons}
	if out.FactsDigest, err = digest(in.Facts); err != nil {
		return PublicSnapshot{}, err
	}
	out.ResultDigest, err = digest(struct {
		WorkspaceID      string     `json:"workspace_id"`
		ProjectID        *string    `json:"project_id"`
		ReportIssueID    string     `json:"report_issue_id"`
		Window           Window     `json:"window"`
		TimeBasis        *TimeBasis `json:"time_basis"`
		AsOf             time.Time  `json:"as_of"`
		SemanticsVersion string     `json:"semantics_version"`
		CoverageVersion  string     `json:"coverage_version"`
		Metrics          []Metric   `json:"metrics"`
		ProjectReasons   []string   `json:"project_reasons"`
		Gaps             []Gap      `json:"gaps"`
	}{in.WorkspaceID, in.ProjectID, in.ReportIssueID, in.Window, in.TimeBasis, in.AsOf, SemanticsVersion, CoverageVersion, metrics, reasons, in.Facts.Gaps})
	if err != nil {
		return PublicSnapshot{}, err
	}
	if out.ProjectionDigest, err = digest(out); err != nil {
		return PublicSnapshot{}, err
	}
	raw, err := CanonicalJSON(out)
	if err != nil || len(raw) > MaxSnapshotBytes {
		return PublicSnapshot{}, ErrTooLarge
	}
	return out, nil
}

func ValidatePublic(s PublicSnapshot) error {
	if s.FormalScope != "ordinary_formal_as_of" || s.ProtocolVersion != ProtocolVersion || s.PublicProjectionVersion != PublicProjectionVersion || s.SemanticsVersion != SemanticsVersion || s.CoverageVersion != CoverageVersion || s.TimeRulesVersion != TimeRulesVersion {
		return ErrUnsupported
	}
	expected, err := buildPublic(s.PublicInput)
	if err != nil {
		return err
	}
	a, err := CanonicalJSON(s)
	if err != nil {
		return ErrInvalid
	}
	b, err := CanonicalJSON(expected)
	if err != nil || !bytes.Equal(a, b) {
		return fmt.Errorf("%w: inconsistent public seal", ErrInvalid)
	}
	return nil
}

func DecodePublic(raw []byte) (PublicSnapshot, error) {
	var out PublicSnapshot
	if len(raw) > MaxSnapshotBytes {
		return out, ErrTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := checkJSONValue(decoder); err != nil {
		return out, ErrInvalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return out, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return PublicSnapshot{}, fmt.Errorf("%w: public JSON schema", ErrInvalid)
	}
	// This also rejects missing fields, null-to-zero coercions, case aliases,
	// and lossy integer forms that encoding/json otherwise accepts.
	var incoming, encoded any
	d = json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&incoming); err != nil {
		return PublicSnapshot{}, ErrInvalid
	}
	normalized, err := CanonicalJSON(out)
	if err != nil {
		return PublicSnapshot{}, ErrInvalid
	}
	d = json.NewDecoder(bytes.NewReader(normalized))
	d.UseNumber()
	if err := d.Decode(&encoded); err != nil || !reflect.DeepEqual(incoming, encoded) {
		return PublicSnapshot{}, fmt.Errorf("%w: public field shape", ErrInvalid)
	}
	if err := ValidatePublic(out); err != nil {
		return PublicSnapshot{}, err
	}
	return out, nil
}

func PublicMetricByID(s PublicSnapshot, id string) (Metric, bool) {
	metric := findMetric(s.Metrics, id)
	return metric, metric.ID != ""
}

func normalizePublicInput(input PublicInput) (PublicInput, error) {
	if len(input.Facts.Issues) > MaxIssues+1 || len(input.Facts.Activities) > MaxActivities {
		return PublicInput{}, ErrTooLarge
	}
	raw, err := CanonicalJSON(input)
	if err != nil {
		return PublicInput{}, ErrInvalid
	}
	if len(raw) > MaxSnapshotBytes {
		return PublicInput{}, ErrTooLarge
	}
	var in PublicInput
	if json.Unmarshal(raw, &in) != nil || !canonicalID(in.SnapshotID) || !canonicalID(in.WorkspaceID) || !canonicalID(in.ReportIssueID) || !optionalID(in.ProjectID) || !optionalID(in.CorrectionOf) || in.CorrectionOf != nil && *in.CorrectionOf == in.SnapshotID || in.ReportRevision < 1 || in.TimeBasis == nil {
		return PublicInput{}, ErrInvalid
	}
	in.AsOf, in.Window.Start, in.Window.End = in.AsOf.UTC(), in.Window.Start.UTC(), in.Window.End.UTC()
	if err := validateResolvedWindow(in.Window, in.AsOf, *in.TimeBasis); err != nil {
		return PublicInput{}, err
	}
	f := &in.Facts
	f.Issues = append([]PublicIssueFact{}, f.Issues...)
	sort.Slice(f.Issues, func(i, j int) bool { return f.Issues[i].ID < f.Issues[j].ID })
	issues := map[string]PublicIssueFact{}
	categories := map[string]string{}
	reportFound := false
	for _, issue := range f.Issues {
		if !canonicalID(issue.ID) || !sameID(issue.ProjectID, in.ProjectID) || issue.Revision < 1 || issue.Status == "" || !safeText(issue.Status, 128) || issue.Category != "" && !validCategory(issue.Category) || !timestampValid(issue.CreatedAt) || issue.CreatedAt.After(in.AsOf) || !dateValid(issue.DueDate) {
			return PublicInput{}, ErrInvalid
		}
		if _, exists := issues[issue.ID]; exists {
			return PublicInput{}, fmt.Errorf("%w: duplicate public issue", ErrInvalid)
		}
		if category, exists := categories[issue.Status]; exists && category != issue.Category {
			return PublicInput{}, fmt.Errorf("%w: inconsistent public category", ErrInvalid)
		}
		categories[issue.Status] = issue.Category
		switch issue.AdmissionStatus {
		case "not_required", "accepted", "pending", "rejected", "duplicate":
		default:
			return PublicInput{}, ErrInvalid
		}
		if !publicClassificationValid(issue) || issue.EnvironmentUnavailable != nil && *issue.EnvironmentUnavailable && (issue.AssignmentValid == nil || !*issue.AssignmentValid) {
			return PublicInput{}, ErrInvalid
		}
		if !safeText(issue.BaselineStatus, 128) || issue.BaselineCategory != "" && !validCategory(issue.BaselineCategory) || issue.BaselineAt != nil && !timestampValid(*issue.BaselineAt) {
			return PublicInput{}, ErrInvalid
		}
		if issue.ID == in.ReportIssueID {
			if issue.Revision != in.ReportRevision || issue.Classification != "report" || issue.ClassificationReason != "current_report" {
				return PublicInput{}, ErrInvalid
			}
			reportFound = true
		} else if in.ProjectID == nil || issue.ClassificationReason == "current_report" {
			return PublicInput{}, ErrInvalid
		}
		if f.Project == nil && (issue.Category != "" || issue.AssignmentValid != nil || issue.EnvironmentUnavailable != nil) {
			return PublicInput{}, ErrInvalid
		}
		issues[issue.ID] = issue
	}
	if !reportFound {
		return PublicInput{}, fmt.Errorf("%w: current public report missing", ErrInvalid)
	}
	if p := f.Project; p != nil {
		if in.ProjectID == nil || p.ID != *in.ProjectID || p.Revision < 1 || !dateValid(p.DueDate) || !contains([]string{"planned", "in_progress", "paused", "completed", "cancelled"}, p.Status) {
			return PublicInput{}, ErrInvalid
		}
		if p.InProgressSince != nil && !timestampValid(*p.InProgressSince) || p.LatestUpdateAt != nil && !timestampValid(*p.LatestUpdateAt) {
			return PublicInput{}, ErrInvalid
		}
		if (p.LatestUpdateAt == nil) != (in.TimeBasis.LatestUpdateOffsetSeconds == nil) || (p.InProgressSince == nil) != (in.TimeBasis.InProgressSinceOffsetSeconds == nil) {
			return PublicInput{}, ErrInvalid
		}
	} else if in.TimeBasis.LatestUpdateOffsetSeconds != nil || in.TimeBasis.InProgressSinceOffsetSeconds != nil {
		return PublicInput{}, ErrInvalid
	}
	for _, offset := range []*int{in.TimeBasis.LatestUpdateOffsetSeconds, in.TimeBasis.InProgressSinceOffsetSeconds} {
		if offset != nil && !offsetValid(*offset) {
			return PublicInput{}, ErrInvalid
		}
	}
	f.Activities = append([]PublicActivity{}, f.Activities...)
	sort.Slice(f.Activities, func(i, j int) bool {
		if f.Activities[i].IssueID != f.Activities[j].IssueID {
			return f.Activities[i].IssueID < f.Activities[j].IssueID
		}
		return f.Activities[i].Sequence < f.Activities[j].Sequence
	})
	for i, event := range f.Activities {
		issue, exists := issues[event.IssueID]
		if !exists || event.FactID != fmt.Sprintf("event-%d", i+1) || !issue.HistoryComplete || !timestampValid(event.BusinessAt) || event.BusinessAt.After(in.AsOf) || event.Sequence < 1 || !sameID(event.ProjectID, in.ProjectID) || event.FromStatus == "" || event.ToStatus == "" || !safeText(event.FromStatus, 128) || !safeText(event.ToStatus, 128) || !validCategory(event.FromCategory) || !validCategory(event.ToCategory) {
			return PublicInput{}, ErrInvalid
		}
		if i > 0 && f.Activities[i-1].IssueID == event.IssueID && f.Activities[i-1].Sequence == event.Sequence {
			return PublicInput{}, fmt.Errorf("%w: duplicate public event", ErrInvalid)
		}
	}
	f.Sources = append([]PublicSource{}, f.Sources...)
	sort.Slice(f.Sources, func(i, j int) bool { return f.Sources[i].Kind < f.Sources[j].Kind })
	for i, source := range f.Sources {
		if !publicSourceKind(source.Kind) || source.Version != publicSourceVersion || source.ProofVisibility != "internal" && source.ProofVisibility != "unavailable" || i > 0 && f.Sources[i-1].Kind == source.Kind {
			return PublicInput{}, ErrInvalid
		}
	}
	f.Gaps = append([]Gap{}, f.Gaps...)
	for i := range f.Gaps {
		gap := &f.Gaps[i]
		if gap.Source != "" && !publicGapSource(gap.Source) || publicGapCode(gap.Code) != gap.Code {
			return PublicInput{}, ErrInvalid
		}
		gap.MetricIDs, gap.IssueIDs = normalizeStrings(gap.MetricIDs), normalizeStrings(gap.IssueIDs)
		for _, id := range gap.IssueIDs {
			if _, exists := issues[id]; !exists {
				return PublicInput{}, ErrInvalid
			}
		}
		for _, id := range gap.MetricIDs {
			if !publicMetricID(id) {
				return PublicInput{}, ErrInvalid
			}
		}
	}
	sort.Slice(f.Gaps, func(i, j int) bool {
		a, _ := CanonicalJSON(f.Gaps[i])
		b, _ := CanonicalJSON(f.Gaps[j])
		return bytes.Compare(a, b) < 0
	})
	if len(f.Gaps) > 0 {
		gaps := f.Gaps[:1]
		for _, gap := range f.Gaps[1:] {
			if !reflect.DeepEqual(gap, gaps[len(gaps)-1]) {
				gaps = append(gaps, gap)
			}
		}
		f.Gaps = gaps
	}
	return in, nil
}

func publicGapSource(source string) bool { return source == "calendar" || publicSourceKind(source) }

func publicClassificationValid(issue PublicIssueFact) bool {
	switch issue.Classification {
	case "report":
		return issue.ClassificationReason == "current_report" || issue.ClassificationReason == "builtin_report" || issue.ClassificationReason == "verified_report"
	case "business":
		return issue.ClassificationReason == "verified_business"
	case "unknown":
		return issue.ClassificationReason == "classification_unproven"
	}
	return false
}

func publicSourceKind(kind string) bool {
	switch kind {
	case "issues", "status_catalog", "references", "project", "updates", "activities":
		return true
	}
	return false
}

func publicMetricID(id string) bool {
	if id == "*" {
		return true
	}
	prefix, name, ok := strings.Cut(id, ".")
	if !ok {
		return false
	}
	switch prefix {
	case "stock", "formal_stock":
		return contains([]string{"total", "completed", "cancelled", "open", "unknown_status", "blocked", "overdue", "unassigned", "in_review", "risk_union", "execution_environment_unavailable", "closure_ratio"}, name)
	case "window":
		return contains([]string{"created", "completed", "cancelled", "closed", "reopened", "end_open", "end_in_progress", "end_blocked", "net_created_minus_closed"}, name)
	case "project":
		return contains([]string{"overdue", "lead_valid", "latest_update_at", "progress_age_days", "health"}, name)
	}
	return false
}
