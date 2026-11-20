package service

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationInformationMetadataPresenceAndPrecision(t *testing.T) {
	condition := "Provide the original callback identifier."
	estimate := "2026-11-19T10:30:00.123456+08:00"
	for _, tc := range []struct {
		name      string
		managed   bool
		note      string
		condition *string
		estimate  *string
		code      string
	}{
		{"old_omitted", false, "", nil, nil, ""},
		{"managed_missing", true, "Why", nil, nil, "information_metadata_required"},
		{"missing_estimate", false, "Why", &condition, nil, "information_metadata_invalid"},
		{"missing_condition", false, "Why", nil, &estimate, "information_metadata_invalid"},
		{"managed_complete", true, "Why", &condition, &estimate, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, code := informationMetadata(tc.note, tc.condition, tc.estimate, tc.managed)
			if code != tc.code {
				t.Fatalf("metadata refusal = %q, want %q", code, tc.code)
			}
			if tc.condition != nil && tc.estimate != nil && code == "" {
				want := time.Date(2026, 11, 19, 2, 30, 0, 123456000, time.UTC)
				if value == nil || value.Condition != condition || !value.Estimate.Equal(want) {
					t.Fatalf("metadata changed the persistable instant: %+v", value)
				}
			} else if value != nil {
				t.Fatalf("omitted or invalid metadata was filled in: %+v", value)
			}
		})
	}
	for _, raw := range []string{"2026-11-19T10:30:00Z", "2026-11-19T10:30:00.1Z", estimate} {
		if _, ok := parseInformationEstimate(raw); !ok {
			t.Fatalf("valid explicit timezone/precision refused: %q", raw)
		}
	}
	for _, raw := range []string{"2026-11-19T10:30:00", "2026-11-19T10:30:00.1234567Z", "2026-11-19t10:30:00z", "2026-11-19T10:30:00+25:00", "2026-11-19T10:30:00.123456789+08:00"} {
		if _, ok := parseInformationEstimate(raw); ok {
			t.Fatalf("invalid estimate admitted: %q", raw)
		}
	}
}

func TestCollaborationInformationTextUsesDecodedUTF8Bytes(t *testing.T) {
	for _, value := range []string{"", " \n\t ", "valid\x00text", string([]byte{0xff}), strings.Repeat("x", 4097), strings.Repeat("中", 1366)} {
		if informationText(value) {
			t.Fatalf("invalid information prose admitted: bytes=%d", len(value))
		}
	}
	for _, value := range []string{"one", strings.Repeat("x", 4096), strings.Repeat("中", 1365) + "x"} {
		if !informationText(value) {
			t.Fatalf("legal decoded UTF-8 prose refused: bytes=%d", len(value))
		}
	}
}

func TestCollaborationInformationEstimateUTCWireRange(t *testing.T) {
	condition := "Verify the original receipt."
	for _, tc := range []struct {
		name, raw, want string
	}{
		{"utc_min", "0001-01-01T00:00:00Z", "0001-01-01T00:00:00.000000Z"},
		{"utc_max", "9999-12-31T23:59:59.999999Z", "9999-12-31T23:59:59.999999Z"},
		{"offset_to_min", "0001-01-01T00:01:00+00:01", "0001-01-01T00:00:00.000000Z"},
		{"offset_to_max", "9999-12-31T23:58:59.999999-00:01", "9999-12-31T23:59:59.999999Z"},
		{"offset_below_min", "0001-01-01T00:00:00+00:01", ""},
		{"offset_above_max", "9999-12-31T23:59:59-00:01", ""},
		{"local_year_zero_to_min", "0000-12-31T23:59:00-00:01", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, valid := parseInformationEstimate(tc.raw)
			metadata, code := informationMetadata("Original reason", &condition, &tc.raw, true)
			if tc.want == "" {
				if valid || metadata != nil || code != "information_metadata_invalid" {
					t.Fatalf("UTC wire overflow admitted: raw=%q valid=%t metadata=%+v code=%q", tc.raw, valid, metadata, code)
				}
				return
			}
			if !valid || value.Location() != time.UTC || informationTime(value) != tc.want || metadata == nil || code != "" || !metadata.Estimate.Equal(value) {
				t.Fatalf("valid UTC boundary changed: raw=%q value=%v metadata=%+v code=%q", tc.raw, value, metadata, code)
			}
		})
	}
}

func TestCollaborationInformationBlockRetainedUTCRange(t *testing.T) {
	utcMin := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	utcMax := time.Date(9999, time.December, 31, 23, 59, 59, 999999000, time.UTC)
	const minWire = "0001-01-01T00:00:00.000000Z"
	const maxWire = "9999-12-31T23:59:59.999999Z"
	east, west := time.FixedZone("UTC+00:01", 60), time.FixedZone("UTC-00:01", -60)
	finite := func(instant time.Time) pgtype.Timestamptz {
		return pgtype.Timestamptz{Time: instant, Valid: true}
	}
	for _, tc := range []struct {
		name     string
		estimate pgtype.Timestamptz
		want     string
	}{
		{"utc_min", finite(utcMin), minWire},
		{"utc_max", finite(utcMax), maxWire},
		{"offset_to_min", finite(utcMin.In(west)), minWire},
		{"offset_to_max", finite(utcMax.In(east)), maxWire},
		{"below_min", finite(utcMin.Add(-time.Microsecond)), ""},
		{"above_max", finite(utcMax.Add(time.Microsecond)), ""},
		{"offset_below_min", finite(utcMin.Add(-time.Microsecond).In(east)), ""},
		{"offset_above_max", finite(utcMax.Add(time.Microsecond).In(west)), ""},
		{"unset_eta", pgtype.Timestamptz{}, ""},
		{"positive_infinity", pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}, ""},
		{"negative_infinity", pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true}, ""},
		{"positive_infinity_with_time", pgtype.Timestamptz{Time: utcMax, InfinityModifier: pgtype.Infinity, Valid: true}, ""},
		{"negative_infinity_with_time", pgtype.Timestamptz{Time: utcMax, InfinityModifier: pgtype.NegativeInfinity, Valid: true}, ""},
		{"sub_microsecond", finite(utcMin.Add(time.Nanosecond)), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := db.IssueHandoff{
				Note:                  "Original reason",
				ToUserID:              util.MustParseUUID("00000000-0000-4000-8000-000000000001"),
				UnblockCondition:      pgtype.Text{String: "Verify the original receipt.", Valid: true},
				EstimatedResolutionAt: tc.estimate,
			}
			before := row
			block := informationBlock(row)
			if row != before {
				t.Fatal("projection changed retained metadata")
			}
			if block.ProtocolVersion != 1 {
				t.Fatalf("unexpected block protocol version %d", block.ProtocolVersion)
			}
			if tc.want != "" {
				if block.State != "complete" || block.EstimatedResolutionAt == nil || *block.EstimatedResolutionAt != tc.want || block.UnblockCondition == nil || *block.UnblockCondition != row.UnblockCondition.String || block.ReasonCode != nil {
					t.Fatalf("legal finite UTC boundary unavailable: %+v", block)
				}
			} else if block.State != "unavailable" || block.EstimatedResolutionAt != nil || block.UnblockCondition != nil || block.ReasonCode == nil || *block.ReasonCode != "information_metadata_invalid" {
				t.Fatalf("invalid retained UTC metadata exposed a complete body: %+v", block)
			}
		})
	}
	t.Run("both_unset", func(t *testing.T) {
		if block := informationBlock(db.IssueHandoff{}); block != (CollaborationInformationBlock{ProtocolVersion: 1, State: "legacy"}) {
			t.Fatalf("absent legacy metadata was filled in: %+v", block)
		}
	})
}
