import { webcrypto } from "node:crypto";
import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { CollaborationCreationEntry, CollaborationCreationPanel } from "./creation-form";

// Core delivery.test.ts owns the scope, malformed-response and original-key matrix.
const actor = "00000000-0000-4000-8000-000000000001";
const source = "00000000-0000-4000-8000-000000000002";
const other = "00000000-0000-4000-8000-000000000003";
const fileId = "00000000-0000-4000-8000-000000000004";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const sourceChoice = { id: source, name: "Source workspace", slug: "source" };
const options = { creation_enabled: true, sources: [sourceChoice], source: null, max_attachment_bytes: 104857600 };
const created = { ...collaborationIssueFixture, source_workspace: { ...collaborationIssueFixture.source_workspace, id: source }, handler: { ...collaborationIssueFixture.handler, user_id: other, name: "External recipient" } };
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const json = (data: unknown) => new Response(JSON.stringify(data));

function mount(onCreated = vi.fn()) {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const scope = collaborationScope(actor);
  return { ...renderWithI18n(<QueryClientProvider client={client}><CollaborationCreationPanel scope={scope} onCreated={onCreated} /></QueryClientProvider>), onCreated };
}

it("restores the form and private file after lost responses, then opens the recovered original issue", async () => {
  vi.stubGlobal("crypto", webcrypto);
  let uploaded: Record<string, unknown> | null = null;
  const uploadKeys: string[] = [], createKeys: string[] = [], recoveryPaths: string[] = [];
  const fetch = vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname.endsWith("recipients/resolve")) return json({ issue_id: null, source_workspace_id: source,
      recipient: { user_id: other, name: "External recipient", avatar_url: "", external: true } });
    if (url.pathname.endsWith("attachments/drafts")) {
      const body = init.body as FormData;
      uploadKeys.push((init.headers as Record<string, string>)["Idempotency-Key"]!);
      uploaded = { id: fileId, draft_id: body.get("draft_id"), issue_id: null, intended_issue_id: null, source_workspace_id: source, uploader_user_id: actor,
        name: "notes.txt", content_type: "text/plain", size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: null };
      throw new TypeError("upload response lost");
    }
    if (url.pathname.includes("/operations/")) { recoveryPaths.push(url.pathname); return json(uploaded); }
    if (url.pathname.includes("/attachments/drafts/")) return json({ draft_id: url.pathname.split("/").pop(), attachments: uploaded ? [uploaded] : [] });
    if (url.pathname === "/api/collaboration/issues") {
      createKeys.push((init.headers as Record<string, string>)["Idempotency-Key"]!);
      expect(JSON.parse(String(init.body))).toMatchObject({ title: "Keep this title", description: "Keep this body", handler_user_id: other, attachment_ids: [fileId] });
      throw new TypeError("creation response lost");
    }
    if (url.pathname.includes("/creations/")) { recoveryPaths.push(url.pathname); return json(created); }
    throw new Error(`Unexpected test path ${url.pathname}`);
  });
  vi.stubGlobal("fetch", fetch);
  const user = userEvent.setup();
  const onCreated = vi.fn();
  let view = mount(onCreated);
  await user.type(await screen.findByLabelText("Title"), "Keep this title");
  await user.type(screen.getByLabelText("Description"), "Keep this body");
  await user.type(screen.getByLabelText("Full username, email, or account ID"), "recipient@example.invalid");
  expect(screen.getByRole("button", { name: "Create collaboration issue" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Find account" }));
  await user.click(await screen.findByRole("button", { name: "Use this account" }));
  const file = new File(["hello"], "notes.txt", { type: "text/plain" });
  Object.defineProperty(file, "arrayBuffer", { value: async () => new TextEncoder().encode("hello").buffer });
  await user.upload(screen.getByLabelText("Attach files"), file);
  await screen.findByRole("button", { name: "Check upload" });
  await waitFor(() => expect(screen.getByRole("button", { name: "Check upload" })).toBeEnabled());
  view.unmount();
  view = mount(onCreated);
  expect(await screen.findByLabelText("Title")).toHaveValue("Keep this title");
  expect(screen.getByLabelText("Description")).toHaveValue("Keep this body");
  await user.click(await screen.findByRole("button", { name: "Check upload" }));
  await waitFor(() => expect(screen.getByRole("checkbox", { name: "Include notes.txt" })).toBeChecked());
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  await screen.findByText("The creation result is not confirmed. Your original request is saved.");
  await waitFor(() => expect(screen.getByRole("button", { name: "Check creation result" })).toBeEnabled());
  expect(onCreated).not.toHaveBeenCalled();
  view.unmount();
  mount(onCreated);
  await user.click(await screen.findByRole("button", { name: "Check creation result" }));
  await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
  expect(onCreated.mock.calls[0]![0]).toMatchObject({ id: actor });
  expect(uploadKeys).toHaveLength(1);
  expect(createKeys).toHaveLength(1);
  expect(recoveryPaths).toContain(`/api/collaboration/attachments/drafts/${uploaded!.draft_id}/operations/${uploadKeys[0]}`);
  expect(recoveryPaths).toContain(`/api/collaboration/creations/${createKeys[0]}`);
  expect(fetch.mock.calls.every(([url]) => !url.includes("/api/upload-file") && !url.includes("/uploads/"))).toBe(true);
});

it("keeps account creation entry usable with zero sources and returns keyboard focus on close", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ ...options, creation_enabled: false, sources: [] })));
  const client = new QueryClient(); clients.push(client);
  const navigation: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/account", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={navigation}><CollaborationCreationEntry scope={collaborationScope(actor)} /></NavigationProvider></QueryClientProvider>);
  const trigger = screen.getByRole("button", { name: "New collaboration issue" });
  const user = userEvent.setup();
  await user.click(trigger);
  expect(await screen.findByRole("dialog", { name: "New collaboration issue" })).toBeInTheDocument();
  expect(await screen.findByText("You need a workspace you can create issues in. Shared issues remain available here.")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Create collaboration issue" })).toBeNull();
  await user.keyboard("{Escape}");
  await waitFor(() => expect(trigger).toHaveFocus());
});

it("keeps a durably rejected form through reload and sends identical input only on explicit resubmission", async () => {
  const keys: string[] = [];
  const inputs: unknown[] = [];
  vi.stubGlobal("fetch", vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname.endsWith("recipients/resolve")) return json({ issue_id: null, source_workspace_id: source,
      recipient: { user_id: other, name: "Unavailable recipient", avatar_url: "", external: true } });
    if (url.pathname.endsWith("/result")) return json({ request_id: keys[0], source_workspace_id: source, actor_user_id: actor, outcome: "rejected", code: "account_unavailable" });
    if (url.pathname === "/api/collaboration/issues") {
      keys.push((init.headers as Record<string, string>)["Idempotency-Key"]!);
      inputs.push(JSON.parse(String(init.body)));
      if (keys.length === 1) return new Response(JSON.stringify({ code: "account_unavailable" }), { status: 403 });
      expect(inputs[1]).toEqual(inputs[0]);
      return json(created);
    }
    throw new Error(`Unexpected test path ${url.pathname}`);
  }));
  const user = userEvent.setup();
  const onCreated = vi.fn();
  const view = mount(onCreated);
  await user.type(await screen.findByLabelText("Title"), "Keep the rejected title");
  await user.type(screen.getByLabelText("Full username, email, or account ID"), "unavailable@example.invalid");
  await user.click(screen.getByRole("button", { name: "Find account" }));
  await user.click(await screen.findByRole("button", { name: "Use this account" }));
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  const rejectedMessage = "The issue was not created. Your draft and files are preserved; you can submit again.";
  await screen.findByText(rejectedMessage);
  expect(screen.getByLabelText("Title")).toBeEnabled();
  expect(screen.getByRole("button", { name: "Create collaboration issue" })).toBeEnabled();
  expect(screen.queryByRole("button", { name: "Check creation result" })).toBeNull();
  expect(onCreated).not.toHaveBeenCalled();

  view.unmount();
  mount(onCreated);
  expect(await screen.findByLabelText("Title")).toHaveValue("Keep the rejected title");
  expect(screen.getByRole("button", { name: "Create collaboration issue" })).toBeEnabled();
  expect(keys).toHaveLength(1);
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
  expect(keys).toHaveLength(2);
  expect(keys[1]).not.toBe(keys[0]);
});
