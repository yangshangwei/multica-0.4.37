package projecthealth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
)

// UnavailableError is a failed statistics input after workspace and project
// identity were loaded. The transaction must roll back. Overview may present
// this explicitly incomplete result; publish and drill-down must reject it.
type UnavailableError struct {
	Statistics StatisticsSnapshot
	Err        error
}

func (e *UnavailableError) Error() string { return e.Err.Error() }
func (e *UnavailableError) Unwrap() error { return e.Err }

func unavailable(in Input, reason string, cause error) (Collection, error) {
	var databaseError *pgconn.PgError
	// A disconnected/cancelled database is globally unavailable, not a usable
	// partial overview. Serialization errors stay wrapped for whole-tx retry.
	if !errors.As(cause, &databaseError) || strings.HasPrefix(databaseError.Code, "08") || strings.HasPrefix(databaseError.Code, "57") {
		return Collection{}, cause
	}
	out, err := Compute(in)
	if err != nil {
		return Collection{}, err
	}
	s := out.Statistics
	s.Complete = false
	s.Health = "unavailable"
	s.IncompleteReasons = []string{reason}
	s.Counts = Counts{}
	s.ClosureRatio = nil
	s.LeadValid = nil
	s.LatestUpdateAt = nil
	s.ProgressAgeDays = nil
	s.Reasons = []string{}
	bytes, err := CanonicalJSON(struct {
		Version string
		Reason  string
	}{s.SnapshotVersion, reason})
	if err != nil {
		return Collection{}, err
	}
	hash := sha256.Sum256(bytes)
	s.SnapshotVersion = hex.EncodeToString(hash[:])
	return Collection{}, &UnavailableError{Statistics: s, Err: cause}
}
