"use client";

import { useId, useState, type CSSProperties } from "react";
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Check,
  Circle,
  Code2,
  Crown,
  Flower2,
  Layers2,
  MessageSquare,
  Plus,
  ScanSearch,
  UserRound,
  type LucideIcon,
} from "lucide-react";
import { DOCS_SLUGS } from "@multica/core/docs";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import styles from "./squad-welcome.module.css";

function ExampleAvatar({ icon: Icon, tone }: { icon: LucideIcon; tone: "leader" | "builder" | "reviewer" | "human" }) {
  return (
    <span className={cn(styles.avatar, styles[tone])} aria-hidden="true">
      <Icon className="size-4" />
    </span>
  );
}

function LeaderBadge() {
  const { t } = useT("squads");
  return (
    <span className="inline-flex items-center gap-1 rounded-md bg-warning/10 px-1.5 py-0.5 text-caption font-medium text-warning-foreground">
      <Crown className="size-3" aria-hidden="true" />
      {t(($) => $.members_tab.leader_chip)}
    </span>
  );
}

function DelegationExample() {
  const { t } = useT("squads");
  return (
    <div className={styles.delegation}>
      <div className={styles.issue}>
        <div className="flex items-start gap-2">
          <Circle className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <div className="min-w-0 flex-1">
            <p className="text-body font-medium">
              <span className="mr-2 whitespace-nowrap text-caption font-normal text-muted-foreground">{t(($) => $.welcome.issue_id)}</span>
              {t(($) => $.welcome.issue_title)}
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-caption">
              <span className="text-muted-foreground">{t(($) => $.welcome.assignee)}</span>
              <span className="inline-flex items-center gap-1.5 rounded-full bg-muted px-2 py-1 font-medium">
                <Layers2 className="size-3 text-muted-foreground" aria-hidden="true" />
                {t(($) => $.welcome.squad_name)}
              </span>
            </div>
          </div>
        </div>
      </div>
      <div className={styles.connector} aria-hidden="true"><ArrowDown className="size-4" /></div>
      <div className={styles.leaderNode}>
        <ExampleAvatar icon={Flower2} tone="leader" />
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-body font-medium">{t(($) => $.welcome.people.leader)}</span>
            <LeaderBadge />
          </div>
          <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.welcome.delegation.routing)}</p>
        </div>
      </div>
      <div className={styles.branch} aria-hidden="true">
        <span /><ArrowDown className="size-4" /><ArrowDown className="size-4" />
      </div>
      <div className="grid grid-cols-2 gap-2.5">
        <div className={cn(styles.memberNode, styles.working)}>
          <ExampleAvatar icon={Code2} tone="builder" />
          <div className="min-w-0">
            <p className="text-body font-medium">{t(($) => $.welcome.people.builder)}</p>
            <p className="mt-1 text-caption font-medium text-info-foreground">{t(($) => $.members_tab.status_working)}</p>
          </div>
        </div>
        <div className={styles.memberNode}>
          <ExampleAvatar icon={ScanSearch} tone="reviewer" />
          <div className="min-w-0">
            <p className="text-body font-medium">{t(($) => $.welcome.people.reviewer)}</p>
            <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.members_tab.status_idle)}</p>
          </div>
        </div>
      </div>
    </div>
  );
}

function RolesExample() {
  const { t } = useT("squads");
  const members = [
    { tone: "leader", icon: Flower2, name: t(($) => $.welcome.people.leader), role: t(($) => $.welcome.roles.leader) },
    { tone: "builder", icon: Code2, name: t(($) => $.welcome.people.builder), role: t(($) => $.welcome.roles.builder) },
    { tone: "reviewer", icon: ScanSearch, name: t(($) => $.welcome.people.reviewer), role: t(($) => $.welcome.roles.reviewer) },
    { tone: "human", icon: UserRound, name: t(($) => $.welcome.people.human), role: t(($) => $.welcome.roles.human) },
  ] as const;
  return (
    <div className={styles.roster}>
      <div className="flex items-center justify-between gap-3 border-b border-surface-border px-4 py-3">
        <span className="text-caption font-medium text-muted-foreground">{t(($) => $.members_tab.section_title)}</span>
        <span className="text-caption tabular-nums text-muted-foreground">{members.length}</span>
      </div>
      <ul className="px-1.5 py-1">
        {members.map((member) => (
          <li key={member.tone} className="flex items-center gap-2.5 rounded-lg px-2.5 py-2.5">
            <ExampleAvatar icon={member.icon} tone={member.tone} />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-body font-medium">{member.name}</span>
                {member.tone === "leader" && <LeaderBadge />}
              </div>
              <p className="mt-0.5 text-caption text-muted-foreground">{member.role}</p>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

function HandoffExample() {
  const { t } = useT("squads");
  return (
    <div className={styles.thread}>
      <div className="flex items-center gap-2 border-b border-surface-border px-4 py-3 text-caption text-muted-foreground">
        <MessageSquare className="size-3.5" aria-hidden="true" />
        <span>{t(($) => $.welcome.handoff.thread)}</span>
      </div>
      <div className="space-y-5 p-4">
        <div className="flex items-start gap-2.5">
          <ExampleAvatar icon={Code2} tone="builder" />
          <div className="min-w-0">
            <p className="text-body font-medium">{t(($) => $.welcome.people.builder)}</p>
            <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.welcome.handoff.result)}</p>
          </div>
        </div>
        <div className="flex items-start gap-2.5">
          <ExampleAvatar icon={Flower2} tone="leader" />
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-body font-medium">{t(($) => $.welcome.people.leader)}</span>
              <LeaderBadge />
            </div>
            <p className="mt-1 text-caption text-info-foreground">{t(($) => $.welcome.handoff.next_step)}</p>
          </div>
        </div>
      </div>
      <div className="flex items-start gap-2 border-t border-surface-border px-4 py-3 text-caption text-muted-foreground">
        <Check className="mt-0.5 size-3.5 shrink-0 text-success-foreground" aria-hidden="true" />
        <span>{t(($) => $.welcome.handoff.context)}</span>
      </div>
    </div>
  );
}

export function SquadWelcome() {
  const { t } = useT("squads");
  const paths = useWorkspacePaths();
  const headingId = useId();
  const carouselId = useId();
  const [activeSlide, setActiveSlide] = useState(0);
  const slides = [
    { title: t(($) => $.welcome.delegation.title), description: t(($) => $.welcome.delegation.description), scene: <DelegationExample /> },
    { title: t(($) => $.welcome.roles.title), description: t(($) => $.welcome.roles.description), scene: <RolesExample /> },
    { title: t(($) => $.welcome.handoff.title), description: t(($) => $.welcome.handoff.description), scene: <HandoffExample /> },
  ] as const;
  const currentSlide = slides[activeSlide] ?? slides[0];

  return (
    <section className={styles.welcome} aria-labelledby={headingId}>
      <div className={styles.layout}>
        <div className={styles.intro}>
          <div className="mb-5 inline-flex items-center gap-1.5 rounded-full bg-brand/10 px-3 py-1.5 text-caption font-medium text-info-foreground">
            <Layers2 className="size-3.5" aria-hidden="true" />
            {t(($) => $.welcome.badge)}
          </div>
          <h2 id={headingId} className="text-balance text-display font-semibold tracking-tight">
            <span className="block">{t(($) => $.welcome.title_start)}</span>{" "}
            <span className="block">{t(($) => $.welcome.title_end)}</span>
          </h2>
          <p className="mt-5 max-w-lg text-pretty text-body-lg leading-7 text-muted-foreground">
            {t(($) => $.welcome.description)}
          </p>
          <div className="mt-7 flex flex-wrap items-center gap-x-4 gap-y-2">
            <Button className="h-11 gap-2 px-4" render={<AppLink href={paths.newSquad()} />}>
              <Plus className="size-4" aria-hidden="true" />
              {t(($) => $.page.new_button)}
            </Button>
            <Button variant="ghost" className="h-11 gap-2 px-2" render={<AppLink href={paths.docsPage(DOCS_SLUGS.squads)} />}>
              <BookOpen className="size-4" aria-hidden="true" />
              {t(($) => $.welcome.docs)}
            </Button>
          </div>
        </div>

        <section className={styles.showcase} aria-label={t(($) => $.welcome.examples)}>
          <div className={styles.viewport}>
            <div id={carouselId} className={styles.track} style={{ "--slide": activeSlide } as CSSProperties}>
              {slides.map((slide, index) => (
                <div
                  key={index}
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
                  key={index}
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
            {t(($) => $.welcome.position, { number: activeSlide + 1, total: slides.length, title: currentSlide.title })}
          </p>
        </section>
      </div>
    </section>
  );
}
