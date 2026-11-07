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
