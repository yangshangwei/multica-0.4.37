import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ComponentProps } from "react";
import { ApiError } from "@multica/core/api";
import { I18nProvider } from "@multica/core/i18n/react";
import enModals from "../locales/en/modals.json";
import { IssueDescriptionAssist } from "./issue-description-assist";

const optimize = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/issues/mutations", () => ({
  useOptimizeIssueDescription: () => ({ mutateAsync: optimize }),
}));
vi.mock("../editor/readonly-content", () => ({
  ReadonlyContent: ({ content }: { content: string }) => <div data-testid="ai-readonly">{content}</div>,
}));

function deferred() {
  let resolve!: (value: { text: string; questions: string[] }) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<{ text: string; questions: string[] }>((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}

function setup(overrides: Partial<ComponentProps<typeof IssueDescriptionAssist>> = {}) {
  let markdown = "Original request";
  const onChange = vi.fn();
  const editorRef = { current: {
    getMarkdown: () => markdown,
    focus: vi.fn(),
    flushPendingUpdate: vi.fn((): string | null => markdown),
    adoptContent: vi.fn((value: string) => { markdown = value.replace(/\n+$/, ""); }),
  } };
  const props = {
    wsId: "workspace-1", mode: "manual" as const, value: markdown,
    editorRef, onChange, uploading: false, isBlocked: () => false, submitting: false,
    ...overrides,
  };
  const element = (next = props) => (
    <I18nProvider locale="en" resources={{ en: { modals: enModals } }}>
      <IssueDescriptionAssist {...next} />
    </I18nProvider>
  );
  const view = render(element());
  return { ...view, onChange, editorRef, props, element, read: () => markdown, type: (text: string) => { markdown = text; } };
}

beforeEach(() => {
  optimize.mockReset();
  optimize.mockResolvedValue({ text: "Improved request\n", questions: ["Which version?"] });
});

describe("IssueDescriptionAssist", () => {
  it("preserves answers while collapsed and restores them after merging and undo", async () => {
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Which version?" }), "Version 2");
    const toggle = screen.getByRole("button", { name: "Key details · 1" });
    await userEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("textbox", { name: "Which version?" })).not.toBeInTheDocument();
    expect(toggle).toHaveTextContent("1 to add");
    await userEvent.click(toggle);
    expect(screen.getByRole("textbox", { name: "Which version?" })).toHaveValue("Version 2");
    await userEvent.click(screen.getByRole("button", { name: "Add answers to draft" }));
    expect(screen.getByRole("status")).toHaveTextContent("Added 1 detail");
    await userEvent.click(screen.getByRole("button", { name: "Undo" }));
    expect(screen.getByRole("textbox", { name: "Which version?" })).toHaveValue("Version 2");
    expect(view.read()).toBe("Improved request");
  });

  it("requests space once and reveals the editor only while focus remains in AI controls", async () => {
    const response = deferred();
    optimize.mockReturnValueOnce(response.promise);
    const onNeedsSpace = vi.fn();
    const onRevealEditor = vi.fn();
    const view = setup({ onNeedsSpace, onRevealEditor });
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    const { onText } = optimize.mock.calls[0]![0] as { onText: (text: string) => void };
    act(() => { onText("A"); onText("AB"); });
    expect(onNeedsSpace).toHaveBeenCalledTimes(1);
    await act(async () => { response.resolve({ text: "Improved request", questions: [] }); });
    expect(view.editorRef.current.focus).toHaveBeenCalledTimes(1);
    expect(onRevealEditor).toHaveBeenCalledWith("start");
  });

  it("does not steal focus or scroll when the user moves outside AI controls", async () => {
    const response = deferred();
    optimize.mockReturnValueOnce(response.promise);
    const onRevealEditor = vi.fn();
    const view = setup({ onRevealEditor });
    const otherField = document.createElement("input");
    document.body.append(otherField);
    try {
      await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
      otherField.focus();
      await act(async () => { response.resolve({ text: "Improved request", questions: [] }); });
      expect(otherField).toHaveFocus();
      expect(view.read()).toBe("Improved request");
      expect(view.editorRef.current.focus).not.toHaveBeenCalled();
      expect(onRevealEditor).not.toHaveBeenCalled();
    } finally { otherField.remove(); }
  });

  it("stops reporting AI completion after the adopted draft is edited", async () => {
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    view.type("My revised content");
    view.rerender(view.element({ ...view.props, value: "My revised content" }));
    expect(screen.getByRole("status")).toBeEmptyDOMElement();
  });

  it("automatically fills the validated draft and merges at most two answers into the latest text", async () => {
    optimize.mockResolvedValue({ text: "Improved request", questions: ["Which version?", "Which platform?", "Extra question?"] });
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    expect(view.read()).toBe("Improved request");
    expect(screen.queryByText("Extra question?")).not.toBeInTheDocument();
    await userEvent.type(screen.getByRole("textbox", { name: "Which version?" }), "Version 2");
    await userEvent.click(screen.getAllByRole("button", { name: "Let the assignee decide" })[1]!);
    view.type("Improved request with my latest edit");
    await userEvent.click(screen.getByRole("button", { name: "Add answers to draft" }));
    expect(view.read()).toContain("Improved request with my latest edit");
    expect(view.read()).toContain("Version 2");
    expect(view.read()).toContain("Let the assignee decide");
    expect(optimize).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "Add answers to draft" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Undo" }));
    expect(view.read()).toBe("Improved request with my latest edit");
    expect(screen.getByRole("textbox", { name: "Which version?" })).toHaveValue("Version 2");
    await userEvent.click(screen.getByRole("button", { name: "Add answers to draft" }));
    expect(view.read().match(/Version 2/g)).toHaveLength(1);
  });

  it("flushes the live editor, previews questions separately, and undoes normalized adoption", async () => {
    const view = setup({ title: "Task title" });
    view.type("Newest request with [file](https://example.test/file)");
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    expect(optimize).toHaveBeenCalledWith(expect.objectContaining({ text: "Newest request with [file](https://example.test/file)", title: "Task title", mode: "manual", signal: expect.any(AbortSignal) }));
    expect(view.onChange).toHaveBeenLastCalledWith(view.read());
    expect(await screen.findByText("Which version?")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Apply and replace" })).not.toBeInTheDocument();
    expect(view.read()).toBe("Improved request");
    expect(view.onChange).toHaveBeenLastCalledWith("Improved request");
    await userEvent.click(screen.getByRole("button", { name: "Undo" }));
    expect(view.read()).toBe("Newest request with [file](https://example.test/file)");
    expect(screen.queryByRole("button", { name: "Undo" })).not.toBeInTheDocument();
  });

  it("shows accumulated streaming text inertly and only adopts the validated final result", async () => {
    const response = deferred();
    optimize.mockReturnValueOnce(response.promise);
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    const { onText } = optimize.mock.calls[0]![0] as { onText?: (text: string) => void };
    const partial = "## Draft\n![image](https://example.test/tracker.png)";
    act(() => { onText?.("## Draft"); });
    act(() => { onText?.(partial); });
    expect(screen.getByText(partial, { exact: true, normalizer: (value) => value })).toBeInTheDocument();
    expect(screen.queryByTestId("ai-readonly")).not.toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Apply and replace" })).not.toBeInTheDocument();
    expect(view.read()).toBe("Original request");
    await act(async () => { response.resolve({ text: "Validated final result", questions: ["Confirm owner?"] }); });
    expect(view.read()).toBe("Validated final result");
    expect(screen.queryByText(partial)).not.toBeInTheDocument();
    act(() => { onText?.("Late post-completion text"); });
    expect(screen.queryByText("Late post-completion text")).not.toBeInTheDocument();
    expect(view.read()).toBe("Validated final result");
  });

  it("clears provisional text on stream errors and ignores later callbacks", async () => {
    const response = deferred();
    optimize.mockReturnValueOnce(response.promise);
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    const { onText } = optimize.mock.calls[0]![0] as { onText?: (text: string) => void };
    act(() => { onText?.("Provisional text"); });
    expect(screen.getByText("Provisional text")).toBeInTheDocument();
    await act(async () => { response.reject(new Error("Incomplete stream")); });
    expect(screen.getByRole("alert")).toHaveTextContent("Optimization failed");
    expect(screen.queryByText("Provisional text")).not.toBeInTheDocument();
    act(() => { onText?.("Late failed stream text"); });
    expect(screen.queryByText("Late failed stream text")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Apply and replace" })).not.toBeInTheDocument();
    expect(view.read()).toBe("Original request");
  });

  it("drains queued editor updates before auto-adoption and undo", async () => {
    const response = deferred();
    optimize.mockReturnValueOnce(response.promise);
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    let queued: string | null = "Original request";
    view.editorRef.current.flushPendingUpdate.mockImplementation(() => {
      const result = queued;
      queued = null;
      return result;
    });
    await act(async () => { response.resolve({ text: "Improved request", questions: [] }); });
    if (queued !== null) view.onChange(queued);
    expect(view.onChange).toHaveBeenLastCalledWith("Improved request");
    queued = "Improved request";
    await userEvent.click(screen.getByRole("button", { name: "Undo" }));
    if (queued !== null) view.onChange(queued);
    expect(view.onChange).toHaveBeenLastCalledWith("Original request");
  });

  it("checks live text at completion even if the debounced value has not changed", async () => {
    const response = deferred();
    optimize.mockReturnValueOnce(response.promise);
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    view.type("Newer unsaved edit");
    await act(async () => { response.resolve({ text: "Improved request", questions: [] }); });
    expect(view.read()).toBe("Newer unsaved edit");
    expect(view.editorRef.current.adoptContent).not.toHaveBeenCalled();
    expect(screen.getByRole("status")).toHaveTextContent("Content changed. Optimize again to use your latest edits.");
    expect(screen.getByTestId("ai-readonly")).toHaveTextContent("Improved request");
  });

  it("protects newer edits from undo", async () => {
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    await screen.findByRole("button", { name: "Undo" });
    view.type("Edit after adoption");
    await userEvent.click(screen.getByRole("button", { name: "Undo" }));
    expect(view.read()).toBe("Edit after adoption");
    expect(screen.queryByRole("button", { name: "Undo" })).not.toBeInTheDocument();
  });

  it("aborts cancelled requests and ignores their late results after a retry", async () => {
    const first = deferred();
    const second = deferred();
    optimize.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    const signal = optimize.mock.calls[0]![0].signal as AbortSignal;
    const { onText } = optimize.mock.calls[0]![0] as { onText?: (text: string) => void };
    act(() => { onText?.("Provisional cancelled text"); });
    expect(screen.getByText("Provisional cancelled text")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(signal.aborted).toBe(true);
    expect(screen.queryByText("Provisional cancelled text")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Help me refine" })).toHaveFocus();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    const current = optimize.mock.calls[1]![0] as { onText?: (text: string) => void };
    act(() => { current.onText?.("Current stream text"); });
    act(() => { onText?.("Late cancelled stream text"); });
    expect(screen.queryByText("Late cancelled stream text")).not.toBeInTheDocument();
    expect(screen.getByText("Current stream text")).toBeInTheDocument();
    await act(async () => { second.resolve({ text: "Improved request", questions: [] }); });
    expect(view.read()).toBe("Improved request");
    await act(async () => { first.resolve({ text: "Late cancelled output", questions: [] }); });
    expect(screen.queryByText("Late cancelled output")).not.toBeInTheDocument();
  });

  it("aborts when its creation panel unmounts", async () => {
    optimize.mockReturnValue(deferred().promise);
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    const signal = optimize.mock.calls[0]![0].signal as AbortSignal;
    const { onText } = optimize.mock.calls[0]![0] as { onText?: (text: string) => void };
    view.unmount();
    expect(signal.aborted).toBe(true);
    act(() => { onText?.("Late unmounted stream text"); });
    expect(view.container).toBeEmptyDOMElement();
    expect(view.editorRef.current.adoptContent).not.toHaveBeenCalled();
  });

  it("keeps failures inline and can regenerate from the latest text", async () => {
    optimize.mockRejectedValueOnce(new Error("Private provider diagnostics"));
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Optimization failed. Try again or create with your current content.");
    expect(screen.queryByText("Private provider diagnostics")).not.toBeInTheDocument();
    view.type("Latest edit");
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(view.read()).toBe("Improved request");
    expect(optimize).toHaveBeenLastCalledWith(expect.objectContaining({ text: "Latest edit" }));
    await userEvent.click(screen.getByRole("button", { name: "Undo" }));
    expect(view.read()).toBe("Latest edit");
  });

  it("explains unavailable AI without exposing server diagnostics", async () => {
    optimize.mockRejectedValueOnce(new ApiError("Private provider details", 503, "Unavailable"));
    const view = setup();
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("AI optimization is unavailable. You can still create with your current content.");
    expect(view.read()).toBe("Original request");
    expect(screen.queryByText("Private provider details")).not.toBeInTheDocument();
  });

  it("disables empty input and rechecks uploads at invocation time", async () => {
    const view = setup({ value: "", isBlocked: () => true });
    expect(screen.getByRole("button", { name: "Help me refine" })).toBeDisabled();
    view.rerender(view.element({ ...view.props, value: "Original request" }));
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    expect(optimize).not.toHaveBeenCalled();
  });

  it.each(["upload", "submission"])("keeps the draft when %s starts during generation", async (gate) => {
    const response = deferred();
    optimize.mockReturnValueOnce(response.promise);
    let blocked = false;
    const view = setup({ isBlocked: () => blocked });
    await userEvent.click(screen.getByRole("button", { name: "Help me refine" }));
    if (gate === "upload") blocked = true;
    else view.rerender(view.element({ ...view.props, submitting: true }));
    await act(async () => { response.resolve({ text: "Improved request", questions: [] }); });
    expect(view.editorRef.current.adoptContent).not.toHaveBeenCalled();
    expect(view.read()).toBe("Original request");
    expect(screen.getByTestId("ai-readonly")).toHaveTextContent("Improved request");
  });
});
