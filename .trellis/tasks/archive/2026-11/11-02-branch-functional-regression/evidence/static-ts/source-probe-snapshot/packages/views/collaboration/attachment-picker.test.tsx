import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { newCollaborationDeliveryForm, type CollaborationUploadDraft } from "@multica/core/collaboration";
import { renderWithI18n } from "../test/i18n";
import { CollaborationAttachmentPicker } from "./attachment-picker";

// The core delivery tests own cancellation/replay semantics. This covers the
// button and unmount wiring, including cancellation inside a creation form.
const upload = vi.hoisted(() => ({ stage: vi.fn(), execute: vi.fn() }));
vi.mock("@multica/core/collaboration", async (original) => ({
  ...await original<typeof import("@multica/core/collaboration")>(),
  stageCollaborationUpload: upload.stage,
  executeCollaborationUpload: upload.execute,
}));
const scope = { server: "https://server.example", apiScope: "test", userId: "00000000-0000-4000-8000-000000000001" };
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); vi.clearAllMocks(); });

it.each(["cancel", "unmount"])("aborts the active upload on %s without submitting the form", async (action) => {
  const form = newCollaborationDeliveryForm({ ...scope, context: { kind: "creation", sourceWorkspaceId: scope.userId } }, "creation");
  const draft: CollaborationUploadDraft = { scope: form.scope, draftId: form.draftId, kind: "upload", requestId: "00000000-0000-4000-8000-000000000002",
    createdAt: Date.now(), file: { name: "notes.txt", type: "text/plain", size: 5, sha256: "0".repeat(64) } };
  upload.stage.mockResolvedValue(draft);
  upload.execute.mockImplementation((_client, _scope, _draft, _file, _storage, signal: AbortSignal) => new Promise((_, reject) => {
    signal.addEventListener("abort", () => reject(signal.reason), { once: true });
  }));
  const client = new QueryClient(); clients.push(client);
  const submit = vi.fn((event: React.FormEvent) => event.preventDefault());
  const view = renderWithI18n(<QueryClientProvider client={client}><form onSubmit={submit}>
    <CollaborationAttachmentPicker scope={scope} form={form} update={vi.fn()} uploads={[]} />
  </form></QueryClientProvider>);
  const user = userEvent.setup();
  await user.upload(screen.getByLabelText("Attach files"), new File(["hello"], "notes.txt", { type: "text/plain" }));
  await waitFor(() => expect(upload.execute).toHaveBeenCalledOnce());
  const signal: AbortSignal = upload.execute.mock.calls[0]![5];
  expect(signal.aborted).toBe(false);

  if (action === "cancel") await user.click(screen.getByRole("button", { name: "Cancel upload" }));
  else view.unmount();

  await waitFor(() => expect(signal.aborted).toBe(true));
  expect(submit).not.toHaveBeenCalled();
  if (action === "cancel") expect(await screen.findByText("Upload stopped. You can still check any unconfirmed file.")).toBeInTheDocument();
});
