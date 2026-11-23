// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseCollaborationComment, type CollaborationComment } from "./content-schema";
import { deriveCollaborationCommentThreads } from "./comment-threads";

// Canonical matrix for thread grouping, order, depth and resolution state.
// Views suites only cover wiring and do not repeat these cases through the DOM.
const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const issueId = id(900);
const person = { user_id: id(901), name: "Resolver", avatar_url: null, external: false };
function comment(n: number, over: Partial<Record<keyof CollaborationComment, unknown>> & { minute?: number } = {}): CollaborationComment {
  const { minute = n, ...rest } = over;
  const at = `2026-10-19T00:${String(minute).padStart(2, "0")}:00Z`;
  const parsed = parseCollaborationComment({ id: id(n), issue_id: issueId, author_type: "member", author: null, content: `Comment ${n}`, type: "comment",
    parent_id: null, comment_revision: "1", attachment_ids: [], created_at: at, updated_at: at, deleted_at: null, resolved_at: null, resolved_by: null,
    can_edit: false, can_resolve: true, reactions: [], ...rest });
  if (!parsed) throw new Error(`invalid comment fixture ${n}`);
  return parsed;
}
const deleted = { deleted_at: "2026-10-19T01:00:00Z", content: "", can_resolve: false };
const resolved = { resolved_at: "2026-10-19T02:00:00Z", resolved_by: person };
const shape = (threads: ReturnType<typeof deriveCollaborationCommentThreads>) =>
  threads.map((thread) => ({ id: thread.id, items: thread.items.map((item) => `${item.comment.id.slice(-2)}@${item.depth}`) }));

describe("deriveCollaborationCommentThreads", () => {
  it("returns no threads for an empty page", () => {
    expect(deriveCollaborationCommentThreads([])).toEqual([]);
  });

  it("treats a single unresolved comment as one pending thread resolved at its root", () => {
    const only = comment(1);
    const [thread, ...rest] = deriveCollaborationCommentThreads([only]);
    expect(rest).toHaveLength(0);
    expect(thread).toMatchObject({ id: only.id, root: only, resolvedBy: null, resolveTarget: only, pending: true });
    expect(thread!.items).toEqual([{ comment: only, depth: 0 }]);
  });

  it("orders a thread depth-first with siblings ascending and threads by their first comment", () => {
    // Input order is deliberately shuffled; a later root must not interleave.
    const comments = [
      comment(6, { parent_id: id(2), minute: 6 }),
      comment(5, { minute: 5 }),
      comment(4, { parent_id: id(3), minute: 4 }),
      comment(3, { parent_id: id(1), minute: 9 }),
      comment(2, { parent_id: id(1), minute: 2 }),
      comment(1, { minute: 1 }),
      comment(7, { parent_id: id(4), minute: 10 }),
    ];
    expect(shape(deriveCollaborationCommentThreads(comments))).toEqual([
      { id: id(1), items: ["01@0", "02@1", "06@2", "03@1", "04@2", "07@3"] },
      { id: id(5), items: ["05@0"] },
    ]);
  });

  it("breaks equal creation times by id", () => {
    const comments = [comment(3, { parent_id: id(1), minute: 5 }), comment(2, { parent_id: id(1), minute: 5 }), comment(1, { minute: 1 })];
    expect(shape(deriveCollaborationCommentThreads(comments))[0]!.items).toEqual(["01@0", "02@1", "03@1"]);
  });

  it("marks the whole thread resolved when any member carries the resolution", () => {
    const reply = comment(2, { parent_id: id(1), ...resolved });
    const [thread] = deriveCollaborationCommentThreads([comment(1), reply, comment(3, { parent_id: id(2) })]);
    expect(thread).toMatchObject({ resolvedBy: reply, resolveTarget: null, pending: false });
  });

  it("does not count a deleted root without replies as pending", () => {
    const [thread] = deriveCollaborationCommentThreads([comment(1, deleted)]);
    expect(thread).toMatchObject({ pending: false, resolvedBy: null, resolveTarget: null });
  });

  it("does not count a thread whose members are all deleted as pending", () => {
    const [thread] = deriveCollaborationCommentThreads([comment(1, deleted), comment(2, { parent_id: id(1), ...deleted })]);
    expect(thread).toMatchObject({ pending: false, resolveTarget: null });
  });

  it("keeps a deleted root with a surviving reply pending and resolves at the first resolvable member", () => {
    const first = comment(2, { parent_id: id(1), can_resolve: false });
    const second = comment(3, { parent_id: id(1) });
    const [thread] = deriveCollaborationCommentThreads([comment(1, deleted), first, second]);
    expect(thread).toMatchObject({ pending: true, resolveTarget: second });
  });

  it("prefers a resolvable root over an earlier-listed resolvable reply", () => {
    const root = comment(1);
    const [thread] = deriveCollaborationCommentThreads([comment(2, { parent_id: id(1) }), root]);
    expect(thread!.resolveTarget).toBe(root);
  });

  it("keeps a deleted comment that still carries a resolution as the resolved marker", () => {
    const root = comment(1, { ...deleted, ...resolved, can_resolve: true });
    const [thread] = deriveCollaborationCommentThreads([root, comment(2, { parent_id: id(1) })]);
    expect(thread).toMatchObject({ resolvedBy: root, pending: false, resolveTarget: null });
  });

  it("groups replies of an unloaded parent under that missing id with a null root", () => {
    const missing = id(50);
    const a = comment(2, { parent_id: missing, minute: 2 });
    const b = comment(3, { parent_id: missing, minute: 3 });
    const nested = comment(4, { parent_id: id(2), minute: 4 });
    const threads = deriveCollaborationCommentThreads([comment(1, { minute: 1 }), b, nested, a]);
    expect(shape(threads)).toEqual([
      { id: id(1), items: ["01@0"] },
      { id: missing, items: ["02@1", "04@2", "03@1"] },
    ]);
    expect(threads[1]).toMatchObject({ root: null, pending: true, resolveTarget: a });
  });

  it("keeps separate groups for different unloaded parents", () => {
    const threads = deriveCollaborationCommentThreads([comment(2, { parent_id: id(50) }), comment(3, { parent_id: id(51) })]);
    expect(threads.map((thread) => thread.id)).toEqual([id(50), id(51)]);
  });

  it("terminates on cyclic parents and keeps every comment exactly once", () => {
    const comments = [comment(1, { parent_id: id(2) }), comment(2, { parent_id: id(1) }), comment(3, { parent_id: id(3) }), comment(4, { parent_id: id(1) })];
    const threads = deriveCollaborationCommentThreads(comments);
    expect(shape(threads)).toEqual([
      { id: id(1), items: ["01@0", "02@1", "04@1"] },
      { id: id(3), items: ["03@0"] },
    ]);
    expect(threads[0]!.root).toBe(comments[0]);
  });
});
