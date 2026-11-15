// @vitest-environment jsdom
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { defaultStorage } from "../platform/storage";
import { useCollaborationDeliveryForm } from "./delivery-form";
import { clearCollaborationDeliveryDrafts, finishCollaborationDeliveryDraft, newCollaborationDeliveryForm, saveCollaborationDeliveryRecord } from "./delivery-draft";

const scope = { server: "https://server.example", userId: "00000000-0000-4000-8000-000000000001",
  context: { kind: "issue" as const, issueId: "00000000-0000-4000-8000-000000000002", sourceWorkspaceId: "00000000-0000-4000-8000-000000000003" } };
afterEach(() => { cleanup(); clearCollaborationDeliveryDrafts(defaultStorage); });

it("rotates a remounted form when the original request retires its consumed bundle", () => {
  const saved = { ...newCollaborationDeliveryForm(scope, "comment"), content: "Already delivered", selectedAttachmentIds: [scope.context.issueId] };
  saveCollaborationDeliveryRecord(saved);
  const { result } = renderHook(() => useCollaborationDeliveryForm(scope, "comment"));
  expect(result.current.form).toEqual(saved);

  act(() => { finishCollaborationDeliveryDraft(scope, saved.draftId); });

  expect(result.current.form.draftId).not.toBe(saved.draftId);
  expect(result.current.form.content).toBe("");
  expect(result.current.form.selectedAttachmentIds).toEqual([]);
});

it("preserves the reply parent with its draft across remounts and retires both after sending", () => {
  const first = renderHook(() => useCollaborationDeliveryForm(scope, "comment"));
  act(() => { first.result.current.update({ content: "Reply text", parentCommentId: scope.context.sourceWorkspaceId }); });
  first.unmount();
  const second = renderHook(() => useCollaborationDeliveryForm(scope, "comment"));
  expect(second.result.current.form).toMatchObject({ content: "Reply text", parentCommentId: scope.context.sourceWorkspaceId });
  act(() => { second.result.current.reset(); });
  expect(second.result.current.form.content).toBe("");
  expect(second.result.current.form.parentCommentId).toBeUndefined();
});

it("keeps result basis/source snapshot separate from the concurrently mounted discussion form", () => {
  const comment = renderHook(() => useCollaborationDeliveryForm(scope, "comment"));
  const result = renderHook(() => useCollaborationDeliveryForm(scope, "result"));
  act(() => { comment.result.current.update({ content: "Discussion" }); });
  act(() => { result.result.current.update({ content: "Formal result", basis: "Validated independently", sourceCommentId: scope.context.sourceWorkspaceId }); });
  result.unmount();
  const remounted = renderHook(() => useCollaborationDeliveryForm(scope, "result"));
  expect(remounted.result.current.form).toMatchObject({ content: "Formal result", basis: "Validated independently", sourceCommentId: scope.context.sourceWorkspaceId });
  expect(comment.result.current.form.content).toBe("Discussion");
  act(() => { remounted.result.current.reset(); });
  expect(comment.result.current.form.content).toBe("Discussion");
  expect(remounted.result.current.form).toMatchObject({ content: "", basis: "", sourceCommentId: null });
});

it("adopts a prepared shared comment into an untouched composer and preserves later edits", () => {
  const composer = renderHook(() => useCollaborationDeliveryForm(scope, "comment"));
  const preparation = renderHook(() => useCollaborationDeliveryForm(scope, "comment"));
  const testSubject = { candidate_id: scope.context.issueId, candidate_version: "1", candidate_digest: "a".repeat(64) };
  act(() => preparation.result.current.update({ testSubject }));
  expect(composer.result.current.form.testSubject).toEqual(testSubject);
  expect(composer.result.current.form.draftId).toBe(preparation.result.current.form.draftId);
  act(() => composer.result.current.update({ content: "Existing input" }));
  act(() => preparation.result.current.update({ testSubject: undefined }));
  expect(composer.result.current.form.content).toBe("Existing input");
  expect(composer.result.current.form.testSubject).toBeUndefined();
});
