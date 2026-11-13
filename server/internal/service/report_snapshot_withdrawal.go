package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type ReportSnapshotWithdrawalRequest struct {
	ProtocolVersion           int    `json:"protocol_version"`
	Scope                     string `json:"scope"`
	ExpectedIssueRevision     int64  `json:"expected_issue_revision"`
	ExpectedCurrentSnapshotID string `json:"expected_current_snapshot_id"`
	Reason                    string `json:"reason"`
}

type ReportSnapshotWithdrawalReceipt struct {
	WithdrawalID              string    `json:"withdrawal_id"`
	RequestKey                string    `json:"request_key"`
	ReportIssueID             string    `json:"report_issue_id"`
	ExpectedCurrentSnapshotID string    `json:"expected_current_snapshot_id"`
	RequestDigest             string    `json:"request_digest"`
	WithdrawnAt               time.Time `json:"withdrawn_at"`
}

type ReportSnapshotWithdrawalResponse struct {
	ProtocolVersion   int                             `json:"protocol_version"`
	WithdrawalReceipt ReportSnapshotWithdrawalReceipt `json:"withdrawal_receipt"`
	Replayed          bool                            `json:"replayed"`
}

func ParseReportSnapshotWithdrawalRequest(raw []byte) (ReportSnapshotWithdrawalRequest, error) {
	var out ReportSnapshotWithdrawalRequest
	if !utf8.Valid(raw) {
		return out, reportSnapshotInvalid()
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return out, reportSnapshotInvalid()
	}
	allowed := map[string]bool{"protocol_version": true, "scope": true, "expected_issue_revision": true, "expected_current_snapshot_id": true, "reason": true}
	seen := map[string]bool{}
	for dec.More() {
		token, err := dec.Token()
		key, ok := token.(string)
		if err != nil || !ok || !allowed[key] || seen[key] {
			return out, reportSnapshotInvalid()
		}
		seen[key] = true
		var value json.RawMessage
		if dec.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return out, reportSnapshotInvalid()
		}
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') || dec.Decode(new(any)) != io.EOF || len(seen) != len(allowed) || json.Unmarshal(raw, &out) != nil {
		return out, reportSnapshotInvalid()
	}
	if out.ProtocolVersion != 1 || out.Scope != "report" || out.ExpectedIssueRevision < 1 || !CanonicalReportSnapshotUUID(out.ExpectedCurrentSnapshotID) || strings.TrimSpace(out.Reason) == "" || len(out.Reason) > 4096 || strings.ContainsRune(out.Reason, '\x00') {
		return out, reportSnapshotInvalid()
	}
	return out, nil
}

func ReportSnapshotWithdrawalRequestDigest(reportID, key string, request ReportSnapshotWithdrawalRequest) (string, error) {
	return reportDigest(struct {
		ReportIssueID string                          `json:"report_issue_id"`
		RequestKey    string                          `json:"request_key"`
		Request       ReportSnapshotWithdrawalRequest `json:"request"`
	}{reportID, key, request})
}

func withdrawalReceipt(control db.ReportSnapshotControl) (ReportSnapshotWithdrawalReceipt, error) {
	if !control.WithdrawnAt.Valid || !control.WithdrawalID.Valid || !control.WithdrawalRequestKey.Valid || !control.WithdrawalExpectedSnapshotID.Valid || !control.WithdrawalRequestDigest.Valid || len(control.WithdrawalRequestDigest.String) != 64 {
		return ReportSnapshotWithdrawalReceipt{}, reportSnapshotUnavailable()
	}
	return ReportSnapshotWithdrawalReceipt{WithdrawalID: control.WithdrawalID.String(), RequestKey: control.WithdrawalRequestKey.String(), ReportIssueID: control.ReportIssueID.String(), ExpectedCurrentSnapshotID: control.WithdrawalExpectedSnapshotID.String(), RequestDigest: control.WithdrawalRequestDigest.String, WithdrawnAt: control.WithdrawnAt.Time.UTC().Truncate(time.Microsecond)}, nil
}

// WithdrawReportSnapshotsInTx only mutates the report's shared control record.
// All capture/correction writers update this same row, so old RR views cannot
// wait past a withdrawal and subsequently publish another readable snapshot.
func WithdrawReportSnapshotsInTx(ctx context.Context, tx pgx.Tx, report db.Issue, actorID, key string, request ReportSnapshotWithdrawalRequest) (ReportSnapshotWithdrawalResponse, error) {
	out := ReportSnapshotWithdrawalResponse{ProtocolVersion: 1}
	q := db.New(tx)
	digest, err := ReportSnapshotWithdrawalRequestDigest(report.ID.String(), key, request)
	if err != nil {
		return out, err
	}
	prior, err := q.GetReportSnapshotWithdrawalByRequest(ctx, db.GetReportSnapshotWithdrawalByRequestParams{WorkspaceID: report.WorkspaceID, WithdrawnBy: reportUUID(actorID), WithdrawalRequestKey: reportUUID(key)})
	if err == nil {
		if prior.ReportIssueID != report.ID || prior.WithdrawalRequestDigest.String != digest {
			return out, reportSnapshotConflict()
		}
		out.WithdrawalReceipt, err = withdrawalReceipt(prior)
		out.Replayed = true
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	control, err := q.LockReportSnapshotControl(ctx, db.LockReportSnapshotControlParams{WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID})
	if err != nil {
		return out, err
	}
	if control.WithdrawnAt.Valid || report.Revision != request.ExpectedIssueRevision || !control.CurrentSnapshotID.Valid || control.CurrentSnapshotID.String() != request.ExpectedCurrentSnapshotID {
		return out, reportSnapshotConflict()
	}
	current, err := q.GetReportSnapshot(ctx, db.GetReportSnapshotParams{WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID, ID: control.CurrentSnapshotID})
	if err != nil {
		return out, err
	}
	if current.ProjectID != report.ProjectID || current.Sequence != control.CurrentSequence {
		return out, reportSnapshotConflict()
	}
	withdrawn, err := q.WithdrawReportSnapshots(ctx, db.WithdrawReportSnapshotsParams{WithdrawalID: dbid.NewV7(), WithdrawnBy: reportUUID(actorID), RequestKey: reportUUID(key), RequestDigest: digest, ExpectedSnapshotID: control.CurrentSnapshotID, ReportRevision: report.Revision, Reason: request.Reason, WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID, ExpectedRevision: control.Revision})
	if err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == "23505" && strings.HasPrefix(pgerr.ConstraintName, "report_snapshot_") {
			return out, reportSnapshotConflict()
		}
		return out, err
	}
	out.WithdrawalReceipt, err = withdrawalReceipt(withdrawn)
	return out, err
}
