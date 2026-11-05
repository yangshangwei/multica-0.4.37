import { afterEach, expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { clearCollaborationAccess, collaborationKeys, collaborationScope, newCollaborationDeliveryForm, parseCollaborationIssue, prepareCollaborationAttachmentDelivery,
  readCollaborationDeliveryRecords, saveCollaborationDeliveryRecord, type CollaborationUploadDraft } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationAttachmentPanel, CollaborationCommentAttachments } from "./attachment-panel";

// Core delivery-api.test.ts owns capability/path checks. This proves UI cleanup.
const actor = "00000000-0000-4000-8000-000000000001";
const fileId = "00000000-0000-4000-8000-000000000002";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const metadata = { id: fileId, draft_id: fileId, issue_id: actor, intended_issue_id: null, source_workspace_id: actor, uploader_user_id: actor,
  name: "private.png", content_type: "image/png", size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: "2026-10-18T00:00:00Z" };
const client = new QueryClient();
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

it("does not request metadata or render a file region when no files are referenced", () => {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const getFiles = vi.spyOn(api, "getCollaborationAttachments");
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentAttachments scope={collaborationScope(actor)} issueId={actor} attachmentIds={[]} /></QueryClientProvider>);
  expect(screen.queryByRole("region", { name: "Attachments" })).not.toBeInTheDocument();
  expect(getFiles).not.toHaveBeenCalled();
});

it("shows referenced-file loading and an initial failure with an explicit metadata retry", async () => {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  let rejectRead!: (reason: Error) => void;
  const getFiles = vi.spyOn(api, "getCollaborationAttachments")
    .mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectRead = reject; }))
    .mockResolvedValue({ issue_id: actor, attachments: [metadata] });
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentAttachments scope={collaborationScope(actor)} issueId={actor} attachmentIds={[fileId]} /></QueryClientProvider>);
  const panel = screen.getByRole("region", { name: "Attachments" });
  expect(within(panel).getByRole("status")).toHaveTextContent("Loading files...");
  await act(async () => rejectRead(new TypeError("Offline")));
  expect(await within(panel).findByRole("alert")).toHaveTextContent("Could not load collaboration. Try again.");
  expect(screen.queryByText("private.png")).not.toBeInTheDocument();
  await userEvent.click(within(panel).getByRole("button", { name: "Refresh" }));
  expect(await within(panel).findByRole("button", { name: "Download private.png" })).toBeVisible();
  expect(getFiles).toHaveBeenCalledTimes(2);
});

it("keeps available referenced files and reports missing references without showing unrelated metadata", async () => {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const missingId = "00000000-0000-4000-8000-000000000003";
  vi.spyOn(api, "getCollaborationAttachments").mockResolvedValue({ issue_id: actor, attachments: [metadata,
    { ...metadata, id: "00000000-0000-4000-8000-000000000004", name: "unrelated.txt", content_type: "text/plain" }] });
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentAttachments scope={collaborationScope(actor)} issueId={actor} attachmentIds={[fileId, missingId]} /></QueryClientProvider>);
  expect(await screen.findByRole("button", { name: "Download private.png" })).toBeVisible();
  expect(screen.getByText("Some referenced files are unavailable.")).toBeVisible();
  expect(screen.queryByText("unrelated.txt")).not.toBeInTheDocument();
  expect(screen.queryByText("No files delivered yet.")).not.toBeInTheDocument();
});

it("hides stale referenced metadata and closes its preview when a refresh fails", async () => {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const scope = collaborationScope(actor);
  const getFiles = vi.spyOn(api, "getCollaborationAttachments").mockResolvedValue({ issue_id: actor, attachments: [metadata] });
  const create = vi.fn(() => "blob:referenced-preview"), revoke = vi.fn();
  vi.stubGlobal("URL", class extends URL { static createObjectURL = create; static revokeObjectURL = revoke; });
  vi.stubGlobal("fetch", vi.fn(async () => new Response("bytes", { headers: { "content-type": "image/png" } })));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentAttachments scope={scope} issueId={actor} attachmentIds={[fileId]} /></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: "Preview private.png" }));
  expect(await screen.findByRole("img", { name: "private.png" })).toHaveAttribute("src", "blob:referenced-preview");
  getFiles.mockRejectedValueOnce(new TypeError("Offline"));
  await act(async () => { await client.invalidateQueries({ queryKey: [...collaborationKeys.issue(scope, actor), "attachments"] }); });
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.queryByText("private.png")).not.toBeInTheDocument();
  expect(screen.getByRole("alert")).toHaveTextContent("Could not load collaboration. Try again.");
  expect(revoke).toHaveBeenCalledWith("blob:referenced-preview");
});

it("closes a preview when that reference becomes unavailable while another file remains", async () => {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const scope = collaborationScope(actor);
  const remaining = { ...metadata, id: "00000000-0000-4000-8000-000000000003", name: "remaining.png" };
  const getFiles = vi.spyOn(api, "getCollaborationAttachments").mockResolvedValue({ issue_id: actor, attachments: [metadata, remaining] });
  const create = vi.fn(() => "blob:referenced-preview"), revoke = vi.fn();
  vi.stubGlobal("URL", class extends URL { static createObjectURL = create; static revokeObjectURL = revoke; });
  vi.stubGlobal("fetch", vi.fn(async () => new Response("bytes", { headers: { "content-type": "image/png" } })));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentAttachments scope={scope} issueId={actor} attachmentIds={[fileId, remaining.id]} /></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: "Preview private.png" }));
  expect(await screen.findByRole("img", { name: "private.png" })).toBeVisible();
  getFiles.mockResolvedValue({ issue_id: actor, attachments: [remaining] });
  await act(async () => { await client.invalidateQueries({ queryKey: [...collaborationKeys.issue(scope, actor), "attachments"] }); });
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.getByRole("button", { name: "Download remaining.png" })).toBeVisible();
  expect(screen.getByText("Some referenced files are unavailable.")).toBeVisible();
  expect(screen.queryByText("private.png")).not.toBeInTheDocument();
  expect(revoke).toHaveBeenCalledWith("blob:referenced-preview");
});

it("removes referenced private files and their preview after an authoritative denial", async () => {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const scope = collaborationScope(actor);
  vi.spyOn(api, "getCollaborationAttachments").mockResolvedValue({ issue_id: actor, attachments: [metadata] });
  const create = vi.fn(() => "blob:referenced-preview"), revoke = vi.fn();
  vi.stubGlobal("URL", class extends URL { static createObjectURL = create; static revokeObjectURL = revoke; });
  vi.stubGlobal("fetch", vi.fn(async () => new Response("bytes", { headers: { "content-type": "image/png" } })));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentAttachments scope={scope} issueId={actor} attachmentIds={[fileId]} /></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: "Preview private.png" }));
  expect(await screen.findByRole("img", { name: "private.png" })).toHaveAttribute("src", "blob:referenced-preview");
  act(() => clearCollaborationAccess(client, scope, actor));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.queryByText("private.png")).not.toBeInTheDocument();
  expect(screen.getByRole("alert")).toHaveTextContent("You no longer have access to this issue.");
  expect(revoke).toHaveBeenCalledWith("blob:referenced-preview");
});

it("closes a private preview and revokes its object URL when issue access is cleared", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  const create = vi.fn(() => "blob:private-preview"), revoke = vi.fn();
  vi.stubGlobal("URL", class extends URL { static createObjectURL = create; static revokeObjectURL = revoke; });
  vi.stubGlobal("fetch", vi.fn(async (url: string) => url.endsWith("/content") ? new Response("bytes", { headers: { "content-type": "image/png" } })
    : new Response(JSON.stringify({ issue_id: actor, attachments: [metadata] }))));
  const issue = parseCollaborationIssue({ ...collaborationIssueFixture, capabilities: { ...collaborationIssueFixture.capabilities, comment: false } })!;
  renderWithI18n(<QueryClientProvider client={client}><CollaborationAttachmentPanel scope={scope} issue={issue} /></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: "Preview private.png" }));
  expect(await screen.findByRole("img", { name: "private.png" })).toHaveAttribute("src", "blob:private-preview");
  clearCollaborationAccess(client, scope, actor);
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(revoke).toHaveBeenCalledWith("blob:private-preview");
  expect(screen.queryByRole("button", { name: "Download private.png" })).toBeNull();
});

// Chromium will not run its PDF viewer inside the sandboxed preview frame.
it("offers PDFs for download only", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ issue_id: actor, attachments: [{ ...metadata, name: "brief.pdf", content_type: "application/pdf" }] }))));
  const issue = parseCollaborationIssue({ ...collaborationIssueFixture, capabilities: { ...collaborationIssueFixture.capabilities, comment: false } })!;
  renderWithI18n(<QueryClientProvider client={client}><CollaborationAttachmentPanel scope={scope} issue={issue} /></QueryClientProvider>);
  expect(await screen.findByRole("button", { name: "Download brief.pdf" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Preview brief.pdf" })).toBeNull();
});

it.each(["upload", "delivery"] as const)("opens the enclosing auxiliary disclosure for a saved %s without replacing its original request", async (kind) => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  const form = newCollaborationDeliveryForm({ server: scope.server, userId: scope.userId,
    context: { kind: "issue", issueId: actor, sourceWorkspaceId: actor } }, "attachment");
  if (kind === "delivery") form.selectedAttachmentIds = [fileId];
  saveCollaborationDeliveryRecord(form);
  const requestId = "00000000-0000-4000-8000-000000000003";
  const upload: CollaborationUploadDraft = { scope: form.scope, draftId: form.draftId, kind: "upload", requestId, createdAt: Date.now(),
    file: { name: "unconfirmed.txt", type: "text/plain", size: 5, sha256: "a".repeat(64) } };
  saveCollaborationDeliveryRecord(kind === "upload" ? upload : prepareCollaborationAttachmentDelivery(form, requestId));
  const fetch = vi.fn(async (url: string, _init?: RequestInit) => {
    const path = new URL(url).pathname;
    if (path.endsWith("/attachments")) return new Response(JSON.stringify({ issue_id: actor, attachments: [] }));
    if (path.endsWith(`/drafts/${form.draftId}`)) return new Response(JSON.stringify({ draft_id: form.draftId, attachments: [] }));
    throw new Error(`Unexpected attachment recovery request: ${path}`);
  });
  vi.stubGlobal("fetch", fetch);
  const issue = parseCollaborationIssue(collaborationIssueFixture)!;
  renderWithI18n(<QueryClientProvider client={client}><details><summary>Attachments</summary>
    <CollaborationAttachmentPanel scope={scope} issue={issue} />
  </details></QueryClientProvider>);
  expect(await screen.findByRole("button", { name: kind === "upload" ? "Check upload" : "Check file delivery" })).toBeInTheDocument();
  expect(screen.getByText("Attachments", { selector: "summary" }).closest("details")).toHaveAttribute("open");
  expect(readCollaborationDeliveryRecords(form.scope).find((record) => record.kind === kind)).toMatchObject({ requestId });
  expect(fetch.mock.calls.every(([, init]) => (init?.method ?? "GET") === "GET")).toBe(true);
});
