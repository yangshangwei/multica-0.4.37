package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestStageSourceDeniedDigestAndCachedScopeSelection(t *testing.T) {
	f := newStageReviewFixture(t)
	item := stageChildView(t, f.view(t), f.child)
	raw, _ := json.Marshal(item)
	var source map[string]any
	_ = json.Unmarshal(raw, &source)
	digest, _ := source["content_digest"].(string)
	if !item.CanOpen || !stageDigest(digest) {
		t.Fatal("source reader has no usable digest")
	}
	roles := f.roleSet()
	roles.ProductApprover = util.MustParseUUID(f.handler)
	if _, err := f.svc.ConfigureStageRoles(f.ctx, CollaborationStageRolesParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, Roles: roles}); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.StageView(f.handlerCtx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	denied := stageChildView(t, view, f.child)
	raw, _ = json.Marshal(denied)
	var projected map[string]any
	_ = json.Unmarshal(raw, &projected)
	if denied.CanOpen || projected["content_digest"] != nil || denied.Title != item.Title || denied.Identifier != item.Identifier {
		t.Errorf("source-denied child leaked its content digest or lost its reference: %s", raw)
	}
	_, err = f.svc.ConfirmStageScope(f.handlerCtx, CollaborationStageScopeParams{CollaborationWorkflowRequest: f.req(t, f.handlerCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 2, ExpectedScopeRevision: 1,
		Children: []CollaborationStageScopeChild{{IssueID: util.MustParseUUID(f.child), ContentDigest: digest}}, Reason: "Cached source selection"})
	if !collaborationErrorCode(err, "stage_scope_child_unavailable") {
		t.Fatalf("cached digest granted source-unreadable scope selection: %v", err)
	}
}

type stageAdvanceClockStarter struct {
	TxStarter
	future time.Time
	hits   *int
}
type stageAdvanceClockTx struct {
	pgx.Tx
	future    time.Time
	afterHead bool
	clocks    int
	hits      *int
}
type stageClockRow struct {
	pgx.Row
	future time.Time
}

func (r stageClockRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	*dest[0].(*pgtype.Timestamptz) = pgtype.Timestamptz{Time: r.future, Valid: true}
	return nil
}
func (s stageAdvanceClockStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &stageAdvanceClockTx{Tx: tx, future: s.future, hits: s.hits}, nil
}
func (t *stageAdvanceClockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: GetCollaborationStageReviewHead ") {
		t.afterHead = true
	}
	row := t.Tx.QueryRow(ctx, sql, args...)
	if t.afterHead && strings.Contains(sql, "-- name: ArtifactExecutionDatabaseTime ") {
		t.clocks++
		if t.clocks >= 3 {
			*t.hits++
			return stageClockRow{Row: row, future: t.future}
		}
	}
	return row
}

// Advance the DB-time seam only after the original issuer and decision were
// initially checked. The caller remains valid in the issuer/dependency case.
func TestStageAdvanceRechecksOriginalApprovalAtFinalClock(t *testing.T) {
	for _, kind := range []string{"original_approval", "original_issuer", "caller"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageReviewFixture(t)
			c := f.code(t, nil)
			if _, err := f.svc.AdvanceStage(f.ctx, f.advanceParams(t, c, "code_review")); err != nil {
				t.Fatal(err)
			}
			var issuerExpiry time.Time
			if kind == "original_issuer" {
				issuerExpiry = time.Now().Add(time.Minute)
				f.reviewerCtx = auth.WithAuthenticatedPrincipal(f.reviewerCtx, auth.AuthenticatedPrincipal{UserID: f.reviewer, Kind: auth.CredentialJWT, ExpiresAt: issuerExpiry})
			}
			decision := f.reviewParams(t, c, "approve")
			if !issuerExpiry.IsZero() {
				expires := issuerExpiry.Add(-time.Second)
				decision.ExpiresAt = &expires
			}
			approved, err := f.svc.DecideStage(f.reviewerCtx, decision)
			if err != nil {
				t.Fatal(err)
			}
			future := decision.ExpiresAt.Add(time.Second)
			if !issuerExpiry.IsZero() {
				future = issuerExpiry.Add(time.Second)
			}
			if kind == "caller" {
				expiry := time.Now().Add(time.Minute)
				f.ctx = auth.WithAuthenticatedPrincipal(f.ctx, auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialJWT, ExpiresAt: expiry})
				future = expiry.Add(time.Second)
			}
			p := f.advanceParams(t, c, "qa", approved.StageConfirmation.DecisionID)
			hits := 0
			f.svc.TxStarter = stageAdvanceClockStarter{TxStarter: f.pool, future: future, hits: &hits}
			_, err = f.svc.AdvanceStage(f.ctx, p)
			f.svc.TxStarter = f.pool
			want := "stage_decision_expired"
			if kind == "original_issuer" {
				want = "stage_signer_changed"
			}
			if kind == "caller" {
				want = "forbidden"
			}
			if hits == 0 || !collaborationErrorCode(err, want) {
				t.Fatalf("final clock failed to reject %s: hits=%d err=%v", kind, hits, err)
			}
			if f.view(t).Stage.Current != "code_review" {
				t.Fatal("expired authority advanced the stage")
			}
		})
	}
}
