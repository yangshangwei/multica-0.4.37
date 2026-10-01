// Package installation defines the signed, deployment-scoped management proof.
package installation

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/google/uuid"
)

var ErrProof = errors.New("invalid installation proof")

// Payload order is the wire contract. Clients sign the original issued bytes;
// numeric versions and timestamps are decimal strings to avoid JS precision loss.
type Payload struct {
	ProtocolVersion      string  `json:"protocol_version"`
	Purpose              string  `json:"purpose"`
	ChallengeID          string  `json:"challenge_id"`
	DeploymentID         string  `json:"deployment_id"`
	OrganizationID       string  `json:"organization_id"`
	UserID               string  `json:"user_id"`
	AuthVersion          string  `json:"auth_version"`
	InstallationID       *string `json:"installation_id"`
	WorkspaceID          *string `json:"workspace_id"`
	DaemonID             *string `json:"daemon_id"`
	PublicKeyFingerprint string  `json:"public_key_fingerprint"`
	ExpectedBindingEpoch *string `json:"expected_binding_epoch"`
	Nonce                string  `json:"nonce"`
	ExpiresAt            string  `json:"expires_at"`
	Method               string  `json:"method"`
	Path                 string  `json:"path"`
	BodySHA256           string  `json:"body_sha256"`
}

type Challenge struct {
	ChallengeID      string `json:"challenge_id"`
	Nonce            string `json:"nonce"`
	DeploymentID     string `json:"deployment_id"`
	ExpiresAt        string `json:"expires_at"`
	SignaturePayload string `json:"signature_payload"`
	BodyPayload      string `json:"body_payload"`
}

type Proof struct {
	ChallengeID      string `json:"challenge_id"`
	SignaturePayload string `json:"signature_payload"`
	BodyPayload      string `json:"body_payload"`
	Signature        string `json:"signature"`
}

type Hashes struct{ Payload, Body, Nonce string }

func Digest(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func Decode(raw string, limit int) ([]byte, error) {
	if raw == "" || len(raw) > limit*2 {
		return nil, ErrProof
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || len(b) > limit || base64.RawURLEncoding.EncodeToString(b) != raw {
		return nil, ErrProof
	}
	return b, nil
}

func positiveDecimal(raw string) bool {
	n, err := strconv.ParseInt(raw, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == raw
}

func canonicalID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id != uuid.Nil && id.String() == raw
}

func validate(p Payload) error {
	if p.ProtocolVersion != "1" || p.Method != "POST" || !positiveDecimal(p.AuthVersion) || !positiveDecimal(p.ExpiresAt) {
		return ErrProof
	}
	for _, id := range []string{p.ChallengeID, p.DeploymentID, p.OrganizationID, p.UserID} {
		if !canonicalID(id) {
			return ErrProof
		}
	}
	nonce, err := Decode(p.Nonce, 32)
	if err != nil || len(nonce) != 32 {
		return ErrProof
	}
	for _, hash := range []string{p.PublicKeyFingerprint, p.BodySHA256} {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != hash {
			return ErrProof
		}
	}
	switch p.Purpose {
	case "enroll":
		if p.Path != "/api/installations/enroll" || p.InstallationID != nil || p.WorkspaceID != nil || p.DaemonID != nil || p.ExpectedBindingEpoch != nil {
			return ErrProof
		}
	case "bind", "renew":
		path := "/api/daemon/installation-bindings"
		if p.Purpose == "renew" {
			path += "/renew"
		}
		if p.Path != path {
			return ErrProof
		}
		for _, id := range []*string{p.InstallationID, p.WorkspaceID, p.DaemonID} {
			if id == nil || !canonicalID(*id) {
				return ErrProof
			}
		}
		if p.ExpectedBindingEpoch != nil && !positiveDecimal(*p.ExpectedBindingEpoch) || p.Purpose == "renew" && p.ExpectedBindingEpoch == nil {
			return ErrProof
		}
	default:
		return ErrProof
	}
	return nil
}

func Encode(p Payload, body []byte) (Challenge, Hashes, error) {
	p.BodySHA256 = Digest(body)
	if len(body) == 0 || len(body) > 4096 || !json.Valid(body) || validate(p) != nil {
		return Challenge{}, Hashes{}, ErrProof
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return Challenge{}, Hashes{}, err
	}
	nonce, _ := Decode(p.Nonce, 32)
	return Challenge{ChallengeID: p.ChallengeID, Nonce: p.Nonce, DeploymentID: p.DeploymentID, ExpiresAt: p.ExpiresAt, SignaturePayload: base64.RawURLEncoding.EncodeToString(raw), BodyPayload: base64.RawURLEncoding.EncodeToString(body)}, Hashes{Payload: Digest(raw), Body: p.BodySHA256, Nonce: Digest(nonce)}, nil
}

// Verify checks stored hashes before interpreting any client-supplied fields.
// The service separately checks current authentication, scope, TTL and atomic
// consumption under its transaction; a cryptographic proof is not authorization.
func Verify(proof Proof, publicKey []byte, hashes Hashes) (Payload, []byte, error) {
	var p Payload
	raw, err := Decode(proof.SignaturePayload, 4096)
	if err != nil || Digest(raw) != hashes.Payload {
		return p, nil, ErrProof
	}
	body, err := Decode(proof.BodyPayload, 4096)
	if err != nil || Digest(body) != hashes.Body {
		return p, nil, ErrProof
	}
	signature, err := Decode(proof.Signature, ed25519.SignatureSize)
	if err != nil || len(signature) != ed25519.SignatureSize || len(publicKey) != ed25519.PublicKeySize || !ed25519.Verify(publicKey, raw, signature) {
		return p, nil, ErrProof
	}
	if json.Unmarshal(raw, &p) != nil || validate(p) != nil || p.ChallengeID != proof.ChallengeID || p.BodySHA256 != hashes.Body || p.PublicKeyFingerprint != Digest(publicKey) {
		return p, nil, ErrProof
	}
	canonical, _ := json.Marshal(p)
	if !bytes.Equal(raw, canonical) {
		return p, nil, ErrProof
	}
	nonce, _ := Decode(p.Nonce, 32)
	if Digest(nonce) != hashes.Nonce {
		return p, nil, ErrProof
	}
	return p, body, nil
}
