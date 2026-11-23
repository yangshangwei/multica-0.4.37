package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/projecthealth"
	"github.com/multica-ai/multica/server/internal/reportsnapshot"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type ReportSnapshotError struct {
	Status int
	Code   string
}

func (e ReportSnapshotError) Error() string { return e.Code }
func reportSnapshotInvalid() error {
	return ReportSnapshotError{400, "invalid_report_snapshot_request"}
}
func reportSnapshotConflict() error { return ReportSnapshotError{409, "report_snapshot_conflict"} }
func reportSnapshotUnavailable() error {
	return ReportSnapshotError{503, "report_snapshot_unavailable"}
}

type ReportSnapshotRequest struct {
	ProtocolVersion       int       `json:"protocol_version"`
	ExpectedIssueRevision int64     `json:"expected_issue_revision"`
	Timezone              string    `json:"timezone"`
	Start                 time.Time `json:"start"`
	End                   time.Time `json:"end"`
	PeriodMode            string    `json:"period_mode"`
	CorrectionOf          *string   `json:"correction_of"`
}

func (r ReportSnapshotRequest) Window() reportsnapshot.Window {
	return reportsnapshot.Window{Start: r.Start, End: r.End, Timezone: r.Timezone, PeriodMode: r.PeriodMode}
}

type ReportSnapshotResponse struct {
	ProtocolVersion int                           `json:"protocol_version"`
	Snapshot        reportsnapshot.PublicSnapshot `json:"snapshot"`
	Replayed        bool                          `json:"replayed"`
	CaptureReceipt  *ReportSnapshotCaptureReceipt `json:"capture_receipt,omitempty"`
}

// This receipt belongs only to the human capture response, never Reporter GET.
type ReportSnapshotCaptureReceipt struct {
	RequestKey    string `json:"request_key"`
	ReportIssueID string `json:"report_issue_id"`
	SnapshotID    string `json:"snapshot_id"`
	RequestDigest string `json:"request_digest"`
}

type reportSnapshotStorage struct {
	StorageVersion int                           `json:"storage_version"`
	Internal       reportsnapshot.Snapshot       `json:"internal"`
	Public         reportsnapshot.PublicSnapshot `json:"public"`
	Audit          reportSnapshotAudit           `json:"audit"`
	AuditDigest    string                        `json:"audit_digest"`
}

// ReportSnapshotRequestDigest commits the canonical typed capture intent. It
// intentionally excludes private actor/idempotency metadata from the receipt.
func ReportSnapshotRequestDigest(reportID, key string, request ReportSnapshotRequest) (string, error) {
	return reportDigest(struct {
		ReportIssueID string                `json:"report_issue_id"`
		RequestKey    string                `json:"request_key"`
		Request       ReportSnapshotRequest `json:"request"`
	}{reportID, key, request})
}

func captureReceipt(reportID, key, snapshotID string, request ReportSnapshotRequest) (*ReportSnapshotCaptureReceipt, error) {
	digest, err := ReportSnapshotRequestDigest(reportID, key, request)
	if err != nil {
		return nil, err
	}
	return &ReportSnapshotCaptureReceipt{RequestKey: key, ReportIssueID: reportID, SnapshotID: snapshotID, RequestDigest: digest}, nil
}

func reportSnapshotSealDigest(stored reportSnapshotStorage) (string, error) {
	return reportDigest(struct {
		StorageVersion int     `json:"storage_version"`
		SnapshotID     string  `json:"snapshot_id"`
		WorkspaceID    string  `json:"workspace_id"`
		ReportIssueID  string  `json:"report_issue_id"`
		ProjectID      *string `json:"project_id"`
		InternalDigest string  `json:"internal_digest"`
		PublicDigest   string  `json:"public_digest"`
		AuditDigest    string  `json:"audit_digest"`
	}{stored.StorageVersion, stored.Internal.SnapshotID, stored.Internal.WorkspaceID, stored.Internal.ReportIssueID, stored.Internal.ProjectID, stored.Internal.SnapshotDigest, stored.Public.ProjectionDigest, stored.AuditDigest})
}

// ParseReportSnapshotRequest accepts exactly the declared primitive fields.
// DisallowUnknownFields alone would still permit duplicate and case-alias keys.
func ParseReportSnapshotRequest(raw []byte) (ReportSnapshotRequest, error) {
	var out ReportSnapshotRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return out, reportSnapshotInvalid()
	}
	fields := map[string]json.RawMessage{}
	allowed := map[string]bool{"protocol_version": true, "expected_issue_revision": true, "timezone": true, "start": true, "end": true, "period_mode": true, "correction_of": true}
	for dec.More() {
		key, e := dec.Token()
		name, ok := key.(string)
		if e != nil || !ok || !allowed[name] || fields[name] != nil {
			return out, reportSnapshotInvalid()
		}
		var value json.RawMessage
		if dec.Decode(&value) != nil {
			return out, reportSnapshotInvalid()
		}
		fields[name] = value
	}
	if _, err = dec.Token(); err != nil {
		return out, reportSnapshotInvalid()
	}
	if dec.Decode(new(any)) != io.EOF || len(fields) != len(allowed) {
		return out, reportSnapshotInvalid()
	}
	for key, value := range fields {
		if key != "correction_of" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return out, reportSnapshotInvalid()
		}
	}
	if json.Unmarshal(raw, &out) != nil || out.ProtocolVersion != 1 || out.ExpectedIssueRevision < 1 {
		return out, reportSnapshotInvalid()
	}
	for _, key := range []string{"start", "end"} {
		var value string
		if json.Unmarshal(fields[key], &value) != nil || !strings.HasSuffix(value, "Z") {
			return out, reportSnapshotInvalid()
		}
	}
	if out.Start.Nanosecond()%1000 != 0 || out.End.Nanosecond()%1000 != 0 {
		return out, reportSnapshotInvalid()
	}
	if out.CorrectionOf != nil && !CanonicalReportSnapshotUUID(*out.CorrectionOf) {
		return out, reportSnapshotInvalid()
	}
	// The database cut supplies the future-bound check inside the transaction.
	if reportsnapshot.ValidateWindow(out.Window(), out.End) != nil {
		return out, reportSnapshotInvalid()
	}
	return out, nil
}

func CanonicalReportSnapshotUUID(value string) bool {
	id, err := util.ParseUUID(value)
	return err == nil && id.Valid && id != (pgtype.UUID{Valid: true}) && id.String() == value
}
func reportUUID(value string) pgtype.UUID {
	id, err := util.ParseUUID(value)
	if err != nil {
		panic("invalid trusted report UUID")
	}
	return id
}
func reportNullableUUID(value *string) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return reportUUID(*value)
}
func reportUUIDPtr(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	s := value.String()
	return &s
}
func reportDigest(value any) (string, error) {
	raw, err := projecthealth.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// CaptureReportSnapshotInTx borrows the caller's authorized, report-locked RR
// transaction. It never begins, commits, retries, or publishes a business event.
func CaptureReportSnapshotInTx(ctx context.Context, tx pgx.Tx, report db.Issue, project *db.Project, actorID, key string, request ReportSnapshotRequest) (ReportSnapshotResponse, error) {
	q := db.New(tx)
	out := ReportSnapshotResponse{ProtocolVersion: 1}
	if err := q.EnsureReportSnapshotControl(ctx, db.EnsureReportSnapshotControlParams{WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID}); err != nil {
		return out, err
	}
	control, err := q.LockReportSnapshotControl(ctx, db.LockReportSnapshotControlParams{WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, reportSnapshotUnavailable()
		}
		return out, err
	}
	if control.WithdrawnAt.Valid {
		return out, ReportSnapshotError{409, "report_snapshot_withdrawn"}
	}
	digest, err := reportDigest(struct {
		Actor, Workspace, Report string
		Request                  ReportSnapshotRequest
	}{actorID, report.WorkspaceID.String(), report.ID.String(), request})
	if err != nil {
		return out, err
	}
	prior, err := q.GetReportSnapshotByRequest(ctx, db.GetReportSnapshotByRequestParams{WorkspaceID: report.WorkspaceID, CreatedBy: reportUUID(actorID), RequestKey: reportUUID(key)})
	if err == nil {
		if prior.ReportIssueID != report.ID || prior.ProjectID != report.ProjectID || prior.RequestDigest != digest {
			return out, reportSnapshotConflict()
		}
		out.Snapshot, err = ReadReportSnapshot(prior, report)
		if err == nil {
			out.CaptureReceipt, err = captureReceipt(report.ID.String(), key, prior.ID.String(), request)
		}
		out.Replayed = true
		return out, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if request.ExpectedIssueRevision != report.Revision {
		return out, reportSnapshotConflict()
	}
	cut, err := q.GetReportSnapshotReadCut(ctx)
	if err != nil {
		return out, err
	}
	asOf := cut.AsOf.Time.UTC().Truncate(time.Microsecond)
	if reportsnapshot.ValidateWindow(request.Window(), asOf) != nil {
		return out, reportSnapshotInvalid()
	}
	cycle, err := reportDigest(struct {
		Workspace, Report string
		Project           *string
		Window            reportsnapshot.Window
	}{report.WorkspaceID.String(), report.ID.String(), reportUUIDPtr(report.ProjectID), request.Window()})
	if err != nil {
		return out, err
	}
	currentCycle, err := q.GetReportSnapshotCycle(ctx, db.GetReportSnapshotCycleParams{WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID, CycleDigest: cycle})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if request.CorrectionOf == nil {
		if err == nil {
			return out, reportSnapshotConflict()
		}
	} else if err != nil || currentCycle.ID.String() != *request.CorrectionOf || currentCycle.ProjectID != report.ProjectID {
		return out, reportSnapshotConflict()
	}
	sequence := control.CurrentSequence + 1
	if sequence < 1 {
		return out, reportSnapshotUnavailable()
	}
	if control.CurrentSnapshotID.Valid {
		latest, err := q.GetReportSnapshot(ctx, db.GetReportSnapshotParams{WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID, ID: control.CurrentSnapshotID})
		if err != nil || latest.Sequence != control.CurrentSequence {
			return out, reportSnapshotUnavailable()
		}
	}
	cutHash := sha256.Sum256([]byte(cut.ReadCut))
	cutRef := "rr:" + hex.EncodeToString(cutHash[:])
	facts, audit, err := collectReportSnapshotFacts(ctx, tx, report, project, asOf, request.Timezone, cutRef)
	if err != nil {
		return out, err
	}
	input := reportsnapshot.Input{Identity: reportsnapshot.Identity{SnapshotID: dbid.NewV7().String(), WorkspaceID: report.WorkspaceID.String(), ReportIssueID: report.ID.String(), ProjectID: reportUUIDPtr(report.ProjectID), ReportRevision: report.Revision, Sequence: sequence, CreatedBy: actorID, RequestKey: key, CorrectionOf: request.CorrectionOf}, Window: request.Window(), AsOf: asOf, ReadCut: cutRef, Facts: facts}
	snapshot, err := reportsnapshot.Build(input)
	if err != nil {
		return out, reportSnapshotUnavailable()
	}
	public, err := reportsnapshot.ProjectPublic(snapshot)
	if err != nil {
		return out, reportSnapshotUnavailable()
	}
	if err := validateReportSnapshotAudit(audit, snapshot.Input); err != nil {
		return out, reportSnapshotUnavailable()
	}
	auditDigest, err := reportDigest(audit)
	if err != nil {
		return out, err
	}
	stored := reportSnapshotStorage{StorageVersion: 2, Internal: snapshot, Public: public, Audit: audit, AuditDigest: auditDigest}
	raw, err := projecthealth.CanonicalJSON(stored)
	if err != nil || len(raw) > reportsnapshot.MaxSnapshotBytes {
		return out, reportSnapshotUnavailable()
	}
	sealDigest, err := reportSnapshotSealDigest(stored)
	if err != nil {
		return out, err
	}
	_, err = q.CreateReportSnapshot(ctx, db.CreateReportSnapshotParams{ID: reportUUID(snapshot.SnapshotID), WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID, ProjectID: report.ProjectID, ReportRevision: report.Revision, Sequence: sequence, CreatedBy: reportUUID(actorID), RequestKey: reportUUID(key), RequestDigest: digest, CycleDigest: cycle, CorrectionOf: reportNullableUUID(request.CorrectionOf), CapturedAt: pgtype.Timestamptz{Time: asOf, Valid: true}, SnapshotJson: raw, FactsDigest: snapshot.FactsDigest, ResultDigest: snapshot.ResultDigest, SnapshotDigest: snapshot.SnapshotDigest, SealDigest: pgtype.Text{String: sealDigest, Valid: true}})
	if err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == "23505" && strings.HasPrefix(pgerr.ConstraintName, "report_snapshot_") {
			return out, &pgconn.PgError{Code: "40001", Message: "retry complete report snapshot transaction"}
		}
		return out, err
	}
	advanced, err := q.AdvanceReportSnapshotControl(ctx, db.AdvanceReportSnapshotControlParams{SnapshotID: reportUUID(snapshot.SnapshotID), WorkspaceID: report.WorkspaceID, ReportIssueID: report.ID, ExpectedRevision: control.Revision})
	if err != nil {
		return out, err
	}
	if advanced.CurrentSequence != snapshot.Sequence {
		return out, reportSnapshotUnavailable()
	}
	out.Snapshot = public
	out.CaptureReceipt, err = captureReceipt(report.ID.String(), key, snapshot.SnapshotID, request)
	return out, err
}

func ReadReportSnapshot(row db.ReportSnapshot, report db.Issue) (reportsnapshot.PublicSnapshot, error) {
	var empty reportsnapshot.PublicSnapshot
	if row.WorkspaceID != report.WorkspaceID || row.ReportIssueID != report.ID || row.ProjectID != report.ProjectID {
		return empty, ReportSnapshotError{404, "report_snapshot_not_found"}
	}
	stored, err := decodeReportSnapshotStorage(row.SnapshotJson)
	if err != nil {
		return empty, reportSnapshotUnavailable()
	}
	sealDigest, err := reportSnapshotSealDigest(stored)
	if err != nil || !row.SealDigest.Valid || row.SealDigest.String != sealDigest {
		return empty, reportSnapshotUnavailable()
	}
	value := stored.Internal
	if value.SnapshotID != row.ID.String() || value.WorkspaceID != row.WorkspaceID.String() || value.ReportIssueID != row.ReportIssueID.String() || reportNullableUUID(value.ProjectID) != row.ProjectID || value.ReportRevision != row.ReportRevision || value.Sequence != row.Sequence || value.CreatedBy != row.CreatedBy.String() || value.RequestKey != row.RequestKey.String() || reportNullableUUID(value.CorrectionOf) != row.CorrectionOf || !value.AsOf.Equal(row.CapturedAt.Time) || value.FactsDigest != row.FactsDigest || value.ResultDigest != row.ResultDigest || value.SnapshotDigest != row.SnapshotDigest {
		return empty, reportSnapshotUnavailable()
	}
	public := stored.Public
	if public.SnapshotID != value.SnapshotID || public.WorkspaceID != value.WorkspaceID || public.ReportIssueID != value.ReportIssueID || reportNullableUUID(public.ProjectID) != row.ProjectID || public.ReportRevision != value.ReportRevision || reportNullableUUID(public.CorrectionOf) != row.CorrectionOf || !public.AsOf.Equal(value.AsOf) || public.Window != value.Window {
		return empty, reportSnapshotUnavailable()
	}
	internalResult, err := projecthealth.CanonicalJSON(struct {
		Metrics   []reportsnapshot.Metric
		TimeBasis *reportsnapshot.TimeBasis
	}{value.Metrics, value.TimeBasis})
	if err != nil {
		return empty, reportSnapshotUnavailable()
	}
	publicResult, err := projecthealth.CanonicalJSON(struct {
		Metrics   []reportsnapshot.Metric
		TimeBasis *reportsnapshot.TimeBasis
	}{public.Metrics, public.TimeBasis})
	if err != nil || !bytes.Equal(internalResult, publicResult) {
		return empty, reportSnapshotUnavailable()
	}
	return public, nil
}

// A retained public projection is never rebuilt during GET. Each nested decoder
// validates its own supported immutable format and content digests.
func decodeReportSnapshotStorage(raw []byte) (reportSnapshotStorage, error) {
	var out reportSnapshotStorage
	if len(raw) > reportsnapshot.MaxSnapshotBytes {
		return out, reportSnapshotUnavailable()
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return out, reportSnapshotUnavailable()
	}
	fields := map[string]json.RawMessage{}
	for dec.More() {
		token, err := dec.Token()
		key, ok := token.(string)
		if err != nil || !ok || (key != "storage_version" && key != "internal" && key != "public" && key != "audit" && key != "audit_digest") || fields[key] != nil {
			return out, reportSnapshotUnavailable()
		}
		var value json.RawMessage
		if dec.Decode(&value) != nil {
			return out, reportSnapshotUnavailable()
		}
		fields[key] = value
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') || len(fields) != 5 || dec.Decode(new(any)) != io.EOF || json.Unmarshal(fields["storage_version"], &out.StorageVersion) != nil || out.StorageVersion != 2 {
		return out, reportSnapshotUnavailable()
	}
	out.Internal, err = reportsnapshot.Decode(fields["internal"])
	if err != nil {
		return out, err
	}
	out.Public, err = reportsnapshot.DecodePublic(fields["public"])
	if err != nil {
		return out, err
	}
	auditDecoder := json.NewDecoder(bytes.NewReader(fields["audit"]))
	auditDecoder.DisallowUnknownFields()
	if auditDecoder.Decode(&out.Audit) != nil || auditDecoder.Decode(new(any)) != io.EOF || json.Unmarshal(fields["audit_digest"], &out.AuditDigest) != nil {
		return out, reportSnapshotUnavailable()
	}
	digest, err := reportDigest(out.Audit)
	if err != nil || digest != out.AuditDigest || validateReportSnapshotAudit(out.Audit, out.Internal.Input) != nil {
		return out, reportSnapshotUnavailable()
	}
	return out, nil
}
