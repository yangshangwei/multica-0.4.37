"use client";

import { useId } from "react";
import { Trans } from "react-i18next";
import {
  BookOpen,
  Bot,
  ChartNoAxesColumnIncreasing,
  Check,
  Circle,
  CircleDashed,
  FolderKanban,
  GitBranch,
  Plus,
  Target,
  UsersRound,
} from "lucide-react";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import styles from "./project-welcome.module.css";

const EXAMPLE_TASKS = [
  { id: "REG-101", title: "flow", owner: "product_agent", status: "done", squad: false },
  { id: "REG-102", title: "implementation", owner: "delivery_squad", status: "in_progress", squad: true },
  { id: "REG-103", title: "validation", owner: "qa_agent", status: "todo", squad: false },
] as const;
const EXAMPLE_REPOSITORY = "account-service";
const COMPLETED_TASKS = EXAMPLE_TASKS.filter((task) => task.status === "done").length;
const COMPLETION_PERCENT = Math.round(COMPLETED_TASKS / EXAMPLE_TASKS.length * 100);

export function ProjectWelcome({ docsHref, onCreate }: {
  docsHref: string;
  onCreate: () => void;
}) {
  const { t } = useT("projects");
  const { t: tIssues } = useT("issues");
  const headingId = useId();
  const progressLabel = t(($) => $.welcome.progress_value, {
    completed: COMPLETED_TASKS,
    total: EXAMPLE_TASKS.length,
  });

  return (
    <section className={styles.welcome} aria-labelledby={headingId}>
      <div className={styles.layout}>
        <div className={styles.intro}>
          <h2 id={headingId} className="text-balance text-display-sm font-semibold tracking-tight @md:text-display">
            <Trans
              ns="projects"
              i18nKey={($) => $.welcome.title}
              components={{ term: <span className="whitespace-nowrap" /> }}
            />
          </h2>
          <p className="mt-4 max-w-[52ch] text-pretty text-body-lg leading-7 text-muted-foreground">
            {t(($) => $.welcome.description)}
          </p>
          <div className="mt-6 flex flex-wrap items-center gap-x-3 gap-y-2">
            <Button type="button" className="min-h-11 gap-2 px-4" onClick={onCreate}>
              <Plus className="size-4" aria-hidden="true" />
              {t(($) => $.page.create_first)}
            </Button>
            <AppLink
              href={docsHref}
              className={cn(buttonVariants({ variant: "ghost" }), "min-h-11 gap-2 px-3")}
            >
              <BookOpen className="size-4" aria-hidden="true" />
              {t(($) => $.welcome.docs)}
            </AppLink>
          </div>

          <ul className={styles.steps}>
            <li className={styles.step}>
              <Target className={styles.stepIcon} aria-hidden="true" />
              <div>
                <h3 className="text-body font-medium">{t(($) => $.welcome.steps.goal)}</h3>
                <p className="mt-1 text-caption leading-5 text-muted-foreground">{t(($) => $.welcome.steps.goal_description)}</p>
              </div>
            </li>
            <li className={styles.step}>
              <UsersRound className={styles.stepIcon} aria-hidden="true" />
              <div>
                <h3 className="text-body font-medium">{t(($) => $.welcome.steps.assign)}</h3>
                <p className="mt-1 text-caption leading-5 text-muted-foreground">{t(($) => $.welcome.steps.assign_description)}</p>
              </div>
            </li>
            <li className={styles.step}>
              <ChartNoAxesColumnIncreasing className={styles.stepIcon} aria-hidden="true" />
              <div>
                <h3 className="text-body font-medium">{t(($) => $.welcome.steps.track)}</h3>
                <p className="mt-1 text-caption leading-5 text-muted-foreground">{t(($) => $.welcome.steps.track_description)}</p>
              </div>
            </li>
          </ul>
        </div>

        <figure className={styles.example} aria-label={t(($) => $.welcome.example_label)}>
          <div className={styles.exampleHeader}>
            <div className="flex min-w-0 items-start gap-2.5">
              <FolderKanban className="mt-0.5 size-5 shrink-0 text-info-foreground" aria-hidden="true" />
              <div className="min-w-0">
                <h3 className="text-title-sm font-semibold [overflow-wrap:anywhere]">{t(($) => $.welcome.example_title)}</h3>
                <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.status.in_progress)}</p>
              </div>
            </div>
            <span className={styles.exampleLabel}>{t(($) => $.welcome.example_label)}</span>
          </div>

          <div className={styles.projectContext}>
            <p className="text-body leading-6">{t(($) => $.welcome.example_goal)}</p>
            <p className="mt-3 flex min-w-0 items-center gap-1.5 text-caption text-muted-foreground">
              <GitBranch className="size-3.5 shrink-0" aria-hidden="true" />
              <span className="min-w-0 [overflow-wrap:anywhere]">{EXAMPLE_REPOSITORY}</span>
            </p>
          </div>

          <div className={styles.progressSummary}>
            <div className="flex flex-wrap items-center justify-between gap-2 text-caption">
              <span className="font-medium">{t(($) => $.welcome.progress)}</span>
              <span className="tabular-nums text-muted-foreground">{progressLabel}</span>
            </div>
            <div
              className={styles.progressTrack}
              role="progressbar"
              aria-label={t(($) => $.welcome.progress)}
              aria-valuenow={COMPLETION_PERCENT}
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuetext={progressLabel}
            >
              <span className={styles.progressBar} style={{ width: `${COMPLETION_PERCENT}%` }} />
            </div>
          </div>

          <ul className={styles.taskList} aria-label={t(($) => $.table.issues)}>
            {EXAMPLE_TASKS.map((task) => {
              const StatusIcon = task.status === "done" ? Check : task.status === "in_progress" ? CircleDashed : Circle;
              const OwnerIcon = task.squad ? UsersRound : Bot;
              const statusTone = task.status === "done" ? "text-success-foreground" : task.status === "in_progress" ? "text-info-foreground" : "text-muted-foreground";
              return (
                <li key={task.id} className={styles.task}>
                  <StatusIcon className={cn("mt-0.5 size-4 shrink-0", statusTone)} aria-hidden="true" />
                  <div className="min-w-0">
                    <p className="text-body font-medium [overflow-wrap:anywhere]">{t(($) => $.welcome.tasks[task.title])}</p>
                    <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
                      <span className="tabular-nums">{task.id}</span>
                      <span className="inline-flex items-center gap-1">
                        <OwnerIcon className="size-3.5 shrink-0" aria-hidden="true" />
                        {t(($) => $.welcome.assignees[task.owner])}
                      </span>
                    </p>
                  </div>
                  <span className={cn("shrink-0 text-caption", statusTone)}>
                    {tIssues(($) => $.status[task.status])}
                  </span>
                </li>
              );
            })}
          </ul>
          <figcaption className={styles.caption}>{t(($) => $.welcome.example_caption)}</figcaption>
        </figure>
      </div>
    </section>
  );
}
