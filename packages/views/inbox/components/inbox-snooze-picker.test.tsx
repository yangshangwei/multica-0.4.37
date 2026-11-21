import { useRef, useState } from "react";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api";
import type { InboxItem } from "@multica/core/types";
import { PopupVisibilityProvider } from "@multica/ui/components/ui/popup-visibility";
import { renderWithI18n } from "../../test/i18n";
import { InboxSnoozePicker, type InboxSnoozeTarget } from "./inbox-snooze-picker";

const item: InboxItem = {
  id: "notice-1", workspace_id: "workspace-1", recipient_type: "member", recipient_id: "member-1",
  actor_type: "member", actor_id: "member-2", type: "new_comment", severity: "info", issue_id: "issue-1",
  title: "Review release checklist", body: null, issue_status: null, read: true, archived: false,
  created_at: "2026-11-20T10:00:00Z", details: null,
};

function Fixture({ save, refresh = async () => {}, entry = item }: { save: (until: string) => Promise<unknown>; refresh?: () => Promise<void>; entry?: InboxItem }) {
  const anchor = useRef<HTMLButtonElement>(null);
  const [target, setTarget] = useState<InboxSnoozeTarget | null>(null);
  return <>
    <button type="button" ref={anchor} aria-haspopup="dialog" aria-expanded="false" onClick={() => setTarget({ item: entry, anchor: anchor.current })}>Open reminder</button>
    <InboxSnoozePicker target={target} busy={false} onClose={() => setTarget(null)} onRefresh={refresh}
      onSave={async (until) => { await save(until); setTarget(null); }} />
  </>;
}

// Calendar/timezone/DST matrices live in Core's inbox/snooze-time.test.ts.
// This suite owns the real popup, input wiring, keyboard/focus and recovery.
describe("InboxSnoozePicker", () => {
  it("shows the five quick times with an explicit resolved date", async () => {
    renderWithI18n(<Fixture save={vi.fn().mockResolvedValue(undefined)} />);
    const opener = screen.getByRole("button", { name: "Open reminder" });
    await userEvent.setup().click(opener);

    const dialog = await screen.findByRole("dialog", { name: "Snooze notification" });
    expect(opener).toHaveAttribute("aria-expanded", "true");
    expect(dialog.querySelectorAll("[data-snooze-preset]")).toHaveLength(5);
    for (const label of ["In 1 hour", "In 3 hours", "Tomorrow", "In 3 days", "In 1 week"]) {
      expect(screen.getByRole("button", { name: new RegExp(`^${label}`) })).toHaveTextContent(/\d{4}/);
    }
    expect(screen.getByText(/A direct mention, assignment to you, or agent failure/)).toBeInTheDocument();
  });

  it("owns menu arrows and Escape and returns focus to its visible opener", async () => {
    const user = userEvent.setup();
    renderWithI18n(<Fixture save={vi.fn().mockResolvedValue(undefined)} />);
    const opener = screen.getByRole("button", { name: "Open reminder" });
    await user.click(opener);
    const first = await screen.findByRole("button", { name: /^In 1 hour/ });
    await waitFor(() => expect(first).toHaveFocus());
    const behind = vi.fn();
    document.addEventListener("keydown", behind);
    try {
      await user.keyboard("{ArrowDown}");
      expect(screen.getByRole("button", { name: /^In 3 hours/ })).toHaveFocus();
      await user.keyboard("{Escape}");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(opener).toHaveFocus();
      expect(opener).toHaveAttribute("aria-expanded", "false");
      expect(behind).not.toHaveBeenCalled();
    } finally {
      document.removeEventListener("keydown", behind);
    }
  });

  it("returns focus to its own visible list when a successful action removes the opener", async () => {
    function RemovableOpener() {
      const scope = useRef<HTMLDivElement>(null);
      const anchor = useRef<HTMLButtonElement>(null);
      const [shown, setShown] = useState(true);
      const [target, setTarget] = useState<InboxSnoozeTarget | null>(null);
      return <div ref={scope}>
        {shown && <button type="button" ref={anchor} onClick={() => setTarget({ item, anchor: anchor.current, focusScope: scope.current })}>Open reminder</button>}
        <div role="list" aria-label="Visible inbox" data-tab-scroll-root="list" tabIndex={-1} />
        <InboxSnoozePicker target={target} busy={false} onClose={() => setTarget(null)} onRefresh={async () => {}}
          onSave={async () => { setShown(false); setTarget(null); }} />
      </div>;
    }
    renderWithI18n(<><div hidden data-tab-scroll-root="list" tabIndex={-1} /><RemovableOpener /></>);
    const list = screen.getByRole("list", { name: "Visible inbox" });
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Open reminder" }));
    await user.click(await screen.findByRole("button", { name: /^In 1 hour/ }));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(list).toHaveFocus();
  });

  it("announces a rejected custom time and retains input without submitting", async () => {
    const save = vi.fn();
    const user = userEvent.setup();
    renderWithI18n(<Fixture save={save} />);
    await user.click(screen.getByRole("button", { name: "Open reminder" }));
    await user.click(await screen.findByRole("button", { name: "Choose date and time..." }));
    const input = screen.getByLabelText("Reminder date and time");
    fireEvent.change(input, { target: { value: "2000-01-01T09:00" } });
    await user.click(screen.getByRole("button", { name: "Snooze" }));

    expect(screen.getByRole("alert")).toHaveTextContent("Choose a valid future date and time.");
    expect(input).toHaveValue("2000-01-01T09:00");
    expect(input).toHaveFocus();
    expect(save).not.toHaveBeenCalled();
  });

  it("keeps a failed custom time and allows retry with the same value", async () => {
    const save = vi.fn().mockRejectedValueOnce(new Error("internal_receipt_schema debug text")).mockResolvedValue(undefined);
    const user = userEvent.setup();
    renderWithI18n(<Fixture save={save} />);
    await user.click(screen.getByRole("button", { name: "Open reminder" }));
    await user.click(await screen.findByRole("button", { name: "Choose date and time..." }));
    const input = screen.getByLabelText("Reminder date and time");
    const value = "2030-11-22T12:30";
    fireEvent.change(input, { target: { value } });
    await user.click(screen.getByRole("button", { name: "Snooze" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Could not snooze this notification. Try again.");
    expect(screen.queryByText(/internal_receipt_schema/)).not.toBeInTheDocument();
    expect(screen.getByLabelText("Reminder date and time")).toBe(input);
    expect(input).toHaveValue(value);
    expect(screen.getByText(/Current time zone:/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Snooze" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(save).toHaveBeenNthCalledWith(2, new Date(value).toISOString());
  });

  it("saves an unchanged existing absolute instant including its seconds", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const instant = "2030-11-22T12:30:45.678Z";
    const user = userEvent.setup();
    renderWithI18n(<Fixture save={save} entry={{ ...item, snoozed_until: instant, snooze_revision: 4 }} />);
    await user.click(screen.getByRole("button", { name: "Open reminder" }));
    await user.click(await screen.findByRole("button", { name: "Choose date and time..." }));

    expect(screen.getByText(/GMT(?:[+-]\d+)?/)).toHaveTextContent("2030");
    await user.click(screen.getByRole("button", { name: "Save time" }));

    await waitFor(() => expect(save).toHaveBeenCalledWith(instant));
  });

  it("localizes request failures without rendering internal English text", async () => {
    const save = vi.fn().mockRejectedValue(new Error("internal_receipt_schema debug text"));
    const user = userEvent.setup();
    renderWithI18n(<Fixture save={save} />, { locale: "zh-Hans" });
    await user.click(screen.getByRole("button", { name: "Open reminder" }));
    await user.click(await screen.findByRole("button", { name: /^1 小时后/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent("无法延后这条通知，请重试。");
    expect(screen.queryByText(/internal_receipt_schema/)).not.toBeInTheDocument();
  });

  it("refreshes a conflict without discarding the custom input", async () => {
    const save = vi.fn().mockRejectedValue(new ApiError("changed", 409, "Conflict"));
    const refresh = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    renderWithI18n(<Fixture save={save} refresh={refresh} />);
    await user.click(screen.getByRole("button", { name: "Open reminder" }));
    await user.click(await screen.findByRole("button", { name: "Choose date and time..." }));
    const input = screen.getByLabelText("Reminder date and time");
    fireEvent.change(input, { target: { value: "2030-11-22T12:30" } });
    await user.click(screen.getByRole("button", { name: "Snooze" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("This reminder changed on another device.");
    await user.click(screen.getByRole("button", { name: "Use latest state" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(refresh).toHaveBeenCalledOnce();
    expect(input).toHaveValue("2030-11-22T12:30");
  });

  it("closes its portal when a retained tab hides and does not resurrect it", async () => {
    const user = userEvent.setup();
    const save = vi.fn().mockResolvedValue(undefined);
    const contents = (visible: boolean) => <PopupVisibilityProvider visible={visible}><Fixture save={save} /></PopupVisibilityProvider>;
    const view = renderWithI18n(contents(true));
    await user.click(screen.getByRole("button", { name: "Open reminder" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();

    act(() => view.rerender(contents(false)));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    act(() => view.rerender(contents(true)));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Open reminder" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });
});
