"use client";
import { useRef, useState } from "react";
import {
  useCreateTriageItem,
  createTriageRequestId,
  type TriageFields,
  type TriageItem,
} from "@multica/core/triage";
import type { UploadResult } from "@multica/core/hooks/use-file-upload";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  ContentEditor,
  TitleEditor,
  type ContentEditorRef,
  useEditorUpload,
} from "../editor";
import { FileUploadButton } from "@multica/ui/components/common/file-upload-button";
import { useT } from "../i18n";
import { TriageFieldsEditor, TriageField } from "./triage-fields";
import { TRIAGE_CONTROL } from "./triage-ui";

export function TriageCreateDialog({
  wsId,
  onClose,
  onCreated,
}: {
  wsId: string;
  onClose: () => void;
  onCreated: (item: TriageItem) => void;
}) {
  const { t } = useT("triage");
  const mutation = useCreateTriageItem(wsId);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [sourceUrl, setSourceUrl] = useState("");
  const [fields, setFields] = useState<TriageFields>({ priority: "none" });
  const [uploads, setUploads] = useState<UploadResult[]>([]);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");
  const editor = useRef<ContentEditorRef>(null);
  const intention = useRef<{ signature: string; id: string } | null>(null);
  const inFlight = useRef(false);
  const { uploadWithToast } = useEditorUpload();
  const submit = async () => {
    if (inFlight.current || uploading || editor.current?.hasActiveUploads())
      return;
    if (!title.trim()) {
      setError(t(($) => $.invalid_title));
      return;
    }
    const currentDescription = editor.current?.getMarkdown() ?? description;
    const payload = {
      title: title.trim(),
      description: currentDescription,
      priority: fields.priority,
      candidate_project_id: fields.project_id,
      candidate_assignee_type: fields.assignee_type,
      candidate_assignee_id: fields.assignee_id,
      label_ids: fields.label_ids,
      start_date: fields.start_date,
      due_date: fields.due_date,
      source_url: sourceUrl || undefined,
      attachment_ids: uploads
        .filter(
          (a) =>
            currentDescription.includes(a.markdownLink) ||
            currentDescription.includes(a.link),
        )
        .map((a) => a.id),
    };
    const signature = JSON.stringify(payload);
    if (intention.current?.signature !== signature)
      intention.current = { signature, id: createTriageRequestId() };
    inFlight.current = true;
    setError("");
    try {
      const item = await mutation.mutateAsync({
        ...payload,
        request_id: intention.current.id,
      });
      onCreated(item);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : t(($) => $.failed));
    } finally {
      inFlight.current = false;
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending && !uploading) onClose();
      }}
    >
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $.create)}</DialogTitle>
          <DialogDescription>{t(($) => $.create_hint)}</DialogDescription>
        </DialogHeader>
        <TitleEditor
          autoFocus
          onChange={setTitle}
          placeholder={t(($) => $.title_field)}
          onSubmitShortcut={() => void submit()}
        />
        <ContentEditor
          ref={editor}
          defaultValue=""
          onUpdate={setDescription}
          debounceMs={0}
          placeholder={t(($) => $.description)}
          className="min-h-32"
          onUploadingChange={setUploading}
          onUploadFile={async (file) => {
            const result = await uploadWithToast(file);
            if (result) setUploads((all) => [...all, result]);
            return result;
          }}
        />
        <FileUploadButton
          onSelect={(file) => editor.current?.uploadFile(file)}
        />
        <TriageFieldsEditor
          wsId={wsId}
          fields={fields}
          onChange={setFields}
          showStatus={false}
        />
        <TriageField label={t(($) => $.source_url)}>
          <Input
            type="url"
            aria-label={t(($) => $.source_url)}
            value={sourceUrl}
            onChange={(e) => setSourceUrl(e.target.value)}
          />
        </TriageField>
        {error && (
          <p role="alert" className="text-destructive">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button
            className={TRIAGE_CONTROL}
            variant="outline"
            onClick={onClose}
            disabled={mutation.isPending || uploading}
          >
            {t(($) => $.cancel)}
          </Button>
          <Button
            className={TRIAGE_CONTROL}
            disabled={mutation.isPending || uploading || !title.trim()}
            onClick={() => void submit()}
          >
            {uploading
              ? t(($) => $.uploading)
              : mutation.isPending
                ? t(($) => $.submitting)
                : t(($) => $.create)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
