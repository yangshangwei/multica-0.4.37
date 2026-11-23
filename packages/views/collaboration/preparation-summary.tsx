import type { CollaborationRunRecords } from "@multica/core/collaboration";
import { useT } from "../i18n";

export function CollaborationPreparationSummary({ preparation }: { preparation: CollaborationRunRecords["preparation"] }) {
  const { t } = useT("collaboration");
  const files = preparation?.attachments ?? [];
  const attachmentCount = t(($) => $.runPreparation.attachments, { count: files.length });
  return <section aria-label={t(($) => $.runPreparation.title)} className="min-w-0 space-y-3">
    <h5 className="text-body font-medium">{t(($) => $.runPreparation.title)}</h5>
    {!preparation ? <p className="text-caption text-muted-foreground">{t(($) => $.records.notRecorded)}</p> : <>
      {files.length === 0 ? <p className="text-caption text-muted-foreground">{attachmentCount}</p> : <details className="min-w-0 space-y-3">
        <summary className="cursor-pointer py-2 text-caption pointer-coarse:min-h-11">{attachmentCount}</summary>
        <ul className="space-y-3 text-caption">{files.map((file) => <li key={file.id} className="min-w-0 space-y-1 [overflow-wrap:anywhere]">
          <p className="font-medium">{file.name}</p>
          <p className="text-muted-foreground">{t(($) => $.runPreparation.fileVersion, { version: file.version, bytes: file.size_bytes })}</p>
          <dl className="text-muted-foreground"><dt>{t(($) => $.runPreparation.fileId)}</dt><dd>{file.id}</dd></dl>
        </li>)}</ul>
      </details>}
      <dl className="min-w-0 space-y-1 text-caption">
        <dt className="text-muted-foreground">{t(($) => $.runPreparation.gitCommit)}</dt>
        <dd className="[overflow-wrap:anywhere]">{preparation.git
          ? <><span className="mr-2 text-muted-foreground">{preparation.git.object_format}</span><code>{preparation.git.commit_oid}</code></>
          : t(($) => $.records.notRecorded)}</dd>
      </dl>
      <dl className="grid min-w-0 gap-x-6 gap-y-3 text-caption sm:grid-cols-2 [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:[overflow-wrap:anywhere]">
        {(["attachments", "bundle", "git", "workspace"] as const).map((stage) => {
          const observation = preparation.stages?.find((value) => value.stage === stage);
          const reason = observation?.state === "failed" ? observation.code : undefined;
          return <div key={stage}>
            <dt>{t(($) => $.runPreparation.stages[stage])}</dt>
            <dd>{observation?.state === "prepared" ? t(($) => $.runPreparation.states.prepared)
              : observation?.state === "failed" ? t(($) => $.runPreparation.states.failed) : t(($) => $.records.notRecorded)}
              {reason && <p className="mt-1 text-muted-foreground">{t(($) => $.runPreparation.failures[reason])}</p>}
            </dd>
          </div>;
        })}
      </dl>
    </>}
  </section>;
}
