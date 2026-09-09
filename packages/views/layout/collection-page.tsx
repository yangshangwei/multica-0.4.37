"use client";

import type { ComponentProps, ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@multica/ui/components/ui/empty";
import { cn } from "@multica/ui/lib/utils";
import { AppLink } from "../navigation";
import { PageHeader } from "./page-header";

const LEARN_MORE_CLASS =
  "underline decoration-muted-foreground/30 underline-offset-4 transition-colors hover:text-foreground";

/**
 * The header's supporting link, which since the in-app documentation landed can
 * point either outside the app or at a route inside it.
 *
 * A root-relative href has to go through `AppLink`: the desktop renderer is
 * served from `file://`, where a plain `<a target="_blank">` resolves an in-app
 * path to `file:///{path}` — the shell's http/https allowlist then drops it and
 * the click does nothing at all. External links keep the plain anchor, because
 * handing an off-app URL to the router would be the opposite mistake.
 */
function LearnMoreLink({ href, label }: { href: string; label: ReactNode }) {
  if (href.startsWith("/")) {
    return (
      <AppLink href={href} className={LEARN_MORE_CLASS}>
        {label}
      </AppLink>
    );
  }
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={LEARN_MORE_CLASS}
    >
      {label}
    </a>
  );
}

interface CollectionPageHeaderProps {
  icon: LucideIcon;
  title: ReactNode;
  count?: number;
  description?: ReactNode;
  /**
   * Supporting "learn more" link after the description.
   *
   * A root-relative href is an in-app destination and is navigated in-app; a
   * href with a scheme opens in the browser. The distinction matters on desktop,
   * where the renderer runs at `file://` — a route path in a plain
   * `<a target="_blank">` there resolves to `file:///…` and the click does
   * nothing at all.
   */
  learnMore?: {
    href: string;
    label: ReactNode;
  };
  actions?: ReactNode;
  className?: string;
}

/**
 * Shared dashboard collection header: entity icon, title, optional count and
 * supporting copy on the left; page-level actions on the right.
 */
export function CollectionPageHeader({
  icon: Icon,
  title,
  count,
  description,
  learnMore,
  actions,
  className,
}: CollectionPageHeaderProps) {
  return (
    <PageHeader className={className}>
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <Icon
          aria-hidden="true"
          className="size-4 shrink-0 text-muted-foreground"
        />
        <h1 className="truncate text-body font-medium">{title}</h1>
        {typeof count === "number" && count > 0 ? (
          <span className="shrink-0 font-mono text-caption tabular-nums text-muted-foreground">
            {count}
          </span>
        ) : null}
        {description ? (
          <p className="ml-2 hidden min-w-0 truncate text-caption text-muted-foreground md:block">
            {description}
            {learnMore ? (
              <>
                {" "}
                <LearnMoreLink {...learnMore} />
              </>
            ) : null}
          </p>
        ) : null}
      </div>
      {actions ? (
        <div className="flex shrink-0 items-center justify-end gap-2">
          {actions}
        </div>
      ) : null}
    </PageHeader>
  );
}

interface CollectionPageHeaderActionProps
  extends Omit<ComponentProps<typeof Button>, "children"> {
  icon: LucideIcon;
  label: string;
}

/** Responsive collection action: icon-only below md, labelled above md. */
export function CollectionPageHeaderAction({
  icon: Icon,
  label,
  className,
  type = "button",
  size = "sm",
  variant = "outline",
  ...props
}: CollectionPageHeaderActionProps) {
  const accessibleLabel = props["aria-label"] ?? label;

  return (
    <Button
      type={type}
      size={size}
      variant={variant}
      className={cn("h-8 w-8 gap-1 px-0 md:w-auto md:px-2.5", className)}
      aria-label={accessibleLabel}
      {...props}
    >
      <Icon aria-hidden="true" className="size-3.5" />
      <span className="hidden md:inline">{label}</span>
    </Button>
  );
}

type PageStateTone = "muted" | "destructive" | "warning";

interface CollectionPageStateProps {
  icon: LucideIcon;
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  tone?: PageStateTone;
  role?: "alert" | "status";
  className?: string;
}

const stateToneClass: Record<PageStateTone, string> = {
  muted: "text-muted-foreground",
  destructive: "text-destructive",
  warning: "text-warning",
};

/** Shared centered state for collection empty, error and not-found views. */
export function CollectionPageState({
  icon: Icon,
  title,
  description,
  actions,
  tone = "muted",
  role,
  className,
}: CollectionPageStateProps) {
  return (
    <Empty
      role={role}
      className={cn("rounded-none border-0 px-6 py-16", className)}
    >
      <EmptyHeader>
        <EmptyMedia
          variant="icon"
          className={cn(
            "size-12 rounded-full [&_svg]:size-6",
            stateToneClass[tone],
          )}
        >
          <Icon aria-hidden="true" />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        {description ? (
          <EmptyDescription className="max-w-md">{description}</EmptyDescription>
        ) : null}
      </EmptyHeader>
      {actions ? (
        <EmptyContent className="mt-1 flex-row justify-center">
          {actions}
        </EmptyContent>
      ) : null}
    </Empty>
  );
}
