import type { CollaborationStageBlocker } from "@multica/core/collaboration";
import { useT } from "../i18n";
import { StageBlockers } from "./stage-blockers";
import { useCollaborationStageLabels } from "./stage-labels";

/** A refusal snapshot is separate from the currently refreshed stage and from successful completion. */
export function CollaborationStageRefusalNotice({ refusal, originalUnknown = false }: { refusal: {
  code: string; status?: number; stage_blockers?: CollaborationStageBlocker[]; stage_blockers_truncated?: boolean;
}; originalUnknown?: boolean }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  return <div role="alert" className="min-w-0 space-y-2 text-body [overflow-wrap:anywhere]">
    <p>{originalUnknown ? t(($) => $.stage.outcome.refusedAttempt, { reason: labels.refusal(refusal.code), status: refusal.status, code: refusal.code })
      : t(($) => $.stage.outcome.refused, { reason: labels.refusal(refusal.code), status: refusal.status, code: refusal.code })}</p>
    <StageBlockers blockers={refusal.stage_blockers ?? []} truncated={refusal.stage_blockers_truncated} />
    <p className="text-caption text-muted-foreground">{originalUnknown ? t(($) => $.stage.outcome.originalStillUnknown) : t(($) => $.stage.outcome.refusalSnapshot)}</p>
  </div>;
}
