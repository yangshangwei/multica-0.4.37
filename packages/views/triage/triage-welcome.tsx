"use client";

import { useId } from "react";
import { ArrowDown, BookOpen, Circle, CircleDashed, Inbox, Plus, Upload } from "lucide-react";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AppLink } from "../navigation";

const EXAMPLE_TASK_ID = "TRI-104";

function ExampleTaskRow({ title, status, pending = false }: { title: string; status: string; pending?: boolean }) {
  const Icon = pending ? CircleDashed : Circle;
  return (
    <div className="flex min-w-0 items-start gap-2.5 rounded-lg border border-surface-border bg-surface px-3 py-3">
      <Icon className={cn("mt-0.5 size-4 shrink-0", pending ? "text-info-foreground" : "text-muted-foreground")} aria-hidden="true" />
      <div className="min-w-0">
        <p className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-body font-medium">
          <span className="text-caption font-normal text-muted-foreground">{EXAMPLE_TASK_ID}</span>
          <span>{title}</span>
        </p>
        <p className={cn("mt-1 text-caption", pending ? "font-medium text-info-foreground" : "text-muted-foreground")}>
          {status}
        </p>
      </div>
    </div>
  );
}

export function TriageWelcome({ docsHref, onCreate, onImport }: {
  docsHref: string;
  onCreate: () => void;
  onImport: () => void;
}) {
  const { t } = useT("triage");
  const headingId = useId();
  const taskTitle = t(($) => $.welcome.example_task);

  return (
    <section className="@container h-full overflow-y-auto" aria-labelledby={headingId}>
      <div className="mx-auto grid min-h-full max-w-[76rem] content-center gap-8 px-5 py-8 @sm:px-8 @4xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] @4xl:items-center @4xl:gap-12 @4xl:py-14 @6xl:gap-16 @6xl:px-12">
        <div className="min-w-0">
          <h2 id={headingId} className="max-w-[22ch] text-balance text-display-sm font-semibold tracking-tight @sm:text-display">
            {t(($) => $.welcome.title)}
          </h2>
          <p className="mt-4 max-w-[48ch] text-pretty text-body-lg leading-relaxed text-muted-foreground">
            {t(($) => $.welcome.description)}
          </p>
          <div className="mt-6 flex flex-wrap items-center gap-x-2 gap-y-2.5">
            <Button type="button" className="h-11 gap-2 px-4" onClick={onCreate}>
              <Plus className="size-4" aria-hidden="true" />
              {t(($) => $.create)}
            </Button>
            <Button type="button" variant="outline" className="h-11 gap-2 px-3" onClick={onImport}>
              <Upload className="size-4" aria-hidden="true" />
              {t(($) => $.import_csv)}
            </Button>
            <AppLink href={docsHref} className={cn(buttonVariants({ variant: "ghost" }), "h-11 gap-2 px-3")}>
              <BookOpen className="size-4" aria-hidden="true" />
              {t(($) => $.welcome.docs)}
            </AppLink>
          </div>
        </div>
        <figure className="min-w-0 overflow-hidden rounded-xl border border-surface-border bg-surface">
          <div className="flex items-center justify-between gap-2 px-4 py-3 @sm:px-5">
            <h3 className="text-title-sm font-semibold">{t(($) => $.welcome.example_title)}</h3>
            <span className="rounded-md bg-info/10 px-2 py-1 text-caption font-medium text-info-foreground">
              {t(($) => $.welcome.example)}
            </span>
          </div>
          <ol className="border-y border-surface-border bg-info/5 px-4 py-4 @sm:px-5 @sm:py-5">
            <li>
              <p className="mb-2 flex items-center gap-2 text-caption font-medium text-muted-foreground">
                <Inbox className="size-4 shrink-0" aria-hidden="true" />
                {t(($) => $.welcome.example_submission)}
              </p>
              <ExampleTaskRow title={taskTitle} status={t(($) => $.pending)} pending />
            </li>
            <li>
              <p className="flex items-center gap-2.5 py-2 pl-3 text-caption font-medium text-muted-foreground">
                <ArrowDown className="size-4 shrink-0 text-info-foreground" aria-hidden="true" />
                {t(($) => $.accept)}
              </p>
              <ExampleTaskRow title={taskTitle} status={t(($) => $.welcome.example_list)} />
            </li>
          </ol>
          <figcaption className="px-4 py-3 text-caption text-muted-foreground @sm:px-5">
            {t(($) => $.welcome.example_caption)}
          </figcaption>
        </figure>
      </div>
    </section>
  );
}
