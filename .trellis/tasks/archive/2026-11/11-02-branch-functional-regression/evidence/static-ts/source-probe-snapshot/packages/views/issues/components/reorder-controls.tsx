"use client";

import { useCallback, useState } from "react";
import { ArrowDown, ArrowUp } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import type { OrderDirection } from "../utils/adjacent-order";

export interface ReorderOptions {
  index: number;
  count: number;
  hasMore?: boolean;
  disabled?: boolean;
  onMove: (direction: OrderDirection) => void;
}

/** Shared, pointer-and-keyboard alternative to dragging rows, cards and lanes. */
export function ReorderControls({
  id, name, index, count, hasMore = false, disabled = false, onMove,
}: ReorderOptions & { id: string; name: string }) {
  const { t } = useT("issues");
  return (
    <span className="inline-flex shrink-0 items-center" onPointerDown={(event) => event.stopPropagation()}>
      {([-1, 1] as const).map((direction) => {
        const unavailable = disabled || index < 0 || (direction === -1
          ? index === 0
          : index >= count - (hasMore ? 2 : 1));
        const label = direction === -1
          ? t(($) => $.reorder.move_up, { name })
          : t(($) => $.reorder.move_down, { name });
        return (
          <Button
            key={direction}
            type="button"
            variant="ghost"
            size="icon-sm"
            data-reorder-id={id}
            data-reorder-direction={direction}
            aria-label={label}
            title={label}
            // Keep focus when the moved row reaches a boundary or is pending.
            aria-disabled={unavailable}
            className="text-muted-foreground aria-disabled:opacity-35"
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              if (unavailable) return;
              const button = event.currentTarget;
              const owner = button.closest("[data-reorder-scope]");
              onMove(direction);
              // Virtuoso may replace the focused row at a viewport edge. The
              // caller scrolls to the new index; restore only our own focus.
              requestAnimationFrame(() => {
                if (!owner?.isConnected || (document.activeElement !== button && document.activeElement !== document.body)) return;
                const target = [...owner.querySelectorAll<HTMLButtonElement>("button[data-reorder-id]")]
                  .find((candidate) => candidate.dataset.reorderId === id && candidate.dataset.reorderDirection === String(direction));
                target?.focus({ preventScroll: true });
                target?.scrollIntoView?.({ block: "nearest", inline: "nearest" });
              });
            }}
          >
            {direction === -1 ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />}
          </Button>
        );
      })}
    </span>
  );
}

export function useReorderAnnouncement() {
  const { t } = useT("issues");
  const [message, setMessage] = useState("");
  const announce = useCallback((name: string, position?: number, error?: unknown) => {
    setMessage(error
      ? t(($) => $.page.move_failed)
      : position === undefined
        ? t(($) => $.reorder.moving, { name })
        : t(($) => $.reorder.moved, { name, position }));
  }, [t]);
  return { announce, announcement: <span role="status" aria-live="polite" aria-atomic="true" className="sr-only">{message}</span> };
}
