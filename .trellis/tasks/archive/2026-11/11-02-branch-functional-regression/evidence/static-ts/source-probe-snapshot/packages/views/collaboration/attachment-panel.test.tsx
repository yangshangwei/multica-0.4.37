import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { clearCollaborationAccess, collaborationScope, parseCollaborationIssue } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationAttachmentPanel } from "./attachment-panel";

// Core delivery-api.test.ts owns capability/path checks. This proves UI cleanup.
const actor = "00000000-0000-4000-8000-000000000001";
const fileId = "00000000-0000-4000-8000-000000000002";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const metadata = { id: fileId, draft_id: fileId, issue_id: actor, intended_issue_id: null, source_workspace_id: actor, uploader_user_id: actor,
  name: "private.png", content_type: "image/png", size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: "2026-10-18T00:00:00Z" };
const client = new QueryClient();
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

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
