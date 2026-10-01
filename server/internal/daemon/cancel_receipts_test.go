package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/multica-ai/multica/server/pkg/agent"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagedCancellationWirePreservesExecutionFence(t *testing.T) {
	raw := []byte(`{"id":"11111111-1111-4111-8111-111111111111","runtime_id":"22222222-2222-4222-8222-222222222222","execution_fence":{"runtime_id":"22222222-2222-4222-8222-222222222222","dispatched_at":"2026-10-02T00:00:00.123456Z"}}`)
	var task Task
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatal(err)
	}
	roundtrip, _ := json.Marshal(task)
	var value map[string]any
	_ = json.Unmarshal(roundtrip, &value)
	if value["execution_fence"] == nil {
		t.Fatal("claim discarded immutable execution fence")
	}
}

func TestManagedCancellationAckPreservesOptionalProofGroup(t *testing.T) {
	received := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		received <- body
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	var ack TaskCancelAck
	_ = json.Unmarshal([]byte(`{"operation_id":"33333333-3333-4333-8333-333333333333","binding_epoch":"7","execution_fence":{"runtime_id":"22222222-2222-4222-8222-222222222222","dispatched_at":"2026-10-02T00:00:00.123456Z"},"outcome":"stopped"}`), &ack)
	if err := NewClient(server.URL).AckTaskCancelled(context.Background(), "task", ack); err != nil {
		t.Fatal(err)
	}
	got := <-received
	if got["operation_id"] == nil || got["execution_fence"] == nil || got["outcome"] != "stopped" {
		t.Fatal("cancel acknowledgement discarded explicit process proof")
	}
}

func TestCancellationReceiptRequiresExitEvidenceAndPersistsBeforeDelivery(t *testing.T) {
	d, task, serverCalls := cancellationFixture(t)
	ctx := d.withCancellationExecution(context.Background(), task)
	metadata := TaskCancellationMetadata{OperationID: "33333333-3333-4333-8333-333333333333", BindingEpoch: "7", ExecutionFence: *task.ExecutionFence, AckDeadline: "2026-10-02T00:10:00Z"}
	d.observeTaskCancellation(ctx, TaskStatus{Status: "cancelled", Cancellation: &metadata})
	markCancellationRunner(ctx)
	finish := beginCancellationProcess(ctx)
	// A runner returning without confirmed process exit is not a stopped receipt.
	finish(false)
	if err := d.acknowledgeCancellation(ctx, task.ID, TaskCancelAck{}); err != nil {
		t.Fatal(err)
	}
	for _, body := range serverCalls.snapshot() {
		if body["outcome"] == "stopped" {
			t.Fatal("unproven process exit was confirmed")
		}
	}
	records, err := d.cancelReceipts.records()
	if err != nil || len(records) != 1 || records[0].Ack.Outcome != "" {
		t.Fatalf("observation was not durably retained: %v", err)
	}
	// Restart cannot upgrade the lost execution to stopped.
	d.cancelReceipts.bootID = "44444444-4444-4446-8999-444444444444"
	d.replayCancellationReceipts(context.Background())
	if serverCalls.snapshot()[len(serverCalls.snapshot())-1]["outcome"] != "not_observed" {
		t.Fatal("restart inferred process stopped")
	}
}

type cancellationCallLog struct {
	mu        sync.Mutex
	calls     []map[string]any
	reject    atomic.Bool
	dropReply atomic.Bool
	state     atomic.Pointer[TaskStatus]
}

func (l *cancellationCallLog) snapshot() []map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]map[string]any(nil), l.calls...)
}

func cancellationFixture(t *testing.T) (*Daemon, Task, *cancellationCallLog) {
	t.Helper()
	identity, handoff := managedHandoffFixture(t)
	home := filepath.Dir(filepath.Dir(filepath.Dir(identity.directory)))
	log := &cancellationCallLog{}
	var d *Daemon
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/status") {
			value := log.state.Load()
			if value == nil {
				value = &TaskStatus{Status: "running"}
			}
			_ = json.NewEncoder(w).Encode(value)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/cancel-ack") {
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.Header.Get("Authorization") != "Bearer mdt_receipt_fixture" {
			t.Error("receipt escaped scoped mdt transport")
		}
		if body["outcome"] != nil {
			records, err := d.cancelReceipts.records()
			if err != nil || len(records) == 0 {
				t.Error("receipt delivered before durable write")
			}
		}
		log.mu.Lock()
		log.calls = append(log.calls, body)
		log.mu.Unlock()
		if log.reject.Load() {
			w.WriteHeader(503)
			return
		}
		if log.dropReply.Load() {
			panic(http.ErrAbortHandler)
		}
		w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(server.Close)
	profile := "cancel-test"
	d = New(Config{Profile: profile, ServerBaseURL: server.URL, ManagementDeploymentID: handoff.DeploymentID, WorkspacesRoot: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.managed = &managedDaemonLifecycle{homeDirectory: home, identity: identity, store: &ManagedCredentialStore{Version: "1", Profile: profile, ServerURL: server.URL, DeploymentID: handoff.DeploymentID, OrganizationID: handoff.OrganizationID, InstallationID: handoff.InstallationID, UserID: handoff.UserID, AuthVersion: handoff.AuthVersion, ManagedDaemonID: handoff.ManagedDaemonID}}
	d.client.SetToken("mul_never_for_receipts")
	d.client.enableManagedTransport()
	workspace := handoff.Bindings[0].WorkspaceID
	credential := ManagedDaemonCredential{BindingID: "99999999-9999-4999-8999-999999999999", BindingEpoch: "7", CapabilityVersion: "1", DaemonToken: "mdt_receipt_fixture", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), PrincipalUserID: handoff.UserID, AuthVersion: handoff.AuthVersion}
	if err := d.client.installManagedCredential(workspace, credential); err != nil {
		t.Fatal(err)
	}
	runtimeID := "22222222-2222-4222-8222-222222222222"
	if err := d.client.recordManagedRegistration(workspace, []Runtime{{ID: runtimeID}}); err != nil {
		t.Fatal(err)
	}
	task := Task{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", RuntimeID: runtimeID, WorkspaceID: workspace, ExecutionFence: &ExecutionFence{RuntimeID: runtimeID, DispatchedAt: "2026-10-02T00:00:00.123456Z"}}
	if err := d.client.recordManagedClaims(workspace, []*Task{&task}); err != nil {
		t.Fatal(err)
	}
	if err := d.initializeCancellationReceipts(); err != nil {
		t.Fatal(err)
	}
	return d, task, log
}
func observedCancellation(t *testing.T, d *Daemon, task Task) context.Context {
	t.Helper()
	ctx := d.withCancellationExecution(context.Background(), task)
	if cancellationExecutionFrom(ctx) == nil {
		t.Fatal("fixture has no verified cancellation scope")
	}
	d.observeTaskCancellation(ctx, TaskStatus{Status: "cancelled", Cancellation: &TaskCancellationMetadata{OperationID: "33333333-3333-4333-8333-333333333333", BindingEpoch: "7", ExecutionFence: *task.ExecutionFence, AckDeadline: "2026-10-02T00:10:00Z"}})
	return ctx
}

func TestCancellationStoppedReceiptRetriesAfterRestartWithoutClaimMap(t *testing.T) {
	d, task, log := cancellationFixture(t)
	ctx := observedCancellation(t, d, task)
	markCancellationRunner(ctx)
	finish := beginCancellationProcess(ctx)
	finish(true)
	log.reject.Store(true)
	if err := d.acknowledgeCancellation(ctx, task.ID, TaskCancelAck{BranchName: "kept-work"}); err == nil {
		t.Fatal("fixture did not lose acknowledgement")
	}
	records, err := d.cancelReceipts.records()
	if err != nil || len(records) != 1 || records[0].Ack.Outcome != "stopped" {
		t.Fatal("lost acknowledgement did not retain completion proof")
	}
	info, err := os.Stat(filepath.Join(d.cancelReceipts.directory, receiptName(records[0])))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("receipt is not private")
	}
	raw, _ := os.ReadFile(filepath.Join(d.cancelReceipts.directory, receiptName(records[0])))
	if bytes.Contains(raw, []byte("mdt_")) || bytes.Contains(raw, []byte("mul_")) {
		t.Fatal("receipt persisted a credential")
	}
	d.cancelReceipts.bootID = "44444444-4444-4446-8999-444444444444"
	d.client.managed.mu.Lock()
	d.client.managed.tasks = map[string]string{}
	d.client.managed.mu.Unlock()
	log.reject.Store(false)
	d.replayCancellationReceipts(context.Background())
	calls := log.snapshot()
	if len(calls) != 2 || calls[1]["outcome"] != "stopped" || calls[1]["branch_name"] != "kept-work" {
		t.Fatal("restart did not replay the original exact receipt")
	}
	records, err = d.cancelReceipts.records()
	if err != nil || len(records) != 0 {
		t.Fatal("accepted receipt was not cleared")
	}
}

func TestCancellationReceiptQuarantinesChangedScope(t *testing.T) {
	for _, change := range []string{"installation", "user", "auth_version", "binding", "epoch"} {
		t.Run(change, func(t *testing.T) {
			d, task, log := cancellationFixture(t)
			ctx := observedCancellation(t, d, task)
			markCancellationRunner(ctx)
			beginCancellationProcess(ctx)(true)
			log.reject.Store(true)
			_ = d.acknowledgeCancellation(ctx, task.ID, TaskCancelAck{})
			switch change {
			case "installation":
				d.managed.store.InstallationID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
			case "user":
				d.managed.store.UserID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
			case "auth_version":
				d.managed.store.AuthVersion = "8"
			case "binding":
				d.client.managed.workspaces[task.WorkspaceID].credential.BindingID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
			case "epoch":
				d.client.managed.workspaces[task.WorkspaceID].credential.BindingEpoch = "8"
			}
			log.reject.Store(false)
			d.replayCancellationReceipts(context.Background())
			if len(log.snapshot()) != 1 {
				t.Fatal("receipt was retargeted after scope change")
			}
			records, err := d.cancelReceipts.records()
			if err != nil || len(records) != 1 {
				t.Fatal("quarantined receipt was deleted")
			}
		})
	}
}

func TestCancellationObservationRejectsWrongFenceAndNonCancellation(t *testing.T) {
	for _, change := range []string{"runtime", "dispatch", "epoch", "completed", "failed"} {
		t.Run(change, func(t *testing.T) {
			d, task, _ := cancellationFixture(t)
			ctx := d.withCancellationExecution(context.Background(), task)
			state := TaskStatus{Status: "cancelled", Cancellation: &TaskCancellationMetadata{OperationID: "33333333-3333-4333-8333-333333333333", BindingEpoch: "7", ExecutionFence: *task.ExecutionFence, AckDeadline: "2026-10-02T00:10:00Z"}}
			switch change {
			case "runtime":
				state.Cancellation.ExecutionFence.RuntimeID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
			case "dispatch":
				state.Cancellation.ExecutionFence.DispatchedAt = "2026-10-02T00:00:01Z"
			case "epoch":
				state.Cancellation.BindingEpoch = "8"
			default:
				state.Status = change
			}
			d.observeTaskCancellation(ctx, state)
			records, err := d.cancelReceipts.records()
			if err != nil || len(records) != 0 {
				t.Fatal("unrelated state became cancellation evidence")
			}
		})
	}
}

func TestCancellationProcessWaitDoesNotInferExit(t *testing.T) {
	for _, proof := range []string{"closed", "timeout", "result_without_exit", "exited"} {
		t.Run(proof, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), cancelExecutionKey{}, &cancellationExecution{})
			finish := beginCancellationProcess(ctx)
			results := make(chan agent.Result, 1)
			switch proof {
			case "closed":
				close(results)
			case "result_without_exit":
				results <- agent.Result{}
			case "exited":
				results <- agent.Result{ProcessExited: true}
			}
			awaitCancellationProcessResult(ctx, results, time.Millisecond, finish)
			execution := cancellationExecutionFrom(ctx)
			if (execution.exited == 1) != (proof == "exited") {
				t.Fatal("process exit inference did not require explicit evidence")
			}
		})
	}
}

func TestCancellationStateIgnoresMalformedMetadataWithoutLosingStopSignal(t *testing.T) {
	for _, payload := range []string{`{"status":"cancelled","cancellation":{"operation_id":"bad"}}`, `{"status":"cancelled"}`, `{"status":"completed","cancellation":{"operation_id":"bad"}}`} {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(payload)) }))
			defer server.Close()
			state, err := NewClient(server.URL).GetTaskState(context.Background(), "task")
			if err != nil || !shouldInterruptAgent(state.Status, err) || state.Cancellation != nil {
				t.Fatal("invalid proof prevented ordinary cancellation or became evidence")
			}
		})
	}
}

func TestCancellationAuthenticationFailureStopsWithoutReceipt(t *testing.T) {
	for _, status := range []int{401, 404} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			d, task, _ := cancellationFixture(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				w.Write([]byte(`{"error":"task not found"}`))
			}))
			defer server.Close()
			d.client.managed.workspaces[task.WorkspaceID].client.baseURL = server.URL
			ctx, cancel := context.WithCancel(d.withCancellationExecution(context.Background(), task))
			defer cancel()
			select {
			case <-d.watchTaskCancellation(ctx, task.ID, time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil))):
			case <-time.After(time.Second):
				t.Fatal("terminal access failure did not stop task")
			}
			records, err := d.cancelReceipts.records()
			if err != nil || len(records) != 0 {
				t.Fatal("access failure manufactured cancellation receipt")
			}
		})
	}
}

func TestCancellationReceiptRejectsCorruptionAndSymlinks(t *testing.T) {
	d, task, _ := cancellationFixture(t)
	_ = observedCancellation(t, d, task)
	records, _ := d.cancelReceipts.records()
	path := filepath.Join(d.cancelReceipts.directory, receiptName(records[0]))
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.cancelReceipts.records(); err == nil {
		t.Fatal("corrupt receipt accepted")
	}
	if runtime.GOOS == "windows" {
		t.Skip("symlink privilege is verified in native Windows acceptance")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(target, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if _, err := d.cancelReceipts.records(); err == nil {
		t.Fatal("receipt symlink accepted")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "unchanged" {
		t.Fatal("receipt touched linked target")
	}
}

func TestManagedCancellationHandleTaskWaitsForExplicitProcessEvidence(t *testing.T) {
	for _, exited := range []bool{false, true} {
		t.Run(strconv.FormatBool(exited), func(t *testing.T) {
			d, task, log := cancellationFixture(t)
			d.runtimeIndex[task.RuntimeID] = Runtime{ID: task.RuntimeID, Provider: "test"}
			d.cancelPollInterval = time.Millisecond
			messages := make(chan agent.Message)
			results := make(chan agent.Result, 1)
			backend := sessionBackend{session: &agent.Session{Messages: messages, Result: results}}
			started := make(chan struct{})
			runnerFinished := make(chan struct{})
			finished := make(chan struct{})
			d.runner = taskRunnerFunc(func(ctx context.Context, task Task, _ string, _ int, logger *slog.Logger) (TaskResult, error) {
				markCancellationRunner(ctx)
				close(started)
				result, _, err := d.executeAndDrain(ctx, backend, "", agent.ExecOptions{}, logger, task.ID, "", &atomic.Int32{})
				close(runnerFinished)
				return TaskResult{Status: result.Status, BranchName: "preserved"}, err
			})
			go func() { d.handleTask(context.Background(), task, 0); close(finished) }()
			<-started
			log.state.Store(&TaskStatus{Status: "cancelled", Cancellation: &TaskCancellationMetadata{OperationID: "33333333-3333-4333-8333-333333333333", BindingEpoch: "7", ExecutionFence: *task.ExecutionFence, AckDeadline: "2026-10-02T00:10:00Z"}})
			deadline := time.Now().Add(time.Second)
			for {
				records, _ := d.cancelReceipts.records()
				if len(records) > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("watcher did not persist cancellation observation")
				}
				time.Sleep(time.Millisecond)
			}
			if len(log.snapshot()) != 0 {
				t.Fatal("ack arrived before runner/process completion")
			}
			results <- agent.Result{Status: "aborted", ProcessExited: exited}
			close(results)
			close(messages)
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("cancelled task did not finish")
			}
			select {
			case <-runnerFinished:
			default:
				t.Fatal("ack bypassed runner completion")
			}
			calls := log.snapshot()
			if len(calls) != 1 {
				t.Fatalf("got %d acknowledgements", len(calls))
			}
			if (calls[0]["outcome"] == "stopped") != exited {
				t.Fatal("stop confirmation did not follow affirmative process evidence")
			}
		})
	}
}

func TestCancellationReceiptRejectsWrongTaskAtAcknowledgement(t *testing.T) {
	d, task, log := cancellationFixture(t)
	ctx := observedCancellation(t, d, task)
	markCancellationRunner(ctx)
	beginCancellationProcess(ctx)(true)
	if err := d.acknowledgeCancellation(ctx, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", TaskCancelAck{}); err == nil {
		t.Fatal("receipt crossed task identity")
	}
	if len(log.snapshot()) != 0 {
		t.Fatal("wrong task made an acknowledgement request")
	}
}

func TestCancellationReceiptDoesNotDowngradeDurableStoppedEvidence(t *testing.T) {
	d, task, log := cancellationFixture(t)
	ctx := observedCancellation(t, d, task)
	markCancellationRunner(ctx)
	beginCancellationProcess(ctx)(true)
	log.reject.Store(true)
	_ = d.acknowledgeCancellation(ctx, task.ID, TaskCancelAck{})
	records, _ := d.cancelReceipts.records()
	weaker := records[0]
	weaker.Ack.Outcome = "not_observed"
	if err := d.cancelReceipts.put(weaker); err != nil {
		t.Fatal(err)
	}
	records, _ = d.cancelReceipts.records()
	if len(records) != 1 || records[0].Ack.Outcome != "stopped" {
		t.Fatal("weaker receipt overwrote completion proof")
	}
}

func TestCancellationUnknownAcknowledgementReplaysSameReceipt(t *testing.T) {
	d, task, log := cancellationFixture(t)
	ctx := observedCancellation(t, d, task)
	markCancellationRunner(ctx)
	beginCancellationProcess(ctx)(true)
	log.dropReply.Store(true)
	if err := d.acknowledgeCancellation(ctx, task.ID, TaskCancelAck{BranchName: "durable-result"}); err == nil {
		t.Fatal("fixture did not drop acknowledgement after accepting bytes")
	}
	records, err := d.cancelReceipts.records()
	if err != nil || len(records) != 1 {
		t.Fatal("unknown acknowledgement lost its receipt")
	}
	log.dropReply.Store(false)
	d.replayCancellationReceipts(context.Background())
	calls := log.snapshot()
	if len(calls) != 2 {
		t.Fatalf("receipt request count %d", len(calls))
	}
	before, _ := json.Marshal(calls[0])
	after, _ := json.Marshal(calls[1])
	if !bytes.Equal(before, after) {
		t.Fatal("unknown commit generated a different operation/fence/body")
	}
	records, err = d.cancelReceipts.records()
	if err != nil || len(records) != 0 {
		t.Fatal("acknowledged replay was not cleared")
	}
}
