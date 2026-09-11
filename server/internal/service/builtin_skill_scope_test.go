package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

func TestBuiltinSkillsExcludeOnboarding(t *testing.T) {
	svc := &TaskService{}
	skills := svc.BuiltinSkills()
	if len(skills) != len(loadBuiltinSkills())-1 {
		t.Fatalf("general skills = %d, want the catalog without onboarding", len(skills))
	}
	for _, skill := range skills {
		if skill.Name == "multica-onboarding" {
			t.Fatal("Mika onboarding must not be distributed as a general platform skill")
		}
	}
}

// The identity/provenance matrix is exercised against real rows in
// handler/daemon_builtin_skills_scope_test.go. These tests own read failures
// and the no-extra-query contract for general skill downloads.
type onboardingScopeDBTX struct {
	skillReadDBTX
	allowed bool
	err     error
	reads   int
}

func (d *onboardingScopeDBTX) QueryRow(context.Context, string, ...any) pgx.Row {
	d.reads++
	return onboardingScopeRow{allowed: d.allowed, err: d.err}
}

type onboardingScopeRow struct {
	allowed bool
	err     error
}

func (r onboardingScopeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*bool) = r.allowed
	return nil
}

func TestTaskBuiltinSkills_NonChatDoesNotReadScope(t *testing.T) {
	fake := &onboardingScopeDBTX{err: errInjectedSkillRead}
	svc := &TaskService{Queries: db.New(fake)}
	skills, err := svc.TaskBuiltinSkills(context.Background(), db.AgentTaskQueue{AgentID: testUUID(9)})
	if err != nil || len(skills) != len(svc.BuiltinSkills()) || fake.reads != 0 {
		t.Fatalf("non-chat skills = %d, scope reads = %d, error = %v", len(skills), fake.reads, err)
	}
}

func TestLoadAgentSkillBundles_OnboardingScopeFailure(t *testing.T) {
	fake := &onboardingScopeDBTX{err: errInjectedSkillRead}
	svc := &TaskService{Queries: db.New(fake)}
	task := db.AgentTaskQueue{AgentID: testUUID(9), ChatSessionID: testUUID(10)}
	bundles, refs, err := svc.LoadAgentSkillBundles(context.Background(), task)
	if !errors.Is(err, errInjectedSkillRead) || bundles != nil || refs != nil {
		t.Fatalf("failed scope read returned bundles=%d refs=%d error=%v", len(bundles), len(refs), err)
	}
}

func TestLoadRequestedAgentSkillBundles_OnboardingScope(t *testing.T) {
	for _, tc := range []struct {
		name      string
		skillName string
		allowed   bool
		readErr   error
		wantReads int
		wantCount int
	}{
		{name: "general skill needs no scope read", skillName: "multica-working-on-issues", readErr: errInjectedSkillRead, wantCount: 1},
		{name: "onboarding denied", skillName: "multica-onboarding", wantReads: 1},
		{name: "onboarding allowed", skillName: "multica-onboarding", allowed: true, wantReads: 1, wantCount: 1},
		{name: "onboarding read failure", skillName: "multica-onboarding", readErr: errInjectedSkillRead, wantReads: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &onboardingScopeDBTX{allowed: tc.allowed, err: tc.readErr}
			svc := &TaskService{Queries: db.New(fake)}
			task := db.AgentTaskQueue{AgentID: testUUID(9), ChatSessionID: testUUID(10)}
			bundles, err := svc.LoadRequestedAgentSkillBundles(context.Background(), task, []AgentSkillBundleRef{{
				ID: BuiltinSkillID(tc.skillName), Source: skillbundle.SourceBuiltin,
			}})
			if tc.wantReads > 0 && tc.readErr != nil {
				if !errors.Is(err, tc.readErr) || bundles != nil {
					t.Fatalf("failed scope read returned bundles=%d error=%v", len(bundles), err)
				}
			} else if err != nil || len(bundles) != tc.wantCount {
				t.Fatalf("resolved bundles=%d error=%v, want %d bundles", len(bundles), err, tc.wantCount)
			}
			if fake.reads != tc.wantReads {
				t.Fatalf("scope reads = %d, want %d", fake.reads, tc.wantReads)
			}
		})
	}
}
