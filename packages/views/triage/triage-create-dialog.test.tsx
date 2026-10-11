import type { ReactNode } from "react";
import { Editor } from "@tiptap/core";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { configureShortcutPlatform, useShortcutStore } from "@multica/core/shortcuts";
import { projectListOptions } from "@multica/core/projects/queries";
import { memberListOptions } from "@multica/core/workspace/queries";
import type { Attachment } from "@multica/core/types";
import { RESOURCES } from "../locales";
import en from "../locales/en/triage.json";
import zh from "../locales/zh-Hans/triage.json";
import { NavigationProvider } from "../navigation/context";
import { TriageCreateDialog } from "./triage-create-dialog";
import { TriageFieldsEditor } from "./triage-fields";

const { mutate, close, created, upload } = vi.hoisted(() => ({
  mutate: vi.fn(), close: vi.fn(), created: vi.fn(), upload: vi.fn(),
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  // Keep isPending false so the synchronous guard must protect rapid actions.
  useCreateTriageItem: () => ({ mutateAsync: mutate, isPending: false }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/api", async (original) => ({
  ...(await original<typeof import("@multica/core/api")>()),
  api: { getBaseUrl: () => "http://localhost", uploadFile: upload },
}));
vi.mock("@multica/core/auth", async (original) => {
  const state = { user: null };
  return {
    ...(await original<typeof import("@multica/core/auth")>()),
    useAuthStore: Object.assign(
      (selector?: (value: typeof state) => unknown) => selector ? selector(state) : state,
      { getState: () => state },
    ),
  };
});

let client: QueryClient;
const rangeGeometry = ["getClientRects", "getBoundingClientRect"].map((name) => ({
  name, descriptor: Object.getOwnPropertyDescriptor(Range.prototype, name),
}));
beforeAll(() => {
  Object.defineProperty(Range.prototype, "getClientRects", { configurable: true, value: () => [] });
  Object.defineProperty(Range.prototype, "getBoundingClientRect", { configurable: true, value: () => new DOMRect() });
});
afterAll(() => {
  for (const { name, descriptor } of rangeGeometry) {
    if (descriptor) Object.defineProperty(Range.prototype, name, descriptor);
    else Reflect.deleteProperty(Range.prototype, name);
  }
});
beforeEach(() => {
  vi.resetAllMocks();
  configureShortcutPlatform("macos");
  useShortcutStore.setState({ overrides: {} });
  client = new QueryClient({ defaultOptions: { queries: { enabled: false, retry: false, gcTime: 0 } } });
});
afterEach(() => {
  cleanup();
  client.clear();
  useShortcutStore.setState({ overrides: {} });
});

function renderInContext(children: ReactNode, locale: "en" | "zh-Hans" = "en") {
  return render(<QueryClientProvider client={client}>
    <I18nProvider locale={locale} resources={RESOURCES}>
      <WorkspaceSlugProvider slug="test">
        <NavigationProvider value={{ push: () => {}, replace: () => {}, back: () => {}, pathname: "/test/triage", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path }}>
          {children}
        </NavigationProvider>
      </WorkspaceSlugProvider>
    </I18nProvider>
  </QueryClientProvider>);
}

async function renderDialog(locale: "en" | "zh-Hans" = "en") {
  renderInContext(<TriageCreateDialog wsId="ws-1" onClose={close} onCreated={created} />, locale);
  const copy = locale === "en" ? en : zh;
  const title = await screen.findByRole("textbox", { name: copy.title_field });
  const description = await screen.findByRole("textbox", { name: copy.description });
  await waitFor(() => expect(title).toHaveFocus());
  return { title, description, copy };
}

// Use the actual ProseMirror commands to update the real editors. Their onUpdate
// callbacks, accessible attributes and submit shortcut all remain in the path.
function writeEditor(element: HTMLElement, text: string) {
  if (!("editor" in element) || !(element.editor instanceof Editor)) {
    throw new Error("Expected a mounted Tiptap editor");
  }
  const editor = element.editor;
  act(() => {
    editor.commands.setContent({ type: "doc", content: [{ type: "paragraph", ...(text ? { content: [{ type: "text", text }] } : {}) }] });
  });
}
function submitShortcut(title: HTMLElement) {
  fireEvent.keyDown(title, { key: "Enter", code: "Enter", metaKey: true });
}
function changeField(name: string, value: string) {
  fireEvent.change(screen.getByLabelText(name, { exact: true }), { target: { value } });
}
function submitButton() {
  return screen.getByRole("button", { name: en.create });
}

describe("triage create dialog", () => {
  // The limit, calendar, URL and server-message matrices are canonical in
  // triage-create-dialog.validation.test.ts; this suite tests real form wiring.
  it.each([["en", en], ["zh-Hans", zh]] as const)("names the real fields and candidate pickers in %s", async (locale, copy) => {
    const { title, description } = await renderDialog(locale);
    expect(title).toHaveAttribute("contenteditable", "true");
    expect(title).toHaveAttribute("aria-required", "true");
    expect(description).toHaveAttribute("contenteditable", "true");
    for (const name of [copy.priority, copy.candidate_project, copy.candidate_assignee, copy.labels]) {
      expect(screen.getByRole("button", { name })).toBeEnabled();
    }
    for (const name of [copy.start_date, copy.due_date, copy.source_url]) {
      expect(screen.getByLabelText(name, { exact: true })).toBeEnabled();
    }
    expect(screen.getByRole("button", { name: copy.close })).toBeEnabled();
    expect(screen.getByRole("button", { name: copy.create })).toBeDisabled();
  });

  it("blocks a blank-title shortcut and connects URL/date errors to their controls", async () => {
    const { title } = await renderDialog();
    submitShortcut(title);
    expect(await screen.findByText(en.invalid_title)).toBeVisible();
    expect(title).toHaveAttribute("aria-invalid", "true");
    expect(title).toHaveAccessibleDescription(en.invalid_title);
    await waitFor(() => expect(title).toHaveFocus());
    writeEditor(title, "Retain this title");
    await waitFor(() => expect(title).not.toHaveAttribute("aria-invalid"));
    changeField(en.source_url, "http:example.com");
    changeField(en.start_date, "2026-10-12");
    changeField(en.due_date, "2026-10-11");
    fireEvent.click(submitButton());
    expect(await screen.findByText(en.invalid_source_url)).toBeVisible();
    expect(screen.getByLabelText(en.source_url, { exact: true })).toHaveAccessibleDescription(en.invalid_source_url);
    expect(screen.getByLabelText(en.start_date, { exact: true })).toHaveAccessibleDescription(en.start_date_after_due_date);
    expect(screen.getByLabelText(en.due_date, { exact: true })).toHaveAccessibleDescription(en.due_date_before_start_date);
    expect(screen.getByLabelText(en.start_date, { exact: true })).toHaveFocus();
    expect(title).toHaveTextContent("Retain this title");
    expect(mutate).not.toHaveBeenCalled();
    changeField(en.due_date, "2026-10-13");
    expect(screen.queryByText(en.start_date_after_due_date)).not.toBeInTheDocument();
    expect(screen.queryByText(en.due_date_before_start_date)).not.toBeInTheDocument();
  });

  it("validates the current editor contents before sending an overlong draft", async () => {
    const { title, description } = await renderDialog();
    writeEditor(title, "a".repeat(501));
    fireEvent.click(submitButton());
    expect(await screen.findByText(en.invalid_title_length)).toBeVisible();
    expect(title).toHaveAccessibleDescription(en.invalid_title_length);
    writeEditor(title, "Valid title");
    writeEditor(description, "a".repeat((1 << 20) + 1));
    fireEvent.click(submitButton());
    expect(await screen.findByText(en.invalid_description)).toBeVisible();
    expect(description).toHaveAttribute("aria-invalid", "true");
    expect(description).toHaveAccessibleDescription(en.invalid_description);
    expect(mutate).not.toHaveBeenCalled();
  });

  it("preserves input and request identity after failures, maps candidate fields, and closes only on success", async () => {
    const user = userEvent.setup();
    client.setQueryData(projectListOptions("ws-1").queryKey, { projects: [{ id: "project-1", workspace_id: "ws-1", title: "Candidate roadmap", description: null, icon: null, status: "planned", priority: "none", lead_type: null, lead_id: null, start_date: null, due_date: null, created_at: "2026-10-11", updated_at: "2026-10-11", issue_count: 0, done_count: 0, resource_count: 0 }], total: 1 });
    client.setQueryData(memberListOptions("ws-1").queryKey, [{ id: "membership-1", workspace_id: "ws-1", user_id: "member-1", name: "Alex", email: "alex@example.com", role: "member", created_at: "2026-10-11", avatar_url: null }]);
    const item = { issue: { id: "new-issue" } };
    mutate.mockRejectedValueOnce(new Error("Network unavailable"))
      .mockRejectedValueOnce(new Error("Still unavailable"))
      .mockResolvedValueOnce(item);
    const { title, description } = await renderDialog();
    writeEditor(title, "  Review the request  ");
    writeEditor(description, "Preserve the details");
    changeField(en.source_url, " https://example.com/request ");
    changeField(en.start_date, "2026-10-11");
    changeField(en.due_date, "2026-10-12");
    await user.click(screen.getByRole("button", { name: en.candidate_project }));
    await user.click(await screen.findByText("Candidate roadmap", { exact: true }));
    expect(screen.getByRole("button", { name: en.candidate_project })).toHaveAccessibleDescription("Candidate roadmap");
    await user.click(screen.getByRole("button", { name: en.candidate_assignee }));
    await user.click(await screen.findByText("Alex", { exact: true }));
    expect(screen.getByRole("button", { name: en.candidate_assignee })).toHaveAccessibleDescription("Alex");
    fireEvent.click(submitButton());
    expect(await screen.findByText(en.create_failed)).toBeVisible();
    expect(close).not.toHaveBeenCalled();
    expect(created).not.toHaveBeenCalled();
    expect(title).toHaveTextContent("Review the request");
    expect(description).toHaveTextContent("Preserve the details");
    // Native URL inputs trim surrounding ASCII whitespace before onChange.
    expect(screen.getByLabelText(en.source_url, { exact: true })).toHaveValue("https://example.com/request");
    const first = mutate.mock.calls[0]![0];
    expect(first).toMatchObject({ title: "Review the request", description: "Preserve the details", priority: "none", candidate_project_id: "project-1", candidate_assignee_type: "member", candidate_assignee_id: "member-1", start_date: "2026-10-11", due_date: "2026-10-12", source_url: "https://example.com/request", attachment_ids: [] });
    expect(first.request_id).toEqual(expect.any(String));
    fireEvent.click(submitButton());
    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(2));
    await screen.findByText(en.create_failed);
    expect(mutate.mock.calls[1]![0]).toEqual(first);
    writeEditor(title, "A new intention");
    fireEvent.click(submitButton());
    await waitFor(() => expect(close).toHaveBeenCalledOnce());
    expect(mutate.mock.calls[2]![0].request_id).not.toBe(first.request_id);
    expect(created).toHaveBeenCalledWith(item);
    expect(created.mock.invocationCallOrder[0]).toBeLessThan(close.mock.invocationCallOrder[0]!);
  });

  it("fences a rapid button/shortcut pair while the request is unresolved", async () => {
    const item = { issue: { id: "new-issue" } };
    let resolveRequest: (value: typeof item) => void = () => {};
    mutate.mockImplementationOnce(() => new Promise<typeof item>((resolve) => { resolveRequest = resolve; }));
    const { title } = await renderDialog();
    writeEditor(title, "Create once");
    fireEvent.click(submitButton());
    submitShortcut(title);
    fireEvent.click(submitButton());
    expect(mutate).toHaveBeenCalledOnce();
    expect(close).not.toHaveBeenCalled();
    await act(async () => { resolveRequest(item); });
    expect(created).toHaveBeenCalledOnce();
    expect(close).toHaveBeenCalledOnce();
  });

  it("blocks submission during attachment upload and sends only attachments retained in the draft", async () => {
    const attachment: Attachment = {
      id: "attachment-1", workspace_id: "ws-1", issue_id: null, comment_id: null,
      chat_session_id: null, chat_message_id: null, uploader_type: "member", uploader_id: "member-1",
      filename: "context.txt", content_type: "text/plain", size_bytes: 7,
      url: "https://example.com/temporary/context.txt",
      download_url: "https://example.com/api/attachments/attachment-1/download",
      markdown_url: "https://example.com/api/attachments/attachment-1/download",
      created_at: "2026-10-11T00:00:00Z",
    };
    let finishUpload!: (value: Attachment) => void;
    upload.mockImplementationOnce(() => new Promise<Attachment>((resolve) => { finishUpload = resolve; }));
    mutate.mockRejectedValueOnce(new Error("Network unavailable"))
      .mockResolvedValueOnce({ issue: { id: "new-issue" } });
    const { title, description } = await renderDialog();
    writeEditor(title, "Review the attachment");
    const input = screen.getByRole("dialog").querySelector<HTMLInputElement>('input[type="file"]');
    expect(input).not.toBeNull();
    fireEvent.change(input!, { target: { files: [new File(["context"], attachment.filename, { type: attachment.content_type })] } });
    expect(upload).toHaveBeenCalledOnce();
    expect(await screen.findByRole("button", { name: en.uploading })).toBeDisabled();
    expect(screen.getByRole("button", { name: en.cancel })).toBeDisabled();
    submitShortcut(title);
    expect(mutate).not.toHaveBeenCalled();
    await act(async () => { finishUpload(attachment); });
    await waitFor(() => expect(submitButton()).toBeEnabled());
    fireEvent.click(submitButton());
    expect(await screen.findByText(en.create_failed)).toBeVisible();
    const first = mutate.mock.calls[0]![0];
    expect(first.description).toContain(attachment.markdown_url);
    expect(first.attachment_ids).toEqual([attachment.id]);
    writeEditor(description, "Attachment removed from draft");
    fireEvent.click(submitButton());
    await waitFor(() => expect(close).toHaveBeenCalledOnce());
    expect(mutate.mock.calls[1]![0].attachment_ids).toEqual([]);
    expect(mutate.mock.calls[1]![0].request_id).not.toBe(first.request_id);
  });

  it("localizes a known 4xx field error without clearing the valid draft", async () => {
    mutate.mockRejectedValueOnce(new ApiError("source_url must be an HTTP or HTTPS URL", 400, "Bad Request"));
    const { title } = await renderDialog();
    writeEditor(title, "Keep this draft");
    changeField(en.source_url, "https://example.com/request");
    fireEvent.click(submitButton());
    expect(await screen.findByText(en.invalid_source_url)).toBeVisible();
    expect(screen.getByLabelText(en.source_url, { exact: true })).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText(en.source_url, { exact: true })).toHaveAccessibleDescription(en.invalid_source_url);
    expect(screen.getByLabelText(en.source_url, { exact: true })).toHaveFocus();
    expect(title).toHaveTextContent("Keep this draft");
    expect(close).not.toHaveBeenCalled();
  });

  it.each([
    new ApiError("title is required (maximum 500 characters); description maximum is 1 MiB", 400, "Bad Request"),
    new ApiError("source_url must be an HTTP or HTTPS URL", 500, "Server Error", { code: "invalid_source_url" }),
  ])("keeps an ambiguous intake or server failure global without blaming valid fields", async (error) => {
    mutate.mockRejectedValueOnce(error);
    const { title, description } = await renderDialog();
    writeEditor(title, "Valid title");
    writeEditor(description, "Valid description");
    changeField(en.source_url, "https://example.com/request");
    fireEvent.click(submitButton());
    expect(await screen.findByText(en.create_failed)).toBeVisible();
    expect(title).not.toHaveAttribute("aria-invalid");
    expect(description).not.toHaveAttribute("aria-invalid");
    expect(screen.getByLabelText(en.source_url, { exact: true })).not.toHaveAttribute("aria-invalid");
    expect(screen.queryByText(error.message)).not.toBeInTheDocument();
    expect(close).not.toHaveBeenCalled();
  });
});

it.each([["en", en], ["zh-Hans", zh]] as const)("keeps intake and acceptance field names and errors independent in %s", (locale, copy) => {
  renderInContext(<>
    <fieldset aria-label="Intake">
      <TriageFieldsEditor wsId="ws-1" fields={{}} onChange={() => {}} showStatus={false}
        candidateMode pickerAffordance idPrefix="triage-create"
        errors={{ start_date: copy.invalid_start_date, due_date: copy.invalid_due_date }} />
    </fieldset>
    <fieldset aria-label="Acceptance">
      <TriageFieldsEditor wsId="ws-1" fields={{}} onChange={() => {}} showStatus={false}
        errors={{ start_date: copy.start_date_after_due_date, due_date: copy.due_date_before_start_date }} />
    </fieldset>
  </>, locale);
  const intake = within(screen.getByRole("group", { name: "Intake" }));
  const acceptance = within(screen.getByRole("group", { name: "Acceptance" }));
  expect(intake.getByRole("button", { name: copy.candidate_project })).toBeEnabled();
  expect(intake.getByRole("button", { name: copy.candidate_assignee })).toBeEnabled();
  expect(acceptance.getByRole("button", { name: copy.project })).toBeEnabled();
  expect(acceptance.getByRole("button", { name: copy.assignee })).toBeEnabled();
  expect(intake.getByLabelText(copy.start_date, { exact: true })).toHaveAccessibleDescription(copy.invalid_start_date);
  expect(acceptance.getByLabelText(copy.start_date, { exact: true })).toHaveAccessibleDescription(copy.start_date_after_due_date);
  expect(intake.getByLabelText(copy.due_date, { exact: true })).toHaveAccessibleDescription(copy.invalid_due_date);
  expect(acceptance.getByLabelText(copy.due_date, { exact: true })).toHaveAccessibleDescription(copy.due_date_before_start_date);
});
