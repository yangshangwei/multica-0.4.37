import type { CollaborationStageBlocker } from "@multica/core/collaboration";
import { useT } from "../i18n";
import { useCollaborationStageLabels } from "./stage-labels";

/** Safe blocker projection only; never render hidden evidence identities or raw payloads. */
export function StageBlockers({ blockers, truncated = false }: { blockers: CollaborationStageBlocker[]; truncated?: boolean }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  if (!blockers.length) return null;
  return <div className="min-w-0 space-y-1 text-caption text-muted-foreground">
    <ul className="list-disc space-y-1 pl-5">{blockers.map((blocker, index) => <li key={index}>{labels.stage(blocker.stage)}: {labels.refusal(blocker.code)}</li>)}</ul>
    {truncated && <p>{t(($) => $.stage.code.moreBlockers)}</p>}
  </div>;
}
