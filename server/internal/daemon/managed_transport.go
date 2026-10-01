package daemon

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ManagedDaemonCredential never leaves main/daemon or its protected local store.
type ManagedDaemonCredential struct {
	BindingID         string `json:"binding_id"`
	BindingEpoch      string `json:"binding_epoch"`
	CapabilityVersion string `json:"capability_version"`
	DaemonToken       string `json:"daemon_token"`
	ExpiresAt         string `json:"expires_at"`
	PrincipalUserID   string `json:"principal_user_id"`
	AuthVersion       string `json:"auth_version"`
}

func (c ManagedDaemonCredential) String() string   { return "ManagedDaemonCredential{token redacted}" }
func (c ManagedDaemonCredential) GoString() string { return c.String() }

type managedWorkspaceTransport struct {
	workspaceID      string
	client           *Client
	rpc              *wsRPCClient
	credential       ManagedDaemonCredential
	batchUnsupported atomic.Bool
	fallbackAfter    atomic.Int64
}

type managedClientRegistry struct {
	mu         sync.RWMutex
	workspaces map[string]*managedWorkspaceTransport
	runtimes   map[string]string
	tasks      map[string]string
	issues     map[string]string
	chats      map[string]string
	autopilots map[string]string
	nextGroup  int
}

type managedRuntimeGroup struct {
	workspace  *managedWorkspaceTransport
	runtimeIDs []string
}

func (c *Client) enableManagedTransport() {
	if c.managed != nil {
		return
	}
	c.managed = &managedClientRegistry{workspaces: make(map[string]*managedWorkspaceTransport), runtimes: make(map[string]string), tasks: make(map[string]string), issues: make(map[string]string), chats: make(map[string]string), autopilots: make(map[string]string)}
}

func (c *Client) installManagedCredential(workspaceID string, credential ManagedDaemonCredential) error {
	if c.managed == nil || !managedUUID.MatchString(workspaceID) || !managedUUID.MatchString(credential.BindingID) || !managedUUID.MatchString(credential.PrincipalUserID) || credential.CapabilityVersion != "1" || !strings.HasPrefix(credential.DaemonToken, "mdt_") {
		return errors.New("invalid managed daemon credential")
	}
	if _, ok := managedNumber(credential.BindingEpoch); !ok {
		return errors.New("invalid managed binding epoch")
	}
	if _, ok := managedNumber(credential.AuthVersion); !ok {
		return errors.New("invalid managed account version")
	}
	expires, err := time.Parse(time.RFC3339, credential.ExpiresAt)
	if err != nil || !expires.After(time.Now()) {
		return errors.New("managed credential expired")
	}
	registry := c.managed
	registry.mu.Lock()
	defer registry.mu.Unlock()
	current := registry.workspaces[workspaceID]
	if current == nil {
		child := NewClient(c.baseURL)
		child.managedWorkspaceID = workspaceID
		child.cancelAckReceiptsReady = c.cancelAckReceiptsReady
		child.platform = c.platform
		child.version = c.version
		child.os = c.os
		child.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		child.bundleClient.CheckRedirect = child.client.CheckRedirect
		current = &managedWorkspaceTransport{workspaceID: workspaceID, client: child, rpc: newWSRPCClient(wsRPCResponseGrace)}
		registry.workspaces[workspaceID] = current
	} else {
		previousEpoch, _ := managedNumber(current.credential.BindingEpoch)
		nextEpoch, _ := managedNumber(credential.BindingEpoch)
		if nextEpoch < previousEpoch || current.credential.PrincipalUserID != credential.PrincipalUserID || current.credential.AuthVersion != credential.AuthVersion {
			return errors.New("managed credential scope changed")
		}
	}
	current.credential = credential
	current.client.SetToken(credential.DaemonToken)
	return nil
}

func (r *managedClientRegistry) workspaceLocked(workspaceID string) (*managedWorkspaceTransport, error) {
	scoped := r.workspaces[workspaceID]
	if scoped == nil {
		return nil, errors.New("managed workspace has no verified binding")
	}
	expires, err := time.Parse(time.RFC3339, scoped.credential.ExpiresAt)
	if err != nil || !expires.After(time.Now()) {
		return nil, errors.New("managed workspace credential expired")
	}
	return scoped, nil
}

func (c *Client) managedRequestClient(path string, body any) (*Client, error) {
	if c.managed == nil {
		return c, nil
	}
	if path == "/api/daemon/workspaces" || path == "/api/workspaces" || path == "/api/tokens/current/renew" {
		return c, nil
	}
	registry := c.managed
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	workspaceID := ""
	segments := strings.Split(strings.TrimPrefix(path, "/api/daemon/"), "/")
	if len(segments) >= 2 {
		switch segments[0] {
		case "workspaces":
			workspaceID = segments[1]
		case "runtimes":
			workspaceID = registry.runtimes[segments[1]]
		case "tasks":
			workspaceID = registry.tasks[segments[1]]
		case "issues":
			workspaceID = registry.issues[segments[1]]
		case "chat-sessions":
			workspaceID = registry.chats[segments[1]]
		case "autopilot-runs":
			workspaceID = registry.autopilots[segments[1]]
		}
	}
	parameters, _ := body.(map[string]any)
	switch path {
	case "/api/daemon/register":
		workspaceID, _ = parameters["workspace_id"].(string)
	case "/api/daemon/heartbeat":
		runtimeID, _ := parameters["runtime_id"].(string)
		workspaceID = registry.runtimes[runtimeID]
	}
	scoped, err := registry.workspaceLocked(workspaceID)
	if err != nil {
		return nil, err
	}
	return scoped.client, nil
}

func (c *Client) recordManagedRegistration(workspaceID string, runtimes []Runtime) error {
	if c.managed == nil {
		return nil
	}
	registry := c.managed
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, err := registry.workspaceLocked(workspaceID); err != nil {
		return err
	}
	for _, runtime := range runtimes {
		if !managedUUID.MatchString(runtime.ID) || registry.runtimes[runtime.ID] != "" && registry.runtimes[runtime.ID] != workspaceID {
			return errors.New("registration returned a runtime outside the managed workspace")
		}
	}
	for _, runtime := range runtimes {
		registry.runtimes[runtime.ID] = workspaceID
	}
	return nil
}

func (c *Client) recordManagedClaims(workspaceID string, tasks []*Task) error {
	if c.managed == nil {
		return nil
	}
	registry := c.managed
	registry.mu.Lock()
	defer registry.mu.Unlock()
	seen := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		if task == nil || seen[task.ID] || !managedUUID.MatchString(task.ID) || task.WorkspaceID != workspaceID || registry.runtimes[task.RuntimeID] != workspaceID || registry.tasks[task.ID] != "" && registry.tasks[task.ID] != workspaceID {
			return errors.New("claim returned a task outside the managed workspace")
		}
		seen[task.ID] = true
	}
	for _, task := range tasks {
		registry.tasks[task.ID] = workspaceID
		if task.IssueID != "" {
			registry.issues[task.IssueID] = workspaceID
		}
		if task.ChatSessionID != "" {
			registry.chats[task.ChatSessionID] = workspaceID
		}
		if task.AutopilotRunID != "" {
			registry.autopilots[task.AutopilotRunID] = workspaceID
		}
	}
	return nil
}

func (c *Client) managedRuntimeGroups(runtimeIDs []string, rotate bool) ([]managedRuntimeGroup, error) {
	registry := c.managed
	if registry == nil {
		return nil, errors.New("managed transport is not enabled")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	byWorkspace := make(map[string][]string)
	for _, id := range runtimeIDs {
		workspace := registry.runtimes[id]
		if workspace == "" {
			return nil, errors.New("unknown managed runtime")
		}
		byWorkspace[workspace] = append(byWorkspace[workspace], id)
	}
	ids := make([]string, 0, len(byWorkspace))
	for id := range byWorkspace {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if rotate && len(ids) > 0 {
		start := registry.nextGroup % len(ids)
		registry.nextGroup++
		ids = append(ids[start:], ids[:start]...)
	}
	groups := make([]managedRuntimeGroup, 0, len(ids))
	for _, id := range ids {
		scoped, err := registry.workspaceLocked(id)
		if err != nil {
			return nil, err
		}
		groups = append(groups, managedRuntimeGroup{scoped, byWorkspace[id]})
	}
	return groups, nil
}

func (c *Client) claimManagedTasks(ctx context.Context, daemonID string, runtimeIDs []string, maxTasks int) ([]*Task, error) {
	groups, err := c.managedRuntimeGroups(runtimeIDs, true)
	if err != nil {
		return nil, err
	}
	tasks := make([]*Task, 0)
	for _, group := range groups {
		if len(tasks) >= maxTasks {
			break
		}
		claimed, err := group.workspace.client.ClaimTasks(ctx, daemonID, group.runtimeIDs, maxTasks-len(tasks))
		if err != nil {
			if len(tasks) > 0 {
				return tasks, nil
			}
			return nil, err
		}
		if len(claimed) > maxTasks-len(tasks) {
			if len(tasks) > 0 {
				return tasks, nil
			}
			return nil, errors.New("managed claim exceeded requested capacity")
		}
		if err = c.recordManagedClaims(group.workspace.workspaceID, claimed); err != nil {
			if len(tasks) > 0 {
				return tasks, nil
			}
			return nil, err
		}
		tasks = append(tasks, claimed...)
	}
	return tasks, nil
}

func (c *Client) deregisterManagedRuntimes(ctx context.Context, runtimeIDs []string, reasons map[string]RuntimeOfflineReason) error {
	groups, err := c.managedRuntimeGroups(runtimeIDs, false)
	if err != nil {
		return err
	}
	var result error
	for _, group := range groups {
		subset := make(map[string]RuntimeOfflineReason)
		for _, id := range group.runtimeIDs {
			if reason, ok := reasons[id]; ok {
				subset[id] = reason
			}
		}
		result = errors.Join(result, group.workspace.client.Deregister(ctx, group.runtimeIDs, subset))
	}
	return result
}

// Replay uses the stored scope and a currently verified binding, never the
// process-local task map or the discovery PAT. An old scope remains quarantined.
func (d *Daemon) cancellationReceiptTransport(scope cancelReceiptScope) (*Client, string, error) {
	if d.managed == nil || d.client.managed == nil {
		return nil, "", errors.New("managed cancellation transport unavailable")
	}
	d.managed.mu.Lock()
	store := *d.managed.store
	d.managed.mu.Unlock()
	if scope.ServerURL != store.ServerURL || scope.DeploymentID != store.DeploymentID || scope.InstallationID != store.InstallationID || scope.UserID != store.UserID || scope.AuthVersion != store.AuthVersion || scope.Profile != store.Profile {
		return nil, "", errors.New("cancellation receipt scope is quarantined")
	}
	registry := d.client.managed
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	workspace, err := registry.workspaceLocked(scope.WorkspaceID)
	if err != nil {
		return nil, "", err
	}
	credential := workspace.credential
	if credential.BindingID != scope.BindingID || credential.BindingEpoch != scope.BindingEpoch || credential.PrincipalUserID != scope.UserID || credential.AuthVersion != scope.AuthVersion {
		return nil, "", errors.New("cancellation receipt binding is quarantined")
	}
	return workspace.client, credential.DaemonToken, nil
}
