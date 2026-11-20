package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationInformationHistoryExactProvenanceAndAffectedPaging(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	id := f.issue.Issue.ID
	f.fx.Exec(t, `UPDATE issue SET status='blocked',assignee_id=$2 WHERE id=$1`, id, f.creator)
	ids := map[string]bool{}
	for i := 0; i < 55; i++ {
		row := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": id, "actor_user_id": f.handler,
			"from_user_id": f.handler, "to_user_id": f.creator, "purpose": "information", "note": "Retained legacy information row",
			"requester_user_id": f.handler, "previous_status": "todo"})
		ids[row] = true
	}
	returned := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": id, "actor_user_id": f.handler,
		"from_user_id": f.handler, "to_user_id": f.creator, "purpose": "information", "note": "Earlier returned row",
		"requester_user_id": f.handler, "previous_status": "todo", "resolution": "returned"})
	priorSuperseded := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": id, "actor_user_id": f.handler,
		"from_user_id": f.handler, "to_user_id": f.creator, "purpose": "information", "note": "Earlier superseded row",
		"requester_user_id": f.handler, "previous_status": "todo", "resolution": "superseded"})
	repair := informationTestRepair(t, f.svc, id, f.ctx, f.creator, "todo", false)
	result, err := f.svc.RepairInformation(f.ctx, repair)
	if err != nil || result.InformationRepairConfirmation == nil || result.InformationRepairConfirmation.SupersededCount != "55" {
		t.Fatalf("full affected denominator: %+v %v", result, err)
	}
	repairID := result.InformationRepairConfirmation.RepairHandoffID
	seen, cursor, pages := map[string]bool{}, "", 0
	var upper *CollaborationInformationTuple
	for {
		page, err := f.svc.ListInformationRepairAffected(f.ctx, id, repairID, CollaborationInformationHistoryQuery{Limit: 10, Cursor: cursor})
		if err != nil || page.SupersededRequestCount != "55" || page.RepairHandoffID != repairID {
			t.Fatalf("affected page: %+v %v", page, err)
		}
		if pages == 0 {
			upper = page.UpperHandoff
		} else if page.UpperHandoff == nil || upper == nil || *page.UpperHandoff != *upper {
			t.Fatal("page chain changed its upper tuple")
		}
		for _, item := range page.Items {
			key := util.UUIDToString(item.RequestID)
			if seen[key] || !ids[key] || item.Provenance.State != "legacy_unlinked" || item.Provenance.EventID.Valid || item.RequestEventID.Valid ||
				item.Provenance.EventSequence != nil || item.SupersededByRepairHandoffID != repairID {
				t.Fatalf("legacy row omitted, duplicated or assigned fabricated provenance: %+v", item)
			}
			seen[key] = true
		}
		pages++
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(seen) != 55 || pages < 6 || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=ANY($1::uuid[]) AND superseded_by_repair_handoff_id IS NULL`, []string{returned, priorSuperseded}) != 2 {
		t.Fatal("full affected identity set or earlier immutable rows changed")
	}
	detail, err := f.svc.InformationHistoryDetail(f.ctx, id, repairID)
	if err != nil || detail.Item.Kind != "information_repair" || detail.Item.Provenance.State != "linked" {
		t.Fatalf("exact repair detail: %+v %v", detail, err)
	}
	encoded, err := json.Marshal(detail.Item)
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if json.Unmarshal(encoded, &item) != nil || item["superseded_request_count"] != "55" || item["actor_user_id"] != f.creator || item["basis"] == nil || item["any"] != nil {
		t.Fatalf("repair wire lost closed flat fields: %s", encoded)
	}
	activity, err := f.svc.ListActivity(f.ctx, id, CollaborationActivityQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range activity.Items {
		if entry.Kind == "information_repaired" && (entry.InformationRepair == nil || entry.Handoff != nil || entry.Note != "") {
			t.Fatal("generic repair activity leaked note/metadata instead of safe references")
		}
	}
}

func TestCollaborationInformationHistorySourceOnlyCursorAndRevocation(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	id := f.issue.Issue.ID
	for i := 0; i < 3; i++ {
		f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": id, "actor_user_id": f.handler,
			"from_user_id": f.handler, "to_user_id": f.creator, "purpose": "information", "note": "Source-only readable legacy row",
			"requester_user_id": f.handler, "previous_status": "todo", "resolution": "returned"})
	}
	viewer := f.fx.User(t, "Independent source viewer", "information-viewer-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, f.workspace, viewer, "member")
	ctx := auth.WithAuthenticatedPrincipal(context.Background(), stageCredential(viewer).Principal)
	access, err := f.svc.ResolveIssueAccess(ctx, nil, id)
	if err != nil || !access.SourceMember || access.ParticipantEpoch.Valid || access.AccountEpoch.Valid {
		t.Fatalf("source-only authority acquired invented epochs: %+v %v", access, err)
	}
	page, err := f.svc.ListInformationHistory(ctx, id, CollaborationInformationHistoryQuery{Limit: 1})
	if err != nil || page.NextCursor == nil || len(page.Items) != 1 {
		t.Fatalf("source-only first page: %+v %v", page, err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, viewer)
	if _, err := f.svc.ListInformationHistory(ctx, id, CollaborationInformationHistoryQuery{Limit: 1, Cursor: *page.NextCursor}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("old cursor restored lost read basis: %v", err)
	}
	f.fx.Member(t, f.workspace, viewer, "member")
	if _, err := f.svc.ListInformationHistory(ctx, id, CollaborationInformationHistoryQuery{Limit: 1, Cursor: *page.NextCursor}); !collaborationErrorCode(err, "information_context_changed") {
		t.Fatalf("remove/rejoin revived old member identity: %v", err)
	}
}

func TestCollaborationInformationHistoryByteBudgetPreservesFirstUnreturnedRow(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f := newCollaborationStageFixture(t)
	id := f.issue.Issue.ID
	for i := 0; i < 4; i++ {
		f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": id, "actor_user_id": f.handler,
			"from_user_id": f.handler, "to_user_id": f.creator, "purpose": "information", "note": strings.Repeat("\"", 18000),
			"requester_user_id": f.handler, "previous_status": "todo", "resolution": "returned"})
	}
	seen, cursor := map[pgtype.UUID]bool{}, ""
	for {
		page, err := f.svc.ListInformationHistory(f.ctx, id, CollaborationInformationHistoryQuery{Limit: 50, Cursor: cursor})
		if err != nil || len(page.Items) == 0 {
			t.Fatalf("budgeted page lost a legacy note/row: %+v %v", page, err)
		}
		wire, err := json.Marshal(page)
		if err != nil || len(wire)+1 > 65536 {
			t.Fatal("encoded envelope/escaping/newline exceeded endpoint budget")
		}
		for _, item := range page.Items {
			if seen[item.HandoffID] || item.row.Note != strings.Repeat("\"", 18000) {
				t.Fatal("byte budget duplicated a row or omitted its full text")
			}
			seen[item.HandoffID] = true
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(seen) != 4 {
		t.Fatal("first unreturned row fell out of the page chain")
	}
	f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": id, "actor_user_id": f.handler, "from_user_id": f.handler,
		"to_user_id": f.creator, "purpose": "information", "note": strings.Repeat("\"", 40000), "requester_user_id": f.handler,
		"previous_status": "todo", "resolution": "returned"})
	if _, err := f.svc.ListInformationHistory(f.ctx, id, CollaborationInformationHistoryQuery{Limit: 50}); !collaborationErrorCode(err, "information_context_unavailable") {
		t.Fatalf("oversized first complete item became omitted/empty success: %v", err)
	}
}

func TestCollaborationInformationHistoryCursorBindingAndCanonicalFields(t *testing.T) {
	id, actor, row := util.MustParseUUID(uuid.NewString()), util.MustParseUUID("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"), util.MustParseUUID(uuid.NewString())
	state := &informationState{Actor: actor, Access: CollaborationIssueAccess{Issue: db.Issue{ID: id}, Revision: 1, LifecycleEpoch: 1}, AuthorityVersion: strings.Repeat("a", 64)}
	tuple := CollaborationInformationTuple{CreatedAt: informationTime(time.Now().Truncate(time.Microsecond)), HandoffID: row}
	encoded, err := encodeInformationCursor(informationPageCursor(state, "information_history", pgtype.UUID{}, tuple, tuple))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeInformationCursor(encoded, state, "information_history", pgtype.UUID{}); err != nil {
		t.Fatal(err)
	}
	changed := *state
	changed.Access.Revision = 2
	if _, err := decodeInformationCursor(encoded, &changed, "information_history", pgtype.UUID{}); !collaborationErrorCode(err, "information_context_changed") {
		t.Fatalf("valid stale cursor was not distinguished from malformed input: %v", err)
	}
	for _, malformed := range []string{encoded + "=", strings.Repeat("a", 2049), "invalid cursor"} {
		if _, err := decodeInformationCursor(malformed, state, "information_history", pgtype.UUID{}); !collaborationErrorCode(err, "invalid_information_query") {
			t.Fatalf("malformed cursor accepted: %v", err)
		}
	}
	raw, _ := base64.RawURLEncoding.DecodeString(encoded)
	changedRaw := bytes.Replace(raw, []byte(util.UUIDToString(actor)), []byte(strings.ToUpper(util.UUIDToString(actor))), 1)
	if _, err := decodeInformationCursor(base64.RawURLEncoding.EncodeToString(changedRaw), state, "information_history", pgtype.UUID{}); !collaborationErrorCode(err, "invalid_information_query") {
		t.Fatalf("noncanonical UUID payload preserved the original MAC: %v", err)
	}
}
