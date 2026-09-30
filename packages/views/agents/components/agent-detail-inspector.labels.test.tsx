import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import type { Agent } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { AgentDetailInspector } from "./agent-detail-inspector";

vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-query")>()),
  useQuery: () => ({ data: undefined, isSuccess: false }),
}));

// The picker is the only way agent labels ever reached the agent settings
// form. Mocking it means the assertion below fails loudly if the row is
// re-added, instead of passing because a real picker rendered nothing.
vi.mock("../../labels/resource-label-picker", () => ({
  ResourceLabelPicker: () => <div data-testid="resource-label-picker" />,
}));

vi.mock("../../common/avatar-upload-control", () => ({
  AvatarUploadControl: () => <div data-testid="avatar-upload" />,
}));

vi.mock("./inspector/model-picker", () => ({
  ModelPicker: () => <div data-testid="model-picker" />,
}));

vi.mock("./inspector/runtime-picker", () => ({
  RuntimePicker: () => <div data-testid="runtime-picker" />,
}));

vi.mock("./inspector/thinking-prop-row", () => ({
  ThinkingSettingField: () => <div data-testid="thinking-field" />,
}));

vi.mock("./inspector/service-tier-setting-field", () => ({
  ServiceTierSettingField: () => <div data-testid="service-tier-field" />,
}));

const agent = {
  id: "agent-1",
  workspace_id: "workspace-1",
  name: "Lambda",
  description: "Test agent",
  runtime_id: "runtime-1",
} as Agent;

describe("AgentDetailInspector labels", () => {
  afterEach(() => {
    cleanup();
  });

  // Agent labels were removed from the product (MUL-5600). Label Settings no
  // longer manages an agent catalog, so an attach-only picker here would be a
  // dead end pointing at a catalog the user cannot populate.
  it("does not offer a label picker", () => {
    renderWithI18n(
      <AgentDetailInspector
        agent={agent}
        runtime={null}
        runtimes={[]}
        members={[]}
        currentUserId="user-1"
        canEdit
        onUpdate={vi.fn(async () => {})}
      />,
    );

    // Sanity check: the profile card actually rendered.
    expect(screen.getByLabelText("Name")).toBeInTheDocument();

    expect(screen.queryByTestId("resource-label-picker")).toBeNull();
    expect(screen.queryByText("Labels")).toBeNull();
  });
});
describe("AgentDetailInspector categories", () => {
  function renderInspector(canEdit = true, onUpdate = vi.fn(async () => {})) {
    const view = (nextAgent: Agent) => (
      <AgentDetailInspector agent={nextAgent} runtime={null}
        runtimes={[]} members={[]} currentUserId="user-1" canEdit={canEdit} onUpdate={onUpdate} />
    );
    const { rerender } = renderWithI18n(view({ ...agent, category: "研发" }));
    return { onUpdate, refresh: (updates: Partial<Agent>) => rerender(view({ ...agent, category: "研发", ...updates })) };
  }

  it("autosaves a category change and can clear it", async () => {
    const { onUpdate } = renderInspector();
    const input = screen.getByLabelText("Category");
    fireEvent.change(input, { target: { value: "  运营  " } });
    fireEvent.blur(input);
    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith("agent-1", { category: "运营" }));
    fireEvent.change(input, { target: { value: "" } });
    fireEvent.blur(input);
    await waitFor(() => expect(onUpdate).toHaveBeenLastCalledWith("agent-1", expect.objectContaining({ category: "" })));
  });

  it("disables category changes without edit permission", () => {
    const { onUpdate } = renderInspector(false);
    expect(screen.getByLabelText("Category")).toBeDisabled();
    expect(onUpdate).not.toHaveBeenCalled();
  });

  it("waits for IME composition to finish before saving", async () => {
    const { onUpdate } = renderInspector();
    const input = screen.getByLabelText("Category");
    fireEvent.compositionStart(input);
    fireEvent.change(input, { target: { value: "运营" } });
    fireEvent.blur(input);
    expect(onUpdate).not.toHaveBeenCalled();

    fireEvent.compositionEnd(input);
    fireEvent.blur(input);
    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith(
      "agent-1", expect.objectContaining({ category: "运营" }),
    ));
  });

  // The boundary matrix lives in core/agents/category.test.ts. Here we pin
  // accessible feedback and the autosave guard's connection to validation.
  it("keeps an invalid category unsaved until it is corrected", async () => {
    const { onUpdate } = renderInspector();
    const input = screen.getByLabelText("Category");
    fireEvent.change(input, { target: { value: "a".repeat(51) } });
    fireEvent.blur(input);
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription("Use up to 50 characters without control characters.");
    expect(onUpdate).not.toHaveBeenCalled();

    fireEvent.change(input, { target: { value: "运营" } });
    fireEvent.blur(input);
    await waitFor(() => expect(onUpdate).toHaveBeenCalledWith(
      "agent-1", expect.objectContaining({ category: "运营" }),
    ));
  });

  it("retains an unsaved category and shows a failure when saving fails", async () => {
    renderInspector(true, vi.fn().mockRejectedValue(new Error("Offline")));
    const input = screen.getByLabelText("Category");
    fireEvent.change(input, { target: { value: "运营" } });
    fireEvent.blur(input);
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("Couldn't save"));
    expect(input).toHaveValue("运营");
  });

  it("follows a remote category refresh without writing it back when renaming", async () => {
    const { onUpdate, refresh } = renderInspector();
    refresh({ category: "运营" });
    expect(screen.getByLabelText("Category")).toHaveValue("运营");
    fireEvent.blur(screen.getByLabelText("Category"));
    expect(onUpdate).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Renamed" } });
    fireEvent.blur(screen.getByLabelText("Name"));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledExactlyOnceWith(
      "agent-1", { name: "Renamed", description: "Test agent" },
    ));
  });

  it("patches only category after unrelated profile data changes remotely", async () => {
    const { onUpdate, refresh } = renderInspector();
    refresh({ name: "Remote name", description: "Remote description" });
    fireEvent.change(screen.getByLabelText("Category"), { target: { value: "运营" } });
    fireEvent.blur(screen.getByLabelText("Category"));
    await waitFor(() => expect(onUpdate).toHaveBeenCalledExactlyOnceWith(
      "agent-1", { category: "运营" },
    ));
  });

  it.each(["支持", ""])("keeps the newer category %j through an in-flight cache refresh", async (nextCategory) => {
    let finishSave!: () => void;
    const pendingSave = new Promise<void>((resolve) => { finishSave = resolve; });
    const onUpdate = vi.fn().mockImplementationOnce(() => pendingSave).mockResolvedValue(undefined);
    const { refresh } = renderInspector(true, onUpdate);
    const input = screen.getByLabelText("Category");
    fireEvent.change(input, { target: { value: "运营" } });
    fireEvent.blur(input);
    fireEvent.change(input, { target: { value: nextCategory } });
    refresh({ category: "运营" });
    expect(input).toHaveValue(nextCategory);
    fireEvent.blur(input);
    await act(async () => { finishSave(); await pendingSave; });
    await waitFor(() => expect(onUpdate).toHaveBeenNthCalledWith(2, "agent-1", { category: nextCategory }));
  });
});
