import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import { ResourceEditor } from "./resource-editor";
import en from "../../locales/en/admin.json";
import zh from "../../locales/zh-Hans/admin.json";
const state = vi.hoisted(() => ({ preview: vi.fn(), mutate: vi.fn(), lookup: vi.fn(), close: vi.fn(), settled: vi.fn(), reset: vi.fn(), busy: vi.fn(), Uncertain: class extends Error {} }));
vi.mock("@multica/core/admin", () => ({
  useAdminResourcePreview: () => ({ mutateAsync: state.preview, reset: state.reset }),
  useAdminResourceMutation: () => ({ mutateAsync: state.mutate, reset: state.reset }),
  useAdminResourceLookup: () => ({ mutateAsync: state.lookup, reset: state.reset }),
  AdminResourceUncertainError: state.Uncertain,
  AdminResourceUnsupportedError: class extends Error {},
}));
const resource = { kind: "skill" as const, key: "sample", name: "Sample", description: "Description", source: "managed" as const, state: "published" as const, version: "r1", contentDigest: "d1", fileCount: 2, byteCount: 100, updatedAt: null, updatedBy: null };
const limits = { maxUploadBytes: 16777216, maxPrimaryBytes: 1048576, maxFileBytes: 1048576, maxSupportingBytes: 8388608, maxTotalBytes: 9437184, maxFiles: 257, maxArchiveEntries: 512, maxMcpBytes: 65536 };
const preview = { resource, files: [{ path: "SKILL.md", size: 60 }, { path: "references/style.md", size: 40 }], preview: "# Sample\nUploaded content", previewDigest: "digest", expectedVersion: null };
function editor(locale: "en" | "zh-Hans" = "en", action: "publish" | "withdraw" = "publish", disabled = false) {
  return <I18nProvider locale={locale} resources={{ en: { admin: en }, "zh-Hans": { admin: zh } }}><ResourceEditor kind="skill" scope={{ apiScope: "server", userId: "actor", organizationId: "org" }} action={action} target={action === "withdraw" ? resource : undefined} limits={limits} disabled={disabled} onClose={state.close} onBusy={state.busy} onSuccess={state.settled} /></I18nProvider>;
}
function selectFile() {
  fireEvent.change(screen.getByLabelText(en.resources.key), { target: { value: "sample" } });
  fireEvent.change(screen.getByLabelText(en.resources.file), { target: { files: [new File(["content"], "SKILL.md", { type: "text/markdown" })] } });
}
beforeEach(() => { vi.resetAllMocks(); state.preview.mockResolvedValue(preview); });
it("validates the upload, shows its full inventory, and only publishes after reason and explicit confirmation", async () => {
  state.mutate.mockResolvedValue({ resource, operationId: "operation", replayed: false });
  render(editor()); selectFile();
  expect(screen.getByRole("heading", { name: en.resources.new })).toHaveFocus();
  fireEvent.click(screen.getByRole("button", { name: en.resources.preview }));
  await screen.findByText("references/style.md");
  expect(screen.getByText("# Sample Uploaded content", { exact: false })).toBeInTheDocument();
  expect(state.mutate).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: en.resources.confirmPublish }));
  expect(screen.getByLabelText(en.resources.reason)).toHaveFocus();
  fireEvent.change(screen.getByLabelText(en.resources.reason), { target: { value: "New team guidance" } });
  fireEvent.click(screen.getByRole("button", { name: en.resources.confirmPublish }));
  await waitFor(() => expect(state.settled).toHaveBeenCalled());
  expect(state.mutate).toHaveBeenCalledWith(expect.objectContaining({ key: "sample", action: "publish", expectedVersion: null, previewDigest: "digest", reason: "New team guidance" }));
});
it("preserves the file on recoverable failures and clears preview when the file changes", async () => {
  state.preview.mockRejectedValueOnce(new ApiError("Invalid", 400, "", { code: "resource_invalid" }));
  render(editor()); selectFile();
  fireEvent.click(screen.getByRole("button", { name: en.resources.preview }));
  await screen.findByRole("alert");
  fireEvent.click(screen.getByRole("button", { name: en.resources.preview }));
  await screen.findByText("references/style.md");
  expect(state.preview.mock.calls[0]![0].file).toBe(state.preview.mock.calls[1]![0].file);
  fireEvent.change(screen.getByLabelText(en.resources.file), { target: { files: [new File(["replacement"], "second.skill")] } });
  expect(screen.queryByRole("button", { name: en.resources.confirmPublish })).not.toBeInTheDocument();
});
it("freezes an unknown operation and checks its original receipt without a second write", async () => {
  state.mutate.mockRejectedValueOnce(new state.Uncertain("uncertain"));
  state.lookup.mockResolvedValue(null);
  render(editor()); selectFile();
  fireEvent.click(screen.getByRole("button", { name: en.resources.preview }));
  await screen.findByText("references/style.md");
  fireEvent.change(screen.getByLabelText(en.resources.reason), { target: { value: "New guidance" } });
  fireEvent.click(screen.getByRole("button", { name: en.resources.confirmPublish }));
  await screen.findByText(en.resources.unknownOutcome);
  expect(screen.getByRole("button", { name: en.resources.confirmPublish })).toBeDisabled();
  expect(screen.getByRole("button", { name: en.resources.cancel })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: en.resources.check }));
  await screen.findByText(en.resources.pendingReceipt);
  expect(state.lookup).toHaveBeenCalledWith(state.mutate.mock.calls[0]![0]);
  expect(state.mutate).toHaveBeenCalledTimes(1);
});
it("offers a localized withdrawal confirmation without requesting an upload", async () => {
  render(editor("zh-Hans", "withdraw"));
  expect(screen.getByText(zh.resources.withdrawImpact)).toBeInTheDocument();
  expect(screen.queryByLabelText(zh.resources.file)).not.toBeInTheDocument();
  fireEvent.change(screen.getByLabelText(zh.resources.reason), { target: { value: "已过期" } });
  fireEvent.click(screen.getByRole("button", { name: zh.resources.confirmWithdraw }));
  await waitFor(() => expect(state.mutate).toHaveBeenCalledWith(expect.objectContaining({ action: "withdraw", expectedVersion: "r1" })));
});

it("ignores a cancelled preview that finishes after selecting a different file", async () => {
  let release!: (value: typeof preview) => void;
  state.preview.mockImplementationOnce(() => new Promise(resolve => { release = resolve; }));
  render(editor()); selectFile();
  fireEvent.click(screen.getByRole("button", { name: en.resources.preview }));
  fireEvent.change(screen.getByLabelText(en.resources.file), { target: { files: [new File(["replacement"], "new.skill")] } });
  release(preview);
  await waitFor(() => expect(state.preview.mock.calls[0]![0].signal.aborted).toBe(true));
  expect(screen.queryByText("references/style.md")).not.toBeInTheDocument();
});

it("keeps receipt recovery available when the catalog becomes unavailable", async () => {
  state.mutate.mockRejectedValueOnce(new state.Uncertain("uncertain"));
  const view = render(editor()); selectFile();
  fireEvent.click(screen.getByRole("button", { name: en.resources.preview }));
  await screen.findByText("references/style.md");
  fireEvent.change(screen.getByLabelText(en.resources.reason), { target: { value: "Publish" } });
  fireEvent.click(screen.getByRole("button", { name: en.resources.confirmPublish }));
  await screen.findByText(en.resources.unknownOutcome);
  view.rerender(editor("en", "publish", true));
  expect(screen.getByRole("button", { name: en.resources.check })).toBeEnabled();
});
