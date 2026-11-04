//go:build agentintegration && !windows

package daemon

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
)

// The bridge is excluded from production builds. It drives the same ordinary
// claim/run/report code as the daemon without registration, discovery, GC,
// account resolution or background execution loops.
type RK1KnowledgeDriverOptions struct {
	Config     Config
	Token      string
	RuntimeID  string
	CLIPath    string
	HTTPClient *http.Client
}

type RK1KnowledgeDriver struct {
	d         *Daemon
	runtimeID string
	close     func()
}

var rk1KnowledgeDriverMutex sync.Mutex

func NewRK1KnowledgeDriver(options RK1KnowledgeDriverOptions, logger *slog.Logger) (*RK1KnowledgeDriver, error) {
	entry, ok := options.Config.Agents["claude"]
	if !ok || !filepath.IsAbs(entry.Path) || entry.Command != "" ||
		!filepath.IsAbs(options.CLIPath) || options.Config.ServerBaseURL == "" ||
		options.Config.DaemonID == "" || options.Token == "" || options.RuntimeID == "" ||
		!filepath.IsAbs(options.Config.WorkspacesRoot) {
		return nil, errors.New("explicit knowledge test driver configuration required")
	}
	rk1KnowledgeDriverMutex.Lock()
	prior := resolveSelfExecutable
	resolveSelfExecutable = func() (string, error) { return options.CLIPath, nil }
	d := New(options.Config, logger)
	// This is the existing package test seam for the in-process preparer.
	d.executionEnvironmentCommand = nil
	d.client.SetToken(options.Token)
	if options.HTTPClient != nil {
		d.client.client = options.HTTPClient
		d.client.bundleClient = options.HTTPClient
	}
	d.runtimeIndex[options.RuntimeID] = Runtime{ID: options.RuntimeID, Provider: "claude"}
	var once sync.Once
	return &RK1KnowledgeDriver{d: d, runtimeID: options.RuntimeID, close: func() {
		once.Do(func() {
			resolveSelfExecutable = prior
			rk1KnowledgeDriverMutex.Unlock()
		})
	}}, nil
}

func (x *RK1KnowledgeDriver) Close() { x.close() }

func (x *RK1KnowledgeDriver) Claim(ctx context.Context) (*Task, error) {
	return x.d.client.ClaimTask(ctx, x.runtimeID)
}

func (x *RK1KnowledgeDriver) Run(ctx context.Context, task Task) (TaskResult, error) {
	return x.d.runTask(ctx, task, "claude", 0, x.d.logger)
}

func (x *RK1KnowledgeDriver) Report(ctx context.Context, task Task, result TaskResult) {
	x.d.reportTaskResult(ctx, task.ID, result, x.d.logger)
}

func (x *RK1KnowledgeDriver) Execute(ctx context.Context, task Task) (TaskResult, error) {
	result, err := x.Run(ctx, task)
	if err == nil {
		x.Report(ctx, task, result)
	}
	return result, err
}
