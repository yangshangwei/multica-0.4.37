import { useLayoutEffect, useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enIssues from "../../../locales/en/issues.json";
import { PickerItem, PropertyPicker } from "./property-picker";

function EarlyEnter({ enabled, input }: { enabled: boolean; input: React.RefObject<HTMLInputElement | null> }) {
  useLayoutEffect(() => {
    if (enabled && input.current) fireEvent.keyDown(input.current, { key: "Enter" });
  }, [enabled, input]);
  return null;
}

function Harness({ reset = true, earlyEnter = false, onPick = vi.fn(), triggerAriaLabel }: { reset?: boolean; earlyEnter?: boolean; onPick?: (name: string) => void; triggerAriaLabel?: string }) {
  const [open, setOpen] = useState(true);
  const [query, setQuery] = useState("");
  const [mode, setMode] = useState("initial");
  const input = useRef<HTMLInputElement>(null);
  const names = mode === "swap" ? ["Grace"] : mode === "more" ? ["Ada", "Grace", "Linus"] : ["Ada", "Grace"];
  return <I18nProvider locale="en" resources={{ en: { issues: enIssues } }}>
    <PropertyPicker open={open} onOpenChange={setOpen} trigger="Open" triggerAriaLabel={triggerAriaLabel} searchable searchPlaceholder="Search"
      onSearchChange={setQuery} searchInputRef={input}
      navigationResetKey={reset ? `${mode === "more" ? "initial" : mode}:${query}` : undefined}
      header={<><button onClick={() => setMode("swap")}>Swap</button><button onClick={() => setMode("more")}>More</button><button onClick={() => input.current?.focus()}>Focus search</button></>}>
      {names.filter((name) => name.toLowerCase().includes(query.toLowerCase())).map((name) => <PickerItem key={name} selected={false} onClick={() => { onPick(name); setOpen(false); }}>{name}</PickerItem>)}
      <EarlyEnter enabled={earlyEnter && mode === "swap"} input={input} />
    </PropertyPicker>
  </I18nProvider>;
}

describe("PropertyPicker optional external navigation reset", () => {
  it.each([undefined, "Project without decoration"])("keeps custom render children and their selected description (%s)", (description) => {
    render(<I18nProvider locale="en" resources={{ en: { issues: enIssues } }}>
      <PropertyPicker open={false} onOpenChange={() => {}} trigger="Generated fallback"
        triggerRender={<button><span>Selected project</span></button>}
        triggerAriaLabel="Candidate project" triggerAriaDescription={description}>
        <span>Options</span>
      </PropertyPicker>
    </I18nProvider>);
    const trigger = screen.getByRole("button", { name: "Candidate project" });
    expect(trigger).toHaveTextContent("Selected project");
    expect(trigger).toHaveAccessibleDescription(description ?? "Selected project");
    expect(screen.queryByText("Generated fallback")).not.toBeInTheDocument();
  });

  it("exposes the field name on the trigger when provided", () => {
    render(<Harness triggerAriaLabel="Candidate project" />);
    expect(screen.getByRole("button", { name: "Candidate project" })).toHaveAccessibleDescription("Open");
  });

  it("blocks stale index and unique-result Enter after an external change until a new arrow", async () => {
    const user = userEvent.setup(); const onPick = vi.fn();
    render(<Harness onPick={onPick} />);
    const input = await screen.findByPlaceholderText("Search");
    await user.click(input); await user.keyboard("{ArrowDown}");
    await user.click(screen.getByText("Swap")); await user.click(input); await user.keyboard("{Enter}");
    expect(onPick).not.toHaveBeenCalled();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onPick).toHaveBeenCalledExactlyOnceWith("Grace");
  });

  it("gives typed first-match selection priority when the reset key changes in the same render", async () => {
    const user = userEvent.setup(); const onPick = vi.fn();
    render(<Harness onPick={onPick} />);
    await user.type(await screen.findByPlaceholderText("Search"), "gr");
    await user.keyboard("{Enter}");
    expect(onPick).toHaveBeenCalledExactlyOnceWith("Grace");
  });

  it("guards an Enter dispatched before the shell's layout effect sees an external change", async () => {
    const user = userEvent.setup(); const onPick = vi.fn();
    render(<Harness earlyEnter onPick={onPick} />);
    await user.click(await screen.findByPlaceholderText("Search"));
    await user.keyboard("{ArrowDown}");
    await user.click(screen.getByText("Swap"));
    expect(onPick).not.toHaveBeenCalled();
  });

  it("retains the existing highlight when a display page appends candidates", async () => {
    const user = userEvent.setup(); const onPick = vi.fn();
    render(<Harness onPick={onPick} />);
    const input = await screen.findByPlaceholderText("Search");
    await user.click(input); await user.keyboard("{ArrowDown}{ArrowDown}");
    await user.click(screen.getByText("More")); await user.click(input); await user.keyboard("{Enter}");
    expect(onPick).toHaveBeenCalledExactlyOnceWith("Grace");
  });

  it("preserves the old unique-result fallback when no reset key is supplied", async () => {
    const user = userEvent.setup(); const onPick = vi.fn();
    render(<Harness reset={false} onPick={onPick} />);
    const input = await screen.findByPlaceholderText("Search");
    await user.click(screen.getByText("Swap")); await user.click(input); await user.keyboard("{Enter}");
    expect(onPick).toHaveBeenCalledExactlyOnceWith("Grace");
  });

  it("ignores composing Enter/arrows including Safari 229 and exposes the real input ref", async () => {
    const user = userEvent.setup(); const onPick = vi.fn();
    render(<Harness onPick={onPick} />);
    const input = await screen.findByPlaceholderText("Search");
    await user.type(input, "gr");
    fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    fireEvent.keyDown(input, { key: "ArrowDown", isComposing: true });
    fireEvent.keyDown(input, { key: "Enter", keyCode: 229 });
    expect(onPick).not.toHaveBeenCalled();
    await user.click(screen.getByText("Focus search")); expect(input).toHaveFocus();
    await user.keyboard("{Enter}"); expect(onPick).toHaveBeenCalledExactlyOnceWith("Grace");
    await user.click(screen.getByText("Open"));
    expect(await screen.findByPlaceholderText("Search")).toHaveValue("");
  });
});
