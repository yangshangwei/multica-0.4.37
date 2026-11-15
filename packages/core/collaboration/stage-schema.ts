import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./account-schema";
import { collaborationTextByteLength } from "./text-limits";
import { nonnegativeVersion, positiveVersion } from "./version";

/**
 * PAY-241 S2 stage wire contract (`collaboration-stage-v1`). This is a pure
 * schema leaf: it must not import the API barrel, the client or any request
 * helper. Every network value passes through one of these schemas; a value
 * that does not fit is *unknown/unavailable*, never "gates passed".
 */
export const COLLABORATION_STAGE_CONTRACT = "collaboration-stage-v1";
export const COLLABORATION_STAGE_PROTOCOL_VERSION = 1;

const ZERO_UUID = "00000000-0000-0000-0000-000000000000";
const NUL = String.fromCharCode(0);
const uuidPattern = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;
/** Canonical lowercase identity; the zero UUID is never a real identifier. */
const stageId = z.string().regex(uuidPattern).transform((value) => value.toLowerCase()).refine((value) => value !== ZERO_UUID, "Zero UUID");
const digest = z.string().regex(/^[0-9a-f]{64}$/);
const timestamp = z.string().datetime({ offset: true });
const expiryTimestamp = timestamp.regex(/T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?Z$/);

function wellFormedText(value: string): boolean {
  if (value.includes(NUL)) return false;
  // A lone surrogate has no UTF-8 encoding; the server would refuse the body.
  try { encodeURIComponent(value); return true; } catch { return false; }
}
const characters = (value: string) => [...value].length;

/** A server enum that may grow: an unfamiliar string is `unknown`, a non-string is malformed. */
function openEnum<const T extends readonly [string, ...string[]]>(values: T) {
  return z.string().transform((value): T[number] | "unknown" => (values as readonly string[]).includes(value) ? value as T[number] : "unknown");
}

export const COLLABORATION_STAGES = ["development", "code_review", "qa", "release", "verification"] as const;
export const COLLABORATION_STAGE_ROLES = ["business_owner", "implementer", "product_approver", "code_reviewer", "qa_approver", "release_approver"] as const;
export const COLLABORATION_STAGE_PRIORITIES = ["urgent", "high", "medium", "low", "none"] as const;
export type CollaborationStageRole = typeof COLLABORATION_STAGE_ROLES[number];

export const COLLABORATION_STAGE_LIMITS = {
  summaryBytes: 16_384, childTitleCharacters: 500, childDescriptionBytes: 65_536, children: 20, contentBytes: 512 * 1024,
  decisionReasonCharacters: 2000, policyReasonCharacters: 1000, scopeChildren: 200, proposalPage: 50, proposalVersions: 50,
  smallBodyBytes: 64 * 1024, proposalBodyBytes: 2 * 1024 * 1024,
} as const;

// ---------------------------------------------------------------------------
// Read projections
// ---------------------------------------------------------------------------

/**
 * Contract section 5 as revised in 13.1: a role holder is the existing
 * collaboration content account summary (`external` is a boolean), not a
 * second account shape. A role whose account no longer resolves is null.
 */
const roleAccount = CollaborationAccountSummarySchema.nullable();

/** P1. No revision yet is the virtual, disabled revision "0". */
export const CollaborationStagePolicySchema = z.object({
  contract: z.literal(COLLABORATION_STAGE_CONTRACT),
  project_id: stageId,
  policy_revision: nonnegativeVersion,
  enabled: z.boolean(),
  reason: z.string().catch(""),
  created_by: stageId.nullable().catch(null),
  created_at: timestamp.nullable().catch(null),
  // Absent on a server that cannot say: enabling is then not offered.
  activation_available: z.boolean().optional(),
}).refine((policy) => policy.policy_revision !== "0" || policy.enabled === false, "The virtual revision is disabled");
export type CollaborationStagePolicy = z.infer<typeof CollaborationStagePolicySchema>;

const rolesSchema = z.object({
  role_revision: positiveVersion, configured_by: stageId, configured_at: timestamp,
  business_owner: roleAccount, implementer: roleAccount, product_approver: roleAccount,
  code_reviewer: roleAccount, qa_approver: roleAccount, release_approver: roleAccount,
});
export type CollaborationStageRoles = z.infer<typeof rolesSchema>;

const scopeSchema = z.object({
  scope_revision: positiveVersion, decision_id: stageId, lifecycle_epoch: positiveVersion, policy_revision: positiveVersion,
  role_revision: positiveVersion, confirmed_by: stageId, confirmed_at: timestamp, child_issue_ids: z.array(stageId).max(COLLABORATION_STAGE_LIMITS.scopeChildren),
});

/** A5 uses only the linked PR identities and full heads observed by R1. */
const scopePullRequestSchema = z.strictObject({
  provider: z.enum(["github", "vcs"]), pull_request_id: stageId,
  head_sha: z.string().regex(/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/),
});
const scopeCodeSchema = z.strictObject({ pull_request: scopePullRequestSchema }).nullable();
export type CollaborationStageScopeCode = z.infer<typeof scopeCodeSchema>;
const scopeChildInputSchema = z.strictObject({ issue_id: stageId, content_digest: digest, code: scopeCodeSchema });
export type CollaborationStageScopeChildInput = z.infer<typeof scopeChildInputSchema>;

const viewChildSchema = z.object({
  issue_id: stageId, identifier: z.string(), title: z.string(), can_open: z.boolean(),
  creator_type: z.string().catch(""),
  origin: openEnum(["materialized", "confirmed", "none"]),
  attached: z.boolean(),
  depth: z.number().int().min(1).max(64).nullable(),
  code: scopeCodeSchema,
  code_state: openEnum(["none", "current", "changed", "unavailable"]),
  pull_requests: z.array(scopePullRequestSchema),
  // Source-unreadable children expose no digest of private description text.
  content_digest: digest.nullable(),
  semantic_version: positiveVersion.nullable(),
  approved_lifecycle_epoch: positiveVersion.nullable(),
  approval_current: z.boolean(),
  execution_gated: z.boolean(),
  proposal_id: stageId.nullable(),
  ordinal: z.number().int().min(1).nullable(),
}).refine((child) => child.can_open === (child.content_digest !== null), "Only source-readable children expose a content digest")
  .refine((child) => child.attached === (child.depth !== null), "Only governed descendants have depth")
  .refine((child) => child.code_state === "unknown" || (child.code === null) === (child.code_state === "none"), "A code state needs its confirmed reference")
  .refine((child) => new Set(child.pull_requests.map((pr) => `${pr.provider}:${pr.pull_request_id}`)).size === child.pull_requests.length, "Linked pull requests are unique");
export type CollaborationStageChild = z.infer<typeof viewChildSchema>;

const missingItem = z.enum(["roles", "scope"]);

/** R1. Contradictory facts make the whole view unavailable instead of half trusted. */
export const CollaborationStageViewSchema = z.object({
  protocol_version: z.literal(COLLABORATION_STAGE_PROTOCOL_VERSION),
  issue_id: stageId,
  collaboration_revision: positiveVersion,
  lifecycle_epoch: positiveVersion,
  policy: z.object({
    contract: z.literal(COLLABORATION_STAGE_CONTRACT), project_id: stageId.nullable(),
    state: openEnum(["enabled", "disabled", "none"]), policy_revision: nonnegativeVersion,
  }),
  managed: z.boolean(),
  needs_confirmation: z.boolean(),
  missing: z.array(missingItem).max(2),
  stage: z.object({ current: openEnum(COLLABORATION_STAGES), stage_revision: positiveVersion }).nullable(),
  role_revision: nonnegativeVersion,
  roles: rolesSchema.nullable(),
  scope_revision: nonnegativeVersion,
  scope_state: openEnum(["none", "current", "lifecycle_changed", "policy_changed", "signer_changed"]),
  scope: scopeSchema.nullable(),
  children: z.object({ items: z.array(viewChildSchema).max(COLLABORATION_STAGE_LIMITS.scopeChildren), truncated: z.boolean() }),
  code_candidates: z.object({ items: z.array(z.strictObject({ candidate_id: stageId, version: positiveVersion, content_digest: digest,
    state: openEnum(["current", "superseded", "stale"]) })).max(50), truncated: z.boolean() })
    .refine((v) => new Set(v.items.map((item) => item.candidate_id)).size === v.items.length, "Discovered candidates are unique").optional(),
  // Display hints only. An absent or malformed hint is "not offered".
  capabilities: z.object({
    configure_roles: z.boolean().catch(false), create_proposal: z.boolean().catch(false),
    decide_proposal: z.boolean().catch(false), confirm_scope: z.boolean().catch(false),
  }),
}).refine((view) => view.policy.state === "unknown" || view.managed === (view.policy.state === "enabled"), "Managed follows the policy state")
  .refine((view) => view.needs_confirmation === (view.missing.length > 0) && (view.missing.length < 2 || view.missing[0] === "roles" && view.missing[1] === "scope"), "Missing items decide confirmation")
  .refine((view) => view.managed ? view.stage !== null : !view.needs_confirmation && view.stage === null, "Only a managed root has a stage")
  .refine((view) => view.roles === null ? view.role_revision === "0" : view.roles.role_revision === view.role_revision, "Role revision must describe the projected roles")
  .refine((view) => view.scope === null ? view.scope_revision === "0" : view.scope.scope_revision === view.scope_revision, "Scope revision must describe the projected scope")
  .refine((view) => view.scope_state === "unknown" || (view.scope_state === "none") === (view.scope === null), "A scope state needs its confirmation")
  .refine((view) => !view.managed || view.scope_state === "unknown" || view.missing.includes("scope") === (view.scope_state !== "current"), "Only a current scope satisfies the requirement")
  .refine((view) => new Set(view.children.items.map((child) => child.issue_id)).size === view.children.items.length, "Children are unique");
export type CollaborationStageView = z.infer<typeof CollaborationStageViewSchema>;

export const COLLABORATION_STAGE_STEPS = ["code_review", "qa", "release", "rollback"] as const;
export type CollaborationStageStep = typeof COLLABORATION_STAGE_STEPS[number];
const blockerCodes = ["stage_roles_unconfigured", "stage_scope_unconfirmed", "stage_not_reached", "stage_code_candidate_missing", "stage_candidate_stale",
  "stage_pull_request_unavailable", "stage_head_changed", "stage_trust_changed", "stage_ci_missing", "stage_ci_failed", "stage_ci_stale", "stage_ci_unsupported",
  "stage_code_review_missing", "stage_qa_missing", "stage_release_approval_missing", "stage_test_report_unusable", "stage_execution_fact_missing",
  "stage_decision_expired", "stage_decision_revoked", "stage_signer_changed", "stage_roles_changed", "stage_policy_changed", "stage_evidence_changed",
  "stage_evidence_unavailable", "stage_submission_unbound"] as const;
export const CollaborationStageBlockerSchema = z.strictObject({
  code: openEnum(blockerCodes), stage: openEnum(COLLABORATION_STAGES), candidate_id: stageId.optional(), decision_id: stageId.optional(), child_issue_id: stageId.optional(),
}).refine((v) => v.code !== "stage_evidence_unavailable" || v.candidate_id === undefined && v.decision_id === undefined && v.child_issue_id === undefined, "Unavailable evidence is anonymous");
export type CollaborationStageBlocker = z.infer<typeof CollaborationStageBlockerSchema>;
const blockersSchema = z.array(CollaborationStageBlockerSchema).max(32);
export function parseCollaborationStageBlockers(data: unknown): CollaborationStageBlocker[] | null {
  return parseWithFallback<CollaborationStageBlocker[] | null>(data, blockersSchema, null, { endpoint: "stage blockers", redact: true });
}

const boundedText = (maximum: number, minimum = 0) => z.string().refine((value) => wellFormedText(value) &&
  collaborationTextByteLength(value) >= minimum && collaborationTextByteLength(value) <= maximum);
const environmentSchema = boundedText(128, 1);
const nativeOperationId = boundedText(256, 1);
const buildSchema = z.strictObject({ artifact_digest: digest, environment: environmentSchema, target_digest: digest.optional() });
const rollbackObservationRefSchema = z.strictObject({ observation_id: stageId, observation_digest: digest });
export const CollaborationStageEvidenceSchema = z.strictObject({
  ci_digest: digest.nullable(), test_report_id: stageId.nullable(), build: buildSchema.nullable(), deployment_id: stageId.nullable(), rollback_target_digest: digest.nullable(),
  rollback_observation: rollbackObservationRefSchema.nullable().optional(),
});
export type CollaborationStageEvidence = z.infer<typeof CollaborationStageEvidenceSchema>;
const uniqueIds = z.array(stageId).max(50).refine((v) => new Set(v).size === v.length, "Identifiers are unique");
const codeDecisionSchema = z.object({
  decision_id: stageId, step: openEnum(COLLABORATION_STAGE_STEPS), decision: openEnum(["approved", "returned"]),
  state: openEnum(["valid", "expired", "revoked", "signer_changed", "roles_changed", "policy_changed", "evidence_changed", "superseded"]),
  actor_user_id: stageId, role: openEnum(["code_reviewer", "qa_approver", "release_approver"]), role_revision: positiveVersion, policy_revision: positiveVersion,
  created_at: timestamp, expires_at: timestamp.nullable(), reason: z.string(), evidence: CollaborationStageEvidenceSchema, depends_on: uniqueIds,
  revocation: z.object({ revocation_id: stageId, actor_user_id: stageId, created_at: timestamp, reason: z.string() }).nullable(),
}).refine((v) => v.decision === "unknown" || (v.decision === "approved") === (v.expires_at !== null), "Only approvals expire")
  .refine((v) => v.state !== "valid" || v.revocation === null, "A revoked decision is not valid");
export type CollaborationStageCodeDecision = z.infer<typeof codeDecisionSchema>;
const codeBaselineSchema = z.object({
  requirement_digest: digest, child_issue_id: stageId, child_semantic_version: positiveVersion,
  pull_request: scopePullRequestSchema.extend({ trust_revision: positiveVersion }),
  artifact: z.object({ approval_id: stageId, artifact_digest: digest }).nullable(),
});
const reportSummarySchema = z.object({
  report_id: stageId, run_id: stageId, attempt: positiveVersion, source_kind: z.literal("runtime_reported"),
  reported_result: openEnum(["passed", "failed", "incomplete"]), applicability: openEnum(["usable", "stale", "execution_failed", "execution_unsettled"]),
  case_count: z.number().int().min(1).max(256), failed_case_count: z.number().int().min(0).max(256), received_at: timestamp,
}).refine((v) => v.failed_case_count <= v.case_count && (v.reported_result !== "passed" || v.failed_case_count === 0), "Report counts describe its result");
export type CollaborationStageReportSummary = z.infer<typeof reportSummarySchema>;
const executionFactSchema = z.object({
  fact_id: stageId, kind: openEnum(["deployment", "rollback"]), source_kind: z.literal("integration"), environment: environmentSchema,
  operation_id: nativeOperationId, result: openEnum(["succeeded", "failed"]), artifact_digest: digest, previous_artifact_digest: digest.nullable(),
  approval_decision_id: stageId, observed_at: timestamp,
  intent_id: stageId.optional(), target_digest: digest.optional(),
});
export type CollaborationStageExecutionFact = z.infer<typeof executionFactSchema>;
const executionTargetSchema = z.object({ environment: environmentSchema, target_digest: digest, source_kind: z.literal("integration") });
const executionOperationSchema = z.object({
  intent_id: stageId, kind: openEnum(["deployment", "rollback"]), approval_decision_id: stageId, environment: environmentSchema,
  target_digest: digest, artifact_digest: digest, state: openEnum(["pending", "unknown", "not_started", "succeeded", "failed"]),
  external_operation_id: nativeOperationId.nullable(), fact_id: stageId.nullable(), created_at: timestamp,
}).refine((v) => (v.state === "succeeded" || v.state === "failed") ? v.fact_id !== null && v.external_operation_id !== null : v.fact_id === null,
  "Only confirmed terminal operations identify facts")
  .refine((v) => v.state !== "not_started" || v.external_operation_id === null, "Not-started observations have no native operation");
const rollbackObservationSchema = z.object({
  observation_id: stageId, version: positiveVersion, observation_digest: digest, deployment_id: stageId, environment: environmentSchema,
  target_digest: digest, artifact_digest: digest, condition: openEnum(["anomaly", "healthy", "unknown"]), state: openEnum(["current", "ambiguous", "superseded", "expired", "unavailable"]),
  window_start: timestamp, window_end: timestamp, observed_at: timestamp, valid_until: timestamp, summary: boundedText(2000),
}).refine((v) => Date.parse(v.window_start) <= Date.parse(v.window_end) && Date.parse(v.window_end) <= Date.parse(v.observed_at) &&
  Date.parse(v.observed_at) < Date.parse(v.valid_until), "Metric observation has an ordered finite window");
export type CollaborationStageExecutionTarget = z.infer<typeof executionTargetSchema>;
export type CollaborationStageExecutionOperation = z.infer<typeof executionOperationSchema>;
export type CollaborationStageRollbackObservation = z.infer<typeof rollbackObservationSchema>;
/** R4: evidence is separate from human decisions and external execution facts. */
export const CollaborationStageCodeCandidateSchema = z.object({
  protocol_version: z.literal(1), issue_id: stageId, checked_at: timestamp,
  candidate: z.object({
    candidate_id: stageId, version: positiveVersion, content_digest: digest, parent_candidate_id: stageId.nullable(), lifecycle_epoch: positiveVersion,
    state: openEnum(["current", "superseded", "stale"]), submitted_by: stageId, implementer_user_id: stageId, role_revision: positiveVersion,
    created_at: timestamp, baseline: codeBaselineSchema.nullable(),
  }),
  observed: z.object({
    pull_request: z.object({ state: openEnum(["current", "head_changed", "unlinked", "trust_changed", "unavailable"]), head_sha: scopePullRequestSchema.shape.head_sha.nullable() }),
    ci: z.object({ state: openEnum(["passed", "missing", "failed", "stale", "unsupported", "unavailable"]), ci_digest: digest.nullable(),
      head_sha: scopePullRequestSchema.shape.head_sha.nullable(), observed_at: timestamp.nullable(), check_count: z.number().int().nonnegative() }),
  }),
  test_reports: z.array(reportSummarySchema).max(50), decisions: z.array(codeDecisionSchema).max(50), execution_facts: z.array(executionFactSchema).max(50),
  execution_targets: z.array(executionTargetSchema).max(50).optional(),
  execution_operations: z.array(executionOperationSchema).max(50).optional(),
  rollback_observations: z.array(rollbackObservationSchema).max(50).optional(),
  signable: z.array(z.object({ step: z.enum(COLLABORATION_STAGE_STEPS), allowed: z.boolean(), max_expires_at: timestamp.nullable(), blockers: blockersSchema })
    .refine((v) => v.allowed === (v.max_expires_at !== null) && (!v.allowed || v.blockers.length === 0), "Only signable steps have a horizon")).length(4),
  blocked_evidence: z.number().int().nonnegative(),
}).refine((v) => v.signable.every((entry, index) => entry.step === COLLABORATION_STAGE_STEPS[index]), "Signable steps are complete and ordered")
  .refine((v) => v.observed.ci.state !== "passed" || v.observed.ci.ci_digest !== null && v.observed.ci.head_sha !== null && v.observed.ci.observed_at !== null &&
    v.observed.ci.check_count > 0 && Date.parse(v.observed.ci.observed_at) <= Date.parse(v.checked_at) && Date.parse(v.checked_at) - Date.parse(v.observed.ci.observed_at) <= 30 * 60_000, "Passed CI needs a fresh complete observation")
  .refine((v) => v.candidate.baseline === null ? v.blocked_evidence === 1 && v.observed.pull_request.state === "unavailable" && v.observed.pull_request.head_sha === null &&
    v.observed.ci.state === "unavailable" && v.observed.ci.ci_digest === null && v.observed.ci.head_sha === null && v.observed.ci.observed_at === null && v.observed.ci.check_count === 0 &&
    v.test_reports.length === 0 && v.decisions.length === 0 && v.execution_facts.length === 0 && (v.execution_targets?.length ?? 0) === 0 &&
    (v.execution_operations?.length ?? 0) === 0 && (v.rollback_observations?.length ?? 0) === 0 && v.signable.every((entry) => !entry.allowed && entry.max_expires_at === null &&
      entry.blockers.length === 1 && entry.blockers[0]?.code === "stage_evidence_unavailable") : v.blocked_evidence === 0, "Source-denied evidence has one anonymous withheld group")
  .refine((v) => v.execution_targets === undefined ? v.execution_operations === undefined && v.rollback_observations === undefined :
    v.execution_operations !== undefined && v.rollback_observations !== undefined && v.execution_facts.every((f) => f.intent_id !== undefined && f.target_digest !== undefined),
  "2c execution projections are complete; older omission is unknown")
  .refine((v) => (v.execution_targets === undefined || new Set(v.execution_targets.map((t) => t.target_digest)).size === v.execution_targets.length) &&
    (v.execution_operations === undefined || new Set(v.execution_operations.map((o) => o.intent_id)).size === v.execution_operations.length &&
      new Set(v.execution_operations.map((o) => o.approval_decision_id)).size === v.execution_operations.length) &&
    (v.rollback_observations === undefined || new Set(v.rollback_observations.map((o) => o.observation_id)).size === v.rollback_observations.length), "Execution records are unique")
  .refine((v) => v.execution_targets === undefined ||
    new Set(v.execution_facts.map((f) => f.intent_id)).size === v.execution_facts.length &&
    new Set(v.execution_facts.map((f) => f.approval_decision_id)).size === v.execution_facts.length,
  "Each execution intent and approval has at most one terminal fact")
  .refine((v) => v.execution_operations === undefined || v.execution_operations.every((operation) => {
    const fact = v.execution_facts.find((f) => f.fact_id === operation.fact_id || f.intent_id === operation.intent_id || f.approval_decision_id === operation.approval_decision_id);
    return !fact || fact.fact_id === operation.fact_id && fact.intent_id === operation.intent_id && fact.kind === operation.kind && fact.result === operation.state &&
      fact.operation_id === operation.external_operation_id && fact.target_digest === operation.target_digest && fact.artifact_digest === operation.artifact_digest &&
      fact.environment === operation.environment && fact.approval_decision_id === operation.approval_decision_id;
  }), "Overlapping operation and fact projections agree")
  .refine((v) => new Set(v.decisions.map((d) => d.decision_id)).size === v.decisions.length &&
    new Set(v.test_reports.map((r) => r.report_id)).size === v.test_reports.length && new Set(v.execution_facts.map((f) => f.fact_id)).size === v.execution_facts.length, "Evidence records are unique");
export type CollaborationStageCodeCandidate = z.infer<typeof CollaborationStageCodeCandidateSchema>;
export function parseCollaborationStageCodeCandidate(data: unknown): CollaborationStageCodeCandidate | null {
  return parseWithFallback<CollaborationStageCodeCandidate | null>(data, CollaborationStageCodeCandidateSchema, null, { endpoint: "GET stage code candidate", redact: true });
}

const proposalChildSchema = z.object({ title: z.string(), description: z.string(), priority: z.string() });
export const CollaborationStageProposalContentSchema = z.object({
  summary: z.string(), children: z.array(proposalChildSchema).min(1).max(COLLABORATION_STAGE_LIMITS.children),
});
export type CollaborationStageProposalContent = z.infer<typeof CollaborationStageProposalContentSchema>;

/** Server-stated provenance. A client can never state an agent author. */
const proposalSourceSchema = z.union([
  z.object({ kind: z.literal("manual") }),
  z.object({ kind: z.literal("comment"), comment_id: stageId, comment_revision: positiveVersion,
    author_type: z.string().optional().catch(undefined), author_id: stageId.optional().catch(undefined), digest: digest.optional().catch(undefined) }),
  z.object({ kind: z.literal("run"), run_id: stageId, agent_id: stageId.optional().catch(undefined) }),
  z.object({ kind: z.string() }).transform(() => ({ kind: "unknown" as const })),
]);
export type CollaborationStageProposalSource = z.infer<typeof proposalSourceSchema>;

const candidateState = openEnum(["pending", "superseded", "rejected", "accepted"]);
export const CollaborationStageCandidateSchema = z.object({
  candidate_id: stageId, version: positiveVersion, content_digest: digest, parent_candidate_id: stageId.nullable(),
  lifecycle_epoch: positiveVersion, state: candidateState, submitted_by: stageId, created_at: timestamp,
  content: CollaborationStageProposalContentSchema, source: proposalSourceSchema,
});
export type CollaborationStageCandidate = z.infer<typeof CollaborationStageCandidateSchema>;

export const CollaborationStageDecisionSchema = z.object({
  decision_id: stageId, step: openEnum(["proposal"]), decision: openEnum(["accepted", "rejected"]),
  candidate_id: stageId, version: positiveVersion, content_digest: digest,
  role: z.string(), role_revision: nonnegativeVersion, policy_revision: positiveVersion, lifecycle_epoch: positiveVersion,
  actor_user_id: stageId, reason: z.string().catch(""), created_at: timestamp,
  accepted_with_modification: z.boolean().catch(false),
});
export type CollaborationStageDecision = z.infer<typeof CollaborationStageDecisionSchema>;

const proposalState = openEnum(["pending", "rejected", "accepted"]);
const proposalListItemSchema = z.object({
  proposal_id: stageId, state: proposalState, version_count: positiveVersion, created_at: timestamp,
  current: z.object({
    candidate_id: stageId, version: positiveVersion, content_digest: digest, parent_candidate_id: stageId.nullable(),
    lifecycle_epoch: positiveVersion, stale: z.boolean(), submitted_by: stageId, created_at: timestamp,
    summary: z.string(), child_count: z.number().int().min(0).max(COLLABORATION_STAGE_LIMITS.children),
  }),
  decision: CollaborationStageDecisionSchema.nullable(),
}).refine((item) => item.decision === null || item.decision.candidate_id === item.current.candidate_id, "A list decision belongs to the newest version");
export type CollaborationStageProposalListItem = z.infer<typeof proposalListItemSchema>;

/** R2. */
export const CollaborationStageProposalPageSchema = z.object({
  issue_id: stageId,
  items: z.array(proposalListItemSchema).max(COLLABORATION_STAGE_LIMITS.proposalPage),
  next_cursor: z.string().min(1).max(1024).nullable(),
}).refine((page) => new Set(page.items.map((item) => item.proposal_id)).size === page.items.length, "A page never repeats a proposal");
export type CollaborationStageProposalPage = z.infer<typeof CollaborationStageProposalPageSchema>;

const materializedChildSchema = z.object({
  ordinal: z.number().int().min(1).max(COLLABORATION_STAGE_LIMITS.children), issue_id: stageId,
  identifier: z.string(), title: z.string(), can_open: z.boolean(), attached: z.boolean(),
});

/** R3. The child mapping is read here under current authority, never from a receipt. */
export const CollaborationStageProposalDetailSchema = z.object({
  issue_id: stageId, proposal_id: stageId, state: proposalState,
  versions: z.array(CollaborationStageCandidateSchema).min(1).max(COLLABORATION_STAGE_LIMITS.proposalVersions),
  versions_truncated: z.boolean(),
  decisions: z.array(CollaborationStageDecisionSchema),
  children: z.array(materializedChildSchema).max(COLLABORATION_STAGE_LIMITS.children),
}).refine((detail) => detail.versions.every((version, index) => index === 0 || BigInt(version.version) < BigInt(detail.versions[index - 1]!.version)), "Versions are newest first")
  .refine((detail) => new Set(detail.versions.map((version) => version.candidate_id)).size === detail.versions.length, "Versions are unique")
  .refine((detail) => new Set(detail.decisions.map((decision) => decision.decision_id)).size === detail.decisions.length, "Decisions are unique")
  .refine((detail) => new Set(detail.children.map((child) => child.ordinal)).size === detail.children.length &&
    new Set(detail.children.map((child) => child.issue_id)).size === detail.children.length, "Materialized children are unique")
  .refine((detail) => detail.children.length === 0 || detail.state === "accepted", "Only an accepted proposal has children");
export type CollaborationStageProposalDetail = z.infer<typeof CollaborationStageProposalDetailSchema>;

const redacted = { redact: true } as const;
export function parseCollaborationStagePolicy(data: unknown): CollaborationStagePolicy | null {
  return parseWithFallback<CollaborationStagePolicy | null>(data, CollaborationStagePolicySchema, null, { endpoint: "GET project stage policy", ...redacted });
}
export function parseCollaborationStageView(data: unknown): CollaborationStageView | null {
  return parseWithFallback<CollaborationStageView | null>(data, CollaborationStageViewSchema, null, { endpoint: "GET collaboration stage", ...redacted });
}
export function parseCollaborationStageProposalPage(data: unknown): CollaborationStageProposalPage | null {
  return parseWithFallback<CollaborationStageProposalPage | null>(data, CollaborationStageProposalPageSchema, null, { endpoint: "GET collaboration stage proposals", ...redacted });
}
export function parseCollaborationStageProposalDetail(data: unknown): CollaborationStageProposalDetail | null {
  return parseWithFallback<CollaborationStageProposalDetail | null>(data, CollaborationStageProposalDetailSchema, null, { endpoint: "GET collaboration stage proposal", ...redacted });
}

/** A read never becomes "passed" by being absent: every non-available state disables the actions. */
export type CollaborationStageRead<T> =
  | { state: "available"; value: T }
  /** The route does not exist on this server (older build, or an increment that is not served yet). */
  | { state: "unsupported" }
  | { state: "unavailable"; reason: "schema" | "busy" | "malformed" }
  /** A proposal or project that does not exist here; not a statement about issue access. */
  | { state: "not_found" }
  /** The viewer is not a current human member of the source workspace (project policy only). */
  | { state: "denied" };

// ---------------------------------------------------------------------------
// Requests (increment 1). The client freezes the key and the exact body.
// ---------------------------------------------------------------------------

const observations = {
  expected_collaboration_revision: positiveVersion, expected_lifecycle_epoch: positiveVersion,
  expected_participant_epoch: positiveVersion, expected_account_epoch: positiveVersion,
};
const reasonText = (min: number, max: number) => z.string().transform((value) => value.trim())
  .refine((value) => wellFormedText(value) && characters(value) >= min && characters(value) <= max);

export const CollaborationStageCandidateRefSchema = z.strictObject({ candidate_id: stageId, version: positiveVersion, content_digest: digest });
export type CollaborationStageCandidateRef = z.infer<typeof CollaborationStageCandidateRefSchema>;

export const CollaborationStageProposalContentInputSchema = z.strictObject({
  summary: boundedText(COLLABORATION_STAGE_LIMITS.summaryBytes),
  children: z.array(z.strictObject({
    title: z.string().transform((value) => value.trim()).refine((value) => wellFormedText(value) && value !== "" && characters(value) <= COLLABORATION_STAGE_LIMITS.childTitleCharacters),
    description: boundedText(COLLABORATION_STAGE_LIMITS.childDescriptionBytes),
    priority: z.enum(COLLABORATION_STAGE_PRIORITIES),
  })).min(1).max(COLLABORATION_STAGE_LIMITS.children),
}).refine((content) => collaborationTextByteLength(JSON.stringify(content)) <= COLLABORATION_STAGE_LIMITS.contentBytes, "Proposal content is too large");
export type CollaborationStageProposalContentInput = z.infer<typeof CollaborationStageProposalContentInputSchema>;

/** Exactly one shape; server fields such as an agent author are never sent. */
export const CollaborationStageProposalSourceInputSchema = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("manual") }),
  z.strictObject({ kind: z.literal("comment"), comment_id: stageId, comment_revision: positiveVersion }),
  z.strictObject({ kind: z.literal("run"), run_id: stageId }),
]);
export type CollaborationStageProposalSourceInput = z.infer<typeof CollaborationStageProposalSourceInputSchema>;

export const CollaborationStageRolesInputSchema = z.strictObject({
  business_owner_user_id: stageId, implementer_user_id: stageId, product_approver_user_id: stageId,
  code_reviewer_user_id: stageId, qa_approver_user_id: stageId, release_approver_user_id: stageId,
});
export type CollaborationStageRolesInput = z.infer<typeof CollaborationStageRolesInputSchema>;

/** The two separations the server enforces; checked locally so a key is not spent on them. */
export function collaborationStageRoleConflicts(roles: Partial<CollaborationStageRolesInput>): ("self_review" | "qa_release")[] {
  const same = (a?: string, b?: string) => a !== undefined && b !== undefined && a !== "" && a.toLowerCase() === b.toLowerCase();
  return [
    ...(same(roles.implementer_user_id, roles.code_reviewer_user_id) ? ["self_review" as const] : []),
    ...(same(roles.qa_approver_user_id, roles.release_approver_user_id) ? ["qa_release" as const] : []),
  ];
}

export const CollaborationStageRequestSchemas = {
  configure_roles: z.strictObject({ ...observations, expected_policy_revision: positiveVersion, expected_role_revision: nonnegativeVersion,
    roles: CollaborationStageRolesInputSchema }).refine((body) => collaborationStageRoleConflicts(body.roles).length === 0, "Role separation"),
  create_proposal: z.strictObject({ ...observations, expected_policy_revision: positiveVersion,
    content: CollaborationStageProposalContentInputSchema, source: CollaborationStageProposalSourceInputSchema }),
  revise_proposal: z.strictObject({ ...observations, expected_policy_revision: positiveVersion, expected_candidate: CollaborationStageCandidateRefSchema,
    content: CollaborationStageProposalContentInputSchema, source: CollaborationStageProposalSourceInputSchema }),
  decide_proposal: z.strictObject({ ...observations, expected_policy_revision: positiveVersion, expected_role_revision: nonnegativeVersion,
    candidate: CollaborationStageCandidateRefSchema, decision: z.enum(["accept", "reject"]),
    reason: reasonText(0, COLLABORATION_STAGE_LIMITS.decisionReasonCharacters) })
    .refine((body) => body.decision === "accept" || body.reason !== "", "A rejection needs a reason"),
  confirm_scope: z.strictObject({ ...observations, expected_policy_revision: positiveVersion, expected_role_revision: nonnegativeVersion,
    expected_scope_revision: nonnegativeVersion,
    children: z.array(scopeChildInputSchema).max(COLLABORATION_STAGE_LIMITS.scopeChildren),
    reason: reasonText(0, COLLABORATION_STAGE_LIMITS.decisionReasonCharacters) })
    .refine((body) => new Set(body.children.map((child) => child.issue_id)).size === body.children.length, "Scope children are unique"),
  create_code_candidate: z.strictObject({ ...observations, expected_policy_revision: positiveVersion, expected_role_revision: positiveVersion,
    child_issue_id: stageId, supersedes: CollaborationStageCandidateRefSchema.nullable(), pull_request: scopePullRequestSchema,
    artifact: z.strictObject({ approval_id: stageId }).nullable() }),
  decide_stage: z.strictObject({ ...observations, expected_policy_revision: positiveVersion, expected_role_revision: positiveVersion, expected_stage_revision: positiveVersion,
    step: z.enum(COLLABORATION_STAGE_STEPS), decision: z.enum(["approve", "return"]), candidate: CollaborationStageCandidateRefSchema,
    evidence: CollaborationStageEvidenceSchema, depends_on: uniqueIds, reason: reasonText(0, COLLABORATION_STAGE_LIMITS.decisionReasonCharacters), expires_at: expiryTimestamp.nullable(),
  }).refine((v) => v.decision === "return" ? v.expires_at === null && v.reason !== "" : v.expires_at !== null, "Approvals expire; returns require a reason")
    .refine((v) => v.decision === "return" || (v.step === "code_review" ? v.evidence.ci_digest !== null : v.step === "qa" ? v.evidence.test_report_id !== null && v.depends_on.length > 0 :
      v.step === "release" ? v.evidence.build?.target_digest !== undefined && v.depends_on.length === 1 :
        v.evidence.deployment_id !== null && v.evidence.rollback_target_digest !== null && v.evidence.rollback_observation != null && v.depends_on.length === 1), "Required evidence per step")
    .refine((v) => (v.step === "rollback" || v.evidence.rollback_observation == null) &&
      (v.step === "release" || v.evidence.build?.target_digest === undefined) &&
      (v.step !== "release" || v.evidence.deployment_id === null && v.evidence.rollback_target_digest === null) &&
      (v.step !== "rollback" || v.evidence.build === null), "Evidence belongs to the selected step"),
  revoke_decision: z.strictObject({ ...observations, expected_policy_revision: positiveVersion, expected_role_revision: positiveVersion,
    reason: reasonText(1, COLLABORATION_STAGE_LIMITS.decisionReasonCharacters) }),
  advance_stage: z.strictObject({ ...observations, expected_policy_revision: positiveVersion, expected_role_revision: positiveVersion, expected_stage_revision: positiveVersion,
    target_stage: z.enum(COLLABORATION_STAGES), candidate: CollaborationStageCandidateRefSchema, decision_ids: uniqueIds }),
  revise_policy: z.strictObject({ expected_policy_revision: nonnegativeVersion, enabled: z.boolean(),
    reason: reasonText(1, COLLABORATION_STAGE_LIMITS.policyReasonCharacters) }),
} as const;

export type CollaborationStageIssueAction = typeof COLLABORATION_STAGE_ISSUE_ACTIONS[number];
export type CollaborationStageRequestAction = CollaborationStageIssueAction | "revise_policy";
export type CollaborationStageRequestBody<A extends CollaborationStageRequestAction = CollaborationStageRequestAction> = z.infer<typeof CollaborationStageRequestSchemas[A]>;
export const COLLABORATION_STAGE_ISSUE_ACTIONS = ["configure_roles", "create_proposal", "revise_proposal", "decide_proposal", "confirm_scope", "create_code_candidate", "decide_stage", "revoke_decision", "advance_stage"] as const;

/** Decoded JSON bytes on the wire (contract 2.8). */
export function collaborationStageBodyLimit(action: CollaborationStageRequestAction): number {
  return action === "create_proposal" || action === "revise_proposal" ? COLLABORATION_STAGE_LIMITS.proposalBodyBytes
    : action === "create_code_candidate" || action === "decide_stage" ? 256 * 1024 : COLLABORATION_STAGE_LIMITS.smallBodyBytes;
}

/** The frozen request a pending intent stands for: routing identity plus the exact decoded body. */
export type CollaborationStageFrozenRequest =
  | { action: "configure_roles"; issueId: string; body: CollaborationStageRequestBody<"configure_roles"> }
  | { action: "create_proposal"; issueId: string; body: CollaborationStageRequestBody<"create_proposal"> }
  | { action: "revise_proposal"; issueId: string; proposalId: string; body: CollaborationStageRequestBody<"revise_proposal"> }
  | { action: "decide_proposal"; issueId: string; proposalId: string; body: CollaborationStageRequestBody<"decide_proposal"> }
  | { action: "confirm_scope"; issueId: string; body: CollaborationStageRequestBody<"confirm_scope"> }
  | { action: "create_code_candidate"; issueId: string; body: CollaborationStageRequestBody<"create_code_candidate"> }
  | { action: "decide_stage"; issueId: string; body: CollaborationStageRequestBody<"decide_stage"> }
  | { action: "revoke_decision"; issueId: string; decisionId: string; body: CollaborationStageRequestBody<"revoke_decision"> }
  | { action: "advance_stage"; issueId: string; body: CollaborationStageRequestBody<"advance_stage"> }
  | { action: "revise_policy"; workspaceId: string; projectId: string; body: CollaborationStageRequestBody<"revise_policy"> };

/** Only the identity used by the one receipt matcher; never a reconstructable write payload. */
export const CollaborationStageConfirmationIdentitySchema = z.discriminatedUnion("action", [
  z.strictObject({ action: z.literal("revise_policy"), workspaceId: stageId, projectId: stageId,
    body: z.strictObject({ expected_policy_revision: nonnegativeVersion, enabled: z.boolean() }) }),
  z.strictObject({ action: z.literal("configure_roles"), issueId: stageId, body: z.strictObject({ expected_role_revision: nonnegativeVersion }) }),
  z.strictObject({ action: z.literal("create_proposal"), issueId: stageId }),
  z.strictObject({ action: z.literal("revise_proposal"), issueId: stageId, proposalId: stageId,
    body: z.strictObject({ expected_candidate: CollaborationStageCandidateRefSchema }) }),
  z.strictObject({ action: z.literal("decide_proposal"), issueId: stageId, proposalId: stageId,
    body: z.strictObject({ candidate: CollaborationStageCandidateRefSchema, decision: z.enum(["accept", "reject"]), expected_role_revision: nonnegativeVersion, expected_policy_revision: positiveVersion }) }),
  z.strictObject({ action: z.literal("confirm_scope"), issueId: stageId,
    body: z.strictObject({ expected_lifecycle_epoch: positiveVersion, expected_scope_revision: nonnegativeVersion }) }),
  z.strictObject({ action: z.literal("create_code_candidate"), issueId: stageId,
    body: z.strictObject({ supersedes: CollaborationStageCandidateRefSchema.nullable() }) }),
  z.strictObject({ action: z.literal("decide_stage"), issueId: stageId,
    body: z.strictObject({ candidate: CollaborationStageCandidateRefSchema, step: z.enum(COLLABORATION_STAGE_STEPS), decision: z.enum(["approve", "return"]),
      expected_role_revision: positiveVersion, expected_policy_revision: positiveVersion, expires_at: expiryTimestamp.nullable() }) }),
  z.strictObject({ action: z.literal("revoke_decision"), issueId: stageId, decisionId: stageId }),
  z.strictObject({ action: z.literal("advance_stage"), issueId: stageId,
    body: z.strictObject({ target_stage: z.enum(COLLABORATION_STAGES), expected_stage_revision: positiveVersion }) }),
]);
export type CollaborationStageConfirmationIdentity = z.infer<typeof CollaborationStageConfirmationIdentitySchema>;
export function collaborationStageConfirmationIdentity(request: CollaborationStageFrozenRequest): CollaborationStageConfirmationIdentity {
  const action = request.action;
  if (action === "revise_policy") return { action, workspaceId: request.workspaceId, projectId: request.projectId,
    body: { expected_policy_revision: request.body.expected_policy_revision, enabled: request.body.enabled } };
  const issueId = request.issueId;
  switch (action) {
    case "configure_roles": return { action, issueId, body: { expected_role_revision: request.body.expected_role_revision } };
    case "create_proposal": return { action, issueId };
    case "revise_proposal": return { action, issueId, proposalId: request.proposalId, body: { expected_candidate: request.body.expected_candidate } };
    case "decide_proposal": return { action, issueId, proposalId: request.proposalId, body: { candidate: request.body.candidate, decision: request.body.decision,
      expected_role_revision: request.body.expected_role_revision, expected_policy_revision: request.body.expected_policy_revision } };
    case "confirm_scope": return { action, issueId, body: { expected_lifecycle_epoch: request.body.expected_lifecycle_epoch, expected_scope_revision: request.body.expected_scope_revision } };
    case "create_code_candidate": return { action, issueId, body: { supersedes: request.body.supersedes } };
    case "decide_stage": return { action, issueId, body: { candidate: request.body.candidate, step: request.body.step, decision: request.body.decision,
      expected_role_revision: request.body.expected_role_revision, expected_policy_revision: request.body.expected_policy_revision, expires_at: request.body.expires_at } };
    case "revoke_decision": return { action, issueId, decisionId: request.decisionId };
    case "advance_stage": return { action, issueId, body: { target_stage: request.body.target_stage, expected_stage_revision: request.body.expected_stage_revision } };
  }
}

// ---------------------------------------------------------------------------
// stage_confirmation: the closed union of contract section 11 (all ten variants).
// ---------------------------------------------------------------------------

const candidateFields = { candidate_id: stageId, version: positiveVersion, content_digest: digest };
export const CollaborationStageConfirmationSchema = z.discriminatedUnion("action", [
  z.strictObject({ action: z.literal("revise_policy"), project_id: stageId, policy_revision: positiveVersion, enabled: z.boolean() }),
  z.strictObject({ action: z.literal("configure_roles"), issue_id: stageId, role_revision: positiveVersion }),
  z.strictObject({ action: z.literal("create_proposal"), issue_id: stageId, proposal_id: stageId, ...candidateFields }),
  z.strictObject({ action: z.literal("revise_proposal"), issue_id: stageId, proposal_id: stageId, ...candidateFields, parent_candidate_id: stageId }),
  z.strictObject({ action: z.literal("decide_proposal"), issue_id: stageId, proposal_id: stageId, ...candidateFields,
    decision: z.enum(["accepted", "rejected"]), decision_id: stageId, role_revision: positiveVersion, policy_revision: positiveVersion }),
  z.strictObject({ action: z.literal("confirm_scope"), issue_id: stageId, decision_id: stageId, scope_revision: positiveVersion, lifecycle_epoch: positiveVersion }),
  // Increment 2 variants are frozen and parsed; no increment-1 request produces them.
  z.strictObject({ action: z.literal("create_code_candidate"), issue_id: stageId, ...candidateFields, parent_candidate_id: stageId.nullable() }),
  z.strictObject({ action: z.literal("decide_stage"), issue_id: stageId, step: z.enum(["code_review", "qa", "release", "rollback"]),
    decision: z.enum(["approved", "returned"]), ...candidateFields, decision_id: stageId,
    role_revision: positiveVersion, policy_revision: positiveVersion, expires_at: timestamp.nullable() }),
  z.strictObject({ action: z.literal("revoke_decision"), issue_id: stageId, decision_id: stageId, revocation_id: stageId }),
  z.strictObject({ action: z.literal("advance_stage"), issue_id: stageId, stage: z.enum(COLLABORATION_STAGES), stage_revision: positiveVersion }),
]);
export type CollaborationStageConfirmation = z.infer<typeof CollaborationStageConfirmationSchema>;
export type CollaborationStageConfirmationAction = CollaborationStageConfirmation["action"];

/** Success code(s) per confirmation variant (contract section 11). */
export const COLLABORATION_STAGE_SUCCESS_CODES: Record<CollaborationStageConfirmationAction, readonly string[]> = {
  revise_policy: ["stage_policy_revised"],
  configure_roles: ["stage_roles_configured", "stage_roles_unchanged"],
  create_proposal: ["stage_proposal_created"],
  revise_proposal: ["stage_proposal_revised"],
  decide_proposal: ["stage_proposal_accepted", "stage_proposal_rejected"],
  confirm_scope: ["stage_scope_confirmed"],
  create_code_candidate: ["stage_code_candidate_created"],
  decide_stage: ["stage_decision_recorded"],
  revoke_decision: ["stage_decision_revoked"],
  advance_stage: ["stage_advanced"],
};

/** Refusals committed with the operation: final for that key (contract 3.3, 3.6, 7.x, 8.x). */
const STORED_REFUSALS: Record<string, number> = {
  forbidden: 403, stale_authority: 409, stale_revision: 409, stage_policy_disabled: 409, stage_policy_changed: 409,
  stage_roles_unconfigured: 409, stage_roles_changed: 409, stage_role_required: 403, issue_closed: 409,
  stage_configuration_denied: 403, stage_self_review_forbidden: 409, stage_qa_release_conflict: 409, stage_role_target_unavailable: 409,
  stage_source_unavailable: 409, stage_proposal_not_found: 404, stage_candidate_changed: 409, stage_proposal_already_materialized: 409,
  stage_proposal_unchanged: 409, stage_candidate_not_pending: 409, stage_candidate_stale: 409, stage_needs_confirmation: 409,
  stage_source_creation_denied: 403, stage_child_creation_refused: 409, stage_scope_changed: 409, stage_scope_child_unavailable: 409,
  stage_scope_code_unavailable: 409, stage_incomplete: 409, stage_policy_exit_required: 409,
  // Increment 2 (frozen).
  stage_child_unconfirmed: 409, stage_pull_request_unavailable: 409, stage_head_changed: 409, stage_trust_changed: 409,
  stage_artifact_unavailable: 409, stage_ci_missing: 409, stage_ci_failed: 409, stage_ci_stale: 409, stage_ci_unsupported: 409,
  stage_evidence_changed: 409, stage_evidence_unavailable: 409, stage_dependency_invalid: 409, stage_test_report_unusable: 409,
  stage_execution_fact_missing: 409, stage_stage_changed: 409, stage_decision_expired: 409, stage_signer_changed: 409,
  stage_decision_not_found: 404, stage_decision_not_revocable: 409, stage_transition_invalid: 409,
};
/** Issue-action refusals without an operation row (contract 3.4): this attempt wrote nothing. */
const UNSTORED_ISSUE_REFUSALS: Record<string, number> = {
  invalid_request: 400, invalid_version: 400, invalid_request_id: 400, invalid_stage_request: 400, stage_expiry_exceeds_limit: 400,
  idempotency_conflict: 409, request_too_large: 413, json_required: 415, collaboration_busy: 503, stage_schema_unavailable: 503,
};
/** Project policy refusals (contract 4.2): never stored; the revision table holds successes only. */
const POLICY_REFUSALS: Record<string, number> = {
  invalid_stage_policy_request: 400, invalid_request_id: 400, stage_policy_denied: 403, stage_policy_management_denied: 403,
  stage_policy_project_not_found: 404, stage_policy_revision_conflict: 409, idempotency_conflict: 409, stage_policy_busy: 409,
  stage_policy_unavailable: 503, stage_schema_unavailable: 503, request_too_large: 413, json_required: 415,
};
/** Synthetic code for a write route that this server does not register. */
export const COLLABORATION_STAGE_ROUTE_UNAVAILABLE = "stage_route_unavailable";

/** HTTP status a stored stage refusal must carry, or null for a code this client does not know. */
export function collaborationStageRefusalStatus(code: string): number | null {
  return Object.hasOwn(STORED_REFUSALS, code) ? STORED_REFUSALS[code]! : null;
}

export const CollaborationStageRefusalFieldsSchema = z.object({
  stage_blockers: blockersSchema.min(1).optional(), stage_blockers_truncated: z.boolean().optional(),
});
/** Shared by stage, ordinary collaboration and administration boundaries. */
export function parseCollaborationStageRefusalFields(body: unknown, code: string): { blockers?: CollaborationStageBlocker[]; blockersTruncated?: boolean } | null {
  const parsed = CollaborationStageRefusalFieldsSchema.safeParse(body);
  if (!parsed.success) return null;
  const { stage_blockers, stage_blockers_truncated } = parsed.data;
  if (code !== "stage_incomplete") return stage_blockers === undefined && stage_blockers_truncated === undefined ? {} : null;
  return stage_blockers !== undefined && stage_blockers_truncated !== undefined ? { blockers: stage_blockers, blockersTruncated: stage_blockers_truncated } : null;
}
/** An invalid stage payload is unavailable, never an absent legacy operation. */
export function hasCollaborationStageRefusalData(body: unknown): boolean {
  return typeof body === "object" && body !== null && (Object.hasOwn(body, "stage_blockers") || Object.hasOwn(body, "stage_blockers_truncated") ||
    Object.hasOwn(body, "stage_confirmation") || "code" in body && (body.code === "stage_incomplete" || body.code === "stage_policy_exit_required"));
}

const refusalCode = z.string().min(1).max(128).regex(/^[a-z0-9_]+$/);
const issueSuccessSchema = z.object({
  code: refusalCode, collaboration_revision: positiveVersion, stage_confirmation: CollaborationStageConfirmationSchema,
});
const storedRefusalSchema = z.object({
  code: refusalCode, collaboration_revision: positiveVersion, status: z.number().int().min(400).max(599),
});
const policySuccessSchema = z.object({ code: refusalCode, stage_confirmation: CollaborationStageConfirmationSchema });

export type CollaborationStageWriteResult =
  | { type: "success"; code: string; collaborationRevision: string | null; confirmation: CollaborationStageConfirmation }
  /** `stored`: final for the key. Otherwise this attempt wrote nothing (contract 3.4 / 4.2). */
  | { type: "refusal"; stored: boolean; status: number; code: string; collaborationRevision: string | null; blockers?: CollaborationStageBlocker[]; blockersTruncated?: boolean }
  /** O1 409 `operation_incomplete`: not terminal. */
  | { type: "incomplete" }
  /** O1 / P3 404: the original result is unknown. Never a reason to write again automatically. */
  | { type: "not_found" }
  | { type: "unknown"; reason: "malformed" | "unavailable"; status: number | null; code: string | null }
  /** Not a stage outcome: the transport must surface the original access or session error. */
  | { type: "denied" };

/** What the transport hands to its caller: access answers are thrown, never returned. */
export type CollaborationStageSettledResult = Exclude<CollaborationStageWriteResult, { type: "denied" }>;

export type CollaborationStageTransportMode = "write" | "recovery";
export type CollaborationStageTransportTarget = "issue" | "policy";

const hasOwn = (body: unknown, key: string) => typeof body === "object" && body !== null && Object.hasOwn(body, key);
const malformed = (status: number | null, code: string | null = null): CollaborationStageWriteResult => ({ type: "unknown", reason: "malformed", status, code });
function bodyCode(body: unknown): string | null {
  const parsed = z.object({ code: refusalCode }).safeParse(body);
  return parsed.success ? parsed.data.code : null;
}

function successResult(code: string, confirmation: CollaborationStageConfirmation, collaborationRevision: string | null, target: CollaborationStageTransportTarget): CollaborationStageWriteResult {
  // A success names exactly the variant its code belongs to, and a project
  // confirmation never answers an issue operation (or the reverse).
  if (!COLLABORATION_STAGE_SUCCESS_CODES[confirmation.action].includes(code) || (confirmation.action === "revise_policy") !== (target === "policy")) return malformed(200, code);
  return { type: "success", code, collaborationRevision, confirmation };
}

/** Interpret an HTTP 2xx body of a stage write or of its original-key recovery. */
export function classifyCollaborationStageResponse(status: number, body: unknown, mode: CollaborationStageTransportMode, target: CollaborationStageTransportTarget): CollaborationStageWriteResult {
  if (status !== 200) return malformed(status, bodyCode(body));
  if (target === "policy") {
    const parsed = policySuccessSchema.safeParse(body);
    return parsed.success && !hasOwn(body, "status") ? successResult(parsed.data.code, parsed.data.stage_confirmation, null, target) : malformed(status, bodyCode(body));
  }
  if (hasOwn(body, "stage_confirmation")) {
    const parsed = issueSuccessSchema.safeParse(body);
    // A confirmation next to a refusal status is contradictory, not a success.
    return parsed.success && !hasOwn(body, "status") ? successResult(parsed.data.code, parsed.data.stage_confirmation, parsed.data.collaboration_revision, target) : malformed(status, bodyCode(body));
  }
  // Only the recovery read reports a stored refusal inside a 200.
  const refusal = storedRefusalSchema.safeParse(body);
  if (mode !== "recovery" || !refusal.success) return malformed(status, bodyCode(body));
  const expected = collaborationStageRefusalStatus(refusal.data.code);
  if (expected !== null && expected !== refusal.data.status) return malformed(status, refusal.data.code);
  const extras = parseCollaborationStageRefusalFields(body, refusal.data.code);
  return extras === null ? malformed(status, refusal.data.code) : { type: "refusal", stored: true, status: refusal.data.status, code: refusal.data.code, collaborationRevision: refusal.data.collaboration_revision, ...extras };
}

/** Interpret an HTTP error of a stage write or of its original-key recovery. */
export function classifyCollaborationStageError(status: number, body: unknown, mode: CollaborationStageTransportMode, target: CollaborationStageTransportTarget): CollaborationStageWriteResult {
  const code = bodyCode(body);
  if (status === 401) return { type: "denied" };
  if (hasOwn(body, "stage_confirmation")) return malformed(status, code);
  if (mode === "recovery") {
    if (status === 404) return { type: "not_found" };
    if (target === "issue" && status === 409 && code === "operation_incomplete") return { type: "incomplete" };
    if (target === "issue" && status === 403) return { type: "denied" };
    return { type: "unknown", reason: status >= 500 ? "unavailable" : "malformed", status, code };
  }
  if (target === "policy") {
    if (code !== null && Object.hasOwn(POLICY_REFUSALS, code)) {
      return POLICY_REFUSALS[code] === status ? { type: "refusal", stored: false, status, code, collaborationRevision: null } : malformed(status, code);
    }
    if (code === null && (status === 404 || status === 405)) return { type: "refusal", stored: false, status, code: COLLABORATION_STAGE_ROUTE_UNAVAILABLE, collaborationRevision: null };
    // Workspace middleware and unfamiliar refusals: nothing proves a write, nothing proves its absence.
    return { type: "unknown", reason: status >= 500 ? "unavailable" : "malformed", status, code };
  }
  // A committed refusal declares itself with its own status and revision. It
  // says nothing about the caller's independent access to the issue.
  if (hasOwn(body, "status") || hasOwn(body, "collaboration_revision")) {
    const refusal = storedRefusalSchema.safeParse(body);
    if (!refusal.success || refusal.data.status !== status) return malformed(status, code);
    const expected = collaborationStageRefusalStatus(refusal.data.code);
    if (expected !== null && expected !== status) return malformed(status, refusal.data.code);
    const extras = parseCollaborationStageRefusalFields(body, refusal.data.code);
    return extras === null ? malformed(status, refusal.data.code) : { type: "refusal", stored: true, status, code: refusal.data.code, collaborationRevision: refusal.data.collaboration_revision, ...extras };
  }
  if (code !== null && Object.hasOwn(UNSTORED_ISSUE_REFUSALS, code)) {
    return UNSTORED_ISSUE_REFUSALS[code] === status ? { type: "refusal", stored: false, status, code, collaborationRevision: null } : malformed(status, code);
  }
  if (code === null && (status === 404 || status === 405)) return { type: "refusal", stored: false, status, code: COLLABORATION_STAGE_ROUTE_UNAVAILABLE, collaborationRevision: null };
  // Plain role/access answers keep the ordinary collaboration cleanup. Any
  // other coded answer proves neither an outcome nor a loss of access.
  if (isCollaborationStageAccessAnswer(status, code)) return { type: "denied" };
  return { type: "unknown", reason: status >= 500 ? "unavailable" : "malformed", status, code };
}

/** The existing object-denial answers of the collaboration account group. */
export function isCollaborationStageAccessAnswer(status: number, code: string | null): boolean {
  return status === 401 || status === 410 || status === 403 && (code === null || code === "forbidden") || status === 404 && code === "not_found";
}

/** Body `code` of an error answer, when it is a well-formed machine code. */
export function collaborationStageErrorCode(body: unknown): string | null {
  return bodyCode(body);
}

const next = (version: string) => String(BigInt(version) + 1n);

/**
 * Contract 3.2: a pending request is retired only when the action, its owner
 * and every field the client knew before sending equal the frozen request.
 * Server-created identifiers were already proven to be non-zero UUIDs.
 */
export function matchesCollaborationStageConfirmation(request: CollaborationStageFrozenRequest | CollaborationStageConfirmationIdentity,
  result: Extract<CollaborationStageWriteResult, { type: "success" }>): boolean {
  const confirmation = result.confirmation;
  if (confirmation.action !== request.action || !COLLABORATION_STAGE_SUCCESS_CODES[confirmation.action].includes(result.code)) return false;
  if (confirmation.action === "revise_policy") {
    return request.action === "revise_policy" && confirmation.project_id === request.projectId.toLowerCase() && confirmation.enabled === request.body.enabled &&
      BigInt(confirmation.policy_revision) > BigInt(request.body.expected_policy_revision);
  }
  if (request.action === "revise_policy" || confirmation.issue_id !== request.issueId.toLowerCase()) return false;
  switch (request.action) {
    case "configure_roles":
      // `unchanged` creates no revision; `configured` never moves backwards.
      return confirmation.action === "configure_roles" && (result.code === "stage_roles_unchanged"
        ? confirmation.role_revision === request.body.expected_role_revision
        : BigInt(confirmation.role_revision) > BigInt(request.body.expected_role_revision));
    case "create_proposal":
      return confirmation.action === "create_proposal" && confirmation.version === "1";
    case "revise_proposal":
      return confirmation.action === "revise_proposal" && confirmation.proposal_id === request.proposalId.toLowerCase() &&
        confirmation.parent_candidate_id === request.body.expected_candidate.candidate_id &&
        confirmation.version === next(request.body.expected_candidate.version);
    case "decide_proposal": {
      const candidate = request.body.candidate;
      const decision = request.body.decision === "accept" ? "accepted" : "rejected";
      return confirmation.action === "decide_proposal" && confirmation.proposal_id === request.proposalId.toLowerCase() &&
        confirmation.candidate_id === candidate.candidate_id && confirmation.version === candidate.version && confirmation.content_digest === candidate.content_digest &&
        confirmation.decision === decision && result.code === `stage_proposal_${decision}` &&
        confirmation.role_revision === request.body.expected_role_revision && confirmation.policy_revision === request.body.expected_policy_revision;
    }
    case "confirm_scope":
      return confirmation.action === "confirm_scope" && confirmation.lifecycle_epoch === request.body.expected_lifecycle_epoch &&
        confirmation.scope_revision === next(request.body.expected_scope_revision);
    case "create_code_candidate":
      return confirmation.action === "create_code_candidate" && (request.body.supersedes === null
        ? confirmation.parent_candidate_id === null && confirmation.version === "1"
        : confirmation.parent_candidate_id === request.body.supersedes.candidate_id && confirmation.version === next(request.body.supersedes.version));
    case "decide_stage":
      return confirmation.action === "decide_stage" && confirmation.step === request.body.step &&
        confirmation.decision === (request.body.decision === "approve" ? "approved" : "returned") &&
        confirmation.candidate_id === request.body.candidate.candidate_id && confirmation.version === request.body.candidate.version &&
        confirmation.content_digest === request.body.candidate.content_digest && confirmation.role_revision === request.body.expected_role_revision &&
        confirmation.policy_revision === request.body.expected_policy_revision && stageExpiryMatches(confirmation.expires_at, request.body.expires_at);
    case "revoke_decision":
      return confirmation.action === "revoke_decision" && confirmation.decision_id === request.decisionId;
    case "advance_stage":
      return confirmation.action === "advance_stage" && confirmation.stage === request.body.target_stage && confirmation.stage_revision === next(request.body.expected_stage_revision);

    default: return false;
  }
}

/** Compare timestamp instants without losing a server's fractional-second expiry. */
function stageExpiryMatches(a: string | null, b: string | null): boolean {
  if (a === null || b === null) return a === b;
  const instant = (value: string) => {
    const fraction = value.match(/\.(\d+)(?:Z|[+-]\d{2}:\d{2})$/)?.[1] ?? "";
    return `${Date.parse(value.replace(/\.\d+/, ""))}:${fraction.replace(/0+$/, "")}`;
  };
  return instant(a) === instant(b);
}
