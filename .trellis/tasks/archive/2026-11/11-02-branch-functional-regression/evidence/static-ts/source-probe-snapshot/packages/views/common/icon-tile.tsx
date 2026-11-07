import type { ComponentProps } from "react";
import type { LucideIcon } from "lucide-react";
import { cn } from "@multica/ui/lib/utils";

const SIZES = {
  sm: { box: "size-6 rounded-md", icon: "size-3.5" },
  md: { box: "size-10 rounded-lg", icon: "size-5" },
  lg: { box: "size-12 rounded-xl", icon: "size-6" },
} as const;

export type IconTileSize = keyof typeof SIZES;

/**
 * The toned icon square on skill, MCP and automation template cards. One size
 * table keeps those catalogs reading as one family. Decorative — callers put
 * the accessible name on the text next to it.
 */
export function IconTile({
  icon: Icon,
  tone,
  size = "md",
  className,
  ...props
}: Omit<ComponentProps<"span">, "children"> & {
  icon: LucideIcon;
  /** Background and glyph colour classes, e.g. `bg-skill-data/12 text-skill-data`. */
  tone: string;
  size?: IconTileSize;
}) {
  const s = SIZES[size];
  return (
    <span
      {...props}
      aria-hidden="true"
      className={cn("inline-flex shrink-0 items-center justify-center", s.box, tone, className)}
    >
      <Icon className={s.icon} />
    </span>
  );
}
