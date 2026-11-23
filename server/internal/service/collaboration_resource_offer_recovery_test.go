package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

type resourceOfferFailureStarter struct {
	TxStarter
	queryName string
	eventKind string
}

type resourceOfferFailureTx struct {
	pgx.Tx
	queryName string
	eventKind string
}

func (s resourceOfferFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return resourceOfferFailureTx{Tx: tx, queryName: s.queryName, eventKind: s.eventKind}, nil
}

func (tx resourceOfferFailureTx) fail(sql string, args []any) bool {
	return strings.HasPrefix(sql, "-- name: "+tx.queryName+" ") && (tx.eventKind == "" || len(args) > 3 && args[3] == tx.eventKind)
}

func (tx resourceOfferFailureTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.fail(sql, args) {
		return queueFailureRow{errors.New("injected resource offer persistence failure")}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func (tx resourceOfferFailureTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx.fail(sql, args) {
		return nil, errors.New("injected resource offer persistence failure")
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func (tx resourceOfferFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.fail(sql, args) {
		return pgconn.CommandTag{}, errors.New("injected resource offer persistence failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func resourceOfferStateSnapshot(t *testing.T, f *collaborationOfferFixture) string {
	t.Helper()
	control := controlStateSnapshot(t, &collaborationQueueFixture{collaborationFixture: f.collaborationFixture, issue: f.issue})
	var offers string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'offers', (SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_resource_offer o WHERE o.issue_id=$1),
		'source', (SELECT to_jsonb(i) FROM issue_collaboration i WHERE i.issue_id=$1),
		'operations', (SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_operation o WHERE o.scope_kind='issue' AND o.scope_id=$1)
	)::text`, f.issue.Issue.ID).Scan(&offers)
	return control + offers
}

func TestCollaborationResourceOfferRequiredWritesRollbackTogether(t *testing.T) {
	for _, action := range []string{"offer", "withdraw", "adopt"} {
		queries := []string{"CreateCollaborationEvent", "CompleteCollaborationOperation"}
		switch action {
		case "offer":
			queries = append(queries, "CreateCollaborationResourceOffer", "TryLockCollaborationAuthorityAgents", "TryLockCollaborationResourceOfferMember")
		case "withdraw":
			queries = append(queries, "WithdrawCollaborationResourceOffer")
		case "adopt":
			queries = append(queries, "CreateConsentedCollaborationTarget", "EnsureCollaborationChannel", "BindCollaborationTargetChannel", "AdvanceCollaborationTargetSet", "AdoptCollaborationResourceOffer")
		}
		for _, query := range queries {
			t.Run(action+"/"+query, func(t *testing.T) {
				f := newCollaborationOfferFixture(t)
				p := f.offerParams(t)
				run := func() error {
					_, err := f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible)
					return err
				}
				if action != "offer" {
					offer := f.offer(t)
					if action == "withdraw" {
						p := CollaborationWithdrawResourceOfferParams{CollaborationQueueRequest: f.request(t, f.outsider), OfferID: util.MustParseUUID(*offer.OfferID), ExpectedOfferVersion: 1}
						run = func() error { _, err := f.svc.WithdrawResourceOffer(collaborationContext(f.outsider), p); return err }
					} else {
						p := f.adoptParams(t, offer, f.creator)
						run = func() error { _, err := f.svc.AdoptResourceOffer(f.ctx, p); return err }
					}
				}
				before := resourceOfferStateSnapshot(t, f)
				fault := resourceOfferFailureStarter{TxStarter: f.pool, queryName: query}
				if action == "adopt" && query == "CreateCollaborationEvent" {
					// Fail the second event, after the target and adopted offer
					// have both been written inside the same transaction.
					fault.eventKind = "resource_offer_adopted"
				}
				f.svc.TxStarter = fault
				if err := run(); err == nil || !strings.Contains(err.Error(), "injected resource offer persistence failure") {
					t.Fatalf("required offer write did not fail: %v", err)
				}
				if after := resourceOfferStateSnapshot(t, f); after != before {
					t.Fatal("failed required write left a target, offer, queue, event sequence or operation changed")
				}
			})
		}
	}
}

func TestCollaborationResourceOfferUnknownCommitRecoversOriginalOperation(t *testing.T) {
	for _, action := range []string{"offer", "withdraw", "adopt"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationOfferFixture(t)
			p := f.offerParams(t)
			actor, key, code, kind := f.outsider, p.RequestID, "resource_offered", "offer_resource"
			run := func() (CollaborationQueueReceipt, error) {
				return f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible)
			}
			if action != "offer" {
				offer := f.offer(t)
				if action == "withdraw" {
					p := CollaborationWithdrawResourceOfferParams{CollaborationQueueRequest: f.request(t, f.outsider), OfferID: util.MustParseUUID(*offer.OfferID), ExpectedOfferVersion: 1}
					key, code, kind = p.RequestID, "resource_offer_withdrawn", "withdraw_resource_offer"
					run = func() (CollaborationQueueReceipt, error) {
						return f.svc.WithdrawResourceOffer(collaborationContext(f.outsider), p)
					}
				} else {
					p := f.adoptParams(t, offer, f.handler)
					actor, key, code, kind = f.handler, p.RequestID, "resource_offer_adopted", "adopt_resource_offer"
					run = func() (CollaborationQueueReceipt, error) {
						return f.svc.AdoptResourceOffer(collaborationContext(f.handler), p)
					}
				}
			}
			fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
			f.svc.TxStarter = fault
			if _, err := run(); err == nil || fault.begins.Load() != 1 {
				t.Fatalf("unknown offer commit succeeded or was retried: %v (begins %d)", err, fault.begins.Load())
			}
			f.svc.TxStarter = f.pool
			before := resourceOfferStateSnapshot(t, f)
			recovered, err := f.svc.GetResourceOperation(collaborationContext(actor), f.issue.Issue.ID, key)
			if err != nil || recovered.Code != code || recovered.RequestID != key || recovered.OfferID == nil || recovered.Action != kind {
				t.Fatalf("original GET recovery: %+v %v", recovered, err)
			}
			replay, err := run()
			if err != nil || !reflect.DeepEqual(replay, recovered) || resourceOfferStateSnapshot(t, f) != before {
				t.Fatalf("same-key replay changed committed state: %+v %v", replay, err)
			}
			if f.fx.Count(t, "SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind=$2 AND actor_user_id=$3 AND payload->>'offer_id'=$4 AND payload->>'action'=$5", f.issue.Issue.ID, code, actor, *recovered.OfferID, kind) != 1 {
				t.Fatal("offer event lost its exact identity or true actor")
			}
			if _, err = f.svc.GetResourceOperation(f.ctx, f.issue.Issue.ID, key); !collaborationErrorCode(err, "operation_not_found") {
				t.Fatalf("creator recovered another actor's operation: %v", err)
			}
		})
	}
}

func TestCollaborationResourceOfferReplayAndRecoveryDoNotRecomputeOldAuthority(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	p := f.offerParams(t)
	original, err := f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, "UPDATE agent SET instructions='Configuration changed after offering' WHERE id=$1", f.resource.AgentID)
	replay, err := f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible)
	if err != nil || !reflect.DeepEqual(replay, original) {
		t.Fatalf("replay evaluated the later resource configuration: %+v %v", replay, err)
	}
	changed := p
	changed.Roster = append([]collaboration.TargetResource(nil), p.Roster...)
	changed.Roster[0].RuntimeID = uuid.NewString()
	if _, err = f.svc.OfferResource(collaborationContext(f.outsider), changed, offerVisible); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("same key changed its selected resource: %v", err)
	}
	withdraw := CollaborationWithdrawResourceOfferParams{CollaborationQueueRequest: p.CollaborationQueueRequest, OfferID: util.MustParseUUID(*original.OfferID), ExpectedOfferVersion: 1}
	if _, err = f.svc.WithdrawResourceOffer(collaborationContext(f.outsider), withdraw); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("same key changed its action: %v", err)
	}
	if _, err = f.svc.Leave(collaborationContext(f.outsider), f.workflowRequest(t, f.outsider)); err != nil {
		t.Fatal(err)
	}
	redacted, err := f.svc.GetResourceOperation(collaborationContext(f.outsider), f.issue.Issue.ID, p.RequestID)
	if err != nil || !redacted.Redacted || redacted.OfferID != nil || redacted.OfferVersion != 0 || redacted.TargetID.Valid || redacted.Code != original.Code {
		t.Fatalf("departure recovery returned private offer facts: %+v %v", redacted, err)
	}
	if _, err = f.svc.GetResourceOffer(collaborationContext(f.outsider), f.issue.Issue.ID, util.MustParseUUID(*original.OfferID)); err == nil {
		t.Fatal("own historical receipt restored departed offer access")
	}
	f.fx.Exec(t, "UPDATE \"user\" SET disabled_at=now() WHERE id=$1", f.outsider)
	if _, err = f.svc.GetResourceOperation(collaborationContext(f.outsider), f.issue.Issue.ID, p.RequestID); err == nil {
		t.Fatal("disabled actor recovered an offer receipt")
	}
	f.fx.Exec(t, "UPDATE \"user\" SET disabled_at=NULL WHERE id=$1", f.outsider)
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": f.outsider, "username": "offer-recovery-" + uuid.NewString(),
		"password_hash": "test-only-unused-hash", "session_version": 2}, "user_id=$1", f.outsider)
	t.Setenv("MULTICA_AUTH_MODE", "password")
	stale := auth.WithPasswordSession(collaborationContext(f.outsider), auth.PasswordSession{UserID: f.outsider, Version: 1, Kind: "jwt"})
	if _, err = f.svc.GetResourceOperation(stale, f.issue.Issue.ID, p.RequestID); err == nil {
		t.Fatal("old password session recovered the restored account's receipt")
	}
	current := auth.WithPasswordSession(collaborationContext(f.outsider), auth.PasswordSession{UserID: f.outsider, Version: 2, Kind: "jwt"})
	if _, err = f.svc.GetResourceOperation(current, f.issue.Issue.ID, p.RequestID); err != nil {
		t.Fatalf("current password account lost its minimal own receipt: %v", err)
	}
}

func TestCollaborationResourceOfferMalformedReceiptNeverConfirmsSuccess(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	p := f.offerParams(t)
	receipt, err := f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"offer_id", "offer_version", "action", "issue_id", "request_id", "status", "target_id"} {
		t.Run(field, func(t *testing.T) {
			broken := receipt
			switch field {
			case "offer_id":
				broken.OfferID = nil
			case "offer_version":
				broken.OfferVersion = 2
			case "action":
				broken.Action = "adopt_resource_offer"
			case "issue_id":
				broken.IssueID = util.MustParseUUID(uuid.NewString())
			case "request_id":
				broken.RequestID = uuid.NewString()
			case "status":
				broken.Status = 409
			case "target_id":
				broken.TargetID = util.MustParseUUID(uuid.NewString())
			}
			encoded, err := json.Marshal(broken)
			if err != nil {
				t.Fatal(err)
			}
			f.fx.Exec(t, "UPDATE collaboration_operation SET result=$3 WHERE scope_id=$1 AND request_id=$2", f.issue.Issue.ID, p.RequestID, encoded)
			before := resourceOfferStateSnapshot(t, f)
			if _, err = f.svc.GetResourceOperation(collaborationContext(f.outsider), f.issue.Issue.ID, p.RequestID); err == nil {
				t.Fatal("malformed offer receipt settled GET recovery")
			}
			if _, err = f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible); err == nil || resourceOfferStateSnapshot(t, f) != before {
				t.Fatal("malformed offer receipt replay succeeded or recreated work")
			}
		})
	}
}
