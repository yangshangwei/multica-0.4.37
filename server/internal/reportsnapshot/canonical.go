package reportsnapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/projecthealth"
)

func CanonicalJSON(v any) ([]byte, error) { return projecthealth.CanonicalJSON(v) }
func ptr[T any](v T) *T                   { return &v }
func digest(v any) (string, error) {
	raw, e := CanonicalJSON(v)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func canonicalID(s string) bool {
	id, e := uuid.Parse(s)
	return e == nil && id != uuid.Nil && id.String() == s
}
func optionalID(s *string) bool { return s == nil || canonicalID(*s) }
func sameID(a, b *string) bool  { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func safeText(s string, limit int) bool {
	if len(s) > limit {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func dateValid(s *string) bool {
	if s == nil {
		return true
	}
	t, e := time.Parse(time.DateOnly, *s)
	return e == nil && t.Format(time.DateOnly) == *s
}
func validCategory(s string) bool {
	switch s {
	case "backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled":
		return true
	}
	return false
}
func normalizeStrings(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return unique(out)
}
func unique(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := in[:1]
	for _, v := range in[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

// Build validates and copies trusted collector input, resolves calendar facts
// once, computes results, and seals all three content identities.
func Build(input Input) (Snapshot, error) {
	in, e := normalize(input)
	if e != nil {
		return Snapshot{}, e
	}
	metrics, e := computeMetrics(in)
	if e != nil {
		return Snapshot{}, e
	}
	complete := true
	for _, m := range metrics {
		if m.Quality != "exact" && m.Quality != "not_applicable" {
			complete = false
		}
	}
	state := "sealed_complete"
	if !complete {
		state = "sealed_partial"
	}
	out := Snapshot{FormalScope: "ordinary_formal_as_of", Input: in, ProtocolVersion: ProtocolVersion, SchemaVersion: SchemaVersion, SemanticsVersion: SemanticsVersion, CoverageVersion: CoverageVersion, CollectorVersion: CollectorVersion, TimeRulesVersion: TimeRulesVersion, ReferenceDate: in.TimeBasis.ReferenceDate, State: state, Complete: complete, Metrics: metrics}
	out.FactsDigest, e = digest(in.Facts)
	if e != nil {
		return Snapshot{}, e
	}
	out.ResultDigest, e = digest(struct {
		WorkspaceID      string     `json:"workspace_id"`
		ProjectID        *string    `json:"project_id"`
		ReportIssueID    string     `json:"report_issue_id"`
		Window           Window     `json:"window"`
		AsOf             time.Time  `json:"as_of"`
		TimeBasis        *TimeBasis `json:"time_basis"`
		SemanticsVersion string     `json:"semantics_version"`
		CoverageVersion  string     `json:"coverage_version"`
		Metrics          []Metric   `json:"metrics"`
		Gaps             []Gap      `json:"gaps"`
	}{in.WorkspaceID, in.ProjectID, in.ReportIssueID, in.Window, in.AsOf, in.TimeBasis, SemanticsVersion, CoverageVersion, metrics, in.Facts.Gaps})
	if e != nil {
		return Snapshot{}, e
	}
	out.SnapshotDigest, e = digest(out)
	if e != nil {
		return Snapshot{}, e
	}
	raw, e := CanonicalJSON(out)
	if e != nil {
		return Snapshot{}, e
	}
	if len(raw) > MaxSnapshotBytes {
		return Snapshot{}, ErrTooLarge
	}
	return out, nil
}

// Validate replays only retained facts. It performs no source reads and uses
// frozen calendar offsets rather than today's timezone rules.
func Validate(s Snapshot) error {
	if s.FormalScope != "ordinary_formal_as_of" || s.ProtocolVersion != ProtocolVersion || s.SchemaVersion != SchemaVersion || s.SemanticsVersion != SemanticsVersion || s.CoverageVersion != CoverageVersion || s.CollectorVersion != CollectorVersion || s.TimeRulesVersion != TimeRulesVersion {
		return ErrUnsupported
	}
	if s.TimeBasis == nil {
		return ErrInvalid
	}
	expected, e := Build(s.Input)
	if e != nil {
		return e
	}
	a, e := CanonicalJSON(s)
	if e != nil {
		return ErrInvalid
	}
	b, e := CanonicalJSON(expected)
	if e != nil || !bytes.Equal(a, b) {
		return fmt.Errorf("%w: inconsistent sealed content", ErrInvalid)
	}
	return nil
}

// Decode rejects duplicate/case-alias/unknown keys and trailing documents before
// validating a supported immutable snapshot. Extra future schema is not guessed.
func Decode(raw []byte) (Snapshot, error) {
	if len(raw) > MaxSnapshotBytes {
		return Snapshot{}, ErrTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if e := checkJSONValue(decoder); e != nil {
		return Snapshot{}, ErrInvalid
	}
	if _, e := decoder.Token(); e != io.EOF {
		return Snapshot{}, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var out Snapshot
	if e := d.Decode(&out); e != nil {
		return Snapshot{}, fmt.Errorf("%w: JSON schema", ErrInvalid)
	}
	// encoding/json otherwise accepts case-insensitive aliases for struct fields.
	var incoming any
	n := json.NewDecoder(bytes.NewReader(raw))
	n.UseNumber()
	if e := n.Decode(&incoming); e != nil {
		return Snapshot{}, ErrInvalid
	}
	encoded, e := CanonicalJSON(out)
	if e != nil {
		return Snapshot{}, ErrInvalid
	}
	var encodedValue any
	n = json.NewDecoder(bytes.NewReader(encoded))
	n.UseNumber()
	if e = n.Decode(&encodedValue); e != nil {
		return Snapshot{}, ErrInvalid
	}
	if !reflect.DeepEqual(incoming, encodedValue) {
		return Snapshot{}, fmt.Errorf("%w: noncanonical field shape", ErrInvalid)
	}
	if e = Validate(out); e != nil {
		return Snapshot{}, e
	}
	return out, nil
}

func checkJSONValue(d *json.Decoder) error {
	token, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return ErrInvalid
			}
			seen[s] = true
			if e = checkJSONValue(d); e != nil {
				return e
			}
		}
		end, e := d.Token()
		if e != nil || end != json.Delim('}') {
			return ErrInvalid
		}
	case '[':
		for d.More() {
			if e = checkJSONValue(d); e != nil {
				return e
			}
		}
		end, e := d.Token()
		if e != nil || end != json.Delim(']') {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func normalize(input Input) (Input, error) {
	if len(input.Facts.Issues) > MaxIssues+1 || len(input.Facts.Activities) > MaxActivities {
		return Input{}, ErrTooLarge
	}
	raw, e := CanonicalJSON(input)
	if e != nil {
		return Input{}, ErrInvalid
	}
	if len(raw) > MaxSnapshotBytes {
		return Input{}, ErrTooLarge
	}
	var in Input
	if e = json.Unmarshal(raw, &in); e != nil {
		return Input{}, ErrInvalid
	}
	if !canonicalID(in.SnapshotID) || !canonicalID(in.WorkspaceID) || !canonicalID(in.ReportIssueID) || !canonicalID(in.CreatedBy) || !canonicalID(in.RequestKey) || !optionalID(in.ProjectID) || !optionalID(in.CorrectionOf) || in.CorrectionOf != nil && *in.CorrectionOf == in.SnapshotID || in.ReportRevision < 1 || in.Sequence < 1 || in.ReadCut == "" || !safeText(in.ReadCut, 2048) {
		return Input{}, ErrInvalid
	}
	in.AsOf = in.AsOf.UTC()
	in.Window.Start = in.Window.Start.UTC()
	in.Window.End = in.Window.End.UTC()
	if in.TimeBasis == nil {
		in.TimeBasis, e = resolveTimeBasis(in)
		if e != nil {
			return Input{}, e
		}
	}
	if e = validateResolvedWindow(in.Window, in.AsOf, *in.TimeBasis); e != nil {
		return Input{}, e
	}
	if e = normalizeFacts(&in); e != nil {
		return Input{}, e
	}
	for i := range in.Facts.Sources {
		s := &in.Facts.Sources[i]
		s.ContentDigest = ""
		var payload any
		switch s.Kind {
		case "issues":
			payload = in.Facts.Issues
		case "activities":
			payload = in.Facts.Activities
		case "project":
			if in.Facts.Health != nil {
				payload = in.Facts.Health.Project
			}
		case "status_catalog":
			if in.Facts.Health != nil {
				payload = in.Facts.Health.Statuses
			}
		case "references":
			if in.Facts.Health != nil {
				payload = in.Facts.Health.References
			}
		case "updates":
			if in.Facts.Health != nil {
				payload = in.Facts.Health.LatestUpdateAt
			}
		default:
			return Input{}, ErrInvalid
		}
		s.ContentDigest, e = digest(struct {
			Source  SourceFact `json:"source"`
			Payload any        `json:"payload"`
		}{*s, payload})
		if e != nil {
			return Input{}, e
		}
	}
	return in, nil
}

func normalizeFacts(in *Input) error {
	f := &in.Facts
	f.Issues = append([]IssueFact{}, f.Issues...)
	sort.Slice(f.Issues, func(i, j int) bool { return f.Issues[i].ID < f.Issues[j].ID })
	issues := map[string]IssueFact{}
	out := f.Issues[:0]
	reportFound := false
	wanted := map[string]bool{}
	for _, i := range f.Issues {
		if !canonicalID(i.ID) || !optionalID(i.ProjectID) || !sameID(i.ProjectID, in.ProjectID) || i.Revision < 1 || !timestampValid(i.CreatedAt) || i.CreatedAt.After(in.AsOf) || !dateValid(i.DueDate) || !optionalID(i.OriginID) || !safeText(i.Status, 128) || i.Status == "" || !safeText(i.OriginType, 128) || !safeText(i.ClassificationReason, 256) {
			return ErrInvalid
		}
		if i.AssigneeID != "" && !canonicalID(i.AssigneeID) || i.AssigneeType != "" && i.AssigneeType != "member" && i.AssigneeType != "agent" && i.AssigneeType != "squad" {
			return ErrInvalid
		}
		if !safeText(i.BaselineStatus, 128) || !safeText(i.BaselineCategory, 128) {
			return ErrInvalid
		}
		switch i.AdmissionStatus {
		case "not_required", "pending", "accepted", "rejected", "duplicate":
		default:
			return ErrInvalid
		}
		if i.Classification != "unknown" && i.ClassificationReason == "" {
			return ErrInvalid
		}
		if i.Classification != "business" && i.Classification != "report" && i.Classification != "unknown" {
			return ErrInvalid
		}
		if i.ID == in.ReportIssueID {
			if i.Revision != in.ReportRevision {
				return ErrInvalid
			}
			i.Classification = "report"
			i.ClassificationReason = "current_report"
			reportFound = true
		}
		if in.ProjectID == nil && i.ID != in.ReportIssueID {
			return ErrInvalid
		}
		i.CreatedAt = i.CreatedAt.UTC()
		if i.BaselineAt != nil {
			if !timestampValid(*i.BaselineAt) {
				return ErrInvalid
			}
			i.BaselineAt = copyTime(i.BaselineAt)
		}
		if old, ok := issues[i.ID]; ok {
			if !reflect.DeepEqual(old, i) {
				return fmt.Errorf("%w: conflicting issue", ErrInvalid)
			}
			continue
		}
		issues[i.ID] = i
		out = append(out, i)
		if i.AssigneeType != "" && i.AssigneeID != "" {
			wanted[i.AssigneeType+":"+i.AssigneeID] = true
		}
	}
	f.Issues = out
	if !reportFound {
		return fmt.Errorf("%w: current report missing", ErrInvalid)
	}
	if f.Health != nil {
		h := f.Health
		p := h.Project
		if in.ProjectID == nil || p.ID != *in.ProjectID || p.WorkspaceID != in.WorkspaceID || p.Revision < 1 || !dateValid(p.DueDate) || p.LeadID != "" && !canonicalID(p.LeadID) {
			return ErrInvalid
		}
		for _, t := range []*time.Time{p.InProgressSince, h.LatestUpdateAt} {
			if t != nil && !timestampValid(*t) {
				return ErrInvalid
			}
		}
		if (h.LatestUpdateAt == nil) != (in.TimeBasis.LatestUpdateOffsetSeconds == nil) || (p.InProgressSince == nil) != (in.TimeBasis.InProgressSinceOffsetSeconds == nil) {
			return ErrInvalid
		}
		for _, n := range []*int{in.TimeBasis.LatestUpdateOffsetSeconds, in.TimeBasis.InProgressSinceOffsetSeconds} {
			if n != nil && !offsetValid(*n) {
				return ErrInvalid
			}
		}
		h.Project.InProgressSince = copyTime(h.Project.InProgressSince)
		h.LatestUpdateAt = copyTime(h.LatestUpdateAt)
		if p.LeadType != "" && p.LeadID != "" {
			wanted[p.LeadType+":"+p.LeadID] = true
		}
		h.Statuses = append([]StatusFact{}, h.Statuses...)
		sort.Slice(h.Statuses, func(i, j int) bool { return h.Statuses[i].Key < h.Statuses[j].Key })
		seenStatus := map[string]bool{}
		for _, s := range h.Statuses {
			if s.Key == "" || !safeText(s.Key, 128) || !safeText(s.Category, 128) || seenStatus[s.Key] {
				return ErrInvalid
			}
			seenStatus[s.Key] = true
		}
		refs := []ReferenceFact{}
		seenRefs := map[string]bool{}
		for _, r := range h.References {
			key := r.Kind + ":" + r.ID
			if !wanted[key] {
				continue
			}
			if !canonicalID(r.ID) || seenRefs[key] {
				return ErrInvalid
			}
			seenRefs[key] = true
			refs = append(refs, r)
		}
		sort.Slice(refs, func(i, j int) bool { return refs[i].Kind+refs[i].ID < refs[j].Kind+refs[j].ID })
		h.References = refs
	} else if in.TimeBasis.LatestUpdateOffsetSeconds != nil || in.TimeBasis.InProgressSinceOffsetSeconds != nil {
		return ErrInvalid
	}
	acts := map[string]ActivityFact{}
	f.Activities = append([]ActivityFact{}, f.Activities...)
	outActs := []ActivityFact{}
	for _, a := range f.Activities {
		if !canonicalID(a.ID) || !canonicalID(a.IssueID) || !timestampValid(a.RecordedAt) || !optionalID(a.ProjectID) || !safeText(a.Action, 128) || !safeText(a.SourceVersion, 128) || a.SourceVersion == "" {
			return ErrInvalid
		}
		if a.ProjectID != nil && !sameID(a.ProjectID, in.ProjectID) {
			return ErrInvalid
		}
		if _, ok := issues[a.IssueID]; !ok {
			return ErrInvalid
		}
		if a.BusinessAt != nil {
			if !timestampValid(*a.BusinessAt) {
				return ErrInvalid
			}
			a.BusinessAt = copyTime(a.BusinessAt)
		}
		if a.Sequence != nil && *a.Sequence < 1 {
			return ErrInvalid
		}
		a.RecordedAt = a.RecordedAt.UTC()
		for _, s := range []string{a.FromStatus, a.ToStatus, a.FromCategory, a.ToCategory} {
			if !safeText(s, 128) {
				return ErrInvalid
			}
		}
		if old, ok := acts[a.ID]; ok {
			if !reflect.DeepEqual(old, a) {
				return fmt.Errorf("%w: conflicting activity", ErrInvalid)
			}
			continue
		}
		acts[a.ID] = a
		outActs = append(outActs, a)
	}
	sort.Slice(outActs, func(i, j int) bool {
		a, b := outActs[i], outActs[j]
		if !a.RecordedAt.Equal(b.RecordedAt) {
			return a.RecordedAt.Before(b.RecordedAt)
		}
		return a.ID < b.ID
	})
	f.Activities = outActs
	f.Sources = append([]SourceFact{}, f.Sources...)
	sort.Slice(f.Sources, func(i, j int) bool { return f.Sources[i].Kind < f.Sources[j].Kind })
	seenSources := map[string]bool{}
	for j := range f.Sources {
		s := &f.Sources[j]
		if seenSources[s.Kind] || s.Version == "" || !safeText(s.Version, 128) || !safeText(s.Ordering, 256) || s.Count != nil && *s.Count < 0 {
			return ErrInvalid
		}
		seenSources[s.Kind] = true
		for _, v := range []*string{s.Watermark, s.Cursor} {
			if v != nil && !safeText(*v, 2048) {
				return ErrInvalid
			}
		}
		s.Filters = normalizeStrings(s.Filters)
		for _, v := range s.Filters {
			if !safeText(v, 256) {
				return ErrInvalid
			}
		}
		s.Identities = append([]SourceIdentity{}, s.Identities...)
		sort.Slice(s.Identities, func(i, j int) bool { return s.Identities[i].ID < s.Identities[j].ID })
		for i, id := range s.Identities {
			if id.ID == "" || !safeText(id.ID, 256) || id.Revision == "" || !safeText(id.Revision, 256) || i > 0 && id.ID == s.Identities[i-1].ID {
				return ErrInvalid
			}
		}
		if s.Kind == "issues" && s.ReadComplete && s.Count != nil && *s.Count != int64(len(f.Issues)) {
			s.ReadComplete = false
			f.Gaps = append(f.Gaps, Gap{Code: "source_count_mismatch", Source: "issues"})
		}
		if s.Kind == "activities" && s.ReadComplete && s.Count != nil && *s.Count != int64(len(f.Activities)) {
			s.ReadComplete = false
			f.Gaps = append(f.Gaps, Gap{Code: "source_count_mismatch", Source: "activities"})
		}
	}
	f.Gaps = append([]Gap{}, f.Gaps...)
	for j := range f.Gaps {
		g := &f.Gaps[j]
		if g.Code == "" || !safeText(g.Code, 128) || !safeText(g.Source, 128) {
			return ErrInvalid
		}
		g.MetricIDs = normalizeStrings(g.MetricIDs)
		g.IssueIDs = normalizeStrings(g.IssueIDs)
		for _, id := range g.IssueIDs {
			if _, ok := issues[id]; !ok {
				return ErrInvalid
			}
		}
		for _, id := range g.MetricIDs {
			if !safeText(id, 128) {
				return ErrInvalid
			}
		}
	}
	sort.Slice(f.Gaps, func(i, j int) bool {
		a, _ := CanonicalJSON(f.Gaps[i])
		b, _ := CanonicalJSON(f.Gaps[j])
		return strings.Compare(string(a), string(b)) < 0
	})
	if len(f.Gaps) > 0 {
		out := f.Gaps[:1]
		for _, g := range f.Gaps[1:] {
			if !reflect.DeepEqual(g, out[len(out)-1]) {
				out = append(out, g)
			}
		}
		f.Gaps = out
	}
	return nil
}
