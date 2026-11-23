"use client";

import { ChevronRight, Plus, Sparkles } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import { CreationPageShell } from "../../common/create-page-shell";
import { AppLink, useBackOrReplace } from "../../navigation";
import { useT } from "../../i18n";

export function ChooseCreateSquadMethodPage() {
  const { t } = useT("squads");
  const paths = useWorkspacePaths();
  const backOrReplace = useBackOrReplace();
  const methods = [
    {
      href: paths.newSquadTemplate(),
      icon: Sparkles,
      title: t(($) => $.page.template_button),
      description: t(($) => $.create_chooser.template_description),
    },
    {
      href: paths.newSquadManual(),
      icon: Plus,
      title: t(($) => $.page.custom_button),
      description: t(($) => $.create_chooser.custom_description),
    },
  ];

  return (
    <CreationPageShell
      title={t(($) => $.page.new_button)}
      step={t(($) => $.create_chooser.step)}
      onBack={() => backOrReplace(paths.squads())}
      backLabel={t(($) => $.create_chooser.back)}
    >
      <main className="flex min-h-0 flex-1 flex-col overflow-y-auto px-5 py-10">
        <div className="m-auto w-full max-w-3xl">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-balance text-display-sm font-semibold tracking-tight sm:text-display">
              {t(($) => $.create_chooser.title)}
            </h2>
            <p className="mt-3 text-pretty text-body text-muted-foreground">
              {t(($) => $.create_chooser.description)}
            </p>
          </div>
          <div className="mt-9 grid gap-4 sm:grid-cols-2">
            {methods.map(({ href, icon: Icon, title, description }) => (
              <AppLink
                key={href}
                href={href}
                className="group flex min-h-56 min-w-0 flex-col items-start rounded-xl border bg-card p-5 text-left transition-colors hover:border-primary/40 hover:bg-accent/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <span className="flex size-11 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground group-hover:text-foreground">
                  <Icon className="size-5" aria-hidden="true" />
                </span>
                <span className="mt-7 text-title-sm font-semibold">{title}</span>
                <span className="mt-2 text-body leading-6 text-muted-foreground">
                  {description}
                </span>
                <span className="mt-auto flex items-center gap-1 pt-5 text-caption font-medium text-foreground">
                  {t(($) => $.create_chooser.continue)}
                  <ChevronRight className="size-3.5" aria-hidden="true" />
                </span>
              </AppLink>
            ))}
          </div>
        </div>
      </main>
    </CreationPageShell>
  );
}
