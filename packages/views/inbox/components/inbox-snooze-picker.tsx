"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";
import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import { ArrowLeft, CalendarClock, Clock } from "lucide-react";
import { ApiError } from "@multica/core/api";
import {
  inboxSnoozePresets,
  parseInboxSnoozeDateTime,
  toInboxLocalDateTime,
} from "@multica/core/inbox";
import type { InboxItem } from "@multica/core/types";
import { isImeComposing } from "@multica/core/utils";
import { Button } from "@multica/ui/components/ui/button";
import { Calendar } from "@multica/ui/components/ui/calendar";
import { Input } from "@multica/ui/components/ui/input";
import { Popover } from "@multica/ui/components/ui/popover";
import { usePopupVisibility } from "@multica/ui/components/ui/popup-visibility";
import { useT } from "../../i18n";
import { formatInboxReminderTime } from "./inbox-reminder-time";

export interface InboxSnoozeTarget {
  item: InboxItem;
  anchor: HTMLElement | null;
  focusScope?: HTMLElement | null;
}

/** One anchored picker serves row, context-menu, touch and detail actions. */
export function InboxSnoozePicker({
  target,
  busy,
  onClose,
  onSave,
  onRefresh,
}: {
  target: InboxSnoozeTarget | null;
  busy: boolean;
  onClose: () => void;
  onSave: (until: string) => Promise<void>;
  onRefresh: () => Promise<void>;
}) {
  const { t, i18n } = useT("inbox");
  const open = target !== null;
  const visible = usePopupVisibility();
  const popupId = useId();
  const groupKey = target ? target.item.issue_id ?? target.item.id : "";
  const targetRef = useRef(target);
  const firstPresetRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const timesRef = useRef<HTMLDivElement>(null);
  const inFlight = useRef(false);
  const [openedAt, setOpenedAt] = useState(() => new Date());
  const [custom, setCustom] = useState(false);
  const [value, setValue] = useState("");
  const [originalTime, setOriginalTime] = useState<{ value: string; date: Date } | null>(null);
  const [error, setError] = useState("");
  const [conflict, setConflict] = useState(false);
  const [saving, setSaving] = useState(false);
  const pending = busy || saving;
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const locale = i18n?.language ?? "en";
  const presets = useMemo(() => inboxSnoozePresets(openedAt), [openedAt]);

  useEffect(() => {
    if (target) targetRef.current = target;
  }, [target]);

  // A refreshed revision must not reset an unsaved custom time. Reset only
  // when opening a picker or moving it to a different notification group.
  useEffect(() => {
    if (!open) return;
    const current = targetRef.current;
    const now = new Date();
    const existing = current?.item.snoozed_until
      ? new Date(current.item.snoozed_until)
      : null;
    const initial = existing && existing.getTime() > now.getTime()
      ? existing
      : inboxSnoozePresets(now).find((preset) => preset.key === "tomorrow")?.date;
    setOpenedAt(now);
    const initialValue = initial ? toInboxLocalDateTime(initial) : "";
    setValue(initialValue);
    setOriginalTime(initial ? { value: initialValue, date: initial } : null);
    setCustom(false);
    setError("");
    setConflict(false);
  }, [open, groupKey]);

  useEffect(() => {
    const anchor = target?.anchor;
    const row = anchor?.closest('[role="listitem"]');
    row?.setAttribute("data-snooze-open", "");
    const trigger = anchor?.getAttribute("aria-haspopup") === "dialog" ? anchor : null;
    trigger?.setAttribute("aria-expanded", "true");
    trigger?.setAttribute("aria-controls", popupId);
    return () => {
      row?.removeAttribute("data-snooze-open");
      trigger?.setAttribute("aria-expanded", "false");
      trigger?.removeAttribute("aria-controls");
    };
  }, [target?.anchor, popupId]);

  useEffect(() => {
    if (custom && open) inputRef.current?.focus();
  }, [custom, open]);
  useEffect(() => {
    if (!visible && open) onClose();
  }, [visible, open, onClose]);

  const submit = async (date: Date | null) => {
    if (inFlight.current || pending) return;
    if (!date || !Number.isFinite(date.getTime()) || date.getTime() <= Date.now()) {
      setError(t(($) => $.snooze.invalid_time));
      inputRef.current?.focus();
      return;
    }
    setError("");
    setConflict(false);
    inFlight.current = true;
    setSaving(true);
    try {
      await onSave(date.toISOString());
    } catch (failure) {
      const changed = failure instanceof ApiError && failure.status === 409;
      setConflict(changed);
      setError(changed
        ? t(($) => $.snooze.changed)
        : t(($) => $.snooze.failed));
    } finally {
      inFlight.current = false;
      setSaving(false);
    }
  };

  const refresh = async () => {
    if (inFlight.current || pending) return;
    inFlight.current = true;
    setSaving(true);
    try {
      await onRefresh();
      setConflict(false);
      setError("");
    } catch {
      setError(t(($) => $.snooze.refresh_failed));
    } finally {
      inFlight.current = false;
      setSaving(false);
    }
  };

  const selectedDate = value ? new Date(`${value.slice(0, 10)}T12:00:00`) : undefined;
  // An unchanged wall time retains its original instant, including seconds
  // and the chosen offset in a repeated DST hour. submit rechecks its expiry.
  const resolveCustomDate = () => originalTime && value === originalTime.value
    ? originalTime.date
    : parseInboxSnoozeDateTime(value, new Date());
  const validCustomDate = resolveCustomDate();
  const today = new Date(openedAt.getFullYear(), openedAt.getMonth(), openedAt.getDate());

  return (
    <Popover open={open} modal onOpenChange={(nextOpen) => {
      if (!nextOpen && !pending && !inFlight.current) onClose();
    }}>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Positioner anchor={target?.anchor} side="bottom" align="end" sideOffset={6} className="isolate z-50">
          <PopoverPrimitive.Popup
            id={popupId}
            data-slot="popover-content"
            aria-busy={pending}
            hidden={!visible}
            inert={!visible}
            aria-hidden={!visible || undefined}
            initialFocus={firstPresetRef}
            finalFocus={() => {
              if (!visible) return false;
              const anchor = targetRef.current?.anchor;
              if (anchor?.isConnected) return anchor;
              const scope = targetRef.current?.focusScope;
              return scope?.isConnected
                ? scope.querySelector<HTMLElement>('[data-tab-scroll-root="list"]') ?? scope.querySelector<HTMLElement>("button[data-inbox-snoozed-entry]") ?? false
                : false;
            }}
            className="flex w-80 max-w-[calc(100vw-1rem)] max-h-[calc(100dvh-2rem)] flex-col gap-2 overflow-y-auto rounded-lg bg-surface-raised p-2.5 text-body text-popover-foreground shadow-[var(--menu-shadow)] ring-1 ring-surface-border outline-none"
            onKeyDown={(event) => {
              event.stopPropagation();
              if (isImeComposing(event)) return;
              if (event.key === "Escape") {
                event.preventDefault();
                if (!pending && !inFlight.current) onClose();
                return;
              }
              if (!timesRef.current?.contains(event.target as Node)) return;
              if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
              event.preventDefault();
              const buttons = Array.from(timesRef.current.querySelectorAll<HTMLButtonElement>("button:not(:disabled)"));
              const current = buttons.findIndex((button) => button === document.activeElement);
              const next = event.key === "Home" ? 0 : event.key === "End" ? buttons.length - 1
                : (current + (event.key === "ArrowDown" ? 1 : -1) + buttons.length) % buttons.length;
              buttons[next]?.focus();
            }}
          >
            <div className="flex items-center gap-2 px-1 py-1">
              <Clock className="size-4 shrink-0 text-muted-foreground" />
              <PopoverPrimitive.Title className="min-w-0 flex-1 text-body font-semibold">
                {target?.item.snoozed_until ? t(($) => $.snooze.change_time) : t(($) => $.snooze.title)}
              </PopoverPrimitive.Title>
            </div>
            {!custom ? (
              <div ref={timesRef} role="group" aria-label={t(($) => $.snooze.title)} className="space-y-0.5">
                {presets.map((preset, index) => (
                  <button
                    ref={index === 0 ? firstPresetRef : undefined}
                    key={preset.key}
                    type="button"
                    disabled={pending}
                    data-snooze-preset={preset.key}
                    onClick={() => void submit(preset.date)}
                    className="flex min-h-11 w-full items-center justify-between gap-3 rounded-md px-2 text-left text-body outline-none hover:bg-accent focus-visible:bg-accent focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50"
                  >
                    <span className="shrink-0">{t(($) => $.snooze.presets[preset.key])}</span>
                    <span className="text-right text-caption tabular-nums text-muted-foreground">{formatInboxReminderTime(preset.date.toISOString(), locale)}</span>
                  </button>
                ))}
                <button
                  type="button"
                  disabled={pending}
                  onClick={() => { setCustom(true); setError(""); }}
                  className="flex min-h-11 w-full items-center gap-2 rounded-md px-2 text-left text-body outline-none hover:bg-accent focus-visible:bg-accent focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50"
                >
                  <CalendarClock className="size-4 shrink-0 text-muted-foreground" />
                  {t(($) => $.snooze.custom)}
                </button>
              </div>
            ) : (
              <form className="space-y-3 px-1" onSubmit={(event) => { event.preventDefault(); void submit(resolveCustomDate()); }}>
                <Button variant="ghost" size="sm" disabled={pending} onClick={() => setCustom(false)} className="min-h-11 -ml-2 gap-1.5 text-muted-foreground">
                  <ArrowLeft className="size-3.5" />{t(($) => $.snooze.back)}
                </Button>
                <Calendar
                  mode="single"
                  selected={selectedDate && Number.isFinite(selectedDate.getTime()) ? selectedDate : undefined}
                  defaultMonth={selectedDate}
                  disabled={(date) => pending || date.getTime() < today.getTime()}
                  onSelect={(date) => {
                    if (date) setValue(`${toInboxLocalDateTime(date).slice(0, 10)}T${value.slice(11, 16) || "09:00"}`);
                    setError("");
                  }}
                  formatters={{
                    formatCaption: (date) => new Intl.DateTimeFormat(locale, { month: "long", year: "numeric" }).format(date),
                    formatWeekdayName: (date) => new Intl.DateTimeFormat(locale, { weekday: "short" }).format(date),
                  }}
                  labels={{
                    labelNext: () => t(($) => $.snooze.next_month),
                    labelPrevious: () => t(($) => $.snooze.previous_month),
                    labelDayButton: (date) => new Intl.DateTimeFormat(locale, { dateStyle: "full" }).format(date),
                  }}
                  className="mx-auto [@media(hover:none)]:hidden"
                />
                <label className="block space-y-1.5 text-caption font-medium">
                  <span>{t(($) => $.snooze.custom_time)}</span>
                  <Input ref={inputRef} type="datetime-local" value={value} disabled={pending} aria-invalid={!!error}
                    aria-label={t(($) => $.snooze.custom_time)} onChange={(event) => { setValue(event.target.value); setError(""); }} className="min-h-11 w-full text-body text-foreground [color-scheme:light] dark:[color-scheme:dark]" />
                </label>
                <p className="text-caption text-muted-foreground">{t(($) => $.snooze.timezone, { zone })}</p>
                {validCustomDate && <p className="text-caption tabular-nums text-muted-foreground">{formatInboxReminderTime(validCustomDate.toISOString(), locale, true)}</p>}
                <Button type="submit" disabled={pending} className="min-h-11 w-full">
                  {pending ? t(($) => $.snooze.saving) : target?.item.snoozed_until ? t(($) => $.snooze.save_time) : t(($) => $.snooze.confirm)}
                </Button>
              </form>
            )}
            {error && (
              <div role="alert" className="space-y-2 px-1 text-caption">
                <p className="text-destructive">{error}</p>
                {conflict && <Button variant="outline" size="sm" disabled={pending} onClick={() => void refresh()} className="min-h-11">{t(($) => $.snooze.refresh)}</Button>}
              </div>
            )}
            <PopoverPrimitive.Description className="px-1 text-caption leading-relaxed text-muted-foreground">{t(($) => $.snooze.hint)}</PopoverPrimitive.Description>
            <Button variant="ghost" size="sm" disabled={pending} onClick={onClose} className="min-h-11 self-end text-muted-foreground">{t(($) => $.snooze.cancel)}</Button>
          </PopoverPrimitive.Popup>
        </PopoverPrimitive.Positioner>
      </PopoverPrimitive.Portal>
    </Popover>
  );
}
