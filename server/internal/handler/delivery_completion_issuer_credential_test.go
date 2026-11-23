package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func deliveryIssuerCredentialFixture(t *testing.T) service.ArtifactCredentialSnapshot {
	t.Helper()
	registered := passwordRegister(t, fmt.Sprintf("issuer%d", time.Now().UnixNano()))
	var credential service.ArtifactCredentialSnapshot
	passwordCall(t, http.MethodGet, "/api/me", nil, registered.Token, func(w http.ResponseWriter, r *http.Request) {
		principal, ok := auth.AuthenticatedPrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("actual JWT authentication supplied no principal")
		}
		credential = artifactCredentialSnapshot(r.Context(), principal)
		w.WriteHeader(http.StatusNoContent)
	}).Want(http.StatusNoContent)
	if credential.Principal.UserID != registered.User.ID || credential.Principal.Kind != auth.CredentialJWT || credential.PasswordSession == nil || credential.PasswordSession.UserID != registered.User.ID || credential.PasswordSession.Version < 1 {
		t.Fatal("actual JWT authentication did not preserve issuer identity and password version")
	}
	return credential
}

func checkDeliveryIssuerCredential(t *testing.T, ctx context.Context, credential service.ArtifactCredentialSnapshot) error {
	t.Helper()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	return service.CheckArtifactCredentialInTx(ctx, tx, credential)
}

// This is an explicit-credential helper regression. Both identities and the
// managed binding are real database fixtures, and the issuer JWT is authenticated
// through middleware. Full completion routes must independently authenticate the
// consumer; an issuer check is not a substitute for that separate check.
func TestDeliveryCompletionExplicitIssuerCredentialIsolation(t *testing.T) {
	consumer := managedDaemonSetup(t)
	identity, err := auth.CheckPasswordToken(t.Context(), consumer.h.Queries, consumer.token, true)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Session.UserID != consumer.owner || identity.Session.BindingID != consumer.binding {
		t.Fatal("actual managed token did not resolve its own binding")
	}
	ambient := auth.WithPasswordSession(t.Context(), identity.Session)
	issuer := deliveryIssuerCredentialFixture(t)
	if issuer.Principal.UserID == identity.Session.UserID {
		t.Fatal("issuer and consumer must be distinct")
	}
	if err := checkDeliveryIssuerCredential(t, t.Context(), issuer); err != nil {
		t.Fatalf("independent current issuer fixture is not valid: %v", err)
	}
	if _, err := auth.CheckPasswordVersion(ambient, consumer.h.Queries, identity.Session.UserID, identity.Session.Version); err != nil {
		t.Fatalf("independent current managed consumer fixture is not valid: %v", err)
	}

	t.Run("current_issuer_under_different_managed_consumer", func(t *testing.T) {
		if err := checkDeliveryIssuerCredential(t, ambient, issuer); err != nil {
			t.Fatalf("explicit current issuer was contaminated by unrelated managed consumer: %v", err)
		}
	})

	for _, test := range []struct {
		name   string
		change func(*testing.T, service.ArtifactCredentialSnapshot)
		want   error
	}{
		{"issuer_password_version_changed", func(t *testing.T, c service.ArtifactCredentialSnapshot) {
			dbfx.Exec(t, "UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1", c.Principal.UserID)
		}, auth.ErrPasswordSession},
		{"issuer_password_change_required", func(t *testing.T, c service.ArtifactCredentialSnapshot) {
			dbfx.Exec(t, "UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1", c.Principal.UserID)
		}, auth.ErrPasswordSession},
		{"issuer_account_disabled", func(t *testing.T, c service.ArtifactCredentialSnapshot) {
			dbfx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, c.Principal.UserID)
		}, auth.ErrPasswordSession},
		{"issuer_credential_deleted", func(t *testing.T, c service.ArtifactCredentialSnapshot) {
			dbfx.Exec(t, "DELETE FROM user_password_credential WHERE user_id=$1", c.Principal.UserID)
		}, pgx.ErrNoRows},
	} {
		t.Run(test.name, func(t *testing.T) {
			credential := deliveryIssuerCredentialFixture(t)
			if err := checkDeliveryIssuerCredential(t, t.Context(), credential); err != nil {
				t.Fatal(err)
			}
			test.change(t, credential)
			if err := checkDeliveryIssuerCredential(t, ambient, credential); !errors.Is(err, test.want) {
				t.Fatalf("explicit issuer lost its own current-credential refusal: got %v, want %v", err, test.want)
			}
		})
	}

	t.Run("legacy_context_cannot_authorize_another_user", func(t *testing.T) {
		_, err := auth.CheckPasswordVersion(ambient, consumer.h.Queries, issuer.Principal.UserID, issuer.PasswordSession.Version)
		if !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatalf("legacy managed context accepted a different subject: %v", err)
		}
	})

	t.Run("legacy_context_rechecks_its_own_managed_binding", func(t *testing.T) {
		tx, err := testPool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(t.Context())
		q := db.New(tx)
		if _, err := auth.CheckPasswordVersion(ambient, q, identity.Session.UserID, identity.Session.Version); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(t.Context(), "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1", consumer.binding); err != nil {
			t.Fatal(err)
		}
		if _, err := auth.CheckPasswordVersion(ambient, q, identity.Session.UserID, identity.Session.Version); !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatalf("legacy adapter ignored its own revoked managed binding: %v", err)
		}
	})
}
