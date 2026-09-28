// @vitest-environment jsdom

import { focusManager, onlineManager, QueryObserver, type QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { createQueryClient } from "../query-client";
import type { SkillTemplate } from "../types";
import { skillTemplateListOptions, workspaceKeys } from "./queries";

vi.mock("../api", () => ({ api: { listSkillTemplates: vi.fn() } }));

function template(description: string): SkillTemplate {
  return { name: "team-review", description, version: 0, content: `# ${description}`, files: [] };
}

describe("skill template live Query observers", () => {
  let client: QueryClient;
  let observer: QueryObserver<SkillTemplate[], Error, SkillTemplate[], SkillTemplate[], ReturnType<typeof workspaceKeys.skillTemplates>>;
  let unsubscribe: () => void;

  async function settle() {
    await vi.advanceTimersByTimeAsync(1);
  }

  beforeEach(() => {
    vi.useFakeTimers();
    focusManager.setFocused(true);
    onlineManager.setOnline(true);
    vi.mocked(api.listSkillTemplates).mockResolvedValue([]);
    client = createQueryClient();
    client.mount();
    observer = new QueryObserver(client, skillTemplateListOptions("ws-1", { poll: true }));
    unsubscribe = observer.subscribe(() => {});
  });

  afterEach(() => {
    unsubscribe();
    observer.destroy();
    client.unmount();
    client.clear();
    focusManager.setFocused(undefined);
    onlineManager.setOnline(true);
    vi.useRealTimers();
    vi.resetAllMocks();
  });

  it("discovers additions, edits and removals within 30 seconds in the same picker", async () => {
    await settle();
    expect(observer.getCurrentResult().data).toEqual([]);
    for (const catalog of [[template("Added")], [template("Edited")], []]) {
      vi.mocked(api.listSkillTemplates).mockResolvedValue(catalog);
      await vi.advanceTimersByTimeAsync(30_000);
      expect(observer.getCurrentResult().data).toEqual(catalog);
    }
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(4);
  });

  it.each(["reopen", "focus", "reconnect"])("refreshes on %s despite global infinite freshness", async (trigger) => {
    await settle();
    vi.mocked(api.listSkillTemplates).mockResolvedValue([template("Newly mounted")]);
    if (trigger === "reopen") {
      unsubscribe();
      unsubscribe = observer.subscribe(() => {});
    } else if (trigger === "focus") {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    } else {
      onlineManager.setOnline(false);
      onlineManager.setOnline(true);
    }
    await settle();
    expect(observer.getCurrentResult().data).toEqual([template("Newly mounted")]);
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(2);
  });

  it("does not poll the entry observer unless browsing opts in", async () => {
    await settle();
    observer.setOptions(skillTemplateListOptions("ws-1"));
    await vi.advanceTimersByTimeAsync(90_000);
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(1);
  });

  it("suspends polling while hidden or editing, refreshes on returning, and stops on unmount", async () => {
    await settle();
    focusManager.setFocused(false);
    await vi.advanceTimersByTimeAsync(90_000);
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(1);
    focusManager.setFocused(true);
    await settle();
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(2);

    observer.setOptions({ ...skillTemplateListOptions("ws-1", { poll: false }), enabled: false });
    await vi.advanceTimersByTimeAsync(90_000);
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(2);
    observer.setOptions(skillTemplateListOptions("ws-1", { poll: true }));
    await settle();
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(3);

    unsubscribe();
    await vi.advanceTimersByTimeAsync(90_000);
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(3);
  });

  it("keeps successful data through failed refresh and retry, then recovers on the next poll", async () => {
    await settle();
    vi.mocked(api.listSkillTemplates).mockResolvedValue([template("Cached")]);
    await observer.refetch();
    vi.mocked(api.listSkillTemplates).mockRejectedValue(new Error("offline"));
    await vi.advanceTimersByTimeAsync(31_001);
    expect(observer.getCurrentResult().isError).toBe(true);
    expect(observer.getCurrentResult().data).toEqual([template("Cached")]);
    expect(api.listSkillTemplates).toHaveBeenCalledTimes(4);

    vi.mocked(api.listSkillTemplates).mockResolvedValue([template("Recovered")]);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(observer.getCurrentResult().isSuccess).toBe(true);
    expect(observer.getCurrentResult().data).toEqual([template("Recovered")]);
  });
});
