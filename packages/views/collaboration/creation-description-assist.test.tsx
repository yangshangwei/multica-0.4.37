import type { CSSProperties } from "react";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationCreationOptions, collaborationScope, finishCollaborationDeliveryDraft, readCollaborationDeliveryRecords, saveCollaborationDeliveryRecord } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { CollaborationCreationEntry, CollaborationCreationPanel } from "./creation-form";

const actor = "00000000-0000-4000-8000-000000000001";
const source = "00000000-0000-4000-8000-000000000002";
const alternate = "00000000-0000-4000-8000-000000000003";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const sources = [{ id: source, name: "Source", slug: "source" }, { id: alternate, name: "Other", slug: "other" }];
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
function mountAI({ modal = false, optimize = async () => json({ text: "Improved description", questions: [] }), sourceDenied = () => false }: {
  modal?: boolean; optimize?: (init: RequestInit) => Promise<Response>; sourceDenied?: () => boolean;
} = {}) {
  const requests: RequestInit[] = [];
  const creations: unknown[] = [];
  const fetch = vi.fn(async (input: string, init: RequestInit = {}) => {
    const url = new URL(input);
    if (url.pathname.endsWith("creation-options")) return sourceDenied() && url.searchParams.has("source_workspace_id")
      ? json({ code: "source_access_denied" }, 403) : json({ creation_enabled: true, sources,
      source: sources.find((item) => item.id === url.searchParams.get("source_workspace_id")) ?? null, max_attachment_bytes: 104857600 });
    if (url.pathname === "/api/collaboration/issues") { creations.push(JSON.parse(String(init.body))); return json(collaborationIssueFixture); }
    if (url.pathname.includes("/attachments/drafts/")) return json({ draft_id: url.pathname.split("/").pop(), attachments: [] });
    if (url.pathname.endsWith("optimize-description")) { requests.push(init); return optimize(init); }
    throw new Error(`Unexpected test request ${url.pathname}`);
  });
  vi.stubGlobal("fetch", fetch);
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  const scope = collaborationScope(actor);
  const navigation: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/account", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
  const view = renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={navigation}>
    {modal ? <CollaborationCreationEntry scope={scope} sourceWorkspaceId={source} /> : <CollaborationCreationPanel scope={scope} sourceWorkspaceId={source} onCreated={vi.fn()} />}
  </NavigationProvider></QueryClientProvider>);
  return { ...view, requests, creations, client };
}

it("expands without remounting the draft and resets its size after reopening", async () => {
  mountAI({ modal: true });
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "New collaboration issue" }));
  const title = await screen.findByLabelText("Title");
  fireEvent.change(title, { target: { value: "Keep this draft" } });
  const expand = screen.getByRole("button", { name: "Expand" });
  expect(expand).toHaveAttribute("aria-pressed", "false");
  await user.click(expand);
  expect(screen.getByRole("button", { name: "Collapse" })).toHaveAttribute("aria-pressed", "true");
  expect(screen.getByLabelText("Title")).toBe(title);
  expect(title).toHaveValue("Keep this draft");
  await user.click(screen.getByRole("button", { name: "Close" }));
  await user.click(screen.getByRole("button", { name: "New collaboration issue" }));
  expect(await screen.findByLabelText("Title")).toHaveValue("Keep this draft");
  expect(screen.getByRole("button", { name: "Expand" })).toHaveAttribute("aria-pressed", "false");
});

it("uses the selected source, adopts into the real textarea and restores exact whitespace on undo", async () => {
  const { requests } = mountAI();
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  expect(screen.getByRole("button", { name: "Help me improve" })).toBeDisabled();
  fireEvent.change(description, { target: { value: "  Original description\n\n" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  await waitFor(() => expect(description).toHaveValue("Improved description"));
  expect(new Headers(requests[0]!.headers).get("X-Workspace-ID")).toBe(source);
  expect(JSON.parse(String(requests[0]!.body))).toMatchObject({ text: "  Original description\n\n", mode: "manual" });
  await user.click(screen.getByRole("button", { name: "Undo" }));
  expect(description).toHaveValue("  Original description\n\n");
});

it("keeps late completions away from a different source and preserves newer edits", async () => {
  let finish!: (response: Response) => void;
  mountAI({ optimize: () => new Promise((resolve) => { finish = resolve; }) });
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Original" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  fireEvent.change(description, { target: { value: "Newer edit" } });
  await act(async () => finish(json({ text: "Stale suggestion", questions: [] })));
  expect(description).toHaveValue("Newer edit");
  await user.click(screen.getByRole("button", { name: "Regenerate" }));
  await user.selectOptions(screen.getByLabelText("Source workspace"), alternate);
  const otherDescription = await screen.findByLabelText("Description");
  await act(async () => finish(json({ text: "Wrong source", questions: [] })));
  expect(otherDescription).toHaveValue("");
  expect(screen.queryByText("Wrong source")).not.toBeInTheDocument();
});

it.each([404, 503])("preserves an editable draft after AI HTTP %s", async (status) => {
  mountAI({ optimize: async () => json({ error: "Unavailable" }, status) });
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Keep my description" } });
  fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Create normally" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  await screen.findByRole("alert");
  expect(description).toHaveValue("Keep my description");
  expect(description).toBeEnabled();
  expect(screen.getByRole("button", { name: "Create collaboration issue" })).toBeEnabled();
});

it("limits only AI by Unicode characters without truncating the draft", async () => {
  const { requests } = mountAI();
  await screen.findByLabelText("Description");
  const body = "😀".repeat(20001);
  fireEvent.change(screen.getByLabelText("Description"), { target: { value: body } });
  fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Allowed creation" } });
  expect(screen.getByRole("button", { name: "Help me improve" })).toBeDisabled();
  expect(screen.getByText(/20,000/)).toBeVisible();
  expect(screen.getByLabelText("Description")).toHaveValue(body);
  expect(screen.getByRole("button", { name: "Create collaboration issue" })).toBeEnabled();
  fireEvent.change(screen.getByLabelText("Description"), { target: { value: "😀".repeat(20000) } });
  expect(screen.getByRole("button", { name: "Help me improve" })).toBeEnabled();
  fireEvent.change(screen.getByLabelText("Title"), { target: { value: "文".repeat(501) } });
  expect(screen.getByRole("button", { name: "Help me improve" })).toBeDisabled();
  expect(screen.getByText(/500/)).toBeVisible();
  expect(requests).toHaveLength(0);
});


it("keeps streaming fragments out of native creation and preserves the stream through expansion", async () => {
  let stream!: ReadableStreamDefaultController<Uint8Array>;
  const { creations } = mountAI({ modal: true, optimize: async () => new Response(new ReadableStream<Uint8Array>({ start(controller) { stream = controller; } }), { headers: { "Content-Type": "text/event-stream" } }) });
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "New collaboration issue" }));
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Original payload" } });
  fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Create now" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  await act(async () => stream.enqueue(new TextEncoder().encode('event: text_delta\ndata: {"text":"Provisional fragment"}\n\n')));
  await screen.findByText("Provisional fragment");
  await user.click(screen.getByRole("button", { name: "Expand" }));
  expect(screen.getByLabelText("Description")).toBe(description);
  expect(screen.getByText("Provisional fragment")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Create collaboration issue" }));
  await waitFor(() => expect(creations).toHaveLength(1));
  expect(creations[0]).toMatchObject({ description: "Original payload" });
});

it("leaves late results as inert preview while an upload awaits confirmation", async () => {
  let finish!: (response: Response) => void;
  mountAI({ optimize: () => new Promise((resolve) => { finish = resolve; }) });
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Original before upload" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  const owner = readCollaborationDeliveryRecords({ server: "https://server.example", userId: actor }).find((item) => item.kind === "form");
  expect(owner).toBeDefined();
  act(() => saveCollaborationDeliveryRecord({ kind: "upload", scope: owner!.scope, draftId: owner!.draftId, createdAt: Date.now(), requestId: "00000000-0000-4000-8000-000000000004",
    file: { name: "pending.txt", size: 1, type: "text/plain", sha256: "0".repeat(64) } }));
  const suggestion = "!file[private](https://external.example/private)";
  await act(async () => finish(json({ text: suggestion, questions: [] })));
  expect(description).toHaveValue("Original before upload");
  expect(description).toBeDisabled();
  expect(screen.getByText(suggestion)).toBeVisible();
  expect(screen.queryByRole("link", { name: "private" })).not.toBeInTheDocument();
  expect(screen.queryByRole("img", { name: "private" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Regenerate" })).toBeDisabled();
});

it("discards late AI content after the API identity changes", async () => {
  let finish!: (response: Response) => void;
  mountAI({ optimize: () => new Promise((resolve) => { finish = resolve; }) });
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Owned draft" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  setApiInstance(new ApiClient("https://another-server.example"));
  await act(async () => finish(json({ text: "Foreign result", questions: [] })));
  expect(description).toHaveValue("Owned draft");
  expect(screen.queryByText("Foreign result")).not.toBeInTheDocument();
});

it("discards late AI output after an authoritative source denial without restoring its private draft", async () => {
  let finish!: (response: Response) => void;
  let denied = false;
  const { client, requests } = mountAI({ sourceDenied: () => denied, optimize: () => new Promise((resolve) => { finish = resolve; }) });
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Private source draft" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  const scope = collaborationScope(actor);
  denied = true;
  await act(async () => {
    await client.refetchQueries({ queryKey: collaborationCreationOptions(client, scope, source).queryKey, exact: true });
  });
  await waitFor(() => expect(screen.queryByLabelText("Description")).not.toBeInTheDocument());
  expect(requests[0]!.signal?.aborted).toBe(true);
  expect(readCollaborationDeliveryRecords(scope)).toHaveLength(0);
  await act(async () => finish(json({ text: "Private late result", questions: ["Private follow-up?"] })));
  expect(screen.queryByText(/Private/)).not.toBeInTheDocument();
  expect(readCollaborationDeliveryRecords(scope)).toHaveLength(0);
});

it("keeps a replacement draft intact when AI completes after the original draft retires", async () => {
  let finish!: (response: Response) => void;
  const { requests } = mountAI({ optimize: () => new Promise((resolve) => { finish = resolve; }) });
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Retired draft" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  const scope = collaborationScope(actor);
  const original = readCollaborationDeliveryRecords(scope).find((record) => record.kind === "form");
  expect(original).toBeDefined();
  act(() => finishCollaborationDeliveryDraft(original!.scope, original!.draftId));
  await waitFor(() => expect(screen.getByLabelText("Description")).toHaveValue(""));
  expect(requests[0]!.signal?.aborted).toBe(true);
  fireEvent.change(screen.getByLabelText("Description"), { target: { value: "New draft contents" } });
  const replacement = readCollaborationDeliveryRecords(scope).find((record) => record.kind === "form");
  expect(replacement?.draftId).not.toBe(original!.draftId);
  await act(async () => finish(json({ text: "Retired AI output", questions: ["Old question?"] })));
  expect(screen.getByLabelText("Description")).toHaveValue("New draft contents");
  expect(screen.queryByText("Retired AI output")).not.toBeInTheDocument();
  expect(screen.queryByText("Old question?")).not.toBeInTheDocument();
  expect(readCollaborationDeliveryRecords(scope)).toEqual([replacement]);
});

it("merges clarification answers into the latest textarea and undoes only that merge", async () => {
  mountAI({ optimize: async () => json({ text: "Improved description", questions: ["Which output format?"] }) });
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Initial text" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  const answer = await screen.findByLabelText("Which output format?");
  fireEvent.change(description, { target: { value: "  Newer human draft  " } });
  await user.type(answer, "PDF");
  await user.click(screen.getByRole("button", { name: "Add answers to draft" }));
  expect(description).toHaveValue("  Newer human draft  \n\nAdditional details\n\nWhich output format?\nPDF");
  await user.click(screen.getByRole("button", { name: "Undo" }));
  expect(description).toHaveValue("  Newer human draft  ");
  expect(screen.getByLabelText("Which output format?")).toHaveValue("PDF");
});


it("cancels AI without clearing the original draft or admitting a late completion", async () => {
  let finish!: (response: Response) => void;
  const { requests } = mountAI({ optimize: () => new Promise((resolve) => { finish = resolve; }) });
  const user = userEvent.setup();
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Keep this after cancel" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(requests[0]!.signal?.aborted).toBe(true);
  await act(async () => finish(json({ text: "Cancelled result", questions: [] })));
  expect(description).toHaveValue("Keep this after cancel");
  expect(screen.queryByText("Cancelled result")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Help me improve" })).toBeEnabled();
});


it.each([false, true])("keeps the popup interactive over desktop drag regions and closes its stream: expanded=%s", async (expanded) => {
  let finish!: (response: Response) => void;
  const { requests } = mountAI({ modal: true, optimize: () => new Promise((resolve) => { finish = resolve; }) });
  const user = userEvent.setup();
  const opener = screen.getByRole("button", { name: "New collaboration issue" });
  await user.click(opener);
  const description = await screen.findByLabelText("Description");
  fireEvent.change(description, { target: { value: "Preserve the closing draft" } });
  await user.click(screen.getByRole("button", { name: "Help me improve" }));
  if (expanded) await user.click(screen.getByRole("button", { name: "Expand" }));
  const popup = screen.getByRole("dialog");
  // Electron consumes its drag-region property before DOM events; keep the
  // owning popup excluded in every size. Native hit testing is browser QA.
  expect((popup.style as CSSProperties & { WebkitAppRegion?: string }).WebkitAppRegion).toBe("no-drag");
  const close = screen.getByRole("button", { name: "Close" });
  close.focus();
  await user.keyboard("{Enter}");
  await waitFor(() => expect(opener).toHaveFocus());
  expect(requests[0]!.signal?.aborted).toBe(true);
  await act(async () => finish(json({ text: "Late result after Close", questions: [] })));
  await user.click(opener);
  expect(await screen.findByLabelText("Description")).toHaveValue("Preserve the closing draft");
  expect(screen.queryByText("Late result after Close")).not.toBeInTheDocument();
});
