// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

vi.mock("../platform/workspace-storage", async (importOriginal) => ({
  ...await importOriginal<typeof import("../platform/workspace-storage")>(),
  getCurrentSlug: () => "other-active-workspace",
}));

afterEach(() => vi.unstubAllGlobals());

const workspaceId = "11111111-1111-4111-8111-111111111111";
const groupId = "22222222-2222-4222-8222-222222222222";
const itemId = "33333333-3333-4333-8333-333333333333";
const until = "2027-01-01T09:00:00Z";
const receipt = {
  workspace_id: workspaceId,
  group_kind: "issue",
  group_id: groupId,
  revision: 4,
  snoozed_until: until,
  reminded_at: null,
  undo_token: "opaque-server-token",
};
const unsnoozeReceipt = {
  ...receipt,
  snoozed_until: null,
  reminded_at: "2026-11-21T09:00:00Z",
};

function respond(value: unknown) {
  const fetchMock = vi.fn().mockImplementation(async () => new Response(JSON.stringify(value), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  }));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("Inbox snooze API boundary", () => {
  it("sends a captured workspace and exact snooze version/time before returning its receipt", async () => {
    const fetchMock = respond(receipt);
    const client = new ApiClient("https://api.example.test");

    await expect(client.snoozeInboxItem(itemId, {
      snoozed_until: until, expected_revision: 3,
    }, { workspaceId })).resolves.toEqual(receipt);

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(url).toBe(`https://api.example.test/api/inbox/${itemId}/snooze`);
    expect(init).toMatchObject({ method: "POST" });
    expect(JSON.parse(String(init?.body))).toEqual({ snoozed_until: until, expected_revision: 3 });
    expect(new Headers(init?.headers).get("X-Workspace-ID")).toBe(workspaceId);
    expect(new Headers(init?.headers).get("X-Workspace-Slug")).toBe("");
  });

  it("uses dedicated now-reminder and undo endpoints", async () => {
    const fetchMock = respond(unsnoozeReceipt);
    const client = new ApiClient("https://api.example.test");
    await client.unsnoozeInboxItem(itemId, { expected_revision: 3 }, { workspaceId });
    await client.undoInboxSnooze(itemId, { undo_token: "opaque-server-token" }, { workspaceId });

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      `https://api.example.test/api/inbox/${itemId}/unsnooze`,
      `https://api.example.test/api/inbox/${itemId}/snooze/undo`,
    ]);
    expect(fetchMock.mock.calls.map(([, init]) => JSON.parse(String(init?.body)))).toEqual([
      { expected_revision: 3 }, { undo_token: "opaque-server-token" },
    ]);
  });

  it.each([0, 2, 3])("rejects a snooze receipt with non-advancing revision %s", async (revision) => {
    respond({ ...receipt, revision });
    await expect(new ApiClient("https://api.example.test").snoozeInboxItem(itemId, {
      snoozed_until: until, expected_revision: 3,
    }, { workspaceId })).rejects.toThrow(/invalid inbox snooze response/i);
  });

  it.each([null, "2027-01-01T10:00:00Z", "2027-01-01T09:00:00.001Z", "2027-01-01T09:00:00.000001Z"])(
    "rejects a snooze receipt that does not confirm its requested instant %s",
    async (snoozedUntil) => {
      respond({ ...receipt, snoozed_until: snoozedUntil });
      await expect(new ApiClient("https://api.example.test").snoozeInboxItem(itemId, {
        snoozed_until: until, expected_revision: 3,
      }, { workspaceId })).rejects.toThrow(/invalid inbox snooze response/i);
    },
  );

  it("accepts the same snooze instant expressed with a different timezone offset", async () => {
    respond(receipt);
    await expect(new ApiClient("https://api.example.test").snoozeInboxItem(itemId, {
      snoozed_until: "2027-01-01T10:00:00+01:00", expected_revision: 3,
    }, { workspaceId })).resolves.toEqual(receipt);
  });

  it("accepts equivalent fractional seconds without discarding their precision", async () => {
    const precise = { ...receipt, snoozed_until: "2027-01-01T09:00:00.123456Z" };
    respond(precise);
    await expect(new ApiClient("https://api.example.test").snoozeInboxItem(itemId, {
      snoozed_until: "2027-01-01T10:00:00.123456000+01:00", expected_revision: 3,
    }, { workspaceId })).resolves.toEqual(precise);
  });

  it.each([0, 2, 3])("rejects a now-reminder receipt with non-advancing revision %s", async (revision) => {
    respond({ ...unsnoozeReceipt, revision });
    await expect(new ApiClient("https://api.example.test").unsnoozeInboxItem(itemId, {
      expected_revision: 3,
    }, { workspaceId })).rejects.toThrow(/invalid inbox snooze response/i);
  });

  it.each([
    { ...unsnoozeReceipt, snoozed_until: until },
    { ...unsnoozeReceipt, reminded_at: null },
    { ...unsnoozeReceipt, reminded_at: "invalid-time" },
  ])("rejects a now-reminder receipt without a confirmed active reminder", async (response) => {
    respond(response);
    await expect(new ApiClient("https://api.example.test").unsnoozeInboxItem(itemId, {
      expected_revision: 3,
    }, { workspaceId })).rejects.toThrow(/invalid inbox snooze response/i);
  });

  it("requires a positive undo revision without guessing the restored deadline", async () => {
    const client = new ApiClient("https://api.example.test");
    respond({ ...receipt, revision: 0 });
    await expect(client.undoInboxSnooze(itemId, { undo_token: "opaque-server-token" }, { workspaceId }))
      .rejects.toThrow(/invalid inbox snooze response/i);

    const restored = { ...receipt, revision: 5, snoozed_until: "2027-01-02T09:00:00Z" };
    respond(restored);
    await expect(client.undoInboxSnooze(itemId, { undo_token: "opaque-server-token" }, { workspaceId }))
      .resolves.toEqual(restored);
  });

  it.each([
    {},
    { ...receipt, group_id: "" },
    { ...receipt, revision: 1.5 },
    { ...receipt, snoozed_until: "invalid-time" },
    { ...receipt, workspace_id: "44444444-4444-4444-8444-444444444444" },
  ])("rejects an unconfirmed or wrong-workspace success receipt", async (response) => {
    respond(response);
    await expect(new ApiClient("https://api.example.test").snoozeInboxItem(itemId, {
      snoozed_until: until, expected_revision: 3,
    }, { workspaceId })).rejects.toThrow(/invalid inbox snooze response/i);
  });

  it("reads the snoozed list in its captured workspace and preserves optional projections", async () => {
    const row = {
      id: itemId, workspace_id: workspaceId, recipient_id: "member-1", recipient_type: "member",
      type: "new_comment", severity: "info", issue_id: groupId, title: "Saved notification",
      read: true, archived: false, created_at: "2026-11-01T08:00:00Z",
      snoozed_until: until, snooze_revision: 4, reminded_at: null,
    };
    const fetchMock = respond([row]);
    await expect(new ApiClient("https://api.example.test").listSnoozedInbox({ workspaceId })).resolves.toEqual([row]);
    expect(new Headers(fetchMock.mock.calls[0]?.[1]?.headers).get("X-Workspace-ID")).toBe(workspaceId);
  });

  it("does not accept malformed optional snooze fields as valid list state", async () => {
    respond([{
      id: itemId, workspace_id: workspaceId, recipient_id: "member-1", recipient_type: "member",
      type: "new_comment", severity: "info", title: "Saved notification", read: true,
      archived: false, created_at: "2026-11-01T08:00:00Z", snoozed_until: 123,
    }]);
    await expect(new ApiClient("https://api.example.test").listInbox()).resolves.toEqual([]);
  });

  it("keeps a malformed snoozed-list response distinct from a genuinely empty list", async () => {
    respond({ items: [] });
    await expect(new ApiClient("https://api.example.test").listSnoozedInbox({ workspaceId }))
      .rejects.toThrow(/invalid snoozed inbox response/i);
  });
});
