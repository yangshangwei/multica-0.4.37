// @vitest-environment jsdom

import { focusManager, onlineManager, QueryObserver, type QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import { createQueryClient } from "../query-client";
import { changelogKeys, changelogOptions } from "./queries";
import type { ChangelogFeed } from "./types";

function response(title: string, stale = false): Response {
  return new Response(JSON.stringify({
    schema_version: 1,
    generated_at: "2026-09-01T00:00:00Z",
    releases: [{ id: "fork:example/multica:unreleased", title, version: "Unreleased", source: "fork", status: "unreleased", published_at: null, commit: "1".repeat(40), base_commit: "0".repeat(40), sections: [] }],
    is_stale: stale,
    warning: stale ? "changelog_file_invalid" : null,
  }));
}

describe("changelog live Query observers", () => {
  let client: QueryClient;
  let fetch: ReturnType<typeof vi.fn>;
  let observer: QueryObserver<ChangelogFeed, Error, ChangelogFeed, ChangelogFeed, ReturnType<typeof changelogKeys.feed>>;
  let unsubscribe: () => void;

  async function settle() {
    await vi.advanceTimersByTimeAsync(1);
  }

  beforeEach(() => {
    vi.useFakeTimers();
    focusManager.setFocused(true);
    onlineManager.setOnline(true);
    fetch = vi.fn().mockImplementation(() => Promise.resolve(response("First")));
    vi.stubGlobal("fetch", fetch);
    setApiInstance(new ApiClient("https://one.example.test"));
    client = createQueryClient();
    client.mount();
    observer = new QueryObserver(client, changelogOptions());
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
    vi.unstubAllGlobals();
  });

  it("loads first-open content then discovers new data in the same observer within 60 seconds", async () => {
    await settle();
    expect(observer.getCurrentResult().data?.releases[0]?.title).toBe("First");
    fetch.mockImplementation(() => Promise.resolve(response("New publication")));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(observer.getCurrentResult().data?.releases[0]?.title).toBe("New publication");
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("refetches on focus, reconnect, manual refresh and reopening despite global Infinity", async () => {
    await settle();
    focusManager.setFocused(false);
    focusManager.setFocused(true);
    await settle();
    expect(fetch).toHaveBeenCalledTimes(2);
    onlineManager.setOnline(false);
    onlineManager.setOnline(true);
    await settle();
    expect(fetch).toHaveBeenCalledTimes(3);
    await observer.refetch();
    expect(fetch).toHaveBeenCalledTimes(4);
    unsubscribe();
    unsubscribe = observer.subscribe(() => {});
    await settle();
    expect(fetch).toHaveBeenCalledTimes(5);
  });

  it("does not poll hidden or inactive views, then immediately refreshes a kept-alive view on activation", async () => {
    await settle();
    focusManager.setFocused(false);
    await vi.advanceTimersByTimeAsync(120_000);
    expect(fetch).toHaveBeenCalledTimes(1);
    observer.setOptions(changelogOptions(false));
    focusManager.setFocused(true);
    await vi.advanceTimersByTimeAsync(120_000);
    expect(fetch).toHaveBeenCalledTimes(1);
    observer.setOptions(changelogOptions(true));
    await settle();
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it("retains successful content on network and malformed refreshes, and reports server stale state", async () => {
    await settle();
    fetch.mockRejectedValue(new Error("offline"));
    await observer.refetch();
    expect(observer.getCurrentResult().isError).toBe(true);
    expect(observer.getCurrentResult().data?.releases[0]?.title).toBe("First");
    fetch.mockImplementation(() => Promise.resolve(new Response('{"releases":[]}')));
    await observer.refetch();
    expect(observer.getCurrentResult().isError).toBe(true);
    expect(observer.getCurrentResult().data?.releases[0]?.title).toBe("First");
    fetch.mockImplementation(() => Promise.resolve(response("Last valid file", true)));
    await observer.refetch();
    expect(observer.getCurrentResult().data?.isStale).toBe(true);
    expect(observer.getCurrentResult().data?.warning).toBe("changelog_file_invalid");
  });

  it("isolates deployments, including in-flight work from the previous API instance", async () => {
    await settle();
    const previousOptions = changelogOptions();
    setApiInstance(new ApiClient("https://two.example.test"));
    fetch.mockImplementation(() => Promise.resolve(response("Other deployment")));
    observer.setOptions(changelogOptions());
    expect(observer.getCurrentResult().data).toBeUndefined();
    await settle();
    expect(fetch.mock.calls.at(-1)?.[0]).toBe("https://two.example.test/api/changelog");
    expect(observer.getCurrentResult().data?.releases[0]?.title).toBe("Other deployment");
    expect(client.getQueryData<ChangelogFeed>(previousOptions.queryKey)?.releases[0]?.title).toBe("First");
    await client.fetchQuery(previousOptions);
    expect(fetch.mock.calls.at(-1)?.[0]).toBe("https://one.example.test/api/changelog");
  });
});
