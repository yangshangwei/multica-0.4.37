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

it.each([false, true].flatMap((compact) => ["cancel", "unmount", "finish", "move-focus"].map((action) => ({ compact, action }))))("settles $action upload with compact=$compact without submitting or losing focus", async ({ compact, action }) => {
  const form = newCollaborationDeliveryForm({ ...scope, context: { kind: "creation", sourceWorkspaceId: scope.userId } }, "creation");
  const draft: CollaborationUploadDraft = { scope: form.scope, draftId: form.draftId, kind: "upload", requestId: "00000000-0000-4000-8000-000000000002",
    createdAt: Date.now(), file: { name: "notes.txt", type: "text/plain", size: 5, sha256: "0".repeat(64) } };
  upload.stage.mockResolvedValue(draft);
  let complete!: () => void;
  let activeAtAbort: Element | null = null;
  upload.execute.mockImplementation((_client, _scope, _draft, _file, _storage, signal: AbortSignal) => new Promise<void>((resolve, reject) => {
    complete = resolve;
    signal.addEventListener("abort", () => { activeAtAbort = document.activeElement; reject(signal.reason); }, { once: true });
  }));
  const client = new QueryClient(); clients.push(client);
  const submit = vi.fn((event: React.FormEvent) => event.preventDefault());
  const view = renderWithI18n(<QueryClientProvider client={client}><form onSubmit={submit}>
    <CollaborationAttachmentPicker scope={scope} form={form} update={vi.fn()} uploads={[]}
      renderComposer={compact ? ({ trigger, details }) => <><div>{details}</div><footer>{trigger}</footer></> : undefined} />
    <button type="button">Another control</button>
  </form></QueryClientProvider>);
  const user = userEvent.setup();
  if (compact) {
    const trigger = screen.getByRole("button", { name: "Attach files" });
    trigger.focus();
    const click = vi.spyOn(screen.getByLabelText("Attach files"), "click");
    await user.keyboard("{Enter}");
    expect(click).toHaveBeenCalledOnce();
    expect(submit).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Attach files")).not.toBeVisible();
  }
  await user.upload(screen.getByLabelText("Attach files"), new File(["hello"], "notes.txt", { type: "text/plain" }));
  await waitFor(() => expect(upload.execute).toHaveBeenCalledOnce());
  const signal: AbortSignal = upload.execute.mock.calls[0]![5];
  expect(signal.aborted).toBe(false);

  if (action === "cancel") await user.click(screen.getByRole("button", { name: "Cancel upload" }));
  else if (action === "unmount") view.unmount();
  else {
    if (action === "move-focus") await user.click(screen.getByRole("button", { name: "Another control" }));
    else screen.getByRole("button", { name: "Cancel upload" }).focus();
    complete();
    await waitFor(() => expect(screen.queryByRole("button", { name: "Cancel upload" })).not.toBeInTheDocument());
  }

  await waitFor(() => expect(signal.aborted).toBe(action === "cancel" || action === "unmount"));
  expect(submit).not.toHaveBeenCalled();
  if (action === "cancel") {
    expect(await screen.findByText("Upload stopped. You can still check any unconfirmed file.")).toBeInTheDocument();
    // The modal restores removed-button focus on a later frame; hand off before
    // abort can remove Cancel so that fallback never takes ownership.
    if (compact) expect(activeAtAbort).toBe(screen.getByRole("button", { name: "Attach files" }));
  }
  if (action === "cancel" || action === "finish") {
    await waitFor(() => expect(compact ? screen.getByRole("button", { name: "Attach files" }) : screen.getByLabelText("Attach files")).toHaveFocus());
  }
  if (action === "move-focus") expect(screen.getByRole("button", { name: "Another control" })).toHaveFocus();
});
