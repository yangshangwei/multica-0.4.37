package handler

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/skillbundle"
)

// This matrix covers the actual claim and resolve boundaries. The kickoff is
// durable session context: a continuation or retry must retain the skill even
// when the kickoff belongs to an earlier task's input batch.
func TestClaimTaskByRuntime_BuiltinSkillsFollowOnboardingScope(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	modes := []struct {
		name       string
		capability string
	}{
		{name: "inline"},
		{name: "refs", capability: protocol.DaemonCapabilitySkillBundlesV1},
	}
	cases := []struct {
		name           string
		displayName    string
		mika           bool
		kickoff        bool
		otherChat      bool
		previousStatus string
		wantOnboarding bool
	}{
		{name: "ordinary agent", displayName: "Assistant"},
		{name: "display name is not identity", displayName: "Mika", kickoff: true},
		{name: "Mika ordinary chat", displayName: "Chief of Staff", mika: true},
		{name: "Mika kickoff in another chat", displayName: "Chief of Staff", mika: true, kickoff: true, otherChat: true},
		{name: "Mika first onboarding turn", displayName: "Chief of Staff", mika: true, kickoff: true, wantOnboarding: true},
		{name: "Mika continuation", displayName: "Chief of Staff", mika: true, kickoff: true, previousStatus: "completed", wantOnboarding: true},
		{name: "Mika retry", displayName: "Chief of Staff", mika: true, kickoff: true, previousStatus: "failed", wantOnboarding: true},
	}
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					runtimeID := createClaimReclaimRuntime(t, context.Background(), t.Name())
					agentID := dbfx.Agent(t, tc.displayName, runtimeID)
					if tc.mika {
						markAsMika(t, agentID)
					}
					sessionID := dbfx.ChatSession(t, agentID, testutil.Cols{
						"runtime_id":            runtimeID,
						"explicitly_created_at": testutil.Raw("now()"),
					})

					previousID := ""
					if tc.previousStatus != "" {
						previousID = dbfx.Task(t, agentID, testutil.Cols{
							"runtime_id":      runtimeID,
							"chat_session_id": sessionID,
							"status":          tc.previousStatus,
							"completed_at":    testutil.Raw("now()"),
						})
					}
					taskCols := testutil.Cols{"runtime_id": runtimeID, "chat_session_id": sessionID}
					if tc.previousStatus == "failed" {
						taskCols["retry_of_task_id"] = previousID
						taskCols["parent_task_id"] = previousID
						taskCols["chat_input_task_id"] = previousID
					}
					taskID := dbfx.Task(t, agentID, taskCols)
					inputOwner := taskID
					if tc.previousStatus == "failed" {
						inputOwner = previousID
					} else {
						dbfx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id = id WHERE id = $1`, taskID)
					}
					// Mentioning a skill in an ordinary message must not turn a
					// chat into a product-authored onboarding session.
					dbfx.Insert(t, "chat_message", testutil.Cols{
						"chat_session_id": sessionID,
						"role":            "user",
						"content":         "Please use multica-onboarding to help me.",
						"task_id":         inputOwner,
					})
					if tc.kickoff {
						kickoffSession, kickoffOwner := sessionID, inputOwner
						if previousID != "" {
							kickoffOwner = previousID
						}
						if tc.otherChat {
							kickoffSession = dbfx.ChatSession(t, agentID)
						}
						dbfx.Insert(t, "chat_message", testutil.Cols{
							"chat_session_id": kickoffSession,
							"role":            "user",
							"message_kind":    protocol.ChatMessageKindOnboardingKickoff,
							"content":         "Product-authored onboarding context",
							"task_id":         kickoffOwner,
						})
					}

					req := newDaemonTokenRequest(http.MethodPost,
						"/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil,
						testWorkspaceID, "builtin-scope-daemon")
					if mode.capability != "" {
						req.Header.Set("X-Client-Capabilities", mode.capability)
					}
					req = withURLParam(req, "runtimeId", runtimeID)
					var claimed struct {
						Task *AgentTaskResponse `json:"task"`
					}
					testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(http.StatusOK).JSON(&claimed)
					if claimed.Task == nil || claimed.Task.ID != taskID || claimed.Task.Agent == nil {
						t.Fatalf("claimed task = %+v, want task %s with agent data", claimed.Task, taskID)
					}
					names := []string{}
					if mode.capability == "" {
						if len(claimed.Task.Agent.SkillRefs) != 0 {
							t.Fatal("inline claim unexpectedly returned skill refs")
						}
						for _, skill := range claimed.Task.Agent.Skills {
							names = append(names, skill.Name)
						}
					} else {
						if len(claimed.Task.Agent.Skills) != 0 {
							t.Fatal("refs claim unexpectedly returned inline skill bodies")
						}
						for _, ref := range claimed.Task.Agent.SkillRefs {
							names = append(names, ref.Name)
						}
					}
					want := []string{
						"multica-autopilots", "multica-creating-agents", "multica-mentioning",
						"multica-projects-and-resources", "multica-runtimes-and-repos",
						"multica-skill-importing", "multica-squads", "multica-working-on-issues",
					}
					if tc.wantOnboarding {
						want = append(want, "multica-onboarding")
					}
					slices.Sort(names)
					slices.Sort(want)
					if !slices.Equal(names, want) {
						t.Errorf("claim skills = %v, want %v", names, want)
					}

					onboardingRef := resolveSkillBundleRef{
						ID: service.BuiltinSkillID("multica-onboarding"), Source: skillbundle.SourceBuiltin, Hash: "sha256:stale",
					}
					if tc.wantOnboarding {
						var resolved struct {
							Bundles []service.AgentSkillData `json:"bundles"`
						}
						resolveBundles(t, testHandler, runtimeID, taskID, onboardingRef).Want(http.StatusOK).JSON(&resolved)
						if len(resolved.Bundles) != 1 || resolved.Bundles[0].Name != "multica-onboarding" || resolved.Bundles[0].Content == "" {
							t.Fatalf("onboarding resolve = %+v, want its full skill bundle", resolved.Bundles)
						}
					} else {
						resolveBundles(t, testHandler, runtimeID, taskID, onboardingRef).Want(http.StatusNotFound)
					}
					// Rejecting onboarding must not revoke platform skills used by
					// every role, including read-only inspection and commenting.
					resolveBundles(t, testHandler, runtimeID, taskID, resolveSkillBundleRef{
						ID: service.BuiltinSkillID("multica-mentioning"), Source: skillbundle.SourceBuiltin, Hash: "sha256:stale",
					}).Want(http.StatusOK)
				})
			}
		})
	}
}

func TestResolveTaskSkillBundles_OnboardingRequiresTheSessionsOwnAgent(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	runtimeID := createClaimReclaimRuntime(t, context.Background(), t.Name())
	mikaID := markAsMika(t, dbfx.Agent(t, "Mika", runtimeID))
	otherID := dbfx.Agent(t, "Other agent", runtimeID)
	sessionID := dbfx.ChatSession(t, mikaID)
	dbfx.Insert(t, "chat_message", testutil.Cols{
		"chat_session_id": sessionID,
		"role":            "user",
		"message_kind":    protocol.ChatMessageKindOnboardingKickoff,
		"content":         "Product-authored onboarding context",
	})
	taskID := dbfx.Task(t, otherID, testutil.Cols{
		"runtime_id":      runtimeID,
		"chat_session_id": sessionID,
		"status":          "dispatched",
		"dispatched_at":   testutil.Raw("now()"),
	})
	resolveBundles(t, testHandler, runtimeID, taskID, resolveSkillBundleRef{
		ID: service.BuiltinSkillID("multica-onboarding"), Source: skillbundle.SourceBuiltin, Hash: "sha256:stale",
	}).Want(http.StatusNotFound)
	resolveBundles(t, testHandler, runtimeID, taskID, resolveSkillBundleRef{
		ID: service.BuiltinSkillID("multica-mentioning"), Source: skillbundle.SourceBuiltin, Hash: "sha256:stale",
	}).Want(http.StatusOK)
}
