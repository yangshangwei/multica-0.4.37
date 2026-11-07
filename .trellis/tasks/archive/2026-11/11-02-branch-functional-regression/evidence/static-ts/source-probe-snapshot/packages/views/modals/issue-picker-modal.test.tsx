import { useState, type ComponentProps } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { projectIssueCandidatesOptions } from "@multica/core/projects/queries";
import { renderWithI18n } from "../test/i18n";
import { IssuePickerModal } from "./issue-picker-modal";

const mocks = vi.hoisted(() => ({ searchIssues: vi.fn(), listIssues: vi.fn(), getIssue: vi.fn() }));

vi.mock("@multica/core/api", async (original) => ({
  ...(await original<typeof import("@multica/core/api")>()),
  api: { searchIssues: mocks.searchIssues, listIssues: mocks.listIssues, getIssue: mocks.getIssue },
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
}: Pick<ComponentProps<typeof IssuePickerModal>, "onSelect" | "onOpenChange" | "browseOptions">) {
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
    expect(screen.getByRole("combobox")).toHaveAccessibleName(pickerProps.title);
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

const browseOptions = (search: string) => projectIssueCandidatesOptions("workspace-1", search);

function BrowsePicker(props: Pick<ComponentProps<typeof IssuePickerModal>, "onSelect" | "onOpenChange">) {
  const [client] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } }));
  return <QueryClientProvider client={client}>
    <Picker {...props} browseOptions={browseOptions} />
  </QueryClientProvider>;
}

describe("IssuePickerModal browsing", () => {
  beforeEach(() => {
    mocks.listIssues.mockReset().mockResolvedValue({ issues: [issue()], total: 1 });
    mocks.getIssue.mockReset();
    mocks.searchIssues.mockReset();
  });

  it("loads candidates immediately and selects a row with the keyboard", async () => {
    const onSelect = vi.fn();
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    renderWithI18n(<BrowsePicker onSelect={onSelect} onOpenChange={onOpenChange} />);

    await screen.findByRole("option", { name: /Link this issue/ });
    expect(screen.getByRole("combobox")).toHaveValue("");
    expect(screen.getByRole("combobox")).toHaveAccessibleName(pickerProps.title);
    expect(screen.getByRole("listbox")).toHaveAccessibleName("Eligible issues");
    expect(mocks.searchIssues).not.toHaveBeenCalled();
    expect(screen.queryByText("Type to search issues")).not.toBeInTheDocument();
    await user.click(screen.getByRole("combobox"));
    await user.keyboard("{ArrowDown}{Enter}");

    expect(onSelect).toHaveBeenCalledExactlyOnceWith(issue());
    expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
  });

  it("hides previous candidates while a new search waits and restores browsing after clearing", async () => {
    const oldSearch = deferred<{ issues: Issue[]; total: number }>();
    mocks.listIssues.mockImplementation(async (params: { q: string }) => {
      if (params.q === "old") return oldSearch.promise;
      return { issues: [params.q === "latest" ? issue({ id: "latest", title: "Latest result" }) : issue()], total: 1 };
    });
    renderWithI18n(<BrowsePicker onSelect={vi.fn()} onOpenChange={vi.fn()} />);
    await screen.findByRole("option", { name: /Link this issue/ });
    const input = screen.getByRole("combobox");

    fireEvent.change(input, { target: { value: "old" } });
    expect(screen.queryByRole("option")).not.toBeInTheDocument();
    await waitFor(() => expect(mocks.listIssues).toHaveBeenCalledWith(expect.objectContaining({ q: "old" })));
    fireEvent.change(input, { target: { value: "latest" } });
    await screen.findByRole("option", { name: /Latest result/ });
    await act(async () => oldSearch.resolve({ issues: [issue({ id: "stale", title: "Stale result" })], total: 1 }));
    expect(screen.queryByText("Stale result")).not.toBeInTheDocument();

    fireEvent.change(input, { target: { value: "" } });
    await screen.findByRole("option", { name: /Link this issue/ });
    expect(screen.queryByText("Latest result")).not.toBeInTheDocument();
  });

  it("loads more pages and retries a failed next page without discarding available candidates", async () => {
    mocks.listIssues.mockResolvedValueOnce({ issues: [issue()], total: 2 })
      .mockRejectedValueOnce(new Error("Offline"))
      .mockResolvedValueOnce({ issues: [issue({ id: "second", title: "Second candidate" })], total: 2 });
    const onSelect = vi.fn();
    const user = userEvent.setup();
    renderWithI18n(<BrowsePicker onSelect={onSelect} onOpenChange={vi.fn()} />);
    await screen.findByRole("option", { name: /Link this issue/ });
    const more = screen.getByRole("button", { name: "Load more" });
    more.focus();
    await user.keyboard("{Enter}");

    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load more issues.");
    expect(screen.getByRole("option", { name: /Link this issue/ })).toBeInTheDocument();
    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.getByRole("combobox")).toHaveFocus();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(screen.getByRole("combobox")).toHaveFocus();
    await screen.findByRole("option", { name: /Second candidate/ });

    expect(screen.getAllByRole("option")).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
    expect(mocks.listIssues.mock.calls.map(([params]) => params.offset)).toEqual([0, 1, 1]);
  });

  it("distinguishes initial loading, recoverable errors and an empty eligible list", async () => {
    const firstPage = deferred<{ issues: Issue[]; total: number }>();
    mocks.listIssues.mockReturnValueOnce(firstPage.promise)
      .mockResolvedValueOnce({ issues: [], total: 0 });
    renderWithI18n(<BrowsePicker onSelect={vi.fn()} onOpenChange={vi.fn()} />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading issues...");
    await act(async () => firstPage.reject(new Error("Offline")));

    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load issues.");
    expect(screen.queryByText("No eligible issues available.")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("No eligible issues available.")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("keeps the modal open when linking fails and disables all controls during selection", async () => {
    const selection = deferred<boolean>();
    const onSelect = vi.fn().mockReturnValueOnce(selection.promise).mockResolvedValueOnce(true);
    const onOpenChange = vi.fn();
    mocks.listIssues.mockResolvedValue({ issues: [issue()], total: 2 });
    renderWithI18n(<BrowsePicker onSelect={onSelect} onOpenChange={onOpenChange} />);
    const option = await screen.findByRole("option", { name: /Link this issue/ });
    fireEvent.click(option);

    expect(screen.getByRole("combobox")).toBeDisabled();
    expect(screen.getByRole("button", { name: "Load more" })).toBeDisabled();
    fireEvent.click(option);
    expect(onSelect).toHaveBeenCalledTimes(1);
    await act(async () => selection.resolve(false));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    fireEvent.click(option);
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledExactlyOnceWith(false));
  });

  it("drops a closed session's in-flight search and reopens with the default candidates", async () => {
    const stale = deferred<{ issues: Issue[]; total: number }>();
    mocks.listIssues.mockImplementation(async (params: { q: string }) =>
      params.q ? stale.promise : { issues: [issue()], total: 1 });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    const props = { ...pickerProps, browseOptions, onSelect: vi.fn(), onOpenChange: vi.fn() };
    const renderPicker = (open: boolean) => <QueryClientProvider client={client}>
      <IssuePickerModal {...props} open={open} />
    </QueryClientProvider>;
    const { rerender } = renderWithI18n(renderPicker(true));
    await screen.findByRole("option", { name: /Link this issue/ });
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "stale" } });
    await waitFor(() => expect(mocks.listIssues).toHaveBeenCalledWith(expect.objectContaining({ q: "stale" })));

    rerender(renderPicker(false));
    rerender(renderPicker(true));
    await act(async () => stale.resolve({ issues: [issue({ id: "stale", title: "Stale result" })], total: 1 }));

    await screen.findByRole("option", { name: /Link this issue/ });
    expect(screen.getByRole("combobox")).toHaveValue("");
    expect(screen.queryByText("Stale result")).not.toBeInTheDocument();
    client.clear();
  });
});
