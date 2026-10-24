package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRuntimePreparationInvocationFreezesLockedAgent(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	want := protocol.CollaborationPreparationInvocation{Instructions: "PRIVATE-INVOCATION-SENTINEL\nKeep the original task constraints.", Model: "custom-model[1m]", ThinkingLevel: "high"}
	f.fx.Exec(t, `UPDATE agent SET instructions=$2,model=$3,thinking_level=$4 WHERE id=$1`, f.grant.AgentID, want.Instructions, want.Model, want.ThinkingLevel)
	registerPreparationFixture(t, f)
	refreshPreparationConsent(t, f)
	runID := preparationCandidate(t, f)
	request, claim := claimPreparedCandidate(t, f, runID)
	if claim.Preparation.Invocation != want {
		t.Fatal("first preparation claim discarded the authorized agent invocation")
	}
	f.fx.Exec(t, `UPDATE agent SET name='Presentation changed' WHERE id=$1`, f.grant.AgentID)
	replay, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request)
	if err != nil || !reflect.DeepEqual(claim.Preparation, replay.Preparation) {
		t.Fatalf("claim replay rebuilt the frozen invocation: %v", err)
	}
	page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(page)
	if err != nil || strings.Contains(string(raw), "PRIVATE-INVOCATION-SENTINEL") || strings.Contains(string(raw), claim.Preparation.ManifestDigest) {
		t.Fatal("private invocation or its manifest digest entered the human record stream")
	}
	f.fx.Exec(t, `UPDATE agent SET instructions='Changed execution instructions',model='changed-model',thinking_level='low' WHERE id=$1`, f.grant.AgentID)
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request); err == nil {
		t.Fatal("changed agent execution settings kept stale claim authority")
	}
	stored, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	var retained protocol.CollaborationPreparation
	if err = json.Unmarshal(stored.PreparationSnapshot, &retained); err != nil || retained.Invocation != want || retained.ManifestDigest != claim.Preparation.ManifestDigest {
		t.Fatalf("configuration change rewrote the private first-claim invocation: %v", err)
	}
}

func TestCollaborationRuntimePreparationUnsupportedInvocationDoesNotReserve(t *testing.T) {
	for name, mutation := range map[string]string{
		"custom_environment":    `custom_env='{"API_KEY":"PRIVATE-CONFIG-SENTINEL"}'`,
		"custom_arguments":      `custom_args='["--private-override"]'`,
		"mcp":                   `mcp_config='{"mcpServers":{"private":{"command":"PRIVATE-CONFIG-SENTINEL"}}}'`,
		"connected_apps":        `composio_toolkit_allowlist=ARRAY['private-toolkit']`,
		"service_tier":          `service_tier='priority'`,
		"runtime_configuration": `runtime_config='{"gateway":{"token":"PRIVATE-CONFIG-SENTINEL"}}'`,
		"skill_exclusions":      `disabled_runtime_skills='[{"path":"private"}]'`,
		"autonomy":              `autonomy_level='observer'`,
		"invocation":            `thinking_level='invalid effort'`,
	} {
		t.Run(name, func(t *testing.T) {
			f := preparedRuntimeFixture(t)
			f.fx.Exec(t, `UPDATE agent SET `+mutation+` WHERE id=$1`, f.grant.AgentID)
			refreshPreparationConsent(t, f)
			f.submitManaged(t, f.creator, "honor every execution setting")
			f.resumeManaged(t)
			list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil || len(list.RunIDs) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
				t.Fatalf("unsupported %s was ignored during prepared admission: %v", name, err)
			}
			_, err = preparationInvocation(f.ctx, f.svc.Queries, f.grant.AgentID, f.runtime.Provider)
			if !collaborationErrorCode(err, "unsupported_provider_"+name) {
				t.Fatalf("unsupported category did not have a finite public reason: %v", err)
			}
			if strings.Contains(err.Error(), "PRIVATE-CONFIG-SENTINEL") {
				t.Fatal("private configuration escaped in the refusal")
			}
		})
	}
}

func TestCollaborationRuntimePreparationEnabledSkillsRefuseButEmptySettingsRemainSupported(t *testing.T) {
	f := preparedRuntimeFixture(t)
	// These are the actual empty forms emitted by existing agent APIs.
	f.fx.Exec(t, `UPDATE agent SET custom_env='{}',custom_args='[]',runtime_config='{}',mcp_config='{"mcpServers":{}}',disabled_runtime_skills='[]',service_tier='',composio_toolkit_allowlist='{}' WHERE id=$1`, f.grant.AgentID)
	refreshPreparationConsent(t, f)
	if _, err := preparationInvocation(f.ctx, f.svc.Queries, f.grant.AgentID, f.runtime.Provider); err != nil {
		t.Fatalf("explicit empty settings were treated as an executable overlay: %v", err)
	}
	skill := f.fx.Insert(t, "skill", testutil.Cols{"workspace_id": f.workspace, "name": "Preparation skill " + uuid.NewString(), "content": "PRIVATE-SKILL-CONTENT", "created_by": f.creator})
	f.fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": f.grant.AgentID, "skill_id": skill, "enabled": true}, "agent_id=$1 AND skill_id=$2", f.grant.AgentID, skill)
	refreshPreparationConsent(t, f)
	f.submitManaged(t, f.creator, "requires enabled skill materialization")
	f.resumeManaged(t)
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(list.RunIDs) != 0 {
		t.Fatalf("enabled skill was silently omitted by prepared admission: %v", err)
	}
	if _, err = preparationInvocation(f.ctx, f.svc.Queries, f.grant.AgentID, f.runtime.Provider); !collaborationErrorCode(err, "unsupported_provider_skills") {
		t.Fatalf("enabled skill did not have a finite public refusal: %v", err)
	}
	f.fx.Exec(t, `UPDATE agent_skill SET enabled=false WHERE agent_id=$1 AND skill_id=$2`, f.grant.AgentID, skill)
	if _, err = preparationInvocation(f.ctx, f.svc.Queries, f.grant.AgentID, f.runtime.Provider); err != nil {
		t.Fatalf("disabled binding was mistaken for an invocation dependency: %v", err)
	}
}
