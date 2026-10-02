import { AVATAR_ICON_COMPONENTS } from "@multica/ui/lib/avatar-icon";
import { cn } from "@multica/ui/lib/utils";

/** Template identity survives instance renames; never infer it from a name. */
export function McpTemplateIcon({ templateKey, category }: {
  templateKey: string;
  category?: string;
}) {
  const reasoning = templateKey === "sequential-thinking" || category === "reasoning";
  const documentation = templateKey === "microsoft-learn" || templateKey === "deepwiki" || category === "documentation";
  const Icon = templateKey === "chrome-devtools"
    ? AVATAR_ICON_COMPONENTS.bug
    : templateKey === "playwright"
      ? AVATAR_ICON_COMPONENTS.workflow
      : templateKey === "microsoft-learn"
        ? AVATAR_ICON_COMPONENTS["book-open"]
        : templateKey === "deepwiki"
          ? AVATAR_ICON_COMPONENTS.search
          : reasoning
            ? AVATAR_ICON_COMPONENTS.brain
            : AVATAR_ICON_COMPONENTS.globe;

  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex size-10 shrink-0 items-center justify-center rounded-lg",
        documentation
          ? "bg-skill-writing/12 text-skill-writing"
          : reasoning
            ? "bg-skill-research/12 text-skill-research"
            : "bg-skill-engineering/12 text-skill-engineering",
      )}
    >
      <Icon className="size-5" />
    </span>
  );
}
