import { useState } from "react";
import { useCollaborationTestReport, useCollaborationTestReports, type CollaborationIssue, type CollaborationScope, type CollaborationTestReport } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { CollaborationQueryState } from "./query-state";
import { useCollaborationStageLabels } from "./stage-labels";

export function StageTestReportDetail({ report }: { report: CollaborationTestReport }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  return <section aria-label={t(($) => $.stage.report.detail)} className="min-w-0 space-y-3 text-caption [overflow-wrap:anywhere]">
    <p>{labels.reportResult(report.reported_result)} · {labels.reportApplicability(report.applicability)}</p>
    <p>{t(($) => $.stage.code.reportCounts, { total: report.case_count, failed: report.failed_case_count })}</p>
    <p>{t(($) => $.stage.code.reportRun, { run: report.run_id, attempt: report.attempt })}</p>
    <p>{t(($) => $.stage.testInput.subject, { candidate: report.subject.candidate_id, version: report.subject.candidate_version })}</p>
    <dl className="space-y-2 [&_dt]:text-muted-foreground">
      <div><dt>{t(($) => $.stage.code.report)}</dt><dd>{report.report_id}</dd><dd>{report.report_digest}</dd></div>
      <div><dt>{t(($) => $.stage.report.candidateDigest)}</dt><dd>{report.subject.candidate_digest}</dd></div>
      <div><dt>{t(($) => $.stage.report.agent)}</dt><dd>{report.agent_id}</dd></div>
      <div><dt>{t(($) => $.stage.report.suite)}</dt><dd>{report.suite_id}</dd></div>
      <div><dt>{t(($) => $.stage.report.source)}</dt><dd>{report.subject.source_revision}</dd></div>
      <div><dt>{t(($) => $.stage.code.build)}</dt><dd>{report.subject.artifact_digest ?? t(($) => $.stage.code.noBuild)}</dd></div>
      <div><dt>{t(($) => $.stage.report.binding)}</dt><dd>{report.subject.binding_digest}</dd></div>
    </dl>
    <p className="text-muted-foreground">{t(($) => $.stage.code.reportHint)}</p>
    <ol className="space-y-2">{report.cases.map((testCase) => <li key={testCase.id}>
      <p className="font-medium">{testCase.id} · {t(($) => $.stage.report.outcomes[testCase.outcome])} · {t(($) => $.stage.report.logNumber, { index: testCase.log_index + 1 })}</p>
    </li>)}</ol>
    <ol className="space-y-4">{report.logs.map((log, index) => <li key={index} className="space-y-2">
        <p className="font-medium">{t(($) => $.stage.report.logNumber, { index: index + 1 })}</p>
        <p className="text-muted-foreground">{log.kind === "record" ? t(($) => $.stage.report.recordLog, { event: log.event_id, index: log.message_index }) : t(($) => $.stage.report.inlineLog)}</p>
        <p className="text-muted-foreground">{t(($) => $.stage.report.logDigest, { digest: log.body_sha256 })}</p>
        <pre className="max-h-96 overflow-auto whitespace-pre-wrap rounded-md bg-muted p-3 font-mono [overflow-wrap:anywhere]">{log.body}</pre>
      </li>)}</ol>
  </section>;
}

export function StageTestReportReader({ scope, issue, report }: { scope: CollaborationScope; issue: CollaborationIssue; report: { run_id: string; report_id: string } }) {
  const { t } = useT("collaboration");
  const query = useCollaborationTestReport(scope, issue.id, issue.source_workspace.id, report);
  return <div className="space-y-2">
    {query.isPending && <p role="status">{t(($) => $.stage.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {!query.error && query.data?.state === "available" && <StageTestReportDetail report={query.data.value} />}
    {!query.error && query.data && query.data.state !== "available" && <p role="status">{query.data.state === "unavailable" && query.data.reason === "log_not_retained" ? t(($) => $.stage.report.logUnavailable) : t(($) => $.stage.report.unavailable)}</p>}
  </div>;
}

/** Each run starts its own server-owned cursor chain; selecting a report is always explicit. */
export function StageTestReportHistory({ scope, issue, runId }: { scope: CollaborationScope; issue: CollaborationIssue; runId: string }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const query = useCollaborationTestReports(scope, issue.id, issue.source_workspace.id, runId);
  const [selectedId, select] = useState<string | null>(null);
  const pages = !query.error ? query.data?.pages : undefined;
  const available = pages?.every((page) => page.state === "available") === true;
  const items = pages?.flatMap((page) => page.state === "available" ? page.value.items : []) ?? [];
  return <section aria-label={t(($) => $.stage.report.runHistory)} className="space-y-3">
    {query.isPending && <p role="status">{t(($) => $.stage.loading)}</p>}
    {query.error && <CollaborationQueryState error={query.error} retry={() => void query.refetch()} />}
    {pages && !available && <p role="status">{t(($) => $.stage.report.unavailable)}</p>}
    {available && items.length === 0 && !query.hasNextPage && <p>{t(($) => $.stage.code.noReports)}</p>}
    {items.map((item) => <div key={item.report_id} className="space-y-1 text-caption [overflow-wrap:anywhere]">
      <p>{item.report_id} · {labels.reportResult(item.reported_result)} · {labels.reportApplicability(item.applicability)}</p>
      <p>{t(($) => $.stage.code.reportCounts, { total: item.case_count, failed: item.failed_case_count })}</p>
      <Button type="button" variant="outline" size="sm" onClick={() => select(item.report_id)}>{t(($) => $.stage.report.open)}</Button>
    </div>)}
    {available && query.hasNextPage && <Button type="button" variant="outline" disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{t(($) => $.stage.report.more)}</Button>}
    {selectedId && <StageTestReportReader key={selectedId} scope={scope} issue={issue} report={{ run_id: runId, report_id: selectedId }} />}
  </section>;
}
