import { useState, type ComponentProps } from "react";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../test/i18n";
import { IssuePickerModal } from "./issue-picker-modal";

const mocks = vi.hoisted(() => ({ searchIssues: vi.fn() }));

vi.mock("@multica/core/api", () => ({
  api: { searchIssues: mocks.searchIssues },
}));

function issue(overrides: Partial<Issue> = {}): Issue {
  return {
    id: "issue-1",
    workspace_id: "workspace-1",
    number: 1,
    identifier: "MUL-1",
    title: "Link this issue",
    description: null,
    status: "todo",
    priority: "none",
    assignee_type: "member",
    assignee_id: "member-1",
    creator_type: "member",
    creator_id: "member-1",
    parent_issue_id: null,
    project_id: null,
    position: 0,
    stage: null,
    start_date: null,
    due_date: null,
    metadata: {},
    properties: {},
    created_at: "2026-09-28T00:00:00Z",
    updated_at: "2026-09-28T00:00:00Z",
    ...overrides,
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

const pickerProps = {
  title: "Link an existing issue",
  description: "Only issues without a project can be linked.",
  excludeIds: [],
};

function Picker({
  onOpenChange,
  ...props
}: Pick<ComponentProps<typeof IssuePickerModal>, "onSelect" | "onOpenChange">) {
  const [open, setOpen] = useState(true);
  return (
    <IssuePickerModal
      {...pickerProps}
      {...props}
      open={open}
      onOpenChange={(nextOpen) => {
        onOpenChange(nextOpen);
        setOpen(nextOpen);
      }}
    />
  );
}

async function search() {
  fireEvent.change(screen.getByRole("combobox"), { target: { value: "issue" } });
  return screen.findByRole("option", { name: /Link this issue/ });
}

describe("IssuePickerModal", () => {
  beforeEach(() => {
    mocks.searchIssues.mockReset();
    mocks.searchIssues.mockResolvedValue({ issues: [issue()] });
  });

  it("waits for selection to succeed before closing and blocks duplicate selections", async () => {
    const selection = deferred<boolean>();
    const onSelect = vi.fn(() => selection.promise);
    const onOpenChange = vi.fn();
    renderWithI18n(<Picker onSelect={onSelect} onOpenChange={onOpenChange} />);
    const option = await search();

    fireEvent.click(option);
    fireEvent.click(option);

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith(issue());
    expect(onOpenChange).not.toHaveBeenCalled();
    expect(option).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByRole("combobox")).toBeDisabled();

    await act(async () => selection.resolve(true));

    expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("preserves synchronous selection callbacks", async () => {
    const onSelect = vi.fn();
    const onOpenChange = vi.fn();
    renderWithI18n(<Picker onSelect={onSelect} onOpenChange={onOpenChange} />);

    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAccessibleDescription(pickerProps.description);
    expect(within(dialog).getByText(pickerProps.description)).toHaveAttribute("aria-hidden", "true");

    fireEvent.click(await search());

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false));
    expect(onSelect).toHaveBeenCalledExactlyOnceWith(issue());
  });

  it.each([
    { name: "synchronous", result: () => false },
    { name: "asynchronous", result: () => Promise.resolve(false) },
  ])("keeps the picker available for retry after a $name false result", async ({ result }) => {
    const onSelect = vi.fn(result);
    const onOpenChange = vi.fn();
    renderWithI18n(<Picker onSelect={onSelect} onOpenChange={onOpenChange} />);
    const option = await search();

    fireEvent.click(option);

    await waitFor(() => expect(screen.getByRole("combobox")).not.toBeDisabled());
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(onOpenChange).not.toHaveBeenCalled();
    fireEvent.click(option);
    await waitFor(() => expect(onSelect).toHaveBeenCalledTimes(2));
  });

  it("shows a recoverable error for rejection and lets the same selection succeed on retry", async () => {
    const onSelect = vi.fn()
      .mockRejectedValueOnce(new Error("Failed to persist selection"))
      .mockResolvedValueOnce(undefined);
    const onOpenChange = vi.fn();
    renderWithI18n(<Picker onSelect={onSelect} onOpenChange={onOpenChange} />);

    fireEvent.click(await search());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Could not select this issue. Please try again.",
    );
    expect(screen.getByRole("combobox")).toHaveValue("issue");
    expect(screen.getByRole("combobox")).not.toBeDisabled();
    expect(onOpenChange).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("option", { name: /Link this issue/ }));

    await waitFor(() => expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false));
    expect(onSelect).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("blocks Escape during selection and restores dismissal after a false result", async () => {
    const selection = deferred<boolean>();
    const onOpenChange = vi.fn();
    renderWithI18n(
      <Picker onSelect={() => selection.promise} onOpenChange={onOpenChange} />,
    );
    fireEvent.click(await search());

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();

    await act(async () => selection.resolve(false));
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
  });

  it("ignores a stale successful selection after the parent closes and reopens the picker", async () => {
    const selection = deferred<boolean>();
    const props = {
      ...pickerProps,
      onSelect: () => selection.promise,
      onOpenChange: vi.fn(),
    };
    const { rerender } = renderWithI18n(<IssuePickerModal {...props} open />);
    fireEvent.click(await search());

    rerender(<IssuePickerModal {...props} open={false} />);
    rerender(<IssuePickerModal {...props} open />);
    await act(async () => selection.resolve(true));

    expect(props.onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("combobox")).not.toBeDisabled();
    expect(screen.getByRole("combobox")).toHaveValue("");
  });

  it("combines optional candidate filtering with excluded IDs", async () => {
    mocks.searchIssues.mockResolvedValue({
      issues: [
        issue(),
        issue({ id: "current-project-issue", title: "Already linked", project_id: "project-1" }),
        issue({ id: "other-project-issue", title: "Other project", project_id: "project-2" }),
        issue({ id: "excluded-issue", title: "Excluded issue" }),
      ],
    });
    const { rerender } = renderWithI18n(
      <IssuePickerModal
        {...pickerProps}
        open
        onOpenChange={vi.fn()}
        onSelect={vi.fn()}
        excludeIds={["excluded-issue"]}
        filterIssue={(candidate) => candidate.project_id === null}
      />,
    );

    await search();

    expect(screen.getAllByRole("option")).toHaveLength(1);

    rerender(
      <IssuePickerModal
        {...pickerProps}
        open
        onOpenChange={vi.fn()}
        onSelect={vi.fn()}
        filterIssue={() => false}
      />,
    );

    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    expect(screen.getByText("No issues found.")).toBeInTheDocument();
  });
});
