"use client";

import { useId, useState, type CSSProperties } from "react";
import { ArrowLeft, ArrowRight, BookOpen, Check, Download, Laptop, Plus, Server, Terminal } from "lucide-react";
import { useConfigStore } from "@multica/core/config";
import { useWorkspaceSlug } from "@multica/core/paths";
import { Button, buttonVariants } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { ProviderLogo } from "./provider-logo";
import { daemonRuntimesDocsHref } from "./runtime-docs";
import styles from "./runtime-welcome.module.css";

const EXAMPLE_PROVIDERS = [
  { provider: "claude", label: "Claude" },
  { provider: "codex", label: "Codex" },
  { provider: "cursor", label: "Cursor" },
  { provider: "copilot", label: "Copilot" },
];
const EXAMPLE_SETUP_COMMAND = "multica setup";
const CAROUSEL_ARROW_CLASS =
  "size-11 aria-disabled:pointer-events-none aria-disabled:not-focus-visible:opacity-50 aria-disabled:focus-visible:text-muted-foreground";

function DesktopExample() {
  const { t } = useT("runtimes");
  return (
    <div className={styles.machine}>
      <div className="flex items-center gap-2.5">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted" aria-hidden="true">
          <Laptop className="size-5" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-body font-medium">{t(($) => $.welcome.desktop.machine)}</p>
          <p className="text-caption text-muted-foreground">{t(($) => $.welcome.desktop.machine_hint)}</p>
        </div>
        <span className="flex shrink-0 items-center gap-1.5 text-caption">
          <span aria-hidden="true" className="size-1.5 rounded-full bg-success" />
          {t(($) => $.welcome.desktop.online)}
        </span>
      </div>
      <p className="mb-2 mt-4 text-caption text-muted-foreground">{t(($) => $.welcome.desktop.detected)}</p>
      <div className={styles.providers}>
        {EXAMPLE_PROVIDERS.map(({ provider, label }) => (
          <span key={provider}>
            <span aria-hidden="true"><ProviderLogo provider={provider} className="size-3.5" /></span>
            {label}
          </span>
        ))}
      </div>
    </div>
  );
}

function RemoteExample() {
  const { t } = useT("runtimes");
  return (
    <div className={styles.terminal}>
      <div className="flex items-center gap-2 border-b border-surface-border px-3.5 py-2.5 text-caption text-muted-foreground">
        <Terminal className="size-3.5" aria-hidden="true" />
        {t(($) => $.welcome.remote.prompt)}
      </div>
      <div className="space-y-3 p-3.5">
        <p className="text-caption text-muted-foreground">{t(($) => $.welcome.remote.install_hint)}</p>
        <code className={styles.command}>{EXAMPLE_SETUP_COMMAND}</code>
        <p className="flex items-start gap-1.5 text-caption text-success-foreground">
          <Check className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          {t(($) => $.welcome.remote.connected)}
        </p>
      </div>
    </div>
  );
}

export function RuntimeWelcome({ onConnectRemote }: { onConnectRemote: () => void }) {
  const { t } = useT("runtimes");
  const workspaceSlug = useWorkspaceSlug();
  const docsHref = daemonRuntimesDocsHref(workspaceSlug);
  const downloadUrl = useConfigStore((state) => state.desktopDownloadUrl);
  const headingId = useId();
  const carouselId = useId();
  const [activeSlide, setActiveSlide] = useState(0);
  const slides = [
    { key: "desktop", title: t(($) => $.welcome.desktop.title), description: t(($) => $.welcome.desktop.description), scene: <DesktopExample /> },
    { key: "remote", title: t(($) => $.welcome.remote.title), description: t(($) => $.welcome.remote.description), scene: <RemoteExample /> },
  ];

  return (
    <section className={styles.welcome} aria-labelledby={headingId}>
      <div className={styles.layout}>
        <div className={styles.intro}>
          <div className="mb-5 inline-flex items-center gap-1.5 rounded-full bg-brand/10 px-3 py-1.5 text-caption font-medium text-info-foreground">
            <Server className="size-3.5" aria-hidden="true" />
            {t(($) => $.page.title)}
          </div>
          <h2 id={headingId} className="text-balance text-display font-semibold tracking-tight">{t(($) => $.welcome.title)}</h2>
          <p className="mt-4 text-pretty text-body-lg leading-7 text-muted-foreground">{t(($) => $.welcome.description)}</p>
          <div className="mt-7 flex flex-wrap items-center gap-x-4 gap-y-2">
            <Button type="button" onClick={onConnectRemote} className="h-11 gap-2 px-4">
              <Plus className="size-4" aria-hidden="true" />
              {t(($) => $.welcome.add_runtime)}
            </Button>
            {downloadUrl ? (
              <a href={downloadUrl} target="_blank" rel="noopener noreferrer" className={cn(buttonVariants({ variant: "ghost" }), "h-11 gap-2 px-2")}>
                <Download className="size-4" aria-hidden="true" />
                {t(($) => $.welcome.download_desktop)}
              </a>
            ) : docsHref ? (
              <AppLink className={cn(buttonVariants({ variant: "ghost" }), "h-11 gap-2 px-2")} href={docsHref}>
                <BookOpen className="size-4" aria-hidden="true" />
                {t(($) => $.welcome.guide)}
              </AppLink>
            ) : null}
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
                <button key={slide.key} type="button" aria-label={t(($) => $.welcome.show_example, { number: index + 1, title: slide.title })} aria-pressed={activeSlide === index} aria-controls={carouselId} onClick={() => setActiveSlide(index)} className="group flex size-11 items-center justify-center rounded-lg outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring">
                  <span aria-hidden="true" className="h-1.5 w-1.5 rounded-full bg-input transition-[width,background-color] group-aria-pressed:w-5 group-aria-pressed:bg-foreground motion-reduce:transition-none" />
                </button>
              ))}
            </div>
            <div className="flex gap-1.5">
              <Button
                type="button"
                size="icon"
                variant="outline"
                className={CAROUSEL_ARROW_CLASS}
                aria-label={t(($) => $.welcome.previous)}
                aria-controls={carouselId}
                disabled={activeSlide === 0}
                focusableWhenDisabled
                onClick={() => setActiveSlide((current) => Math.max(0, current - 1))}
              >
                <ArrowLeft className="size-4" aria-hidden="true" />
              </Button>
              <Button
                type="button"
                size="icon"
                variant="outline"
                className={CAROUSEL_ARROW_CLASS}
                aria-label={t(($) => $.welcome.next)}
                aria-controls={carouselId}
                disabled={activeSlide === slides.length - 1}
                focusableWhenDisabled
                onClick={() => setActiveSlide((current) => Math.min(slides.length - 1, current + 1))}
              >
                <ArrowRight className="size-4" aria-hidden="true" />
              </Button>
            </div>
          </div>
          <p role="status" aria-atomic="true" className="sr-only">{t(($) => $.welcome.position, { number: activeSlide + 1, total: slides.length, title: slides[activeSlide]?.title })}</p>
        </section>
      </div>
    </section>
  );
}
