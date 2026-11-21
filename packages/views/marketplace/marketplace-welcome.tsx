"use client";

import { useId } from "react";
import { Tabs as BaseTabs } from "@base-ui/react/tabs";
import { ArrowDownRight, ArrowRight, BookOpen, Bot, ChevronDown, Circle, ClipboardList, Plug, UsersRound } from "lucide-react";
import { DOCS_SLUGS } from "@multica/core/docs";
import { useWorkspacePaths } from "@multica/core/paths";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { PAGE_GUTTER } from "../layout/page-header";
import { AppLink } from "../navigation";
import styles from "./marketplace-welcome.module.css";

export const MARKETPLACE_CATEGORIES = [
  { value: "mcp", copyKey: "mcp", icon: Plug, docs: DOCS_SLUGS.mcp },
  { value: "skills", copyKey: "skills", icon: ClipboardList, docs: DOCS_SLUGS.skills },
  { value: "squad-templates", copyKey: "squad_templates", icon: UsersRound, docs: DOCS_SLUGS.squads },
  { value: "agent-templates", copyKey: "agent_templates", icon: Bot, docs: DOCS_SLUGS.agents },
] as const;

export type MarketplaceCategory = (typeof MARKETPLACE_CATEGORIES)[number];

function ExampleTicket({ category }: { category: MarketplaceCategory }) {
  const { t } = useT("layout");
  const paths = useWorkspacePaths();
  const { icon: Icon, copyKey } = category;

  return (
    <div className={styles.ticket}>
      <div className={styles.ticketHeader}>
        <Circle className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <p className="min-w-0 flex-1 text-title-sm font-semibold">{t(($) => $.marketplace.example.task)}</p>
        <span className={styles.exampleLabel}>{t(($) => $.marketplace.example.label)}</span>
      </div>
      <div aria-live="polite" className={styles.ticketBody}>
        <div key={category.value} className={styles.caseDetail}>
          <div className={styles.capabilityStep}>
            <Icon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            <div>
              <p className="text-body font-semibold">{t(($) => $.marketplace[copyKey].resource)}</p>
              <p className="mt-0.5 text-label text-foreground">{t(($) => $.marketplace[copyKey].setup)}</p>
            </div>
          </div>
          <p className={styles.outcome}>
            <ArrowDownRight className="mt-0.5 size-4 shrink-0 text-info-foreground" aria-hidden="true" />
            {t(($) => $.marketplace[copyKey].outcome)}
          </p>
        </div>
      </div>
      <div className={styles.ticketFooter}>
        <AppLink href={paths.docsPage(category.docs)} className={styles.docsLink}>
          <BookOpen className="size-3.5 shrink-0" aria-hidden="true" />
          {t(($) => $.marketplace.example.docs)}
        </AppLink>
      </div>
    </div>
  );
}

export function MarketplaceWelcome({ category }: { category: MarketplaceCategory }) {
  const { t } = useT("layout");
  const id = useId();

  return (
    <section aria-labelledby={id} className={cn(styles.welcome, PAGE_GUTTER)}>
      <div className={styles.intro}>
        <h2 id={id} className={styles.title}>{t(($) => $.marketplace.welcome.title)}</h2>
        <p className={styles.description}>{t(($) => $.marketplace.welcome.description)}</p>
        <BaseTabs.List activateOnFocus={false} aria-label={t(($) => $.marketplace.type_label)} className={styles.directory}>
          {MARKETPLACE_CATEGORIES.map(({ value, icon: Icon, copyKey }) => {
            const labelId = `${id}-${value}`;
            const purposeId = `${labelId}-purpose`;

            return (
              <BaseTabs.Tab key={value} value={value} aria-labelledby={labelId} aria-describedby={purposeId} className={styles.entry}>
                <Icon className={styles.entryIcon} aria-hidden="true" />
                <span className="min-w-0">
                  <span id={labelId} className="block font-semibold">{t(($) => $.marketplace[copyKey].label)}</span>
                  <span id={purposeId} className={styles.purpose}>{t(($) => $.marketplace[copyKey].purpose)}</span>
                </span>
                <ArrowRight className={styles.entryArrow} aria-hidden="true" />
              </BaseTabs.Tab>
            );
          })}
        </BaseTabs.List>
      </div>
      <figure className={styles.wideExample} aria-label={t(($) => $.marketplace.example.label)}>
        <ExampleTicket category={category} />
      </figure>
    </section>
  );
}

export function MarketplaceCompactExample({ category }: { category: MarketplaceCategory }) {
  const { t } = useT("layout");

  return (
    <details className={styles.compactExample}>
      <summary className={styles.summary}>
        <BookOpen className="size-4 shrink-0 text-info-foreground" aria-hidden="true" />
        <span className="min-w-0 flex-1">{t(($) => $.marketplace.example.show)}</span>
        <ChevronDown className={styles.chevron} aria-hidden="true" />
      </summary>
      <ExampleTicket category={category} />
    </details>
  );
}
