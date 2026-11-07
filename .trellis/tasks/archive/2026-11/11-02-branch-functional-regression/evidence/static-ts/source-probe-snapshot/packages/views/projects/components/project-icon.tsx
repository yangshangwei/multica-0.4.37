import type { Project } from "@multica/core/types";
import { AVATAR_ICON_COMPONENTS, AVATAR_ICON_TONE, resolveAvatarIcon, type AvatarIconName } from "@multica/ui/lib/avatar-icon";
import { cn } from "@multica/ui/lib/utils";

export type ProjectIconSize = "sm" | "md" | "lg";

export interface ProjectIconProps {
  project?: Pick<Project, "icon"> | null;
  size?: ProjectIconSize;
  className?: string;
}

const SIZE_CLASS: Record<ProjectIconSize, string> = {
  sm: "size-3.5 text-caption leading-none",
  md: "size-4 text-body leading-none",
  lg: "size-6 text-display-sm leading-none",
};

export function resolveProjectIcon(value?: string | null): AvatarIconName {
  if (!value || value === "📁") return "package";
  return resolveAvatarIcon(value.startsWith("icon:") || value.startsWith("emoji:") ? value : `emoji:${value}`) ?? "package";
}

export function ProjectIcon({ project, size = "sm", className }: ProjectIconProps) {
  const name = resolveProjectIcon(project?.icon);
  const Icon = AVATAR_ICON_COMPONENTS[name];
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex shrink-0 items-center justify-center",
        SIZE_CLASS[size],
        AVATAR_ICON_TONE[name].text,
        className,
      )}
    >
      <Icon className="size-full" />
    </span>
  );
}
