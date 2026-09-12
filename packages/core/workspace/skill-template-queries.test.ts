// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import type { Skill } from "../types";
import { cacheSkillResponse, skillDetailOptions, skillListOptions, skillTemplateListOptions, workspaceKeys } from "./queries";

vi.mock("../api", () => ({ api: { listSkills: vi.fn(), listSkillTemplates: vi.fn() } }));

afterEach(() => { vi.clearAllMocks(); });

describe("skillListOptions", () => {
  it("loads existing names from the query workspace with its cancellation signal", async () => {
    vi.mocked(api.listSkills).mockResolvedValue([]);
    const client = new QueryClient();
    try {
      await client.fetchQuery(skillListOptions("ws-original"));
      expect(api.listSkills).toHaveBeenCalledWith({
        workspaceId: "ws-original",
        signal: expect.any(AbortSignal),
      });
    } finally {
      client.clear();
    }
  });
});

describe("skillTemplateListOptions", () => {
  it("keeps catalog caches separated by workspace", () => {
    expect(workspaceKeys.skillTemplates("ws-a")).toEqual(["workspaces", "ws-a", "skill-templates"]);
    expect(skillTemplateListOptions("ws-a").queryKey).not.toEqual(
      skillTemplateListOptions("ws-b").queryKey,
    );
    expect(skillTemplateListOptions("").enabled).toBe(false);
  });

  it("passes the query cancellation signal and captured workspace to the API", async () => {
    vi.mocked(api.listSkillTemplates).mockResolvedValue([]);
    const client = new QueryClient();
    try {
      await client.fetchQuery(skillTemplateListOptions("ws-original"));
      expect(api.listSkillTemplates).toHaveBeenCalledWith("ws-original", expect.any(AbortSignal));
    } finally {
      client.clear();
    }
  });
});

describe("cacheSkillResponse", () => {
  it("seeds the detail and invalidates projections only in the originating workspace", () => {
    const client = new QueryClient();
    const skill: Skill = {
      id: "skill-copy",
      workspace_id: "ws-original",
      name: "review-copy",
      description: "Review a change",
      content: "# Review",
      config: {},
      files: [],
      created_by: "creator",
      created_at: "2026-09-12T00:00:00Z",
      updated_at: "2026-09-12T00:00:00Z",
    };
    try {
      for (const wsId of ["ws-original", "ws-another"]) {
        client.setQueryData(workspaceKeys.skills(wsId), []);
        client.setQueryData(workspaceKeys.agents(wsId), []);
      }

      cacheSkillResponse(client, "ws-original", skill);

      expect(client.getQueryData(skillDetailOptions("ws-original", skill.id).queryKey)).toBe(skill);
      expect(client.getQueryState(workspaceKeys.skills("ws-original"))?.isInvalidated).toBe(true);
      expect(client.getQueryState(workspaceKeys.agents("ws-original"))?.isInvalidated).toBe(true);
      expect(client.getQueryState(workspaceKeys.skills("ws-another"))?.isInvalidated).toBe(false);
      expect(client.getQueryState(workspaceKeys.agents("ws-another"))?.isInvalidated).toBe(false);
    } finally {
      client.clear();
    }
  });
});
