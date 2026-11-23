package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/projecthealth"
	"github.com/multica-ai/multica/server/internal/reportsnapshot"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// optionalReportSource preserves the RR cut after a source-specific SQL failure.
// A broken connection, cancellation, or transaction conflict cannot be sealed.
func optionalReportSource(ctx context.Context, tx pgx.Tx, read func() error) (bool, error) {
	if _, err := tx.Exec(ctx, "SAVEPOINT report_snapshot_source"); err != nil {
		return false, err
	}
	err := read()
	if err == nil {
		_, err = tx.Exec(ctx, "RELEASE SAVEPOINT report_snapshot_source")
		return true, err
	}
	if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT report_snapshot_source"); rollbackErr != nil {
		return false, rollbackErr
	}
	if _, releaseErr := tx.Exec(ctx, "RELEASE SAVEPOINT report_snapshot_source"); releaseErr != nil {
		return false, releaseErr
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		if strings.HasPrefix(pgerr.Code, "08") || strings.HasPrefix(pgerr.Code, "57") || pgerr.Code == "40001" || pgerr.Code == "40P01" || pgerr.Code == "55P03" {
			return false, err
		}
		return false, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return false, err
}
func reportCount(n int) *int64 { v := int64(n); return &v }
func reportDate(v pgtype.Date) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.Format(time.DateOnly)
	return &s
}
func reportText(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
func reportID(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return v.String()
}

type reportSnapshotCollector struct {
	ctx      context.Context
	tx       pgx.Tx
	q        *db.Queries
	report   db.Issue
	project  *db.Project
	asOf     time.Time
	timezone string
	facts    reportsnapshot.Facts
	audit    reportSnapshotAudit
}

func collectReportSnapshotFacts(ctx context.Context, tx pgx.Tx, report db.Issue, project *db.Project, asOf time.Time, timezone, cutRef string) (reportsnapshot.Facts, reportSnapshotAudit, error) {
	c := reportSnapshotCollector{ctx: ctx, tx: tx, q: db.New(tx), report: report, project: project, asOf: asOf, timezone: timezone, facts: reportsnapshot.Facts{Issues: []reportsnapshot.IssueFact{}, Activities: []reportsnapshot.ActivityFact{}, Sources: []reportsnapshot.SourceFact{}, Gaps: []reportsnapshot.Gap{}}, audit: reportSnapshotAudit{Version: 1, CutRef: cutRef, Sources: []reportSnapshotSourceAudit{}, Classifications: []reportSnapshotClassificationProof{}}}
	for _, collect := range []func() error{c.collectProject, c.collectIssues, c.collectStatuses, c.collectReferences, c.collectLatestUpdate, c.collectActivities} {
		if err := collect(); err != nil {
			return reportsnapshot.Facts{}, reportSnapshotAudit{}, err
		}
	}
	return c.facts, c.audit, nil
}

func (c *reportSnapshotCollector) gap(code, source string) {
	c.facts.Gaps = append(c.facts.Gaps, reportsnapshot.Gap{Code: code, Source: source, MetricIDs: []string{}, IssueIDs: []string{}})
}

func (c *reportSnapshotCollector) source(kind, version, ordering string) (reportsnapshot.SourceFact, reportSnapshotSourceAudit) {
	filters := []string{"workspace_id=" + c.report.WorkspaceID.String(), "report_issue_id=" + c.report.ID.String(), "project_id=" + reportID(c.report.ProjectID)}
	return reportsnapshot.SourceFact{Kind: kind, Version: version, Ordering: ordering, Filters: filters, Identities: []reportsnapshot.SourceIdentity{}}, reportSnapshotSourceAudit{Kind: kind, Ordering: ordering, Pages: []reportSnapshotReadPage{}, RootBoundIDs: []string{}}
}

func (c *reportSnapshotCollector) retain(source reportsnapshot.SourceFact, audit reportSnapshotSourceAudit) {
	c.facts.Sources = append(c.facts.Sources, source)
	c.audit.Sources = append(c.audit.Sources, audit)
}

func (c *reportSnapshotCollector) collectProject() error {
	source, audit := c.source("project", "project-health-input-v1", "id ASC")
	rows := []reportSnapshotAuditRow{}
	if p := c.project; p != nil {
		in := projecthealth.Input{Project: projecthealth.Project{WorkspaceID: p.WorkspaceID.String(), ID: p.ID.String(), Revision: p.Revision, Status: p.Status, DueDate: reportDate(p.DueDate), LeadType: reportText(p.LeadType), LeadID: reportID(p.LeadID), InProgressSince: reportTimestamp(p.InProgressSince), InProgressSinceSource: reportText(p.InProgressSinceSource)}}
		health := reportsnapshot.FreezeHealth(in)
		c.facts.Health = &health
		revision := strconv.FormatInt(p.Revision, 10)
		source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: p.ID.String(), Revision: revision})
		row, err := reportAuditRow(p.ID.String(), p.ID.String(), revision, health.Project, true)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	page, err := reportAuditPage("", 0, rows, true)
	if err != nil {
		return err
	}
	audit.Pages, audit.Termination = append(audit.Pages, page), "root_bound"
	source.ReadComplete, source.ProducerComplete, source.Count = true, true, reportCount(len(rows))
	c.retain(source, audit)
	return nil
}

func (c *reportSnapshotCollector) collectIssues() error {
	source, audit := c.source("issues", "ordinary-formal-cohort-v1", "id ASC")
	source.Filters = append(source.Filters, "admission_status=not_required|accepted", "independent_collaboration=excluded")
	complete, foundReport := true, false
	ok, err := optionalReportSource(c.ctx, c.tx, func() error {
		after := pgtype.UUID{Valid: true}
		for {
			page, err := c.q.ListReportSnapshotSourceIssues(c.ctx, db.ListReportSnapshotSourceIssuesParams{WorkspaceID: c.report.WorkspaceID, ProjectID: c.report.ProjectID, ReportIssueID: c.report.ID, AfterID: after, RowLimit: 500})
			if err != nil {
				return err
			}
			proofRows := []reportSnapshotAuditRow{}
			for _, row := range page {
				fact, proof := reportIssueSource(row, c.report.ID)
				retained := len(c.facts.Issues) < reportsnapshot.MaxIssues
				auditRow, err := reportAuditRow(row.ID.String(), row.ID.String(), strconv.FormatInt(row.Revision, 10), fact, retained)
				if err != nil {
					return err
				}
				proofRows = append(proofRows, auditRow)
				if !retained {
					complete = false
					continue
				}
				c.facts.Issues = append(c.facts.Issues, fact)
				c.audit.Classifications = append(c.audit.Classifications, proof)
				foundReport = foundReport || row.ID == c.report.ID
			}
			receipt, err := reportAuditPage(after.String(), 500, proofRows, len(page) < 500)
			if err != nil {
				return err
			}
			audit.Pages = append(audit.Pages, receipt)
			if !complete || len(page) < 500 {
				break
			}
			after = page[len(page)-1].ID
		}
		return nil
	})
	if err != nil {
		return err
	}
	audit.Termination = "eof"
	if !ok {
		c.facts.Issues, c.audit.Classifications = []reportsnapshot.IssueFact{}, []reportSnapshotClassificationProof{}
		complete, foundReport, audit.Termination = false, false, "unavailable"
		if err := reportAuditDiscardPages(&audit); err != nil {
			return err
		}
		c.gap("source_unavailable", "issues")
	} else if !complete {
		audit.Termination = "limit"
		c.gap("scan_limit", "issues")
	}
	if !foundReport {
		if complete {
			return reportSnapshotUnavailable()
		}
		r := c.report
		fact := reportsnapshot.IssueFact{ID: r.ID.String(), Revision: r.Revision, ProjectID: reportUUIDPtr(r.ProjectID), Status: r.Status, AdmissionStatus: r.AdmissionStatus, CreatedAt: r.CreatedAt.Time.UTC(), DueDate: reportDate(r.DueDate), AssigneeType: reportText(r.AssigneeType), AssigneeID: reportID(r.AssigneeID), OriginType: reportText(r.OriginType), OriginID: reportUUIDPtr(r.OriginID), Classification: "report", ClassificationReason: "current_report"}
		c.facts.Issues = append(c.facts.Issues, fact)
		c.audit.Classifications = append(c.audit.Classifications, reportSnapshotClassificationProof{IssueID: fact.ID, IssueRevision: fact.Revision, Kind: "current_report", OriginType: fact.OriginType, OriginID: fact.OriginID})
		audit.RootBoundIDs = append(audit.RootBoundIDs, fact.ID)
	}
	for _, fact := range c.facts.Issues {
		source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: fact.ID, Revision: strconv.FormatInt(fact.Revision, 10)})
	}
	for _, proof := range c.audit.Classifications {
		revision, err := reportContentRevision(proof)
		if err != nil {
			return err
		}
		source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: "classification:" + proof.IssueID, Revision: revision})
	}
	source.ReadComplete, source.ProducerComplete = complete, true
	if complete {
		source.Count = reportCount(len(c.facts.Issues))
	}
	c.retain(source, audit)
	return nil
}

func reportIssueSource(row db.ListReportSnapshotSourceIssuesRow, currentReportID pgtype.UUID) (reportsnapshot.IssueFact, reportSnapshotClassificationProof) {
	proof := reportSnapshotClassificationProof{IssueID: row.ID.String(), IssueRevision: row.Revision, Kind: "unknown", OriginType: reportText(row.OriginType), OriginID: reportUUIDPtr(row.OriginID), SnapshotID: reportUUIDPtr(row.KnownReportSnapshotID), SnapshotRevision: row.KnownReportRevision, SnapshotSequence: row.KnownReportSequence, AutopilotID: reportUUIDPtr(row.AutopilotID), TemplateKey: row.TemplateKey, TemplateVersion: row.TemplateVersion, LinkedRunID: reportUUIDPtr(row.LinkedRunID), LinkedRunCreatedAt: reportTimestamp(row.LinkedRunCreatedAt)}
	classification, reason := "unknown", "report_classification_unproven"
	if row.ID == currentReportID {
		classification, reason, proof.Kind = "report", "current_report", "current_report"
		proof.SnapshotID, proof.SnapshotRevision, proof.SnapshotSequence = nil, 0, 0
		proof.AutopilotID, proof.TemplateKey, proof.TemplateVersion, proof.LinkedRunID, proof.LinkedRunCreatedAt = nil, "", 0, nil, nil
	} else if row.KnownReportSnapshotID.Valid && row.KnownReportRevision > 0 && row.KnownReportSequence > 0 {
		classification, reason, proof.Kind = "report", "sealed_report_identity", "sealed_report"
		proof.AutopilotID, proof.TemplateKey, proof.TemplateVersion, proof.LinkedRunID, proof.LinkedRunCreatedAt = nil, "", 0, nil, nil
	} else if row.OriginType.String == "autopilot" && row.OriginID.Valid && row.AutopilotID == row.OriginID && row.LinkedRunID.Valid && row.LinkedRunCreatedAt.Valid && row.TemplateVersion > 0 && (row.TemplateKey == "daily-progress-report" || row.TemplateKey == "weekly-progress-report") {
		classification, reason, proof.Kind = "report", "builtin_"+row.TemplateKey+"_v"+strconv.Itoa(int(row.TemplateVersion)), "builtin_report"
	}
	return reportsnapshot.IssueFact{ID: row.ID.String(), Revision: row.Revision, ProjectID: reportUUIDPtr(row.ProjectID), Status: row.Status, AdmissionStatus: row.AdmissionStatus, CreatedAt: row.CreatedAt.Time.UTC(), DueDate: reportDate(row.DueDate), AssigneeType: reportText(row.AssigneeType), AssigneeID: reportID(row.AssigneeID), OriginType: reportText(row.OriginType), OriginID: reportUUIDPtr(row.OriginID), Classification: classification, ClassificationReason: reason}, proof
}

func (c *reportSnapshotCollector) collectStatuses() error {
	source, audit := c.source("status_catalog", "project-health-input-v1", "key ASC")
	keys := []string{}
	for _, issue := range c.facts.Issues {
		keys = append(keys, issue.Status)
	}
	sort.Strings(keys)
	keys = compactReportStrings(keys)
	filter, err := reportDigest(keys)
	if err != nil {
		return err
	}
	source.Filters = append(source.Filters, "cohort_status_keys="+filter, "builtin_categories=project-health-v1")
	proofRows := []reportSnapshotAuditRow{}
	ok := true
	if c.project != nil {
		ok, err = optionalReportSource(c.ctx, c.tx, func() error {
			rows, err := c.q.ListReportSnapshotSourceStatuses(c.ctx, db.ListReportSnapshotSourceStatusesParams{WorkspaceID: c.report.WorkspaceID, StatusKeys: keys})
			if err != nil {
				return err
			}
			// Canonical order of the materialized key/category projection is
			// independent of the database's locale collation.
			sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
			for _, row := range rows {
				fact := reportsnapshot.StatusFact{Key: row.Key, Category: row.Category}
				revision, err := reportContentRevision(fact)
				if err != nil {
					return err
				}
				proof, err := reportAuditRow(row.Key, row.ID.String(), revision, fact, true)
				if err != nil {
					return err
				}
				c.facts.Health.Statuses = append(c.facts.Health.Statuses, fact)
				proofRows = append(proofRows, proof)
				source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: row.ID.String(), Revision: revision})
			}
			return nil
		})
	}
	if err != nil {
		return err
	}
	if !ok {
		c.facts.Health.Statuses = []reportsnapshot.StatusFact{}
		proofRows, source.Identities = []reportSnapshotAuditRow{}, []reportsnapshot.SourceIdentity{}
		c.gap("source_unavailable", source.Kind)
	}
	return c.retainProjectionSource(source, audit, proofRows, ok)
}

func (c *reportSnapshotCollector) collectReferences() error {
	source, audit := c.source("references", "project-health-input-v1", "kind,id ASC")
	source.Filters = append(source.Filters, "projection=associated-lead-and-assignees")
	proofRows := []reportSnapshotAuditRow{}
	ok, err := true, error(nil)
	if c.project != nil {
		ok, err = optionalReportSource(c.ctx, c.tx, func() error {
			p := c.facts.Health.Project
			in := projecthealth.Input{Project: projecthealth.Project{WorkspaceID: p.WorkspaceID, ID: p.ID, Revision: p.Revision, Status: p.Status, DueDate: p.DueDate, LeadType: p.LeadType, LeadID: p.LeadID, InProgressSince: p.InProgressSince, InProgressSinceSource: p.InProgressSinceSource}, Now: c.asOf, Timezone: &c.timezone}
			for _, status := range c.facts.Health.Statuses {
				in.Statuses = append(in.Statuses, projecthealth.Status{Key: status.Key, Category: status.Category})
			}
			for _, issue := range c.facts.Issues {
				in.Issues = append(in.Issues, projecthealth.Issue{ID: issue.ID, Status: issue.Status, DueDate: issue.DueDate, AssigneeType: issue.AssigneeType, AssigneeID: issue.AssigneeID})
			}
			refs, err := projecthealth.CollectReferences(c.ctx, c.q, *c.project, in)
			if err != nil {
				return err
			}
			sort.Slice(refs, func(i, j int) bool { return refs[i].Kind+":"+refs[i].ID < refs[j].Kind+":"+refs[j].ID })
			for _, ref := range refs {
				fact := reportsnapshot.ReferenceFact{Kind: ref.Kind, ID: ref.ID, Valid: ref.Valid, EnvironmentUnavailable: ref.EnvironmentUnavailable}
				revision, err := reportContentRevision(fact)
				if err != nil {
					return err
				}
				id := ref.Kind + ":" + ref.ID
				proof, err := reportAuditRow(id, id, revision, fact, true)
				if err != nil {
					return err
				}
				proofRows = append(proofRows, proof)
				c.facts.Health.References = append(c.facts.Health.References, fact)
				source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: id, Revision: revision})
			}
			return nil
		})
	}
	if err != nil {
		return err
	}
	if !ok {
		c.facts.Health.References = []reportsnapshot.ReferenceFact{}
		proofRows, source.Identities = []reportSnapshotAuditRow{}, []reportsnapshot.SourceIdentity{}
		c.gap("source_unavailable", source.Kind)
	}
	return c.retainProjectionSource(source, audit, proofRows, ok)
}

func (c *reportSnapshotCollector) collectLatestUpdate() error {
	source, audit := c.source("updates", "project-health-input-v1", "published_at DESC,id DESC")
	source.Filters = append(source.Filters, "selection=latest-published-update")
	proofRows := []reportSnapshotAuditRow{}
	ok, err := true, error(nil)
	if c.project != nil {
		ok, err = optionalReportSource(c.ctx, c.tx, func() error {
			row, err := c.q.GetReportSnapshotSourceLatestUpdate(c.ctx, db.GetReportSnapshotSourceLatestUpdateParams{WorkspaceID: c.report.WorkspaceID, ProjectID: c.project.ID})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			proof := reportSnapshotUpdateProof{ID: row.ID.String(), Revision: row.CurrentRevision, PublishedAt: row.PublishedAt.Time.UTC()}
			c.audit.LatestUpdate, c.facts.Health.LatestUpdateAt = &proof, &proof.PublishedAt
			revision := strconv.FormatInt(proof.Revision, 10)
			auditRow, err := reportAuditRow(proof.ID, proof.ID, revision, &proof, true)
			if err != nil {
				return err
			}
			proofRows = append(proofRows, auditRow)
			source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: proof.ID, Revision: revision})
			return nil
		})
	}
	if err != nil {
		return err
	}
	if !ok {
		c.audit.LatestUpdate, c.facts.Health.LatestUpdateAt = nil, nil
		proofRows, source.Identities = []reportSnapshotAuditRow{}, []reportsnapshot.SourceIdentity{}
		c.gap("source_unavailable", source.Kind)
	}
	return c.retainProjectionSource(source, audit, proofRows, ok)
}

func (c *reportSnapshotCollector) retainProjectionSource(source reportsnapshot.SourceFact, audit reportSnapshotSourceAudit, rows []reportSnapshotAuditRow, complete bool) error {
	page, err := reportAuditPage("", 0, rows, true)
	if err != nil {
		return err
	}
	audit.Pages, audit.Termination = append(audit.Pages, page), "selection_complete"
	source.ReadComplete, source.ProducerComplete = complete, complete
	if complete {
		source.Count = reportCount(len(rows))
	} else {
		audit.Termination = "unavailable"
	}
	if c.project == nil {
		source.Filters = append(source.Filters, "not_applicable_without_project")
	}
	c.retain(source, audit)
	return nil
}

func (c *reportSnapshotCollector) collectActivities() error {
	source, audit := c.source("activities", "legacy-status-activity-v1", "created_at ASC,id ASC")
	issueIDs := []pgtype.UUID{}
	ids := []string{}
	issuesComplete := false
	for _, item := range c.facts.Sources {
		if item.Kind == "issues" {
			issuesComplete = item.ReadComplete
		}
	}
	for _, issue := range c.facts.Issues {
		issueIDs = append(issueIDs, reportUUID(issue.ID))
		ids = append(ids, issue.ID)
	}
	sort.Strings(ids)
	cohortDigest, err := reportDigest(ids)
	if err != nil {
		return err
	}
	source.Filters = append(source.Filters, "action=status_changed", "cohort="+cohortDigest)
	complete := true
	ok, err := optionalReportSource(c.ctx, c.tx, func() error {
		afterTime := pgtype.Timestamptz{Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
		afterID := pgtype.UUID{Valid: true}
		for {
			page, err := c.q.ListReportSnapshotSourceActivities(c.ctx, db.ListReportSnapshotSourceActivitiesParams{WorkspaceID: c.report.WorkspaceID, IssueIds: issueIDs, AfterTime: afterTime, AfterID: afterID, RowLimit: 1000})
			if err != nil {
				return err
			}
			proofRows := []reportSnapshotAuditRow{}
			for _, row := range page {
				fact := reportsnapshot.ActivityFact{ID: row.ID.String(), IssueID: row.IssueID.String(), Action: row.Action, RecordedAt: row.CreatedAt.Time.UTC(), FromStatus: row.FromStatus, ToStatus: row.ToStatus, SourceVersion: "legacy-status-activity-v1"}
				revision, err := reportContentRevision(fact)
				if err != nil {
					return err
				}
				retained := len(c.facts.Activities) < reportsnapshot.MaxActivities
				proof, err := reportAuditRow(reportActivityKey(fact.RecordedAt, fact.ID), fact.ID, revision, fact, retained)
				if err != nil {
					return err
				}
				proofRows = append(proofRows, proof)
				if !retained {
					complete = false
					continue
				}
				c.facts.Activities = append(c.facts.Activities, fact)
			}
			receipt, err := reportAuditPage(reportActivityKey(afterTime.Time, afterID.String()), 1000, proofRows, len(page) < 1000)
			if err != nil {
				return err
			}
			audit.Pages = append(audit.Pages, receipt)
			if !complete || len(page) < 1000 {
				break
			}
			last := page[len(page)-1]
			afterTime, afterID = last.CreatedAt, last.ID
		}
		return nil
	})
	if err != nil {
		return err
	}
	audit.Termination = "eof"
	if !ok {
		c.facts.Activities, complete, audit.Termination = []reportsnapshot.ActivityFact{}, false, "unavailable"
		if err := reportAuditDiscardPages(&audit); err != nil {
			return err
		}
		c.gap("source_unavailable", source.Kind)
	} else if !complete {
		audit.Termination = "limit"
		c.gap("scan_limit", source.Kind)
	} else if !issuesComplete {
		complete, audit.Termination = false, "scope_incomplete"
		c.gap("cohort_incomplete", source.Kind)
	}
	for _, fact := range c.facts.Activities {
		revision, err := reportContentRevision(fact)
		if err != nil {
			return err
		}
		source.Identities = append(source.Identities, reportsnapshot.SourceIdentity{ID: fact.ID, Revision: revision})
	}
	source.ReadComplete, source.ProducerComplete = complete, false
	if complete {
		source.Count = reportCount(len(c.facts.Activities))
	}
	if len(c.facts.Activities) > 0 {
		last := c.facts.Activities[len(c.facts.Activities)-1]
		cursor := reportActivityKey(last.RecordedAt, last.ID)
		source.Cursor = &cursor
	}
	c.gap("producer_history_unproven", source.Kind)
	c.retain(source, audit)
	return nil
}

func reportTimestamp(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func compactReportStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
