package installation

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
)

type Heartbeat struct {
	ProtocolVersion string `json:"protocol_version"`
	Purpose         string `json:"purpose"`
	DeploymentID    string `json:"deployment_id"`
	InstallationID  string `json:"installation_id"`
	UserID          string `json:"user_id"`
	AuthVersion     string `json:"auth_version"`
	BootID          string `json:"boot_id"`
	Sequence        string `json:"sequence"`
	ReportedAt      string `json:"reported_at"`
	DesktopVersion  string `json:"desktop_version"`
	OS              string `json:"os"`
	Method          string `json:"method"`
	Path            string `json:"path"`
}

type HeartbeatProof struct {
	SignaturePayload string `json:"signature_payload"`
	Signature        string `json:"signature"`
}

func VerifyHeartbeat(proof HeartbeatProof, publicKey []byte) (Heartbeat, error) {
	var p Heartbeat
	raw, err := Decode(proof.SignaturePayload, 4096)
	if err != nil {
		return p, err
	}
	sig, err := Decode(proof.Signature, 64)
	if err != nil || len(publicKey) != 32 || len(sig) != 64 || !ed25519.Verify(publicKey, raw, sig) {
		return p, ErrProof
	}
	if json.Unmarshal(raw, &p) != nil {
		return p, ErrProof
	}
	canonical, _ := json.Marshal(p)
	if !bytes.Equal(raw, canonical) || p.ProtocolVersion != "1" || p.Purpose != "heartbeat" || p.Method != "POST" || p.Path != "/api/installations/"+p.InstallationID+"/heartbeat" {
		return p, ErrProof
	}
	for _, id := range []string{p.DeploymentID, p.InstallationID, p.UserID, p.BootID} {
		if !canonicalID(id) {
			return p, ErrProof
		}
	}
	for _, n := range []string{p.AuthVersion, p.Sequence, p.ReportedAt} {
		if !positiveDecimal(n) {
			return p, ErrProof
		}
	}
	return p, nil
}
