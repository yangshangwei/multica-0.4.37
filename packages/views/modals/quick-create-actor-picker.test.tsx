// Pure search/ranking/grouping boundaries live in quick-create-actor-picker-model.test.ts.
// These tests deliberately use the real PropertyPicker and Base UI popover.
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import { Dialog, DialogContent, DialogTitle } from "@multica/ui/components/ui/dialog";
import type { QuickCreateActorRef } from "@multica/core/issues/stores/quick-create-store";
import enModals from "../locales/en/modals.json";
import enIssues from "../locales/en/issues.json";
import { QuickCreateActorPicker, type QuickCreateActorPickerProps } from "./quick-create-actor-picker";

vi.mock("../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));
const agents = [
  { id: "a", name: "Ada", description: "**Find** bugs" },
  { id: "b", name: "Bob", description: "" },
  { id: "c", name: "Chris", description: "Review changes" },
  { id: "d", name: "Dana", description: "Write documentation" },
];
const squads = [{ id: "a", name: "Review squad", description: "Review releases" }];
const a = { type: "agent" as const, id: "a" };
const b = { type: "agent" as const, id: "b" };

function Harness(props: Partial<QuickCreateActorPickerProps>) {
  const [favorites, setFavorites] = useState<QuickCreateActorRef[]>(props.favoriteActors ?? []);
  return <I18nProvider locale="en" resources={{ en: { modals: enModals, issues: enIssues } }}>
    <QuickCreateActorPicker actor={a} visibleAgents={agents} visibleSquads={squads}
      preferencesReady recentActors={[]} onPick={vi.fn()}
      {...props} onToggleFavorite={(ref) => {
        props.onToggleFavorite?.(ref);
        setFavorites((current) => current.some((item) => item.type === ref.type && item.id === ref.id)
          ? current.filter((item) => item.type !== ref.type || item.id !== ref.id) : [...current, ref]);
      }} favoriteActors={favorites} />
  </I18nProvider>;
}

async function openPicker(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: /Created by/ }));
  return screen.findByPlaceholderText("Search names or responsibilities...");
}

describe("QuickCreateActorPicker", () => {
  it.each(["Browse all (5)", "View all favorites (1)", "Back to shortcuts", "Show more (5 remaining)", "Retry agents"])("keeps focus inside after %s so Escape dismisses one layer at a time", async (action) => {
    function NestedDialog() {
      const [open, setOpen] = useState(true);
      const [retried, setRetried] = useState(false);
      return <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent showCloseButton={false}>
          <DialogTitle>Create an issue</DialogTitle>
          <Harness favoriteActors={[a]}
            {...(action.startsWith("Show more") ? {
              visibleAgents: Array.from({length: 55}, (_, index) => ({id:String(index),name:`Agent ${index}`,description:""})), visibleSquads:[],
            } : {})}
            {...(action === "Retry agents" ? {
              agentState: {pending:false,error:!retried,hasData:retried,onRetry:()=>setRetried(true)},
            } : {})} />
        </DialogContent>
      </Dialog>;
    }
    const user = userEvent.setup();
    render(<NestedDialog />);
    const input = await openPicker(user);
    if (action === "Back to shortcuts") {
      await user.click(screen.getByRole("button", {name: "Browse all (5)"}));
    }
    const browse = screen.getByRole("button", {name: action});
    await user.click(browse);
    // Chromium blurs a focused button when it becomes disabled. jsdom does
    // not implement that browser behavior, so reproduce it before Escape.
    if (browse instanceof HTMLButtonElement && browse.disabled) browse.blur();
    await user.keyboard("{Escape}");
    expect(input).not.toBeInTheDocument();
    expect(screen.getByRole("dialog", {name: "Create an issue"})).toBeInTheDocument();
    expect(screen.getByRole("button", {name: /Created by/})).toHaveFocus();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", {name: "Create an issue"})).not.toBeInTheDocument();
  });

  it("shows truthful two-line descriptions and separate accessible pin controls without selecting", async () => {
    const user = userEvent.setup(); const onPick = vi.fn(); const onToggleFavorite = vi.fn();
    render(<Harness onPick={onPick} onToggleFavorite={onToggleFavorite} />);
    await openPicker(user);
    expect(screen.getByText("Find bugs")).toBeInTheDocument();
    expect(screen.getByText("No responsibility description")).toBeInTheDocument();
    expect(document.querySelector("button button")).toBeNull();
    const pin = screen.getByRole("button", { name: "Pin Ada to favorites" });
    pin.focus(); await user.keyboard(" ");
    expect(onToggleFavorite).toHaveBeenCalledExactlyOnceWith(a);
    expect(onPick).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Unpin Ada from favorites" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Unpin Ada from favorites" })).toHaveFocus();
  });

  it("keeps moved pin focus and falls back to the next row then search when favorites disappear", async () => {
    const user = userEvent.setup();
    render(<Harness favoriteActors={[a, b]} recentActors={[{type:"agent",id:"c"}]} />);
    const input = await openPicker(user);
    await user.click(screen.getByRole("button", { name: "Unpin Ada from favorites" }));
    const bobRow = document.querySelector('[data-actor-key="agent:b"]')!;
    expect(bobRow.querySelector("button[data-picker-item]")).toHaveFocus();
    // Pin Chris while it moves from Recent into Favorites.
    await user.click(screen.getByRole("button", { name: "Pin Chris to favorites" }));
    expect(screen.getByRole("button", { name: "Unpin Chris from favorites" })).toHaveFocus();
    await user.click(screen.getByRole("button", { name: /View all favorites/ }));
    await user.click(screen.getByRole("button", { name: "Unpin Bob from favorites" }));
    await user.click(screen.getByRole("button", { name: "Unpin Chris from favorites" }));
    expect(input).toHaveFocus();
    expect(screen.getByText("No favorites yet")).toBeInTheDocument();
  });

  it("searches beyond shortcuts, preserves query across types and resets on Escape", async () => {
    const user = userEvent.setup(); const onPick = vi.fn();
    render(<Harness favoriteActors={[a]} onPick={onPick} />);
    const input = await openPicker(user);
    await user.type(input, "Review");
    await user.click(screen.getByRole("button", { name: "AI squads" }));
    expect(input).toHaveValue("Review");
    await user.click(input); await user.keyboard("{Enter}");
    expect(onPick).not.toHaveBeenCalled();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onPick).toHaveBeenCalledExactlyOnceWith({type:"squad",id:"a"});
    const reopened = await openPicker(user);
    expect(reopened).toHaveValue("");
    expect(screen.getByRole("button", { name: "All" })).toHaveAttribute("aria-pressed", "true");
    await user.keyboard("{Escape}");
    expect(screen.getByRole("button", { name: /Created by/ })).toHaveFocus();
  });

  it("types to select the first result, ignores IME, and arrows skip all pin controls", async () => {
    const user = userEvent.setup(); const onPick = vi.fn();
    render(<Harness onPick={onPick} />);
    const input = await openPicker(user);
    await user.type(input, "Review");
    fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    expect(onPick).not.toHaveBeenCalled();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onPick).toHaveBeenCalledExactlyOnceWith({type:"agent",id:"c"});
  });

  it("preserves usable agents during squad failure and retries without a false empty result", async () => {
    const user = userEvent.setup(); const retry = vi.fn();
    render(<Harness squadState={{pending:false,error:true,hasData:false,onRetry:retry}} />);
    const input = await openPicker(user);
    expect(screen.getByText("Could not load AI squads")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Ada Agent Find bugs Selected/ })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry AI squads" }));
    expect(retry).toHaveBeenCalledOnce();
    await user.type(input, "No match");
    expect(screen.queryByText("No matching actors")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Agents" }));
    expect(screen.getByText("No matching actors")).toBeInTheDocument();
  });

  it("disables pins until preferences are ready and reports initial loading", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Harness preferencesReady={false} />);
    await openPicker(user);
    expect(screen.getByRole("button", { name: "Pin Ada to favorites" })).toBeDisabled();
    rerender(<Harness preferencesReady={false} agentState={{pending:true,error:false,hasData:false,onRetry:vi.fn()}} squadState={{pending:true,error:false,hasData:false,onRetry:vi.fn()}} />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading actors...");
    expect(screen.queryByText("No matching actors")).not.toBeInTheDocument();
  });

  it("appends a display page, retains it through pinning and resets it when the search changes", async () => {
    const user = userEvent.setup();
    const directory = Array.from({length: 55}, (_, i) => ({id:String(i),name:`Agent ${String(i).padStart(2,"0")}`,description:"Saved responsibilities"}));
    render(<Harness actor={null} visibleAgents={directory} visibleSquads={[]} />);
    const input = await openPicker(user);
    expect(document.querySelectorAll("button[data-picker-item]")).toHaveLength(50);
    await user.type(input, "Agent");
    await user.click(screen.getByRole("button", {name:"Show more (5 remaining)"}));
    expect(document.querySelectorAll("button[data-picker-item]")).toHaveLength(55);
    await user.click(screen.getByRole("button", {name:"Pin Agent 54 to favorites"}));
    expect(document.querySelectorAll("button[data-picker-item]")).toHaveLength(55);
    expect(screen.getByRole("button", {name:"Unpin Agent 54 from favorites"})).toHaveFocus();
    await user.type(input, " ");
    expect(document.querySelectorAll("button[data-picker-item]")).toHaveLength(50);
  });

  it("keeps a type-filtered empty shortcut view and restores favorites after clearing search", async () => {
    const user = userEvent.setup();
    render(<Harness favoriteActors={[a, b, {type:"agent", id:"c"}, {type:"agent", id:"d"}]} />);
    const input = await openPicker(user);
    expect(document.querySelectorAll("button[data-picker-item]")).toHaveLength(3);
    await user.click(screen.getByRole("button", {name:"View all favorites (4)"}));
    expect(document.querySelectorAll("button[data-picker-item]")).toHaveLength(4);
    await user.type(input, "Review squad");
    expect(document.querySelector('[data-actor-key="squad:a"]')).toBeInTheDocument();
    await user.clear(input);
    expect(document.querySelectorAll("button[data-picker-item]")).toHaveLength(4);
    await user.click(screen.getByRole("button", {name:"Back to shortcuts"}));
    await user.click(screen.getByRole("button", {name:"AI squads"}));
    expect(screen.getByText("No favorites or recently used actors of this type")).toBeInTheDocument();
    expect(screen.getByRole("button", {name:"Browse all (1)"})).toBeEnabled();
  });
});
