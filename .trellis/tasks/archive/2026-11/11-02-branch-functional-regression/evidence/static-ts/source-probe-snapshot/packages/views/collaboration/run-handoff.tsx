import { useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationRunHandoffOptions, type CollaborationIssue, type CollaborationRunHandoff, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { RichContent } from "../rich-content";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";

export function CollaborationRunHandoff({ scope, issueId, runId, handler }: {
  scope: CollaborationScope; issueId: string; runId: string; handler?: CollaborationIssue["handler"];
}) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useQuery(collaborationRunHandoffOptions(client, scope, issueId, runId));
  const handoff = query.data;
  return <section className="min-w-0 space-y-3" aria-label={t(($) => $.recall.handoffTitle)}>
    <h4 className="text-body font-medium">{t(($) => $.recall.handoffTitle)}</h4>
    {query.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.page.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {!handoff && !query.isPending && <p className="text-caption text-muted-foreground">{t(($) => $.recall.handoffUnavailable)}</p>}
    {handoff && <>
      <p className="text-caption text-muted-foreground">{handoff.mode === "live" ? t(($) => $.recall.handoffLive) : t(($) => $.recall.handoffSnapshot)}</p>
      <dl className="grid min-w-0 gap-x-6 gap-y-3 text-body sm:grid-cols-2 [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
        <div><dt>{t(($) => $.detail.handler)}</dt><dd>
          {handler && handler.user_id === handoff.authority.handler_id ? handler.name : t(($) => $.detail.unknownAuthor)}
          {handoff.authority.handler_id && <span className="block text-caption text-muted-foreground">{handoff.authority.handler_id}</span>}
        </dd></div>
        <div><dt>{t(($) => $.recall.assignment)}</dt><dd>{t(($) => $.execution[handoff.assignment.role])}</dd></div>
        <div><dt>{t(($) => $.recall.priorResults)}</dt><dd>{t(($) => $.recall.loadedResults, { returned: handoff.results.returned, total: handoff.results.total })}</dd></div>
        <div><dt>{t(($) => $.recall.wakeupCauses)}</dt><dd>{handoff.wakeup_causes.completeness === "unknown" ? t(($) => $.recall.causesUnknown)
          : t(($) => $.recall.loadedResults, { returned: handoff.wakeup_causes.returned, total: handoff.wakeup_causes.total })}</dd></div>
      </dl>
      <HandoffMemory issueId={issueId} memory={handoff.memory} live={handoff.mode === "live"} />
      {(handoff.missing.length > 0 || handoff.inputs.truncated || handoff.results.truncated || handoff.wakeup_causes.truncated) && <p className="text-caption text-muted-foreground">{t(($) => $.recall.handoffPartial)}</p>}
      {handoff.results.items.map((result) => <p key={result.run_id} className="text-caption [overflow-wrap:anywhere]">
        <AppLink className="underline underline-offset-4" href={accountPath({ view: "participating", issueId, runId: result.run_id })}>{result.run_id}</AppLink>
        {result.result_state === "absent" && <span className="ml-2 text-muted-foreground">{t(($) => $.recall.resultAbsent)}</span>}
      </p>)}
      <ol className="space-y-4">{handoff.sources.map((source) => <li key={`${source.kind}:${source.id}`} className="min-w-0 space-y-2">
        <p className="text-caption text-muted-foreground">{source.trust === "admitted_assignment" ? t(($) => $.recall.admittedAssignment) : t(($) => $.recall.sourceEvidence)}</p>
        {source.availability === "available" ? <RichContent resourcePolicy="restricted" content={source.content} /> : <p className="text-caption text-muted-foreground">{t(($) => $.recall.sourceUnavailable)}</p>}
        {source.truncated && <p className="text-caption text-muted-foreground">{t(($) => $.recall.sourcePartial)}</p>}
        {source.run_id && <AppLink className="text-caption underline underline-offset-4" href={accountPath({ view: "participating", issueId, runId: source.run_id })}>{t(($) => $.recall.sourceRun)}</AppLink>}
      </li>)}</ol>
    </>}
  </section>;
}

// A snapshot's version is the run's launch base. A live read's version is the
// latest; the launch base, when the run has read one, arrives as base_version.
function HandoffMemory({ issueId, memory, live }: { issueId: string; memory: CollaborationRunHandoff["memory"]; live: boolean }) {
  const { t } = useT("collaboration");
  const available = memory.availability === "available";
  const base = live ? memory.base_version : available ? memory.version : undefined;
  const latest = live && available && memory.version !== base ? memory.version : undefined;
  const link = (version: string, label: string) => <AppLink className="text-caption underline underline-offset-4" href={accountPath({ view: "participating", issueId, memoryVersion: version })}>{label}</AppLink>;
  if (base === undefined && latest === undefined) {
    return null;
  }
  return <p className="flex flex-wrap gap-x-4 gap-y-1 text-caption">
    {base === "0" ? <span className="text-muted-foreground">{t(($) => $.records.memoryBaseEmpty)}</span>
      : base !== undefined && link(base, t(($) => $.records.memoryBase, { version: base }))}
    {latest !== undefined && link(latest, t(($) => $.recall.memoryLatest, { version: latest }))}
  </p>;
}
