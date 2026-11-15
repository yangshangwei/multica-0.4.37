import type { CollaborationIssue } from "./schema";
import type {
  CollaborationStageCandidateRef, CollaborationStageChild, CollaborationStageProposalDetail, CollaborationStageRead, CollaborationStageView, COLLABORATION_STAGES,
  CollaborationStageScopeCode, CollaborationStageScopeChildInput,
} from "./stage-schema";

type Stage = typeof COLLABORATION_STAGES[number];

/** R1 child readability comes from source membership, which applies to this workspace. */
export function collaborationStageSourceDenied(view: CollaborationStageView): boolean {
  return view.children.items.some((child) => child.can_open === false);
}

/**
 * What the shared stage panel may show and offer. Server capabilities are
 * display hints (`=== true`); the server rechecks every write. Anything that
 * is not an available, fully known, in-sync view offers nothing and is never
 * presented as a satisfied gate.
 */
export interface CollaborationStageState {
  availability: "loading" | "available" | "unsupported" | "unavailable";
  /** Every server enum of the view is one this client knows. */
  known: boolean;
  managed: boolean | null;
  /** The view and the collaboration detail observe the same revision and lifecycle. */
  inSync: boolean;
  policyState: "enabled" | "disabled" | "none" | "unknown" | null;
  stage: Stage | "unknown" | null;
  missing: ("roles" | "scope")[];
  can: { configureRoles: boolean; createProposal: boolean; decideProposal: boolean; confirmScope: boolean };
}

const nothing = (availability: CollaborationStageState["availability"]): CollaborationStageState => ({
  availability, known: false, managed: null, inSync: false, policyState: null, stage: null, missing: [],
  can: { configureRoles: false, createProposal: false, decideProposal: false, confirmScope: false },
});

export function deriveCollaborationStageState(read: CollaborationStageRead<CollaborationStageView> | undefined, issue: CollaborationIssue): CollaborationStageState {
  if (read === undefined) return nothing("loading");
  if (read.state === "unsupported") return nothing("unsupported");
  if (read.state !== "available" || read.value.issue_id !== issue.id.toLowerCase()) return nothing("unavailable");
  const view = read.value;
  const known = view.policy.state !== "unknown" && view.scope_state !== "unknown" && (view.stage === null || view.stage.current !== "unknown") &&
    view.children.items.every((child) => child.origin !== "unknown" && child.code_state !== "unknown") &&
    (view.code_candidates?.items.every((candidate) => candidate.state !== "unknown") ?? true);
  const inSync = view.collaboration_revision === issue.collaboration_revision && view.lifecycle_epoch === issue.lifecycle_epoch;
  const writable = known && inSync && view.managed === true && issue.write_context != null;
  return {
    availability: "available", known, managed: view.managed, inSync, policyState: view.policy.state, stage: view.stage?.current ?? null, missing: [...view.missing],
    can: {
      configureRoles: writable && view.capabilities.configure_roles === true,
      createProposal: writable && view.capabilities.create_proposal === true,
      decideProposal: writable && view.capabilities.decide_proposal === true,
      confirmScope: writable && view.capabilities.confirm_scope === true,
    },
  };
}

/**
 * Actions on one proposal. Both pin the newest version exactly as it is
 * displayed; the server never rebinds a decision to a later version.
 */
export function collaborationStageProposalActions(state: CollaborationStageState, detail: CollaborationStageProposalDetail, issue: CollaborationIssue): {
  newest: CollaborationStageCandidateRef; stale: boolean; canRevise: boolean; canDecide: boolean;
} {
  const version = detail.versions[0]!;
  const newest = { candidate_id: version.candidate_id, version: version.version, content_digest: version.content_digest };
  const stale = version.lifecycle_epoch !== issue.lifecycle_epoch;
  const own = detail.issue_id === issue.id.toLowerCase();
  // Revision is allowed while the newest version is pending, rejected or from an earlier lifecycle.
  const revisable = own && (detail.state === "pending" || detail.state === "rejected") && (version.state === "pending" || version.state === "rejected");
  const decidable = own && detail.state === "pending" && version.state === "pending" && !stale;
  return { newest, stale, canRevise: revisable && state.can.createProposal, canDecide: decidable && state.can.decideProposal };
}

/**
 * The explicit existing-child set a product approver carries into the current
 * lifecycle (A5). Children are named by identity plus the server's current
 * content digest and an explicit code choice from its observed pull requests.
 * No new head is substituted for a chosen version.
 */
export function collaborationStageScopeSelection(view: CollaborationStageView): {
  candidates: CollaborationStageChild[]; preselected: string[]; complete: boolean;
  entries: (issueIds: readonly string[], codes: Readonly<Record<string, CollaborationStageScopeCode | undefined>>) => CollaborationStageScopeChildInput[] | null;
} {
  // Includes descendants at every projected depth; detached mappings cannot be confirmed.
  const candidates = view.children.items.filter((child): child is CollaborationStageChild & { content_digest: string } => child.attached === true && child.can_open === true && child.content_digest !== null);
  const byId = new Map(candidates.map((child) => [child.issue_id, child]));
  return {
    candidates,
    preselected: candidates.filter((child) => child.approval_current === true).map((child) => child.issue_id),
    // Beyond the projected page the list is not the whole set; the panel must say so.
    complete: view.children.truncated === false,
    entries: (issueIds, codes) => {
      const ids = issueIds.map((id) => id.toLowerCase());
      if (new Set(ids).size !== ids.length || !ids.every((id) => byId.has(id))) return null;
      const entries: CollaborationStageScopeChildInput[] = [];
      for (const id of ids) {
        const child = byId.get(id)!;
        const code = codes[id];
        if (code === undefined) return null;
        if (code !== null && !child.pull_requests.some((pr) => pr.provider === code.pull_request.provider &&
          pr.pull_request_id === code.pull_request.pull_request_id && pr.head_sha === code.pull_request.head_sha)) return null;
        entries.push({ issue_id: id, content_digest: child.content_digest, code });
      }
      return entries;
    },
  };
}
