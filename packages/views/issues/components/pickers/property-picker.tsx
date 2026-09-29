"use client";

import { useState, useCallback, useRef, useEffect, useLayoutEffect } from "react";
import { Check } from "lucide-react";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@multica/ui/components/ui/popover";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { isImeComposing } from "@multica/core/utils";
import { useT } from "../../../i18n";

const HIGHLIGHT_CLASS = "bg-accent";
const ITEM_SELECTOR = "button[data-picker-item]:not(:disabled)";
/**
 * Marks the fixed empty-value row ("No project", "Unassigned", …). The row is
 * pinned first for the eye, but it is not a search result — typing a query and
 * pressing Enter must commit the first real match, never clear the field.
 */
const EMPTY_ITEM_ATTR = "data-picker-empty";

const isEmptyItem = (el: HTMLButtonElement | undefined) =>
  el?.hasAttribute(EMPTY_ITEM_ATTR) === true;

/**
 * Default class of the picker popover trigger. Shared with the deferred
 * (pre-mount) lookalike trigger in `DeferredPopup` call sites so the swap on
 * first interaction is pixel-identical.
 */
export const PICKER_TRIGGER_CLASS =
  "flex items-center gap-1.5 cursor-pointer rounded px-1 -mx-1 hover:bg-accent/30 transition-colors overflow-hidden";

// ---------------------------------------------------------------------------
// PropertyPicker — generic Popover shell with optional search
// ---------------------------------------------------------------------------

export function PropertyPicker({
  open,
  onOpenChange,
  trigger,
  triggerRender,
  width = "w-48",
  align = "end",
  side = "bottom",
  searchable = false,
  searchPlaceholder,
  onSearchChange,
  navigationResetKey,
  searchInputRef,
  header,
  tooltip,
  children,
  footer,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  trigger: React.ReactNode;
  triggerRender?: React.ReactElement;
  width?: string;
  align?: "start" | "center" | "end";
  side?: React.ComponentProps<typeof PopoverContent>["side"];
  searchable?: boolean;
  searchPlaceholder?: string | undefined;
  onSearchChange?: (query: string) => void;
  /** Invalidate navigation after an external result change. Typed search wins
   * over a simultaneous reset; append-only display pages may keep this key. */
  navigationResetKey?: string;
  searchInputRef?: React.Ref<HTMLInputElement>;
  /** Custom sticky header rendered above the scrollable list. Use for
   *  filter toggles, search inputs, or any UI that must stay visible while
   *  the list scrolls. The built-in `searchable` input renders just above
   *  this header when both are present. */
  header?: React.ReactNode;
  /** Optional design-system tooltip shown when the trigger is hovered while
   *  the popover is closed. Suppressed automatically when the popover is
   *  open (otherwise tooltip + popover would stack on the same anchor). */
  tooltip?: React.ReactNode;
  children: React.ReactNode;
  /**
   * Optional footer rendered below the listbox. Unlike items rendered as
   * children, the footer is *not* included in arrow-key navigation — use it
   * for actions like "Create new…" or "Manage…" that shouldn't be treated as
   * selectable listbox options.
   */
  footer?: React.ReactNode;
}) {
  const { t } = useT("issues");
  const placeholder = searchPlaceholder ?? t(($) => $.filters.placeholder);
  const filterAria = t(($) => $.pickers.filter_options_aria);
  const [query, setQuery] = useState("");
  const highlightedIndex = useRef(-1);
  const navigation = useRef<{ key: string | undefined; items: HTMLButtonElement[] }>({ key: navigationResetKey, items: [] });
  const suppressUniqueSelection = useRef(false);
  const [tooltipHover, setTooltipHover] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);
  // Show the tooltip only while the trigger is hovered AND the popover is
  // closed — avoids the awkward state where the tooltip floats next to (or
  // on top of) the popover that just opened on click.
  const tooltipOpen = !!tooltip && tooltipHover && !open;

  const getItems = useCallback(() => {
    if (!listRef.current) return [];
    return Array.from(
      listRef.current.querySelectorAll<HTMLButtonElement>(ITEM_SELECTOR),
    );
  }, []);

  const pendingSearchHighlight = useRef(false);
  const paintHighlight = useCallback((items: HTMLButtonElement[], index: number) => {
    highlightedIndex.current = index;
    for (const item of items) {
      item.classList.remove(HIGHLIGHT_CLASS);
    }
    items[index]?.classList.add(HIGHLIGHT_CLASS);
  }, []);

  // Resolve against the committed candidates, not the previous search DOM.
  // Also called at keydown: a child layout effect can fire Enter before this
  // shell's layout effect has observed a changed candidate signature.
  const syncNavigation = useCallback((items: HTMLButtonElement[]) => {
    const previous = navigation.current;
    if (pendingSearchHighlight.current) {
      pendingSearchHighlight.current = false;
      suppressUniqueSelection.current = false;
      paintHighlight(items, items.findIndex((item) => !isEmptyItem(item)));
    } else if (navigationResetKey !== undefined && (
      previous.key !== navigationResetKey ||
      previous.items.some((item, index) => item !== items[index])
    )) {
      suppressUniqueSelection.current = true;
      paintHighlight(items, -1);
    } else {
      paintHighlight(items, highlightedIndex.current);
    }
    navigation.current = { key: navigationResetKey, items };
  }, [navigationResetKey, paintHighlight]);

  useLayoutEffect(() => {
    syncNavigation(getItems());
  }, [children, getItems, syncNavigation, open]);

  // Reset the search state on the open -> closed transition rather than inside
  // an open-change handler. Every picker closes itself after a selection by
  // calling its own `setOpen(false)`, which flips this `open` prop directly and
  // never routes through the popover's `onOpenChange` — so a handler-only reset
  // left the stale query (and the filtered list) in place on the next open.
  const wasOpen = useRef(open);
  useEffect(() => {
    if (wasOpen.current && !open) {
      setQuery("");
      highlightedIndex.current = -1;
      pendingSearchHighlight.current = false;
      suppressUniqueSelection.current = false;
      navigation.current = { key: navigationResetKey, items: [] };
      onSearchChange?.("");
    }
    wasOpen.current = open;
  }, [open, onSearchChange, navigationResetKey]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      // IME is composing — Enter/Arrow belong to the IME (Enter commits
      // composition; Arrow rotates candidates). Don't hijack them.
      if (isImeComposing(e)) return;
      const items = getItems();
      syncNavigation(items);
      if (items.length === 0) return;

      if (e.key === "ArrowDown") {
        e.preventDefault();
        suppressUniqueSelection.current = false;
        const next = highlightedIndex.current < items.length - 1 ? highlightedIndex.current + 1 : 0;
        paintHighlight(items, next);
        items[next]?.scrollIntoView({ block: "nearest" });
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        suppressUniqueSelection.current = false;
        const next = highlightedIndex.current > 0 ? highlightedIndex.current - 1 : items.length - 1;
        paintHighlight(items, next);
        items[next]?.scrollIntoView({ block: "nearest" });
      } else if (e.key === "Enter") {
        e.preventDefault();
        if (highlightedIndex.current >= 0 && highlightedIndex.current < items.length) {
          items[highlightedIndex.current]?.click();
        } else if (!suppressUniqueSelection.current && items.length === 1 && !isEmptyItem(items[0])) {
          // Auto-select when only one result. The empty row is excluded: a
          // query with no matches leaves it as the sole item, and Enter there
          // would clear the field the user was trying to search in.
          items[0]?.click();
        }
      }
    },
    [getItems, syncNavigation, paintHighlight],
  );

  const popoverTrigger = (
    <PopoverTrigger
      className={triggerRender ? undefined : PICKER_TRIGGER_CLASS}
      render={triggerRender}
    >
      {trigger}
    </PopoverTrigger>
  );

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      {tooltip ? (
        <Tooltip open={tooltipOpen} onOpenChange={setTooltipHover}>
          <TooltipTrigger render={popoverTrigger} />
          <TooltipContent side="top">{tooltip}</TooltipContent>
        </Tooltip>
      ) : (
        popoverTrigger
      )}
      <PopoverContent align={align} side={side} className={`${width} gap-0 p-0`}>
        {searchable && (
          <div className="px-2 py-1.5 border-b">
            <input
              ref={searchInputRef}
              type="text"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                pendingSearchHighlight.current = true;
                onSearchChange?.(e.target.value);
              }}
              onKeyDown={handleKeyDown}
              placeholder={placeholder}
              aria-label={filterAria}
              className="w-full bg-transparent text-body placeholder:text-muted-foreground outline-none"
            />
          </div>
        )}
        {header && <div className="border-b">{header}</div>}
        <div ref={listRef} className="p-1 max-h-72 overflow-y-auto">{children}</div>
        {footer && <div className="border-t p-1">{footer}</div>}
      </PopoverContent>
    </Popover>
  );
}

// ---------------------------------------------------------------------------
// PickerItem — single selectable row
// ---------------------------------------------------------------------------

export function PickerItem({
  selected,
  disabled,
  onClick,
  hoverClassName,
  tooltip,
  emptyValue = false,
  children,
}: {
  selected: boolean;
  disabled?: boolean;
  onClick: () => void;
  hoverClassName?: string;
  /**
   * Marks this row as the field's fixed empty value ("No project",
   * "Unassigned", "No stage"). Such a row is pinned first for the eye but
   * excluded from search-result keyboard defaults, so typing a query and
   * pressing Enter commits the first real match instead of clearing the
   * field. Arrow keys can still reach it. Set this on rows that write
   * null/undefined — not on a real enum member that happens to read as
   * "none" (issue priority), which is a value like any other.
   */
  emptyValue?: boolean;
  /** Design-system tooltip for the row — useful when truncated content needs
   *  the full string, or when the row carries metadata that doesn't fit on
   *  a single line. Wrapped in a real Tooltip component (200ms delay,
   *  styled), not a native `title` attribute. */
  tooltip?: React.ReactNode;
  children: React.ReactNode;
}) {
  const button = (
    <button
      type="button"
      data-picker-item
      {...(emptyValue ? { [EMPTY_ITEM_ATTR]: "" } : {})}
      disabled={disabled}
      onClick={onClick}
      className={`flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-left text-body ${disabled ? "opacity-50 cursor-not-allowed" : hoverClassName ?? "hover:bg-accent"} transition-colors`}
    >
      {/* min-w-0 lets long children (like truncated label names) shrink
          inside the flex row instead of pushing the selected checkmark off
          the right edge. The check column always reserves its 14px slot
          (visible when selected, invisible otherwise) so unselected rows
          align with selected rows and the eye doesn't chase a jittery
          right edge. */}
      <span className="flex min-w-0 flex-1 items-center gap-2">{children}</span>
      <Check
        className={`h-3.5 w-3.5 shrink-0 text-muted-foreground ${
          selected ? "" : "invisible"
        }`}
      />
    </button>
  );

  if (!tooltip) return button;

  return (
    <Tooltip>
      <TooltipTrigger render={button} />
      <TooltipContent side="top">{tooltip}</TooltipContent>
    </Tooltip>
  );
}

// ---------------------------------------------------------------------------
// PickerSection — group header
// ---------------------------------------------------------------------------

export function PickerSection({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="px-2 pt-2 pb-1 text-caption font-medium text-muted-foreground uppercase tracking-wider">
        {label}
      </div>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// PickerEmpty — no results state
// ---------------------------------------------------------------------------

export function PickerEmpty() {
  const { t } = useT("issues");
  return (
    <div className="px-2 py-3 text-center text-body text-muted-foreground">
      {t(($) => $.pickers.no_results)}
    </div>
  );
}
