package daemon

import (
	"context"
	"errors"
	"time"
)

func (d *Daemon) managedRuntimeBelongsToWorkspace(workspaceID, runtimeID string) bool {
	if workspaceID == "" {
		return true
	}
	if d.client.managed == nil || runtimeID == "" {
		return false
	}
	d.client.managed.mu.RLock()
	defer d.client.managed.mu.RUnlock()
	return d.client.managed.runtimes[runtimeID] == workspaceID
}

func (d *Daemon) clearManagedWSHeartbeatAcks(workspaceID string, runtimeIDs []string) {
	if workspaceID == "" {
		d.clearWSHeartbeatAcks()
		return
	}
	d.wsHBMu.Lock()
	defer d.wsHBMu.Unlock()
	for _, id := range runtimeIDs {
		delete(d.wsHBLastAck, id)
	}
}

func (d *Daemon) managedWorkspaceRuntimeIDs(workspaceID string) []string {
	all := d.allRuntimeIDs()
	result := make([]string, 0, len(all))
	for _, id := range all {
		if d.managedRuntimeBelongsToWorkspace(workspaceID, id) {
			result = append(result, id)
		}
	}
	return result
}

func (d *Daemon) managedTaskWakeupLoop(ctx context.Context, wakeups chan<- taskWakeup) {
	changes, unsubscribe := d.runtimeSet.Subscribe()
	defer unsubscribe()
	running := make(map[string]context.CancelFunc)
	defer func() {
		for _, cancel := range running {
			cancel()
		}
	}()
	for {
		registry := d.client.managed
		registry.mu.RLock()
		scoped := make([]*managedWorkspaceTransport, 0, len(registry.workspaces))
		for _, workspace := range registry.workspaces {
			scoped = append(scoped, workspace)
		}
		registry.mu.RUnlock()
		for _, workspace := range scoped {
			if running[workspace.workspaceID] != nil {
				continue
			}
			childCtx, cancel := context.WithCancel(ctx)
			running[workspace.workspaceID] = cancel
			go d.managedWorkspaceWakeupLoop(childCtx, workspace, wakeups)
		}
		select {
		case <-ctx.Done():
			return
		case <-changes:
		}
	}
}

func (d *Daemon) managedWorkspaceWakeupLoop(ctx context.Context, workspace *managedWorkspaceTransport, wakeups chan<- taskWakeup) {
	changes, unsubscribe := d.runtimeSet.Subscribe()
	defer unsubscribe()
	backoff := time.Second
	for {
		runtimes := d.managedWorkspaceRuntimeIDs(workspace.workspaceID)
		if len(runtimes) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-changes:
				continue
			}
		}
		connectedFor, err := d.runTaskWakeupConnectionUsingTransport(ctx, workspace.client, workspace.rpc, &workspace.batchUnsupported, workspace.workspaceID, runtimes, wakeups, changes)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errRuntimeSetChanged) {
			backoff = time.Second
			continue
		}
		if shouldResetTaskWakeupBackoff(connectedFor) {
			backoff = time.Second
		}
		if err != nil {
			d.logger.Debug("managed workspace websocket unavailable; scoped HTTP polling remains active", "workspace_id", workspace.workspaceID, "retry_in", backoff)
		}
		if err = sleepWithContextOrRuntimeChange(ctx, jitterDuration(backoff), changes); err != nil {
			return
		}
		backoff *= 2
		if backoff > taskWakeupMaxBackoff {
			backoff = taskWakeupMaxBackoff
		}
	}
}

func (d *Daemon) claimManagedTasksWSFirst(ctx context.Context, daemonID string, runtimeIDs []string, maxTasks int) ([]*Task, error) {
	if maxTasks <= 0 {
		return nil, nil
	}
	groups, err := d.client.managedRuntimeGroups(runtimeIDs, true)
	if err != nil {
		return nil, err
	}
	tasks := make([]*Task, 0)
	for _, group := range groups {
		if len(tasks) >= maxTasks {
			break
		}
		workspace := group.workspace
		claimed, err := d.claimTasksUsingTransport(ctx, workspace.client, workspace.rpc, &workspace.batchUnsupported, &workspace.fallbackAfter, daemonID, group.runtimeIDs, maxTasks-len(tasks))
		if err != nil {
			if len(tasks) > 0 {
				return tasks, nil
			}
			return nil, err
		}
		if len(claimed) > maxTasks-len(tasks) {
			return tasks, errors.New("managed WS claim exceeded requested capacity")
		}
		if err = d.client.recordManagedClaims(workspace.workspaceID, claimed); err != nil {
			if len(tasks) > 0 {
				return tasks, nil
			}
			return nil, err
		}
		tasks = append(tasks, claimed...)
	}
	return tasks, nil
}
