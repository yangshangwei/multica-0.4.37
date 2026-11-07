import { webcrypto } from "node:crypto";
import { afterEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, saveCollaborationDeliveryRecord } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { CollaborationCreationEntry, CollaborationCreationPanel } from "./creation-form";

// Core delivery.test.ts owns the scope, malformed-response and original-key matrix.
const actor = "00000000-0000-4000-8000-000000000001";
const source = "00000000-0000-4000-8000-000000000002";
const other = "00000000-0000-4000-8000-000000000003";
const fileId = "00000000-0000-4000-8000-000000000004";
const otherSource = "00000000-0000-4000-8000-000000000007";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const sourceChoice = { id: source, name: "Source workspace", slug: "source" };
const options = { creation_enabled: true, sources: [sourceChoice], source: null, max_attachment_bytes: 104857600 };
const created = { ...collaborationIssueFixture, source_workspace: { ...collaborationIssueFixture.source_workspace, id: source }, handler: { ...collaborationIssueFixture.handler, user_id: other, name: "External recipient" } };
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const json = (data: unknown) => new Response(JSON.stringify(data));
const releaseParent = { id: "00000000-0000-4000-8000-000000000005", identifier: "SRC-12", name: "Prepare the release" };
const designParent = { id: "00000000-0000-4000-8000-000000000006", identifier: "SRC-11", name: "Review the design" };
function parentOptions(parents = [releaseParent, designParent]) {
  return { ...options, source: sourceChoice, fields: { labels: [], properties: [], statuses: [{ key: "todo", name: "Todo", category: "todo" }], default_status: "todo",
    source_options: { projects: [], parents, parents_truncated: false, iterations: [], iterations_enabled: false } } };
}

async function chooseExactRecipient(user: ReturnType<typeof userEvent.setup>, value: string) {
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  await user.type(screen.getByPlaceholderText("Search recipients"), "recipient");
  await user.click(await screen.findByRole("button", { name: "Find by full username, email, or account ID" }));
  await user.type(screen.getByLabelText("Find by full username, email, or account ID"), value);
  await user.click(screen.getByRole("button", { name: "Find account" }));
  await user.click(await screen.findByRole("button", { name: "Use this account" }));
}

function mount(onCreated = vi.fn(), sourceWorkspaceId?: string, locale: "en" | "zh-Hans" = "en", constrained = false) {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const scope = collaborationScope(actor);
  return { ...renderWithI18n(<QueryClientProvider client={client}><CollaborationCreationPanel scope={scope} sourceWorkspaceId={sourceWorkspaceId} onCreated={onCreated} constrained={constrained} /></QueryClientProvider>, { locale }), onCreated, client };
}

it("preserves the manual policy choice with the creation draft and sends it explicitly", async () => {
  const fetch = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>(async (url) => new URL(url).pathname.endsWith("creation-options")
    ? json({ ...options, source: new URL(url).searchParams.has("source_workspace_id") ? sourceChoice : null })
    : json({ ...created, ai_policy: "manual" }));
  vi.stubGlobal("fetch", fetch);
  const user = userEvent.setup();
  const initial = mount(vi.fn(), source);
  expect(await screen.findByLabelText("AI participation")).toHaveValue("assisted");
  await user.selectOptions(screen.getByLabelText("AI participation"), "manual");
  await user.type(screen.getByLabelText("Title"), "A manual collaboration");
  initial.unmount();
  const current = mount(vi.fn(), source);
  expect(await screen.findByLabelText("AI participation")).toHaveValue("manual");
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  await waitFor(() => expect(current.onCreated).toHaveBeenCalledWith(expect.objectContaining({ ai_policy: "manual" })));
  const write = fetch.mock.calls.find(([url]) => new URL(url).pathname === "/api/collaboration/issues");
  expect(JSON.parse(String(write?.[1]?.body))).toMatchObject({ title: "A manual collaboration", ai_policy: "manual" });
});

it("locks the source through delayed creation and completes the original form once", async () => {
  let finishCreate!: (response: Response) => void;
  const keys: string[] = [];
  const sources = [sourceChoice, { id: otherSource, name: "Other workspace", slug: "other" }];
  vi.stubGlobal("fetch", vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, sources, source: sources.find((item) => item.id === url.searchParams.get("source_workspace_id")) ?? null });
    if (url.pathname === "/api/collaboration/issues") {
      keys.push((init.headers as Record<string, string>)["Idempotency-Key"]!);
      return new Promise<Response>((resolve) => { finishCreate = resolve; });
    }
    throw new Error(`Unexpected test path ${url.pathname}`);
  }));
  const user = userEvent.setup();
  const { onCreated } = mount(vi.fn(), source);
  await user.type(await screen.findByLabelText("Title"), "Complete the original source");
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  await waitFor(() => expect(keys).toHaveLength(1));
  const sourcePicker = screen.getByLabelText("Source workspace");
  expect(sourcePicker).toBeDisabled();
  fireEvent.change(sourcePicker, { target: { value: otherSource } });
  expect(sourcePicker).toHaveValue(source);
  expect(screen.getByLabelText("Title")).toHaveValue("Complete the original source");
  await act(async () => { finishCreate(json(created)); });
  await waitFor(() => expect(onCreated).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ id: created.id })));
  expect(keys).toHaveLength(1);
});

it("keeps an unknown request on its source and locks original-key recovery until rejection settles", async () => {
  let finishRecovery!: (response: Response) => void;
  const keys: string[] = [], recovered: string[] = [];
  const sources = [sourceChoice, { id: otherSource, name: "Other workspace", slug: "other" }];
  vi.stubGlobal("fetch", vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, sources, source: sources.find((item) => item.id === url.searchParams.get("source_workspace_id")) ?? null });
    if (url.pathname === "/api/collaboration/issues") { keys.push((init.headers as Record<string, string>)["Idempotency-Key"]!); throw new TypeError("Response lost"); }
    if (url.pathname.endsWith("/result")) return json({ request_id: keys[0], source_workspace_id: source, actor_user_id: actor, outcome: "rejected", code: "account_unavailable" });
    if (url.pathname.includes("/creations/")) {
      recovered.push(url.pathname);
      return new Promise<Response>((resolve) => { finishRecovery = resolve; });
    }
    throw new Error(`Unexpected test path ${url.pathname}`);
  }));
  const user = userEvent.setup();
  const view = mount(vi.fn(), source);
  await user.type(await screen.findByLabelText("Title"), "Keep the uncertain intention");
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  const recover = await screen.findByRole("button", { name: "Check creation result" });
  await waitFor(() => expect(recover).toBeEnabled());
  expect(screen.getByLabelText("Source workspace")).toBeDisabled();
  view.unmount();
  const { onCreated } = mount(vi.fn(), otherSource);
  expect(await screen.findByLabelText("Title")).toHaveValue("Keep the uncertain intention");
  expect(screen.getByLabelText("Source workspace")).toHaveValue(source);
  await user.click(screen.getByRole("button", { name: "Check creation result" }));
  await waitFor(() => expect(recovered).toEqual([`/api/collaboration/creations/${keys[0]}`]));
  expect(screen.getByLabelText("Source workspace")).toBeDisabled();
  await act(async () => { finishRecovery(new Response(JSON.stringify({ code: "account_unavailable" }), { status: 403 })); });
  await waitFor(() => expect(screen.getByLabelText("Source workspace")).toBeEnabled());
  expect(screen.getByLabelText("Title")).toHaveValue("Keep the uncertain intention");
  expect(keys).toHaveLength(1);
  expect(onCreated).not.toHaveBeenCalled();
});

it("allows another source beside idle content-free recovery but locks its active GET", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  saveCollaborationDeliveryRecord({ kind: "creation", scope: { server: scope.server, userId: actor, context: { kind: "creation", sourceWorkspaceId: source } },
    draftId: fileId, requestId: other, createdAt: 1, input: null });
  let finishRecovery!: (response: Response) => void;
  const recovered: string[] = [];
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, sources: [{ id: otherSource, name: "Other workspace", slug: "other" }],
      source: url.searchParams.has("source_workspace_id") ? { id: otherSource, name: "Other workspace", slug: "other" } : null });
    if (url.pathname.includes("/creations/")) {
      recovered.push(url.pathname);
      return new Promise<Response>((resolve) => { finishRecovery = resolve; });
    }
    throw new Error(`Unexpected test path ${url.pathname}`);
  }));
  const user = userEvent.setup();
  const { onCreated } = mount(vi.fn(), otherSource);
  await screen.findByLabelText("Title");
  const sourcePicker = screen.getByLabelText("Source workspace");
  expect(sourcePicker).toBeEnabled();
  await user.click(screen.getByRole("button", { name: "Check creation result" }));
  await waitFor(() => expect(recovered).toEqual([`/api/collaboration/creations/${other}`]));
  expect(sourcePicker).toBeDisabled();
  await act(async () => { finishRecovery(json(created)); });
  await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
  expect(sourcePicker).toBeEnabled();
});

it("restores the form and private file after lost responses, then opens the recovered original issue", async () => {
  vi.stubGlobal("crypto", webcrypto);
  let uploaded: Record<string, unknown> | null = null;
  const uploadKeys: string[] = [], createKeys: string[] = [], recoveryPaths: string[] = [];
  const fetch = vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname.endsWith("recipients/search")) return json({ issue_id: null, source_workspace_id: source, scope: "directory", source_member: true, recent: [], recipients: [], next_cursor: null });
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
  await chooseExactRecipient(user, "recipient@example.invalid");
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
  await user.click(trigger);
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(trigger).toHaveFocus());
});

it("keeps a durably rejected form through reload and sends identical input only on explicit resubmission", async () => {
  const keys: string[] = [];
  const inputs: unknown[] = [];
  vi.stubGlobal("fetch", vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname.endsWith("recipients/search")) return json({ issue_id: null, source_workspace_id: source, scope: "directory", source_member: true, recent: [], recipients: [], next_cursor: null });
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
  await chooseExactRecipient(user, "unavailable@example.invalid");
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  const rejectedMessage = "The issue was not created. Your draft and files are preserved.";
  await screen.findByText(rejectedMessage);
  expect(screen.getByText("The selected recipient is unavailable. Choose another account.")).toBeVisible();
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

it("opens real source-option candidates and ignores a late previous search without submitting the creation form", async () => {
  let finishOldSearch!: (response: Response) => void;
  const requested: URL[] = [];
  const fetch = vi.fn(async (input: string) => {
    const url = new URL(input);
    requested.push(url);
    if (!url.pathname.endsWith("creation-options")) throw new Error(`Unexpected test path ${url.pathname}`);
    if (!url.searchParams.has("source_workspace_id")) return json(options);
    expect(url.searchParams.get("source_workspace_id")).toBe(source);
    const search = url.searchParams.get("parent_search");
    if (search === "release") return new Promise<Response>((resolve) => { finishOldSearch = resolve; });
    return json(parentOptions(search === "design" ? [designParent] : [releaseParent, designParent]));
  });
  vi.stubGlobal("fetch", fetch);
  const user = userEvent.setup();
  const { onCreated } = mount();
  await user.type(await screen.findByLabelText("Title"), "Keep this creation title");
  await user.type(screen.getByLabelText("Description"), "Keep the description");
  await user.click(screen.getByText("Initial fields", { exact: true }));
  const trigger = screen.getByRole("button", { name: "Parent issue" });
  await user.click(trigger);
  expect(await screen.findByRole("button", { name: "SRC-12 Prepare the release" })).toBeVisible();
  const search = screen.getByPlaceholderText("Find source issues by title or number");
  fireEvent.change(search, { target: { value: "release" } });
  await screen.findByText("Loading tasks...");
  await waitFor(() => expect(requested.some((url) => url.searchParams.get("parent_search") === "release")).toBe(true));
  expect(screen.queryByRole("button", { name: "SRC-11 Review the design" })).not.toBeInTheDocument();
  fireEvent.change(search, { target: { value: "design" } });
  expect(await screen.findByRole("button", { name: "SRC-11 Review the design" })).toBeVisible();
  await act(async () => { finishOldSearch(json(parentOptions([releaseParent]))); });
  expect(screen.queryByRole("button", { name: "SRC-12 Prepare the release" })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "SRC-11 Review the design" }));
  expect(trigger).toHaveTextContent("SRC-11 Review the design");
  expect(screen.getByLabelText("Title")).toHaveValue("Keep this creation title");
  expect(screen.getByLabelText("Description")).toHaveValue("Keep the description");
  expect(requested.some((url) => url.searchParams.get("parent_search") === "release")).toBe(true);
  expect(requested.some((url) => url.searchParams.get("parent_search") === "design")).toBe(true);
  expect(onCreated).not.toHaveBeenCalled();
  expect(requested.every((url) => url.pathname.endsWith("creation-options"))).toBe(true);
});

it("keeps the parent picker and draft mounted after a failed search and retries that search", async () => {
  let failSearch = true;
  const searches: string[] = [];
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    const url = new URL(input);
    if (!url.pathname.endsWith("creation-options")) throw new Error(`Unexpected test path ${url.pathname}`);
    if (!url.searchParams.has("source_workspace_id")) return json(options);
    const search = url.searchParams.get("parent_search") ?? "";
    searches.push(search);
    if (search === "release" && failSearch) return new Response(JSON.stringify({ error: "Temporary failure" }), { status: 500 });
    return json(parentOptions(search ? [releaseParent] : [releaseParent, designParent]));
  }));
  const user = userEvent.setup();
  mount();
  await user.type(await screen.findByLabelText("Title"), "Keep this draft through retry");
  await user.click(screen.getByText("Initial fields", { exact: true }));
  await user.click(screen.getByRole("button", { name: "Parent issue" }));
  const search = await screen.findByPlaceholderText("Find source issues by title or number");
  fireEvent.change(search, { target: { value: "release" } });
  expect(await screen.findByText("Tasks could not be loaded. Try again.")).toBeVisible();
  expect(search).toBeInTheDocument();
  expect(search).toHaveValue("release");
  expect(screen.queryByRole("button", { name: "SRC-12 Prepare the release" })).not.toBeInTheDocument();
  expect(screen.getByLabelText("Title")).toHaveValue("Keep this draft through retry");
  failSearch = false;
  await user.click(screen.getByRole("button", { name: "Retry loading tasks" }));
  expect(await screen.findByRole("button", { name: "SRC-12 Prepare the release" })).toBeVisible();
  expect(searches.filter((value) => value === "release")).toHaveLength(2);
  await user.click(screen.getByRole("button", { name: "SRC-12 Prepare the release" }));
  expect(screen.getByRole("button", { name: "Parent issue" })).toHaveTextContent("SRC-12 Prepare the release");
  expect(screen.getByLabelText("Title")).toHaveValue("Keep this draft through retry");
});

it("does not reuse retained parent options after authoritative source-access denial", async () => {
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    const url = new URL(input);
    if (!url.pathname.endsWith("creation-options")) throw new Error(`Unexpected test path ${url.pathname}`);
    if (!url.searchParams.has("source_workspace_id")) return json(options);
    return url.searchParams.has("parent_search") ? new Response(JSON.stringify({ code: "source_access_denied" }), { status: 403 }) : json(parentOptions());
  }));
  const user = userEvent.setup();
  mount();
  await user.type(await screen.findByLabelText("Title"), "Private source draft");
  await user.click(screen.getByText("Initial fields", { exact: true }));
  await user.click(screen.getByRole("button", { name: "Parent issue" }));
  fireEvent.change(await screen.findByPlaceholderText("Find source issues by title or number"), { target: { value: "release" } });
  await waitFor(() => expect(screen.queryByLabelText("Title")).not.toBeInTheDocument());
  expect(screen.queryByRole("button", { name: "SRC-12 Prepare the release" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Parent issue" })).not.toBeInTheDocument();
  expect(screen.getByRole("alert")).toBeVisible();
});


for (const locale of ["en", "zh-Hans"] as const) it(`explains a confirmed issue limit in ${locale} while keeping the draft and file`, async () => {
  vi.stubGlobal("crypto", webcrypto);
  let requestId = "";
  let uploaded: Record<string, unknown> | null = null;
  const writes: string[] = [];
  const fetch = vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname.endsWith("attachments/drafts")) {
      const form = init.body as FormData;
      uploaded = { id: fileId, draft_id: form.get("draft_id"), issue_id: null, intended_issue_id: null, source_workspace_id: source, uploader_user_id: actor,
        name: "quota.txt", content_type: "text/plain", size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: null };
      return json(uploaded);
    }
    if (url.pathname.includes("/attachments/drafts/")) return json({ draft_id: url.pathname.split("/").pop(), attachments: uploaded ? [uploaded] : [] });
    if (url.pathname === "/api/collaboration/issues") {
      requestId = (init.headers as Record<string, string>)["Idempotency-Key"]!;
      writes.push(requestId);
      expect(JSON.parse(String(init.body))).toMatchObject({ title: "Keep my quota draft", attachment_ids: [fileId] });
      return new Response(JSON.stringify({ code: "issue_limit_reached" }), { status: 402 });
    }
    if (url.pathname.endsWith("/result")) return json({ request_id: requestId, source_workspace_id: source, actor_user_id: actor, outcome: "rejected", code: "issue_limit_reached" });
    throw new Error(`Unexpected test path ${url.pathname}`);
  });
  vi.stubGlobal("fetch", fetch);
  const user = userEvent.setup();
  const { onCreated } = mount(vi.fn(), source, locale);
  await user.type(await screen.findByLabelText(locale === "en" ? "Title" : "标题"), "Keep my quota draft");
  const file = new File(["hello"], "quota.txt", { type: "text/plain" });
  Object.defineProperty(file, "arrayBuffer", { value: async () => new TextEncoder().encode("hello").buffer });
  await user.upload(screen.getByLabelText(locale === "en" ? "Attach files" : "添加文件"), file);
  const included = await screen.findByRole("checkbox", { name: /quota\.txt/ });
  await waitFor(() => expect(included).toBeChecked());
  await user.click(screen.getByRole("button", { name: locale === "en" ? "Create collaboration issue" : "创建协作任务" }));
  await screen.findByText(locale === "en" ? /issue limit.*administrator/i : /任务数量.*上限.*管理员/);
  expect(screen.getByLabelText(locale === "en" ? "Title" : "标题")).toHaveValue("Keep my quota draft");
  expect(screen.getByRole("checkbox", { name: /quota\.txt/ })).toBeChecked();
  expect(screen.getByLabelText(locale === "en" ? "Source workspace" : "来源工作空间")).toBeEnabled();
  expect(writes).toHaveLength(1);
  expect(onCreated).not.toHaveBeenCalled();
});

it("keeps parent input immediate but waits 250ms and blocks Enter on previous candidates", async () => {
  const requested: string[] = [];
  const updatedParent = { ...designParent, name: "协作任务改进" };
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    const url = new URL(input);
    if (!url.searchParams.has("source_workspace_id")) return json(options);
    const search = url.searchParams.get("parent_search");
    if (search) requested.push(search);
    return json(parentOptions(search ? [updatedParent] : [releaseParent, designParent]));
  }));
  const user = userEvent.setup();
  const { onCreated } = mount();
  await screen.findByLabelText("Title");
  await user.click(screen.getByText("Initial fields", { exact: true }));
  const trigger = screen.getByRole("button", { name: "Parent issue" });
  await user.click(trigger);
  await screen.findByRole("button", { name: "SRC-12 Prepare the release" });
  const input = screen.getByPlaceholderText("Find source issues by title or number");
  const initialParentLabel = trigger.textContent!;
  vi.useFakeTimers();
  fireEvent.change(input, { target: { value: "协作任务改进" } });
  expect(input).toHaveValue("协作任务改进");
  expect(screen.queryByRole("button", { name: "SRC-12 Prepare the release" })).not.toBeInTheDocument();
  fireEvent.keyDown(input, { key: "Enter" });
  expect(trigger).toHaveTextContent(initialParentLabel);
  expect(onCreated).not.toHaveBeenCalled();
  expect(requested).toEqual([]);
  await act(async () => { await vi.advanceTimersByTimeAsync(249); });
  expect(requested).toEqual([]);
  await act(async () => { await vi.advanceTimersByTimeAsync(1); });
  expect(requested).toEqual(["协作任务改进"]);
  await act(async () => { await vi.advanceTimersByTimeAsync(0); });
  vi.useRealTimers();
  await user.click(await screen.findByRole("button", { name: "SRC-11 协作任务改进" }));
  expect(trigger).toHaveTextContent("SRC-11 协作任务改进");
});

it("cancels a queued parent search when the source changes before debounce", async () => {
  const requested: { source: string | null; search: string | null }[] = [];
  const sources = [sourceChoice, { id: otherSource, name: "Other workspace", slug: "other" }];
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    const url = new URL(input);
    const id = url.searchParams.get("source_workspace_id");
    requested.push({ source: id, search: url.searchParams.get("parent_search") });
    return json({ ...parentOptions(), sources, source: sources.find((item) => item.id === id) ?? null, ...(id ? {} : { fields: undefined }) });
  }));
  const user = userEvent.setup();
  mount(vi.fn(), source);
  await screen.findByLabelText("Title");
  await user.click(screen.getByText("Initial fields", { exact: true }));
  await user.click(screen.getByRole("button", { name: "Parent issue" }));
  const input = screen.getByPlaceholderText("Find source issues by title or number");
  vi.useFakeTimers();
  fireEvent.change(input, { target: { value: "old source search" } });
  fireEvent.change(screen.getByLabelText("Source workspace"), { target: { value: otherSource } });
  await act(async () => { await vi.advanceTimersByTimeAsync(300); });
  expect(input).not.toBeInTheDocument();
  expect(requested.filter((request) => request.search !== null)).toEqual([]);
  expect(requested.some((request) => request.source === otherSource)).toBe(true);
});

it("cancels with its draft intact and still submits natively by Enter after reopening", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const writes: unknown[] = [];
  vi.stubGlobal("fetch", vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname === "/api/collaboration/issues") { writes.push(JSON.parse(String(init.body))); return json(created); }
    throw new Error(`Unexpected test path ${url.pathname}`);
  }));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  const navigation: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/account", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={navigation}><CollaborationCreationEntry scope={collaborationScope(actor)} /></NavigationProvider></QueryClientProvider>);
  const user = userEvent.setup();
  const entry = screen.getByRole("button", { name: "New collaboration issue" });
  await user.click(entry);
  await user.type(await screen.findByLabelText("Title"), "Retain the cancelled draft");
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(entry).toHaveFocus());
  expect(writes).toEqual([]);
  await user.click(entry);
  const title = await screen.findByLabelText("Title");
  expect(title).toHaveValue("Retain the cancelled draft");
  await user.click(title);
  await user.keyboard("{Enter}");
  await waitFor(() => expect(navigation.push).toHaveBeenCalledOnce());
  expect(writes).toEqual([expect.objectContaining({ title: "Retain the cancelled draft" })]);
});


it("reveals settled creation feedback once without moving focus or scrolling again on refresh and edits", async () => {
  const scroll = vi.spyOn(HTMLElement.prototype, "scrollIntoView");
  let requestId = "";
  let writes = 0;
  vi.stubGlobal("fetch", vi.fn(async (input: string, init: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname === "/api/collaboration/issues") {
      writes += 1;
      requestId = (init.headers as Record<string, string>)["Idempotency-Key"]!;
      return new Response(JSON.stringify({ code: "issue_limit_reached" }), { status: 402 });
    }
    if (url.pathname.endsWith("/result")) return json({ request_id: requestId, source_workspace_id: source, actor_user_id: actor, outcome: "rejected", code: "issue_limit_reached" });
    throw new Error(`Unexpected test path ${url.pathname}`);
  }));
  const user = userEvent.setup();
  const { client } = mount(vi.fn(), source, "en", true);
  const title = await screen.findByLabelText("Title");
  await user.type(title, "Reveal the quota feedback");
  expect(scroll).not.toHaveBeenCalled();
  const submit = screen.getByRole("button", { name: "Create collaboration issue" });
  await user.click(submit);
  const explanation = await screen.findByText(/issue limit.*administrator/i);
  await waitFor(() => expect(scroll).toHaveBeenCalledExactlyOnceWith({ block: "nearest" }));
  expect(scroll.mock.contexts[0]).toContainElement(explanation);
  expect(submit).toHaveFocus();
  await act(() => client.invalidateQueries());
  await user.type(title, " edited");
  expect(title).toHaveFocus();
  expect(scroll).toHaveBeenCalledTimes(1);
  expect(writes).toBe(1);
});

for (const constrained of [true, false]) it(`reveals original request recovery only for a constrained form: ${constrained}`, async () => {
  const scroll = vi.spyOn(HTMLElement.prototype, "scrollIntoView");
  let writes = 0;
  vi.stubGlobal("fetch", vi.fn(async (input: string) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname === "/api/collaboration/issues") { writes += 1; throw new TypeError("Original response lost"); }
    if (url.pathname.endsWith("/result")) return new Response(JSON.stringify({}), { status: 202 });
    throw new Error(`Unexpected test path ${url.pathname}`);
  }));
  const user = userEvent.setup();
  const { onCreated } = mount(vi.fn(), source, "en", constrained);
  await user.type(await screen.findByLabelText("Title"), "Retain the original request");
  const submit = screen.getByRole("button", { name: "Create collaboration issue" });
  await user.click(submit);
  const recovery = await screen.findByRole("button", { name: "Check creation result" });
  await waitFor(() => expect(recovery).toBeEnabled());
  if (constrained) {
    expect(scroll).toHaveBeenCalledExactlyOnceWith({ block: "nearest" });
    expect(scroll.mock.contexts[0]).toContainElement(recovery);
    expect(scroll.mock.contexts[0]).not.toContainElement(screen.getByText("Could not create the issue. Your draft and uploaded files are preserved."));
  } else expect(scroll).not.toHaveBeenCalled();
  expect(submit).toHaveFocus();
  expect(writes).toBe(1);
  expect(onCreated).not.toHaveBeenCalled();
});


for (const constrained of [true, false]) it(`reveals clipped body focus without taking over portal focus: ${constrained}`, async () => {
  const scroll = vi.spyOn(HTMLElement.prototype, "scrollIntoView");
  const fetch = vi.fn(async (input: string) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    if (url.pathname.endsWith("recipients/search")) return json({ issue_id: null, source_workspace_id: source, scope: "directory", source_member: true, recent: [], recipients: [], next_cursor: null });
    throw new Error(`Unexpected test path ${url.pathname}`);
  });
  vi.stubGlobal("fetch", fetch);
  const user = userEvent.setup();
  const view = mount(vi.fn(), source, "en", constrained);
  const title = await screen.findByLabelText("Title");
  const description = screen.getByLabelText("Description");
  const body = view.container.querySelector<HTMLDivElement>("[data-collaboration-creation-body]")!;
  vi.spyOn(body, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 0, 300, 150));
  Object.defineProperty(body, "clientHeight", { configurable: true, value: 150 });
  const descriptionBounds = vi.spyOn(description, "getBoundingClientRect").mockReturnValue(new DOMRect(8, 100, 250, 112));
  await user.click(description);
  expect(description).toHaveFocus();
  if (constrained) expect(scroll).toHaveBeenCalledExactlyOnceWith({ block: "nearest" });
  else expect(scroll).not.toHaveBeenCalled();
  descriptionBounds.mockReturnValue(new DOMRect(8, 20, 250, 80));
  await user.click(title);
  await user.click(description);
  expect(scroll).toHaveBeenCalledTimes(constrained ? 1 : 0);
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  const search = await screen.findByRole("textbox", { name: "Filter options" });
  vi.spyOn(search, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 250, 250, 44));
  const self = await screen.findByRole("option", { name: /Assign to me/ });
  await act(() => self.focus());
  await act(() => search.focus());
  expect(search).toHaveFocus();
  expect(body.contains(search)).toBe(false);
  expect(scroll).toHaveBeenCalledTimes(constrained ? 1 : 0);
  expect(fetch.mock.calls.filter(([input]) => new URL(input).pathname === "/api/collaboration/issues")).toHaveLength(0);
});


it("summarizes retained non-default fields outside the collapsed details", async () => {
  vi.stubGlobal("fetch", vi.fn(async (input: string) => new URL(input).searchParams.has("source_workspace_id") ? json(parentOptions()) : json(options)));
  const user = userEvent.setup();
  let view = mount(vi.fn(), source, "en", true);
  await screen.findByLabelText("Title");
  await user.click(screen.getByText("Initial fields"));
  await user.selectOptions(screen.getByLabelText("Priority"), "high");
  await user.type(screen.getByLabelText("Due date"), "2026-12-01");
  view.unmount();
  view = mount(vi.fn(), source, "en", true);
  const summary = await screen.findByText("2 fields set");
  expect(summary.closest("details")).not.toHaveAttribute("open");
  expect(summary).toBeVisible();
  await user.click(summary);
  expect(screen.getByLabelText("Priority")).toHaveValue("high");
  expect(screen.getByLabelText("Due date")).toHaveValue("2026-12-01");
  await user.selectOptions(screen.getByLabelText("Priority"), "none");
  await user.clear(screen.getByLabelText("Due date"));
  expect(screen.queryByText(/fields? set/)).not.toBeInTheDocument();
});

it("keeps composing Enter out of native submission while plain Enter still creates", async () => {
  const fetch = vi.fn(async (input: string) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return json({ ...options, source: url.searchParams.has("source_workspace_id") ? sourceChoice : null });
    return json(created);
  });
  vi.stubGlobal("fetch", fetch);
  const user = userEvent.setup();
  const { onCreated } = mount(vi.fn(), source);
  const title = await screen.findByLabelText("Title");
  await user.type(title, "中文协作任务");
  fireEvent.compositionStart(title);
  expect(fireEvent.keyDown(title, { key: "Enter", keyCode: 13, isComposing: true })).toBe(false);
  expect(fireEvent.keyDown(title, { key: "Enter", keyCode: 229 })).toBe(false);
  fireEvent.compositionEnd(title);
  expect(onCreated).not.toHaveBeenCalled();
  await user.keyboard("{Enter}");
  await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
  expect(fetch.mock.calls.filter(([input]) => new URL(input).pathname === "/api/collaboration/issues")).toHaveLength(1);
});

it.each([false, true])("focuses a delayed known-source title after touch open without stealing a live control: %s", async (keepControlFocus) => {
  const frames = new Map<number, FrameRequestCallback>();
  let frameId = 0;
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
    frames.set(++frameId, callback);
    return frameId;
  });
  vi.stubGlobal("cancelAnimationFrame", (id: number) => { frames.delete(id); });
  setApiInstance(new ApiClient("https://server.example"));
  let resolveSource!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn(async (input: string) => new URL(input).searchParams.has("source_workspace_id")
    ? new Promise<Response>((resolve) => { resolveSource = resolve; }) : json(options)));
  const client = new QueryClient(); clients.push(client);
  const navigation: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/account", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={navigation}><CollaborationCreationEntry scope={collaborationScope(actor)} sourceWorkspaceId={source} /></NavigationProvider></QueryClientProvider>);
  const trigger = screen.getByRole("button", { name: "New collaboration issue" });
  const user = userEvent.setup();
  await user.pointer([{ keys: "[TouchA>]", target: trigger }, { keys: "[/TouchA]", target: trigger }]);
  const sourcePicker = await screen.findByLabelText("Source workspace");
  await waitFor(() => expect(resolveSource).toBeDefined());
  // Keep the modal's initial focus frame pending until the user can interact.
  await waitFor(() => expect(frames.size).toBeGreaterThan(0));
  if (keepControlFocus) sourcePicker.focus();
  await act(async () => { resolveSource(json({ ...options, source: sourceChoice })); });
  const title = await screen.findByLabelText("Title");
  await waitFor(async () => {
    await act(() => {
      const callbacks = [...frames.values()];
      frames.clear();
      callbacks.forEach((callback) => callback(performance.now()));
    });
    expect(keepControlFocus ? sourcePicker : title).toHaveFocus();
  });
});

it("restores the opener when creation closes before its initial focus frame", async () => {
  const frames = new Map<number, FrameRequestCallback>();
  let frameId = 0;
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
    frames.set(++frameId, callback);
    return frameId;
  });
  vi.stubGlobal("cancelAnimationFrame", (id: number) => { frames.delete(id); });
  setApiInstance(new ApiClient("https://server.example"));
  vi.stubGlobal("fetch", vi.fn(async (input: string) => json({ ...options, source: new URL(input).searchParams.has("source_workspace_id") ? sourceChoice : null })));
  const client = new QueryClient(); clients.push(client);
  const navigation: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/account", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={navigation}><CollaborationCreationEntry scope={collaborationScope(actor)} sourceWorkspaceId={source} /></NavigationProvider></QueryClientProvider>);
  const trigger = screen.getByRole("button", { name: "New collaboration issue" });
  const user = userEvent.setup();
  await user.pointer([{ keys: "[TouchA>]", target: trigger }, { keys: "[/TouchA]", target: trigger }]);
  await screen.findByLabelText("Title");
  await waitFor(() => expect(frames.size).toBeGreaterThan(0));
  const dialog = screen.getByRole("dialog", { name: "New collaboration issue" });
  await user.click(screen.getByRole("button", { name: "Close" }));
  await waitFor(async () => {
    await act(() => {
      const callbacks = [...frames.values()];
      frames.clear();
      callbacks.forEach((callback) => callback(performance.now()));
    });
    expect(screen.queryByRole("dialog", { name: "New collaboration issue" })).not.toBeInTheDocument();
    expect(dialog).not.toHaveFocus();
    expect(trigger).toHaveFocus();
  });
});
