package collaborationrun

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

var ErrSessionBusy = errors.New("collaboration session is held by another execution")

func hostSessionKey(claim protocol.CollaborationRunClaimResponse) (string, error) {
	c := claim.HostContext
	if c == nil || claim.Preparation == nil || claim.Preparation.Host == nil ||
		validateIdentity(claim.Identity) != nil || !validUUID(claim.AgentID) || !validUUID(claim.ResourceOwnerID) ||
		!validUUID(c.AgentOwnerUserID) || !validUUID(c.RuntimeID) || !validSessionGeneration(c.SessionGeneration) ||
		!validDigest(claim.Preparation.Host.ConfigurationDigest) || len(c.StepKey) > 256 || strings.ContainsRune(c.StepKey, 0) {
		return "", ErrProtocol
	}
	role := c.Role
	if role == "continuation" {
		role = "leader"
	}
	if role != "agent" && role != "leader" && role != "member" {
		return "", ErrProtocol
	}
	parts := []string{"collaboration", claim.Identity.DeploymentID, claim.ResourceOwnerID, c.AgentOwnerUserID, claim.AgentID,
		claim.Identity.IssueID, claim.Identity.SessionID, claim.Identity.ChannelID, c.RuntimeID, claim.Preparation.Provider.Provider,
		role, c.SessionGeneration, claim.Preparation.Host.ConfigurationDigest}
	if role == "member" {
		parts = append(parts, claim.Identity.GroupID, c.StepKey)
	}
	raw, err := json.Marshal(parts)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func validSessionGeneration(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if !positiveDecimal(part) {
			return false
		}
	}
	return true
}

type hostSessionRecord struct {
	Version           int    `json:"version"`
	Key               string `json:"key"`
	SessionID         string `json:"session_id"`
	ResumeUnavailable bool   `json:"resume_unavailable"`
}

type hostSession struct {
	lock   *os.File
	path   string
	record hostSessionRecord
}

// lockHostSession is nonblocking. A waiter returns its machine slot, while a
// crashed supervisor's unknown ledger continues holding the logical identity.
func (l *Ledger) lockHostSession(key, runID string) (*hostSession, error) {
	if !validDigest(key) || !validUUID(runID) {
		return nil, ErrProtocol
	}
	base := filepath.Join(l.dir, ".session-"+key)
	lock, err := lockPrivateFile(base + ".lock")
	if errors.Is(err, ErrBusy) {
		return nil, ErrSessionBusy
	}
	if err != nil {
		return nil, err
	}
	session := &hostSession{lock: lock, path: base + ".state", record: hostSessionRecord{Version: 1, Key: key}}
	ok := false
	defer func() {
		if !ok {
			_ = session.Close()
		}
	}()
	retained, err := l.Retained()
	if err != nil {
		return nil, err
	}
	for _, run := range retained {
		if run.Identity.RunID != runID && run.Host && run.SessionKey == key && run.Unconfirmed {
			return nil, ErrSessionBusy
		}
	}
	f, err := openPrivateFile(session.path)
	if err == nil {
		raw, readErr := io.ReadAll(io.LimitReader(f, 4097))
		closeErr := f.Close()
		var record hostSessionRecord
		if readErr == nil && closeErr == nil && len(raw) <= 4096 && decodeStrict(raw, &record) == nil && record.Version == 1 &&
			record.Key == key && (record.SessionID == "" || validProviderSessionID(record.SessionID)) {
			session.record = record
		} else {
			// An unreadable pointer cannot pick a private/ordinary transcript.
			// Retained execution ownership was checked independently above.
			session.record.ResumeUnavailable = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, ErrUnconfirmed
	}
	ok = true
	return session, nil
}

func validProviderSessionID(value string) bool {
	if len(value) < 1 || len(value) > 256 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func (s *hostSession) save(sessionID string, resumeUnavailable bool) error {
	if s.lock == nil || sessionID != "" && !validProviderSessionID(sessionID) {
		return ErrUnconfirmed
	}
	next := hostSessionRecord{Version: 1, Key: s.record.Key, SessionID: sessionID, ResumeUnavailable: resumeUnavailable}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err := atomicPrivateWrite(s.path, raw); err != nil {
		return err
	}
	s.record = next
	return nil
}

func (s *hostSession) Close() error {
	if s.lock == nil {
		return nil
	}
	err := unlockPrivateFile(s.lock)
	err = errors.Join(err, s.lock.Close())
	s.lock = nil
	return err
}
