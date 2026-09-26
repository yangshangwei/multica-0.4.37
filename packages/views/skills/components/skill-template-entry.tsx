"use client";

import { useQuery } from "@tanstack/react-query";
import { skillTemplateListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { getSkillTemplateDiscoveryItems } from "../lib/skill-template-discovery";

export function SkillTemplateEntry({ workspaceId, onBrowse, buttonRef }: {
  workspaceId: string;
  onBrowse: () => void;
  buttonRef?: React.Ref<HTMLButtonElement>;
}) {
  const { t } = useT("skills");
  const catalog = useQuery(skillTemplateListOptions(workspaceId));
  const items = getSkillTemplateDiscoveryItems(catalog.data ?? [], t);
  const builtinCount = items.filter(({ source }) => source === "builtin").length;
  const hasData = catalog.data !== undefined;
  const counts = items.length > 0
    ? [
        t(($) => $.template_entry.total, { count: items.length }),
        t(($) => $.template_entry.builtin_count, { count: builtinCount }),
        t(($) => $.template_entry.deployment_count, { count: items.length - builtinCount }),
      ].join(" · ")
    : t(($) => $.create.template.empty);

  return (
    <section aria-label={t(($) => $.template_entry.title)} className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 border-b px-6 py-2 md:grid-cols-[auto_minmax(0,1fr)_auto]">
      <h2 className="min-w-0 text-body font-medium">{t(($) => $.template_entry.title)}</h2>
      <p className="col-span-2 row-start-2 min-w-0 text-caption text-muted-foreground md:col-span-1 md:col-start-2 md:row-start-1">
        {hasData ? counts : !catalog.isError && <span role="status">{t(($) => $.create.template.loading)}</span>}
      </p>
      <Button ref={buttonRef} size="sm" variant="outline" className="col-start-2 row-start-1 md:col-start-3 md:row-span-2" onClick={onBrowse}>
        {t(($) => $.template_entry.browse)}
      </Button>
      <p className="col-span-2 row-start-3 text-caption text-muted-foreground md:row-start-2">{t(($) => $.template_entry.description)}</p>
      {(catalog.isError || (hasData && catalog.isFetching)) && (
        <div role={catalog.isError ? "alert" : "status"} className="col-span-full flex flex-wrap items-center gap-2 text-caption text-muted-foreground">
          <span>{catalog.isError
            ? hasData ? t(($) => $.create.template.refresh_failed) : t(($) => $.create.template.load_failed)
            : t(($) => $.create.template.refreshing)}</span>
          {catalog.isError && <Button variant="ghost" size="sm" onClick={() => void catalog.refetch()}>{t(($) => $.create.template.retry)}</Button>}
        </div>
      )}
    </section>
  );
}
