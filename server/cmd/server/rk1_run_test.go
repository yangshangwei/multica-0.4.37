//go:build agentintegration && !windows

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
)

func (f *rk1Fixture) control(stop bool) {
	t := f.t
	t.Helper()
	session, err := f.q.GetCollaborationSession(t.Context(), util.MustParseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	targets, err := f.q.ListCollaborationTargets(t.Context(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	p := service.CollaborationStopQueueParams{CollaborationQueueRequest: f.queue(), Scope: collaboration.ControlIssue,
		ExpectedControlVersion: session.ControlVersion, ExpectedTargetSetVersion: session.TargetSetVersion}
	for _, target := range targets {
		control := service.CollaborationTargetControl{TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
			ExpectedControlVersion: target.ControlVersion, ObservedGroupID: target.ActiveGroupID, ObservedGroupGeneration: target.GroupGeneration}
		if !stop {
			control.ExpectedHoldVersion = target.HoldVersion
		}
		p.Targets = append(p.Targets, control)
	}
	var receipt service.CollaborationQueueReceipt
	if stop {
		receipt, err = f.svc.StopQueue(f.owner, p)
	} else {
		gate, gateErr := f.q.GetCollaborationGate(t.Context())
		if gateErr != nil {
			t.Fatal(gateErr)
		}
		receipt, err = f.svc.ResumeQueue(f.owner, service.CollaborationResumeQueueParams{CollaborationStopQueueParams: p, ExpectedExecutionEpoch: gate.Epoch})
	}
	if err != nil || receipt.Status >= 400 {
		t.Fatalf("queue control stop=%v: %v %s", stop, err, receipt.Code)
	}
}

// nextRun waits briefly for exactly one pending run. Zero or several runs is a
// harness fault, never something to pick from.
func (f *rk1Fixture) nextRun() string {
	t := f.t
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		list, err := f.client.List(t.Context(), f.base.runtime)
		if err != nil {
			t.Fatalf("runtime list: %v", err)
		}
		if len(list.RunIDs) == 1 {
			return list.RunIDs[0]
		}
		if len(list.RunIDs) > 1 || time.Now().After(deadline) {
			t.Fatalf("expected one pending run, got %d", len(list.RunIDs))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// execute runs one admitted run through the real Runner/HostExecutor with a
// fresh ledger and working directory, so no earlier run's native session
// pointer can turn this launch into a resume.
func (f *rk1Fixture) execute(run string, plan rk1Plan, during func(ctx context.Context, run string) error) (error, error) {
	t := f.t
	t.Helper()
	if err := rk1WriteJSON(filepath.Join(f.plans, run+".json"), plan); err != nil {
		t.Fatal(err)
	}
	// Keep pending checkpoint/seal identities after an incomplete attempt.
	// This private ledger contains credentials; it is never public evidence.
	ledgerRoot := filepath.Join(f.dir, "ledger")
	if err := os.MkdirAll(ledgerRoot, 0700); err != nil {
		t.Fatal(err)
	}
	ledger, err := collaborationrun.OpenLedger(filepath.Join(ledgerRoot, run))
	if err != nil {
		t.Fatal(err)
	}
	logFile, err := os.OpenFile(filepath.Join(f.dir, "host-"+run+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	timeout := 60 * time.Second
	if plan.Mode == "real" && !f.env.sim {
		timeout = 5 * time.Minute
	}
	host := &collaborationrun.HostExecutor{Config: f.host, Directory: filepath.Join(t.TempDir(), "host-work"), CLIExecutable: f.env.cli,
		Logger: slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})), Options: agent.ExecOptions{Timeout: timeout}}
	runner := &collaborationrun.Runner{Ledger: ledger, Client: f.client, Host: host, StopPollInterval: 100 * time.Millisecond, PermitAttempts: 2, PermitBackoff: 10 * time.Millisecond}
	ctx, cancel := context.WithTimeout(t.Context(), timeout+time.Minute)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx, run, f.request) }()
	var duringErr error
	if during != nil {
		duringErr = during(ctx, run)
	}
	runErr := <-done
	if work := rk1BriefPath(host.Directory); work != "" {
		if brief, err := os.ReadFile(work); err == nil {
			_ = rk1WriteFile(filepath.Join(f.dir, "brief-"+run+".md"), brief)
		}
	}
	return runErr, duringErr
}

func rk1BriefPath(root string) string {
	matches, _ := filepath.Glob(filepath.Join(root, "*", "work", "CLAUDE.md"))
	if len(matches) != 1 {
		return ""
	}
	return matches[0]
}

// stopAfterCheckpoint is the H1 seeding hook: wait until the server holds a
// committed checkpoint for the run, then issue a participant Stop.
func (f *rk1Fixture) stopAfterCheckpoint(ctx context.Context, run string) error {
	for ctx.Err() == nil {
		if f.base.managed.fx.Count(f.t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='record_checkpoint'`, run) > 0 {
			f.control(true)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("no committed checkpoint before the deadline")
}

func rk1CloneTriggers(t *testing.T, f *platformAdminRouterFixture) {
	t.Helper()
	rows, err := f.pool.Query(t.Context(), `SELECT replace(pg_get_triggerdef(t.oid),
		' ON public.'||quote_ident(c.relname)||' ',
		' ON '||quote_ident(current_schema())||'.'||quote_ident(c.relname)||' ')
		FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname='public' AND NOT t.tgisinternal
		AND to_regclass(format('%I.%I',current_schema(),c.relname)) IS NOT NULL
		AND NOT EXISTS(SELECT 1 FROM pg_trigger present WHERE present.tgrelid=to_regclass(format('%I.%I',current_schema(),c.relname)) AND present.tgname=t.tgname)
		ORDER BY c.relname,t.tgname`)
	if err != nil {
		t.Fatal(err)
	}
	var statements []string
	for rows.Next() {
		var sql string
		if err = rows.Scan(&sql); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		statements = append(statements, sql)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(statements) == 0 {
		t.Fatal("RK1 did not install the migrated production triggers", err)
	}
	for _, sql := range statements {
		f.fx.Exec(t, sql)
	}
}

func rk1CommentTime(at int) time.Time {
	return time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC).Add(time.Duration(at) * time.Minute)
}
