import { collaborationStageExecutionKnown, type CollaborationStageCodeCandidate } from "@multica/core/collaboration";
import { useT } from "../i18n";
import { useCollaborationStageLabels } from "./stage-labels";

export function StageExecutionHistory({ record, now }: { record: CollaborationStageCodeCandidate; now: number }) {
  const { t } = useT("collaboration");
  const labels = useCollaborationStageLabels();
  const known = collaborationStageExecutionKnown(record);
  return <div className="min-w-0 space-y-4">
    {!known && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.stage.execution.unavailable)}</p>}
    {known && <>
      <p className="text-caption text-muted-foreground">{t(($) => $.stage.execution.boundedHistory)}</p>
      <section aria-label={t(($) => $.stage.execution.operations)} className="space-y-2">
        <h5 className="text-body font-medium">{t(($) => $.stage.execution.operations)}</h5>
        <p className="text-caption text-muted-foreground">{t(($) => $.stage.execution.operationHint)}</p>
        {record.execution_operations?.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.stage.execution.noOperations)}</p>}
        {record.execution_operations?.map((operation) => <div key={operation.intent_id} className="min-w-0 space-y-1 text-caption [overflow-wrap:anywhere]">
          <p>{labels.executionKind(operation.kind)} · {operation.environment}</p>
          <p>{labels.executionOperationState(operation.state)}</p>
          <p>{t(($) => $.stage.execution.operationId, { id: operation.intent_id })}</p>
          <p>{t(($) => $.stage.execution.targetDigest, { digest: operation.target_digest })}</p>
          <p>{t(($) => $.stage.execution.intendedArtifact, { digest: operation.artifact_digest })}</p>
          {operation.external_operation_id && <p>{t(($) => $.stage.execution.nativeId, { id: operation.external_operation_id })}</p>}
        </div>)}
      </section>
      <section aria-label={t(($) => $.stage.execution.metrics)} className="space-y-2">
        <h5 className="text-body font-medium">{t(($) => $.stage.execution.metrics)}</h5>
        {record.rollback_observations?.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.stage.execution.noMetrics)}</p>}
        {record.rollback_observations?.map((observation) => <div key={observation.observation_id} className="min-w-0 space-y-1 text-caption [overflow-wrap:anywhere]">
          <p>{observation.environment} · {labels.metricCondition(observation.condition)} · {labels.metricState(observation.state === "current" && Date.parse(observation.valid_until) <= now ? "expired" : observation.state)}</p>
          <p className="whitespace-pre-wrap">{observation.summary}</p>
          <p>{t(($) => $.stage.execution.metricIdentity, { id: observation.observation_id, version: observation.version })}</p>
          <p>{t(($) => $.stage.execution.metricWindow, { start: observation.window_start, end: observation.window_end })}</p>
          <p>{t(($) => $.stage.execution.validUntil, { time: observation.valid_until })}</p>
          <p>{t(($) => $.stage.execution.originalDeployment, { id: observation.deployment_id })}</p>
          <p>{t(($) => $.stage.execution.targetDigest, { digest: observation.target_digest })}</p>
        </div>)}
      </section>
    </>}
    <section aria-label={t(($) => $.stage.code.execution)} className="space-y-2">
      <h5 className="text-body font-medium">{t(($) => $.stage.code.execution)}</h5>
      <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.executionHint)}</p>
      {known && record.execution_facts.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.stage.code.noExecution)}</p>}
      {record.execution_facts.map((fact) => <div key={fact.fact_id} className="min-w-0 space-y-1 text-caption [overflow-wrap:anywhere]">
        <p>{labels.executionKind(fact.kind)} · {labels.executionResult(fact.result)} · {fact.environment}</p>
        <p>{t(($) => $.stage.execution.nativeId, { id: fact.operation_id })}</p>
        <p>{t(($) => $.stage.execution.intendedArtifact, { digest: fact.artifact_digest })}</p>
        {fact.intent_id && <p>{t(($) => $.stage.execution.operationId, { id: fact.intent_id })}</p>}
        {fact.target_digest && <p>{t(($) => $.stage.execution.targetDigest, { digest: fact.target_digest })}</p>}
        {fact.previous_artifact_digest && <p>{t(($) => $.stage.code.rollbackTarget, { digest: fact.previous_artifact_digest })}</p>}
      </div>)}
    </section>
  </div>;
}
