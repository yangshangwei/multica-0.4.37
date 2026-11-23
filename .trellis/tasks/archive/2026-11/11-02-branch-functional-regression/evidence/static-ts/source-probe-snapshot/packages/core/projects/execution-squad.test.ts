// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { Agent, AgentRuntime, ProjectResource, Squad } from "../types";
import { eligibleProjectRuntimes, getProjectExecutionSquads, getProjectSquadReadiness, projectLocalDaemonIds, projectSquadSelection, replaceProjectSquadSelection, selectProjectRuntime } from "./execution-squad";

const runtime = (id: string, overrides: Partial<AgentRuntime> = {}): AgentRuntime => ({
  id, workspace_id: "ws-1", daemon_id: "machine-1", name: id, runtime_mode: "local",
  provider: "codex", launch_header: "", status: "online", device_info: "", metadata: {},
  owner_id: "user-1", visibility: "private", last_seen_at: null, created_at: "", updated_at: "", ...overrides,
});
const SQUAD = { id: "squad-1", workspace_id: "ws-1", leader_id: "agent-1", archived_at: null } as Squad;
const LEADER = { id: "agent-1", workspace_id: "ws-1", runtime_id: "actual-runtime", archived_at: null } as Agent;
const CONFIG = { state: "configured" as const, squad_id: "squad-1", runtime_id: "requested-runtime" };

describe("project candidate selections", () => {
  it("reads old cached defaults but respects an explicitly cleared candidate list", () => {
    expect(getProjectExecutionSquads({ execution_squad: CONFIG })).toEqual([CONFIG]);
    expect(getProjectExecutionSquads({ execution_squad: CONFIG, execution_squads: [] })).toEqual([]);
    expect(getProjectExecutionSquads({ execution_squad: { state: "none" } })).toEqual([]);
  });

  it("retains template provenance and the requested runtime when editing another candidate", () => {
    expect(projectSquadSelection({ ...CONFIG, template_key: "delivery" })).toEqual({
      template_key: "delivery", runtime_id: "requested-runtime",
    });
    expect(projectSquadSelection(CONFIG)).toEqual({ squad_id: "squad-1" });
  });

  it("updates an existing template in place instead of appending a duplicate", () => {
    const configs = [{ ...CONFIG, template_key: "delivery" }, { ...CONFIG, squad_id: "squad-2" }];
    expect(replaceProjectSquadSelection(configs, configs.length, { template_key: "delivery", runtime_id: "new-runtime" })).toEqual([
      { template_key: "delivery", runtime_id: "new-runtime" }, { squad_id: "squad-2" },
    ]);
  });

  it("recognizes a configured template's squad when it is chosen from existing squads", () => {
    const configs = [{ ...CONFIG, template_key: "delivery" }, { ...CONFIG, squad_id: "squad-2" }];
    expect(replaceProjectSquadSelection(configs, configs.length, { squad_id: "squad-1" })).toEqual([
      { template_key: "delivery", runtime_id: "requested-runtime" }, { squad_id: "squad-2" },
    ]);
  });

  it("merges an edited candidate with a squad that is already selected", () => {
    const configs = [CONFIG, { ...CONFIG, squad_id: "squad-2" }];
    expect(replaceProjectSquadSelection(configs, 0, { squad_id: "squad-2" })).toEqual([{ squad_id: "squad-2" }]);
    expect(replaceProjectSquadSelection(configs, 1, { squad_id: "squad-1" })).toEqual([{ squad_id: "squad-1" }]);
  });
});

describe("project runtime selection", () => {
  it("honors workspace, owner and project machine together", () => {
    const runtimes = [runtime("good"), runtime("foreign", { workspace_id: "ws-2" }),
      runtime("private", { owner_id: "someone-else" }), runtime("other-machine", { daemon_id: "machine-2" }),
      runtime("ownerless", { owner_id: null, visibility: "public" })];
    expect(eligibleProjectRuntimes(runtimes, { workspaceId: "ws-1", userId: "user-1", daemonId: "machine-1" }).map(r => r.id)).toEqual(["good"]);
  });
  it("waits for identity and never selects the first of several runtimes", () => {
    expect(selectProjectRuntime([runtime("a")], { workspaceId: "ws-1" })).toBeNull();
    expect(selectProjectRuntime([runtime("a"), runtime("b")], { workspaceId: "ws-1", userId: "user-1" })).toBeNull();
  });
  it("prefers a suitable existing selection, otherwise uses a sole online runtime", () => {
    const options = { workspaceId: "ws-1", userId: "user-1" };
    expect(selectProjectRuntime([runtime("a"), runtime("b")], { ...options, preferredRuntimeId: "b" })?.id).toBe("b");
    expect(selectProjectRuntime([runtime("a", { status: "offline" }), runtime("b")], options)?.id).toBe("b");
    expect(selectProjectRuntime([runtime("a", { status: "offline" })], options)).toBeNull();
  });
  it("supports any explicitly mapped project machine", () => {
    const options = { workspaceId: "ws-1", userId: "user-1", daemonIds: ["machine-1", "machine-2"] };
    expect(eligibleProjectRuntimes([runtime("a"), runtime("b", { daemon_id: "machine-2" }), runtime("c", { daemon_id: "machine-3" })], options).map(r=>r.id)).toEqual(["a", "b"]);
  });
});

describe("project squad readiness", () => {
  const targets = { squad: SQUAD, leader: LEADER, runtime: runtime("actual-runtime"), canInvoke: true,
    members: [{ member_type: "agent" as const, member_id: LEADER.id }], agents: [LEADER], runtimes: [runtime("actual-runtime")],
  };
  it("uses the actual leader runtime instead of the requested template runtime", () => {
    expect(getProjectSquadReadiness(CONFIG, targets)).toBe("ready");
    expect(getProjectSquadReadiness(CONFIG, { ...targets, runtime: runtime("requested-runtime") })).toBe("unavailable");
  });
  it("keeps setup state separate from execution availability", () => {
    expect(getProjectSquadReadiness(null, targets)).toBe("none");
    expect(getProjectSquadReadiness({ state: "needs_runtime", template_key: "feature-delivery" }, targets)).toBe("needs_runtime");
    expect(getProjectSquadReadiness({ state: "failed" }, targets)).toBe("failed");
    expect(getProjectSquadReadiness(CONFIG, { ...targets, runtime: runtime("actual-runtime", { status: "offline" }) })).toBe("offline");
  });
  it("does not enable dispatch for revoked, archived, missing or foreign targets", () => {
    expect(getProjectSquadReadiness(CONFIG, { ...targets, canInvoke: false })).toBe("unavailable");
    expect(getProjectSquadReadiness(CONFIG, { ...targets, squad: null })).toBe("unavailable");
    expect(getProjectSquadReadiness(CONFIG, { ...targets, leader: { ...LEADER, archived_at: "yesterday" } })).toBe("unavailable");
    expect(getProjectSquadReadiness(CONFIG, { ...targets, runtime: runtime("actual-runtime", { workspace_id: "ws-2" }) })).toBe("unavailable");
    expect(getProjectSquadReadiness(CONFIG, { ...targets, leader: { ...LEADER, runtime_bound: false } })).toBe("unavailable");
  });
  it("requires the effective runtime to have a mapped local directory", () => {
    expect(getProjectSquadReadiness(CONFIG, { ...targets, localDaemonIds: ["machine-2"] })).toBe("wrong_machine");
    expect(getProjectSquadReadiness(CONFIG, { ...targets, localDaemonIds: ["machine-1", "machine-2"] })).toBe("ready");
  });
  it("checks current non-leader member bindings after a squad was configured", () => {
    const implementer = { ...LEADER, id: "implementer", runtime_id: "worker-runtime" };
    const squad = { ...targets,
      members: [...targets.members, { member_type: "agent" as const, member_id: implementer.id }],
      agents: [LEADER, implementer],
      runtimes: [runtime("actual-runtime"), runtime("worker-runtime", { daemon_id: "machine-2" })],
      localDaemonIds: ["machine-1"],
    };
    expect(getProjectSquadReadiness(CONFIG, squad)).toBe("wrong_machine");
    expect(getProjectSquadReadiness(CONFIG, { ...squad, localDaemonIds: [] })).toBe("ready");
    expect(getProjectSquadReadiness(CONFIG, { ...squad, localDaemonIds: [],
      runtimes: [runtime("actual-runtime"), runtime("worker-runtime", { status: "offline" })] })).toBe("offline");
    expect(getProjectSquadReadiness(CONFIG, { ...squad, agents: [LEADER] })).toBe("unavailable");
  });
  it("waits for a complete roster before declaring readiness", () => {
    expect(getProjectSquadReadiness(CONFIG, { ...targets, members: undefined })).toBe("unavailable");
    expect(getProjectSquadReadiness(CONFIG, { ...targets, members: [] })).toBe("unavailable");
  });
});

it("derives unique local machine mappings defensively", () => {
  const resources = [
    { resource_type: "local_directory", resource_ref: { daemon_id: "a" } },
    { resource_type: "local_directory", resource_ref: { daemon_id: "a" } },
    { resource_type: "local_directory", resource_ref: { daemon_id: 42 } },
    { resource_type: "github_repo", resource_ref: { url: "https://example.test/repo" } },
  ] as ProjectResource[];
  expect(projectLocalDaemonIds(resources)).toEqual(["a"]);
});
