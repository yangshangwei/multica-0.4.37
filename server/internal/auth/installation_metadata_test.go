package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestInstallationMetadataCannotMintHumanJWT(t *testing.T) {
	t.Setenv("JWT_SECRET", "installation-metadata-test-secret")
	deployment, _ := util.ParseUUID("00000000-0000-4000-8000-000000000001")
	id, _ := util.ParseUUID("00000000-0000-4000-8000-000000000002")
	s := PasswordSession{Kind: "jwt", UserID: "00000000-0000-4000-8000-000000000003", Version: 1}
	proof, err := MintInstallationMetadataProof(s, db.ManagedInstallation{ID: id, DeploymentID: deployment, KeyVersion: 1}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(proof, "mip_") {
		t.Fatal("missing scoped prefix")
	}
	if _, err = jwt.Parse(strings.TrimPrefix(proof, "mip_"), func(*jwt.Token) (any, error) { return JWTSecret(), nil }, jwt.WithValidMethods([]string{"HS256"})); err == nil {
		t.Fatal("metadata proof became a human bearer credential")
	}
	s.Kind = "pat"
	if _, err = MintInstallationMetadataProof(s, db.ManagedInstallation{ID: id, DeploymentID: deployment, KeyVersion: 1}, time.Now()); err == nil {
		t.Fatal("PAT minted a proof without the human installation signature flow")
	}
}
