import { useState } from "react";
import { CollaborationUnavailableError, isKnowledgeProtocolUnsupported, prepareKnowledgeAutomationRetry, selectKnowledgeAutomationJob,
  useKnowledgeAutomationJobs, type CollaborationIssue, type CollaborationScope, type KnowledgeAutomationJob } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useLocale, useT } from "../i18n";
import { useKnowledgeAutomationControl } from "./knowledge-automation-recovery";

export function KnowledgeAutomationStatus({ scope, issue, onOpenKnowledge }: {
  scope: CollaborationScope; issue: CollaborationIssue; onOpenKnowledge?: (opener: HTMLElement) => void;
}) {
  const { t } = useT("collaboration");
  const submission = issue.delivery?.latest;
  const accepted = submission?.decision?.kind === "accepted" ? submission : null;
  const projectId = issue.source_structure?.project?.id ?? null;
  const eligible = projectId !== null && accepted !== null && issue.ai_policy !== "manual";
  const query = useKnowledgeAutomationJobs(scope, issue.id, eligible);
  const control = useKnowledgeAutomationControl(scope, { kind: "retry", issueId: issue.id });
  const [invalid, setInvalid] = useState(false);
  const job = selectKnowledgeAutomationJob(query.data?.value, { issueId: issue.id, projectId, submissionId: accepted?.id ?? null,
    acceptedEventSeq: accepted?.decision?.event_seq ?? null, aiPolicy: issue.ai_policy });
  if (!eligible && control.pendingCount === 0 || eligible && query.data && !job && control.pendingCount === 0) return null;
  const retry = () => {
    if (!job || control.locked) return;
    try { setInvalid(false); void control.submit(prepareKnowledgeAutomationRetry(scope, job)); }
    catch { setInvalid(true); }
  };
  const working = job && ["queued", "generating", "checking", "settling"].includes(job.phase);
  const outcome = !job ? null : !job.source_current ? t(($) => $.knowledge.automation.paused)
    : working ? t(($) => $.knowledge.automation.working)
      : job.phase === "waiting_configuration" ? t(($) => $.knowledge.automation.unconfigured)
        : job.phase === "review_pending" ? t(($) => $.knowledge.automation.pending)
          : job.phase === "cancelled" ? t(($) => $.knowledge.automation.cancelled)
            : job.phase === "skipped" ? t(($) => $.knowledge.automation.skipped)
              : job.counts.saved + job.counts.restored + job.counts.revised === 0 ? t(($) => $.knowledge.automation.noNew) : null;
  return <section aria-label={t(($) => $.knowledge.automation.title)} className="min-w-0 space-y-3 rounded-md bg-muted/30 p-4 pointer-coarse:[&_button]:min-h-11">
    <h4 className="text-body font-semibold">{t(($) => $.knowledge.automation.title)}</h4>
    {control.notice}
    {!job && eligible && !query.data && <div className="space-y-2"><p role={query.isFetching ? "status" : "alert"} className="text-caption text-muted-foreground">
      {isKnowledgeProtocolUnsupported(query.error) || query.error instanceof CollaborationUnavailableError ? t(($) => $.knowledge.protocolUnsupported)
        : query.isFetching ? t(($) => $.knowledge.automation.loading) : t(($) => $.knowledge.automation.loadError)}</p>
      {!query.isFetching && <Button type="button" variant="outline" size="sm" onClick={() => void query.refetch()}>{t(($) => $.knowledge.refresh)}</Button>}</div>}
    {job && <>
      {outcome && <p role="status" className="text-body">{outcome}</p>}
      {job.counts.saved + job.counts.restored + job.counts.revised > 0 && <div className="flex flex-wrap gap-x-4 gap-y-1 text-caption text-muted-foreground">
        {job.counts.saved > 0 && <span>{t(($) => $.knowledge.automation.saved, { count: job.counts.saved })}</span>}
        {job.counts.restored > 0 && <span>{t(($) => $.knowledge.automation.restored, { count: job.counts.restored })}</span>}
        {job.counts.revised > 0 && <span>{t(($) => $.knowledge.automation.revised, { count: job.counts.revised })}</span>}
      </div>}
      {(onOpenKnowledge && (job.counts.saved + job.counts.restored + job.counts.revised + job.counts.pending > 0 || job.phase === "review_pending") || job.can_retry) && <div className="flex flex-wrap gap-2">
        {onOpenKnowledge && (job.counts.saved + job.counts.restored + job.counts.revised + job.counts.pending > 0 || job.phase === "review_pending") &&
          <Button type="button" variant="outline" size="sm" onClick={(event) => onOpenKnowledge(event.currentTarget)}>{t(($) => $.knowledge.automation.review)}</Button>}
        {job.can_retry && <Button type="button" variant="outline" size="sm" disabled={control.locked} onClick={retry}>{t(($) => $.knowledge.automation.retry)}</Button>}
      </div>}
      {job.can_retry && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.automation.retryHint)}</p>}
      {invalid && <p role="alert" className="text-caption text-muted-foreground">{t(($) => $.knowledge.automation.changed)}</p>}
      {job.attempts.length > 0 && <KnowledgeAutomationUsage job={job} />}
      {query.data?.value.has_more && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.automation.moreHistory)}</p>}
    </>}
  </section>;
}

function KnowledgeAutomationUsage({ job }: { job: KnowledgeAutomationJob }) {
  const { t } = useT("collaboration"), locale = useLocale();
  const number = new Intl.NumberFormat(locale);
  return <details className="min-w-0 text-caption"><summary className="cursor-pointer text-muted-foreground pointer-coarse:min-h-11 pointer-coarse:py-3">{t(($) => $.knowledge.automation.details)}</summary>
    <ol className="mt-3 space-y-4">{job.attempts.map((attempt, index) => {
      const values = [[t(($) => $.knowledge.automation.inputTokens), attempt.usage.input_tokens], [t(($) => $.knowledge.automation.outputTokens), attempt.usage.output_tokens],
        [t(($) => $.knowledge.automation.totalTokens), attempt.usage.total_tokens], ...(attempt.usage.cached_input_tokens != null ? [[t(($) => $.knowledge.automation.cachedTokens), attempt.usage.cached_input_tokens]] : []),
        ...(attempt.usage.reasoning_tokens != null ? [[t(($) => $.knowledge.automation.reasoningTokens), attempt.usage.reasoning_tokens]] : [])] as const;
      return <li key={`${attempt.stage}:${index}`} className="min-w-0 space-y-2"><p className="font-medium [overflow-wrap:anywhere]">{t(($) => $.knowledge.automation[attempt.stage])} · {attempt.model ?? t(($) => $.knowledge.automation.unknownModel)}</p>
        <p className="text-muted-foreground">{attempt.usage.status === "known" ? t(($) => $.knowledge.automation.usageKnown) : attempt.usage.status === "partial" ? t(($) => $.knowledge.automation.usagePartial) : t(($) => $.knowledge.automation.usageUnknown)}</p>
        <dl className="flex flex-wrap gap-x-5 gap-y-2 text-muted-foreground">{values.map(([label, value]) => <div key={String(label)}><dt>{label}</dt><dd className="tabular-nums">{typeof value === "number" ? number.format(value) : t(($) => $.knowledge.automation.unknown)}</dd></div>)}</dl>
      </li>;
    })}</ol>
  </details>;
}
