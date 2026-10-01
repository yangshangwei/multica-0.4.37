package installation

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestChallengeProofBindsOriginalBytesAndKey(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := Payload{ProtocolVersion: "1", Purpose: "enroll", ChallengeID: "00000000-0000-4000-8000-000000000001", DeploymentID: "00000000-0000-4000-8000-000000000002", OrganizationID: "00000000-0000-4000-8000-000000000003", UserID: "00000000-0000-4000-8000-000000000004", AuthVersion: "5", PublicKeyFingerprint: Digest(public), Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), ExpiresAt: "1900000000", Method: "POST", Path: "/api/installations/enroll"}
	body := []byte(`{"public_key":"` + base64.RawURLEncoding.EncodeToString(public) + `","desktop_version":"0.4.40","os":"macos"}`)
	challenge, hashes, err := Encode(payload, body)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(challenge.SignaturePayload)
	proof := Proof{ChallengeID: challenge.ChallengeID, SignaturePayload: challenge.SignaturePayload, BodyPayload: challenge.BodyPayload, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, raw))}
	decoded, actual, err := Verify(proof, public, hashes)
	if err != nil || decoded.UserID != payload.UserID || string(actual) != string(body) {
		t.Fatalf("valid original proof: %+v %s %v", decoded, actual, err)
	}
	for _, field := range []string{"payload", "body", "signature", "challenge", "padding", "key"} {
		t.Run(field, func(t *testing.T) {
			bad := proof
			key := public
			switch field {
			case "payload":
				var p map[string]any
				_ = json.Unmarshal(raw, &p)
				p["user_id"] = "00000000-0000-4000-8000-000000000005"
				b, _ := json.Marshal(p)
				bad.SignaturePayload = base64.RawURLEncoding.EncodeToString(b)
			case "body":
				bad.BodyPayload = base64.RawURLEncoding.EncodeToString([]byte(`{}`))
			case "signature":
				bad.Signature = base64.RawURLEncoding.EncodeToString(make([]byte, 64))
			case "challenge":
				bad.ChallengeID = "00000000-0000-4000-8000-000000000005"
			case "padding":
				bad.SignaturePayload += "="
			case "key":
				key, _, _ = ed25519.GenerateKey(rand.Reader)
			}
			if _, _, err := Verify(bad, key, hashes); err == nil {
				t.Fatal("accepted substituted proof")
			}
		})
	}
}

func TestChallengeEncodingRejectsScopeConfusion(t *testing.T) {
	p := Payload{ProtocolVersion: "1", Purpose: "bind", ChallengeID: "00000000-0000-4000-8000-000000000001", DeploymentID: "00000000-0000-4000-8000-000000000002", OrganizationID: "00000000-0000-4000-8000-000000000003", UserID: "00000000-0000-4000-8000-000000000004", AuthVersion: "1", Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), ExpiresAt: "1900000000", Method: "POST", Path: "/api/installations/enroll"}
	if _, _, err := Encode(p, []byte(`{}`)); err == nil {
		t.Fatal("bind accepted enroll path and absent binding scope")
	}
}
