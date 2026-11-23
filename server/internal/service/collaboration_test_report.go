package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type collaborationStoredTestReport struct {
	Version         int                               `json:"version"`
	Identity        protocol.CollaborationRunIdentity `json:"identity"`
	RequestID       string                            `json:"request_id"`
	RequestDigest   string                            `json:"request_digest"`
	ReportDigest    string                            `json:"report_digest"`
	AgentID         string                            `json:"agent_id"`
	AttemptSeq      string                            `json:"attempt_seq"`
	AttemptNonce    string                            `json:"attempt_nonce"`
	ClaimGeneration string                            `json:"claim_generation"`
	Containment     string                            `json:"containment"`
	SourceKind      string                            `json:"source_kind"`
	Report          protocol.CollaborationTestReport  `json:"report"`
}

func testReportProducerKey(attempt int64) string { return "test-report:" + stageVersion(attempt) }

// Look up originally issued authority before any current-admission check so an
// exact content-free receipt survives Stop, revocation and natural settlement.
func issuedTestReportCapability(ctx context.Context, q *db.Queries, run pgtype.UUID, token string) (db.CollaborationBusinessCapability, error) {
	var c db.CollaborationBusinessCapability
	if !run.Valid || !strings.HasPrefix(token, CollaborationBusinessTokenPrefix) || len(token) != 68 {
		return c, collaborationError(403, "forbidden")
	}
	c, err := q.GetCollaborationBusinessCapabilityByHash(ctx, db.GetCollaborationBusinessCapabilityByHashParams{RunID: run, TokenHash: collaborationCapabilityHash(token)})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, collaborationError(403, "forbidden")
	}
	return c, err
}

func testReportAttemptIdentity(ctx context.Context, q *db.Queries, run pgtype.UUID, seq int64) (db.CollaborationAttempt, protocol.CollaborationRunIdentity, error) {
	a, err := q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: run, AttemptSeq: seq})
	if err != nil {
		return a, protocol.CollaborationRunIdentity{}, err
	}
	term, err := q.GetCollaborationTermination(ctx, run)
	if err != nil {
		return a, protocol.CollaborationRunIdentity{}, err
	}
	id, err := collaborationFrozenIdentity(term)
	if err != nil {
		return a, id, err
	}
	if id.RunID != util.UUIDToString(run) || id.ClaimGeneration != stageVersion(a.ClaimGeneration) || !stageUUID(a.AttemptNonce) || !a.AuthorizedAt.Valid ||
		(a.Containment != protocol.CollaborationContainmentLinuxPIDNamespace && a.Containment != protocol.CollaborationContainmentHostProcessSupervision) {
		return a, id, collaborationError(409, "test_subject_mismatch")
	}
	return a, id, nil
}

func decodeStoredTestReport(e db.CollaborationRunEvent) (collaborationStoredTestReport, error) {
	var p collaborationStoredTestReport
	if e.Kind != "test_report" || decodeCollaborationPreparationJSON(e.Payload, &p) != nil || p.Version != 1 || p.SourceKind != "runtime_reported" ||
		p.Identity.RunID != util.UUIDToString(e.RunID) || p.Identity.IssueID != util.UUIDToString(e.IssueID) || !stageUUID(p.AgentID) || !stageUUID(p.RequestID) || !stageDigest(p.RequestDigest) || !stageDigest(p.ReportDigest) || !stageUUID(p.AttemptNonce) || p.ClaimGeneration != p.Identity.ClaimGeneration ||
		protocol.ValidateCollaborationTestReportRequest(protocol.CollaborationTestReportRequest{Version: 1, Identity: p.Identity, RequestID: p.RequestID, Report: p.Report}) != nil {
		return p, collaborationError(503, "test_report_unavailable")
	}
	seq, ok := collaborationRuntimeVersion(p.AttemptSeq, false)
	if !ok || e.ProducerEventID != testReportProducerKey(seq) {
		return p, collaborationError(503, "test_report_unavailable")
	}
	_, digest, err := stageJSONDigest(p.Report)
	if err != nil || digest != p.ReportDigest {
		return p, collaborationError(503, "test_report_unavailable")
	}
	return p, nil
}

func testReportACK(e db.CollaborationRunEvent, p collaborationStoredTestReport) protocol.CollaborationTestReportReceipt {
	return protocol.CollaborationTestReportReceipt{Version: 1, IssueID: util.UUIDToString(e.IssueID), RunID: util.UUIDToString(e.RunID), RequestID: p.RequestID, ReportID: util.UUIDToString(e.ID), ReportDigest: p.ReportDigest, Accepted: true}
}

func recoverRuntimeTestReport(ctx context.Context, q *db.Queries, run pgtype.UUID, capability db.CollaborationBusinessCapability, requestID, requestDigest string, expected *protocol.CollaborationRunIdentity) (protocol.CollaborationTestReportReceipt, bool, error) {
	var out protocol.CollaborationTestReportReceipt
	e, err := q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: testReportProducerKey(capability.AttemptSeq)})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	p, err := decodeStoredTestReport(e)
	if err != nil {
		return out, false, err
	}
	a, id, err := testReportAttemptIdentity(ctx, q, run, capability.AttemptSeq)
	if err != nil {
		return out, false, err
	}
	if p.Identity != id || p.AttemptSeq != stageVersion(capability.AttemptSeq) || p.AttemptNonce != a.AttemptNonce || p.Containment != a.Containment || expected != nil && *expected != id {
		return out, false, collaborationError(409, "test_subject_mismatch")
	}
	if p.RequestID != requestID {
		if requestDigest == "" {
			return out, false, collaborationError(404, "not_found")
		}
		return out, false, collaborationError(409, "test_report_already_recorded")
	}
	if requestDigest != "" && p.RequestDigest != requestDigest {
		return out, false, collaborationError(409, "test_report_conflict")
	}
	return testReportACK(e, p), true, nil
}

func (s *CollaborationService) RuntimeTestReportReceipt(ctx context.Context, run pgtype.UUID, token, requestID string) (protocol.CollaborationTestReportReceipt, error) {
	if !stageUUID(requestID) {
		return protocol.CollaborationTestReportReceipt{}, collaborationError(400, "invalid_test_report")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationTestReportReceipt, error) {
		q := s.Queries.WithTx(tx)
		capability, err := issuedTestReportCapability(ctx, q, run, token)
		if err != nil {
			return protocol.CollaborationTestReportReceipt{}, err
		}
		out, found, err := recoverRuntimeTestReport(ctx, q, run, capability, requestID, "", nil)
		if err == nil && !found {
			err = collaborationError(404, "not_found")
		}
		return out, err
	})
}

func (s *CollaborationService) RuntimeTestSubject(ctx context.Context, run pgtype.UUID, token string) (protocol.CollaborationTestSubjectResponse, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationTestSubjectResponse, error) {
		var out protocol.CollaborationTestSubjectResponse
		if _, err := issuedTestReportCapability(ctx, s.Queries.WithTx(tx), run, token); err != nil {
			return out, err
		}
		r, err := s.lockRuntimeBusiness(ctx, tx, run, token)
		if err != nil {
			return out, err
		}
		f, err := loadFrozenTestSubject(ctx, r.q, run, r.access.Issue.ID, collaborationRunIdentity(r.run, r.access.Issue.ID))
		if err != nil {
			return out, err
		}
		if err = checkRuntimeTestSource(ctx, r, f); err != nil {
			return out, err
		}
		return protocol.CollaborationTestSubjectResponse{Version: 1, IssueID: util.UUIDToString(r.access.Issue.ID), RunID: util.UUIDToString(run), Subject: f.Subject}, nil
	})
}

func (s *CollaborationService) RecordRuntimeTestReport(ctx context.Context, run pgtype.UUID, token string, p protocol.CollaborationTestReportRequest) (protocol.CollaborationTestReportReceipt, error) {
	if protocol.ValidateCollaborationTestReportRequest(p) != nil {
		return protocol.CollaborationTestReportReceipt{}, collaborationError(400, "invalid_test_report")
	}
	_, requestDigest, err := stageJSONDigest(p)
	if err != nil {
		return protocol.CollaborationTestReportReceipt{}, err
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationTestReportReceipt, error) {
		var out protocol.CollaborationTestReportReceipt
		q := s.Queries.WithTx(tx)
		capability, err := issuedTestReportCapability(ctx, q, run, token)
		if err != nil {
			return out, err
		}
		if ack, found, err := recoverRuntimeTestReport(ctx, q, run, capability, p.RequestID, requestDigest, &p.Identity); found || err != nil {
			return ack, err
		}
		r, err := s.lockCollaborationRuntimeRun(ctx, tx, run, true, false)
		if err != nil {
			return out, err
		}
		// Concurrent submissions serialize on the original runtime locks.
		if ack, found, err := recoverRuntimeTestReport(ctx, q, run, capability, p.RequestID, requestDigest, &p.Identity); found || err != nil {
			return ack, err
		}
		capability, err = issuedTestReportCapability(ctx, q, run, token)
		if err != nil {
			return out, err
		}
		if capability.RevokedAt.Valid || capability.AttemptSeq != r.run.AuthorizedAttemptSeq {
			return out, collaborationError(403, "forbidden")
		}
		if err = r.currentAdmission(ctx); err != nil {
			return out, err
		}
		if err = r.requireReservation(ctx); err != nil {
			return out, err
		}
		attempt, identity, err := testReportAttemptIdentity(ctx, q, run, capability.AttemptSeq)
		if err != nil {
			return out, err
		}
		if p.Identity != identity || identity != collaborationRunIdentity(r.run, r.access.Issue.ID) {
			return out, collaborationError(409, "test_subject_mismatch")
		}
		f, err := loadFrozenTestSubject(ctx, q, run, r.access.Issue.ID, identity)
		if err != nil {
			return out, err
		}
		if !reflect.DeepEqual(f.Subject, p.Report.Subject) {
			return out, collaborationError(409, "test_subject_mismatch")
		}
		if err = checkRuntimeTestSource(ctx, r, f); err != nil {
			return out, err
		}
		// Copy through JSON before redaction; callers may retain the same report
		// value as their immutable retry intent.
		raw, err := json.Marshal(p.Report)
		if err != nil {
			return out, err
		}
		var report protocol.CollaborationTestReport
		if err = json.Unmarshal(raw, &report); err != nil {
			return out, err
		}
		report.SuiteID = redact.Text(report.SuiteID)
		for i := range report.Cases {
			report.Cases[i].ID = redact.Text(report.Cases[i].ID)
		}
		for i := range report.Logs {
			if report.Logs[i].Kind == "inline" {
				report.Logs[i].Body = redact.Text(report.Logs[i].Body)
			} else if _, err = resolveTestReportLog(ctx, q, run, identity, attempt, report.Logs[i]); err != nil {
				return out, err
			}
		}
		if protocol.ValidateCollaborationTestReport(report) != nil {
			return out, collaborationError(400, "invalid_test_report")
		}
		_, digest, err := stageJSONDigest(report)
		if err != nil {
			return out, err
		}
		stored := collaborationStoredTestReport{Version: 1, Identity: identity, RequestID: p.RequestID, RequestDigest: requestDigest, ReportDigest: digest, AgentID: util.UUIDToString(r.run.AgentID),
			AttemptSeq: stageVersion(attempt.AttemptSeq), AttemptNonce: attempt.AttemptNonce, ClaimGeneration: stageVersion(attempt.ClaimGeneration), Containment: attempt.Containment, SourceKind: "runtime_reported", Report: report}
		raw, err = json.Marshal(stored)
		if err != nil {
			return out, err
		}
		e, err := q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: run, EventID: testReportProducerKey(attempt.AttemptSeq), Kind: "test_report", Payload: raw})
		if err != nil {
			return out, err
		}
		if err = recordCollaborationExecutionChange(ctx, q, r.access.Issue.ID, r.group.ID, run, "test_report_recorded"); err != nil {
			return out, err
		}
		return testReportACK(e, stored), nil
	})
}

func resolveTestReportLog(ctx context.Context, q *db.Queries, run pgtype.UUID, identity protocol.CollaborationRunIdentity, attempt db.CollaborationAttempt, log protocol.CollaborationTestLog) (string, error) {
	missing := collaborationError(409, "test_report_log_not_retained")
	e, err := q.GetCollaborationTestReportLogEvent(ctx, db.GetCollaborationTestReportLogEventParams{RunID: run, ID: util.MustParseUUID(log.EventID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", missing
	}
	if err != nil {
		return "", err
	}
	if e.IssueID != util.MustParseUUID(identity.IssueID) {
		return "", missing
	}
	var messages []protocol.CollaborationRecordedMessage
	switch e.Kind {
	case "record_checkpoint":
		var p protocol.CollaborationRecordingCheckpoint
		if decodeCollaborationPreparationJSON(e.Payload, &p) != nil || protocol.ValidateCollaborationRecordingCheckpoint(p) != nil || p.Identity != identity || p.AttemptSeq != stageVersion(attempt.AttemptSeq) || p.AttemptNonce != attempt.AttemptNonce {
			return "", missing
		}
		messages = p.Messages
	case "outcome":
		saved, decodeErr := decodeCollaborationStoredOutcome(e.Payload)
		p := saved.CollaborationRunOutcome
		if decodeErr != nil || p.Record == nil || protocol.ValidateCollaborationRunRecord(p.Record) != nil || p.Identity != identity || p.AttemptSeq != stageVersion(attempt.AttemptSeq) || p.AttemptNonce != attempt.AttemptNonce || !collaborationRuntimeMessageMode(p.ProtocolVersion, p.Containment, attempt.Containment) {
			return "", missing
		}
		messages = p.Record.Messages
	default:
		return "", missing
	}
	if log.MessageIndex < 0 || log.MessageIndex >= len(messages) {
		return "", missing
	}
	body := messages[log.MessageIndex].Body
	if len(body) > protocol.CollaborationTestReportLogBodyLimit || protocol.CollaborationKnowledgeHash(body) != log.BodySHA256 {
		return "", missing
	}
	return body, nil
}
