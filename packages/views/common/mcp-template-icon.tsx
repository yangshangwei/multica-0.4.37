import { AVATAR_ICON_COMPONENTS } from "@multica/ui/lib/avatar-icon";
import { cn } from "@multica/ui/lib/utils";

/** Template identity survives instance renames; never infer it from a name. */
export function McpTemplateIcon({ templateKey, category }: {
  templateKey: string;
  category?: string;
}) {
  const reasoning = templateKey === "sequential-thinking" || category === "reasoning";
  const Icon = templateKey === "chrome-devtools"
    ? AVATAR_ICON_COMPONENTS.bug
    : templateKey === "playwright"
      ? AVATAR_ICON_COMPONENTS.workflow
      : reasoning
        ? AVATAR_ICON_COMPONENTS.brain
        : AVATAR_ICON_COMPONENTS.globe;

  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex size-10 shrink-0 items-center justify-center rounded-lg",
        reasoning
          ? "bg-skill-research/12 text-skill-research"
          : "bg-skill-engineering/12 text-skill-engineering",
      )}
    >
      <Icon className="size-5" />
    </span>
  );
}
