package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type platformAdminInitializationReceipt struct {
	SchemaVersion    int    `json:"schema_version"`
	Status           string `json:"status"`
	OrganizationID   string `json:"organization_id,omitempty"`
	UserID           string `json:"user_id,omitempty"`
	InitializationID string `json:"initialization_id,omitempty"`
	CredentialsFile  string `json:"credentials_file,omitempty"`
}

func runPlatformAdminInitialize(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("platform-admin initialize", flag.ContinueOnError)
	flags.SetOutput(io.Discard) // Flag errors must never echo arbitrary arguments.
	path := flags.String("credentials-file", "", "Absolute private credential file path")
	reason := flags.String("reason", "", "Reason for administrator initialization")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid_initialization_arguments")
	}
	*reason = strings.TrimSpace(*reason)
	if flags.NArg() != 0 || !filepath.IsAbs(*path) || filepath.Clean(*path) != *path || *reason == "" || len(*reason) > 1000 || !utf8.ValidString(*reason) || strings.ContainsRune(*reason, 0) {
		return errors.New("invalid_initialization_arguments")
	}
	receipt := platformAdminInitializationReceipt{SchemaVersion: 1}
	if !auth.PasswordMode() {
		receipt.Status = "skipped_mode_disabled"
		return json.NewEncoder(output).Encode(receipt)
	}
	if os.Getenv("MULTICA_PLATFORM_ADMIN_ENABLED") != "true" {
		receipt.Status = "skipped_admin_disabled"
		return json.NewEncoder(output).Encode(receipt)
	}
	deployment, err := util.ParseUUID(auth.ManagedDeploymentID())
	if err != nil {
		return errors.New("invalid_deployment_id")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("database_unavailable")
	}
	defer pool.Close()
	if err = preparePlatformOrganization(ctx, pool); err != nil {
		return errors.New("platform_schema_unavailable")
	}
	if err = validatePlatformInitializationSchema(ctx, pool); err != nil {
		return errors.New("initialization_schema_unavailable")
	}
	q := db.New(pool)
	org, err := q.GetInternalOrganization(ctx)
	if err != nil {
		return errors.New("organization_unavailable")
	}
	runner := platformAdminInitializer{queries: q, transactions: pool, organization: org.ID, deployment: deployment, path: *path, reason: *reason, openFile: openPlatformAdminCredentialFile}
	receipt, err = runner.run(ctx)
	if err != nil {
		return safePlatformInitializationError(err)
	}
	if err = json.NewEncoder(output).Encode(receipt); err != nil {
		return errors.New("initialization_receipt_unavailable")
	}
	return nil
}

func validatePlatformInitializationSchema(ctx context.Context, pool *pgxpool.Pool) error {
	for _, index := range [][2]string{
		{"platform_admin_initialization_org_uidx", "platform_admin_initialization"},
		{"platform_admin_initialization_singleton_uidx", "platform_admin_initialization"},
		{"user_password_credential_user_uidx", "user_password_credential"},
		{"user_password_credential_username_uidx", "user_password_credential"},
	} {
		var valid bool
		if err := pool.QueryRow(ctx, `SELECT COALESCE((SELECT indisvalid AND indisready AND indisunique AND indrelid=to_regclass($2) FROM pg_index WHERE indexrelid=to_regclass($1)),false)`, index[0], index[1]).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return errors.New("initialization_schema_unavailable")
		}
	}
	return nil
}

type platformAdminInitializer struct {
	queries      *db.Queries
	transactions service.TxStarter
	organization pgtype.UUID
	deployment   pgtype.UUID
	path         string
	reason       string
	openFile     func(string) (*platformAdminCredentialFile, error)
}

type initializationPreparation struct {
	file               *platformAdminCredentialFile
	credentials        platformAdminCredentials
	fileExists         bool
	fileError          error
	passwordHash       string
	verifiedCredential db.UserPasswordCredential
	verified           bool
}

func (r platformAdminInitializer) prepare(ctx context.Context) (initializationPreparation, error) {
	var p initializationPreparation
	marker, err := r.queries.GetPlatformAdminInitialization(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, err
	}
	markerExists := err == nil
	p.file, p.fileError = r.openFile(r.path)
	if p.fileError == nil {
		p.credentials, p.fileExists, p.fileError = p.file.read()
	}
	if p.fileError != nil {
		return p, nil
	}
	if !markerExists {
		if !p.fileExists {
			p.credentials, err = newPlatformAdminCredentials(util.UUIDToString(r.organization), util.UUIDToString(r.deployment))
			if err != nil {
				return p, err
			}
		}
		if !r.matchesDeployment(p.credentials) {
			p.fileError = errors.New("credentials_file_conflict")
			return p, nil
		}
		p.passwordHash, err = auth.HashPassword(ctx, p.credentials.InitialPassword)
		return p, err
	}
	if marker.State == "created" && p.fileExists && r.matchesMarker(p.credentials, marker) {
		credential, err := r.queries.GetPasswordCredential(ctx, marker.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return p, nil
		}
		if err != nil {
			return p, err
		}
		if credential.MustChangePassword && credential.SessionVersion == marker.InitialCredentialVersion.Int64 {
			p.verified, err = auth.VerifyPassword(ctx, credential.PasswordHash, p.credentials.InitialPassword)
			if err != nil {
				return p, err
			}
			p.verifiedCredential = credential
		}
	}
	return p, nil
}

func (r platformAdminInitializer) matchesDeployment(c platformAdminCredentials) bool {
	return c.OrganizationID == util.UUIDToString(r.organization) && c.DeploymentID == util.UUIDToString(r.deployment)
}
func (r platformAdminInitializer) matchesMarker(c platformAdminCredentials, marker db.PlatformAdminInitialization) bool {
	return r.matchesDeployment(c) && c.InitializationID == util.UUIDToString(marker.InitializationID)
}

var errInitializationSnapshotChanged = errors.New("initialization_snapshot_changed")

func (r platformAdminInitializer) run(ctx context.Context) (platformAdminInitializationReceipt, error) {
	var empty platformAdminInitializationReceipt
	reconcile := false
	for attempt := 0; attempt < 4; attempt++ {
		p, err := r.prepare(ctx)
		if err != nil {
			if p.file != nil {
				p.file.close()
			}
			if reconcile {
				return empty, errors.New("initialization_unconfirmed")
			}
			return empty, err
		}
		receipt, committed, err := r.attempt(ctx, p, reconcile)
		if p.file != nil {
			p.file.close()
		}
		if committed {
			reconcile = true
			continue
		}
		if errors.Is(err, errInitializationSnapshotChanged) {
			continue
		}
		if reconcile && err != nil {
			var unavailable credentialsUnavailableError
			if errors.As(err, &unavailable) {
				return empty, unavailable
			}
			return empty, errors.New("initialization_unconfirmed")
		}
		return receipt, err
	}
	return empty, errors.New("initialization_unconfirmed")
}

// attempt owns commit/rollback. A commit error is never a creation retry: the
// next attempt obtains the same advisory lock in a fresh transaction and may
// only reconcile a terminal marker. A durable candidate file is always retained.
func (r platformAdminInitializer) attempt(ctx context.Context, p initializationPreparation, reconcile bool) (platformAdminInitializationReceipt, bool, error) {
	var receipt platformAdminInitializationReceipt
	tx, err := r.transactions.Begin(ctx)
	if err != nil {
		return receipt, false, err
	}
	defer tx.Rollback(ctx)
	state, err := service.InspectPlatformAdminInitializationInTx(ctx, tx, r.organization, r.deployment)
	if err != nil {
		return receipt, false, err
	}
	receipt = platformAdminInitializationReceipt{SchemaVersion: 1, Status: state.Status, OrganizationID: util.UUIDToString(r.organization)}
	marker := state.Marker
	if reconcile && !marker.InitializationID.Valid {
		return receipt, false, errors.New("initialization_not_committed")
	}
	switch state.Status {
	case "preserved_existing_admin":
		if !marker.InitializationID.Valid {
			marker, err = service.PreservePlatformAdminInitializationInTx(ctx, tx, r.organization)
			if err != nil {
				return receipt, false, err
			}
			if err = tx.Commit(ctx); err != nil {
				return receipt, true, nil
			}
		}
	case "already_initialized":
		if state.Credential.UserID.Valid {
			// The verified hash/version snapshot must still be current under the user
			// lock. A concurrent first creator can require one fresh preparation.
			if p.file == nil {
				return receipt, false, credentialsUnavailable(marker)
			}
			current, exists, readErr := p.file.read()
			if readErr != nil || !exists || !r.matchesMarker(current, marker) {
				return receipt, false, credentialsUnavailable(marker)
			}
			if p.fileError != nil || !p.fileExists || current != p.credentials {
				return receipt, false, errInitializationSnapshotChanged
			}
			if p.verifiedCredential.UserID != state.Credential.UserID || p.verifiedCredential.PasswordHash != state.Credential.PasswordHash || p.verifiedCredential.SessionVersion != state.Credential.SessionVersion {
				return receipt, false, errInitializationSnapshotChanged
			}
			if !p.verified {
				return receipt, false, credentialsUnavailable(marker)
			}
			if err = p.file.durable(p.credentials); err != nil {
				return receipt, false, credentialsUnavailable(marker)
			}
			receipt.CredentialsFile = r.path
		}
	case "eligible":
		if p.fileError != nil {
			return receipt, false, p.fileError
		}
		if p.passwordHash == "" {
			return receipt, false, errInitializationSnapshotChanged
		}
		if err = p.file.durable(p.credentials); err != nil {
			return receipt, false, err
		}
		intent, err := util.ParseUUID(p.credentials.InitializationID)
		if err != nil {
			return receipt, false, errors.New("credentials_file_invalid")
		}
		marker, err = service.CreatePlatformAdminInitializationInTx(ctx, tx, service.PlatformAdminInitializationParams{
			OrganizationID: r.organization, DeploymentID: r.deployment, InitializationID: intent, PasswordHash: p.passwordHash, Reason: r.reason, RequestID: uuid.NewString(),
		})
		if err != nil {
			return receipt, false, err
		}
		if err = tx.Commit(ctx); err != nil {
			return receipt, true, nil
		}
		receipt.Status, receipt.CredentialsFile = "initialized", r.path
	default:
		return receipt, false, errors.New("initialization_unconfirmed")
	}
	if marker.UserID.Valid {
		receipt.UserID = util.UUIDToString(marker.UserID)
	}
	receipt.InitializationID = util.UUIDToString(marker.InitializationID)
	return receipt, false, nil
}

type credentialsUnavailableError struct{ user, intent string }

func (e credentialsUnavailableError) Error() string {
	return "credentials_unavailable user_id=" + e.user + " initialization_id=" + e.intent
}
func credentialsUnavailable(marker db.PlatformAdminInitialization) error {
	return credentialsUnavailableError{util.UUIDToString(marker.UserID), util.UUIDToString(marker.InitializationID)}
}

// Database/OS errors can include connection strings, SQL values or file data.
// Only explicitly controlled error codes and public identity leave the CLI.
func safePlatformInitializationError(err error) error {
	var unavailable credentialsUnavailableError
	if errors.As(err, &unavailable) {
		return unavailable
	}
	var adminErr *service.PlatformAdminError
	if errors.As(err, &adminErr) {
		return errors.New(adminErr.Code)
	}
	switch err.Error() {
	case "credentials_directory_unsafe", "credentials_file_unsafe", "credentials_file_invalid", "credentials_file_conflict", "credentials_file_create_failed", "credentials_file_write_failed", "credentials_file_publish_failed", "credentials_file_sync_failed", "credentials_directory_changed", "credentials_directory_sync_failed", "randomness_unavailable", "initialization_unconfirmed", "initialization_not_committed", "invalid_credentials_path":
		return errors.New(err.Error())
	default:
		return errors.New("database_unavailable")
	}
}
