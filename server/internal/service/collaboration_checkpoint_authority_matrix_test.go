package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationCheckpointAuthorityTransactionOrder(t *testing.T) {
	for _, action := range []string{"stop", "revoke_grant", "author_leave"} {
		for _, first := range []string{"checkpoint", "authority"} {
			t.Run(action+"/"+first+"_first", func(t *testing.T) {
				f, caps := newHostCollaborationFixture(t)
				var leave CollaborationWorkflowRequest
				if action == "author_leave" {
					// The input author remains a participant, but must not be the
					// current handler when exercising voluntary departure.
					transferred, err := f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
						CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), TargetUserID: util.MustParseUUID(f.creator)}})
					if err != nil {
						t.Fatal(err)
					}
					leave = workflowRequest(f.collaborationQueueFixture, transferred.CollaborationRevision)
				}
				input := submitHostCommand(t, f, f.handler, "retain this author's discoveries")
				run, claim := claimHostRun(t, f, caps)
				if len(claim.Inputs) != 1 || claim.Inputs[0].AuthorUserID != f.handler {
					t.Fatal("fixture did not claim the departing human's input")
				}
				launch := hostPermitRequest(claim, 1, "provider")
				permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, launch)
				if err != nil {
					t.Fatal(err)
				}
				prefix := checkpointFor(claim, permit)
				prefix.Messages[0].Body = "confirmed prefix before the authority race"
				if ack, err := f.svc.RecordRuntimeCheckpoint(t.Context(), run, permit.BusinessToken, prefix); err != nil || !ack.Accepted || ack.RetainedThrough != 1 {
					t.Fatalf("initial checkpoint: %+v: %v", ack, err)
				}
				racing := prefix
				racing.Sequence, racing.MessageOffset = 2, 1
				racing.Messages = []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "discovery competing with authority loss"}}
				stop := f.issueControl(t)
				stop.ExecutionViewVersion = 6
				revoke := CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}
				ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				defer cancel()
				ownerCtx, cancelOwner := context.WithTimeout(f.ctx, 15*time.Second)
				defer cancelOwner()
				authorCtx, cancelAuthor := context.WithTimeout(collaborationPasswordContext(util.MustParseUUID(f.handler), 1), 15*time.Second)
				defer cancelAuthor()
				appendCheckpoint := func(svc *CollaborationService) error {
					ack, err := svc.RecordRuntimeCheckpoint(ctx, run, permit.BusinessToken, racing)
					if err == nil && (!ack.Accepted || ack.RetainedThrough != 2 || ack.Identity != claim.Identity) {
						return fmt.Errorf("racing checkpoint returned an invalid receipt: %+v", ack)
					}
					return err
				}
				withdrawAuthority := func(svc *CollaborationService) error {
					switch action {
					case "stop":
						_, err := svc.StopQueue(ownerCtx, stop)
						return err
					case "revoke_grant":
						receipt, err := svc.RevokeGrant(ownerCtx, revoke)
						if err == nil && receipt.Code != "grant_revoked" {
							return fmt.Errorf("grant was not revoked: %s", receipt.Code)
						}
						return err
					default:
						receipt, err := svc.Leave(authorCtx, leave)
						if err == nil && receipt.Code != "participant_left" {
							return fmt.Errorf("input author did not leave: %s", receipt.Code)
						}
						return err
					}
				}
				leading, following := appendCheckpoint, withdrawAuthority
				if first == "authority" {
					leading, following = withdrawAuthority, appendCheckpoint
				}
				release := make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				defer unblock()
				barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
				held := *f.svc
				held.TxStarter = barrier
				leadingDone := make(chan error, 1)
				go func() { leadingDone <- leading(&held) }()
				var blocker int
				select {
				case blocker = <-barrier.reached:
				case err := <-leadingDone:
					t.Fatalf("first transaction never reached the commit barrier: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				followingDone := make(chan error, 1)
				go func() { followingDone <- following(f.svc) }()
				waitFIFOBlocked(t, f.collaborationQueueFixture, blocker, followingDone)
				unblock()
				if err := checkpointMatrixResult(t, ctx, leadingDone); err != nil {
					t.Fatalf("first transaction failed: %v", err)
				}
				err = checkpointMatrixResult(t, ctx, followingDone)
				retained := 2
				if first == "authority" {
					retained = 1
					if !collaborationErrorCode(err, "business_capability_required") {
						t.Fatalf("checkpoint used authority from before the winning transaction: %v", err)
					}
				} else if err != nil {
					t.Fatalf("authority withdrawal failed after the admitted checkpoint: %v", err)
				}
				intent, err := f.svc.RuntimeStopIntent(ctx, run, claim.TerminationToken)
				if err != nil || !intent.StopRequested {
					t.Fatalf("authority withdrawal did not request cleanup: %+v: %v", intent, err)
				}
				if action == "revoke_grant" && f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL AND epoch=$2`, f.grant.ID, f.grant.Epoch+1) != 1 {
					t.Fatal("grant revocation was not committed")
				}
				if action == "author_leave" && f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2 AND revoked_at IS NOT NULL`, f.issue.Issue.ID, f.handler) != 1 {
					t.Fatal("input author departure was not committed")
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND author_user_id=$2`, input.CommandID, f.handler) != 1 {
					t.Fatal("authority loss rewrote the original input author")
				}
				ack, err := f.svc.RecordRuntimeCheckpoint(ctx, run, permit.BusinessToken, prefix)
				if err != nil || !ack.Accepted || ack.RetainedThrough != 1 {
					t.Fatalf("original accepted checkpoint lost its ACK after authority loss: %+v: %v", ack, err)
				}
				encoded, err := json.Marshal(ack)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err = json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				for key := range fields {
					switch key {
					case "version", "identity", "stream_id", "sequence", "retained_through", "accepted":
					default:
						t.Fatalf("lost-ACK replay exposed a non-receipt field: %s", key)
					}
				}
				changed := prefix
				changed.Messages = []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "rewrite after authority loss"}}
				if _, err = f.svc.RecordRuntimeCheckpoint(ctx, run, permit.BusinessToken, changed); !collaborationErrorCode(err, "checkpoint_conflict") {
					t.Fatalf("changed accepted checkpoint was not rejected: %v", err)
				}
				late := changed
				late.Sequence, late.MessageOffset = retained+1, retained
				if _, err = f.svc.RecordRuntimeCheckpoint(ctx, run, permit.BusinessToken, late); !collaborationErrorCode(err, "business_capability_required") {
					t.Fatalf("new content was not rejected after authority loss: %v", err)
				}
				if _, err = f.svc.RecordRuntimeCheckpoint(ctx, run, claim.TerminationToken, late); !collaborationErrorCode(err, "business_capability_required") {
					t.Fatalf("termination capability acquired checkpoint authority: %v", err)
				}
				observed := 3
				seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: claim.Identity, AttemptSeq: permit.AttemptSeq,
					AttemptNonce: permit.AttemptNonce, StreamID: prefix.StreamID, AcknowledgedThrough: 0, ObservedThrough: &observed, Reason: "stopped"}
				sealed, err := f.svc.SealRuntimeRecording(ctx, run, claim.TerminationToken, seal)
				if err != nil || !sealed.Sealed || sealed.RetainedThrough != retained || sealed.ObservedThrough == nil || *sealed.ObservedThrough != observed {
					t.Fatalf("metadata-only seal lost the committed prefix or missing tail: %+v: %v", sealed, err)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, run) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NULL AND proof_state='unknown'`, run) != 1 {
					t.Fatal("recording seal replaced termination proof or released capacity")
				}
				ended := hostLaunchEnded(launch, true)
				if _, err = f.svc.AcceptRuntimeLaunchTermination(ctx, run, claim.TerminationToken, ended); err != nil {
					t.Fatal(err)
				}
				proof := hostFinalProof(t, claim, ended)
				for range 2 {
					if _, err = f.svc.AcceptRuntimeProof(ctx, run, claim.TerminationToken, proof); err != nil {
						t.Fatalf("original cleanup proof or its replay was refused: %v", err)
					}
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NOT NULL AND proof_state='host_exit_observed'`, run) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, run) != 1 {
					t.Fatal("original cleanup proof did not settle the stopped run and release capacity")
				}
				viewer := WithCollaborationExecutionViewVersion(f.ctx, 6)
				page, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, run, CollaborationRunRecordsQuery{Limit: 100})
				if err != nil {
					t.Fatal(err)
				}
				var bodies []string
				for _, item := range page.Items {
					if item.Kind == "message" {
						bodies = append(bodies, item.Body)
					}
				}
				want := []string{prefix.Messages[0].Body}
				if retained == 2 {
					want = append(want, racing.Messages[0].Body)
				}
				if !reflect.DeepEqual(bodies, want) || page.Recording.Status != "partial" || page.Recording.Retention == nil || page.Recording.Retention.RetainedThrough != retained {
					t.Fatalf("human history lost or duplicated admitted content: bodies=%v recording=%+v", bodies, page.Recording)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='record_checkpoint'`, run) != retained ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='recording_seal'`, run) != 1 {
					t.Fatal("denial, replay or cleanup changed the immutable recording event inventory")
				}
			})
		}
	}
}

func checkpointMatrixResult(t *testing.T, ctx context.Context, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Fatal("competing service transaction did not finish after releasing its lock: ", ctx.Err())
		return ctx.Err()
	}
}
