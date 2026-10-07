"use client";

import { useState } from "react";
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
    <details onToggle={(event) => setOpen(event.currentTarget.open)}>
      <summary>{t(($) => $.iterations.participation)}</summary>
      {history.isFetching && <p role="status">{t(($) => $.iterations.loading)}</p>}
      {history.error && <IterationError error={history.error} />}
      {!history.error && <ul className="space-y-2 py-3">
        {history.data?.items.map((iteration) => <li key={iteration.id}>
          <AppLink href={paths.iterationDetail(iteration.id)}>{iteration.name}</AppLink>
          <span className="text-muted-foreground"> · {labels.status(iteration.status)} · {iteration.start_date} – {iteration.end_date}</span>
        </li>)}
      </ul>}
      {history.data?.next_cursor && <Button onClick={() => setCursor(history.data!.next_cursor!)}>{t(($) => $.iterations.next)}</Button>}
    </details>
  );
}
