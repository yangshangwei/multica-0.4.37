"use client";

import type { ReactNode } from "react";
import { isKnownAutonomyLevel, type AgentRoleTemplate } from "@multica/core/types";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";

export function RoleTemplateCard({
  template,
  createHref,
  createLabel,
  secondaryAction = false,
  onNavigate,
  children,
}: {
  template: AgentRoleTemplate;
  createHref: string;
  createLabel: string;
  secondaryAction?: boolean;
  onNavigate?: () => void;
  children?: ReactNode;
}) {
  const title = template.title || template.name;
  return (
    <li aria-label={title} className="flex min-w-0 flex-col items-start gap-3 border-t border-border py-5">
      <div className="flex w-full flex-wrap items-start justify-between gap-2">
        <h3 className="min-w-0 break-words text-body font-semibold">
          <ActorAvatar name={template.title} initials="" avatarUrl={`emoji:${template.avatar_emoji}`} isAgent size="md" className="mr-2" />
          {title}
        </h3>
        <AutonomyBadge level={template.autonomy_level} />
      </div>
      <p className="break-words text-caption leading-5 text-muted-foreground">{template.description}</p>
      {children}
      <AppLink
        href={createHref}
        onClick={onNavigate}
        onAuxClick={onNavigate}
        className={cn(
          "mt-auto flex min-h-11 items-center rounded-md px-3 py-2 text-caption font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:min-h-9 pointer-coarse:min-h-11",
          secondaryAction
            ? "text-muted-foreground hover:bg-surface-hover hover:text-foreground"
            : "border border-border hover:bg-surface-hover",
        )}
      >
        {createLabel}
      </AppLink>
    </li>
  );
}

/** Unknown policies must not claim a permission limit this client cannot explain. */
export function AutonomyBadge({ level, className }: { level: string | undefined; className?: string }) {
  const { t } = useT("agents");
  if (!isKnownAutonomyLevel(level)) return null;
  return (
    <span
      className={cn("rounded-full border px-2 py-0.5 text-micro font-medium text-muted-foreground", className)}
      title={t(($) => $.role_templates.autonomy_hint[level])}
    >
      {t(($) => $.role_templates.autonomy[level])}
    </span>
  );
}
