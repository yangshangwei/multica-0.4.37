package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Clients must discover this capability before relying on optional request
// fields: older servers silently ignore fields they do not recognize.
const instructionsPreconditionHeader = "X-Multica-Instructions-Precondition"

func parseInstructionsPrecondition(instructions, expectedInstructions, expectedUpdatedAt *string, rawFields map[string]json.RawMessage) (pgtype.Text, pgtype.Timestamptz, error) {
	// Match encoding/json's case-insensitive field decoding, including nulls.
	hasPrecondition := false
	for field := range rawFields {
		if strings.EqualFold(field, "expected_instructions") || strings.EqualFold(field, "expected_updated_at") {
			hasPrecondition = true
			break
		}
	}
	if !hasPrecondition {
		return pgtype.Text{}, pgtype.Timestamptz{}, nil
	}
	if instructions == nil || expectedInstructions == nil || expectedUpdatedAt == nil {
		return pgtype.Text{}, pgtype.Timestamptz{}, errors.New("instructions, expected_instructions, and expected_updated_at must be provided together when using instruction preconditions")
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, *expectedUpdatedAt)
	if err != nil {
		return pgtype.Text{}, pgtype.Timestamptz{}, errors.New("expected_updated_at must be a valid RFC3339 timestamp")
	}
	return pgtype.Text{String: *expectedInstructions, Valid: true}, pgtype.Timestamptz{Time: updatedAt, Valid: true}, nil
}

// Keep the full database timestamp so a GET response can be echoed as an
// update precondition without losing changes made within the same second.
func instructionsUpdateTimestamp(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format(time.RFC3339Nano)
}
