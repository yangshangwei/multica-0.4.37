"use client";

import { useState } from "react";
import { CalendarRange, ChevronRight } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { iterationCapabilitiesOptions, iterationListOptions } from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { IterationError } from "./iteration-error";
import { useIterationLabels } from "./labels";

export function IterationParticipation({ wsId, issueId }: { wsId: string; issueId: string }) {
  const { t } = useT("projects");
  const labels = useIterationLabels();
  const paths = useWorkspacePaths();
  const [open, setOpen] = useState(false);
  const [cursor, setCursor] = useState("");
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  const history = useQuery({
    ...iterationListOptions(wsId, { issue_id: issueId, ...(cursor ? { cursor } : {}) }),
    enabled: open && capability.data?.supported === true,
  });
  if (capability.data?.supported !== true) return null;
  return (
    <details
      className="group/participation col-span-2 min-w-0"
      onToggle={(event) => setOpen(event.currentTarget.open)}
    >
      <summary className="-mx-2 flex min-h-8 cursor-pointer list-none items-center gap-1 rounded-md px-2 text-caption text-muted-foreground outline-none transition-colors hover:bg-accent/50 hover:text-foreground focus-visible:bg-accent/50 focus-visible:ring-2 focus-visible:ring-ring group-open/participation:text-foreground pointer-coarse:min-h-11 [&::-webkit-details-marker]:hidden">
        <span className="min-w-0">{t(($) => $.iterations.participation)}</span>
        <ChevronRight className="size-3 shrink-0 stroke-[2.5] transition-transform group-open/participation:rotate-90 motion-reduce:transition-none" aria-hidden />
      </summary>
      <div className="min-w-0 space-y-2 pb-3 pt-1 text-caption">
        {history.isFetching && <p role="status" className="py-2 text-muted-foreground">{t(($) => $.iterations.loading)}</p>}
        {history.error && (
          <div className="space-y-2 text-destructive">
            <IterationError error={history.error} />
            <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" disabled={history.isFetching} onClick={() => void history.refetch()}>
              {t(($) => $.iterations.retry)}
            </Button>
          </div>
        )}
        {!history.error && history.data && (
          history.data.items.length > 0 ? (
            <ul className="space-y-2">
              {history.data.items.map((iteration) => (
                <li key={iteration.id} className="min-w-0">
                  <AppLink
                    href={paths.iterationDetail(iteration.id)}
                    title={iteration.name}
                    className="-mx-2 flex min-h-8 min-w-0 items-center gap-1.5 rounded-md px-2 text-caption outline-none transition-colors hover:bg-accent/50 focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11"
                  >
                    <CalendarRange className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
                    <span className="truncate">{iteration.name}</span>
                  </AppLink>
                  <p className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5 pl-5 text-caption text-muted-foreground">
                    <span>{labels.status(iteration.status)}</span>
                    <span aria-hidden>·</span>
                    <span className="whitespace-nowrap">{iteration.start_date} – {iteration.end_date}</span>
                  </p>
                </li>
              ))}
            </ul>
          ) : !history.isFetching && (
            <p className="py-2 text-muted-foreground">{t(($) => $.iterations.empty)}</p>
          )
        )}
        {!history.error && history.data?.next_cursor && (
          <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" disabled={history.isFetching} onClick={() => setCursor(history.data!.next_cursor!)}>
            {t(($) => $.iterations.next)}
          </Button>
        )}
      </div>
    </details>
  );
}
