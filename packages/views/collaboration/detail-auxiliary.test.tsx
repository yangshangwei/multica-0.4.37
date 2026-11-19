import { useEffect, useRef, useState } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, type CollaborationIssue } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import type { AccountDestination } from "@multica/core/paths";
import { renderWithI18n } from "../test/i18n";
import type { CollaborationPanelStatusChange } from "./panel-status";
import { CollaborationDetailAuxiliary, type CollaborationDetailAuxiliaryHandle } from "./detail-auxiliary";

// Existing panel suites own writes/receipts/privacy. This suite owns the real
// modal lifetime, hidden content, presentation callbacks and navigation handoff.
const fileState = vi.hoisted(() => ({ attention: false, count: undefined as number | undefined, mounts: 0 }));
vi.mock("./attachment-panel", () => ({ CollaborationAttachmentPanel: function Files({ onStatusChange }: { onStatusChange?: CollaborationPanelStatusChange }) {
  const [draft, setDraft] = useState("");
  useEffect(() => { fileState.mounts += 1; }, []);
  const { attention, count } = fileState;
  useEffect(() => { onStatusChange?.({ attention, count }); }, [onStatusChange, attention, count]);
  return <section aria-label="File editor"><label>File note<input value={draft} onChange={(e) => setDraft(e.target.value)} /></label></section>;
} }));
vi.mock("./memory-panel", () => ({ CollaborationMemoryPanel: ({ active }: { active: boolean }) => <section id="memory" tabIndex={-1}><p>{active ? "Visible memory" : "Hidden memory"}</p></section> }));
vi.mock("./knowledge-panel", () => ({ CollaborationKnowledgePanel: ({ active }: { active: boolean }) => <section><p>{active ? "Visible knowledge" : "Hidden knowledge"}</p><button>Review candidate</button></section> }));
vi.mock("./field-summary", () => ({ CollaborationFieldSummary: () => null }));
vi.mock("./activity", () => ({ CollaborationActivity: () => <section>Recorded history</section> }));
vi.mock("./children", () => ({ CollaborationChildren: () => <section>Child references</section> }));
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
const destination: AccountDestination = { view: "participating", issueId: issue.id, commentId: null };
function resources() { return screen.getByRole("group", { name: "Issue resources" }); }
function categories() { return screen.getByRole("group", { name: "Browse issue resources" }); }
afterEach(() => { fileState.attention = false; fileState.count = undefined; fileState.mounts = 0; });
function setup(props: Partial<Parameters<typeof CollaborationDetailAuxiliary>[0]> = {}) {
  setApiInstance(new ApiClient("https://drawer.example"));
  return <CollaborationDetailAuxiliary scope={collaborationScope(issue.creator.user_id)} issue={issue} destination={destination} {...props} />;
}

it("keeps one editor mounted through category switches and close, with inactive content hidden", async () => {
  const user = userEvent.setup();
  renderWithI18n(setup());
  expect(within(resources()).getAllByRole("button")).toHaveLength(6);
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.querySelector('[data-slot="dialog-content"]')).not.toBeVisible();
  expect(document.querySelector('[data-slot="dialog-overlay"]')).not.toBeVisible();
  expect(screen.queryByRole("textbox", { name: "File note" })).toBeNull();
  expect(fileState.mounts).toBe(1);
  const files = within(resources()).getByRole("button", { name: "Attachments" });
  await user.click(files);
  expect(screen.getByRole("dialog", { name: "Attachments" })).toBeVisible();
  await user.type(screen.getByRole("textbox", { name: "File note" }), "Keep this draft");
  await user.click(within(categories()).getByRole("button", { name: "Shared context" }));
  expect(screen.queryByRole("textbox", { name: "File note" })).toBeNull();
  expect(screen.getByText("Visible memory")).toBeVisible();
  await user.click(within(categories()).getByRole("button", { name: "Attachments" }));
  expect(screen.getByRole("textbox", { name: "File note" })).toHaveValue("Keep this draft");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(files).toHaveFocus());
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByRole("textbox", { name: "File note" })).toBeNull();
  await user.click(files);
  expect(screen.getByRole("textbox", { name: "File note" })).toHaveValue("Keep this draft");
  expect(fileState.mounts).toBe(1);
});

it("hides the complete drawer and backdrop with its close button while retaining drafts for reopening", async () => {
  const user = userEvent.setup();
  renderWithI18n(setup());
  const files = within(resources()).getByRole("button", { name: "Attachments" });
  await user.click(files);
  const drawer = screen.getByRole("dialog", { name: "Attachments" });
  const backdrop = document.querySelector('[data-slot="dialog-overlay"]');
  const editor = screen.getByRole("textbox", { name: "File note" });
  await user.type(editor, "Keep this draft after closing");
  await user.click(within(categories()).getByRole("button", { name: "Activity" }));
  expect(drawer).toBeVisible();
  expect(backdrop).toBeVisible();
  expect(screen.getByText("Recorded history")).toBeVisible();

  await user.click(within(drawer).getByRole("button", { name: "Close resources" }));
  await waitFor(() => {
    expect(drawer).not.toBeVisible();
    expect(backdrop).not.toBeVisible();
    expect(files).toHaveFocus();
  });
  expect(screen.queryByRole("button", { name: "Close resources" })).toBeNull();
  expect(drawer).toBeInTheDocument();
  expect(editor).toBeInTheDocument();

  await user.click(within(resources()).getByRole("button", { name: "Activity" }));
  expect(screen.getByRole("dialog", { name: "Activity" })).toBe(drawer);
  expect(backdrop).toBeVisible();
  expect(screen.getByText("Recorded history")).toBeVisible();
  await user.click(within(categories()).getByRole("button", { name: "Attachments" }));
  expect(screen.getByRole("textbox", { name: "File note" })).toBe(editor);
  expect(editor).toHaveValue("Keep this draft after closing");
  expect(fileState.mounts).toBe(1);
});

it("reports attention without opening a modal and only displays confirmed counts", async () => {
  const user = userEvent.setup();
  const view = renderWithI18n(setup());
  expect(within(resources()).getByRole("button", { name: "Attachments" })).toBeVisible();
  fileState.count = 3; fileState.attention = true;
  view.rerender(setup());
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(within(resources()).getByRole("button", { name: "Attachments 3" })).toHaveAccessibleDescription(/Needs attention/);
  await user.click(within(screen.getByRole("status")).getByRole("button", { name: "Attachments" }));
  expect(screen.getByRole("dialog", { name: "Attachments" })).toBeVisible();
  await user.keyboard("{Escape}");
  fileState.count = undefined; fileState.attention = false;
  view.rerender(setup());
  expect(screen.queryByRole("status")).toBeNull();
  expect(within(resources()).getByRole("button", { name: "Attachments" })).toBeVisible();
  expect(within(resources()).queryByRole("button", { name: /Attachments 0/ })).toBeNull();
});

it("passes knowledge visibility through open, category switch and close without unmounting", async () => {
  const user = userEvent.setup();
  renderWithI18n(setup());
  const hidden = screen.getByText("Hidden knowledge");
  expect(hidden).not.toBeVisible();
  const knowledge = within(resources()).getByRole("button", { name: "Knowledge candidates" });
  await user.click(knowledge);
  expect(screen.getByText("Visible knowledge")).toBeVisible();
  const panel = screen.getByText("Visible knowledge").closest("section");
  await user.click(within(categories()).getByRole("button", { name: "Shared context" }));
  expect(screen.getByText("Hidden knowledge").closest("section")).toBe(panel);
  expect(screen.getByText("Hidden knowledge")).not.toBeVisible();
  await user.click(within(categories()).getByRole("button", { name: "Knowledge candidates" }));
  expect(screen.getByText("Visible knowledge").closest("section")).toBe(panel);
  await user.keyboard("{Escape}");
  await waitFor(() => expect(knowledge).toHaveFocus());
  expect(screen.getByText("Hidden knowledge").closest("section")).toBe(panel);
  expect(screen.getByText("Hidden knowledge")).not.toBeVisible();
});

it("returns focus to an external opener and waits for the modal to close before main navigation", async () => {
  const user = userEvent.setup();
  setApiInstance(new ApiClient("https://drawer.example"));
  function Journey() {
    const drawer = useRef<CollaborationDetailAuxiliaryHandle>(null);
    const main = useRef<HTMLHeadingElement>(null);
    return <><h2 ref={main} tabIndex={-1}>Main destination</h2><button onClick={(e) => drawer.current?.open("files", e.currentTarget)}>Reference files</button>
      <button onClick={() => drawer.current?.closeForNavigation(() => main.current?.focus())}>Follow main link</button>
      <CollaborationDetailAuxiliary ref={drawer} scope={collaborationScope(issue.creator.user_id)} issue={issue} destination={destination} /></>;
  }
  renderWithI18n(<Journey />);
  const trigger = screen.getByRole("button", { name: "Reference files" });
  await user.click(trigger);
  await user.keyboard("{Escape}");
  await waitFor(() => expect(trigger).toHaveFocus());
  await user.click(trigger);
  // External navigation can arrive from a notification/platform adapter even
  // while background controls are inert, so invoke its real callback directly.
  screen.getByRole("button", { name: "Follow main link", hidden: true }).click();
  await waitFor(() => expect(screen.getByRole("heading", { name: "Main destination" })).toHaveFocus());
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("does not reopen on scope resume and fences editor lifetime by issue identity", async () => {
  const user = userEvent.setup();
  const current = (value: CollaborationIssue) => setup({ key: value.id, issue: value, destination: { ...destination, issueId: value.id, anchor: "memory" }, isScopeResume: true });
  const view = renderWithI18n(current(issue));
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(within(resources()).getByRole("button", { name: "Attachments" }));
  await user.type(screen.getByRole("textbox", { name: "File note" }), "Private first issue");
  view.rerender(current({ ...issue, id: "00000000-0000-4000-8000-000000000009" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(within(resources()).getByRole("button", { name: "Attachments" }));
  expect(screen.getByRole("textbox", { name: "File note" })).toHaveValue("");
});
