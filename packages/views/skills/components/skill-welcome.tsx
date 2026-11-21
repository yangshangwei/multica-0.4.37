"use client";

import { useId, useState, type CSSProperties, type Ref } from "react";
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Check,
  Download,
  FileText,
  FolderOpen,
  HardDrive,
  LayoutTemplate,
  Orbit,
  Plus,
  Puzzle,
} from "lucide-react";
import { DOCS_SLUGS } from "@multica/core/docs";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { SkillIcon } from "../lib/skill-icon";
import styles from "./skill-welcome.module.css";

function AddingExample() {
  const { t } = useT("skills");
  const methods = [
    { key: "manual", icon: Plus },
    { key: "template", icon: LayoutTemplate },
    { key: "local", icon: FolderOpen },
    { key: "url", icon: Download },
    { key: "runtime", icon: HardDrive },
  ] as const;

  return (
    <div className={styles.methodMenu}>
      {methods.map(({ key, icon: Icon }) => (
        <div key={key} className={styles.method} data-highlighted={key === "url"}>
          <span className={styles.methodIcon} aria-hidden="true"><Icon className="size-4" /></span>
          <div className="min-w-0 flex-1">
            <p className="text-body font-medium">{t(($) => $.create.method[key].title)}</p>
            <p className="mt-0.5 text-caption text-muted-foreground">{t(($) => $.create.method[key].desc)}</p>
          </div>
        </div>
      ))}
    </div>
  );
}

function AssignmentExample() {
  const { t } = useT("skills");
  return (
    <div className={styles.assignment}>
      <div className={styles.skillSummary}>
        <div className="flex min-w-0 items-center gap-2">
          <SkillIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <span className="min-w-0 font-mono text-body font-medium">{t(($) => $.welcome.assignment.skill_name)}</span>
        </div>
        <p className="mt-2 text-body text-muted-foreground">{t(($) => $.welcome.assignment.instruction)}</p>
      </div>
      <div className={styles.connector} aria-hidden="true"><ArrowDown className="size-4" /></div>
      <div className={styles.agentList}>
        <p className="mb-2 text-caption text-muted-foreground">{t(($) => $.welcome.assignment.used_by)}</p>
        <div className={styles.agent}>
          <span className={styles.agentAvatar} aria-hidden="true"><Orbit className="size-4" /></span>
          <span className="min-w-0 flex-1 text-body font-medium">{t(($) => $.welcome.assignment.agent_name)}</span>
          <span className={styles.role}>{t(($) => $.welcome.assignment.reviewer)}</span>
        </div>
        <div className={styles.agent}>
          <span className={styles.agentAvatar} aria-hidden="true"><Puzzle className="size-4" /></span>
          <span className="min-w-0 flex-1 text-body font-medium">{t(($) => $.welcome.assignment.other_agent_name)}</span>
          <span className={styles.role}>{t(($) => $.welcome.assignment.engineer)}</span>
        </div>
        <div className="mt-2 flex items-center gap-2 text-caption text-muted-foreground">
          <Plus className="size-3.5" aria-hidden="true" />{t(($) => $.welcome.assignment.add)}
        </div>
      </div>
    </div>
  );
}

function ReuseExample() {
  const { t } = useT("skills");
  return (
    <div className={styles.reuse}>
      <div className={styles.file}>
        <div className={styles.fileHeader}>
          <FileText className="size-4 text-muted-foreground" aria-hidden="true" />
          <span className="font-mono text-caption font-medium">{t(($) => $.welcome.reuse.filename)}</span>
          <span className={styles.saved}><Check className="size-3" aria-hidden="true" />{t(($) => $.welcome.reuse.saved)}</span>
        </div>
        <div className={styles.fileContent}>
          <p className="font-semibold">## {t(($) => $.welcome.reuse.checklist)}</p>
          <p>- {t(($) => $.welcome.reuse.tests)}</p>
          <p>- {t(($) => $.welcome.reuse.accessibility)}</p>
          <p className={styles.addedLine}>+ {t(($) => $.welcome.reuse.performance)}</p>
        </div>
      </div>
      <div className={styles.nextRun}>
        <Check className="size-4 shrink-0 text-success-foreground" aria-hidden="true" />
        <span>{t(($) => $.welcome.reuse.next_run)}</span>
      </div>
      <p className="mt-3 text-center text-caption text-muted-foreground">{t(($) => $.welcome.reuse.snapshot)}</p>
    </div>
  );
}

export function SkillWelcome({ onCreate, buttonRef }: { onCreate: () => void; buttonRef: Ref<HTMLButtonElement> }) {
  const { t } = useT("skills");
  const paths = useWorkspacePaths();
  const headingId = useId();
  const carouselId = useId();
  const [activeSlide, setActiveSlide] = useState(0);
  const slides = [
    { key: "add", title: t(($) => $.welcome.add.title), description: t(($) => $.welcome.add.description), scene: <AddingExample /> },
    { key: "assignment", title: t(($) => $.welcome.assignment.title), description: t(($) => $.welcome.assignment.description), scene: <AssignmentExample /> },
    { key: "reuse", title: t(($) => $.welcome.reuse.title), description: t(($) => $.welcome.reuse.description), scene: <ReuseExample /> },
  ];

  return (
    <section className={styles.welcome} aria-labelledby={headingId}>
      <div className={styles.layout}>
        <div className={styles.intro}>
          <div className="mb-5 inline-flex items-center gap-1.5 rounded-full bg-brand/10 px-3 py-1.5 text-caption font-medium text-info-foreground">
            <SkillIcon className="size-3.5" aria-hidden="true" />
            {t(($) => $.page.title)}
          </div>
          <h2 id={headingId} className="text-balance text-display font-semibold tracking-tight">{t(($) => $.welcome.title)}</h2>
          <p className="mt-4 text-pretty text-body-lg leading-7 text-muted-foreground">{t(($) => $.welcome.description)}</p>
          <div className="mt-7 flex flex-wrap items-center gap-x-4 gap-y-2">
            <Button ref={buttonRef} type="button" onClick={onCreate} className="min-h-11 gap-2 px-4">
              <Plus className="size-4" aria-hidden="true" />
              {t(($) => $.page.new_skill)}
            </Button>
            <AppLink className={cn(buttonVariants({ variant: "ghost" }), "min-h-11 gap-2 px-2")} href={paths.docsPage(DOCS_SLUGS.skills)}>
              <BookOpen className="size-4" aria-hidden="true" />
              {t(($) => $.welcome.docs)}
            </AppLink>
          </div>
        </div>

        <section className={styles.showcase} aria-label={t(($) => $.welcome.examples)}>
          <div className={styles.viewport}>
            <div id={carouselId} className={styles.track} style={{ "--slide": activeSlide } as CSSProperties}>
              {slides.map((slide, index) => (
                <div key={slide.key} role="group" aria-label={slide.title} aria-hidden={activeSlide !== index} inert={activeSlide !== index} data-active={activeSlide === index} className={styles.card}>
                  <div className={styles.scene}>
                    <span className={styles.exampleLabel}>{t(($) => $.welcome.example)}</span>
                    {slide.scene}
                  </div>
                  <div className={styles.caption}>
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
                <button key={slide.key} type="button" aria-label={t(($) => $.welcome.show_example, { number: index + 1, title: slide.title })} aria-pressed={activeSlide === index} aria-controls={carouselId} onClick={() => setActiveSlide(index)} className="group flex size-11 items-center justify-center rounded-lg outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring">
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
          <p role="status" aria-live="polite" aria-atomic="true" className="sr-only">
            {t(($) => $.welcome.position, { number: activeSlide + 1, total: slides.length, title: slides[activeSlide]?.title })}
          </p>
        </section>
      </div>
    </section>
  );
}
