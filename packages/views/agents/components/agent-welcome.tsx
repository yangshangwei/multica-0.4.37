"use client";

import { useId, useState, type CSSProperties } from "react";
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Bot,
  Check,
  Circle,
  CircleCheck,
  GitPullRequest,
  Laptop,
  MessageSquare,
  Orbit,
  Plus,
  Puzzle,
  Search,
  UserRound,
} from "lucide-react";
import { DOCS_SLUGS } from "@multica/core/docs";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { ProviderLogo } from "../../runtimes/components/provider-logo";
import styles from "./agent-welcome.module.css";

function ResponsibilityExample() {
  const { t } = useT("agents");
  return (
    <div className={styles.prompt}>
      <p className="text-title-sm font-semibold">{t(($) => $.welcome.setup.question)}</p>
      <div className={styles.promptText}>
        {t(($) => $.welcome.setup.prompt)}
        <span className={styles.caret} aria-hidden="true" />
      </div>
      <div className={styles.chips}>
        <span><Laptop className="size-3.5" aria-hidden="true" />{t(($) => $.welcome.setup.machine)}</span>
        <span><span aria-hidden="true"><ProviderLogo provider="claude" className="size-3.5" /></span>{t(($) => $.welcome.setup.provider)}</span>
        <span>{t(($) => $.welcome.setup.model)}</span>
      </div>
    </div>
  );
}

function AssignmentExample() {
  const { t } = useT("agents");
  return (
    <div className={styles.assignment}>
      <div className={styles.issue}>
        <Circle className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <p className="min-w-0 text-body">
          <span className="mr-2 whitespace-nowrap text-muted-foreground">{t(($) => $.welcome.assignment.issue_id)}</span>
          {t(($) => $.welcome.assignment.issue_title)}
        </p>
      </div>
      <div className={styles.assigneeMenu}>
        <div className="flex items-center gap-2 border-b border-surface-border px-3 py-2.5 text-body text-muted-foreground">
          <Search className="size-3.5" aria-hidden="true" />
          {t(($) => $.welcome.assignment.search)}
        </div>
        <div className="p-1.5">
          <p className="px-2 py-1 text-caption text-muted-foreground">{t(($) => $.page.title)}</p>
          <div className="flex items-center gap-2 rounded-md bg-muted px-2 py-2 text-body font-medium">
            <Puzzle className="size-4 text-success-foreground" aria-hidden="true" />
            <span className="min-w-0 flex-1">{t(($) => $.welcome.people.agent)}</span>
            <Check className="size-3.5 text-muted-foreground" aria-hidden="true" />
          </div>
          <div className="flex items-center gap-2 px-2 py-2 text-body text-muted-foreground">
            <Orbit className="size-4" aria-hidden="true" />{t(($) => $.welcome.people.other_agent)}
          </div>
          <p className="px-2 pb-1 pt-2 text-caption text-muted-foreground">{t(($) => $.welcome.assignment.members)}</p>
          <div className="flex items-center gap-2 px-2 py-2 text-body text-muted-foreground">
            <UserRound className="size-4" aria-hidden="true" />{t(($) => $.welcome.people.member)}
          </div>
        </div>
      </div>
    </div>
  );
}

function DeliveryExample() {
  const { t } = useT("agents");
  return (
    <div className={styles.delivery}>
      <div className="flex items-center gap-2 border-b border-surface-border px-4 py-3 text-caption text-muted-foreground">
        <MessageSquare className="size-3.5" aria-hidden="true" />
        {t(($) => $.welcome.delivery.thread)}
      </div>
      <div className="flex items-start gap-2.5 p-4">
        <span className={styles.avatar} aria-hidden="true"><Puzzle className="size-4" /></span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span className="text-body font-semibold">{t(($) => $.welcome.people.agent)}</span>
            <span className="text-caption text-muted-foreground">{t(($) => $.welcome.delivery.just_now)}</span>
          </div>
          <p className="mt-2 text-body">{t(($) => $.welcome.delivery.result)}</p>
          <div className="mt-3 inline-flex items-center gap-1.5 text-caption font-medium text-info-foreground">
            <GitPullRequest className="size-3.5" aria-hidden="true" />
            {t(($) => $.welcome.delivery.review)}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-2 border-t border-surface-border px-4 py-3 text-caption text-muted-foreground">
        <CircleCheck className="size-3.5 text-success-foreground" aria-hidden="true" />
        {t(($) => $.welcome.delivery.status)}
      </div>
    </div>
  );
}

export function AgentWelcome({ runtimeConnected }: { runtimeConnected: boolean }) {
  const { t } = useT("agents");
  const paths = useWorkspacePaths();
  const headingId = useId();
  const carouselId = useId();
  const [activeSlide, setActiveSlide] = useState(0);
  const slides = [
    { key: "setup", title: t(($) => $.welcome.setup.title), description: t(($) => $.welcome.setup.description), scene: <ResponsibilityExample /> },
    { key: "assignment", title: t(($) => $.welcome.assignment.title), description: t(($) => $.welcome.assignment.description), scene: <AssignmentExample /> },
    { key: "delivery", title: t(($) => $.welcome.delivery.title), description: t(($) => $.welcome.delivery.description), scene: <DeliveryExample /> },
  ];
  const RuntimeIcon = runtimeConnected ? CircleCheck : Circle;

  return (
    <section className={styles.welcome} aria-labelledby={headingId}>
      <div className={styles.layout}>
        <div className={styles.intro}>
          <div className="mb-5 inline-flex items-center gap-1.5 rounded-full bg-brand/10 px-3 py-1.5 text-caption font-medium text-info-foreground">
            <Bot className="size-3.5" aria-hidden="true" />
            {t(($) => $.page.title)}
          </div>
          <h2 id={headingId} className="text-balance text-display font-semibold tracking-tight">{t(($) => $.welcome.title)}</h2>
          <p className="mt-4 text-pretty text-body-lg leading-7 text-muted-foreground">{t(($) => $.welcome.description)}</p>
          <div className="mt-7 flex flex-wrap items-center gap-x-4 gap-y-2">
            <AppLink className={cn(buttonVariants(), "h-11 gap-2 px-4")} href={paths.newAgent()}>
              <Plus className="size-4" aria-hidden="true" />
              {t(($) => $.page.new_agent)}
            </AppLink>
            <AppLink className={cn(buttonVariants({ variant: "ghost" }), "h-11 gap-2 px-2")} href={paths.docsPage(DOCS_SLUGS.agents)}>
              <BookOpen className="size-4" aria-hidden="true" />
              {t(($) => $.welcome.docs)}
            </AppLink>
          </div>
          <ol className={styles.checklist} aria-label={t(($) => $.welcome.steps.label)}>
            <li className={styles.step}>
              <RuntimeIcon className={cn("size-4 shrink-0", runtimeConnected ? "text-success-foreground" : "text-muted-foreground")} aria-hidden="true" />
              <span className="min-w-0 flex-1 font-medium">{t(($) => runtimeConnected ? $.welcome.steps.connected : $.welcome.steps.connect)}</span>
              <AppLink className={styles.stepLink} href={paths.runtimes()}>
                {t(($) => runtimeConnected ? $.welcome.steps.view_runtimes : $.welcome.steps.connect_action)}
              </AppLink>
            </li>
            <li className={styles.step}>
              <Circle className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              <span className={runtimeConnected ? "font-medium" : undefined}>{t(($) => $.welcome.steps.create)}</span>
            </li>
            <li className={styles.step}>
              <Circle className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              <span>{t(($) => $.welcome.steps.assign)}</span>
            </li>
          </ol>
        </div>

        <section className={styles.showcase} aria-label={t(($) => $.welcome.examples)}>
          <div className={styles.viewport}>
            <div id={carouselId} className={styles.track} style={{ "--slide": activeSlide } as CSSProperties}>
              {slides.map((slide, index) => (
                <div
                  key={slide.key}
                  role="group"
                  aria-label={slide.title}
                  aria-hidden={activeSlide !== index}
                  inert={activeSlide !== index}
                  data-active={activeSlide === index}
                  className={styles.card}
                >
                  <div className={styles.scene}>
                    <span className={styles.exampleLabel}>{t(($) => $.welcome.example)}</span>
                    {slide.scene}
                  </div>
                  <div className="px-5 py-5">
                    <h3 className="text-title-sm font-semibold">{slide.title}</h3>
                    <p className="mt-1.5 text-body text-muted-foreground">{slide.description}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className={styles.controls}>
            <div className="flex items-center">
              {slides.map((slide, index) => (
                <button
                  key={slide.key}
                  type="button"
                  aria-label={t(($) => $.welcome.show_example, { number: index + 1, title: slide.title })}
                  aria-pressed={activeSlide === index}
                  aria-controls={carouselId}
                  onClick={() => setActiveSlide(index)}
                  className="group flex size-11 items-center justify-center rounded-lg outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <span aria-hidden="true" className="h-1.5 w-1.5 rounded-full bg-input transition-[width,background-color] group-aria-pressed:w-5 group-aria-pressed:bg-foreground motion-reduce:transition-none" />
                </button>
              ))}
            </div>
            <div className="flex gap-1.5">
              <Button type="button" size="icon" variant="outline" className="size-11" aria-label={t(($) => $.welcome.previous)} aria-controls={carouselId} disabled={activeSlide === 0} onClick={() => setActiveSlide((current) => Math.max(0, current - 1))}>
                <ArrowLeft className="size-4" aria-hidden="true" />
              </Button>
              <Button type="button" size="icon" variant="outline" className="size-11" aria-label={t(($) => $.welcome.next)} aria-controls={carouselId} disabled={activeSlide === slides.length - 1} onClick={() => setActiveSlide((current) => Math.min(slides.length - 1, current + 1))}>
                <ArrowRight className="size-4" aria-hidden="true" />
              </Button>
            </div>
          </div>
          <p role="status" aria-atomic="true" className="sr-only">
            {t(($) => $.welcome.position, { number: activeSlide + 1, total: slides.length, title: slides[activeSlide]?.title })}
          </p>
        </section>
      </div>
    </section>
  );
}
