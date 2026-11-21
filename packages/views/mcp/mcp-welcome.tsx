"use client";

import { useId, useState, type CSSProperties, type Ref } from "react";
import {
  ArrowDown, ArrowLeft, ArrowRight, Check, Compass, Orbit, Plus,
  Puzzle, Server, SquareTerminal,
} from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { McpTemplateIcon } from "../common/mcp-template-icon";
import { useT } from "../i18n";
import styles from "./mcp-welcome.module.css";

function ServiceExample() {
  const { t } = useT("settings");
  const services = [
    { key: "playwright", name: t(($) => $.mcp.welcome.browser_name), description: t(($) => $.mcp.welcome.browser_description) },
    { key: "postgres-mcp", name: t(($) => $.mcp.welcome.database_name), description: t(($) => $.mcp.welcome.database_description) },
    { key: "markitdown", name: t(($) => $.mcp.welcome.documents_name), description: t(($) => $.mcp.welcome.documents_description) },
  ];

  return (
    <div className={styles.serviceList}>
      <div className={styles.previewHeader}>
        <Compass className="size-4" aria-hidden="true" />
        <span>{t(($) => $.mcp.market.title)}</span>
      </div>
      {services.map((service, index) => (
        <div key={service.key} className={styles.service} data-selected={index === 0}>
          <span aria-hidden="true"><McpTemplateIcon templateKey={service.key} /></span>
          <div className="min-w-0 flex-1">
            <p className="text-body font-medium">{service.name}</p>
            <p className="mt-0.5 text-caption text-muted-foreground">{service.description}</p>
          </div>
          {index === 0 ? <Check className="size-4 shrink-0 text-info-foreground" aria-hidden="true" /> : null}
        </div>
      ))}
    </div>
  );
}

function ConfigurationExample() {
  const { t } = useT("settings");
  return (
    <div className={styles.configuration}>
      <div className={styles.previewHeader}>
        <SquareTerminal className="size-4" aria-hidden="true" />
        <span>{t(($) => $.mcp.welcome.configuration_label)}</span>
      </div>
      <dl className={styles.fields}>
        <div>
          <dt>{t(($) => $.mcp.welcome.name_label)}</dt>
          <dd>{t(($) => $.mcp.welcome.server_name)}</dd>
        </div>
        <div>
          <dt>{t(($) => $.mcp.welcome.command_label)}</dt>
          <dd className="font-mono">{t(($) => $.mcp.welcome.command_value)}</dd>
        </div>
        <div>
          <dt>{t(($) => $.mcp.welcome.arguments_label)}</dt>
          <dd className="font-mono">{t(($) => $.mcp.welcome.arguments_value)}</dd>
        </div>
      </dl>
      <p className={styles.configurationNote}>{t(($) => $.mcp.welcome.configuration_note)}</p>
    </div>
  );
}

function AssignmentExample() {
  const { t } = useT("settings");
  return (
    <div className={styles.assignment}>
      <div className={styles.savedServer}>
        <span aria-hidden="true"><McpTemplateIcon templateKey="playwright" /></span>
        <div className="min-w-0">
          <p className="text-body font-medium">{t(($) => $.mcp.welcome.server_name)}</p>
          <p className="mt-0.5 text-caption text-muted-foreground">{t(($) => $.mcp.welcome.workspace_configuration)}</p>
        </div>
      </div>
      <div className={styles.connector} aria-hidden="true"><ArrowDown className="size-4" /></div>
      <div className={styles.agentList}>
        <p className="mb-2 text-caption text-muted-foreground">{t(($) => $.mcp.welcome.assignment_label)}</p>
        <div className={styles.agent}>
          <span className={styles.avatar} aria-hidden="true"><Puzzle className="size-4" /></span>
          <span className="min-w-0 flex-1 text-body font-medium">{t(($) => $.mcp.welcome.selected_agent)}</span>
          <span className={styles.selection} data-selected="true" aria-hidden="true"><Check className="size-3" /></span>
        </div>
        <div className={styles.agent}>
          <span className={styles.avatar} aria-hidden="true"><Orbit className="size-4" /></span>
          <span className="min-w-0 flex-1 text-body">{t(($) => $.mcp.welcome.other_agent)}</span>
          <span className={styles.selection} aria-hidden="true" />
        </div>
      </div>
      <p className={styles.assignmentNote}>{t(($) => $.mcp.welcome.assignment_note)}</p>
    </div>
  );
}

export function McpWelcome({ onBrowse, onCreate, createButtonRef }: {
  onBrowse: () => void;
  onCreate?: () => void;
  createButtonRef?: Ref<HTMLButtonElement>;
}) {
  const { t } = useT("settings");
  const headingId = useId();
  const carouselId = useId();
  const [activeSlide, setActiveSlide] = useState(0);
  const slides = [
    { key: "choose", title: t(($) => $.mcp.welcome.choose_title), description: t(($) => $.mcp.welcome.choose_description), scene: <ServiceExample /> },
    { key: "configure", title: t(($) => $.mcp.welcome.configure_title), description: t(($) => $.mcp.welcome.configure_description), scene: <ConfigurationExample /> },
    { key: "assign", title: t(($) => $.mcp.welcome.assign_title), description: t(($) => $.mcp.welcome.assign_description), scene: <AssignmentExample /> },
  ];

  return (
    <section className={styles.welcome} aria-labelledby={headingId}>
      <div className={styles.layout}>
        <div className={styles.intro}>
          <h2 id={headingId} className="text-balance text-display font-semibold tracking-tight">{t(($) => $.mcp.welcome.title)}</h2>
          <p className="mt-4 text-pretty text-body-lg leading-7 text-muted-foreground">{t(($) => $.mcp.welcome.description)}</p>
          <div className="mt-7 flex flex-wrap items-center gap-x-3 gap-y-2">
            <Button type="button" onClick={onBrowse} className="min-h-11 gap-2 px-4">
              <Compass className="size-4" aria-hidden="true" />
              {t(($) => $.mcp.browse_market)}
            </Button>
            {onCreate ? (
              <Button ref={createButtonRef} type="button" variant="ghost" onClick={onCreate} className="min-h-11 gap-2 px-2">
                <Plus className="size-4" aria-hidden="true" />
                {t(($) => $.mcp.add_server)}
              </Button>
            ) : null}
          </div>
          {!onCreate ? <p className="mt-4 text-body text-muted-foreground">{t(($) => $.mcp.welcome.member_note)}</p> : null}
          <div className={styles.boundaryNote}>
            <Server className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            <p>{t(($) => $.mcp.welcome.boundary_note)}</p>
          </div>
        </div>

        <section className={styles.showcase} aria-label={t(($) => $.mcp.welcome.examples)}>
          <div className={styles.viewport}>
            <div id={carouselId} className={styles.track} style={{ "--slide": activeSlide } as CSSProperties}>
              {slides.map((slide, index) => (
                <div key={slide.key} role="group" aria-label={slide.title} aria-hidden={activeSlide !== index} inert={activeSlide !== index} data-active={activeSlide === index} className={styles.card}>
                  <div className={styles.scene}>
                    <span className={styles.exampleLabel}>{t(($) => $.mcp.welcome.example)}</span>
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
                <button key={slide.key} type="button" aria-label={t(($) => $.mcp.welcome.show_example, { number: index + 1, title: slide.title })} aria-pressed={activeSlide === index} aria-controls={carouselId} onClick={() => setActiveSlide(index)} className="group flex size-11 items-center justify-center rounded-lg outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-foreground">
                  <span aria-hidden="true" className="h-1.5 w-1.5 rounded-full bg-input transition-[width,background-color] group-aria-pressed:w-5 group-aria-pressed:bg-foreground motion-reduce:transition-none" />
                </button>
              ))}
            </div>
            <div className="flex gap-1.5">
              <Button type="button" size="icon" variant="outline" className="size-11 focus-visible:ring-foreground" aria-label={t(($) => $.mcp.welcome.previous)} aria-controls={carouselId} disabled={activeSlide === 0} onClick={() => setActiveSlide((current) => Math.max(0, current - 1))}>
                <ArrowLeft className="size-4" aria-hidden="true" />
              </Button>
              <Button type="button" size="icon" variant="outline" className="size-11 focus-visible:ring-foreground" aria-label={t(($) => $.mcp.welcome.next)} aria-controls={carouselId} disabled={activeSlide === slides.length - 1} onClick={() => setActiveSlide((current) => Math.min(slides.length - 1, current + 1))}>
                <ArrowRight className="size-4" aria-hidden="true" />
              </Button>
            </div>
          </div>
          <p role="status" aria-live="polite" aria-atomic="true" className="sr-only">
            {t(($) => $.mcp.welcome.position, { number: activeSlide + 1, total: slides.length, title: slides[activeSlide]?.title })}
          </p>
        </section>
      </div>
    </section>
  );
}
