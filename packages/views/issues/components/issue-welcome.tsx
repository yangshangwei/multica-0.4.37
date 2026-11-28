"use client";

import { useId, type ReactNode } from "react";
import { BookOpen, Bot, Check, Circle, CircleDashed, ListChecks, MessageSquare, Target, UserRound, UsersRound } from "lucide-react";
import { buttonVariants } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import styles from "./issue-welcome.module.css";

const STEPS = [
  { key: "describe", icon: Target },
  { key: "assign", icon: UsersRound },
  { key: "follow", icon: MessageSquare },
] as const;

const EXAMPLE_STEPS = [
  { key: "plan", icon: Check, status: "done", tone: "text-success-foreground" },
  { key: "build", icon: CircleDashed, status: "in_progress", tone: "text-info-foreground" },
  { key: "verify", icon: Circle, status: "todo", tone: "text-muted-foreground" },
] as const;

export function IssueWelcome({ createAction, secondaryAction, docsHref }: {
  createAction: ReactNode;
  secondaryAction?: ReactNode;
  docsHref: string;
}) {
  const { t } = useT("issues");
  const headingId = useId();

  return (
    <section className={styles.welcome} aria-labelledby={headingId}>
      <div className={styles.layout}>
        <div className={styles.intro}>
          <h2 id={headingId} className="text-balance text-display-sm font-semibold tracking-tight @md:text-display">
            {t(($) => $.welcome.title)}
          </h2>
          <p className="mt-4 max-w-[48ch] text-pretty text-body-lg leading-7 text-muted-foreground">
            {t(($) => $.welcome.description)}
          </p>
          <div className="mt-6 flex flex-wrap items-center gap-x-3 gap-y-2">
            {createAction}
            <AppLink href={docsHref} className={cn(buttonVariants({ variant: "ghost" }), "min-h-11 gap-2 px-3")}>
              <BookOpen className="size-4" aria-hidden="true" />
              {t(($) => $.welcome.docs)}
            </AppLink>
          </div>
          {secondaryAction && <div className="mt-2 flex flex-wrap items-center">{secondaryAction}</div>}
          <ul className={styles.steps}>
            {STEPS.map(({ key, icon: Icon }) => (
              <li key={key} className={styles.step}>
                <Icon className={styles.stepIcon} aria-hidden="true" />
                <div className="min-w-0">
                  <h3 className="text-body font-medium">{t(($) => $.welcome.steps[key].title)}</h3>
                  <p className="mt-1 text-caption leading-5 text-muted-foreground">{t(($) => $.welcome.steps[key].description)}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>

        <figure className={styles.example} aria-label={t(($) => $.welcome.example_label)}>
          <div className={styles.exampleHeader}>
            <span className="inline-flex items-center gap-1.5 text-caption font-medium text-info-foreground">
              <CircleDashed className="size-3.5" aria-hidden="true" />
              {t(($) => $.status.in_progress)}
            </span>
            <span className={styles.exampleLabel}>{t(($) => $.welcome.example_label)}</span>
          </div>
          <div className={styles.exampleContent}>
            <h3 className="text-title font-semibold [overflow-wrap:anywhere]">{t(($) => $.welcome.example_title)}</h3>
            <p className="mt-3 text-body leading-6 text-muted-foreground">{t(($) => $.welcome.example_description)}</p>
            <dl className={styles.people}>
              <div>
                <dt className="text-caption text-muted-foreground">{t(($) => $.welcome.creator)}</dt>
                <dd className="mt-2 flex items-center gap-1.5 text-body">
                  <UserRound className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                  {t(($) => $.welcome.example_creator)}
                </dd>
              </div>
              <div>
                <dt className="text-caption text-muted-foreground">{t(($) => $.welcome.assignee)}</dt>
                <dd className="mt-2 flex items-center gap-1.5 text-body">
                  <Bot className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                  {t(($) => $.welcome.example_assignee)}
                </dd>
              </div>
            </dl>
          </div>
          <div className={styles.progress}>
            <p className="flex items-center gap-2 text-caption font-medium">
              <ListChecks className="size-4 text-muted-foreground" aria-hidden="true" />
              {t(($) => $.welcome.progress)}
            </p>
            <ul className={styles.checklist}>
              {EXAMPLE_STEPS.map(({ key, icon: Icon, status, tone }) => (
                <li key={key} className={styles.checklistItem}>
                  <Icon className={cn("mt-0.5 size-4 shrink-0", tone)} aria-hidden="true" />
                  <span className="min-w-0 text-body [overflow-wrap:anywhere]">{t(($) => $.welcome.example_steps[key])}</span>
                  <span className={cn("text-caption", tone)}>{t(($) => $.status[status])}</span>
                </li>
              ))}
            </ul>
          </div>
          <figcaption className={styles.caption}>{t(($) => $.welcome.example_caption)}</figcaption>
        </figure>
      </div>
    </section>
  );
}
