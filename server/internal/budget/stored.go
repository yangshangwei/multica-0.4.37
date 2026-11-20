package budget

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// The stored form of an entry is the bytes kept for it, one row per entry: the
// JSON encoding of the Entry, with every key in the order of the struct, no
// white space, and amounts and counts as decimal strings. An entry has one
// stored form and a stored form is of one entry, so the bytes can be digested
// and compared in place of the entry.

// EntryFormVersion is the version of the stored form this package writes and
// reads. A row carries the version of its form.
//
// A change to the stored form (a key, the order of the keys, the way a value
// is written) needs a new version here and a new entryDigestDomain with it,
// and DecodeEntry must go on reading the versions that rows already have.
const EntryFormVersion = 1

// entryDigestDomain goes in front of a stored form when it is digested. It
// names the form version, so the digests of two versions cannot be taken for
// each other.
const entryDigestDomain = "budget-entry-v1\n"

// The limits of the table that keeps the entries, in bytes.
const (
	// MaxEntryIDBytes is the longest entry ID.
	MaxEntryIDBytes = 200
	// MaxStoredEntryBytes is the longest stored form.
	MaxStoredEntryBytes = 65536
)

// EncodeEntry returns the stored form of e and its digest, the one Digest
// gives. An entry that no ledger accepts has no stored form: it is refused
// with the error a ledger gives for it. An entry whose ID or stored form is
// longer than the table takes is refused with ErrInvalidEntry. Nothing here
// depends on a ledger: whether the reservation exists and whether the ID is
// taken are for With to say.
func EncodeEntry(e Entry) (raw []byte, digest string, err error) {
	if len(e.ID) > MaxEntryIDBytes {
		return nil, "", fmt.Errorf("%w: an entry ID of %d bytes, the limit is %d", ErrInvalidEntry, len(e.ID), MaxEntryIDBytes)
	}
	if err := validateEntry(e); err != nil {
		return nil, "", err
	}
	raw, err = json.Marshal(e)
	if err != nil {
		return nil, "", err
	}
	if len(raw) > MaxStoredEntryBytes {
		return nil, "", fmt.Errorf("%w: entry %q: a stored form of %d bytes, the limit is %d", ErrInvalidEntry, e.ID, len(raw), MaxStoredEntryBytes)
	}
	return raw, storedDigest(raw), nil
}

// DecodeEntry reads the stored form of one entry, as a row holds it under
// formVersion. With ErrInvalidStoredForm it refuses every version but
// EntryFormVersion, and every byte string that is not exactly the stored form
// of the entry it reads as: a key that is missing, added, repeated, moved or
// in another letter case, white space, another way of writing a value,
// anything behind the entry. So two rows that differ in one byte never give
// the same entry, and the digest of a row's bytes is the digest of its entry.
//
// It proves the form and nothing else. The entry may be one no ledger
// accepts; With judges that when the entry is replayed.
func DecodeEntry(formVersion int, raw []byte) (Entry, error) {
	if formVersion != EntryFormVersion {
		return Entry{}, fmt.Errorf("%w: form version %d, only version %d is read", ErrInvalidStoredForm, formVersion, EntryFormVersion)
	}
	// The cause is reported as text: an error of the decoder is not an error
	// of a replay, even where the two share a sentinel.
	var e Entry
	if err := json.Unmarshal(raw, &e); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", ErrInvalidStoredForm, err)
	}
	// encoding/json reads many texts as one entry. The stored form is the one
	// text it writes for that entry.
	form, err := json.Marshal(e)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: %v", ErrInvalidStoredForm, err)
	}
	if !bytes.Equal(form, raw) {
		return Entry{}, fmt.Errorf("%w: %d bytes that read as an entry whose stored form has %d", ErrInvalidStoredForm, len(raw), len(form))
	}
	return e, nil
}

// Digest is the SHA-256 of the entry's stored form under a versioned domain
// prefix, in lowercase hex. It does not judge the entry; EncodeEntry does.
func (e Entry) Digest() (string, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return storedDigest(raw), nil
}

// storedDigest is the digest of a stored form.
func storedDigest(raw []byte) string {
	h := sha256.New()
	h.Write([]byte(entryDigestDomain))
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}
