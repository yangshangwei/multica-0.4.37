// @vitest-environment node

import { describe, expect, it } from "vitest";
import type { AgentRuntime } from "@multica/core/types";
import { computerBaseline, newlyConnectedComputer } from "./connect-remote-machine";

function runtime(overrides: Partial<AgentRuntime> = {}): AgentRuntime {
  return {
    id: "rt-1", workspace_id: "ws-1", daemon_id: "daemon-1", owner_id: "user-1",
    name: "Codex (Office-PC)", provider: "codex", runtime_mode: "local", status: "online",
    launch_header: "", device_info: "Office-PC · windows-amd64", metadata: {}, visibility: "private",
    last_seen_at: "2026-10-11T12:00:00Z", created_at: "2026-10-11T12:00:00Z", updated_at: "2026-10-11T12:00:00Z",
    ...overrides,
  };
}

describe("new computer confirmation", () => {
  it("confirms owned, online local runtimes and groups the real computer details", () => {
    const machine = newlyConnectedComputer([
      runtime(), runtime({ id: "rt-2", provider: "claude", name: "Claude (Office-PC)" }),
    ], computerBaseline([], "user-1", "ws-1"), "user-1", "ws-1");
    expect(machine).toMatchObject({ title: "Office-PC", daemonId: "daemon-1", onlineCount: 2 });
    expect(machine?.runtimes).toHaveLength(2);
  });

  it.each([
    { owner_id: "other-user" }, { workspace_id: "other-workspace" },
    { runtime_mode: "cloud" as const }, { status: "offline" as const },
    { daemon_id: null, name: "Codex", device_info: "" },
    { name: "Codex", device_info: "" },
  ])("does not confirm an unqualified runtime %j", (overrides) => {
    expect(newlyConnectedComputer([runtime(overrides)], new Set(), "user-1", "ws-1")).toBeNull();
  });

  it("ignores existing computer reconnects and newly installed providers", () => {
    const baseline = computerBaseline([runtime({ status: "offline" })], "user-1", "ws-1");
    expect(newlyConnectedComputer([runtime()], baseline, "user-1", "ws-1")).toBeNull();
    expect(newlyConnectedComputer([runtime({ id: "new-provider", provider: "claude" })], baseline, "user-1", "ws-1")).toBeNull();
  });

  it("does not turn missing legacy daemon identity into an apparent new computer", () => {
    const baseline = computerBaseline([runtime({ daemon_id: null })], "user-1", "ws-1");
    expect(newlyConnectedComputer([runtime({ id: "new-provider" })], baseline, "user-1", "ws-1")).toBeNull();
  });

  it("keeps existing runtime IDs in the baseline across metadata changes", () => {
    const baseline = computerBaseline([runtime()], "user-1", "ws-1");
    expect(newlyConnectedComputer([runtime({ daemon_id: "other-daemon", name: "Codex (renamed)", device_info: "" })], baseline, "user-1", "ws-1")).toBeNull();
  });

  it("does not let another member or workspace contaminate the baseline", () => {
    const baseline = computerBaseline([
      runtime({ owner_id: "other-user" }), runtime({ workspace_id: "other-workspace" }),
    ], "user-1", "ws-1");
    expect(newlyConnectedComputer([runtime()], baseline, "user-1", "ws-1")).not.toBeNull();
  });

  it("keeps hostname identity across case, daemon and provider changes", () => {
    const baseline = computerBaseline([runtime()], "user-1", "ws-1");
    expect(newlyConnectedComputer([runtime({
      id: "new-provider", daemon_id: "new-daemon", name: "Claude (office-pc)", provider: "claude",
    })], baseline, "user-1", "ws-1")).toBeNull();
  });
});
