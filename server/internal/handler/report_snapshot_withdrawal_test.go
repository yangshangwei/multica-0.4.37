package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/reportsnapshot"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func f2WithdrawalBody(t *testing.T, report, current string) map[string]any {
	t.Helper()
	var revision int64
	dbfx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", report).Scan(&revision)
	return map[string]any{"protocol_version": 1, "scope": "report", "expected_issue_revision": revision, "expected_current_snapshot_id": current, "reason": "Withdraw this report's retained content"}
}

func f2WithdrawalRequest(report, key string, body any) *http.Request {
	req := f2HumanRequest(http.MethodPost, report, "", key, body)
	req.URL.Path += "/withdrawals"
	return req
}

func TestF2ReportSnapshotWithdrawalRejectsMalformedAndStaleIntent(t *testing.T) {
	report := f2ReportIssue(t, "")
	first := f2Capture(t, testHandler, report, uuid.NewString(), f2CaptureBody(t, report), 201)
	body := f2WithdrawalBody(t, report, first.Snapshot.SnapshotID)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{
		strings.Replace(string(raw), `"scope":"report"`, `"scope":"source"`, 1),
		strings.TrimSuffix(string(raw), "}") + `,"scope":"report"}`,
		strings.Replace(string(raw), `"scope"`, `"Scope"`, 1),
		strings.Replace(string(raw), `"reason":"Withdraw this report's retained content"`, `"reason":"  "`, 1),
		strings.TrimSuffix(string(raw), "}") + `,"unknown":true}`,
	} {
		f2Call(t, testHandler, f2WithdrawalRequest(report, uuid.NewString(), json.RawMessage(malformed))).Want(400)
	}
	stale := f2WithdrawalBody(t, report, uuid.NewString())
	f2Call(t, testHandler, f2WithdrawalRequest(report, uuid.NewString(), stale)).Want(409)
	stale = f2WithdrawalBody(t, report, first.Snapshot.SnapshotID)
	stale["expected_issue_revision"] = int64(999)
	f2Call(t, testHandler, f2WithdrawalRequest(report, uuid.NewString(), stale)).Want(409)
	f2Call(t, testHandler, f2HumanRequest("GET", report, first.Snapshot.SnapshotID, "", nil)).Want(200)
	if dbfx.Count(t, "SELECT count(*) FROM report_snapshot_control WHERE report_issue_id=$1 AND withdrawn_at IS NOT NULL", report) != 0 {
		t.Fatal("malformed withdrawal changed retained content authority")
	}
}

func TestF2ReportSnapshotReportWithdrawalIsTerminalAndRecoverable(t *testing.T) {
	project := dbfx.Project(t, "F2 withdrawn report")
	report := f2ReportIssue(t, project)
	captureBody := f2CaptureBody(t, report)
	captureKey := uuid.NewString()
	first := f2Capture(t, testHandler, report, captureKey, captureBody, 201)
	correctionBody := f2CaptureBody(t, report)
	for _, k := range []string{"start", "end"} {
		correctionBody[k] = captureBody[k]
	}
	correctionBody["correction_of"] = first.Snapshot.SnapshotID
	second := f2Capture(t, testHandler, report, uuid.NewString(), correctionBody, 201)
	key := uuid.NewString()
	body := f2WithdrawalBody(t, report, second.Snapshot.SnapshotID)
	var firstReceipt service.ReportSnapshotWithdrawalResponse
	response := f2Call(t, testHandler, f2WithdrawalRequest(report, key, body)).Want(201)
	response.JSON(&firstReceipt)
	canonical, err := reportsnapshot.CanonicalJSON(map[string]any{"report_issue_id": report, "request_key": key, "request": body})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	r := firstReceipt.WithdrawalReceipt
	if firstReceipt.Replayed || r.WithdrawalID == "" || r.RequestKey != key || r.ReportIssueID != report || r.ExpectedCurrentSnapshotID != second.Snapshot.SnapshotID || r.RequestDigest != hex.EncodeToString(digest[:]) || r.WithdrawnAt.IsZero() || r.WithdrawnAt.Nanosecond()%1000 != 0 {
		t.Fatal("withdrawal receipt lost the original explicit binding")
	}
	if strings.Contains(response.Body.String(), "\"snapshot\"") || strings.Contains(response.Body.String(), "facts_digest") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("withdrawal returned cacheable content")
	}
	for _, id := range []string{"", first.Snapshot.SnapshotID, second.Snapshot.SnapshotID, uuid.NewString()} {
		denied := f2Call(t, testHandler, f2HumanRequest("GET", report, id, "", nil)).Want(404)
		if strings.Contains(denied.Body.String(), "projection_digest") || strings.Contains(denied.Body.String(), "withdrawal") {
			t.Fatal("withdrawn read leaked content or internal control state")
		}
	}
	var replay service.ReportSnapshotWithdrawalResponse
	f2Call(t, testHandler, f2WithdrawalRequest(report, key, body)).Want(200).JSON(&replay)
	if !replay.Replayed || replay.WithdrawalReceipt != firstReceipt.WithdrawalReceipt {
		t.Fatal("original withdrawal recovery changed its receipt")
	}
	f2Call(t, testHandler, f2WithdrawalRequest(report, uuid.NewString(), body)).Want(409)
	changed := f2WithdrawalBody(t, report, second.Snapshot.SnapshotID)
	changed["reason"] = "different intent"
	f2Call(t, testHandler, f2WithdrawalRequest(report, key, changed)).Want(409)
	for _, item := range []struct {
		key  string
		body map[string]any
	}{{captureKey, captureBody}, {uuid.NewString(), captureBody}, {uuid.NewString(), correctionBody}} {
		denied := f2Call(t, testHandler, f2HumanRequest("POST", report, "", item.key, item.body)).Want(409)
		if strings.Contains(denied.Body.String(), "\"snapshot\"") {
			t.Fatal("capture recovery returned withdrawn content")
		}
	}
	otherProject := dbfx.Project(t, "F2 move-away")
	for _, id := range []string{otherProject, project} {
		dbfx.Exec(t, "UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2", id, report)
		f2Call(t, testHandler, f2HumanRequest("GET", report, first.Snapshot.SnapshotID, "", nil)).Want(404)
	}
	var recovered service.ReportSnapshotWithdrawalResponse
	f2Call(t, testHandler, f2WithdrawalRequest(report, key, body)).Want(200).JSON(&recovered)
	if recovered.WithdrawalReceipt != r || dbfx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", report) != 2 {
		t.Fatal("moving or recovering a withdrawn report restored or deleted historical seals")
	}
}

func TestF2ReportSnapshotConcurrentWithdrawalReturnsOneReceipt(t *testing.T) {
	report := f2ReportIssue(t, "")
	captured := f2Capture(t, testHandler, report, uuid.NewString(), f2CaptureBody(t, report), 201)
	body := f2WithdrawalBody(t, report, captured.Snapshot.SnapshotID)
	key := uuid.NewString()
	results := make(chan service.ReportSnapshotWithdrawalResponse, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := f2Call(t, testHandler, f2WithdrawalRequest(report, key, body))
			if response.Code != 200 && response.Code != 201 {
				t.Errorf("concurrent original withdrawal returned %d: %s", response.Code, response.Body.String())
				return
			}
			var out service.ReportSnapshotWithdrawalResponse
			response.JSON(&out)
			results <- out
		}()
	}
	wg.Wait()
	close(results)
	count, id := 0, ""
	for result := range results {
		count++
		if id != "" && id != result.WithdrawalReceipt.WithdrawalID {
			t.Fatal("same intent returned different withdrawal facts")
		}
		id = result.WithdrawalReceipt.WithdrawalID
	}
	if count != 2 || dbfx.Count(t, "SELECT count(*) FROM report_snapshot_control WHERE report_issue_id=$1 AND withdrawn_at IS NOT NULL", report) != 1 {
		t.Fatal("one withdrawal intent did not have one terminal receipt")
	}
}

type f2WithdrawalRaceStarter struct {
	inner                                   txStarter
	commitReached, releaseCommit, firstRead chan struct{}
	commitOnce, readOnce                    sync.Once
	beginCount                              atomic.Int32
}

func (s *f2WithdrawalRaceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	s.beginCount.Add(1)
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &f2WithdrawalRaceTx{Tx: tx, starter: s}, nil
}

type f2WithdrawalRaceTx struct {
	pgx.Tx
	starter *f2WithdrawalRaceStarter
}

func (tx *f2WithdrawalRaceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if tx.starter.firstRead != nil && strings.Contains(sql, "-- name: LockWorkspaceForChatSessionCreate") {
		tx.starter.readOnce.Do(func() { close(tx.starter.firstRead) })
	}
	return row
}
func (tx *f2WithdrawalRaceTx) Commit(ctx context.Context) error {
	if tx.starter.commitReached != nil {
		tx.starter.commitOnce.Do(func() {
			close(tx.starter.commitReached)
			select {
			case <-tx.starter.releaseCommit:
			case <-ctx.Done():
			}
		})
	}
	return tx.Tx.Commit(ctx)
}

func TestF2ReportSnapshotWithdrawalAndCorrectionSerializeBothOrders(t *testing.T) {
	for _, withdrawFirst := range []bool{true, false} {
		name := "correction_first"
		if withdrawFirst {
			name = "withdrawal_first"
		}
		t.Run(name, func(t *testing.T) {
			report := f2ReportIssue(t, "")
			body := f2CaptureBody(t, report)
			first := f2Capture(t, testHandler, report, uuid.NewString(), body, 201)
			withdrawBody := f2WithdrawalBody(t, report, first.Snapshot.SnapshotID)
			body["correction_of"] = first.Snapshot.SnapshotID
			capture := f2HumanRequest("POST", report, "", uuid.NewString(), body)
			withdraw := f2WithdrawalRequest(report, uuid.NewString(), withdrawBody)
			firstRequest, secondRequest := capture, withdraw
			if withdrawFirst {
				firstRequest, secondRequest = withdraw, capture
			}
			ctx, cancel := context.WithTimeout(firstRequest.Context(), 15*time.Second)
			defer cancel()
			one := &f2WithdrawalRaceStarter{inner: testHandler.TxStarter, commitReached: make(chan struct{}), releaseCommit: make(chan struct{})}
			two := &f2WithdrawalRaceStarter{inner: testHandler.TxStarter, firstRead: make(chan struct{})}
			h1, h2 := *testHandler, *testHandler
			h1.TxStarter, h2.TxStarter = one, two
			var release sync.Once
			unblock := func() { release.Do(func() { close(one.releaseCommit) }) }
			defer unblock()
			results1, results2 := make(chan *testutil.Response, 1), make(chan *testutil.Response, 1)
			go func() { results1 <- f2Call(t, &h1, firstRequest.WithContext(ctx)) }()
			select {
			case <-one.commitReached:
			case <-ctx.Done():
				t.Fatal("first writer did not reach commit barrier")
			}
			go func() { results2 <- f2Call(t, &h2, secondRequest.WithContext(ctx)) }()
			select {
			case <-two.firstRead:
			case <-ctx.Done():
				t.Fatal("second writer did not establish its old RR cut")
			}
			unblock()
			select {
			case res := <-results1:
				res.Want(201)
			case <-ctx.Done():
				t.Fatal("first writer did not finish")
			}
			select {
			case res := <-results2:
				res.Want(409)
			case <-ctx.Done():
				t.Fatal("second writer did not resolve the conflict")
			}
			if two.beginCount.Load() < 2 {
				t.Fatal("old RR view did not retry after the shared control tuple changed")
			}
			if withdrawFirst {
				f2Call(t, testHandler, f2HumanRequest("GET", report, "", "", nil)).Want(404)
				if dbfx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", report) != 1 {
					t.Fatal("correction escaped terminal withdrawal")
				}
			} else {
				f2Call(t, testHandler, f2HumanRequest("GET", report, "", "", nil)).Want(200)
				if dbfx.Count(t, "SELECT count(*) FROM report_snapshot_control WHERE report_issue_id=$1 AND withdrawn_at IS NOT NULL", report) != 0 {
					t.Fatal("stale withdrawal consumed an unreviewed correction")
				}
			}
		})
	}
}
