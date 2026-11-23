package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type CollaborationTestReportsQuery struct {
	Limit  int
	Cursor string
}

type CollaborationTestReportItem struct {
	ReportID        string                            `json:"report_id"`
	ReportDigest    string                            `json:"report_digest"`
	AgentID         string                            `json:"agent_id"`
	Attempt         string                            `json:"attempt"`
	SourceKind      string                            `json:"source_kind"`
	Subject         protocol.CollaborationTestSubject `json:"subject"`
	ReportedResult  string                            `json:"reported_result"`
	Applicability   string                            `json:"applicability"`
	CaseCount       int                               `json:"case_count"`
	FailedCaseCount int                               `json:"failed_case_count"`
	ReceivedAt      time.Time                         `json:"received_at"`
}

type CollaborationTestReportsPage struct {
	Version    int                           `json:"version"`
	IssueID    string                        `json:"issue_id"`
	RunID      string                        `json:"run_id"`
	Items      []CollaborationTestReportItem `json:"items"`
	NextCursor *string                       `json:"next_cursor"`
}

type CollaborationTestReportLogView struct {
	Kind         string `json:"kind"`
	EventID      string `json:"event_id,omitempty"`
	MessageIndex *int   `json:"message_index,omitempty"`
	BodySHA256   string `json:"body_sha256"`
	Body         string `json:"body"`
}

type CollaborationTestReportDetail struct {
	Version int    `json:"version"`
	IssueID string `json:"issue_id"`
	RunID   string `json:"run_id"`
	CollaborationTestReportItem
	SuiteID string                           `json:"suite_id"`
	Cases   []protocol.CollaborationTestCase `json:"cases"`
	Logs    []CollaborationTestReportLogView `json:"logs"`
}

type collaborationTestReportCursor struct {
	IssueID  string `json:"issue_id"`
	RunID    string `json:"run_id"`
	ViewerID string `json:"viewer_id"`
	Through  string `json:"through"`
	Before   string `json:"before"`
	MAC      string `json:"mac"`
}

func testReportCursorMAC(v collaborationTestReportCursor, key string) (string, error) {
	v.MAC = ""
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte("collaboration-test-report-cursor-v1:"+key))
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// The run already owns a private random salt for its immutable source binding.
// Reuse that secret to authenticate this separate cursor domain; public source
// hashes cannot sign a new watermark, and no new key configuration is needed.
func testReportCursorKey(ctx context.Context, q *db.Queries, run, issue pgtype.UUID) (string, error) {
	term, err := q.GetCollaborationTermination(ctx, run)
	if err != nil {
		return "", err
	}
	id, err := collaborationFrozenIdentity(term)
	if err != nil {
		return "", err
	}
	frozen, err := loadFrozenTestSubject(ctx, q, run, issue, id)
	return frozen.Salt, err
}

func testReportItem(e db.CollaborationRunEvent, p collaborationStoredTestReport, applicability string) CollaborationTestReportItem {
	failed := 0
	for _, c := range p.Report.Cases {
		if c.Outcome == "failed" || c.Outcome == "error" {
			failed++
		}
	}
	return CollaborationTestReportItem{ReportID: util.UUIDToString(e.ID), ReportDigest: p.ReportDigest, AgentID: p.AgentID, Attempt: p.AttemptSeq, SourceKind: p.SourceKind, Subject: p.Report.Subject,
		ReportedResult: p.Report.ReportedResult(), Applicability: applicability, CaseCount: len(p.Report.Cases), FailedCaseCount: failed, ReceivedAt: e.CreatedAt.Time.UTC()}
}

func testReportReadRun(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, runID pgtype.UUID) (db.CollaborationRun, error) {
	q := db.New(tx)
	if !access.SourceMember {
		return db.CollaborationRun{}, collaborationError(404, "not_found")
	}
	run, err := q.GetCollaborationRecordedRun(ctx, db.GetCollaborationRecordedRunParams{IssueID: access.Issue.ID, RunID: runID})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && run.SessionID != access.SessionID {
		return run, collaborationError(404, "not_found")
	}
	if err != nil {
		return run, err
	}
	// Authorize before cursor validation or page filtering, including an empty
	// report page. Legacy runs without a frozen subject keep their empty list.
	_, err = q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: runID, ProducerEventID: "test-subject"})
	if errors.Is(err, pgx.ErrNoRows) {
		return run, nil
	}
	if err != nil {
		return run, err
	}
	term, err := q.GetCollaborationTermination(ctx, runID)
	if err != nil {
		return run, err
	}
	identity, err := collaborationFrozenIdentity(term)
	if err != nil {
		return run, err
	}
	frozen, err := loadFrozenTestSubject(ctx, q, runID, access.Issue.ID, identity)
	if err != nil {
		return run, err
	}
	return run, testReportFrozenSourceRead(ctx, tx, access, frozen)
}

func testReportLogs(ctx context.Context, q *db.Queries, e db.CollaborationRunEvent, p collaborationStoredTestReport) ([]CollaborationTestReportLogView, error) {
	attempt, id, err := testReportAttemptIdentity(ctx, q, e.RunID, mustStageReportAttempt(p.AttemptSeq))
	if err != nil {
		return nil, err
	}
	if id != p.Identity || p.AttemptNonce != attempt.AttemptNonce || p.Containment != attempt.Containment {
		return nil, collaborationError(503, "test_report_unavailable")
	}
	frozen, err := loadFrozenTestSubject(ctx, q, e.RunID, e.IssueID, id)
	if err != nil || !reflect.DeepEqual(frozen.Subject, p.Report.Subject) {
		return nil, collaborationError(503, "test_report_unavailable")
	}
	logs := make([]CollaborationTestReportLogView, 0, len(p.Report.Logs))
	for _, l := range p.Report.Logs {
		out := CollaborationTestReportLogView{Kind: l.Kind}
		if l.Kind == "inline" {
			out.Body = l.Body
			out.BodySHA256 = protocol.CollaborationKnowledgeHash(l.Body)
		} else {
			body, err := resolveTestReportLog(ctx, q, e.RunID, id, attempt, l)
			if err != nil {
				return nil, err
			}
			index := l.MessageIndex
			out.EventID = l.EventID
			out.MessageIndex = &index
			out.Body = body
			out.BodySHA256 = l.BodySHA256
		}
		logs = append(logs, out)
	}
	return logs, nil
}

func mustStageReportAttempt(v string) int64 { n, _ := collaborationRuntimeVersion(v, false); return n }

// Applicability is a live projection. Failed reports never disappear when a
// new candidate or execution replaces the source they originally described.
func testReportApplicability(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, run db.CollaborationRun, e db.CollaborationRunEvent, p collaborationStoredTestReport) (string, error) {
	if p.AgentID != util.UUIDToString(run.AgentID) {
		return "", collaborationError(503, "test_report_unavailable")
	}
	if p.AttemptSeq != stageVersion(run.AuthorizedAttemptSeq) {
		return "stale", nil
	}
	root, err := loadStageRoot(ctx, q, access.Issue.ID)
	if err != nil {
		return "", err
	}
	if !root.managed {
		return "stale", nil
	}
	state := &collaborationStageState{policy: root.policy}
	if state.lastDisabled, err = q.GetProjectStagePolicyLastDisabled(ctx, db.GetProjectStagePolicyLastDisabledParams{WorkspaceID: access.Issue.WorkspaceID, ProjectID: access.Issue.ProjectID}); err != nil {
		return "", err
	}
	if err = loadStageDetails(ctx, q, access.Issue, access.LifecycleEpoch, state, false); err != nil {
		return "", err
	}
	c, err := q.GetCollaborationStageCodeHead(ctx, access.Issue.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "stale", nil
	}
	if err != nil {
		return "", err
	}
	if p.Report.Subject.CandidateID != util.UUIDToString(c.CandidateID) || p.Report.Subject.CandidateVersion != stageVersion(c.Version) || p.Report.Subject.CandidateDigest != c.ContentDigest || c.LifecycleEpoch != access.LifecycleEpoch {
		return "stale", nil
	}
	content, err := decodeStageCode(c)
	if err != nil {
		return "", err
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return "", err
	}
	_, code, err := stageCodeCurrent(ctx, q, access, state, c, content, now.Time)
	if err != nil {
		return "", err
	}
	if code != "" {
		return "stale", nil
	}
	if _, err = testReportLogs(ctx, q, e, p); err != nil {
		if collaborationErrorCodeForReport(err, "test_report_log_not_retained") {
			return "stale", nil
		}
		return "", err
	}
	if run.ForcedDisposition.Valid || run.BusinessOutcome.String == "failure" {
		return "execution_failed", nil
	}
	if !run.SettledAt.Valid || !collaborationRunProvenEnded(run) || run.BusinessOutcome.String != "success" {
		return "execution_unsettled", nil
	}
	return "usable", nil
}

func collaborationErrorCodeForReport(err error, code string) bool {
	var e *CollaborationError
	return errors.As(err, &e) && e.Code == code
}

func (s *CollaborationService) TestReports(ctx context.Context, issue, run pgtype.UUID, p CollaborationTestReportsQuery) (CollaborationTestReportsPage, error) {
	out := CollaborationTestReportsPage{Version: 1, IssueID: util.UUIDToString(issue), RunID: util.UUIDToString(run), Items: []CollaborationTestReportItem{}}
	if err := validateCollaborationExecutionRead(ctx); err != nil {
		return out, err
	}
	if p.Limit == 0 {
		p.Limit = 20
	}
	invalid := collaborationError(400, "invalid_test_report_query")
	if !issue.Valid || !run.Valid || p.Limit < 1 || p.Limit > 50 || len(p.Cursor) > 1024 {
		return out, invalid
	}
	err := s.readCollaborationStageEvidence(ctx, issue, func(tx pgx.Tx, q *db.Queries, access CollaborationIssueAccess) error {
		r, err := testReportReadRun(ctx, tx, access, run)
		if err != nil {
			return err
		}
		latest, err := q.GetCollaborationRunRecordSequence(ctx, db.GetCollaborationRunRecordSequenceParams{IssueID: issue, RunID: run})
		if err != nil {
			return err
		}
		principal, _ := collaboration.PrincipalFromContext(ctx)
		cursor := collaborationTestReportCursor{IssueID: out.IssueID, RunID: out.RunID, ViewerID: principal.UserID, Through: stageVersion(latest), Before: stageVersion(math.MaxInt64)}
		through, before := latest, int64(math.MaxInt64)
		if p.Cursor != "" {
			raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
			var v collaborationTestReportCursor
			if err != nil || decodeCollaborationPreparationJSON(raw, &v) != nil || v.IssueID != cursor.IssueID || v.RunID != cursor.RunID || v.ViewerID != cursor.ViewerID {
				return invalid
			}
			key, err := testReportCursorKey(ctx, q, run, issue)
			if err != nil {
				return err
			}
			mac, err := testReportCursorMAC(v, key)
			if err != nil || !hmac.Equal([]byte(mac), []byte(v.MAC)) {
				return invalid
			}
			var ok bool
			through, ok = collaborationRuntimeVersion(v.Through, true)
			if !ok || through > latest {
				return invalid
			}
			before, ok = collaborationRuntimeVersion(v.Before, false)
			if !ok || before > through {
				return invalid
			}
			cursor = v
		}
		rows, err := q.ListCollaborationTestReportEvents(ctx, db.ListCollaborationTestReportEventsParams{IssueID: issue, RunID: run, Seq: through, Seq_2: before, Limit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		if len(rows) > p.Limit {
			rows = rows[:p.Limit]
			cursor.Before = stageVersion(rows[len(rows)-1].Seq)
			key, err := testReportCursorKey(ctx, q, run, issue)
			if err != nil {
				return err
			}
			cursor.MAC, err = testReportCursorMAC(cursor, key)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(cursor)
			if err != nil {
				return err
			}
			next := base64.RawURLEncoding.EncodeToString(raw)
			out.NextCursor = &next
		}
		for _, e := range rows {
			stored, err := decodeStoredTestReport(e)
			if err != nil {
				return err
			}
			if err = testReportSourceRead(ctx, tx, access, e, stored); err != nil {
				return err
			}
			app, err := testReportApplicability(ctx, q, access, r, e, stored)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, testReportItem(e, stored, app))
		}
		return nil
	})
	if err != nil {
		return CollaborationTestReportsPage{}, err
	}
	return out, nil
}

func (s *CollaborationService) TestReport(ctx context.Context, issue, run, report pgtype.UUID) (CollaborationTestReportDetail, error) {
	var out CollaborationTestReportDetail
	if err := validateCollaborationExecutionRead(ctx); err != nil {
		return out, err
	}
	err := s.readCollaborationStageEvidence(ctx, issue, func(tx pgx.Tx, q *db.Queries, access CollaborationIssueAccess) error {
		r, err := testReportReadRun(ctx, tx, access, run)
		if err != nil {
			return err
		}
		e, err := q.GetCollaborationTestReportEvent(ctx, db.GetCollaborationTestReportEventParams{IssueID: issue, RunID: run, ID: report})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(404, "not_found")
		}
		if err != nil {
			return err
		}
		stored, err := decodeStoredTestReport(e)
		if err != nil {
			return err
		}
		if err = testReportSourceRead(ctx, tx, access, e, stored); err != nil {
			return err
		}
		logs, err := testReportLogs(ctx, q, e, stored)
		if err != nil {
			return err
		}
		app, err := testReportApplicability(ctx, q, access, r, e, stored)
		if err != nil {
			return err
		}
		out = CollaborationTestReportDetail{Version: 1, IssueID: util.UUIDToString(issue), RunID: util.UUIDToString(run), CollaborationTestReportItem: testReportItem(e, stored, app), SuiteID: stored.Report.SuiteID, Cases: stored.Report.Cases, Logs: logs}
		return nil
	})
	if err != nil {
		return CollaborationTestReportDetail{}, err
	}
	return out, nil
}
