package collaborationrun

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const proofRecoveryLimit = 8

type attemptCursorRecord struct {
	Version int    `json:"version"`
	After   string `json:"after"`
}

// attemptCursor serializes scheduling metadata independently of run locks. Its
// files are not .json: Retained must continue treating every .json as a run.
type attemptCursor struct {
	path   string
	lock   *os.File
	after  string
	failed bool
}

func (l *Ledger) lockAttemptCursor(name string) (*attemptCursor, error) {
	if err := privateDirectory(l.dir); err != nil {
		return nil, ErrUnconfirmed
	}
	path := filepath.Join(l.dir, ".collaboration-"+name)
	lock, err := lockPrivateFile(path + ".lock")
	if err != nil {
		if errors.Is(err, ErrBusy) {
			return nil, ErrBusy
		}
		return nil, ErrUnconfirmed
	}
	cursor := &attemptCursor{path: path + ".cursor", lock: lock}
	f, err := openPrivateFile(cursor.path)
	if errors.Is(err, os.ErrNotExist) {
		return cursor, nil
	}
	if err != nil {
		_ = cursor.Close()
		return nil, ErrUnconfirmed
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, 1025))
	closeErr := f.Close()
	var record attemptCursorRecord
	if readErr != nil || closeErr != nil || len(raw) > 1024 || decodeStrict(raw, &record) != nil || record.Version != 1 || record.After == "" {
		_ = cursor.Close()
		return nil, ErrUnconfirmed
	}
	cursor.after = record.After
	return cursor, nil
}

// advance is called before attempting the run lock or sending a request. A
// crash, lost response or busy peer therefore cannot pin the next round here.
func (c *attemptCursor) advance(key string) error {
	if c.failed || c.lock == nil {
		return ErrUnconfirmed
	}
	raw, err := json.Marshal(attemptCursorRecord{Version: 1, After: key})
	if err == nil {
		err = atomicPrivateWrite(c.path, raw)
	}
	if err != nil {
		c.failed = true
		return ErrUnconfirmed
	}
	c.after = key
	return nil
}

func (c *attemptCursor) Close() error {
	if c.lock == nil {
		return nil
	}
	err := unlockPrivateFile(c.lock)
	closeErr := c.lock.Close()
	c.lock = nil
	return errors.Join(err, closeErr)
}

// ProofRecovery holds one cross-process cursor lock for a bounded round. The
// run inventory was fully validated; unknown runs never become proof candidates.
type ProofRecovery struct {
	cursor    *attemptCursor
	entries   []RetainedRun
	keys      []string
	next      int
	remaining int
}

func (l *Ledger) BeginProofRecovery() (*ProofRecovery, error) {
	// Even a peer-held recovery cursor must not hide a corrupt run inventory.
	retained, err := l.Retained()
	if err != nil {
		return nil, err
	}
	cursor, err := l.lockAttemptCursor("proof-recovery")
	if err != nil {
		return nil, err
	}
	if cursor.after != "" {
		key, err := hex.DecodeString(cursor.after)
		if err != nil || len(key) != sha256.Size || hex.EncodeToString(key) != cursor.after {
			_ = cursor.Close()
			return nil, ErrUnconfirmed
		}
	}
	recovery := &ProofRecovery{cursor: cursor}
	for _, entry := range retained {
		if !entry.CanProve {
			continue
		}
		id := entry.Identity
		digest := sha256.Sum256([]byte(id.DeploymentID + "/" + id.InstallationID + "/" + id.RunID))
		recovery.entries = append(recovery.entries, entry)
		recovery.keys = append(recovery.keys, hex.EncodeToString(digest[:]))
	}
	// Retained follows directory-name order, including when the last attempted
	// record has since been acknowledged and is absent from this inventory.
	recovery.next = sort.Search(len(recovery.keys), func(i int) bool { return recovery.keys[i] > cursor.after })
	recovery.remaining = min(proofRecoveryLimit, len(recovery.entries))
	return recovery, nil
}

func (r *ProofRecovery) Next() (*RetainedRun, error) {
	if r.remaining == 0 {
		return nil, nil
	}
	if r.next == len(r.entries) {
		r.next = 0
	}
	if err := r.cursor.advance(r.keys[r.next]); err != nil {
		return nil, err
	}
	entry := r.entries[r.next]
	r.next++
	r.remaining--
	return &entry, nil
}

func (r *ProofRecovery) Close() error { return r.cursor.Close() }
