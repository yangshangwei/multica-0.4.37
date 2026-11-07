"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useWorkspacePaths } from "@multica/core/paths";
import { useRowLink } from "../../navigation";
import { SKILL_CARD_HEIGHT, SKILL_CARD_WITH_LABELS_HEIGHT, SkillCard } from "./skill-card";
import type { SkillActionsContext } from "./skill-list-actions";
import type { PresentedSkillRow } from "./skills-page";

const MIN_CARD_WIDTH = 240;
const GAP = 12;
const PAGE_PADDING_X = 24;
const BOTTOM_CLEARANCE = 96;

export function columnsForWidth(width: number): number {
  return Math.max(1, Math.floor((width + GAP) / (MIN_CARD_WIDTH + GAP)));
}

/**
 * Card grid virtualized by ROW: `useVirtualizer` is one-dimensional, so the
 * rows are chunked into lines of `columns` cards and each virtual item
 * renders one line. Columns come from the container width via
 * ResizeObserver (a CSS auto-fill grid could not tell us how many cards it
 * placed per line, and the virtualizer needs that number to size lines).
 */
export function SkillCardGrid({
  rows,
  ctx,
  selectedIds,
  onToggleSelected,
}: {
  rows: PresentedSkillRow[];
  ctx: SkillActionsContext;
  selectedIds: ReadonlySet<string>;
  onToggleSelected: (id: string) => void;
}) {
  const paths = useWorkspacePaths();
  const rowLink = useRowLink();
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [columns, setColumns] = useState(1);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const update = () =>
      setColumns(columnsForWidth(el.clientWidth - PAGE_PADDING_X * 2));
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const lines = useMemo(() => {
    const out: { rows: PresentedSkillRow[]; height: number; key: string }[] = [];
    for (let i = 0; i < rows.length; i += columns) {
      const line = rows.slice(i, i + columns);
      const height = line.some((row) => row.labels.length > 0)
        ? SKILL_CARD_WITH_LABELS_HEIGHT
        : SKILL_CARD_HEIGHT;
      // Include height in the key so filtering or label changes cannot reuse
      // a cached estimate from a line with a different content height.
      out.push({ rows: line, height, key: `${height}:${line.map((row) => row.skill.id).join(":")}` });
    }
    return out;
  }, [rows, columns]);

  const virtualizer = useVirtualizer({
    count: lines.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: (index) => lines[index]!.height + GAP,
    getItemKey: (index) => lines[index]!.key,
    overscan: 3,
  });

  return (
    <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto">
      <div
        className="relative w-full"
        style={{
          height: virtualizer.getTotalSize() + BOTTOM_CLEARANCE,
          paddingLeft: PAGE_PADDING_X,
          paddingRight: PAGE_PADDING_X,
        }}
      >
        {virtualizer.getVirtualItems().map((vi) => {
          const line = lines[vi.index];
          if (!line) return null;
          return (
            <div
              key={vi.key}
              className="absolute left-6 right-6 grid"
              style={{
                top: vi.start + GAP,
                gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
                gap: GAP,
              }}
            >
              {line.rows.map((row) => (
                <SkillCard
                  key={row.skill.id}
                  row={row}
                  href={paths.skillDetail(row.skill.id)}
                  ctx={ctx}
                  height={line.height}
                  selected={selectedIds.has(row.skill.id)}
                  onToggleSelected={() => onToggleSelected(row.skill.id)}
                  linkProps={rowLink(
                    paths.skillDetail(row.skill.id),
                    row.presentation.name,
                  )}
                />
              ))}
            </div>
          );
        })}
      </div>
    </div>
  );
}
