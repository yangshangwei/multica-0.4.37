package service

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/reportsnapshot"
)

// Collection audit is private storage provenance. It never participates in the
// computational facts/result digests, so transport page sizes cannot change them.
type reportSnapshotAudit struct {
	Version         int                                 `json:"version"`
	CutRef          string                              `json:"cut_ref"`
	Sources         []reportSnapshotSourceAudit         `json:"sources"`
	Classifications []reportSnapshotClassificationProof `json:"classifications"`
	LatestUpdate    *reportSnapshotUpdateProof          `json:"latest_update"`
}

type reportSnapshotSourceAudit struct {
	Kind         string                   `json:"kind"`
	Ordering     string                   `json:"ordering"`
	Termination  string                   `json:"termination"`
	Pages        []reportSnapshotReadPage `json:"pages"`
	RootBoundIDs []string                 `json:"root_bound_ids"`
}

type reportSnapshotReadPage struct {
	LowerExclusive string                   `json:"lower_exclusive"`
	UpperInclusive string                   `json:"upper_inclusive"`
	RequestedLimit int64                    `json:"requested_limit"`
	ObservedCount  int64                    `json:"observed_count"`
	RetainedCount  int64                    `json:"retained_count"`
	Exhausted      bool                     `json:"exhausted"`
	Rows           []reportSnapshotAuditRow `json:"rows"`
	PageDigest     string                   `json:"page_digest"`
}

type reportSnapshotAuditRow struct {
	Key           string `json:"key"`
	ID            string `json:"id"`
	Revision      string `json:"revision"`
	ContentDigest string `json:"content_digest"`
	Retained      bool   `json:"retained"`
}

type reportSnapshotClassificationProof struct {
	IssueID            string     `json:"issue_id"`
	IssueRevision      int64      `json:"issue_revision"`
	Kind               string     `json:"kind"`
	OriginType         string     `json:"origin_type"`
	OriginID           *string    `json:"origin_id"`
	SnapshotID         *string    `json:"snapshot_id"`
	SnapshotRevision   int64      `json:"snapshot_revision"`
	SnapshotSequence   int64      `json:"snapshot_sequence"`
	AutopilotID        *string    `json:"autopilot_id"`
	TemplateKey        string     `json:"template_key"`
	TemplateVersion    int32      `json:"template_version"`
	LinkedRunID        *string    `json:"linked_run_id"`
	LinkedRunCreatedAt *time.Time `json:"linked_run_created_at"`
}

type reportSnapshotUpdateProof struct {
	ID          string    `json:"id"`
	Revision    int64     `json:"revision"`
	PublishedAt time.Time `json:"published_at"`
}

func reportAuditRow(key, id, revision string, payload any, retained bool) (reportSnapshotAuditRow, error) {
	digest, err := reportDigest(payload)
	return reportSnapshotAuditRow{Key: key, ID: id, Revision: revision, ContentDigest: digest, Retained: retained}, err
}

func reportAuditPage(lower string, limit int, rows []reportSnapshotAuditRow, exhausted bool) (reportSnapshotReadPage, error) {
	page := reportSnapshotReadPage{LowerExclusive: lower, UpperInclusive: lower, RequestedLimit: int64(limit), ObservedCount: int64(len(rows)), Rows: rows, Exhausted: exhausted}
	for _, row := range rows {
		page.UpperInclusive = row.Key
		if row.Retained {
			page.RetainedCount++
		}
	}
	var err error
	page.PageDigest, err = reportDigest(rows)
	return page, err
}

func reportAuditDiscardPages(source *reportSnapshotSourceAudit) error {
	for i := range source.Pages {
		page := &source.Pages[i]
		page.RetainedCount = 0
		for j := range page.Rows {
			page.Rows[j].Retained = false
		}
		var err error
		page.PageDigest, err = reportDigest(page.Rows)
		if err != nil {
			return err
		}
	}
	return nil
}

func reportDigestValid(s string) bool {
	value, err := hex.DecodeString(s)
	return err == nil && len(value) == 32 && strings.ToLower(s) == s
}

func reportContentRevision(payload any) (string, error) {
	digest, err := reportDigest(payload)
	return "content:" + digest, err
}

// validateReportSnapshotAudit checks the retained page membership and source
// provenance against the sealed computational facts, not the current database.
func validateReportSnapshotAudit(a reportSnapshotAudit, in reportsnapshot.Input) error {
	invalid := func() error { return fmt.Errorf("invalid report collection audit") }
	if a.Version != 1 || a.CutRef == "" || a.CutRef != in.ReadCut || len(a.Sources) != len(in.Facts.Sources) {
		return invalid()
	}
	sources := map[string]reportsnapshot.SourceFact{}
	for _, source := range in.Facts.Sources {
		sources[source.Kind] = source
	}
	expected, err := reportAuditExpectedRows(in, a)
	if err != nil {
		return err
	}
	seenSources := map[string]bool{}
	for _, audit := range a.Sources {
		source, exists := sources[audit.Kind]
		if !exists || seenSources[audit.Kind] || audit.Ordering != source.Ordering {
			return invalid()
		}
		seenSources[audit.Kind] = true
		identities := map[string]string{}
		for _, identity := range source.Identities {
			identities[identity.ID] = identity.Revision
		}
		complete := audit.Termination == "eof" || audit.Termination == "selection_complete" || audit.Termination == "root_bound"
		if complete != source.ReadComplete {
			return invalid()
		}
		if complete && (source.Count == nil || *source.Count != int64(len(expected[audit.Kind]))) {
			return invalid()
		}
		if !complete && source.Count != nil {
			return invalid()
		}
		switch audit.Termination {
		case "eof", "selection_complete", "root_bound", "limit", "unavailable", "scope_incomplete":
		default:
			return invalid()
		}
		seenRows := map[string]bool{}
		previous := ""
		for pageIndex, page := range audit.Pages {
			if page.ObservedCount != int64(len(page.Rows)) || page.RequestedLimit < 0 || page.RequestedLimit > 0 && page.ObservedCount > page.RequestedLimit || pageIndex > 0 && page.LowerExclusive != previous || pageIndex+1 < len(audit.Pages) && page.Exhausted {
				return invalid()
			}
			digest, err := reportDigest(page.Rows)
			if err != nil || digest != page.PageDigest {
				return invalid()
			}
			last, retained := page.LowerExclusive, int64(0)
			for _, row := range page.Rows {
				if row.Key <= last || row.ID == "" || row.Revision == "" || !reportDigestValid(row.ContentDigest) || seenRows[row.ID] {
					return invalid()
				}
				last, seenRows[row.ID] = row.Key, true
				if row.Retained {
					lookup := row.ID
					if audit.Kind == "status_catalog" {
						lookup = row.Key
					}
					wanted, exists := expected[audit.Kind][lookup]
					if !exists || wanted.Key != row.Key || wanted.ContentDigest != row.ContentDigest || wanted.Revision != row.Revision || identities[row.ID] != row.Revision {
						return invalid()
					}
					delete(expected[audit.Kind], lookup)
					retained++
				}
			}
			if last != page.UpperInclusive || retained != page.RetainedCount || page.RequestedLimit > 0 && page.Exhausted != (page.ObservedCount < page.RequestedLimit) {
				return invalid()
			}
			previous = page.UpperInclusive
		}
		if audit.Termination == "eof" && (len(audit.Pages) == 0 || !audit.Pages[len(audit.Pages)-1].Exhausted) {
			return invalid()
		}
		if audit.Termination == "selection_complete" || audit.Termination == "root_bound" {
			if len(audit.Pages) != 1 || audit.Pages[0].RequestedLimit != 0 || !audit.Pages[0].Exhausted {
				return invalid()
			}
		}
		for _, id := range audit.RootBoundIDs {
			if audit.Kind != "issues" || id != in.ReportIssueID || audit.Termination == "eof" {
				return invalid()
			}
			if row, exists := expected[audit.Kind][id]; !exists || identities[id] != row.Revision {
				return invalid()
			}
			delete(expected[audit.Kind], id)
		}
		if len(expected[audit.Kind]) != 0 {
			return invalid()
		}
	}
	return nil
}

func reportAuditExpectedRows(in reportsnapshot.Input, a reportSnapshotAudit) (map[string]map[string]reportSnapshotAuditRow, error) {
	out := map[string]map[string]reportSnapshotAuditRow{}
	for _, source := range in.Facts.Sources {
		out[source.Kind] = map[string]reportSnapshotAuditRow{}
	}
	add := func(kind, key, id, revision string, payload any) error {
		if out[kind] == nil {
			return fmt.Errorf("missing audited source")
		}
		row, err := reportAuditRow(key, id, revision, payload, true)
		if err == nil {
			out[kind][id] = row
		}
		return err
	}
	proofs := map[string]reportSnapshotClassificationProof{}
	classificationRevisions := map[string]string{}
	for _, source := range in.Facts.Sources {
		if source.Kind == "issues" {
			for _, identity := range source.Identities {
				classificationRevisions[identity.ID] = identity.Revision
			}
		}
	}
	for _, proof := range a.Classifications {
		if _, exists := proofs[proof.IssueID]; exists {
			return nil, fmt.Errorf("duplicate report classification proof")
		}
		proofs[proof.IssueID] = proof
	}
	for _, issue := range in.Facts.Issues {
		proof, exists := proofs[issue.ID]
		if !exists || proof.IssueRevision != issue.Revision || proof.OriginType != issue.OriginType || !reportSameOptionalID(proof.OriginID, issue.OriginID) {
			return nil, fmt.Errorf("missing report classification proof")
		}
		proofRevision, err := reportContentRevision(proof)
		if err != nil {
			return nil, err
		}
		if classificationRevisions["classification:"+issue.ID] != proofRevision {
			return nil, fmt.Errorf("classification source version mismatch")
		}
		if issue.Classification == "report" {
			switch issue.ClassificationReason {
			case "current_report":
				if proof.Kind != "current_report" || issue.ID != in.ReportIssueID {
					return nil, fmt.Errorf("invalid current report proof")
				}
			case "sealed_report_identity":
				if proof.Kind != "sealed_report" || proof.SnapshotID == nil || !CanonicalReportSnapshotUUID(*proof.SnapshotID) || proof.SnapshotRevision < 1 || proof.SnapshotSequence < 1 {
					return nil, fmt.Errorf("invalid retained report proof")
				}
			default:
				if proof.Kind != "builtin_report" || proof.OriginType != "autopilot" || proof.OriginID == nil || !reportSameOptionalID(proof.OriginID, proof.AutopilotID) || proof.LinkedRunID == nil || !CanonicalReportSnapshotUUID(*proof.LinkedRunID) || proof.LinkedRunCreatedAt == nil || proof.TemplateVersion < 1 || proof.TemplateKey != "daily-progress-report" && proof.TemplateKey != "weekly-progress-report" || issue.ClassificationReason != "builtin_"+proof.TemplateKey+"_v"+strconv.Itoa(int(proof.TemplateVersion)) {
					return nil, fmt.Errorf("invalid builtin report proof")
				}
			}
		} else if issue.Classification != "unknown" || proof.Kind != "unknown" {
			return nil, fmt.Errorf("unproven business classification")
		}
		delete(proofs, issue.ID)
		if err := add("issues", issue.ID, issue.ID, strconv.FormatInt(issue.Revision, 10), issue); err != nil {
			return nil, err
		}
	}
	if len(proofs) != 0 {
		return nil, fmt.Errorf("out-of-scope report classification proof")
	}
	for _, activity := range in.Facts.Activities {
		revision, err := reportContentRevision(activity)
		if err != nil {
			return nil, err
		}
		if err := add("activities", reportActivityKey(activity.RecordedAt, activity.ID), activity.ID, revision, activity); err != nil {
			return nil, err
		}
	}
	if h := in.Facts.Health; h != nil {
		if err := add("project", h.Project.ID, h.Project.ID, strconv.FormatInt(h.Project.Revision, 10), h.Project); err != nil {
			return nil, err
		}
		for _, status := range h.Statuses {
			revision, err := reportContentRevision(status)
			if err != nil {
				return nil, err
			}
			if err := add("status_catalog", status.Key, status.Key, revision, status); err != nil {
				return nil, err
			}
		}
		for _, ref := range h.References {
			revision, err := reportContentRevision(ref)
			if err != nil {
				return nil, err
			}
			id := ref.Kind + ":" + ref.ID
			if err := add("references", id, id, revision, ref); err != nil {
				return nil, err
			}
		}
		if a.LatestUpdate != nil {
			update := a.LatestUpdate
			if h.LatestUpdateAt == nil || !update.PublishedAt.Equal(*h.LatestUpdateAt) || !CanonicalReportSnapshotUUID(update.ID) || update.Revision < 1 {
				return nil, fmt.Errorf("invalid latest update identity")
			}
			if err := add("updates", update.ID, update.ID, strconv.FormatInt(update.Revision, 10), update); err != nil {
				return nil, err
			}
		} else if h.LatestUpdateAt != nil {
			return nil, fmt.Errorf("missing latest update identity")
		}
	} else if a.LatestUpdate != nil {
		return nil, fmt.Errorf("unexpected update identity")
	}
	return out, nil
}

func reportActivityKey(at time.Time, id string) string {
	return at.UTC().Format("2006-01-02T15:04:05.000000Z") + "/" + id
}

func reportSameOptionalID(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
