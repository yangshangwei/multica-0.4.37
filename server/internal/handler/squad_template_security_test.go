package handler

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestListSquadTemplates_SecurityRemediationLanguages(t *testing.T) {
	for _, tc := range []struct {
		language string
		title    string
	}{
		{"en", "Security Remediation Squad"},
		{"zh", "安全漏洞修复小队"},
		{"ja", "Security Remediation Squad"},
		{"ko", "Security Remediation Squad"},
	} {
		t.Run(tc.language, func(t *testing.T) {
			var out struct {
				Templates []SquadTemplateResponse `json:"templates"`
			}
			testutil.Call(t, testHandler.ListSquadTemplates,
				newRequest("GET", "/api/squads/templates?language="+tc.language, nil)).Want(http.StatusOK).JSON(&out)
			index := slices.IndexFunc(out.Templates, func(template SquadTemplateResponse) bool {
				return template.Key == "security-remediation"
			})
			if index < 0 {
				t.Fatal("security-remediation is absent from the catalog")
			}
			template := out.Templates[index]
			if template.Title != tc.title || template.Version != 1 || template.AvatarEmoji != "🛡️" {
				t.Errorf("catalog presentation = %q/%d/%q, want %q/1/🛡️", template.Title, template.Version, template.AvatarEmoji, tc.title)
			}
			if template.Leader.TemplateKey != "security-remediation-lead" || template.Leader.AutonomyLevel != "coordinator" || len(template.Members) != 7 {
				t.Errorf("catalog roster = leader:%+v members:%d", template.Leader, len(template.Members))
			}
		})
	}
}

func TestCreateSquadFromTemplate_SecurityRemediationPrivateRoster(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		t.Run(language, func(t *testing.T) {
			out := staffSquad(t, "security-remediation", map[string]any{"language": language})
			template, ok := service.SquadTemplateByKey("security-remediation")
			if !ok {
				t.Fatal("security-remediation is absent from the registry")
			}
			if len(out.CreatedAgents) != 8 || len(out.ReusedAgents) != 0 {
				t.Fatalf("staffing created/reused = %d/%d, want 8/0", len(out.CreatedAgents), len(out.ReusedAgents))
			}
			if out.Squad.TemplateKey != template.Key || out.Squad.TemplateVersion != 1 || out.Squad.Name != template.Title(language) {
				t.Errorf("squad provenance/name = %s/%d/%q", out.Squad.TemplateKey, out.Squad.TemplateVersion, out.Squad.Name)
			}
			if out.Squad.Instructions != template.Instructions() || out.Squad.AvatarURL == nil || *out.Squad.AvatarURL != "icon:shield-check" {
				t.Error("squad must copy the canonical instructions and existing shield icon")
			}
			wantNotes := map[string]string{template.LeaderTemplateKey: "leader"}
			for _, member := range template.Members {
				wantNotes[member.TemplateKey] = member.Roles[language]
			}
			var keys []string
			for _, agentID := range out.CreatedAgents {
				var key, name, instructions, autonomy, mode, runtimeID, ownerID, note string
				var version, concurrency int32
				dbfx.QueryRow(t, `SELECT a.template_key, a.template_version, a.name, a.instructions,
					a.autonomy_level, a.permission_mode, a.runtime_id, a.owner_id, a.max_concurrent_tasks, m.role
					FROM agent a JOIN squad_member m ON m.member_id = a.id AND m.member_type = 'agent'
					WHERE a.id = $1 AND m.squad_id = $2`, agentID, out.Squad.ID).
					Scan(&key, &version, &name, &instructions, &autonomy, &mode, &runtimeID, &ownerID, &concurrency, &note)
				keys = append(keys, key)
				role, ok := service.AgentRoleTemplateByKey(key)
				if !ok {
					t.Fatalf("created agent has unknown template %q", key)
				}
				if version != role.Version || name != role.Title(language) || instructions != role.Instructions() || autonomy != string(role.Autonomy) || concurrency != role.MaxConcurrentTasks {
					t.Errorf("%s did not retain its role's provenance, localized name, instructions and authority", key)
				}
				if mode != "private" || runtimeID != handlerTestRuntimeID(t) || ownerID != testUserID {
					t.Errorf("%s permission/runtime/owner = %s/%s/%s, want private with the requested runtime and owner", key, mode, runtimeID, ownerID)
				}
				if autonomy == "operator" || note != wantNotes[key] {
					t.Errorf("%s autonomy/note = %s/%q, want a non-Operator with note %q", key, autonomy, note, wantNotes[key])
				}
				if key == template.LeaderTemplateKey && agentID != out.Squad.LeaderID {
					t.Error("dedicated leader was not bound as the squad leader")
				}
				if count := dbfx.Count(t, `SELECT COUNT(*) FROM agent_invocation_target WHERE agent_id = $1`, agentID); count != 0 {
					t.Errorf("%s has %d invocation targets despite private defaults", key, count)
				}
			}
			if !slices.Equal(keys, template.TemplateKeys()) {
				t.Errorf("created role keys = %v, want %v", keys, template.TemplateKeys())
			}
			if count := dbfx.Count(t, `SELECT COUNT(*) FROM squad_member WHERE squad_id = $1`, out.Squad.ID); count != 8 {
				t.Errorf("squad has %d member rows, want eight with the leader seated once", count)
			}
			if count := dbfx.Count(t, `SELECT COUNT(*) FROM skill WHERE workspace_id = $1 AND name LIKE 'multica-%'`, testWorkspaceID); count != 6 {
				t.Errorf("staffing created %d role skills, want six reused methods", count)
			}
		})
	}
}

func TestCreateSquadFromTemplate_SecurityRemediationReusesCustomizedMembers(t *testing.T) {
	newRuntimeID := dbfx.Runtime(t, "Security staffing runtime", testutil.Cols{"status": "offline"})
	first := staffSquad(t, "maintenance", nil)
	var implementerID, reviewSkillID string
	dbfx.QueryRow(t, `SELECT id FROM agent WHERE workspace_id = $1 AND template_key = 'implementer'`, testWorkspaceID).Scan(&implementerID)
	dbfx.Exec(t, `UPDATE agent SET name = 'Our implementer', instructions = 'Keep our repair rules', autonomy_level = 'observer' WHERE id = $1`, implementerID)
	dbfx.QueryRow(t, `SELECT id FROM skill WHERE workspace_id = $1 AND name = 'multica-security-review'`, testWorkspaceID).Scan(&reviewSkillID)
	dbfx.Exec(t, `UPDATE skill SET content = 'Our security review method', description = 'Keep the team method',
		config = config || '{"team_note":"preserve"}'::jsonb WHERE id = $1`, reviewSkillID)

	agentsBefore := map[string]string{}
	for _, agentID := range first.CreatedAgents {
		if agentID == first.Squad.LeaderID {
			continue
		}
		var snapshot string
		dbfx.QueryRow(t, `SELECT to_jsonb(a)::text FROM agent a WHERE id = $1`, agentID).Scan(&snapshot)
		agentsBefore[agentID] = snapshot
	}
	var skillBefore, bindingsBefore string
	dbfx.QueryRow(t, `SELECT to_jsonb(s)::text FROM skill s WHERE id = $1`, reviewSkillID).Scan(&skillBefore)
	dbfx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(s) ORDER BY s.skill_id)::text FROM agent_skill s WHERE s.agent_id = $1`, implementerID).Scan(&bindingsBefore)

	second := staffSquad(t, "security-remediation", map[string]any{
		"runtime_id":         newRuntimeID,
		"language":           "zh",
		"permission_mode":    "public_to",
		"invocation_targets": []map[string]any{{"target_type": "workspace"}},
	})
	if len(second.CreatedAgents) != 3 || len(second.ReusedAgents) != 5 {
		t.Fatalf("security staffing created/reused = %d/%d, want 3/5 (new leader, analyst and code reviewer)", len(second.CreatedAgents), len(second.ReusedAgents))
	}
	for _, agentID := range second.ReusedAgents {
		before, ok := agentsBefore[agentID]
		if !ok {
			t.Errorf("reused unexpected agent %s", agentID)
			continue
		}
		var after string
		dbfx.QueryRow(t, `SELECT to_jsonb(a)::text FROM agent a WHERE id = $1`, agentID).Scan(&after)
		if after != before {
			t.Errorf("reused agent %s changed: preserve names, instructions, autonomy, private Access, runtime and timestamps", agentID)
		}
		delete(agentsBefore, agentID)
	}
	if len(agentsBefore) != 0 {
		t.Errorf("%d existing working members were not reused", len(agentsBefore))
	}
	var skillAfter, bindingsAfter string
	dbfx.QueryRow(t, `SELECT to_jsonb(s)::text FROM skill s WHERE id = $1`, reviewSkillID).Scan(&skillAfter)
	dbfx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(s) ORDER BY s.skill_id)::text FROM agent_skill s WHERE s.agent_id = $1`, implementerID).Scan(&bindingsAfter)
	if skillAfter != skillBefore || bindingsAfter != bindingsBefore {
		t.Error("staffing overwrote a workspace skill or existing bindings")
	}
	for _, agentID := range second.CreatedAgents {
		var runtimeID, mode string
		dbfx.QueryRow(t, `SELECT runtime_id, permission_mode FROM agent WHERE id = $1`, agentID).Scan(&runtimeID, &mode)
		if runtimeID != newRuntimeID || mode != "public_to" {
			t.Errorf("new agent %s did not use the explicitly requested runtime and Access", agentID)
		}
	}
}

func TestCreateSquadFromTemplate_SecurityRemediationAutonomy(t *testing.T) {
	for _, level := range []string{"observer", "contributor", "coordinator"} {
		t.Run(level, func(t *testing.T) {
			agentID, taskID := autonomyTestAgent(t, "Security staffing "+level, level)
			agentsBefore := dbfx.Count(t, `SELECT COUNT(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID)
			squadsBefore := dbfx.Count(t, `SELECT COUNT(*) FROM squad WHERE workspace_id = $1`, testWorkspaceID)
			req := asAgent(newRequest("POST", "/api/squads/from-template", map[string]any{
				"template_key": "security-remediation", "runtime_id": handlerTestRuntimeID(t),
			}), agentID, taskID)
			req = withURLParam(req, "workspaceId", testWorkspaceID)
			if level == "coordinator" {
				var out CreateSquadFromTemplateResponse
				testutil.Call(t, testHandler.CreateSquadFromTemplate, req).Want(http.StatusCreated).JSON(&out)
				cleanupStaffedSquad(t, out.Squad.ID, out.CreatedAgents)
				if len(out.CreatedAgents) != 8 {
					t.Errorf("Coordinator created %d agents, want eight", len(out.CreatedAgents))
				}
				return
			}
			testutil.Call(t, testHandler.CreateSquadFromTemplate, req).Want(http.StatusForbidden)
			if dbfx.Count(t, `SELECT COUNT(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID) != agentsBefore ||
				dbfx.Count(t, `SELECT COUNT(*) FROM squad WHERE workspace_id = $1`, testWorkspaceID) != squadsBefore {
				t.Error("refused staffing left a partial squad or new agents")
			}
		})
	}
}

func TestCreateAgentFromTemplate_SecurityRemediationLeaderIsUnlisted(t *testing.T) {
	before := dbfx.Count(t, `SELECT COUNT(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID)
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "security-remediation-lead", "runtime_id": handlerTestRuntimeID(t),
	})).Want(http.StatusBadRequest)
	if after := dbfx.Count(t, `SELECT COUNT(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID); after != before {
		t.Error("the listed-role endpoint created an unlisted security leader")
	}
}

func TestSquadTemplates_SecurityRemediationStatusBriefing(t *testing.T) {
	out := staffSquad(t, "security-remediation", nil)
	squad, err := testHandler.Queries.GetSquadInWorkspace(context.Background(), db.GetSquadInWorkspaceParams{
		ID: parseUUID(out.Squad.ID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	var leaderInstructions string
	dbfx.QueryRow(t, `SELECT instructions FROM agent WHERE id = $1`, out.Squad.LeaderID).Scan(&leaderInstructions)
	for _, tc := range []struct {
		name string
		owns bool
	}{
		{"assigned", true},
		{"guest", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			briefing := buildSquadLeaderBriefing(context.Background(), testHandler.Queries, squad, tc.owns)
			combined := leaderInstructions + "\n\n" + briefing
			// Exercise the real runtime-config renderer without launching a provider.
			runtimeBrief := leaderCommentRuntimeBrief(t, combined)
			if !strings.Contains(runtimeBrief, leaderInstructions) || !strings.Contains(runtimeBrief, squad.Instructions) {
				t.Fatal("runtime brief lost one of the copied template bodies")
			}
			permission := "只有任务实际分配给本小队，且注入的 Squad Operating Protocol 授予父任务状态管理权时"
			if strings.Count(combined, permission) != 2 {
				t.Error("both copied bodies must qualify status transitions by actual assignment and injected ownership")
			}
			if tc.owns {
				if !strings.Contains(runtimeBrief, "Own the parent issue status") || !strings.Contains(runtimeBrief, "multica issue status <issue-id> in_review") {
					t.Error("owning security leader lost its review closeout responsibility")
				}
				return
			}
			if !strings.Contains(runtimeBrief, "Do NOT change this issue's status") || !strings.Contains(runtimeBrief, "保持当前负责人和状态不变") {
				t.Error("guest security leader must preserve the current assignee and status")
			}
			if strings.Contains(runtimeBrief, "Own the parent issue status") || strings.Contains(runtimeBrief, "multica issue status <issue-id> in_review") {
				t.Error("guest security leader received an owning-leader status grant or command")
			}
		})
	}
}
