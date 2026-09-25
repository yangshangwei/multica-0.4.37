"use client";

import { useId, useState } from "react";
import { useWorkspacePaths } from "@multica/core/paths";
import type { Squad, SquadTemplate } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useSquadTemplates } from "../../agents/create/use-role-templates";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";
import { UseSquadForProjectDialog } from "../../projects/components/use-squad-for-project-dialog";

export function BuiltinSquadCatalog({ squads }: { squads: readonly Squad[] }) {
  const { t } = useT("squads");
  const catalog = useSquadTemplates();
  const paths = useWorkspacePaths();
  const headingId = useId();
  const [selected, setSelected] = useState<SquadTemplate | null>(null);
  const templates = (catalog.data ?? []).filter((template) => template.key);
  const summaries = new Map([
    ["feature-delivery", t(($) => $.catalog.summaries.feature_delivery)],
    ["bug-fix", t(($) => $.catalog.summaries.bug_fix)],
    ["review-gate", t(($) => $.catalog.summaries.review_gate)],
    ["discovery", t(($) => $.catalog.summaries.discovery)],
    ["docs", t(($) => $.catalog.summaries.docs)],
    ["maintenance", t(($) => $.catalog.summaries.maintenance)],
    ["release", t(($) => $.catalog.summaries.release)],
    ["incident", t(($) => $.catalog.summaries.incident)],
  ]);

  return (
    <>
      <section
        aria-labelledby={headingId}
        aria-busy={catalog.isPending}
        className="w-full max-w-6xl px-6 py-5 @container"
      >
        <h2 id={headingId} className="text-title-sm font-semibold">{t(($) => $.catalog.title)}</h2>
        <p className="mt-1 max-w-[70ch] text-caption text-muted-foreground">{t(($) => $.catalog.description)}</p>
        {catalog.isError ? (
          <div role="alert" className="mt-4 flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
            <p>{t(($) => $.catalog.error)}</p>
            <Button type="button" size="sm" variant="outline" onClick={() => void catalog.refetch()}>{t(($) => $.catalog.retry)}</Button>
          </div>
        ) : catalog.isPending ? (
          <div role="status" className="mt-5 space-y-3">
            <span className="sr-only">{t(($) => $.catalog.loading)}</span>
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-4 w-full max-w-md" />
          </div>
        ) : templates.length === 0 ? (
          <p className="mt-4 text-caption text-muted-foreground">{t(($) => $.catalog.empty)}</p>
        ) : (
          <ul className="mt-4 grid gap-x-8 @3xl:grid-cols-2">
            {templates.map((template) => {
              const title = template.title || template.name;
              const instances = squads.filter((squad) => !squad.archived_at && squad.template_key === template.key);
              const roster = [template.leader, ...(template.members ?? [])];
              return (
                <li key={template.key} aria-label={title} className="flex min-w-0 flex-col gap-3 border-t border-border/60 py-4">
                  <div className="min-w-0">
                    <h3 className="flex items-start gap-2 text-body font-semibold">
                      {template.avatar_emoji && <span aria-hidden="true" className="shrink-0">{template.avatar_emoji}</span>}
                      <span className="min-w-0 break-words">{title}</span>
                    </h3>
                    <p className="mt-1.5 break-words text-body leading-5 text-muted-foreground">{summaries.get(template.key) ?? template.description}</p>
                  </div>
                  <div className="text-caption leading-5 text-muted-foreground">
                    <p>{t(($) => $.profile_card.member_count, { count: roster.length })}</p>
                    <p className="break-words">{roster.map((role) => role.title || role.name).join(" · ")}</p>
                  </div>
                  <div className="mt-auto flex flex-wrap items-center gap-x-3 gap-y-2">
                    <Button type="button" size="sm" variant="outline" onClick={() => setSelected(template)}>{t(($) => $.catalog.use_for_project)}</Button>
                    {instances.length > 0 && (
                      <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-caption">
                        <span className="text-muted-foreground">{t(($) => $.catalog.existing_squads)}</span>
                        {instances.map((instance) => (
                          <AppLink
                            key={instance.id}
                            href={paths.squadDetail(instance.id)}
                            newTabTitle={instance.name}
                            className="min-w-0 rounded-sm break-words underline underline-offset-4 decoration-muted-foreground/50 hover:decoration-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                          >
                            {instance.name}
                          </AppLink>
                        ))}
                      </div>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>
      {selected && <UseSquadForProjectDialog template={selected} onClose={() => setSelected(null)} />}
    </>
  );
}
