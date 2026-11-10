package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// DeliveryWebhookProof is constructed only by an authenticated provider adapter.
// It cannot be decoded from normal issue JSON or serialized into a response.
// VerifySignature may only authenticate the original immutable body/headers with
// the currently locked encrypted secret (VCS) or current process config (GitHub).
// It must not decide issue, project, artifact or completion policy.
type DeliveryWebhookProof struct {
	Kind                 string                                                                         `json:"-"`
	WorkspaceID          pgtype.UUID                                                                    `json:"-"`
	IntegrationID        pgtype.UUID                                                                    `json:"-"`
	TrustRevision        int64                                                                          `json:"-"`
	PullRequestID        pgtype.UUID                                                                    `json:"-"`
	RepoOwner            string                                                                         `json:"-"`
	RepoName             string                                                                         `json:"-"`
	PRNumber             int32                                                                          `json:"-"`
	HeadSHA              string                                                                         `json:"-"`
	State                string                                                                         `json:"-"`
	Action               string                                                                         `json:"-"`
	Provider             string                                                                         `json:"-"`
	InstanceURL          string                                                                         `json:"-"`
	GitHubInstallationID int64                                                                          `json:"-"`
	ConfigIdentity       string                                                                         `json:"-"`
	EventID              string                                                                         `json:"-"`
	EventDigest          string                                                                         `json:"-"`
	VerifySignature      func(currentEncryptedSecret string) (currentConfigIdentity string, valid bool) `json:"-"`
}

// DeliveryWebhookConfigIdentity is internal credential metadata, never an API
// capability or a plaintext secret. Blank configuration does not grant trust.
func DeliveryWebhookConfigIdentity(secret string) string {
	if secret == "" {
		return ""
	}
	sum := sha256.Sum256(append([]byte("multica-delivery-github-webhook-config-v1\x00"), []byte(secret)...))
	return hex.EncodeToString(sum[:])
}

type deliveryWebhookTrust struct {
	id, workspace pgtype.UUID
	revision      int64
	installation  int64
	provider      string
	instance      string
	secret        string
}

func lockDeliveryWebhookTrust(ctx context.Context, tx pgx.Tx, kind string, id, workspace pgtype.UUID) (deliveryWebhookTrust, error) {
	var out deliveryWebhookTrust
	if tx == nil || !id.Valid || !workspace.Valid {
		return out, completionInvalid()
	}
	var err error
	switch kind {
	case "github":
		err = tx.QueryRow(ctx, `SELECT id,workspace_id,trust_revision,installation_id
		 FROM github_installation WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, id, workspace).
			Scan(&out.id, &out.workspace, &out.revision, &out.installation)
	case "vcs":
		err = tx.QueryRow(ctx, `SELECT id,workspace_id,trust_revision,provider,instance_url,webhook_secret_encrypted
		 FROM vcs_connection WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, id, workspace).
			Scan(&out.id, &out.workspace, &out.revision, &out.provider, &out.instance, &out.secret)
	default:
		return out, completionUnsupported()
	}
	if err != nil {
		return out, deliveryCompletionError(err)
	}
	if out.revision < 1 || kind == "github" && out.installation <= 0 || kind == "vcs" && (out.secret == "" || out.instance == "" || !deliveryWebhookProvider(out.provider)) {
		return out, completionDenied()
	}
	return out, nil
}

type deliveryWebhookPR struct {
	id, integration pgtype.UUID
	installation    int64
	owner, repo     string
	number          int32
	head, state     string
	provider        string
}

func lockDeliveryWebhookPR(ctx context.Context, tx pgx.Tx, kind string, id, issue, workspace pgtype.UUID) (deliveryWebhookPR, error) {
	var out deliveryWebhookPR
	var err error
	switch kind {
	case "github":
		err = tx.QueryRow(ctx, `SELECT pr.id,pr.installation_id,pr.repo_owner,pr.repo_name,pr.pr_number,pr.head_sha,pr.state
		 FROM github_pull_request pr JOIN issue_pull_request link ON link.pull_request_id=pr.id
		 WHERE pr.id=$1 AND pr.workspace_id=$2 AND link.issue_id=$3 AND NOT link.reference_only
		 FOR SHARE OF pr,link NOWAIT`, id, workspace, issue).
			Scan(&out.id, &out.installation, &out.owner, &out.repo, &out.number, &out.head, &out.state)
	case "vcs":
		err = tx.QueryRow(ctx, `SELECT pr.id,pr.connection_id,pr.repo_owner,pr.repo_name,pr.pr_number,pr.head_sha,pr.state,pr.provider
		 FROM vcs_pull_request pr JOIN issue_vcs_pull_request link ON link.pull_request_id=pr.id
		 WHERE pr.id=$1 AND pr.workspace_id=$2 AND link.issue_id=$3 AND NOT link.reference_only
		 FOR SHARE OF pr,link NOWAIT`, id, workspace, issue).
			Scan(&out.id, &out.integration, &out.owner, &out.repo, &out.number, &out.head, &out.state, &out.provider)
	default:
		return out, completionUnsupported()
	}
	if err != nil {
		return out, deliveryCompletionError(err)
	}
	if out.owner == "" || out.repo == "" || out.number < 1 || !DeliveryHex(out.head, 40, 64) {
		return out, completionConflict()
	}
	return out, nil
}

// FreezeDeliveryWebhookConsumerInTx freezes one live integration and linked PR.
// githubConfigIdentity must come from the server's current webhook configuration,
// not from an authorization request field or an arbitrary user-supplied hash.
func FreezeDeliveryWebhookConsumerInTx(ctx context.Context, tx pgx.Tx, req DeliveryCompletionAuthorizationRequest, issue db.Issue, params *db.CreateDeliveryCompletionAuthorizationParams, githubConfigIdentity string) error {
	if params == nil || req.Target.Kind != "ordinary" || (req.Consumer.Kind != "github" && req.Consumer.Kind != "vcs") {
		return completionInvalid()
	}
	key := req.Consumer.InstallationID
	if req.Consumer.Kind == "vcs" {
		key = req.Consumer.ConnectionID
	}
	id, err := util.ParseUUID(key)
	if err != nil {
		return completionInvalid()
	}
	trust, err := lockDeliveryWebhookTrust(ctx, tx, req.Consumer.Kind, id, issue.WorkspaceID)
	if err != nil {
		return err
	}
	if trust.revision != req.Consumer.ExpectedTrustRevision {
		return completionConflict()
	}
	pr, err := lockDeliveryWebhookPR(ctx, tx, req.Consumer.Kind, completionUUIDPG(req.Consumer.PullRequestID), issue.ID, issue.WorkspaceID)
	if err != nil {
		return err
	}
	if pr.head != req.Evidence.ExpectedSourceRevision || (req.Consumer.Kind == "github" && (trust.installation != pr.installation || !DeliveryHex(githubConfigIdentity, 64))) || (req.Consumer.Kind == "vcs" && (trust.id != pr.integration || trust.provider != pr.provider)) {
		return completionConflict()
	}
	params.ConsumerKind = req.Consumer.Kind
	params.IntegrationID = trust.id
	params.IntegrationTrustRevision = completionInt(trust.revision)
	params.PullRequestID = pr.id
	params.RepoOwner, params.RepoName = completionText(pr.owner), completionText(pr.repo)
	params.PrNumber = pgtype.Int4{Int32: pr.number, Valid: true}
	params.PrHeadSha = completionText(pr.head)
	if req.Consumer.Kind == "github" {
		params.GithubInstallationID = completionInt(trust.installation)
		params.WebhookConfigIdentity = completionText(githubConfigIdentity)
	} else {
		params.VcsProvider, params.VcsInstanceUrl = completionText(trust.provider), completionText(trust.instance)
	}
	return nil
}

func deliveryWebhookProvider(provider string) bool {
	return provider == "forgejo" || provider == "gitea" || provider == "gitlab"
}

func deliveryWebhookProofValid(p *DeliveryWebhookProof) bool {
	if p == nil || p.VerifySignature == nil || !p.WorkspaceID.Valid || !p.IntegrationID.Valid || !p.PullRequestID.Valid || p.TrustRevision < 1 || p.PRNumber < 1 || p.RepoOwner == "" || p.RepoName == "" || !DeliveryHex(p.HeadSHA, 40, 64) || !DeliveryHex(p.EventDigest, 64) || p.EventID == "" || len(p.EventID) > 512 || !utf8.ValidString(p.EventID) || strings.ContainsRune(p.EventID, 0) || (p.State != "merged" && p.State != "closed") {
		return false
	}
	if p.Kind == "github" {
		return p.Action == "closed" && p.GitHubInstallationID > 0 && DeliveryHex(p.ConfigIdentity, 64) && p.Provider == "" && p.InstanceURL == ""
	}
	if p.Kind != "vcs" || !deliveryWebhookProvider(p.Provider) || p.InstanceURL == "" || p.GitHubInstallationID != 0 || p.ConfigIdentity != "" {
		return false
	}
	return p.Action == "closed" || p.Action == "close" || p.Action == "merge" || p.Action == "merged"
}

func checkDeliveryWebhookProofInTx(ctx context.Context, tx pgx.Tx, issue db.Issue, proof *DeliveryWebhookProof) error {
	if !deliveryWebhookProofValid(proof) || proof.WorkspaceID != issue.WorkspaceID {
		return completionDenied()
	}
	trust, err := lockDeliveryWebhookTrust(ctx, tx, proof.Kind, proof.IntegrationID, issue.WorkspaceID)
	if err != nil {
		return err
	}
	if trust.revision != proof.TrustRevision || proof.Kind == "github" && trust.installation != proof.GitHubInstallationID || proof.Kind == "vcs" && (trust.provider != proof.Provider || trust.instance != proof.InstanceURL) {
		return completionDenied()
	}
	config, valid := proof.VerifySignature(trust.secret)
	if !valid || proof.Kind == "github" && config != proof.ConfigIdentity || proof.Kind == "vcs" && config != "" {
		return completionDenied()
	}
	pr, err := lockDeliveryWebhookPR(ctx, tx, proof.Kind, proof.PullRequestID, issue.ID, issue.WorkspaceID)
	if err != nil {
		return err
	}
	if pr.owner != proof.RepoOwner || pr.repo != proof.RepoName || pr.number != proof.PRNumber || pr.head != proof.HeadSHA || pr.state != proof.State || proof.Kind == "github" && pr.installation != proof.GitHubInstallationID || proof.Kind == "vcs" && (pr.integration != proof.IntegrationID || pr.provider != proof.Provider) {
		return completionConflict()
	}
	return nil
}

// CheckDeliveryWebhookConsumerInTx checks only the actual integration consumer.
// The common Prepare/Finish path still owns current policy, issuer and the one
// artifact observer. A valid signature cannot substitute for those checks.
func CheckDeliveryWebhookConsumerInTx(ctx context.Context, tx pgx.Tx, row db.DeliveryCompletionAuthorization, issue db.Issue, proof *DeliveryWebhookProof) error {
	if proof == nil || row.TargetKind != "ordinary" || row.ConsumerKind != proof.Kind || row.WorkspaceID != proof.WorkspaceID || row.IssueID != issue.ID || row.IntegrationID != proof.IntegrationID || !row.IntegrationTrustRevision.Valid || row.IntegrationTrustRevision.Int64 != proof.TrustRevision || row.PullRequestID != proof.PullRequestID || !row.PrNumber.Valid || row.PrNumber.Int32 != proof.PRNumber || row.RepoOwner.String != proof.RepoOwner || row.RepoName.String != proof.RepoName || row.PrHeadSha.String != proof.HeadSHA || row.SourceRevision != proof.HeadSHA {
		return completionDenied()
	}
	if proof.Kind == "github" && (!row.GithubInstallationID.Valid || row.GithubInstallationID.Int64 != proof.GitHubInstallationID || row.WebhookConfigIdentity.String != proof.ConfigIdentity) {
		return completionDenied()
	}
	if proof.Kind == "vcs" && (row.VcsProvider.String != proof.Provider || row.VcsInstanceUrl.String != proof.InstanceURL) {
		return completionDenied()
	}
	return checkDeliveryWebhookProofInTx(ctx, tx, issue, proof)
}

// MatchDeliveryWebhookAuthorizationInTx chooses exactly one still-unused grant
// for this actual event and current issue. It never selects the latest arbitrary
// authorization. The caller subsequently uses the common Prepare/Finish path.
func MatchDeliveryWebhookAuthorizationInTx(ctx context.Context, tx pgx.Tx, issue db.Issue, target string, proof *DeliveryWebhookProof) (binding *DeliveryCompletionBinding, err error) {
	defer func() { err = deliveryCompletionError(err) }()
	_, policy, err := LockDeliveryProjectPolicy(ctx, tx, issue.WorkspaceID, issue.ProjectID)
	if err != nil {
		return nil, err
	}
	if policy.Mode != "enforce" {
		return nil, nil
	}
	if err = PrepareArtifactTransaction(ctx, tx); err != nil {
		return nil, err
	}
	if err = CheckDeliveryCompletionSchema(ctx, tx); err != nil {
		return nil, err
	}
	if err = checkDeliveryWebhookProofInTx(ctx, tx, issue, proof); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT a.id,a.operation_id,a.issue_revision
	 FROM delivery_completion_authorization a
	 LEFT JOIN delivery_completion_terminal t ON t.authorization_id=a.id
	 WHERE a.workspace_id=$1 AND a.issue_id=$2 AND a.issue_revision=$3 AND a.policy_revision=$4
	 AND a.consumer_kind=$5 AND a.integration_id=$6 AND a.pull_request_id=$7
	 AND a.integration_trust_revision=$8 AND a.source_revision=$9 AND a.target_status=$10
	 AND a.contract_version=$11 AND a.target_kind='ordinary' AND t.authorization_id IS NULL
	 AND clock_timestamp()<a.expires_at AND clock_timestamp()<a.evidence_expires_at
	 ORDER BY a.id LIMIT 2 FOR UPDATE OF a NOWAIT`, issue.WorkspaceID, issue.ID, issue.Revision, policy.Revision, proof.Kind, proof.IntegrationID, proof.PullRequestID, proof.TrustRevision, proof.HeadSHA, target, DeliveryCompletionContract)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matches []DeliveryCompletionBinding
	for rows.Next() {
		var id, operation pgtype.UUID
		var revision int64
		if err = rows.Scan(&id, &operation, &revision); err != nil {
			return nil, err
		}
		matches = append(matches, DeliveryCompletionBinding{AuthorizationID: util.UUIDToString(id), OperationID: util.UUIDToString(operation), ExpectedIssueRevision: revision})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, DeliveryCompletionError{428, "delivery_completion_authorization_required"}
	}
	if len(matches) != 1 {
		return nil, DeliveryCompletionError{409, "delivery_completion_authorization_ambiguous"}
	}
	return &matches[0], nil
}
