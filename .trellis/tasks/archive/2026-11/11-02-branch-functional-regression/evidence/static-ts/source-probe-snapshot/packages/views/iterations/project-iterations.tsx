"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  iterationCapabilitiesOptions,
  iterationChoicesOptions,
  iterationIssuesOptions,
} from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { IterationSelect } from "./iteration-assignment";
import { AppLink } from "../navigation";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useT } from "../i18n";
import { IterationError, isDefinitiveReadError } from "./iteration-error";
import { iterationDisclosureClass } from "./iteration-presentation";
export function ProjectIterations({
  wsId,
  projectId,
}: {
  wsId: string;
  projectId: string;
}) {
  return <ProjectIterationContent key={`${wsId}:${projectId}`} wsId={wsId} projectId={projectId} />;
}
function ProjectIterationContent({ wsId, projectId }: { wsId: string; projectId: string }) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const [id, setId] = useState("");
  const [cursors, setCursors] = useState([""]);
  const cursor = cursors[cursors.length - 1]!;
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  const list = useQuery({
    ...iterationChoicesOptions(wsId),
    enabled: capability.data?.enabled === true,
  });
  const issues = useQuery({
    ...iterationIssuesOptions(wsId, id, {
      project_id: projectId,
      ...(cursor ? { cursor } : {}),
    }),
    enabled: capability.data?.enabled === true && !!id,
  });
  if (capability.data?.enabled !== true) return null;
  const data = isDefinitiveReadError(issues.error) ? undefined : issues.data;
  return (
    <details className="min-w-0 px-4 py-3">
      <summary className={iterationDisclosureClass}>{t(($) => $.iterations.title)}</summary>
      <div className="min-w-0 space-y-4 pt-3">
        <IterationSelect
          value={id}
          disabled={list.isPending || !!list.error}
          onChange={(value) => {
            setId(value);
            setCursors([""]);
          }}
          items={list.data ?? []}
        />
        {id && (
          <AppLink href={paths.iterationDetail(id)} className="inline-flex min-h-8 items-center rounded text-caption font-medium underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground pointer-coarse:min-h-11">
            {t(($) => $.iterations.scope)}
          </AppLink>
        )}
        {(list.error || issues.error) && <div className="space-y-2">
          <IterationError error={list.error ?? issues.error} context="read" />
          {!isDefinitiveReadError(list.error ?? issues.error) && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={list.isFetching || issues.isFetching} onClick={() => { if (list.error) void list.refetch(); else if (cursor) setCursors([""]); else void issues.refetch(); }}>{t(($) => $.iterations.retry)}</Button>}
        </div>}
        {(list.isPending || (id && issues.isPending)) && <div role="status" className="space-y-3"><span className="sr-only">{t(($) => $.iterations.loading)}</span><Skeleton className="h-5 w-2/3" /><Skeleton className="h-5 w-full" /><Skeleton className="h-5 w-3/4" /></div>}
        <ul className="min-w-0 space-y-2">
          {data?.items.map((issue) => (
              <li key={issue.issue_id} className="min-w-0">
                <AppLink
                  href={paths.issueDetail(issue.issue_id)}
                  className="inline-flex min-h-8 max-w-full items-center rounded text-body underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground [overflow-wrap:anywhere] pointer-coarse:min-h-11"
                >
                  {issue.identifier} · {issue.title}
                </AppLink>
              </li>
            ))}
        </ul>
        {id && data?.total === 0 && <p className="text-body text-muted-foreground">{t(($) => $.iterations.pages.noMatches)}</p>}
        <div className="flex flex-wrap gap-2">
          {cursors.length > 1 && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={issues.isFetching} onClick={() => setCursors((previous) => previous.slice(0, -1))}>{t(($) => $.iterations.audit.previousPage)}</Button>}
          {data?.next_cursor && <Button variant="outline" className="pointer-coarse:min-h-11" disabled={issues.isFetching} onClick={() => setCursors((previous) => [...previous, data.next_cursor!])}>
            {t(($) => $.iterations.next)}
          </Button>}
        </div>
      </div>
    </details>
  );
}
