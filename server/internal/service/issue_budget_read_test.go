package service

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestIssueBudgetReadProjection(t *testing.T) {
	state := IssueBudgetRootState{Root: db.IssueBudgetRoot{
		ID:             util.MustParseUUID("7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f"),
		RootIssueID:    util.MustParseUUID("0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314"),
		AdmissionState: "open", Mode: "observe", PolicyRevision: 3, PolicyEpoch: 1,
		LimitTicks: pgtype.Int8{Int64: 20, Valid: true},
	}}
	p := budget.Projection{ActualKnown: 7, Estimated: 99, Reserved: 4, UnknownHold: 2, Overrun: 1,
		Coverage: budget.Coverage{IncludedAttempts: 3, NotApplicable: []string{"ignored-ledger-exclusion"}}}
	out := issueBudgetShowProjection(state, &p, nil)
	if !out.TotalKnown || out.RemainingTicks == nil || *out.RemainingTicks != 7 {
		t.Fatalf("remaining must subtract A+R+H, not E: %+v", out)
	}
	if !reflect.DeepEqual(out.Coverage.NotApplicable, []string{"task_without_issue", "collaboration_host_run", "plugin_external_call"}) || *out.Coverage.IncludedAttempts != 3 {
		t.Fatalf("coverage = %+v", out.Coverage)
	}
	state.Root.LimitTicks.Int64 = 5
	if out := issueBudgetShowProjection(state, &p, nil); out.RemainingTicks == nil || *out.RemainingTicks != 0 {
		t.Fatalf("over-limit remaining = %v", out.RemainingTicks)
	}
	state.Root.LimitTicks.Valid = false
	if out := issueBudgetShowProjection(state, &p, nil); out.RemainingTicks != nil || !out.TotalKnown {
		t.Fatalf("unset limit = %+v", out)
	}
	state.Root.Mode = "off"
	state.TombstonedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	if off := issueBudgetShowProjection(state, &p, nil); off.AdmissionState != "tombstoned" || off.Mode != "off" || *off.Amounts != *out.Amounts {
		t.Fatalf("off/deleted changes retained amounts: %+v", off)
	}
	p.Unknown = []budget.UnknownItem{{Reason: budget.ReasonPreActivationUnknown}, {Reason: "lost_contact"}, {Reason: "lost_contact"}}
	gaps := []db.CountIssueBudgetUncoveredGapsRow{
		{Reason: "lost_contact", GapCount: 1}, {Reason: "observation_off", GapCount: 2},
		{Reason: "in_flight_legacy", GapCount: 3}, {Reason: "zero", GapCount: 0},
		{Reason: "future_reason", GapCount: 1},
	}
	out = issueBudgetShowProjection(state, &p, gaps)
	want := []IssueBudgetUnknownCount{{"future_reason", 1}, {"lost_contact", 3}, {"observation_off", 2}}
	if !reflect.DeepEqual(out.Unknown.ByReason, want) || out.Unknown.PreActivation != 4 || out.TotalKnown || out.RemainingTicks != nil {
		t.Fatalf("merged unknown set = %+v", out)
	}
	state.Root.ActualKnownTicks = 999
	out = issueBudgetShowProjection(state, nil, gaps)
	if out.LedgerState != "unavailable" || out.Amounts != nil || out.RemainingTicks != nil || out.TotalKnown || out.Coverage.IncludedAttempts != nil || out.Unknown.PreActivation != 3 {
		t.Fatalf("unavailable leaked unverified figures: %+v", out)
	}
	if out.Unknown.ByReason[1].Count != 1 {
		t.Fatalf("unavailable retained ledger unknowns: %+v", out.Unknown)
	}
	out = issueBudgetShowProjection(state, nil, nil)
	encoded, err := json.Marshal(out)
	if err != nil || !strings.Contains(string(encoded), `"by_reason":[]`) {
		t.Fatalf("empty reasons must be []: %s (%v)", encoded, err)
	}
}

func TestIssueBudgetReadCursorAndTime(t *testing.T) {
	local := time.Date(2026, 11, 19, 9, 2, 3, 456789000, time.FixedZone("east", 8*3600))
	if got := IssueBudgetTime(local); got != "2026-11-19T01:02:03.456789Z" {
		t.Fatalf("UTC microseconds lost: %q", got)
	}
	id := util.MustParseUUID("7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f")
	cursor := encodeIssueBudgetCursor(local, id)
	got, gotID, err := decodeIssueBudgetCursor(cursor)
	if err != nil || !got.Time.Equal(local) || gotID != id {
		t.Fatalf("cursor roundtrip = %v %v %v", got, gotID, err)
	}
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, bad := range []string{"", "!", "e30", cursor + "=", cursor + "A",
		b64("2026-11-19T01:02:03.456789Z,not-a-uuid"),
		b64("2026-11-19T01:02:03.456789Z,7D0C2B9E-5A41-4F7C-8E63-1B2A9C4D5E6F"),
		b64("2026-11-19T09:02:03.456789+08:00," + util.UUIDToString(id)),
		b64("2026-11-19T01:02:03.4567891Z," + util.UUIDToString(id)),
		b64("2026-11-19T01:02:03.456789Z," + util.UUIDToString(id) + ",extra"),
	} {
		if _, _, err := decodeIssueBudgetCursor(bad); IssueBudgetErrorCode(err) != IssueBudgetInvalidRequest {
			t.Errorf("accepted noncanonical cursor %q: %v", bad, err)
		}
	}
	if issueBudgetTicks(pgtype.Int8{}) != nil || *issueBudgetTicks(pgtype.Int8{Valid: true}) != 0 {
		t.Fatal("nullable tick mapping conflates unsized with sized zero")
	}
}
