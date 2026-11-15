import type { CollaborationStageCodeCandidate, CollaborationStageCodeDecision, CollaborationStageExecutionFact, CollaborationStageRollbackObservation } from "./stage-schema";

/** Omitted legacy projections are unknown, never evidence of no pending operation. */
export function collaborationStageExecutionKnown(record: CollaborationStageCodeCandidate): boolean {
  return record.execution_targets !== undefined && record.execution_operations !== undefined && record.rollback_observations !== undefined &&
    record.execution_operations.every((o) => o.kind !== "unknown") && record.rollback_observations.every((o) => o.state !== "unknown");
}

/** A fact is usable only through the one operation belonging to the selected approval. */
export function collaborationStageDeployment(record: CollaborationStageCodeCandidate, approval: CollaborationStageCodeDecision): CollaborationStageExecutionFact | null {
  if (!collaborationStageExecutionKnown(record) || approval.step !== "release" || approval.decision !== "approved" || !approval.evidence.build?.target_digest ||
    record.candidate.baseline?.artifact?.artifact_digest !== approval.evidence.build.artifact_digest) return null;
  const operations = record.execution_operations!.filter((o) => o.approval_decision_id === approval.decision_id);
  if (operations.length !== 1) return null;
  const operation = operations[0]!;
  const facts = record.execution_facts.filter((f) => f.approval_decision_id === approval.decision_id);
  if (facts.length !== 1) return null;
  const fact = facts[0]!;
  const build = approval.evidence.build;
  return operation.kind === "deployment" && operation.state === "succeeded" && operation.fact_id === fact.fact_id &&
    operation.environment === build.environment && operation.target_digest === build.target_digest && operation.artifact_digest === build.artifact_digest &&
    fact.kind === "deployment" && fact.result === "succeeded" && fact.intent_id === operation.intent_id && fact.operation_id === operation.external_operation_id &&
    fact.environment === build.environment && fact.target_digest === build.target_digest && fact.artifact_digest === build.artifact_digest ? fact : null;
}

export function collaborationStageRollbackObservationCurrent(observation: CollaborationStageRollbackObservation, deployment: CollaborationStageExecutionFact, now = Date.now()): boolean {
  return observation.deployment_id === deployment.fact_id && observation.target_digest === deployment.target_digest && observation.environment === deployment.environment &&
    observation.artifact_digest === deployment.artifact_digest && observation.state === "current" && observation.condition === "anomaly" &&
    Date.parse(observation.observed_at) <= now && Date.parse(observation.valid_until) > now;
}

/** The current server projection owns metric ordering/ambiguity; arrival time grants no freshness. */
export function collaborationStageRollbackOptions(record: CollaborationStageCodeCandidate, validDecisions: readonly CollaborationStageCodeDecision[], now = Date.now()) {
  if (!collaborationStageExecutionKnown(record)) return [];
  return validDecisions.flatMap((approval) => {
    const deployment = collaborationStageDeployment(record, approval);
    if (!deployment?.previous_artifact_digest) return [];
    return record.rollback_observations!.filter((observation) => collaborationStageRollbackObservationCurrent(observation, deployment, now))
      .map((observation) => ({ approval, deployment, observation }));
  });
}
