"use client";

import type { Label } from "@multica/core/types";
import { X } from "lucide-react";
import { useT } from "../i18n";

function relativeLuminance(hex: string): number {
  const linear = (offset: number) => {
    const channel = parseInt(hex.slice(offset, offset + 2), 16) / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * linear(1) + 0.7152 * linear(3) + 0.0722 * linear(5);
}

const TEXT_CANDIDATES = ["#111827", "#f9fafb"].map((color) => ({
  color,
  luminance: relativeLuminance(color),
}));

/** Preserve valid label fills and guarantee WCAG AA contrast for small text. */
export function labelChipColors(hex: string) {
  // The API accepts optional-# six-digit hex. Invalid values need an opaque
  // fill as well as a foreground; inheriting a page background is not safe.
  const backgroundColor = /^#?[0-9a-f]{6}$/i.test(hex)
    ? `#${hex.replace("#", "").toLowerCase()}`
    : "#6b7280";
  const luminance = relativeLuminance(backgroundColor);
  let color = "#000000";
  let bestContrast = 0;
  for (const candidate of TEXT_CANDIDATES) {
    const contrast = (Math.max(luminance, candidate.luminance) + 0.05)
      / (Math.min(luminance, candidate.luminance) + 0.05);
    if (contrast > bestContrast) {
      color = candidate.color;
      bestContrast = contrast;
    }
  }
  if (bestContrast < 4.5) {
    // At least one of black/white reaches 4.5 for every opaque sRGB fill.
    color = (luminance + 0.05) / 0.05 >= 1.05 / (luminance + 0.05)
      ? "#000000"
      : "#ffffff";
  }
  return { backgroundColor, color };
}

interface LabelChipProps {
  label: Label;
  onRemove?: () => void;
  className?: string;
  /**
   * When true, show the full label name without truncation. Use this in
   * management/edit surfaces where users need to see or verify the exact
   * name. The default (false) truncates at 12rem to keep chips compact in
   * the issue sidebar and future board/list card rows.
   */
  fullName?: boolean;
}

/**
 * Renders a single label as a colored pill. If `onRemove` is provided, shows
 * an × button that calls it. Used in the issue-detail sidebar, the picker,
 * and the management dialog.
 */
export function LabelChip({ label, onRemove, className, fullName }: LabelChipProps) {
  const { t } = useT("labels");
  const colors = labelChipColors(label.color);
  const nameClass = fullName ? "break-all" : "truncate max-w-[12rem]";
  return (
    <span
      // `min-w-0` lets the chip shrink when it is a flex item in a width-capped
      // container (the create toolbar's label pill). Without it the chip keeps
      // its content width and the parent clips it mid-shape; with it the inner
      // name truncates and the chip keeps its rounded ends.
      className={`inline-flex min-w-0 items-center gap-1 rounded-full px-2 py-0.5 text-caption font-medium ${className ?? ""}`}
      style={colors}
      // aria-label exposes the full name to screen readers when the span
      // visually truncates. title stays for sighted hover-tooltip.
      aria-label={label.name}
      title={label.name}
    >
      <span className={nameClass}>{label.name}</span>
      {onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          // bg-current/20 uses the computed text color so the hover state is
          // visible on both light and dark chip backgrounds. hover:bg-black/10
          // was invisible on darker chips (anything requiring light text).
          className="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full hover:bg-current/20 focus:outline-none focus:ring-1 focus:ring-current"
          aria-label={t(($) => $.remove_label, { name: label.name })}
        >
          <X className="h-2.5 w-2.5" strokeWidth={2.5} />
        </button>
      )}
    </span>
  );
}
