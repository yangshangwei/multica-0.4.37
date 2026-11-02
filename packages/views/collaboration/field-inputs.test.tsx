import { useState } from "react";
import { expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { CollaborationSourceOptions } from "@multica/core/collaboration";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@multica/ui/components/ui/dialog";
import { renderWithI18n } from "../test/i18n";
import { CollaborationSourceInputs, type CollaborationSourceSelection } from "./field-inputs";

const parent = { id: "00000000-0000-4000-8000-000000000001", identifier: "SRC-12", name: "Prepare the release" };
const otherParent = { id: "00000000-0000-4000-8000-000000000002", identifier: "SRC-11", name: "Review the design" };
const options: CollaborationSourceOptions = { projects: [], parents: [parent, otherParent], parents_truncated: false, iterations: [], iterations_enabled: false };

// Backend source-fields tests own title/number/identifier matching. This suite
// exercises the actual shared picker primitives and their observable behavior.
function Harness({ pending = false, error, onRetry = vi.fn(), onChange = vi.fn(), onSubmit = vi.fn(), sourceOptions = options }: {
  pending?: boolean; error?: unknown; onRetry?: () => void; onChange?: (value: CollaborationSourceSelection) => void;
  onSubmit?: () => void; sourceOptions?: CollaborationSourceOptions;
}) {
  const [search, setSearch] = useState("");
  const [selection, setSelection] = useState<CollaborationSourceSelection>({ projectId: null, parentIssueId: null, stage: null });
  const displayed = { ...sourceOptions, parents: sourceOptions.parents.filter((item) => `${item.identifier} ${item.name}`.toLowerCase().includes(search.toLowerCase())) };
  return <form onSubmit={(event) => { event.preventDefault(); onSubmit(); }}>
    <label>Draft title<input defaultValue="Keep this title" /></label>
    <CollaborationSourceInputs options={displayed} value={selection} onChange={(value) => { setSelection(value); onChange(value); }}
      parentSearch={search} onParentSearch={setSearch} parentsPending={pending} parentsError={error} onRetryParents={onRetry} disabled={false} />
    <button type="submit">Submit</button>
  </form>;
}

it("opens recent parent tasks without searching, selects the UUID, and returns focus without submitting", async () => {
  const onChange = vi.fn(), onSubmit = vi.fn();
  const user = userEvent.setup();
  renderWithI18n(<Harness onChange={onChange} onSubmit={onSubmit} />);
  const trigger = screen.getByRole("button", { name: "Parent issue" });
  expect(screen.queryByPlaceholderText("Find source issues by title or number")).not.toBeInTheDocument();
  await user.click(trigger);
  const input = await screen.findByPlaceholderText("Find source issues by title or number");
  expect(input).toHaveValue("");
  expect(input).toHaveAttribute("maxlength", "200");
  await user.click(await screen.findByRole("button", { name: "SRC-12 Prepare the release" }));
  expect(onChange).toHaveBeenCalledExactlyOnceWith({ projectId: null, parentIssueId: parent.id, stage: null });
  expect(trigger).toHaveTextContent("SRC-12");
  expect(trigger).toHaveTextContent("Prepare the release");
  await waitFor(() => expect(trigger).toHaveFocus());
  expect(onSubmit).not.toHaveBeenCalled();
});

it("shows search results inline, restores browsing on reopen, and can clear the optional parent", async () => {
  const user = userEvent.setup();
  const onChange = vi.fn();
  renderWithI18n(<Harness onChange={onChange} />);
  const trigger = screen.getByRole("button", { name: "Parent issue" });
  await user.click(trigger);
  await user.type(await screen.findByPlaceholderText("Find source issues by title or number"), "release");
  expect(screen.queryByRole("button", { name: "SRC-11 Review the design" })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "SRC-12 Prepare the release" }));
  expect(trigger).toHaveTextContent("Prepare the release");
  await user.click(trigger);
  expect(await screen.findByPlaceholderText("Find source issues by title or number")).toHaveValue("");
  expect(await screen.findByRole("button", { name: "SRC-11 Review the design" })).toBeVisible();
  await user.click(screen.getByRole("button", { name: "No parent issue" }));
  expect(onChange).toHaveBeenLastCalledWith({ projectId: null, parentIssueId: null, stage: null });
  expect(trigger).toHaveTextContent("Choose a parent issue (optional)");
});

it("keeps pending or failed search rows unselectable and retries without losing other input", async () => {
  const user = userEvent.setup();
  const onRetry = vi.fn(), onChange = vi.fn();
  const view = renderWithI18n(<Harness pending onRetry={onRetry} onChange={onChange} />);
  const trigger = screen.getByRole("button", { name: "Parent issue" });
  await user.click(trigger);
  expect(await screen.findByText("Loading tasks...")).toBeVisible();
  expect(screen.queryByRole("button", { name: "SRC-12 Prepare the release" })).not.toBeInTheDocument();
  const input = screen.getByPlaceholderText("Find source issues by title or number");
  await user.click(input);
  await user.keyboard("{ArrowDown}{Enter}");
  expect(onChange).not.toHaveBeenCalled();
  view.rerender(<Harness error={new Error("Network unavailable")} onRetry={onRetry} onChange={onChange} />);
  expect(await screen.findByText("Tasks could not be loaded. Try again.")).toBeVisible();
  expect(screen.queryByRole("button", { name: "SRC-12 Prepare the release" })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Retry loading tasks" }));
  expect(onRetry).toHaveBeenCalledOnce();
  expect(screen.getByLabelText("Draft title")).toHaveValue("Keep this title");
  expect(input).toHaveFocus();
});

it("distinguishes an empty workspace from an empty search and explains truncated browsing", async () => {
  const user = userEvent.setup();
  const view = renderWithI18n(<Harness sourceOptions={{ ...options, parents: [] }} />);
  const trigger = screen.getByRole("button", { name: "Parent issue" });
  await user.click(trigger);
  expect(await screen.findByText("No tasks are available in this source workspace.")).toBeVisible();
  await user.type(screen.getByPlaceholderText("Find source issues by title or number"), "missing");
  expect(await screen.findByText("No matching tasks.")).toBeVisible();
  view.rerender(<Harness sourceOptions={{ ...options, parents_truncated: true }} />);
  await user.clear(screen.getByPlaceholderText("Find source issues by title or number"));
  expect(await screen.findByText("Showing the latest 200 matches. Refine the search to find another issue.")).toBeVisible();
});

it("uses arrow/Enter selection, keeps composing Enter untouched, and dismisses only the nested picker", async () => {
  const user = userEvent.setup();
  const onChange = vi.fn(), onSubmit = vi.fn();
  const onDialogChange = vi.fn();
  renderWithI18n(<Dialog open onOpenChange={onDialogChange}><DialogContent closeLabel="Close">
    <DialogTitle>New collaboration</DialogTitle><DialogDescription>Choose optional source fields.</DialogDescription>
    <Harness onChange={onChange} onSubmit={onSubmit} />
  </DialogContent></Dialog>);
  const trigger = screen.getByRole("button", { name: "Parent issue" });
  await user.click(trigger);
  const input = await screen.findByPlaceholderText("Find source issues by title or number");
  await user.click(input);
  await user.keyboard("{ArrowDown}");
  fireEvent.keyDown(input, { key: "Enter", isComposing: true });
  expect(onChange).not.toHaveBeenCalled();
  await user.keyboard("{Enter}");
  expect(onChange).toHaveBeenCalledExactlyOnceWith({ projectId: null, parentIssueId: parent.id, stage: null });
  await user.click(trigger);
  await screen.findByPlaceholderText("Find source issues by title or number");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(trigger).toHaveFocus());
  expect(screen.getByLabelText("Draft title")).toHaveValue("Keep this title");
  expect(onSubmit).not.toHaveBeenCalled();
  expect(onDialogChange).not.toHaveBeenCalledWith(false, expect.anything());
});

it("explains the optional parent relationship in Chinese", () => {
  renderWithI18n(<Harness />, { locale: "zh-Hans" });
  expect(screen.getByRole("button", { name: "父任务" })).toHaveTextContent("选择父任务（可选）");
  expect(screen.getByText("可选。选择后，这项协作任务会成为它的子任务。")).toBeVisible();
});

it("keeps the restored selected name when filtered out and tolerates an older response without an identifier", () => {
  renderWithI18n(<CollaborationSourceInputs options={{ ...options, parents: [] }}
    value={{ projectId: null, parentIssueId: parent.id, stage: null }} onChange={vi.fn()} parentSearch="another task" onParentSearch={vi.fn()} disabled={false}
    current={{ issue_revision: "1", project: null, parent_issue: { id: parent.id, name: "Restored source parent" }, iteration: null, stage: null }} />);
  const trigger = screen.getByRole("button", { name: "Parent issue" });
  expect(trigger).toHaveTextContent("Restored source parent");
  expect(trigger).not.toHaveTextContent("undefined");
});
