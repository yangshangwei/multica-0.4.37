import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import type { WorkspaceNameSelection } from "@multica/core/workspace/workspace-names";
import enWorkspace from "../locales/en/workspace.json";
import zhWorkspace from "../locales/zh-Hans/workspace.json";
import { WorkspaceNamePicker } from "./workspace-name-picker";

const resources = {
  en: { workspace: enWorkspace },
  "zh-Hans": { workspace: zhWorkspace },
};

function Wrapper({ children }: { children: ReactNode }) {
  return <I18nProvider locale="en" resources={resources}>{children}</I18nProvider>;
}

describe("WorkspaceNamePicker", () => {
  it("shows all six series with examples and applies selection without randomizing", async () => {
    const user = userEvent.setup();
    const onRandom = vi.fn();
    const onSelectionChange = vi.fn();
    function Picker() {
      const [selection, setSelection] = useState<WorkspaceNameSelection>("workshop");
      return <WorkspaceNamePicker selection={selection} onRandom={onRandom} onSelectionChange={(next) => {
        setSelection(next);
        onSelectionChange(next);
      }} />;
    }
    render(<Picker />, { wrapper: Wrapper });

    expect(screen.getByText("Current: Dev workshop")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Choose naming series" }));
    expect(await screen.findAllByRole("menuitemradio")).toHaveLength(7);
    expect(screen.getByRole("menuitemradio", { name: /Dev workshop/ })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByText(/Code Workshop/)).toBeInTheDocument();

    await user.click(screen.getByRole("menuitemradio", { name: /Algorithms & computing/ }));
    expect(onSelectionChange).toHaveBeenCalledWith("computing");
    expect(onRandom).not.toHaveBeenCalled();
    expect(screen.getByText("Current: Algorithms & computing")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: "Random" }));
    expect(onRandom).toHaveBeenCalledOnce();
  });

  it("supports keyboard selection and Escape returns focus to the menu trigger", async () => {
    const user = userEvent.setup();
    const onSelectionChange = vi.fn();
    render(<WorkspaceNamePicker selection="workshop" onRandom={vi.fn()} onSelectionChange={onSelectionChange} />, { wrapper: Wrapper });
    const trigger = screen.getByRole("button", { name: "Choose naming series" });
    trigger.focus();
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("menuitemradio", { name: /Dev workshop/ })).toHaveFocus();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onSelectionChange).toHaveBeenCalledWith("computing");
    await waitFor(() => expect(trigger).toHaveFocus());

    await user.keyboard("{ArrowDown}{Escape}");
    await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });

  it("disables random and series selection together", async () => {
    const user = userEvent.setup();
    const onRandom = vi.fn();
    render(<WorkspaceNamePicker selection="workshop" onRandom={onRandom} onSelectionChange={vi.fn()} disabled />, { wrapper: Wrapper });
    const random = screen.getByRole("button", { name: "Random" });
    const series = screen.getByRole("button", { name: "Choose naming series" });
    expect(random).toBeDisabled();
    expect(series).toBeDisabled();
    await user.click(random);
    await user.click(series);
    expect(onRandom).not.toHaveBeenCalled();
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("localizes the current series, menu and catalog examples", async () => {
    const user = userEvent.setup();
    render(<I18nProvider locale="zh-Hans" resources={resources}>
      <WorkspaceNamePicker selection="workshop" onRandom={vi.fn()} onSelectionChange={vi.fn()} />
    </I18nProvider>);
    expect(screen.getByText("当前：研发工坊")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "选择命名系列" }));
    expect(await screen.findByRole("menuitemradio", { name: /算法与计算/ })).toBeInTheDocument();
    expect(screen.getByText(/代码工坊/)).toBeInTheDocument();
  });
});
