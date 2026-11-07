"use client";

import { useId } from "react";
import type { AdminCollaborationInput, AdminCollaborationResult } from "@multica/core/admin";
import { useT } from "../../i18n";

export function AdminCollaborationReceipt({ input, receipt }: { input: AdminCollaborationInput; receipt?: AdminCollaborationResult }) {
  const { t } = useT("admin");
  const id = useId();
  const result = receipt?.result;
  return <section aria-labelledby={`${id}-title`} className="min-w-0 space-y-4">
    <h2 id={`${id}-title`} className="text-body-lg font-semibold">{result ? t($ => $.collaboration.receiptTitle) : t($ => $.collaboration.pendingTitle)}</h2>
    {result && <p role="status" className="text-body font-medium">{t($ => $.collaboration.outcomes[result.code])}</p>}
    {!result && <p className="text-body font-medium">{t($ => $.collaboration.actions[input.body.action])}</p>}
    <p className="max-w-prose text-body text-muted-foreground">{result ? t($ => $.collaboration.receiptHint) : t($ => $.collaboration.pendingHint)}</p>
    <dl className="grid min-w-0 gap-4 text-body sm:grid-cols-2">
      <div className="min-w-0"><dt className="text-caption text-muted-foreground">{t($ => $.collaboration.currentStatus)}</dt><dd>{input.body.expectedStatus}</dd></div>
      <div className="min-w-0"><dt className="text-caption text-muted-foreground">{t($ => $.collaboration.targetStatus)}</dt><dd>{input.body.targetStatus}</dd></div>
      <div className="min-w-0 sm:col-span-2"><dt className="text-caption text-muted-foreground">{t($ => $.collaboration.reason)}</dt><dd className="whitespace-pre-wrap">{input.body.reason}</dd></div>
      {result && <>
        <div className="min-w-0"><dt className="text-caption text-muted-foreground">{t($ => $.collaboration.queue)}</dt><dd>{t($ => $.collaboration.queues[result.queueState])}</dd></div>
        <div className="min-w-0"><dt className="text-caption text-muted-foreground">{t($ => $.collaboration.unsettledRuns)}</dt><dd>{result.unsettledRuns}</dd></div>
      </>}
    </dl>
    <p className="max-w-prose text-caption text-muted-foreground">{t($ => $.collaboration.lifecycleNotice)}</p>
    <details className="min-w-0 text-caption">
      <summary className="min-h-11 cursor-pointer py-3 text-muted-foreground">{t($ => $.collaboration.identifiers)}</summary>
      <dl className="grid min-w-0 gap-3 py-2 sm:grid-cols-2">
        {[{ label: t($ => $.collaboration.issueId), value: input.id }, { label: t($ => $.collaboration.requestId), value: input.key },
          { label: t($ => $.collaboration.revision), value: input.body.expectedCollaborationRevision }, { label: t($ => $.collaboration.creator), value: input.creatorUserId },
          { label: t($ => $.collaboration.previousHandler), value: input.handlerUserId },
          { label: t($ => $.collaboration.handler), value: result?.handlerUserId ?? input.body.successorUserId ?? input.handlerUserId }].map(field => <div className="min-w-0" key={field.label}>
          <dt className="text-muted-foreground">{field.label}</dt><dd>{field.value}</dd>
        </div>)}
      </dl>
    </details>
  </section>;
}
