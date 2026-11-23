package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestKnowledgeUnsafeV1SelectorsRefuseBeforeOrdinaryFallback(t *testing.T) {
	for _, source := range []string{"defaults", "custom"} {
		t.Run(source, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if source == "defaults" {
				f.inv.ExtraArgs = []string{"--continue"}
			} else {
				f.fx.Exec(t, `UPDATE agent SET custom_args='["--continue"]'::jsonb WHERE id=$1`, f.task.AgentID)
			}
			if _, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
				t.Fatalf("unsafe v1 selection silently downgraded: %v", err)
			}
		})
	}
}

// The v1 selector matrix lives in pkg/agent
// (TestKnowledgeSessionSelectorPresentClassifiesClaudeArgv). These suites pin
// how BuildTaskKnowledgeClaim uses it with the gate on and off.

func TestKnowledgeGateOffSafeArgsKeepOrdinaryClaim(t *testing.T) {
	for _, source := range []string{"defaults", "custom", "persistence"} {
		t.Run(source, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			f.tasks.KnowledgeContextEnabled = false
			switch source {
			case "defaults":
				f.inv.ExtraArgs = []string{"--add-dir", "/tmp/x"}
			case "custom":
				f.fx.Exec(t, `UPDATE agent SET custom_args='["--allowedTools","Bash"]'::jsonb WHERE id=$1`, f.task.AgentID)
			case "persistence":
				f.inv.ExtraArgs = []string{"--no-session-persistence"}
			}
			d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "legacy-native")
			if err != nil || d != nil || prior != "legacy-native" {
				t.Fatalf("closed gate refused or rewrote an ordinary claim with safe options: d=%v prior=%q err=%v", d != nil, prior, err)
			}
		})
	}
}

func TestKnowledgeGateOnSafeUnlistedArgsUseBaseExecution(t *testing.T) {
	for _, source := range []string{"custom", "defaults"} {
		t.Run(source, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if source == "defaults" {
				f.inv.ExtraArgs = []string{"--add-dir", "/tmp/x"}
			} else {
				f.fx.Exec(t, `UPDATE agent SET custom_args='["--allowedTools","Bash"]'::jsonb WHERE id=$1`, f.task.AgentID)
			}
			d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "")
			if err != nil || d != nil || prior != "" {
				t.Fatalf("safe option outside the negotiation allowlist did not take base execution: d=%v prior=%q err=%v", d != nil, prior, err)
			}
		})
	}
}

func TestKnowledgeOrdinaryNoSessionPersistenceNeverNegotiates(t *testing.T) {
	for _, source := range []string{"defaults", "custom"} {
		t.Run(source, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if source == "defaults" {
				f.inv.ExtraArgs = []string{"--no-session-persistence"}
			} else {
				f.fx.Exec(t, `UPDATE agent SET custom_args='["--no-session-persistence"]'::jsonb WHERE id=$1`, f.task.AgentID)
			}
			// finalize builds the claim and passes its descriptor to FinalizeTaskClaim.
			if d := f.finalize(t, ""); d != nil {
				t.Fatal("ordinary reusable history negotiated with persistence disabled")
			}
			row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if row.KnowledgeContextVersion.Valid {
				t.Fatal("base execution recorded knowledge negotiation")
			}
		})
	}
}

func TestKnowledgeSessionSelectorsRefusedWhateverTheGate(t *testing.T) {
	forms := [][]string{{"--continue"}, {"-c"}, {"-pc"}, {"--resume", "abc"}, {"--resume=abc"}, {"-rabc"}, {"--session-id=abc"}, {"--fork-session"}, {"--allowedTools", "Bash", "--continue"}, {"'--continue'"},
		// Remote, cloud and attached sessions, a subcommand operand, and a bare
		// "--" that would stop option parsing for later configured arguments.
		{"--teleport"}, {"--teleport", "abc"}, {"--cloud", "session_abc"}, {"--remote-control"}, {"--attach", "abc"}, {"--bg"}, {"attach", "abc"}, {"--"}, {"--allowedTools", "Bash", "--"}}
	for _, gate := range []bool{true, false} {
		for _, source := range []string{"defaults", "custom"} {
			t.Run(fmt.Sprintf("gate=%t/%s", gate, source), func(t *testing.T) {
				f := newTaskKnowledgeFixture(t)
				f.tasks.KnowledgeContextEnabled = gate
				for _, form := range forms {
					inv := f.inv
					custom := []byte("[]")
					if source == "defaults" {
						inv.ExtraArgs = form
					} else {
						raw, err := json.Marshal(form)
						if err != nil {
							t.Fatal(err)
						}
						custom = raw
					}
					f.fx.Exec(t, `UPDATE agent SET custom_args=$2::jsonb WHERE id=$1`, f.task.AgentID, string(custom))
					d, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &inv, "")
					if !collaborationErrorCode(err, "knowledge_invocation_unsupported") || d != nil {
						t.Errorf("session selector %q was stripped or allowed: d=%v err=%v", form, d != nil, err)
					}
				}
			})
		}
	}
}

func TestKnowledgeRetainedNegotiationRefusesSafeUnlistedArgs(t *testing.T) {
	for name, custom := range map[string]string{"allowed_tools": `["--allowedTools","Bash"]`, "no_session_persistence": `["--no-session-persistence"]`} {
		t.Run(name, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if d := f.finalize(t, ""); d == nil {
				t.Fatal("fixture did not negotiate")
			}
			f.fx.Exec(t, `UPDATE agent SET custom_args=$2::jsonb WHERE id=$1`, f.task.AgentID, custom)
			task, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !task.KnowledgeContextVersion.Valid {
				t.Fatal("fixture row has no retained negotiation")
			}
			if _, _, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, task, &f.inv, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
				t.Fatalf("changed contract inherited an old admission: %v", err)
			}
		})
	}
}

// A safe option outside the negotiation allowlist makes a fresh v1 claim
// unsupported; it does not exempt the claim from the namespace history guard.
func TestKnowledgeSafeUnlistedArgsRefuseInExposedNamespace(t *testing.T) {
	for _, gate := range []bool{true, false} {
		for _, source := range []string{"defaults", "custom"} {
			t.Run(fmt.Sprintf("gate=%t/%s", gate, source), func(t *testing.T) {
				f := newTaskKnowledgeFixture(t)
				f.recordSession(t, "exposed-native")
				if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "exposed-native", "", "", false, "", ""); err != nil {
					t.Fatal(err)
				}
				f.tasks.KnowledgeContextEnabled = gate
				runtimeID := util.UUIDToString(f.task.RuntimeID)
				agentID := f.fx.Agent(t, "Unrelated agent on the exposed runtime", runtimeID, testutil.Cols{"runtime_mode": "local"})
				issue := f.fx.Issue(t, "Unrelated fresh scope", testutil.Cols{"assignee_type": "agent", "assignee_id": agentID})
				id := f.fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issue, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1})
				task, err := f.tasks.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
				if err != nil {
					t.Fatal(err)
				}
				inv := f.inv
				if source == "defaults" {
					inv.ExtraArgs = []string{"--allowedTools", "Bash"}
				} else {
					f.fx.Exec(t, `UPDATE agent SET custom_args='["--allowedTools","Bash"]'::jsonb WHERE id=$1`, agentID)
				}
				if d, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, task, &inv, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") || d != nil {
					t.Fatalf("fresh claim in an exposed namespace ran without the history guard: d=%v err=%v", d != nil, err)
				}
			})
		}
	}
}

// custom_args that do not decode to []string refuse a v1 claim whatever the
// gate; they are never read as "no custom arguments".
func TestKnowledgeMalformedCustomArgsRefuseWhateverTheGate(t *testing.T) {
	shapes := []string{`{"--continue":true}`, `"--allowedTools Bash"`, `[1,2]`, `[["--model","sonnet"]]`, `true`, `[{"arg":"--verbose"}]`}
	for _, gate := range []bool{true, false} {
		t.Run(fmt.Sprintf("gate=%t", gate), func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			f.tasks.KnowledgeContextEnabled = gate
			for _, shape := range shapes {
				f.fx.Exec(t, `UPDATE agent SET custom_args=$2::jsonb WHERE id=$1`, f.task.AgentID, shape)
				if d, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") || d != nil {
					t.Errorf("custom_args %s that are not a string array were accepted: d=%v err=%v", shape, d != nil, err)
				}
			}
		})
	}
}

// A known managed session whose exact history is claimed-only, or prepared with
// an empty manifest, has disclosed nothing: preparation commits the manifest
// before any contribution leaves the server, and launch authorization requires
// that preparation. An unsupported v1 invocation (here a safe unlisted option)
// therefore resumes it as ordinary work with the gate on or off. The prepared
// row is the contrast: a committed non-empty manifest refuses even though no
// process was ever authorized or started.
func TestKnowledgeClaimedOnlyManagedSessionResumesForUnsupportedInvocation(t *testing.T) {
	for _, phase := range []string{"claimed", "prepared_empty", "prepared"} {
		for _, gate := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/gate=%t", phase, gate), func(t *testing.T) {
				f := newTaskKnowledgeFixture(t)
				if phase == "prepared_empty" {
					f.withdraw(t, f.candidate.CandidateID)
				}
				d := f.finalize(t, "legacy-native")
				if d == nil || d.Session.NativeSessionID != "legacy-native" {
					t.Fatal("fixture did not bind the managed claim to the chosen native session")
				}
				if phase != "claimed" {
					p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
					if err != nil {
						t.Fatal(err)
					}
					if (len(p.Manifest.Entries) == 0) != (phase == "prepared_empty") {
						t.Fatalf("fixture manifest has %d entries", len(p.Manifest.Entries))
					}
				}
				if _, err := f.tasks.CancelTaskWithResult(f.ctx, f.task.ID, CancelTaskOptions{UserInitiated: true}); err != nil {
					t.Fatal(err)
				}
				f.nextTurn(t)
				f.tasks.KnowledgeContextEnabled = gate
				f.fx.Exec(t, `UPDATE agent SET custom_args='["--allowedTools","Bash"]'::jsonb WHERE id=$1`, f.task.AgentID)
				claim, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "legacy-native")
				if phase == "prepared" {
					if !collaborationErrorCode(err, "knowledge_invocation_unsupported") || claim != nil {
						t.Fatalf("a committed private manifest was resumed by an unsupported invocation: d=%v prior=%q err=%v", claim != nil, prior, err)
					}
					return
				}
				if err != nil || claim != nil || prior != "legacy-native" {
					t.Fatalf("history that disclosed nothing did not resume as ordinary work: d=%v prior=%q err=%v", claim != nil, prior, err)
				}
			})
		}
	}
}

func TestKnowledgeUnsupportedBackendCannotResumeManagedSessionAcrossScopes(t *testing.T) {
	for _, namespace := range []string{"same_runtime", "same_daemon_new_runtime", "same_owner_new_daemon", "deleted_runtime"} {
		t.Run(namespace, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			f.recordSession(t, "private-history")
			if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "private-history", "/shared/repo", "", false, "", ""); err != nil {
				t.Fatal(err)
			}
			oldRuntime := f.task.RuntimeID
			runtime, err := f.tasks.Queries.GetAgentRuntime(f.ctx, oldRuntime)
			if err != nil {
				t.Fatal(err)
			}
			newRuntime := util.UUIDToString(oldRuntime)
			if namespace != "same_runtime" {
				daemon := runtime.DaemonID.String
				if namespace == "same_owner_new_daemon" {
					daemon = "replacement-" + uuid.NewString()
				}
				profile := f.fx.Insert(t, "runtime_profile", testutil.Cols{"workspace_id": f.workspace, "display_name": "Alternate Claude family", "protocol_family": "claude", "command_name": "test-owned-wrapper", "created_by": f.creator, "fixed_args": []byte(`["--continue"]`)})
				newRuntime = f.fx.Runtime(t, "Other protocol-family runtime", testutil.Cols{"runtime_mode": "local", "provider": "claude", "daemon_id": daemon, "profile_id": profile})
			}
			if namespace == "deleted_runtime" {
				f.fx.Exec(t, `UPDATE agent_task_queue SET runtime_id=NULL WHERE id=$1`, f.task.ID)
				f.fx.Exec(t, `UPDATE agent SET runtime_id=NULL WHERE id=$1`, f.task.AgentID)
				f.fx.Exec(t, `DELETE FROM agent_runtime WHERE id=$1`, oldRuntime)
			}
			agentID := f.fx.Agent(t, "Different-scope legacy agent", newRuntime)
			issueID := f.fx.Issue(t, "Different task scope", testutil.Cols{"assignee_type": "agent", "assignee_id": agentID})
			id := f.fx.Task(t, agentID, testutil.Cols{"runtime_id": newRuntime, "issue_id": issueID, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1})
			task, err := f.tasks.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, task, nil, ""); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
				t.Fatalf("implicit continuation could restore another scope's history: %v", err)
			}
		})
	}
}

func TestKnowledgeValidEmptyAndClaimedHistoryPreserveLegacyWork(t *testing.T) {
	for _, phase := range []string{"claimed", "empty_disclosure"} {
		t.Run(phase, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			if phase == "claimed" {
				f.finalize(t, "")
				if _, err := f.tasks.CancelTaskWithResult(f.ctx, f.task.ID, CancelTaskOptions{UserInitiated: true}); err != nil {
					t.Fatal(err)
				}
			} else {
				f.withdraw(t, f.candidate.CandidateID)
				f.recordSession(t, "empty-native")
				if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "empty-native", "", "", false, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			f.nextTurn(t)
			d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, nil, "empty-native")
			if err != nil || d != nil || prior != "empty-native" {
				t.Fatalf("no-exposure history reset legacy work: %q %v", prior, err)
			}
		})
	}
}

func TestKnowledgeRuntimeHistoryRefusesCorruptProof(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	f.finalize(t, "")
	f.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_delivery='{}'::jsonb WHERE id=$1`, f.task.ID)
	newAgent := f.fx.Agent(t, "Legacy history guard", util.UUIDToString(f.task.RuntimeID))
	issue := f.fx.Issue(t, "Unrelated scope")
	id := f.fx.Task(t, newAgent, testutil.Cols{"runtime_id": f.task.RuntimeID, "issue_id": issue, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1})
	task, err := f.tasks.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.tasks.BuildTaskKnowledgeClaim(f.ctx, task, nil, ""); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("unprovable runtime history became legacy: %v", err)
	}
}

func TestKnowledgeClaimDoesNotNegotiateAfterProviderChanges(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d, _, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "")
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_runtime SET provider='opencode' WHERE id=$1`, f.task.RuntimeID)
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if err = f.tasks.finalizeTaskKnowledgeClaim(f.ctx, f.tasks.Queries.WithTx(tx), f.task, d); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("changed effective provider was finalized: %v", err)
	}
}
