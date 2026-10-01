package daemon

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

var managedUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var managedDecimal = regexp.MustCompile(`^[1-9][0-9]*$`)

type managedIdentityRecord struct {
	Version           int     `json:"version"`
	DeploymentID      string  `json:"deployment_id"`
	DaemonNamespaceID string  `json:"daemon_namespace_id"`
	InstallationID    *string `json:"installation_id"`
	KeyVersion        *string `json:"key_version"`
	PublicKey         string  `json:"public_key"`
	PrivateKeySeed    string  `json:"private_key_seed"`
}

// ManagedInstallation keeps private material out of JSON, environment and argv.
type ManagedInstallation struct {
	mu        sync.RWMutex
	record    managedIdentityRecord
	directory string
}

type InstallationChallenge struct {
	ChallengeID      string `json:"challenge_id"`
	Nonce            string `json:"nonce"`
	DeploymentID     string `json:"deployment_id"`
	ExpiresAt        string `json:"expires_at"`
	SignaturePayload string `json:"signature_payload"`
	BodyPayload      string `json:"body_payload"`
}

type InstallationProof struct {
	ChallengeID      string `json:"challenge_id"`
	SignaturePayload string `json:"signature_payload"`
	BodyPayload      string `json:"body_payload"`
	Signature        string `json:"signature"`
}

type ManagedChallengeScope struct {
	Purpose, DeploymentID, OrganizationID, UserID, AuthVersion  string
	InstallationID, WorkspaceID, DaemonID, ExpectedBindingEpoch string
	Method, Path, BodyPayload                                   string
}

type managedSignaturePayload struct {
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

func managedDecode(value string, limit int) ([]byte, error) {
	if value == "" || len(value) > limit*2 {
		return nil, errors.New("invalid management base64url value")
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(data) > limit || base64.RawURLEncoding.EncodeToString(data) != value {
		return nil, errors.New("noncanonical management base64url value")
	}
	return data, nil
}

func managedNumber(value string) (int64, bool) {
	if !managedDecimal.MatchString(value) {
		return 0, false
	}
	number, err := strconv.ParseInt(value, 10, 64)
	return number, err == nil && number > 0
}

func managedCanonicalJSON(raw []byte, out any) error {
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("invalid management JSON envelope")
	}
	canonical, err := json.Marshal(out)
	if err != nil || !bytes.Equal(canonical, raw) {
		return errors.New("noncanonical management JSON envelope")
	}
	return nil
}

func managedCheckPermissions(info os.FileInfo, private bool) error {
	if !managedOwnedByCurrentUser(info) {
		return errors.New("management identity belongs to another OS user")
	}
	if runtime.GOOS != "windows" && private && info.Mode().Perm()&0077 != 0 {
		return errors.New("management identity permissions must be private")
	}
	return nil
}

func managedEnsureDirectory(path string, private bool) error {
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("management identity path must be a real directory")
	}
	return managedCheckPermissions(info, private)
}

func readManagedIdentity(path, deploymentID string) (*managedIdentityRecord, error) {
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("management identity must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || !os.SameFile(before, info) || info.Size() > 4096 {
		return nil, errors.New("management identity file changed while reading")
	}
	if err = managedCheckPermissions(info, true); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return nil, err
	}
	if len(raw) > 4096 {
		return nil, errors.New("oversized management identity")
	}
	var record managedIdentityRecord
	if err = managedCanonicalJSON(bytes.TrimSpace(raw), &record); err != nil {
		return nil, err
	}
	if record.Version != 1 || record.DeploymentID != deploymentID || !managedUUID.MatchString(deploymentID) || !managedUUID.MatchString(record.DaemonNamespaceID) ||
		(record.InstallationID == nil) != (record.KeyVersion == nil) {
		return nil, errors.New("invalid persisted installation identity")
	}
	if record.InstallationID != nil {
		if !managedUUID.MatchString(*record.InstallationID) {
			return nil, errors.New("invalid installation ID")
		}
		if _, ok := managedNumber(*record.KeyVersion); !ok {
			return nil, errors.New("invalid installation key version")
		}
	}
	seed, err := managedDecode(record.PrivateKeySeed, 32)
	if err != nil || len(seed) != 32 {
		return nil, errors.New("invalid installation private key")
	}
	public, err := managedDecode(record.PublicKey, 32)
	if err != nil || len(public) != 32 || !bytes.Equal(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey), public) {
		return nil, errors.New("installation public key does not match private key")
	}
	return &record, nil
}

func writeManagedIdentity(path string, record managedIdentityRecord) error {
	return writeManagementJSON(path, record)
}

func writeManagementJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".installation-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.Write(append(raw, '\n')); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporary, path); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		directory, err := os.Open(filepath.Dir(path))
		if err != nil {
			return err
		}
		defer directory.Close()
		return directory.Sync()
	}
	return nil
}

func withManagedIdentityLock(directory string, operation func() error) error {
	return withManagementFileLock(directory, operation)
}

func LoadManagedInstallation(homeDirectory, deploymentID string) (*ManagedInstallation, error) {
	if !managedUUID.MatchString(deploymentID) {
		return nil, errors.New("invalid management deployment ID")
	}
	root := filepath.Join(homeDirectory, ".multica")
	if err := managedEnsureDirectory(root, false); err != nil {
		return nil, err
	}
	if err := managedEnsureDirectory(filepath.Join(root, "management"), true); err != nil {
		return nil, err
	}
	directory := filepath.Join(root, "management", deploymentID)
	if err := managedEnsureDirectory(directory, true); err != nil {
		return nil, err
	}
	var identity *ManagedInstallation
	err := withManagedIdentityLock(directory, func() error {
		path := filepath.Join(directory, "installation.json")
		record, err := readManagedIdentity(path, deploymentID)
		if err != nil {
			return err
		}
		if record == nil {
			seed := make([]byte, ed25519.SeedSize)
			if _, err = rand.Read(seed); err != nil {
				return err
			}
			public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
			record = &managedIdentityRecord{Version: 1, DeploymentID: deploymentID, DaemonNamespaceID: uuid.NewString(), PublicKey: base64.RawURLEncoding.EncodeToString(public), PrivateKeySeed: base64.RawURLEncoding.EncodeToString(seed)}
			if err = writeManagedIdentity(path, *record); err != nil {
				return err
			}
		}
		identity = &ManagedInstallation{record: *record, directory: directory}
		return nil
	})
	return identity, err
}

// String deliberately omits private fields even when the owner is formatted for a log.
func (m *ManagedInstallation) String() string {
	return "ManagedInstallation{private material redacted}"
}

func (m *ManagedInstallation) snapshot() managedIdentityRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.record
}
func (m *ManagedInstallation) PublicKeyBase64() string { return m.snapshot().PublicKey }
func (m *ManagedInstallation) ManagedDaemonID(userID string) string {
	if !managedUUID.MatchString(userID) {
		return ""
	}
	digest := sha256.Sum256([]byte("multica-managed-daemon-v1\x00" + m.snapshot().DaemonNamespaceID + "\x00" + userID))
	var id uuid.UUID
	copy(id[:], digest[:16])
	id[6] = (id[6] & 0x0f) | 0x80
	id[8] = (id[8] & 0x3f) | 0x80
	return id.String()
}
func (m *ManagedInstallation) DeploymentID() string { return m.snapshot().DeploymentID }
func (m *ManagedInstallation) InstallationID() string {
	record := m.snapshot()
	if record.InstallationID == nil {
		return ""
	}
	return *record.InstallationID
}
func (m *ManagedInstallation) Fingerprint() string {
	public, _ := managedDecode(m.PublicKeyBase64(), 32)
	digest := sha256.Sum256(public)
	return hex.EncodeToString(digest[:])
}

func (m *ManagedInstallation) SaveEnrollment(installationID, keyVersion string) error {
	if !managedUUID.MatchString(installationID) {
		return errors.New("invalid enrollment result")
	}
	version, ok := managedNumber(keyVersion)
	if !ok {
		return errors.New("invalid key version")
	}
	before := m.snapshot()
	return withManagedIdentityLock(m.directory, func() error {
		path := filepath.Join(m.directory, "installation.json")
		current, err := readManagedIdentity(path, before.DeploymentID)
		if err != nil {
			return err
		}
		if current == nil || current.PublicKey != before.PublicKey || current.InstallationID != nil && *current.InstallationID != installationID {
			return errors.New("installation identity changed before enrollment was saved")
		}
		if current.KeyVersion != nil {
			previous, _ := managedNumber(*current.KeyVersion)
			if version < previous {
				return errors.New("installation key version regressed")
			}
		}
		current.InstallationID = &installationID
		current.KeyVersion = &keyVersion
		if err = writeManagedIdentity(path, *current); err != nil {
			return err
		}
		m.mu.Lock()
		m.record = *current
		m.mu.Unlock()
		return nil
	})
}

func managedNullableMatches(value *string, expected string) bool {
	if expected == "" {
		return value == nil
	}
	return value != nil && *value == expected
}

func (m *ManagedInstallation) SignChallenge(response InstallationChallenge, expected ManagedChallengeScope, now time.Time) (InstallationProof, error) {
	var empty InstallationProof
	original, err := managedDecode(response.SignaturePayload, 32768)
	if err != nil {
		return empty, err
	}
	body, err := managedDecode(response.BodyPayload, 65536)
	if err != nil {
		return empty, err
	}
	var payload managedSignaturePayload
	if err = managedCanonicalJSON(original, &payload); err != nil {
		return empty, err
	}
	record := m.snapshot()
	if expected.Purpose == "enroll" {
		var parameters struct {
			PublicKey      string `json:"public_key"`
			DesktopVersion string `json:"desktop_version"`
			OS             string `json:"os"`
		}
		if err = managedCanonicalJSON(body, &parameters); err != nil {
			return empty, err
		}
		if parameters.PublicKey != record.PublicKey || len(parameters.DesktopVersion) > 128 || (parameters.OS != "linux" && parameters.OS != "macos" && parameters.OS != "windows" && parameters.OS != "unknown") {
			return empty, errors.New("invalid enrollment body")
		}
	} else {
		var parameters struct {
			InstallationID       string  `json:"installation_id"`
			WorkspaceID          string  `json:"workspace_id"`
			DaemonID             string  `json:"daemon_id"`
			PublicKey            string  `json:"public_key"`
			ExpectedBindingEpoch *string `json:"expected_binding_epoch"`
		}
		if err = managedCanonicalJSON(body, &parameters); err != nil {
			return empty, err
		}
		if parameters.InstallationID != expected.InstallationID || parameters.WorkspaceID != expected.WorkspaceID || parameters.DaemonID != expected.DaemonID || parameters.PublicKey != record.PublicKey || !managedNullableMatches(parameters.ExpectedBindingEpoch, expected.ExpectedBindingEpoch) {
			return empty, errors.New("challenge body disagrees with binding scope")
		}
	}
	expires, validExpiry := managedNumber(payload.ExpiresAt)
	_, validAuth := managedNumber(payload.AuthVersion)
	nonce, nonceErr := managedDecode(payload.Nonce, 32)
	bodyHash := sha256.Sum256(body)
	purposePath := "/api/installations/enroll"
	if expected.Purpose == "bind" {
		purposePath = "/api/daemon/installation-bindings"
	} else if expected.Purpose == "renew" {
		purposePath = "/api/daemon/installation-bindings/renew"
	}
	if (expected.Purpose != "enroll" && expected.Purpose != "bind" && expected.Purpose != "renew") || payload.ProtocolVersion != "1" || !managedUUID.MatchString(payload.ChallengeID) || !managedUUID.MatchString(payload.OrganizationID) || !managedUUID.MatchString(payload.UserID) ||
		!validAuth || !validExpiry || nonceErr != nil || len(nonce) != 32 || payload.Purpose != expected.Purpose || payload.DeploymentID != record.DeploymentID || payload.DeploymentID != expected.DeploymentID ||
		expected.OrganizationID != "" && payload.OrganizationID != expected.OrganizationID || payload.UserID != expected.UserID || payload.AuthVersion != expected.AuthVersion ||
		!managedNullableMatches(payload.InstallationID, expected.InstallationID) || !managedNullableMatches(payload.WorkspaceID, expected.WorkspaceID) || !managedNullableMatches(payload.DaemonID, expected.DaemonID) || !managedNullableMatches(payload.ExpectedBindingEpoch, expected.ExpectedBindingEpoch) ||
		payload.PublicKeyFingerprint != m.Fingerprint() || payload.Method != "POST" || expected.Method != "POST" || payload.Path != purposePath || expected.Path != purposePath ||
		payload.BodySHA256 != hex.EncodeToString(bodyHash[:]) || response.BodyPayload != expected.BodyPayload || payload.ChallengeID != response.ChallengeID || payload.Nonce != response.Nonce || payload.DeploymentID != response.DeploymentID || payload.ExpiresAt != response.ExpiresAt ||
		expires <= now.Unix() || expires > now.Unix()+300 {
		return empty, errors.New("installation challenge does not match the current scope")
	}
	if expected.Purpose == "enroll" {
		if expected.InstallationID != "" || expected.WorkspaceID != "" || expected.DaemonID != "" || expected.ExpectedBindingEpoch != "" {
			return empty, errors.New("invalid enrollment scope")
		}
	} else {
		if record.InstallationID == nil || *record.InstallationID != expected.InstallationID || !managedUUID.MatchString(expected.WorkspaceID) || expected.DaemonID != m.ManagedDaemonID(expected.UserID) {
			return empty, errors.New("invalid daemon binding scope")
		}
		if expected.Purpose == "renew" && expected.ExpectedBindingEpoch == "" {
			return empty, errors.New("renewal requires the current binding epoch")
		}
		if expected.ExpectedBindingEpoch != "" {
			if _, ok := managedNumber(expected.ExpectedBindingEpoch); !ok {
				return empty, errors.New("invalid binding epoch")
			}
		}
	}
	seed, err := managedDecode(record.PrivateKeySeed, 32)
	if err != nil {
		return empty, fmt.Errorf("load installation signing key: %w", err)
	}
	return InstallationProof{ChallengeID: response.ChallengeID, SignaturePayload: response.SignaturePayload, BodyPayload: response.BodyPayload, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(ed25519.NewKeyFromSeed(seed), original))}, nil
}
