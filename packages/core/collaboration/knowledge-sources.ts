import type { QueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { getApi } from "../api";
import { RECOVERY_REQUEST_TIMEOUT_MS, withRequestDeadline } from "../api/request-lifecycle";
import { collaborationKeys, CollaborationUnavailableError, protectCollaborationRead, type CollaborationScope } from "./access";
import { CollaborationKnowledgeSourceSchema, type CollaborationKnowledgeSource } from "./knowledge-schema";

export type KnowledgeSourceSelection =
  | { kind: "memory"; issue_id: string; version: string }
  | { kind: "comment"; issue_id: string; comment_id: string; version: string }
  | { kind: "run_record"; issue_id: string; run_id: string; record_id: string }
  | { kind: "delivery"; issue_id: string; candidate_id: string; submission_id: string; delivery_event_seq: string; accepted_event_seq: string };
export const knowledgeSourceSelectionKey = (source: KnowledgeSourceSelection) => source.kind === "memory" ? `memory:${source.version}`
  : source.kind === "comment" ? `comment:${source.comment_id}:${source.version}`
    : source.kind === "delivery" ? `delivery:${source.candidate_id}:${source.submission_id}:${source.accepted_event_seq}` : `run_record:${source.run_id}:${source.record_id}`;

async function sourceText(source: Exclude<KnowledgeSourceSelection, { kind: "delivery" }>, signal: AbortSignal): Promise<string> {
  const api = getApi();
  if (source.kind === "memory") {
    const value = await api.getCollaborationMemoryVersion(source.issue_id, source.version, { signal });
    if (!value || value.purged || value.content === null) throw new CollaborationUnavailableError();
    return value.content;
  }
  if (source.kind === "comment") {
    const current = await api.getCollaborationComment(source.issue_id, source.comment_id, { signal });
    if (!current || current.deleted_at !== null || current.content_state === "omitted" || current.content_state === "deleted") throw new CollaborationUnavailableError();
    if (current.comment_revision === source.version) return current.content;
    if (BigInt(current.comment_revision) < BigInt(source.version)) throw new CollaborationUnavailableError();
    const history = await api.getCollaborationCommentHistory(source.issue_id, source.comment_id, { signal });
    const bodies = history?.items.flatMap((event) => event.comment_revision === source.version ? [event.after]
      : BigInt(event.comment_revision) === BigInt(source.version) + 1n && event.before !== null ? [event.before] : []) ?? [];
    if (!bodies.length || bodies.some((body) => body !== bodies[0])) throw new CollaborationUnavailableError();
    return bodies[0]!;
  }
  let cursor: string | undefined;
  const seen = new Set<string>();
  for (;;) {
    signal.throwIfAborted();
    const page = await api.getCollaborationRunRecords(source.issue_id, source.run_id, { cursor, limit: 100 }, { signal });
    if (!page) break;
    const item = page.items.find((record) => record.id === source.record_id);
    if (item) return item.body;
    if (page.next_cursor === null || seen.has(page.next_cursor)) break;
    cursor = page.next_cursor; seen.add(cursor);
  }
  throw new CollaborationUnavailableError();
}

/** Resolve the selected immutable versions, never silently substitute the latest text. */
export async function buildKnowledgeSources(client: QueryClient, scope: CollaborationScope, issueId: string,
  selected: readonly KnowledgeSourceSelection[]): Promise<CollaborationKnowledgeSource[]> {
  if (!selected.length || selected.length > 32 || new Set(selected.map(knowledgeSourceSelectionKey)).size !== selected.length) throw new CollaborationUnavailableError();
  for (const source of selected) {
    if (source.kind === "delivery") {
      z.uuid().parse(source.candidate_id);
      const { candidate_id: _candidateId, ...reference } = source;
      CollaborationKnowledgeSourceSchema.parse({ ...reference, sha256: "0".repeat(64) });
    } else CollaborationKnowledgeSourceSchema.parse({ ...source, sha256: "0".repeat(64) });
    if (source.issue_id !== issueId) throw new CollaborationUnavailableError();
  }
  return protectCollaborationRead(client, scope, issueId, () => Promise.all(selected.map((source) => client.fetchQuery({
    queryKey: [...collaborationKeys.issue(scope, issueId), "knowledge-source", knowledgeSourceSelectionKey(source)],
    staleTime: 0, gcTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => withRequestDeadline({ timeoutMs: RECOVERY_REQUEST_TIMEOUT_MS }, signal, async (readSignal) => {
      if (source.kind === "delivery") {
        // The server owns Go's exact full result/basis canonical hash. A fresh
        // authorized candidate read restores that reference without guessing it
        // from JavaScript serialization or copying its private body into state.
        const detail = await getApi().getCollaborationKnowledgeCandidate(source.candidate_id, { signal: readSignal });
        const candidate = detail?.candidate;
        if (!candidate || candidate.availability !== "available" || candidate.source_issue_id !== issueId) throw new CollaborationUnavailableError();
        const reference = candidate.sources.find((item) => item.kind === "delivery" && item.issue_id === issueId && item.submission_id === source.submission_id &&
          item.delivery_event_seq === source.delivery_event_seq && item.accepted_event_seq === source.accepted_event_seq);
        if (!reference) throw new CollaborationUnavailableError();
        readSignal.throwIfAborted();
        return reference;
      }
      const content = await sourceText(source, readSignal);
      const bytes = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(content));
      readSignal.throwIfAborted();
      const sha256 = Array.from(new Uint8Array(bytes), (byte) => byte.toString(16).padStart(2, "0")).join("");
      return CollaborationKnowledgeSourceSchema.parse({ ...source, sha256 });
    })),
  }))));
}
