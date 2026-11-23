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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

const issueCompletionProtocolVersion = 1

var issueCompletionCmd = &cobra.Command{Use: "completion", Short: "Review and authorize one issue completion"}
var issueCompletionPrepareCmd = &cobra.Command{Use: "prepare <id>", Short: "List or review delivery evidence", Args: exactArgs(1), RunE: runIssueCompletionPrepare}
var issueCompletionAuthorizeCmd = &cobra.Command{Use: "authorize", Short: "Explicitly authorize one issue version", Args: cobra.NoArgs, RunE: runIssueCompletionAuthorize}
var issueCompletionSubmitCmd = &cobra.Command{Use: "submit", Short: "Submit an explicitly authorized completion", Args: cobra.NoArgs, RunE: runIssueCompletionSubmit}
var issueCompletionShowCmd = &cobra.Command{Use: "show", Short: "Read an authorization and current issue state", Args: cobra.NoArgs, RunE: runIssueCompletionShow}
var issueCompletionRevokeCmd = &cobra.Command{Use: "revoke", Short: "Explicitly revoke an issued authorization", Args: cobra.NoArgs, RunE: runIssueCompletionRevoke}

type completionCLIUserResponse struct {
	ID string `json:"id"`
}
type completionCLIIssueResponse struct {
	ID                string                `json:"id"`
	Identifier        string                `json:"identifier"`
	Revision          int64                 `json:"revision"`
	Status            string                `json:"status"`
	CompletionReceipt *completionCLIReceipt `json:"completion_receipt,omitempty"`
}
type completionCLICandidate struct {
	ApprovalID     string    `json:"approval_id"`
	ExecutionID    string    `json:"execution_id"`
	SourceRevision string    `json:"source_revision"`
	SpecDigest     string    `json:"spec_digest"`
	ArtifactDigest *string   `json:"artifact_digest"`
	ReceiptDigest  *string   `json:"receipt_digest"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Summary        string    `json:"summary"`
	Status         string    `json:"status"`
	ResultCode     *string   `json:"result_code"`
	Selectable     bool      `json:"selectable"`
}
type completionCLICandidates struct {
	ProtocolVersion int                      `json:"protocol_version"`
	IssueID         string                   `json:"issue_id"`
	IssueRevision   int64                    `json:"issue_revision"`
	ProjectID       *string                  `json:"project_id"`
	CheckedAt       time.Time                `json:"checked_at"`
	Items           []completionCLICandidate `json:"items"`
	NextCursor      *string                  `json:"next_cursor"`
	HasMore         bool                     `json:"has_more"`
}
type completionCLITarget struct {
	Kind                  string   `json:"kind"`
	IssueID               string   `json:"issue_id"`
	ExpectedIssueRevision int64    `json:"expected_issue_revision"`
	ExpectedStatus        string   `json:"expected_status"`
	TargetStatus          string   `json:"target_status"`
	TargetPosition        *float64 `json:"target_position"`
}
type completionCLIProjectBinding struct {
	ID                     string `json:"id"`
	ExpectedRevision       int64  `json:"expected_revision"`
	ExpectedPolicyRevision int64  `json:"expected_policy_revision"`
}
type completionCLIEvidence struct {
	IssueID                string `json:"issue_id"`
	ExpectedIssueRevision  int64  `json:"expected_issue_revision"`
	ApprovalID             string `json:"approval_id"`
	ExpectedSourceRevision string `json:"expected_source_revision"`
	ExpectedSpecDigest     string `json:"expected_spec_digest"`
	ExpectedArtifactDigest string `json:"expected_artifact_digest"`
}
type completionCLIConsumer struct {
	Kind   string `json:"kind"`
	UserID string `json:"user_id"`
}
type completionCLIBinding struct {
	ContractVersion   string                      `json:"contract_version"`
	SourceWorkspaceID string                      `json:"source_workspace_id"`
	Target            completionCLITarget         `json:"target"`
	Project           completionCLIProjectBinding `json:"project"`
	Evidence          completionCLIEvidence       `json:"evidence"`
	Consumer          completionCLIConsumer       `json:"consumer"`
}
type completionCLIObservation struct {
	Verdict            string     `json:"verdict"`
	Reason             string     `json:"reason"`
	IssueID            string     `json:"issue_id"`
	IssueRevision      int64      `json:"issue_revision"`
	ProjectID          *string    `json:"project_id"`
	ProjectRevision    *int64     `json:"project_revision"`
	ApprovalID         string     `json:"approval_id"`
	ExecutionID        string     `json:"execution_id"`
	SourceRevision     string     `json:"source_revision"`
	ObservedHeadSHA    string     `json:"observed_head_sha"`
	RevisionProvenance string     `json:"revision_provenance"`
	SpecDigest         string     `json:"spec_digest"`
	ArtifactDigest     string     `json:"artifact_digest"`
	ReceiptDigest      string     `json:"receipt_digest"`
	ObservedAt         time.Time  `json:"observed_at"`
	ValidUntil         *time.Time `json:"valid_until"`
}
type completionCLIPreview struct {
	ProtocolVersion      int                       `json:"protocol_version"`
	IssueID              string                    `json:"issue_id"`
	IssueRevision        int64                     `json:"issue_revision"`
	CurrentStatus        string                    `json:"current_status"`
	CurrentCategory      string                    `json:"current_category"`
	TargetStatus         string                    `json:"target_status"`
	TargetCategory       string                    `json:"target_category"`
	ProjectID            *string                   `json:"project_id"`
	ProjectRevision      *int64                    `json:"project_revision"`
	PolicyMode           string                    `json:"policy_mode"`
	PolicyRevision       int64                     `json:"policy_revision"`
	CheckedAt            time.Time                 `json:"checked_at"`
	Reason               string                    `json:"reason"`
	Observation          *completionCLIObservation `json:"observation"`
	AuthorizationBinding *completionCLIBinding     `json:"authorization_binding"`
	MaxExpiresAt         *time.Time                `json:"max_expires_at"`
	Enforced             bool                      `json:"enforced"`
	UsableForCompletion  bool                      `json:"usable_for_completion"`
	SkipSupported        bool                      `json:"skip_supported"`
}
type completionCLIAuthorizationRequest struct {
	ContractVersion   string                      `json:"contract_version"`
	SourceWorkspaceID string                      `json:"source_workspace_id"`
	OperationID       string                      `json:"operation_id"`
	Target            completionCLITarget         `json:"target"`
	Project           completionCLIProjectBinding `json:"project"`
	Evidence          completionCLIEvidence       `json:"evidence"`
	Consumer          completionCLIConsumer       `json:"consumer"`
	ExpiresAt         string                      `json:"expires_at"`
	Reason            string                      `json:"reason"`
}
type completionCLIReceipt struct {
	AuthorizationID   string    `json:"authorization_id"`
	OperationID       string    `json:"operation_id"`
	IssueID           string    `json:"issue_id"`
	CompletedRevision int64     `json:"completed_revision"`
	TargetStatus      string    `json:"target_status"`
	CompletedAt       time.Time `json:"completed_at"`
	Replayed          bool      `json:"replayed"`
}
type completionCLIAuthorization struct {
	AuthorizationID   string                `json:"authorization_id"`
	OperationID       string                `json:"operation_id"`
	IssueID           string                `json:"issue_id"`
	IssueRevision     int64                 `json:"issue_revision"`
	TargetStatus      string                `json:"target_status"`
	ContractVersion   string                `json:"contract_version"`
	CreatedAt         time.Time             `json:"created_at"`
	ExpiresAt         time.Time             `json:"expires_at"`
	State             string                `json:"state"`
	Applicability     string                `json:"applicability"`
	CompletionReceipt *completionCLIReceipt `json:"completion_receipt,omitempty"`
	Replayed          bool                  `json:"replayed"`
}
type completionCLIScope struct {
	ServerURL   string `json:"server_url"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	IssueID     string `json:"issue_id"`
}
type completionCLIOperation struct {
	Version                      int                                `json:"version"`
	Scope                        completionCLIScope                 `json:"scope"`
	IssueRevision                int64                              `json:"issue_revision"`
	CurrentStatus                string                             `json:"current_status"`
	TargetStatus                 string                             `json:"target_status"`
	Candidate                    completionCLICandidate             `json:"candidate"`
	Preview                      completionCLIPreview               `json:"preview"`
	PreparedAt                   time.Time                          `json:"prepared_at"`
	AuthorizationRequestKey      string                             `json:"authorization_request_key,omitempty"`
	AuthorizationRequest         *completionCLIAuthorizationRequest `json:"authorization_request,omitempty"`
	AuthorizationRequestDigest   string                             `json:"authorization_request_digest,omitempty"`
	AuthorizationDispatchStarted bool                               `json:"authorization_dispatch_started,omitempty"`
	Authorization                *completionCLIAuthorization        `json:"authorization,omitempty"`
	CompletionDispatchStarted    bool                               `json:"completion_dispatch_started,omitempty"`
	RevocationRequestKey         string                             `json:"revocation_request_key,omitempty"`
	RevocationDispatchStarted    bool                               `json:"revocation_dispatch_started,omitempty"`
	RecordDigest                 string                             `json:"record_digest"`
}
type completionCLIEnvelope struct {
	ProtocolVersion   int                         `json:"protocol_version"`
	Outcome           string                      `json:"outcome"`
	Operation         map[string]any              `json:"operation"`
	Candidates        *completionCLICandidates    `json:"candidates,omitempty"`
	Authorization     *completionCLIAuthorization `json:"authorization,omitempty"`
	CompletionReceipt *completionCLIReceipt       `json:"completion_receipt,omitempty"`
	CurrentIssue      *completionCLIIssueResponse `json:"current_issue"`
	CurrentIssueRead  string                      `json:"current_issue_read"`
	Reason            string                      `json:"reason,omitempty"`
}

func registerIssueCompletionCommands() {
	issueCompletionCmd.AddCommand(issueCompletionPrepareCmd, issueCompletionAuthorizeCmd, issueCompletionSubmitCmd, issueCompletionShowCmd, issueCompletionRevokeCmd)
	issueCompletionPrepareCmd.Flags().String("status", "", "Target status key")
	issueCompletionPrepareCmd.Flags().String("approval", "", "Selected evidence approval UUID")
	issueCompletionPrepareCmd.Flags().String("operation-file", "", "Private operation file (required with --approval)")
	issueCompletionPrepareCmd.Flags().Int("limit", 20, "Candidate page size (1-50)")
	issueCompletionPrepareCmd.Flags().String("cursor", "", "Candidate page cursor")
	issueCompletionPrepareCmd.Flags().String("output", "json", "Output format: json or table")
	issueCompletionAuthorizeCmd.Flags().String("operation-file", "", "Prepared operation file")
	issueCompletionAuthorizeCmd.Flags().Bool("accept", false, "Explicitly accept this exact issue version and evidence")
	issueCompletionAuthorizeCmd.Flags().String("reason", "", "Reason for accepting this exact issue version")
	issueCompletionAuthorizeCmd.Flags().String("expires-at", "", "Expiry in UTC RFC3339 with six fractional digits and Z")
	issueCompletionAuthorizeCmd.Flags().String("output", "json", "Output format: json or table")
	issueCompletionSubmitCmd.Flags().String("operation-file", "", "Authorized operation file")
	issueCompletionSubmitCmd.Flags().String("output", "json", "Output format: json or table")
	issueCompletionShowCmd.Flags().String("operation-file", "", "Original operation file")
	issueCompletionShowCmd.Flags().String("authorization-id", "", "Authorization UUID to query")
	issueCompletionShowCmd.Flags().String("request-key", "", "Original authorization request key")
	issueCompletionShowCmd.Flags().String("source-workspace", "", "Source workspace UUID for --request-key")
	issueCompletionShowCmd.Flags().String("output", "json", "Output format: json or table")
	issueCompletionShowCmd.MarkFlagsMutuallyExclusive("operation-file", "authorization-id", "request-key")
	issueCompletionRevokeCmd.Flags().String("operation-file", "", "Original operation file")
	issueCompletionRevokeCmd.Flags().String("reason", "", "Reason for revoking authorization")
	issueCompletionRevokeCmd.Flags().String("output", "json", "Output format: json or table")
}

func completionCLIClient(cmd *cobra.Command) (*cli.APIClient, string, error) {
	if err := requireHumanLocalCommand("issue completion"); err != nil {
		return nil, "", err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return nil, "", err
	}
	if strings.HasPrefix(client.Token, cli.TaskTokenPrefix) || client.AgentID != "" || client.TaskID != "" {
		return nil, "", errors.New("completion authorization requires a human CLI credential")
	}
	if !completionCLIUUID(client.WorkspaceID) {
		return nil, "", errors.New("select a workspace with a canonical workspace UUID before using issue completion")
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	var user completionCLIUserResponse
	if err := client.GetJSON(ctx, "/api/me", &user); err != nil {
		return nil, "", fmt.Errorf("verify human identity: %w", err)
	}
	if !completionCLIUUID(user.ID) {
		return nil, "", errors.New("server did not return a valid human user identity")
	}
	return client, user.ID, nil
}

func runIssueCompletionPrepare(cmd *cobra.Command, args []string) error {
	client, userID, err := completionCLIClient(cmd)
	if err != nil {
		return err
	}
	status, _ := cmd.Flags().GetString("status")
	if err := validateIssueStatus(status); err != nil {
		return err
	}
	approval, _ := cmd.Flags().GetString("approval")
	opPath, _ := cmd.Flags().GetString("operation-file")
	limit, _ := cmd.Flags().GetInt("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	output, _ := cmd.Flags().GetString("output")
	if err := completionOutput(output); err != nil {
		return err
	}
	if limit < 1 || limit > 50 {
		return errors.New("--limit must be between 1 and 50")
	}
	if approval == "" && opPath != "" {
		return errors.New("--operation-file requires --approval")
	}
	if approval != "" && opPath == "" {
		return errors.New("--approval requires --operation-file")
	}
	if approval != "" && !completionCLIUUID(approval) {
		return errors.New("--approval must be a canonical nonzero UUID")
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	ref, err := resolveIssueRef(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve issue: %w", err)
	}
	issue, err := completionFetchIssue(ctx, client, ref.ID)
	if err != nil {
		return fmt.Errorf("read issue: %w", err)
	}
	if approval == "" {
		params := url.Values{"limit": {fmt.Sprint(limit)}}
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		var page completionCLICandidates
		if err := client.GetJSON(ctx, "/api/issues/"+url.PathEscape(issue.ID)+"/delivery-evidence/candidates?"+params.Encode(), &page); err != nil {
			return fmt.Errorf("read evidence candidates: %w", err)
		}
		if page.ProtocolVersion != 1 || page.IssueID != issue.ID || page.IssueRevision < 0 || page.CheckedAt.IsZero() || page.Items == nil {
			return errors.New("candidate response is incomplete or uses an unknown protocol")
		}
		out := completionCLIEnvelope{ProtocolVersion: 1, Outcome: "candidates", Operation: map[string]any{"issue_id": issue.ID, "issue_revision": page.IssueRevision, "target_status": status}, Candidates: &page, CurrentIssueRead: "not_requested"}
		if output == "table" {
			return printCompletionCandidates(page)
		}
		return printCompletionEnvelope(output, out)
	}
	var preview completionCLIPreview
	body := map[string]any{"expected_issue_revision": issue.Revision, "target_status": status, "approval_id": approval}
	if err := client.PostJSON(ctx, "/api/issues/"+url.PathEscape(issue.ID)+"/delivery-evidence/completion-preview", body, &preview); err != nil {
		return fmt.Errorf("read selected evidence preview: %w", err)
	}
	if preview.ProtocolVersion != 1 || preview.IssueID != issue.ID || preview.IssueRevision != issue.Revision || preview.TargetStatus != strings.ToLower(strings.TrimSpace(status)) || preview.CheckedAt.IsZero() {
		return errors.New("completion preview does not match current issue; prepare again")
	}
	if preview.AuthorizationBinding == nil || preview.Reason != "authorization_available" || preview.Observation == nil || preview.MaxExpiresAt == nil || preview.Enforced || preview.UsableForCompletion || preview.SkipSupported {
		out := completionCLIEnvelope{ProtocolVersion: 1, Outcome: "not_applicable", Operation: map[string]any{"issue_id": issue.ID, "issue_revision": preview.IssueRevision, "target_status": preview.TargetStatus}, CurrentIssue: &issue, CurrentIssueRead: "ok", Reason: preview.Reason}
		if err := printCompletionEnvelope(output, out); err != nil {
			return err
		}
		return errors.New("selected evidence is not currently authorizable; no operation file was written")
	}
	scope := completionCLIScope{ServerURL: client.BaseURL, WorkspaceID: client.WorkspaceID, UserID: userID, IssueID: issue.ID}
	if !completionBindingValid(scope, preview) {
		return errors.New("completion preview binding is incomplete or mismatched; no operation file was written")
	}
	item := completionCLICandidate{ApprovalID: approval, Selectable: true, Status: preview.Observation.Verdict, Summary: preview.Observation.Reason, SourceRevision: preview.Observation.SourceRevision, SpecDigest: preview.Observation.SpecDigest, ArtifactDigest: completionStringPtr(preview.Observation.ArtifactDigest), ReceiptDigest: completionStringPtr(preview.Observation.ReceiptDigest)}
	op := completionCLIOperation{Version: 1, Scope: scope, IssueRevision: issue.Revision, CurrentStatus: issue.Status, TargetStatus: preview.TargetStatus, Candidate: item, Preview: preview, PreparedAt: time.Now().UTC()}
	if err := createCompletionOperation(opPath, &op); err != nil {
		return err
	}
	return printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: "prepared", Operation: completionOperationView(&op), CurrentIssue: &issue, CurrentIssueRead: "ok"})
}

func runIssueCompletionAuthorize(cmd *cobra.Command, _ []string) error {
	client, userID, err := completionCLIClient(cmd)
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("operation-file")
	accept, _ := cmd.Flags().GetBool("accept")
	reason, _ := cmd.Flags().GetString("reason")
	expiryText, _ := cmd.Flags().GetString("expires-at")
	output, _ := cmd.Flags().GetString("output")
	if path == "" || !accept || strings.TrimSpace(reason) == "" || expiryText == "" {
		return errors.New("authorize requires --operation-file, --accept, --reason and --expires-at")
	}
	if err := completionOutput(output); err != nil {
		return err
	}
	unlock, err := lockCompletionOperation(path)
	if err != nil {
		return err
	}
	defer unlock()
	op, err := readCompletionOperation(path)
	if err != nil {
		return err
	}
	if err := completionScopeMatches(client, userID, op.Scope); err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	if op.AuthorizationDispatchStarted {
		grant, err := completionByKey(ctx, client, op.AuthorizationRequestKey, op.Scope.WorkspaceID)
		if err != nil {
			return completionPrintUnknown(output, &op, "authorization result unknown; query only the original key", err)
		}
		if err := completionValidateAuthorization(grant, op.AuthorizationRequest); err != nil {
			return completionPrintUnknown(output, &op, "authorization does not match original request", err)
		}
		op.Authorization = &grant
		if err := saveCompletionOperation(path, &op); err != nil {
			return err
		}
		return printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: completionAuthOutcome(grant), Operation: completionOperationView(&op), Authorization: &grant, CurrentIssueRead: "not_requested"})
	}
	if op.Authorization != nil {
		return errors.New("operation already has an authorization; use submit, show or revoke")
	}
	if err := completionValidateExpiry(expiryText, op.Preview); err != nil {
		return err
	}
	if !completionBindingValid(op.Scope, op.Preview) {
		return errors.New("prepared authorization binding no longer matches the operation")
	}
	binding := op.Preview.AuthorizationBinding
	request := completionCLIAuthorizationRequest{ContractVersion: binding.ContractVersion, SourceWorkspaceID: binding.SourceWorkspaceID, OperationID: uuid.NewString(), Target: binding.Target, Project: binding.Project, Evidence: binding.Evidence, Consumer: binding.Consumer, ExpiresAt: expiryText, Reason: strings.TrimSpace(reason)}
	key := uuid.NewString()
	digest, err := completionRequestDigest(request)
	if err != nil {
		return err
	}
	op.AuthorizationRequestKey, op.AuthorizationRequest, op.AuthorizationRequestDigest, op.AuthorizationDispatchStarted = key, &request, digest, true
	if err := saveCompletionOperation(path, &op); err != nil {
		return fmt.Errorf("save original authorization intent before POST: %w", err)
	}
	var grant completionCLIAuthorization
	if err := client.PostJSONWithIdempotencyKey(ctx, "/api/delivery-completion-authorizations", request, &grant, key); err != nil {
		var he *cli.HTTPError
		if errors.As(err, &he) && he.StatusCode >= 400 && he.StatusCode < 500 {
			out := completionCLIEnvelope{ProtocolVersion: 1, Outcome: "refused", Operation: completionOperationView(&op), CurrentIssueRead: "not_requested", Reason: "authorization was refused; original key is retained"}
			_ = printCompletionEnvelope(output, out)
			return errors.New("authorization was refused; review evidence before preparing a new operation")
		}
		grant, err = completionByKey(ctx, client, key, op.Scope.WorkspaceID)
		if err != nil {
			return completionPrintUnknown(output, &op, "authorization outcome is unknown; do not create another authorization", err)
		}
	}
	if err := completionValidateAuthorization(grant, &request); err != nil {
		return completionPrintUnknown(output, &op, "authorization response does not match immutable request", err)
	}
	op.Authorization = &grant
	if err := saveCompletionOperation(path, &op); err != nil {
		return fmt.Errorf("authorization confirmed; retain original key %s: %w", key, err)
	}
	return printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: completionAuthOutcome(grant), Operation: completionOperationView(&op), Authorization: &grant, CurrentIssueRead: "not_requested"})
}

func runIssueCompletionSubmit(cmd *cobra.Command, _ []string) error {
	client, userID, err := completionCLIClient(cmd)
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("operation-file")
	output, _ := cmd.Flags().GetString("output")
	if path == "" {
		return errors.New("submit requires --operation-file")
	}
	if err := completionOutput(output); err != nil {
		return err
	}
	unlock, err := lockCompletionOperation(path)
	if err != nil {
		return err
	}
	defer unlock()
	op, err := readCompletionOperation(path)
	if err != nil {
		return err
	}
	if err := completionScopeMatches(client, userID, op.Scope); err != nil {
		return err
	}
	if op.Authorization == nil || op.AuthorizationRequest == nil {
		return errors.New("operation has no confirmed authorization; authorize it explicitly first")
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	if op.CompletionDispatchStarted {
		return recoverCompletionCLI(cmd, client, output, path, &op, "completion may have been sent; read-only recovery")
	}
	if op.Authorization.State != "issued" {
		return errors.New("authorization is terminal; use show")
	}
	if op.Authorization.Applicability != "not_checked" || !op.Authorization.ExpiresAt.After(time.Now()) {
		out := completionCLIEnvelope{ProtocolVersion: 1, Outcome: "refused", Operation: completionOperationView(&op), Authorization: op.Authorization, CurrentIssueRead: "not_requested", Reason: "authorization is expired or is not applicable; review current evidence before creating another operation"}
		_ = printCompletionEnvelope(output, out)
		return errors.New("authorization is expired or is not applicable; no completion PUT was sent")
	}
	op.CompletionDispatchStarted = true
	if err := saveCompletionOperation(path, &op); err != nil {
		return fmt.Errorf("save completion dispatch marker before PUT: %w", err)
	}
	body := map[string]any{"status": op.TargetStatus, "completion": map[string]any{"authorization_id": op.Authorization.AuthorizationID, "operation_id": op.AuthorizationRequest.OperationID, "expected_issue_revision": op.IssueRevision}}
	err = completionPutOnce(ctx, client, "/api/issues/"+url.PathEscape(op.Scope.IssueID), body)
	if err != nil {
		var we *completionCLIWriteError
		if errors.As(err, &we) && we.status >= 400 && we.status < 500 {
			grant, readErr := completionAuthorizationByID(ctx, client, op.Authorization.AuthorizationID)
			if readErr == nil && grant.State == "consumed" && completionReceiptMatches(grant.CompletionReceipt, &op) {
				op.Authorization = &grant
				return recoverCompletionCLI(cmd, client, output, path, &op, "original receipt confirmed after PUT response")
			}
			out := completionCLIEnvelope{ProtocolVersion: 1, Outcome: "refused", Operation: completionOperationView(&op), Authorization: op.Authorization, CurrentIssueRead: "unknown", Reason: "completion was refused; no automatic retry will be sent"}
			_ = printCompletionEnvelope(output, out)
			return errors.New("completion was refused; query the original authorization")
		}
		return recoverCompletionCLI(cmd, client, output, path, &op, "completion response is unknown; do not resubmit")
	}
	return recoverCompletionCLI(cmd, client, output, path, &op, "completion response received; verifying original receipt")
}

func runIssueCompletionShow(cmd *cobra.Command, _ []string) error {
	client, userID, err := completionCLIClient(cmd)
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("operation-file")
	authID, _ := cmd.Flags().GetString("authorization-id")
	key, _ := cmd.Flags().GetString("request-key")
	workspace, _ := cmd.Flags().GetString("source-workspace")
	output, _ := cmd.Flags().GetString("output")
	if err := completionOutput(output); err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	if path != "" {
		unlock, err := lockCompletionOperation(path)
		if err != nil {
			return err
		}
		defer unlock()
		op, err := readCompletionOperation(path)
		if err != nil {
			return err
		}
		if err := completionScopeMatches(client, userID, op.Scope); err != nil {
			return err
		}
		if op.Authorization == nil && op.AuthorizationDispatchStarted {
			grant, err := completionByKey(ctx, client, op.AuthorizationRequestKey, op.Scope.WorkspaceID)
			if err != nil {
				return completionPrintUnknown(output, &op, "authorization may have committed; query original key", err)
			}
			if err := completionValidateAuthorization(grant, op.AuthorizationRequest); err != nil {
				return completionPrintUnknown(output, &op, "authorization does not match original request", err)
			}
			op.Authorization = &grant
			if err := saveCompletionOperation(path, &op); err != nil {
				return err
			}
		}
		if op.Authorization == nil {
			issue, e := completionFetchIssue(ctx, client, op.Scope.IssueID)
			var current *completionCLIIssueResponse
			read := "ok"
			if e == nil {
				current = &issue
			} else {
				read = "denied"
			}
			return printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: "prepared", Operation: completionOperationView(&op), CurrentIssue: current, CurrentIssueRead: read, Reason: "no authorization has been dispatched"})
		}
		return showCompletionOperation(cmd, client, output, path, &op)
	}
	if (authID == "") == (key == "") {
		return errors.New("show requires exactly one of --operation-file, --authorization-id, or --request-key")
	}
	var grant completionCLIAuthorization
	if authID != "" {
		if !completionCLIUUID(authID) {
			return errors.New("--authorization-id must be a canonical UUID")
		}
		grant, err = completionAuthorizationByID(ctx, client, authID)
	} else {
		if !completionCLIUUID(key) || !completionCLIUUID(workspace) || workspace != client.WorkspaceID {
			return errors.New("--request-key requires the current canonical --source-workspace")
		}
		grant, err = completionByKey(ctx, client, key, workspace)
	}
	if err != nil {
		return completionUnknown(output, "authorization unavailable under current authority", err)
	}
	if err := completionValidateAuthorization(grant, nil); err != nil {
		return err
	}
	issue, e := completionFetchIssue(ctx, client, grant.IssueID)
	var current *completionCLIIssueResponse
	read := "ok"
	if e == nil {
		current = &issue
	} else {
		read = "denied"
	}
	reason := ""
	if read != "ok" {
		reason = "authorization fact is retained; current issue state is unavailable"
	}
	if err := printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: completionAuthOutcome(grant), Operation: map[string]any{"authorization_id": grant.AuthorizationID, "operation_id": grant.OperationID, "issue_id": grant.IssueID, "request_key": key, "source_workspace_id": workspace}, Authorization: &grant, CompletionReceipt: grant.CompletionReceipt, CurrentIssue: current, CurrentIssueRead: read, Reason: reason}); err != nil {
		return err
	}
	if read != "ok" {
		return errors.New("authorization was read, but current issue state is unavailable")
	}
	return nil
}

func runIssueCompletionRevoke(cmd *cobra.Command, _ []string) error {
	client, userID, err := completionCLIClient(cmd)
	if err != nil {
		return err
	}
	path, _ := cmd.Flags().GetString("operation-file")
	reason, _ := cmd.Flags().GetString("reason")
	output, _ := cmd.Flags().GetString("output")
	if path == "" || strings.TrimSpace(reason) == "" {
		return errors.New("revoke requires --operation-file and --reason")
	}
	if err := completionOutput(output); err != nil {
		return err
	}
	unlock, err := lockCompletionOperation(path)
	if err != nil {
		return err
	}
	defer unlock()
	op, err := readCompletionOperation(path)
	if err != nil {
		return err
	}
	if err := completionScopeMatches(client, userID, op.Scope); err != nil {
		return err
	}
	if op.Authorization == nil {
		return errors.New("operation has no confirmed authorization to revoke")
	}
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	if op.RevocationDispatchStarted {
		return recoverCompletionCLI(cmd, client, output, path, &op, "revocation may have been sent; query original authorization")
	}
	if op.Authorization.State != "issued" {
		return errors.New("authorization is terminal; use show")
	}
	op.RevocationRequestKey = uuid.NewString()
	op.RevocationDispatchStarted = true
	if err := saveCompletionOperation(path, &op); err != nil {
		return fmt.Errorf("save revocation key before POST: %w", err)
	}
	endpoint := "/api/delivery-completion-authorizations/" + url.PathEscape(op.Authorization.AuthorizationID) + "/revocations"
	var grant completionCLIAuthorization
	err = client.PostJSONWithIdempotencyKey(ctx, endpoint, map[string]string{"reason": strings.TrimSpace(reason)}, &grant, op.RevocationRequestKey)
	if err != nil {
		knownRefusal := false
		var he *cli.HTTPError
		if errors.As(err, &he) && he.StatusCode >= 400 && he.StatusCode < 500 {
			knownRefusal = true
		}
		grant, err = completionAuthorizationByID(ctx, client, op.Authorization.AuthorizationID)
		if err != nil {
			return completionPrintUnknown(output, &op, "revocation outcome is unknown; do not resend", err)
		}
		if grant.State == "issued" {
			if knownRefusal {
				out := completionCLIEnvelope{ProtocolVersion: 1, Outcome: "refused", Operation: completionOperationView(&op), Authorization: &grant, CurrentIssueRead: "not_requested", Reason: "revocation was refused; authorization remains issued"}
				_ = printCompletionEnvelope(output, out)
				return errors.New("revocation was refused")
			}
			return completionPrintUnknown(output, &op, "revocation is not confirmed; do not resend", errors.New("authorization remains issued"))
		}
	}
	if err := completionValidateAuthorization(grant, op.AuthorizationRequest); err != nil {
		return err
	}
	if grant.State != "revoked" {
		out := completionCLIEnvelope{ProtocolVersion: 1, Outcome: completionAuthOutcome(grant), Operation: completionOperationView(&op), Authorization: &grant, CurrentIssueRead: "not_requested", Reason: "authorization reached a terminal state without this revocation"}
		_ = printCompletionEnvelope(output, out)
		return errors.New("authorization was not revoked; inspect the retained terminal result")
	}
	op.Authorization = &grant
	if err := saveCompletionOperation(path, &op); err != nil {
		return err
	}
	return printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: completionAuthOutcome(grant), Operation: completionOperationView(&op), Authorization: &grant, CurrentIssueRead: "not_requested"})
}

func recoverCompletionCLI(cmd *cobra.Command, client *cli.APIClient, output, path string, op *completionCLIOperation, message string) error {
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	grant, err := completionAuthorizationByID(ctx, client, op.Authorization.AuthorizationID)
	if err != nil {
		return completionPrintUnknown(output, op, message, err)
	}
	if err := completionValidateAuthorization(grant, op.AuthorizationRequest); err != nil {
		return completionPrintUnknown(output, op, "authorization does not match original intent", err)
	}
	op.Authorization = &grant
	if err := saveCompletionOperation(path, op); err != nil {
		return err
	}
	issue, e := completionFetchIssue(ctx, client, op.Scope.IssueID)
	var current *completionCLIIssueResponse
	read := "ok"
	if e == nil {
		current = &issue
	} else {
		read = "denied"
	}
	outcome, why := "unknown", message
	if grant.State == "consumed" && completionReceiptMatches(grant.CompletionReceipt, op) && read == "ok" {
		outcome, why = "consumed", "the original completion receipt is confirmed"
		if current.Status != op.TargetStatus || current.Revision > grant.CompletionReceipt.CompletedRevision {
			why = fmt.Sprintf("historical completion was confirmed at revision %d; current issue is %s at revision %d", grant.CompletionReceipt.CompletedRevision, current.Status, current.Revision)
		}
	} else if grant.State == "revoked" {
		outcome, why = "revoked", "authorization was revoked; completion is not confirmed"
	}
	if err := printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: outcome, Operation: completionOperationView(op), Authorization: &grant, CompletionReceipt: grant.CompletionReceipt, CurrentIssue: current, CurrentIssueRead: read, Reason: why}); err != nil {
		return err
	}
	if outcome == "consumed" {
		return nil
	}
	return fmt.Errorf("completion outcome is %s; do not retry the write", outcome)
}

func showCompletionOperation(cmd *cobra.Command, client *cli.APIClient, output, path string, op *completionCLIOperation) error {
	ctx, cancel := cli.APIContext(cmd.Context())
	defer cancel()
	grant, err := completionAuthorizationByID(ctx, client, op.Authorization.AuthorizationID)
	if err != nil {
		return completionPrintUnknown(output, op, "authorization is unavailable under current authority", err)
	}
	if err := completionValidateAuthorization(grant, op.AuthorizationRequest); err != nil {
		return err
	}
	op.Authorization = &grant
	if err := saveCompletionOperation(path, op); err != nil {
		return err
	}
	issue, issueErr := completionFetchIssue(ctx, client, op.Scope.IssueID)
	var current *completionCLIIssueResponse
	read, reason := "ok", ""
	if issueErr != nil {
		read, reason = "denied", "authorization fact is retained; current issue state is unavailable"
	} else {
		current = &issue
	}
	outcome := completionAuthOutcome(grant)
	if op.CompletionDispatchStarted && grant.State == "issued" {
		outcome = "unknown"
		reason = "completion may be in flight; issued is not proof the original PUT did not commit"
	}
	if grant.State == "consumed" && completionReceiptMatches(grant.CompletionReceipt, op) && current != nil && (current.Status != grant.CompletionReceipt.TargetStatus || current.Revision > grant.CompletionReceipt.CompletedRevision) {
		reason = fmt.Sprintf("historical completion was confirmed at revision %d; current issue is %s at revision %d", grant.CompletionReceipt.CompletedRevision, current.Status, current.Revision)
	}
	if err := printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: outcome, Operation: completionOperationView(op), Authorization: &grant, CompletionReceipt: grant.CompletionReceipt, CurrentIssue: current, CurrentIssueRead: read, Reason: reason}); err != nil {
		return err
	}
	if read != "ok" {
		return errors.New("authorization was read, but current issue state is unavailable")
	}
	if outcome == "unknown" {
		return errors.New("completion may be in flight; do not submit again")
	}
	return nil
}

func completionOutput(v string) error {
	if v != "json" && v != "table" {
		return errors.New("--output must be json or table")
	}
	return nil
}
func completionCLIUUID(v string) bool {
	id, err := uuid.Parse(v)
	return err == nil && id != uuid.Nil && id.String() == v
}
func completionCLIHex(v string, sizes ...int) bool {
	ok := false
	for _, n := range sizes {
		if len(v) == n {
			ok = true
		}
	}
	if !ok || v == "" || v != strings.ToLower(v) {
		return false
	}
	for _, r := range v {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func completionStringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
func completionScopeMatches(c *cli.APIClient, user string, s completionCLIScope) error {
	if s.ServerURL != c.BaseURL || s.WorkspaceID != c.WorkspaceID || s.UserID != user {
		return errors.New("operation file belongs to another server, human, or workspace")
	}
	return nil
}
func completionOperationView(op *completionCLIOperation) map[string]any {
	if op == nil {
		return map[string]any{}
	}
	id := ""
	if op.AuthorizationRequest != nil {
		id = op.AuthorizationRequest.OperationID
	}
	return map[string]any{"issue_id": op.Scope.IssueID, "issue_revision": op.IssueRevision, "target_status": op.TargetStatus, "operation_id": id, "authorization_request_key": op.AuthorizationRequestKey, "completion_dispatch_started": op.CompletionDispatchStarted, "revocation_dispatch_started": op.RevocationDispatchStarted}
}
func completionAuthOutcome(g completionCLIAuthorization) string {
	switch g.State {
	case "issued":
		if g.Applicability == "expired" || !g.ExpiresAt.After(time.Now()) {
			return "refused"
		}
		return "authorized"
	case "consumed":
		return "consumed"
	case "revoked":
		return "revoked"
	default:
		return "unknown"
	}
}
func completionFetchIssue(ctx context.Context, c *cli.APIClient, id string) (completionCLIIssueResponse, error) {
	var issue completionCLIIssueResponse
	err := c.GetJSON(ctx, "/api/issues/"+url.PathEscape(id), &issue)
	if err == nil && (issue.ID != id || issue.Revision < 0 || issue.Status == "") {
		err = errors.New("current issue response is incomplete")
	}
	return issue, err
}
func completionAuthorizationByID(ctx context.Context, c *cli.APIClient, id string) (completionCLIAuthorization, error) {
	var g completionCLIAuthorization
	err := c.GetJSON(ctx, "/api/delivery-completion-authorizations/"+url.PathEscape(id), &g)
	return g, err
}
func completionByKey(ctx context.Context, c *cli.APIClient, key, workspace string) (completionCLIAuthorization, error) {
	var g completionCLIAuthorization
	q := url.Values{"workspace_id": {workspace}}
	err := c.GetJSON(ctx, "/api/delivery-completion-authorizations/by-request-key/"+url.PathEscape(key)+"?"+q.Encode(), &g)
	return g, err
}
func completionValidateAuthorization(g completionCLIAuthorization, r *completionCLIAuthorizationRequest) error {
	if !completionCLIUUID(g.AuthorizationID) || !completionCLIUUID(g.OperationID) || !completionCLIUUID(g.IssueID) || g.IssueRevision < 0 || g.TargetStatus == "" || g.ContractVersion != "delivery-completion-enforcement-v1" || g.CreatedAt.IsZero() || g.ExpiresAt.IsZero() || g.Applicability == "" {
		return errors.New("authorization response is incomplete")
	}
	switch g.State {
	case "issued":
		if (g.Applicability != "not_checked" && g.Applicability != "expired") || g.CompletionReceipt != nil {
			return errors.New("issued authorization state is inconsistent")
		}
	case "consumed":
		if g.Applicability != "consumed" || !completionReceiptMatchesAuthorization(g.CompletionReceipt, g) {
			return errors.New("consumed authorization receipt does not match its terminal state")
		}
	case "revoked":
		if g.Applicability != "revoked" || g.CompletionReceipt != nil {
			return errors.New("revoked authorization state is inconsistent")
		}
	default:
		return errors.New("authorization state is unknown")
	}
	if r != nil && (g.OperationID != r.OperationID || g.IssueID != r.Target.IssueID || g.IssueRevision != r.Target.ExpectedIssueRevision || g.TargetStatus != r.Target.TargetStatus || g.ContractVersion != r.ContractVersion || !completionTimeEqual(g.ExpiresAt, r.ExpiresAt)) {
		return errors.New("authorization does not match original request")
	}
	return nil
}
func completionTimeEqual(value time.Time, request string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, request)
	return err == nil && value.Equal(parsed)
}
func completionReceiptMatchesAuthorization(receipt *completionCLIReceipt, grant completionCLIAuthorization) bool {
	return receipt != nil && receipt.AuthorizationID == grant.AuthorizationID && receipt.OperationID == grant.OperationID &&
		receipt.IssueID == grant.IssueID && receipt.TargetStatus == grant.TargetStatus &&
		receipt.CompletedRevision > grant.IssueRevision && !receipt.CompletedAt.IsZero()
}
func completionReceiptMatches(r *completionCLIReceipt, op *completionCLIOperation) bool {
	return r != nil && op != nil && op.Authorization != nil && op.AuthorizationRequest != nil && r.AuthorizationID == op.Authorization.AuthorizationID && r.OperationID == op.AuthorizationRequest.OperationID && r.IssueID == op.Scope.IssueID && r.TargetStatus == op.TargetStatus && r.CompletedRevision > op.IssueRevision && !r.CompletedAt.IsZero()
}
func completionBindingValid(s completionCLIScope, p completionCLIPreview) bool {
	b, o := p.AuthorizationBinding, p.Observation
	if b == nil || o == nil || p.ProjectID == nil || p.ProjectRevision == nil || p.MaxExpiresAt == nil {
		return false
	}
	return b.ContractVersion == "delivery-completion-enforcement-v1" && b.SourceWorkspaceID == s.WorkspaceID && b.Target.Kind == "ordinary" && b.Target.IssueID == s.IssueID && b.Target.ExpectedIssueRevision == p.IssueRevision && b.Target.ExpectedStatus == p.CurrentStatus && b.Target.TargetStatus == p.TargetStatus && b.Target.TargetPosition == nil && p.CurrentCategory != "done" && p.TargetCategory == "done" && p.PolicyMode == "enforce" && p.PolicyRevision > 0 && completionCLIUUID(b.Project.ID) && b.Project.ID == *p.ProjectID && b.Project.ExpectedRevision == *p.ProjectRevision && b.Project.ExpectedPolicyRevision == p.PolicyRevision && b.Project.ExpectedRevision >= 0 && b.Project.ExpectedPolicyRevision > 0 && b.Evidence.IssueID == s.IssueID && b.Evidence.ExpectedIssueRevision == p.IssueRevision && b.Evidence.ApprovalID == o.ApprovalID && completionCLIUUID(b.Evidence.ApprovalID) && b.Evidence.ExpectedSourceRevision == o.SourceRevision && completionCLIHex(o.SourceRevision, 40, 64) && b.Evidence.ExpectedSpecDigest == o.SpecDigest && completionCLIHex(o.SpecDigest, 64) && b.Evidence.ExpectedArtifactDigest == o.ArtifactDigest && completionCLIHex(o.ArtifactDigest, 64) && completionCLIHex(o.ReceiptDigest, 64) && o.Verdict == "pass" && o.IssueID == s.IssueID && o.IssueRevision == p.IssueRevision && o.ProjectID != nil && *o.ProjectID == *p.ProjectID && o.ProjectRevision != nil && *o.ProjectRevision == *p.ProjectRevision && o.ValidUntil != nil && o.ValidUntil.After(p.CheckedAt) && o.ObservedAt.Before(*o.ValidUntil) && p.MaxExpiresAt.After(p.CheckedAt) && !p.MaxExpiresAt.After(*o.ValidUntil) && b.Consumer.Kind == "human" && b.Consumer.UserID == s.UserID
}
func completionValidateExpiry(raw string, p completionCLIPreview) error {
	v, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || !strings.HasSuffix(raw, "Z") || v.Location() != time.UTC || v.Nanosecond()%1000 != 0 || v.Format("2006-01-02T15:04:05.000000Z") != raw {
		return errors.New("--expires-at must be UTC RFC3339 with exactly six fractional digits and Z")
	}
	if p.MaxExpiresAt == nil || !v.After(p.CheckedAt) || v.After(*p.MaxExpiresAt) {
		return errors.New("--expires-at must be after preview and no later than max_expires_at")
	}
	return nil
}
func completionRequestDigest(r completionCLIAuthorizationRequest) (string, error) {
	b, e := json.Marshal(r)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

type completionCLIWriteError struct{ status int }

func (e *completionCLIWriteError) Error() string {
	return fmt.Sprintf("completion PUT returned HTTP %d", e.status)
}
func completionPutOnce(ctx context.Context, c *cli.APIClient, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Capabilities", "stable_attachment_urls")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.WorkspaceID != "" {
		req.Header.Set("X-Workspace-ID", c.WorkspaceID)
	}
	platform, version, osName := c.Platform, c.Version, c.OS
	if platform == "" {
		platform = cli.ClientPlatform
	}
	if version == "" {
		version = cli.ClientVersion
	}
	if osName == "" {
		osName = cli.ClientOS
	}
	if platform != "" {
		req.Header.Set("X-Client-Platform", platform)
	}
	if version != "" {
		req.Header.Set("X-Client-Version", version)
	}
	if osName != "" {
		req.Header.Set("X-Client-OS", osName)
	}
	req.GetBody = nil
	hc := *c.HTTPClient
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("completion outcome unknown: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return &completionCLIWriteError{resp.StatusCode}
	}
	var result completionCLIIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("completion response malformed; outcome unknown: %w", err)
	}
	return nil
}

func completionPrintUnknown(output string, op *completionCLIOperation, reason string, cause error) error {
	return completionPrintOutcome(output, "unknown", completionOperationView(op), op.Authorization, reason, cause)
}
func completionUnknown(output, reason string, cause error) error {
	return completionPrintOutcome(output, "unknown", map[string]any{}, nil, reason, cause)
}
func completionPrintOutcome(output, outcome string, operation map[string]any, auth *completionCLIAuthorization, reason string, cause error) error {
	_ = printCompletionEnvelope(output, completionCLIEnvelope{ProtocolVersion: 1, Outcome: outcome, Operation: operation, Authorization: auth, CurrentIssueRead: "unknown", Reason: reason})
	return fmt.Errorf("%s: %w", reason, cause)
}
func printCompletionEnvelope(output string, out completionCLIEnvelope) error {
	if output == "json" {
		return cli.PrintJSON(os.Stdout, out)
	}
	if output != "table" {
		return fmt.Errorf("unsupported output format %q", output)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "OUTCOME\t%s\n", out.Outcome)
	if out.Authorization != nil {
		fmt.Fprintf(w, "AUTHORIZATION\t%s (%s)\n", out.Authorization.AuthorizationID, out.Authorization.State)
	}
	if out.CompletionReceipt != nil {
		fmt.Fprintf(w, "COMPLETION\t%s at revision %d\n", out.CompletionReceipt.TargetStatus, out.CompletionReceipt.CompletedRevision)
	}
	if out.CurrentIssue != nil {
		fmt.Fprintf(w, "CURRENT ISSUE\t%s %s at revision %d\n", out.CurrentIssue.Identifier, out.CurrentIssue.Status, out.CurrentIssue.Revision)
	}
	if out.Reason != "" {
		fmt.Fprintf(w, "DETAIL\t%s\n", out.Reason)
	}
	return w.Flush()
}
func printCompletionCandidates(p completionCLICandidates) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "APPROVAL\tSTATUS\tRESULT\tCREATED\tEXPIRES\tSUMMARY")
	for _, i := range p.Items {
		result := ""
		if i.ResultCode != nil {
			result = *i.ResultCode
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", i.ApprovalID, i.Status, result, i.CreatedAt.UTC().Format(time.RFC3339), i.ExpiresAt.UTC().Format(time.RFC3339), i.Summary)
	}
	if p.NextCursor != nil {
		fmt.Fprintf(w, "NEXT CURSOR\t%s\n", *p.NextCursor)
	}
	fmt.Fprintf(w, "HAS MORE\t%t\n", p.HasMore)
	return w.Flush()
}

func createCompletionOperation(path string, op *completionCLIOperation) error {
	if path == "" {
		return errors.New("--operation-file is required")
	}
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	path = abs
	if e = completionPrivateDirectory(filepath.Dir(path)); e != nil {
		return e
	}
	if _, e = os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
		if e == nil {
			return errors.New("operation file already exists; it will not be overwritten")
		}
		return e
	}
	op.Version = 1
	if e = sealCompletionOperation(op); e != nil {
		return e
	}
	data, e := json.MarshalIndent(op, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if e != nil {
		return e
	}
	if _, e = f.Write(append(data, '\n')); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	_, e = readCompletionOperation(path)
	return e
}
func completionPrivateDirectory(path string) error {
	info, e := os.Stat(path)
	if e != nil {
		return fmt.Errorf("operation-file directory must already exist: %w", e)
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("operation-file directory must be private with no group/other access")
	}
	return nil
}
func lockCompletionOperation(path string) (func(), error) {
	abs, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	if e = completionPrivateDirectory(filepath.Dir(abs)); e != nil {
		return nil, e
	}
	name := abs + ".lock"
	f, e := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(e, os.ErrExist) {
		return nil, errors.New("operation file is locked by another command")
	}
	if e != nil {
		return nil, e
	}
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Sync()
	return func() { _ = f.Close(); _ = os.Remove(name) }, nil
}
func readCompletionOperation(path string) (completionCLIOperation, error) {
	var op completionCLIOperation
	abs, e := filepath.Abs(path)
	if e != nil {
		return op, e
	}
	if e = completionPrivateDirectory(filepath.Dir(abs)); e != nil {
		return op, e
	}
	info, e := os.Lstat(abs)
	if e != nil {
		return op, e
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return op, errors.New("operation file must be a private regular file, not a symlink")
	}
	f, e := os.Open(abs)
	if e != nil {
		return op, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(info, opened) {
		return op, errors.New("operation file changed while opening")
	}
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if e = d.Decode(&op); e != nil {
		return op, e
	}
	var extra any
	if e = d.Decode(&extra); !errors.Is(e, io.EOF) {
		return op, errors.New("operation file has trailing data")
	}
	if op.Version != 1 || !completionCLIUUID(op.Scope.WorkspaceID) || !completionCLIUUID(op.Scope.UserID) || !completionCLIUUID(op.Scope.IssueID) || op.Scope.ServerURL == "" || op.IssueRevision < 0 || op.TargetStatus == "" || !completionCLIUUID(op.Candidate.ApprovalID) {
		return op, errors.New("operation file is incomplete or unsupported")
	}
	if e = verifyCompletionOperation(&op); e != nil {
		return op, e
	}
	return op, nil
}
func sealCompletionOperation(op *completionCLIOperation) error {
	op.RecordDigest = ""
	data, e := json.Marshal(op)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(data)
	op.RecordDigest = hex.EncodeToString(sum[:])
	return nil
}
func verifyCompletionOperation(op *completionCLIOperation) error {
	want := op.RecordDigest
	op.RecordDigest = ""
	data, e := json.Marshal(op)
	op.RecordDigest = want
	if e != nil {
		return e
	}
	sum := sha256.Sum256(data)
	if want == "" || want != hex.EncodeToString(sum[:]) {
		return errors.New("operation file integrity check failed; no network write was sent")
	}
	if op.AuthorizationRequest != nil {
		digest, e := completionRequestDigest(*op.AuthorizationRequest)
		if e != nil {
			return e
		}
		if digest != op.AuthorizationRequestDigest || !completionCLIUUID(op.AuthorizationRequestKey) || !completionCLIUUID(op.AuthorizationRequest.OperationID) {
			return errors.New("saved authorization request digest does not match")
		}
	}
	if op.AuthorizationDispatchStarted && op.AuthorizationRequest == nil || op.CompletionDispatchStarted && (op.Authorization == nil || op.AuthorizationRequest == nil) || op.RevocationDispatchStarted && (op.Authorization == nil || !completionCLIUUID(op.RevocationRequestKey)) {
		return errors.New("operation dispatch marker lacks original identity")
	}
	return nil
}
func saveCompletionOperation(path string, op *completionCLIOperation) error {
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	if e = completionPrivateDirectory(filepath.Dir(abs)); e != nil {
		return e
	}
	info, e := os.Lstat(abs)
	if e != nil {
		return e
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("operation file must be regular")
	}
	if e = sealCompletionOperation(op); e != nil {
		return e
	}
	data, e := json.MarshalIndent(op, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(abs), ".completion-operation-*.tmp")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0o600); e != nil {
		_ = f.Close()
		return e
	}
	if _, e = f.Write(append(data, '\n')); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(name, abs); e != nil {
		return e
	}
	_, e = readCompletionOperation(abs)
	return e
}

func init() { issueCmd.AddCommand(issueCompletionCmd); registerIssueCompletionCommands() }
