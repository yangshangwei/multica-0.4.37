import { useState, useSyncExternalStore } from "react";
import { collaborationDeliveryRevision, finishCollaborationDeliveryDraft, newCollaborationDeliveryForm, readCollaborationDeliveryRecords,
  saveCollaborationDeliveryRecord, subscribeCollaborationDelivery, type CollaborationDeliveryForm, type CollaborationDeliveryScope,
  type CollaborationUploadDraft, type CollaborationCreationDraft, type CollaborationDeliveryRequest } from "./delivery-draft";

/** Mount under an authorized context key; drafts are shared by both clients. */
export function useCollaborationDeliveryForm(scope: CollaborationDeliveryScope, purpose: CollaborationDeliveryForm["purpose"]) {
  useSyncExternalStore(subscribeCollaborationDelivery, collaborationDeliveryRevision, () => 0);
  const [error, setError] = useState<unknown>(null);
  let storedError: unknown = null;
  let records: ReturnType<typeof readCollaborationDeliveryRecords> = [];
  try { records = readCollaborationDeliveryRecords(scope); } catch (caught) { storedError = caught; }
  const saved = records.find((record): record is CollaborationDeliveryForm => record.kind === "form" && record.purpose === purpose);
  const [current, setCurrent] = useState(() => ({ form: saved ?? newCollaborationDeliveryForm(scope, purpose), persisted: saved !== undefined }));
  const stored = records.find((record): record is CollaborationDeliveryForm => record.kind === "form" && record.draftId === current.form.draftId);
  // A request may finish after its original component unmounts. A remounted
  // form must observe retirement rather than resurrect the consumed draft.
  if (!storedError && stored && !current.persisted) setCurrent({ ...current, persisted: true });
  else if (!storedError && !stored && current.persisted) setCurrent({ form: saved ?? newCollaborationDeliveryForm(scope, purpose), persisted: saved !== undefined });
  const form = stored ?? current.form;
  const update = (fields: Partial<Pick<CollaborationDeliveryForm, "title" | "content" | "recipientQuery" | "recipient" | "selectedAttachmentIds">>) => {
    try {
      const latest = readCollaborationDeliveryRecords(scope).find((record): record is CollaborationDeliveryForm => record.kind === "form" && record.draftId === form.draftId) ?? form;
      saveCollaborationDeliveryRecord({ ...latest, ...fields });
      setError(null);
    } catch (caught) { setError(caught); }
  };
  const reset = () => {
    try {
      finishCollaborationDeliveryDraft(scope, form.draftId);
      setCurrent({ form: newCollaborationDeliveryForm(scope, purpose), persisted: false });
      setError(null);
    } catch (caught) { setError(caught); }
  };
  return { form, update, reset, error: error ?? storedError,
    uploads: records.filter((record): record is CollaborationUploadDraft => record.kind === "upload" && record.draftId === form.draftId),
    creations: records.filter((record): record is CollaborationCreationDraft => record.kind === "creation" && record.draftId === form.draftId),
    deliveries: records.filter((record): record is CollaborationDeliveryRequest => record.kind === "delivery" && record.draftId === form.draftId) };
}
