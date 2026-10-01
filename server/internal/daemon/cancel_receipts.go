package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
)

const maxCancelReceiptBytes = 64 * 1024
const maxCancelReceipts = 1000

type cancelReceiptScope struct {
	ServerURL      string         `json:"server_url"`
	DeploymentID   string         `json:"deployment_id"`
	InstallationID string         `json:"installation_id"`
	UserID         string         `json:"user_id"`
	AuthVersion    string         `json:"auth_version"`
	Profile        string         `json:"profile"`
	WorkspaceID    string         `json:"workspace_id"`
	BindingID      string         `json:"binding_id"`
	BindingEpoch   string         `json:"binding_epoch"`
	TaskID         string         `json:"task_id"`
	ExecutionFence ExecutionFence `json:"execution_fence"`
}

type cancellationReceipt struct {
	Version int                `json:"version"`
	Scope   cancelReceiptScope `json:"scope"`
	BootID  string             `json:"boot_id"`
	Ack     TaskCancelAck      `json:"ack"`
}

type cancellationOutbox struct {
	directory string
	bootID    string
	replayMu  sync.Mutex
	next      int
}

func validExecutionFence(fence ExecutionFence) bool {
	if !managedUUID.MatchString(fence.RuntimeID) {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, fence.DispatchedAt)
	return err == nil
}
func validCancellationMetadata(metadata TaskCancellationMetadata) bool {
	_, epoch := managedNumber(metadata.BindingEpoch)
	_, deadline := time.Parse(time.RFC3339Nano, metadata.AckDeadline)
	return managedUUID.MatchString(metadata.OperationID) && epoch && validExecutionFence(metadata.ExecutionFence) && deadline == nil
}
func validReceipt(receipt cancellationReceipt) bool {
	s := receipt.Scope
	for _, id := range []string{s.DeploymentID, s.InstallationID, s.UserID, s.WorkspaceID, s.BindingID, s.TaskID, receipt.BootID, receipt.Ack.OperationID} {
		if !managedUUID.MatchString(id) {
			return false
		}
	}
	_, epoch := managedNumber(s.BindingEpoch)
	_, version := managedNumber(s.AuthVersion)
	server, err := canonicalManagedServer(s.ServerURL)
	return receipt.Version == 1 && err == nil && server == s.ServerURL && s.Profile != "" && !strings.ContainsAny(s.Profile, "/\\") && epoch && version && validExecutionFence(s.ExecutionFence) && receipt.Ack.BindingEpoch == s.BindingEpoch && receipt.Ack.ExecutionFence != nil && *receipt.Ack.ExecutionFence == s.ExecutionFence && (receipt.Ack.Outcome == "" || receipt.Ack.Outcome == "stopped" || receipt.Ack.Outcome == "not_observed")
}
func receiptName(receipt cancellationReceipt) string {
	raw, _ := json.Marshal(struct {
		Scope       cancelReceiptScope
		OperationID string
	}{receipt.Scope, receipt.Ack.OperationID})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]) + ".json"
}
func (o *cancellationOutbox) read(name string) (cancellationReceipt, error) {
	var receipt cancellationReceipt
	if len(name) != 69 || strings.TrimSuffix(name, ".json") == name {
		return receipt, errors.New("invalid cancellation receipt name")
	}
	path := filepath.Join(o.directory, name)
	info, err := os.Lstat(path)
	if err != nil {
		return receipt, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxCancelReceiptBytes {
		return receipt, errors.New("invalid cancellation receipt file")
	}
	if err := managedCheckPermissions(info, true); err != nil {
		return receipt, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return receipt, err
	}
	if managedCanonicalJSON(bytes.TrimSpace(raw), &receipt) != nil || !validReceipt(receipt) || receiptName(receipt) != name {
		return receipt, errors.New("invalid cancellation receipt scope")
	}
	return receipt, nil
}
func (o *cancellationOutbox) records() ([]cancellationReceipt, error) {
	entries, err := os.ReadDir(o.directory)
	if err != nil {
		return nil, err
	}
	if len(entries) > maxCancelReceipts+1 {
		return nil, errors.New("cancellation receipt capacity exceeded")
	}
	result := make([]cancellationReceipt, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		receipt, err := o.read(entry.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, receipt)
	}
	return result, nil
}
func (o *cancellationOutbox) put(receipt cancellationReceipt) error {
	if !validReceipt(receipt) {
		return errors.New("invalid cancellation receipt")
	}
	raw, _ := json.Marshal(receipt)
	if len(raw) > maxCancelReceiptBytes {
		return errors.New("cancellation receipt exceeds size limit")
	}
	return withManagementFileLock(o.directory, func() error {
		name := receiptName(receipt)
		current, err := o.read(name)
		if err == nil {
			// Never replace durable completion with an earlier observation or weaker evidence.
			if current.Ack.Outcome == "stopped" || receipt.Ack.Outcome == "" {
				return nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if errors.Is(err, os.ErrNotExist) {
			records, err := o.records()
			if err != nil {
				return err
			}
			if len(records) >= maxCancelReceipts {
				return errors.New("cancellation receipt capacity exceeded")
			}
		}
		return writeManagementJSON(filepath.Join(o.directory, name), receipt)
	})
}
func (o *cancellationOutbox) remove(receipt cancellationReceipt) error {
	return withManagementFileLock(o.directory, func() error {
		current, err := o.read(receiptName(receipt))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		before, _ := json.Marshal(receipt)
		after, _ := json.Marshal(current)
		if !bytes.Equal(before, after) {
			return nil
		}
		return os.Remove(filepath.Join(o.directory, receiptName(receipt)))
	})
}
func (d *Daemon) initializeCancellationReceipts() error {
	if d.managed == nil {
		return nil
	}
	state := d.managed
	state.mu.Lock()
	home := state.homeDirectory
	scope := *state.store
	state.mu.Unlock()
	root := filepath.Join(home, ".multica", "management", scope.DeploymentID)
	hash := sha256.Sum256([]byte(scope.Profile))
	paths := []string{filepath.Join(root, "cancellation-receipts"), filepath.Join(root, "cancellation-receipts", scope.UserID), filepath.Join(root, "cancellation-receipts", scope.UserID, hex.EncodeToString(hash[:]))}
	for _, path := range paths {
		if err := managedEnsureDirectory(path, true); err != nil {
			return err
		}
	}
	d.cancelReceipts = &cancellationOutbox{directory: paths[len(paths)-1], bootID: uuid.NewString()}
	if _, err := d.cancelReceipts.records(); err != nil {
		return err
	}
	registry := d.client.managed
	registry.mu.Lock()
	defer registry.mu.Unlock()
	d.client.cancelAckReceiptsReady = true
	for _, workspace := range registry.workspaces {
		workspace.client.cancelAckReceiptsReady = true
	}
	return nil
}

// Cancellation execution facts are private to the exact claimed execution.
// A status/401/404 signal can stop work, but cannot manufacture these facts.
type cancelExecutionKey struct{}
type cancellationExecution struct {
	mu               sync.Mutex
	scope            cancelReceiptScope
	metadata         *TaskCancellationMetadata
	productionRunner bool
	attempts         int
	exited           int
}

func cancellationExecutionFrom(ctx context.Context) *cancellationExecution {
	value, _ := ctx.Value(cancelExecutionKey{}).(*cancellationExecution)
	return value
}
func (d *Daemon) withCancellationExecution(ctx context.Context, task Task) context.Context {
	if d.cancelReceipts == nil || d.managed == nil || task.ExecutionFence == nil || !validExecutionFence(*task.ExecutionFence) || task.ExecutionFence.RuntimeID != task.RuntimeID || !managedUUID.MatchString(task.ID) {
		return ctx
	}
	state := d.managed
	state.mu.Lock()
	store := *state.store
	state.mu.Unlock()
	registry := d.client.managed
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	workspace, err := registry.workspaceLocked(task.WorkspaceID)
	if err != nil || registry.tasks[task.ID] != task.WorkspaceID || registry.runtimes[task.RuntimeID] != task.WorkspaceID {
		return ctx
	}
	credential := workspace.credential
	if credential.PrincipalUserID != store.UserID || credential.AuthVersion != store.AuthVersion {
		return ctx
	}
	scope := cancelReceiptScope{ServerURL: store.ServerURL, DeploymentID: store.DeploymentID, InstallationID: store.InstallationID, UserID: store.UserID, AuthVersion: store.AuthVersion, Profile: store.Profile, WorkspaceID: task.WorkspaceID, BindingID: credential.BindingID, BindingEpoch: credential.BindingEpoch, TaskID: task.ID, ExecutionFence: *task.ExecutionFence}
	return context.WithValue(ctx, cancelExecutionKey{}, &cancellationExecution{scope: scope})
}
func markCancellationRunner(ctx context.Context) {
	if execution := cancellationExecutionFrom(ctx); execution != nil {
		execution.mu.Lock()
		execution.productionRunner = true
		execution.mu.Unlock()
	}
}
func beginCancellationProcess(ctx context.Context) func(bool) {
	execution := cancellationExecutionFrom(ctx)
	if execution == nil {
		return func(bool) {}
	}
	execution.mu.Lock()
	execution.attempts++
	execution.mu.Unlock()
	var once sync.Once
	return func(exited bool) {
		once.Do(func() {
			if exited {
				execution.mu.Lock()
				execution.exited++
				execution.mu.Unlock()
			}
		})
	}
}
func (d *Daemon) observeTaskCancellation(ctx context.Context, state TaskStatus) {
	execution := cancellationExecutionFrom(ctx)
	if execution == nil || state.Status != "cancelled" || state.Cancellation == nil {
		return
	}
	metadata := *state.Cancellation
	execution.mu.Lock()
	if !validCancellationMetadata(metadata) || metadata.BindingEpoch != execution.scope.BindingEpoch || metadata.ExecutionFence != execution.scope.ExecutionFence || (execution.metadata != nil && execution.metadata.OperationID != metadata.OperationID) {
		execution.mu.Unlock()
		return
	}
	execution.metadata = &metadata
	receipt := cancellationReceipt{Version: 1, Scope: execution.scope, BootID: d.cancelReceipts.bootID, Ack: TaskCancelAck{OperationID: metadata.OperationID, BindingEpoch: metadata.BindingEpoch, ExecutionFence: &metadata.ExecutionFence}}
	execution.mu.Unlock()
	if err := d.cancelReceipts.put(receipt); err != nil {
		d.logger.Warn("cancellation observation could not be persisted; process confirmation remains unavailable")
	}
}
func (d *Daemon) acknowledgeCancellation(ctx context.Context, taskID string, ack TaskCancelAck) error {
	execution := cancellationExecutionFrom(ctx)
	if execution == nil {
		return d.client.AckTaskCancelled(ctx, taskID, ack)
	}
	execution.mu.Lock()
	metadata := execution.metadata
	confirmed := execution.productionRunner && execution.attempts == execution.exited
	scope := execution.scope
	execution.mu.Unlock()
	if scope.TaskID != taskID {
		return errors.New("cancellation receipt task scope changed")
	}
	if metadata == nil || !confirmed {
		return d.client.AckTaskCancelled(ctx, taskID, ack)
	}
	ack.OperationID = metadata.OperationID
	ack.BindingEpoch = metadata.BindingEpoch
	fence := metadata.ExecutionFence
	ack.ExecutionFence = &fence
	ack.Outcome = "stopped"
	receipt := cancellationReceipt{Version: 1, Scope: scope, BootID: d.cancelReceipts.bootID, Ack: ack}
	if err := d.cancelReceipts.put(receipt); err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return d.deliverCancellationReceipt(sendCtx, receipt)
}
func (d *Daemon) deliverCancellationReceipt(ctx context.Context, receipt cancellationReceipt) error {
	client, token, err := d.cancellationReceiptTransport(receipt.Scope)
	if err != nil {
		return err
	}
	var response struct {
		Status string `json:"status"`
	}
	if err := client.postJSONWithToken(ctx, fmt.Sprintf("/api/daemon/tasks/%s/cancel-ack", receipt.Scope.TaskID), token, receipt.Ack, &response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return errors.New("cancellation receipt was not acknowledged")
	}
	return d.cancelReceipts.remove(receipt)
}
func (d *Daemon) replayCancellationReceipts(ctx context.Context) {
	outbox := d.cancelReceipts
	if outbox == nil {
		return
	}
	outbox.replayMu.Lock()
	defer outbox.replayMu.Unlock()
	records, err := outbox.records()
	if err != nil {
		d.logger.Warn("cancellation receipt replay deferred: private records unavailable")
		return
	}
	if len(records) == 0 {
		return
	}
	start := outbox.next % len(records)
	sent := 0
	for n := 0; n < len(records) && sent < 32 && ctx.Err() == nil; n++ {
		index := (start + n) % len(records)
		outbox.next = (index + 1) % len(records)
		receipt := records[index]
		if _, _, err := d.cancellationReceiptTransport(receipt.Scope); err != nil {
			continue
		}
		if receipt.Ack.Outcome == "" {
			if receipt.BootID == outbox.bootID {
				continue
			}
			receipt.Ack.Outcome = "not_observed"
			if err := outbox.put(receipt); err != nil {
				continue
			}
			receipt, err = outbox.read(receiptName(receipt))
			if err != nil {
				continue
			}
		}
		sent++
		attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = d.deliverCancellationReceipt(attempt, receipt)
		cancel()
	}
}
func (d *Daemon) cancellationReceiptLoop(ctx context.Context) {
	if d.cancelReceipts == nil {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		round, cancel := context.WithTimeout(ctx, 30*time.Second)
		d.replayCancellationReceipts(round)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func awaitCancellationProcessResult(ctx context.Context, results <-chan agent.Result, wait time.Duration, finish func(bool)) {
	if cancellationExecutionFrom(ctx) == nil {
		return
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case result, ok := <-results:
		finish(ok && result.ProcessExited)
	case <-timer.C:
		finish(false)
	}
}
