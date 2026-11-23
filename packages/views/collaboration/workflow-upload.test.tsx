import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, type CollaborationDeliveryForm, type CollaborationUploadDraft } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationWorkflow } from "./workflow";

// Core owns upload persistence/replay; this regression covers the real form's
// fieldset and dialog boundaries around the attachment cancellation control.
const upload = vi.hoisted(() => ({ stage: vi.fn(), execute: vi.fn() }));
vi.mock("@multica/core/collaboration", async (original) => ({
  ...await original<typeof import("@multica/core/collaboration")>(),
  stageCollaborationUpload: upload.stage,
  executeCollaborationUpload: upload.execute,
}));
const client = new QueryClient();
afterEach(() => { client.clear(); localStorage.clear(); vi.clearAllMocks(); vi.unstubAllGlobals(); });

it("keeps Cancel upload operable while the result dialog is locked against dismissal", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const parsed = parseCollaborationIssue(collaborationIssueFixture)!;
  const issue = { ...parsed, handler: parsed.creator, ai_policy: "manual" as const,
    delivery: { current: null, latest: null, has_history: false }, write_context: { participant_epoch: "2", account_epoch: "3" }, capabilities: { ...parsed.capabilities, complete: true } };
  const scope = collaborationScope(issue.creator.user_id);
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ action: "complete", status_options_version: "a".repeat(64), default_status: "done", options: [{ key: "done", name: "Done", category: "done" }] }))));
  upload.stage.mockImplementation((_client: unknown, _scope: unknown, form: CollaborationDeliveryForm, requestId: string) => Promise.resolve({
    scope: form.scope, draftId: form.draftId, kind: "upload", requestId, createdAt: Date.now(),
    file: { name: "notes.txt", type: "text/plain", size: 5, sha256: "0".repeat(64) },
  } satisfies CollaborationUploadDraft));
  upload.execute.mockImplementation((_client, _scope, _draft, _file, _storage, signal: AbortSignal) => new Promise((_, reject) => {
    signal.addEventListener("abort", () => reject(signal.reason), { once: true });
  }));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Record result and complete" }));
  const dialog = screen.getByRole("dialog", { name: "Record result and complete" });
  await user.upload(within(dialog).getByLabelText("Attach files"), new File(["hello"], "notes.txt", { type: "text/plain" }));
  await waitFor(() => expect(upload.execute).toHaveBeenCalledOnce());
  const signal: AbortSignal = upload.execute.mock.calls[0]![5];
  await user.keyboard("{Escape}");
  expect(dialog).toBeVisible();
  const cancel = within(dialog).getByRole("button", { name: "Cancel upload" });
  expect(cancel).toBeEnabled();
  await user.click(cancel);
  await waitFor(() => expect(signal.aborted).toBe(true));
  expect(await within(dialog).findByText("Upload stopped. You can still check any unconfirmed file.")).toBeVisible();
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
});
