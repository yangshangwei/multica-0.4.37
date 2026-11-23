"use client";

import { useId } from "react";
import { CalendarRange, Check, Circle, Flag, ListChecks, Plus, Settings2 } from "lucide-react";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import styles from "./iteration-welcome.module.css";

export function IterationWelcome({ onCreate, settingsHref }: { onCreate: () => void; settingsHref: string }) {
  const { t } = useT("projects");
  const headingId = useId();

  return (
    <section className={styles.welcome} aria-labelledby={headingId}>
      <div className={styles.layout}>
        <div className={styles.intro}>
          <h2 id={headingId} className="max-w-[18ch] text-balance text-display font-semibold tracking-tight">
            {t(($) => $.iterations.welcome.title)}
          </h2>
          <p className="mt-4 max-w-[48ch] text-pretty text-body-lg leading-7 text-muted-foreground">
            {t(($) => $.iterations.welcome.description)}
          </p>
          <div className="mt-7 flex flex-wrap items-center gap-x-3 gap-y-2">
            <Button type="button" className="min-h-11 gap-2 px-4" onClick={onCreate}>
              <Plus className="size-4" aria-hidden="true" />
              {t(($) => $.iterations.welcome.create)}
            </Button>
            <AppLink
              href={settingsHref}
              className={cn(buttonVariants({ variant: "ghost" }), "min-h-11 w-fit gap-2 px-3")}
            >
              <Settings2 className="size-4" aria-hidden="true" />
              {t(($) => $.iterations.settings)}
            </AppLink>
          </div>

          <ol className={styles.steps} aria-label={t(($) => $.iterations.welcome.steps.label)}>
            <li className={styles.step}>
              <span className={styles.stepIcon} aria-hidden="true"><CalendarRange className="size-4" /></span>
              <span>{t(($) => $.iterations.welcome.steps.plan)}</span>
            </li>
            <li className={styles.step}>
              <span className={styles.stepIcon} aria-hidden="true"><ListChecks className="size-4" /></span>
              <span>{t(($) => $.iterations.welcome.steps.track)}</span>
            </li>
            <li className={styles.step}>
              <span className={styles.stepIcon} aria-hidden="true"><Flag className="size-4" /></span>
              <span>{t(($) => $.iterations.welcome.steps.handoff)}</span>
            </li>
          </ol>
        </div>

        <section className={styles.showcase} aria-label={t(($) => $.iterations.welcome.example)}>
          <div className={styles.exampleLabel}>{t(($) => $.iterations.welcome.example)}</div>
          <div className={styles.iterationCard}>
            <div className={styles.cardHeader}>
              <div className="min-w-0">
                <p className="text-title-sm font-semibold [overflow-wrap:anywhere]">{t(($) => $.iterations.welcome.example_name)}</p>
                <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.iterations.welcome.example_dates)}</p>
              </div>
              <span className={styles.status}><Circle className="size-2.5 fill-current" aria-hidden="true" />{t(($) => $.iterations.active)}</span>
            </div>

            <div className={styles.progressSummary}>
              <div className="flex items-center justify-between gap-3">
                <span className="text-caption font-medium">{t(($) => $.iterations.welcome.example_progress)}</span>
                <span className="text-caption tabular-nums text-muted-foreground">50%</span>
              </div>
              <div className={styles.progressTrack} aria-hidden="true"><span className={styles.progressBar} /></div>
            </div>

            <ul className={styles.taskList} aria-label={t(($) => $.iterations.issues)}>
              <li className={styles.task}>
                <span className={styles.taskIconDone} aria-hidden="true"><Check className="size-3" /></span>
                <span className="min-w-0 flex-1 truncate">{t(($) => $.iterations.welcome.tasks.first)}</span>
                <span className="text-caption text-success-foreground">{t(($) => $.iterations.welcome.done)}</span>
              </li>
              <li className={styles.task}>
                <span className={styles.taskIconDone} aria-hidden="true"><Check className="size-3" /></span>
                <span className="min-w-0 flex-1 truncate">{t(($) => $.iterations.welcome.tasks.second)}</span>
                <span className="text-caption text-success-foreground">{t(($) => $.iterations.welcome.done)}</span>
              </li>
              <li className={styles.task}>
                <span className={styles.taskIcon} aria-hidden="true"><Circle className="size-3" /></span>
                <span className="min-w-0 flex-1 truncate">{t(($) => $.iterations.welcome.tasks.third)}</span>
                <span className="text-caption text-info-foreground">{t(($) => $.iterations.welcome.in_progress)}</span>
              </li>
              <li className={styles.task}>
                <span className={styles.taskIcon} aria-hidden="true"><Circle className="size-3" /></span>
                <span className="min-w-0 flex-1 truncate">{t(($) => $.iterations.welcome.tasks.fourth)}</span>
                <span className="text-caption text-muted-foreground">{t(($) => $.iterations.welcome.planned)}</span>
              </li>
            </ul>

            <p className={styles.handoff}><Flag className="size-3.5 shrink-0 text-warning-foreground" aria-hidden="true" />{t(($) => $.iterations.welcome.handoff)}</p>
          </div>
        </section>
      </div>
    </section>
  );
}
