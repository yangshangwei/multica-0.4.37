import { describe, it, expect } from "vitest";
import { paths, isGlobalPath } from "./paths";

describe("paths.workspace(slug)", () => {
  const ws = paths.workspace("acme");

  it("builds workspace paths with slug prefix", () => {
    expect(ws.usage()).toBe("/acme/usage");
    expect(ws.issues()).toBe("/acme/issues");
    expect(ws.issueDetail("abc-123")).toBe("/acme/issues/abc-123");
    expect(ws.projects()).toBe("/acme/projects");
    expect(ws.projectDetail("p1")).toBe("/acme/projects/p1");
    expect(ws.autopilots()).toBe("/acme/autopilots");
    expect(ws.autopilotDetail("a1")).toBe("/acme/autopilots/a1");
    expect(ws.agents()).toBe("/acme/agents");
    expect(ws.newAgent()).toBe("/acme/agents/new");
    expect(ws.newAgentTemplate()).toBe("/acme/agents/new/template");
    expect(ws.agentApprovals()).toBe("/acme/agents/approvals");
    expect(ws.newAgentAi()).toBe("/acme/agents/new/ai");
    expect(ws.newAgentAiSession("sess_1")).toBe("/acme/agents/new/ai/sess_1");
    expect(ws.memberDetail("u1")).toBe("/acme/members/u1");
    expect(ws.inbox()).toBe("/acme/inbox");
    expect(ws.chatWithAgent("agent one")).toBe(
      "/acme/chat?agent=agent%20one",
    );
    expect(ws.chatSession("session one")).toBe(
      "/acme/chat?session=session%20one",
    );
    expect(ws.myIssues()).toBe("/acme/my-issues");
    expect(ws.runtimes()).toBe("/acme/runtimes");
    expect(ws.runtimeSettings("machine/runtime", "runtime one")).toBe(
      "/acme/runtimes/machine%2Fruntime/runtime/runtime%20one",
    );
    expect(ws.skills()).toBe("/acme/skills");
    expect(ws.skillDetail("skl_123")).toBe("/acme/skills/skl_123");
    expect(ws.squads()).toBe("/acme/squads");
    expect(ws.squadDetail("sq_1")).toBe("/acme/squads/sq_1");
    expect(ws.settings()).toBe("/acme/settings");
    expect(ws.attachmentPreview("att_42")).toBe("/acme/attachments/att_42/preview");
  });

  it("URL-encodes special characters in ids", () => {
    expect(ws.issueDetail("id with space")).toBe("/acme/issues/id%20with%20space");
  });

  it("builds docs paths", () => {
    expect(ws.docs()).toBe("/acme/docs");
    expect(ws.docsPage("agents")).toBe("/acme/docs/agents");
  });

  // The bundle nests pages under a directory, so "/" inside a docs slug is a
  // route separator. Encoding the whole slug would emit %2F and the route
  // would not match.
  it("keeps the separator in a nested docs slug", () => {
    expect(ws.docsPage("developers/contributing")).toBe(
      "/acme/docs/developers/contributing",
    );
  });

  // Heading ids come from github-slugger, which leaves non-ASCII text as-is.
  // The builder encodes so a Chinese anchor survives the URL.
  it("encodes a docs anchor", () => {
    expect(ws.docsPage("daemon-runtimes", "自定义运行时配置")).toBe(
      `/acme/docs/daemon-runtimes#${encodeURIComponent("自定义运行时配置")}`,
    );
    expect(ws.docsPage("autopilots", "event-filters")).toBe(
      "/acme/docs/autopilots#event-filters",
    );
  });
});

describe("paths (global)", () => {
  it("builds global paths without slug", () => {
    expect(paths.login()).toBe("/login");
    expect(paths.newWorkspace()).toBe("/workspaces/new");
    expect(paths.invite("inv-1")).toBe("/invite/inv-1");
    expect(paths.authCallback()).toBe("/auth/callback");
  });
});

describe("isGlobalPath", () => {
  it("returns true for pre-workspace routes", () => {
    expect(isGlobalPath("/login")).toBe(true);
    expect(isGlobalPath("/workspaces/new")).toBe(true);
    expect(isGlobalPath("/invite/abc")).toBe(true);
    expect(isGlobalPath("/auth/callback")).toBe(true);
  });

  it("returns false for workspace-scoped paths", () => {
    expect(isGlobalPath("/acme/issues")).toBe(false);
    expect(isGlobalPath("/")).toBe(false);
  });
});
