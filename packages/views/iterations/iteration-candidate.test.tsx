import { useState } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import { I18nProvider } from "@multica/core/i18n/react";
import { PillButton } from "../common/pill-button";
import projects from "../locales/en/projects.json";
import issues from "../locales/en/issues.json";
import { IterationCandidate } from "./iteration-assignment";
import { source, targetA, targetB, ws } from "./test-fixtures";

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    getBaseUrl: () => "test",
    getSessionScope: () => "session",
    getIterationCapabilities: vi.fn(),
    listIterations: vi.fn(),
  },
}));
vi.mock("../modals/issue-picker-modal", () => ({ IssuePickerModal: () => null }));

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.getIterationCapabilities).mockResolvedValue({
    workspace_id: ws,
    supported: true,
    enabled: true,
    manual: true,
    schema_version: 1,
    atomic_handoff: true,
  });
  vi.mocked(api.listIterations).mockResolvedValue({
    workspace_id: ws,
    items: [source, targetA, { ...targetB, name: "测试迭代", revision: 7 }],
    next_cursor: null,
  });
});

function Harness({
  initialValue = null,
  onChange,
  pill = true,
}: {
  initialValue?: string | null;
  onChange: (id: string | null, revision?: number) => void;
  pill?: boolean;
}) {
  const [value, setValue] = useState(initialValue);
  return (
    <IterationCandidate
      wsId={ws}
      value={value}
      onChange={(id, revision) => {
        setValue(id);
        onChange(id, revision);
      }}
      triggerRender={pill ? <PillButton /> : undefined}
    />
  );
}

function mount(props: Partial<React.ComponentProps<typeof Harness>> = {}) {
  const onChange = props.onChange ?? vi.fn();
  const client = new QueryClient();
  render(
    <QueryClientProvider client={client}>
      <I18nProvider locale="en" resources={{ en: { projects, issues } }}>
        <Harness {...props} onChange={onChange} />
      </I18nProvider>
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), onChange };
}

describe("IterationCandidate create pill", () => {
  it("selects active or planned iterations with their revision and clears from the menu", async () => {
    vi.mocked(api.listIterations).mockResolvedValue({
      workspace_id: ws,
      items: [
        source,
        targetA,
        { ...targetB, id: "completed", name: "Completed delivery", status: "completed" },
        { ...targetB, id: "cancelled", name: "Cancelled delivery", status: "cancelled" },
      ],
      next_cursor: null,
    });
    const { user, onChange } = mount();
    const trigger = await screen.findByRole("button");
    expect(trigger).toHaveAccessibleName("Iterations: No iteration");
    await user.click(trigger);
    expect(await screen.findByRole("button", { name: targetA.name })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Completed delivery" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Cancelled delivery" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: source.name }));
    expect(onChange).toHaveBeenLastCalledWith(source.id, source.revision);
    await waitFor(() => expect(screen.queryByPlaceholderText("Search iterations")).not.toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: `Iterations: ${source.name}` }));
    await user.click(await screen.findByRole("button", { name: "No iteration" }));
    expect(onChange).toHaveBeenLastCalledWith(null, undefined);
    expect(screen.getByRole("button", { name: "Iterations: No iteration" })).toBeInTheDocument();
  });

  it("searches Chinese names with pinyin, commits the real match on Enter, and resets on reopen", async () => {
    const { user, onChange } = mount();
    await user.click(await screen.findByRole("button", { name: "Iterations: No iteration" }));
    await screen.findByRole("button", { name: "测试迭代" });
    await user.type(screen.getByPlaceholderText("Search iterations"), "ceshi");
    expect(screen.getByRole("button", { name: "No iteration" })).toBeInTheDocument();
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledExactlyOnceWith(targetB.id, 7);
    await waitFor(() => expect(screen.queryByPlaceholderText("Search iterations")).not.toBeInTheDocument());

    await user.click(screen.getByRole("button", { name: "Iterations: 测试迭代" }));
    expect(await screen.findByPlaceholderText("Search iterations")).toHaveValue("");
    expect(screen.getByRole("button", { name: targetA.name })).toBeInTheDocument();
  });

  it("does not clear on unmatched Enter but lets arrow navigation reach the empty choice", async () => {
    const { user, onChange } = mount({ initialValue: targetA.id });
    await user.click(await screen.findByRole("button", { name: `Iterations: ${targetA.name}` }));
    await user.type(await screen.findByPlaceholderText("Search iterations"), "no-match");
    expect(screen.getByText(issues.pickers.no_results)).toBeInTheDocument();
    await user.keyboard("{Enter}");
    expect(onChange).not.toHaveBeenCalled();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onChange).toHaveBeenCalledExactlyOnceWith(null, undefined);
  });

  it("keeps the picker and choices query gated by iteration capability", async () => {
    vi.mocked(api.getIterationCapabilities).mockResolvedValue({
      workspace_id: ws,
      supported: true,
      enabled: false,
      manual: true,
      schema_version: 1,
      atomic_handoff: true,
    });
    mount();
    await waitFor(() => expect(api.getIterationCapabilities).toHaveBeenCalledOnce());
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(api.listIterations).not.toHaveBeenCalled();
  });

  it("shows loading and recovers a failed choices request in the popover", async () => {
    let fail!: (error: Error) => void;
    vi.mocked(api.listIterations).mockImplementationOnce(
      () => new Promise((_resolve, reject) => { fail = reject; }),
    );
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Iterations: No iteration" }));
    expect(await screen.findByRole("status")).toHaveTextContent(projects.iterations.loading);
    await act(async () => fail(new Error("Choices unavailable")));
    expect(await screen.findByRole("alert")).toHaveTextContent(projects.iterations.error);
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: targetA.name })).toBeInTheDocument();
    expect(api.listIterations).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("keeps labelled selection and revision callbacks for callers without a pill trigger", async () => {
    const { user, onChange } = mount({ pill: false });
    const select = await screen.findByRole("combobox", { name: "Iterations" });
    await user.click(select);
    await user.click(await screen.findByRole("option", { name: targetA.name }));
    expect(onChange).toHaveBeenLastCalledWith(targetA.id, targetA.revision);
    await user.click(select);
    await user.click(await screen.findByRole("option", { name: "Remove from iteration" }));
    expect(onChange).toHaveBeenLastCalledWith(null, undefined);
  });
});
