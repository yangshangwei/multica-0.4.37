package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/internal/reportsnapshot"
	"github.com/spf13/cobra"
)

const reportSnapshotTimeFormat = "2006-01-02T15:04:05.000000Z"

var issueReportSnapshotCmd = newIssueReportSnapshotCommand()

type reportSnapshotCaptureRequest struct {
	ProtocolVersion       int     `json:"protocol_version"`
	ExpectedIssueRevision int64   `json:"expected_issue_revision"`
	Timezone              string  `json:"timezone"`
	Start                 string  `json:"start"`
	End                   string  `json:"end"`
	PeriodMode            string  `json:"period_mode"`
	CorrectionOf          *string `json:"correction_of"`
}

type reportSnapshotEnvelope struct {
	ProtocolVersion int                           `json:"protocol_version"`
	Snapshot        reportsnapshot.PublicSnapshot `json:"snapshot"`
	Replayed        bool                          `json:"replayed"`
	CaptureReceipt  *reportSnapshotCaptureReceipt `json:"capture_receipt,omitempty"`
}

type reportSnapshotCaptureReceipt struct {
	RequestKey    string `json:"request_key"`
	ReportIssueID string `json:"report_issue_id"`
	SnapshotID    string `json:"snapshot_id"`
	RequestDigest string `json:"request_digest"`
}

type reportSnapshotWithdrawalRequest struct {
	ProtocolVersion           int    `json:"protocol_version"`
	Scope                     string `json:"scope"`
	ExpectedIssueRevision     int64  `json:"expected_issue_revision"`
	ExpectedCurrentSnapshotID string `json:"expected_current_snapshot_id"`
	Reason                    string `json:"reason"`
}

type reportSnapshotWithdrawalReceipt struct {
	WithdrawalID              string    `json:"withdrawal_id"`
	RequestKey                string    `json:"request_key"`
	ReportIssueID             string    `json:"report_issue_id"`
	ExpectedCurrentSnapshotID string    `json:"expected_current_snapshot_id"`
	RequestDigest             string    `json:"request_digest"`
	WithdrawnAt               time.Time `json:"withdrawn_at"`
}

type reportSnapshotWithdrawalEnvelope struct {
	ProtocolVersion   int                             `json:"protocol_version"`
	WithdrawalReceipt reportSnapshotWithdrawalReceipt `json:"withdrawal_receipt"`
	Replayed          bool                            `json:"replayed"`
}

func newIssueReportSnapshotCommand() *cobra.Command {
	command := &cobra.Command{Use: "report-snapshot", Short: "Capture or read immutable report facts"}
	capture := &cobra.Command{
		Use:   "capture <report-uuid>",
		Short: "Explicitly capture report facts with a human credential",
		Long: "Capture a bounded snapshot without publishing a report or changing an issue. " +
			"Retain the original request key and exact flags if the outcome is uncertain. " +
			"The command never chooses another key or retries automatically.",
		Args: exactArgs(1), RunE: runIssueReportSnapshotCapture,
	}
	capture.Flags().String("request-key", "", "Original canonical UUID for this capture intent (required)")
	capture.Flags().Int64("expected-revision", 0, "Expected report issue revision (required)")
	capture.Flags().String("timezone", "", "IANA reporting timezone (required)")
	capture.Flags().String("start", "", "Inclusive UTC start with six fractional digits and Z (required)")
	capture.Flags().String("end", "", "Exclusive UTC end with six fractional digits and Z (required)")
	capture.Flags().String("period-mode", "", "explicit, rolling_24h, rolling_7d, calendar_day, or calendar_week (required)")
	capture.Flags().String("correction-of", "", "Snapshot UUID explicitly corrected by this new capture")
	capture.Flags().String("output", "json", "Output format: json or table")
	show := &cobra.Command{
		Use: "show <report-uuid>", Short: "Read a sealed snapshot without collecting or changing facts",
		Args: exactArgs(1), RunE: runIssueReportSnapshotShow,
	}
	show.Flags().String("snapshot", "", "Exact snapshot UUID; defaults to the current sealed snapshot")
	show.Flags().String("output", "json", "Output format: json or table")
	withdraw := &cobra.Command{
		Use: "withdraw <report-uuid>", Short: "Permanently stop access to all snapshots of this report",
		Long: "Withdraw all snapshot content for this report and prevent further capture or correction. " +
			"This does not delete the report or erase backups. Repeat only the original request key and exact flags to recover its receipt; no automatic retry is performed.",
		Args: exactArgs(1), RunE: runIssueReportSnapshotWithdraw,
	}
	withdraw.Flags().String("request-key", "", "Original canonical UUID for this withdrawal intent (required)")
	withdraw.Flags().Int64("expected-revision", 0, "Expected report issue revision (required)")
	withdraw.Flags().String("current-snapshot", "", "Exact current snapshot UUID reviewed for withdrawal (required)")
	withdraw.Flags().String("reason", "", "Reason for withdrawing this report content (required)")
	withdraw.Flags().Bool("accept", false, "Explicitly accept permanent report-wide snapshot withdrawal (required)")
	withdraw.Flags().String("output", "json", "Output format: json or table")
	command.AddCommand(capture, show, withdraw)
	return command
}

func reportSnapshotUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func reportSnapshotOptions(cmd *cobra.Command, reportID string) (string, error) {
	if !reportSnapshotUUID(reportID) {
		return "", errors.New("report ID must be a canonical nonzero UUID; issue identifiers and prefix lookup are not supported")
	}
	output, _ := cmd.Flags().GetString("output")
	if output != "json" && output != "table" {
		return "", errors.New("--output must be json or table")
	}
	return output, nil
}

func reportSnapshotCaptureOptions(cmd *cobra.Command) (reportSnapshotCaptureRequest, string, reportsnapshot.Window, error) {
	request := reportSnapshotCaptureRequest{ProtocolVersion: reportsnapshot.ProtocolVersion}
	key, _ := cmd.Flags().GetString("request-key")
	if !reportSnapshotUUID(key) {
		return request, "", reportsnapshot.Window{}, errors.New("--request-key must be an explicitly supplied canonical nonzero UUID")
	}
	request.ExpectedIssueRevision, _ = cmd.Flags().GetInt64("expected-revision")
	if request.ExpectedIssueRevision <= 0 {
		return request, "", reportsnapshot.Window{}, errors.New("--expected-revision must be positive")
	}
	request.Timezone, _ = cmd.Flags().GetString("timezone")
	request.Start, _ = cmd.Flags().GetString("start")
	request.End, _ = cmd.Flags().GetString("end")
	request.PeriodMode, _ = cmd.Flags().GetString("period-mode")
	correction, _ := cmd.Flags().GetString("correction-of")
	if correction != "" {
		if !reportSnapshotUUID(correction) {
			return request, "", reportsnapshot.Window{}, errors.New("--correction-of must be a canonical nonzero UUID")
		}
		request.CorrectionOf = &correction
	}
	start, startErr := time.Parse(reportSnapshotTimeFormat, request.Start)
	end, endErr := time.Parse(reportSnapshotTimeFormat, request.End)
	if startErr != nil || endErr != nil || start.Format(reportSnapshotTimeFormat) != request.Start || end.Format(reportSnapshotTimeFormat) != request.End {
		return request, "", reportsnapshot.Window{}, errors.New("--start and --end must be UTC timestamps with exactly six fractional digits and Z")
	}
	window := reportsnapshot.Window{Start: start, End: end, Timezone: request.Timezone, PeriodMode: request.PeriodMode}
	// The server's capture cut is authoritative; local clock skew must not move
	// or reject an otherwise valid requested window.
	if err := reportsnapshot.ValidateWindow(window, end); err != nil {
		return request, "", window, fmt.Errorf("invalid report window: %w", err)
	}
	return request, key, window, nil
}

func reportSnapshotClient(cmd *cobra.Command, writeAction string) (*cli.APIClient, error) {
	if writeAction != "" {
		if err := requireHumanLocalCommand("issue report-snapshot " + writeAction); err != nil {
			return nil, err
		}
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return nil, err
	}
	if writeAction != "" && (strings.HasPrefix(client.Token, cli.TaskTokenPrefix) || strings.HasPrefix(client.Token, "mdt_") || strings.HasPrefix(client.Token, auth.CloudPATPrefix) || client.AgentID != "" || client.TaskID != "") {
		return nil, fmt.Errorf("report snapshot %s requires a human credential; Reporter credentials may only show sealed snapshots", writeAction)
	}
	if client.WorkspaceID != "" && !reportSnapshotUUID(client.WorkspaceID) {
		return nil, errors.New("configured workspace ID must be a canonical nonzero UUID")
	}
	return client, nil
}

func runIssueReportSnapshotCapture(cmd *cobra.Command, args []string) error {
	output, err := reportSnapshotOptions(cmd, args[0])
	if err != nil {
		return err
	}
	request, key, window, err := reportSnapshotCaptureOptions(cmd)
	if err != nil {
		return err
	}
	client, err := reportSnapshotClient(cmd, "capture")
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	raw, status, err := reportSnapshotRequest(ctx, client, http.MethodPost, args[0], "", key, request)
	if err != nil {
		return fmt.Errorf("capture report snapshot: %w; keep the original --request-key and exact capture flags if the outcome is uncertain; no automatic retry was performed", err)
	}
	out, err := decodeReportSnapshotEnvelope(raw)
	if err == nil {
		snapshot := out.Snapshot
		digest, digestErr := reportSnapshotCaptureDigest(args[0], key, request)
		receipt := out.CaptureReceipt
		if digestErr != nil || receipt == nil || receipt.RequestKey != key || receipt.ReportIssueID != args[0] || receipt.SnapshotID != snapshot.SnapshotID || receipt.RequestDigest != digest ||
			snapshot.ReportIssueID != args[0] || snapshot.ReportRevision != request.ExpectedIssueRevision ||
			!snapshot.Window.Start.Equal(window.Start) || !snapshot.Window.End.Equal(window.End) || snapshot.Window.Timezone != window.Timezone || snapshot.Window.PeriodMode != window.PeriodMode ||
			!reportSnapshotOptionalIDEqual(snapshot.CorrectionOf, request.CorrectionOf) || (client.WorkspaceID != "" && snapshot.WorkspaceID != client.WorkspaceID) ||
			(status == http.StatusCreated && out.Replayed) || (status == http.StatusOK && !out.Replayed) {
			err = errors.New("snapshot does not match the original capture intent or replay status")
		}
	}
	if err != nil {
		return fmt.Errorf("capture result unconfirmed: %w; retain the original --request-key and exact capture flags; do not select a new key or correction", err)
	}
	return printReportSnapshot(cmd.OutOrStdout(), output, out)
}

func runIssueReportSnapshotShow(cmd *cobra.Command, args []string) error {
	output, err := reportSnapshotOptions(cmd, args[0])
	if err != nil {
		return err
	}
	snapshotID, _ := cmd.Flags().GetString("snapshot")
	if snapshotID != "" && !reportSnapshotUUID(snapshotID) {
		return errors.New("--snapshot must be a canonical nonzero UUID")
	}
	selector := snapshotID
	if selector == "" {
		selector = "current"
	}
	client, err := reportSnapshotClient(cmd, "")
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	raw, _, err := reportSnapshotRequest(ctx, client, http.MethodGet, args[0], selector, "", nil)
	if err != nil {
		return fmt.Errorf("read sealed report snapshot: %w; no snapshot was generated", err)
	}
	out, err := decodeReportSnapshotEnvelope(raw)
	if err != nil {
		return err
	}
	if out.CaptureReceipt != nil || out.Replayed || out.Snapshot.ReportIssueID != args[0] || (snapshotID != "" && out.Snapshot.SnapshotID != snapshotID) ||
		(client.WorkspaceID != "" && out.Snapshot.WorkspaceID != client.WorkspaceID) {
		return errors.New("snapshot response does not match the requested report, snapshot, or workspace")
	}
	return printReportSnapshot(cmd.OutOrStdout(), output, out)
}

func reportSnapshotOptionalIDEqual(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func runIssueReportSnapshotWithdraw(cmd *cobra.Command, args []string) error {
	output, err := reportSnapshotOptions(cmd, args[0])
	if err != nil {
		return err
	}
	accepted, _ := cmd.Flags().GetBool("accept")
	if !accepted {
		return errors.New("--accept is required to withdraw all snapshot content and stop future capture for this report")
	}
	key, _ := cmd.Flags().GetString("request-key")
	revision, _ := cmd.Flags().GetInt64("expected-revision")
	current, _ := cmd.Flags().GetString("current-snapshot")
	reason, _ := cmd.Flags().GetString("reason")
	if !reportSnapshotUUID(key) || !reportSnapshotUUID(current) || revision <= 0 {
		return errors.New("withdrawal requires an explicit canonical --request-key, --current-snapshot and positive --expected-revision")
	}
	if strings.TrimSpace(reason) == "" || len(reason) > 4096 || strings.ContainsRune(reason, '\x00') {
		return errors.New("--reason must contain nonblank text of at most 4096 bytes without NUL")
	}
	request := reportSnapshotWithdrawalRequest{ProtocolVersion: 1, Scope: "report", ExpectedIssueRevision: revision, ExpectedCurrentSnapshotID: current, Reason: reason}
	client, err := reportSnapshotClient(cmd, "withdraw")
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	raw, status, err := reportSnapshotRequest(ctx, client, http.MethodPost, args[0], "withdrawals", key, request)
	if err != nil {
		return fmt.Errorf("withdraw report snapshots: %w; keep the original --request-key and exact withdrawal flags if the outcome is uncertain; no automatic retry was performed", err)
	}
	out, err := decodeReportSnapshotWithdrawal(raw)
	if err == nil {
		digest, digestErr := reportSnapshotIntentDigest(args[0], key, request)
		receipt := out.WithdrawalReceipt
		if digestErr != nil || receipt.RequestKey != key || receipt.ReportIssueID != args[0] || receipt.ExpectedCurrentSnapshotID != current || receipt.RequestDigest != digest ||
			(status == http.StatusCreated && out.Replayed) || (status == http.StatusOK && !out.Replayed) {
			err = errors.New("withdrawal receipt does not match the original intent or replay status")
		}
	}
	if err != nil {
		return fmt.Errorf("withdrawal result unconfirmed: %w; retain the original --request-key and exact withdrawal flags", err)
	}
	if output == "json" {
		return cli.PrintJSON(cmd.OutOrStdout(), out)
	}
	table := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	for _, row := range [][2]string{{"WITHDRAWAL", out.WithdrawalReceipt.WithdrawalID}, {"REPORT", out.WithdrawalReceipt.ReportIssueID}, {"STATE", "withdrawn"}, {"WITHDRAWN AT", out.WithdrawalReceipt.WithdrawnAt.Format(reportSnapshotTimeFormat)}, {"REPLAYED", fmt.Sprint(out.Replayed)}} {
		if _, err := fmt.Fprintf(table, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	return table.Flush()
}

// The server commits typed UTC timestamps, not the six-digit CLI flag spelling.
// Keep this public intent commitment separate from server actor/idempotency data.
func reportSnapshotCaptureDigest(reportID, key string, request reportSnapshotCaptureRequest) (string, error) {
	start, err := time.Parse(reportSnapshotTimeFormat, request.Start)
	if err != nil {
		return "", err
	}
	end, err := time.Parse(reportSnapshotTimeFormat, request.End)
	if err != nil {
		return "", err
	}
	typed := struct {
		ProtocolVersion       int       `json:"protocol_version"`
		ExpectedIssueRevision int64     `json:"expected_issue_revision"`
		Timezone              string    `json:"timezone"`
		Start                 time.Time `json:"start"`
		End                   time.Time `json:"end"`
		PeriodMode            string    `json:"period_mode"`
		CorrectionOf          *string   `json:"correction_of"`
	}{request.ProtocolVersion, request.ExpectedIssueRevision, request.Timezone, start, end, request.PeriodMode, request.CorrectionOf}
	return reportSnapshotIntentDigest(reportID, key, typed)
}

func reportSnapshotIntentDigest(reportID, key string, request any) (string, error) {
	raw, err := reportsnapshot.CanonicalJSON(struct {
		ReportIssueID string `json:"report_issue_id"`
		RequestKey    string `json:"request_key"`
		Request       any    `json:"request"`
	}{reportID, key, request})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

// Keep the response bounded and read all bytes before decoding. The shared
// APIClient supplies configuration and transport; this endpoint needs exact
// status/redirect/whole-body semantics that the generic JSON helpers do not.
func reportSnapshotRequest(ctx context.Context, client *cli.APIClient, method, reportID, selector, key string, body any) ([]byte, int, error) {
	path := "/api/issues/" + reportID + "/report-snapshots"
	if selector != "" {
		path += "/" + selector
	}
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, client.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Capabilities", "stable_attachment_urls")
	for name, value := range map[string]string{
		"X-Workspace-ID": client.WorkspaceID, "X-Agent-ID": client.AgentID, "X-Task-ID": client.TaskID,
		"X-Client-Platform": cli.ClientPlatform, "X-Client-Version": cli.ClientVersion, "X-Client-OS": cli.ClientOS,
	} {
		if value != "" {
			req.Header.Set(name, value)
		}
	}
	if client.Token != "" {
		req.Header.Set("Authorization", "Bearer "+client.Token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
		req.GetBody = nil
	}
	httpClient := *client.HTTPClient
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	status := response.StatusCode
	if status != http.StatusOK && !(method == http.MethodPost && status == http.StatusCreated) {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, status, &cli.HTTPError{Method: method, Path: path, StatusCode: status, Body: string(errorBody), TaskScoped: strings.HasPrefix(client.Token, cli.TaskTokenPrefix)}
	}
	const maxEnvelopeBytes = reportsnapshot.MaxSnapshotBytes + 2048
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxEnvelopeBytes+1))
	if err != nil {
		return nil, status, err
	}
	if len(raw) > maxEnvelopeBytes {
		return nil, status, errors.New("report snapshot response exceeds the supported size")
	}
	return raw, status, nil
}

func decodeReportSnapshotEnvelope(raw []byte) (reportSnapshotEnvelope, error) {
	var out reportSnapshotEnvelope
	fields, err := reportSnapshotResponseFields(raw, "protocol_version", "snapshot", "replayed", "capture_receipt")
	if err != nil {
		return out, err
	}
	var version *int
	var replayed *bool
	if json.Unmarshal(fields["protocol_version"], &version) != nil || version == nil || *version != reportsnapshot.ProtocolVersion ||
		json.Unmarshal(fields["replayed"], &replayed) != nil || replayed == nil {
		return out, errors.New("report snapshot response is missing a supported protocol or explicit replay status")
	}
	snapshot, err := reportsnapshot.DecodePublic(fields["snapshot"])
	if err != nil {
		return out, fmt.Errorf("invalid sealed public report snapshot: %w", err)
	}
	out = reportSnapshotEnvelope{ProtocolVersion: *version, Snapshot: snapshot, Replayed: *replayed}
	if rawReceipt, present := fields["capture_receipt"]; present {
		receiptFields, err := reportSnapshotResponseFields(rawReceipt, "request_key", "report_issue_id", "snapshot_id", "request_digest")
		if err != nil || len(receiptFields) != 4 {
			return reportSnapshotEnvelope{}, errors.New("invalid report snapshot capture receipt")
		}
		for _, value := range receiptFields {
			var field string
			if json.Unmarshal(value, &field) != nil || field == "" {
				return reportSnapshotEnvelope{}, errors.New("invalid report snapshot capture receipt field")
			}
		}
		var receipt reportSnapshotCaptureReceipt
		if json.Unmarshal(rawReceipt, &receipt) != nil || !reportSnapshotUUID(receipt.RequestKey) || !reportSnapshotUUID(receipt.ReportIssueID) || !reportSnapshotUUID(receipt.SnapshotID) {
			return reportSnapshotEnvelope{}, errors.New("invalid report snapshot capture receipt identity")
		}
		out.CaptureReceipt = &receipt
	}
	return out, nil
}

func reportSnapshotResponseFields(raw []byte, allowed ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return nil, errors.New("invalid report snapshot response object")
	}
	fields := make(map[string]json.RawMessage)
	names := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		names[name] = true
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, errors.New("invalid report snapshot response object")
		}
		name, ok := token.(string)
		if !ok || !names[name] || fields[name] != nil {
			return nil, errors.New("unsupported, duplicate or ambiguous report snapshot response field")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errors.New("invalid report snapshot response field")
		}
		fields[name] = value
	}
	if close, err := decoder.Token(); err != nil || close != json.Delim('}') {
		return nil, errors.New("invalid report snapshot response object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("trailing data in report snapshot response")
	}
	return fields, nil
}

func decodeReportSnapshotWithdrawal(raw []byte) (reportSnapshotWithdrawalEnvelope, error) {
	var out reportSnapshotWithdrawalEnvelope
	fields, err := reportSnapshotResponseFields(raw, "protocol_version", "withdrawal_receipt", "replayed")
	if err != nil || len(fields) != 3 {
		return out, errors.New("invalid report snapshot withdrawal response")
	}
	var version *int
	var replayed *bool
	if json.Unmarshal(fields["protocol_version"], &version) != nil || version == nil || *version != 1 || json.Unmarshal(fields["replayed"], &replayed) != nil || replayed == nil {
		return out, errors.New("withdrawal response is missing a supported protocol or explicit replay status")
	}
	receiptFields, err := reportSnapshotResponseFields(fields["withdrawal_receipt"], "withdrawal_id", "request_key", "report_issue_id", "expected_current_snapshot_id", "request_digest", "withdrawn_at")
	if err != nil || len(receiptFields) != 6 {
		return out, errors.New("invalid report snapshot withdrawal receipt")
	}
	for _, value := range receiptFields {
		var field string
		if json.Unmarshal(value, &field) != nil || field == "" {
			return out, errors.New("invalid report snapshot withdrawal receipt field")
		}
	}
	var receipt reportSnapshotWithdrawalReceipt
	if err := json.Unmarshal(fields["withdrawal_receipt"], &receipt); err != nil || !reportSnapshotUUID(receipt.WithdrawalID) || !reportSnapshotUUID(receipt.RequestKey) || !reportSnapshotUUID(receipt.ReportIssueID) || !reportSnapshotUUID(receipt.ExpectedCurrentSnapshotID) || receipt.WithdrawnAt.IsZero() || receipt.WithdrawnAt.Nanosecond()%1000 != 0 {
		return out, errors.New("invalid report snapshot withdrawal receipt identity or time")
	}
	var timestamp string
	_ = json.Unmarshal(receiptFields["withdrawn_at"], &timestamp)
	if !strings.HasSuffix(timestamp, "Z") {
		return out, errors.New("withdrawal receipt timestamp must be UTC")
	}
	return reportSnapshotWithdrawalEnvelope{ProtocolVersion: *version, WithdrawalReceipt: receipt, Replayed: *replayed}, nil
}

func printReportSnapshot(writer io.Writer, output string, envelope reportSnapshotEnvelope) error {
	if output == "json" {
		return cli.PrintJSON(writer, envelope)
	}
	snapshot := envelope.Snapshot
	table := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	for _, row := range [][2]string{
		{"SNAPSHOT", snapshot.SnapshotID}, {"REPORT", snapshot.ReportIssueID}, {"STATE", snapshot.State},
		{"WINDOW", snapshot.Window.Start.Format(reportSnapshotTimeFormat) + " / " + snapshot.Window.End.Format(reportSnapshotTimeFormat)},
		{"TIMEZONE", snapshot.Window.Timezone}, {"AS OF", snapshot.AsOf.Format(reportSnapshotTimeFormat)}, {"RESULT DIGEST", snapshot.ResultDigest},
	} {
		if _, err := fmt.Fprintf(table, "%s\t%s\n", row[0], row[1]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(table, "METRIC\tQUALITY\tVALUE"); err != nil {
		return err
	}
	for _, metric := range snapshot.Metrics {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\n", metric.ID, metric.Quality, metric.Value); err != nil {
			return err
		}
	}
	return table.Flush()
}
