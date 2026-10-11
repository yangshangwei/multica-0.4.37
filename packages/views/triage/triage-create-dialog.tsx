"use client";
import { useRef, useState } from "react";
import { ApiError, clientErrorMessage, errorCode } from "@multica/core/api";
import {
  useCreateTriageItem,
  createTriageRequestId,
  type CreateTriageItemInput,
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
  type TitleEditorRef,
  useEditorUpload,
} from "../editor";
import { FileUploadButton } from "@multica/ui/components/common/file-upload-button";
import { useT } from "../i18n";
import { TriageFieldsEditor, TriageField } from "./triage-fields";
import { TRIAGE_CONTROL } from "./triage-ui";

type CreateFieldErrors = {
  title?: string;
  description?: string;
  source_url?: string;
  start_date?: string;
  due_date?: string;
};

type CreateValidationInput = Pick<
  CreateTriageItemInput,
  "title" | "description" | "source_url" | "start_date" | "due_date"
>;
type CreateFieldErrorKey =
  | "invalid_title"
  | "invalid_title_length"
  | "invalid_description"
  | "invalid_content"
  | "invalid_source_url"
  | "invalid_start_date"
  | "invalid_due_date"
  | "start_date_after_due_date"
  | "due_date_before_start_date";
type CreateFieldErrorKeys = Partial<
  Record<keyof CreateFieldErrors, CreateFieldErrorKey>
>;
const INTAKE_VALIDATION_MESSAGE =
  "title is required (maximum 500 characters); description maximum is 1 mib";

/** Keep browser-side checks aligned with the strict date-only API contract. */
export function isValidTriageDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const [yearText, monthText, dayText] = value.split("-");
  const year = Number(yearText);
  const month = Number(monthText);
  const day = Number(dayText);
  if (!yearText || !monthText || !dayText || month < 1 || month > 12 || day < 1) {
    return false;
  }
  // Date.UTC treats years below 100 as 1900-based, unlike Go's time.Parse.
  const leapYear = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
  const daysInMonth = [
    31, leapYear ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31,
  ][month - 1]!;
  return day <= daysInMonth;
}

export function isValidTriageSourceUrl(value: string): boolean {
  const normalized = value.trim();
  if (!normalized) return true;
  // Require an explicit authority: WHATWG URL otherwise repairs missing slashes
  // and strips control characters that Go's net/url rejects.
  const parts = /^https?:\/\/([^/?#]+)([^?#]*)(?:\?[^#]*)?(?:#(.*))?$/i.exec(
    normalized,
  );
  if (
    !parts ||
    parts[1]!.includes("\\") ||
    [...normalized].some(
      (character) => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127,
    ) ||
    [parts[1], parts[2], parts[3] ?? ""].some((part) => /%(?![\da-f]{2})/i.test(part!))
  ) {
    return false;
  }
  try {
    const parsed = new URL(normalized);
    return (
      (parsed.protocol === "http:" || parsed.protocol === "https:") &&
      parsed.hostname.length > 0
    );
  } catch {
    return false;
  }
}

export function validateTriageCreateInput(
  input: CreateValidationInput,
): CreateFieldErrorKeys {
  const errors: CreateFieldErrorKeys = {};
  const title = input.title.trim();
  const description = input.description ?? "";
  if (!title) errors.title = "invalid_title";
  else if (title.includes("\0")) errors.title = "invalid_content";
  else if ([...title].length > 500) errors.title = "invalid_title_length";
  if (description.includes("\0")) errors.description = "invalid_content";
  else if (new TextEncoder().encode(description).length > (1 << 20)) {
    errors.description = "invalid_description";
  }
  if (!isValidTriageSourceUrl(input.source_url ?? "")) {
    errors.source_url = "invalid_source_url";
  }
  const start = input.start_date ?? "";
  const due = input.due_date ?? "";
  if (start && !isValidTriageDate(start)) errors.start_date = "invalid_start_date";
  if (due && !isValidTriageDate(due)) errors.due_date = "invalid_due_date";
  if (!errors.start_date && !errors.due_date && start && due && start > due) {
    errors.start_date = "start_date_after_due_date";
    errors.due_date = "due_date_before_start_date";
  }
  return errors;
}

export function triageCreateServerFieldErrors(
  error: unknown,
  input: CreateValidationInput,
): CreateFieldErrorKeys {
  if (!(error instanceof ApiError) || error.status < 400 || error.status >= 500) {
    return {};
  }
  const message = clientErrorMessage(error)?.toLowerCase() ?? "";
  const code = errorCode(error)?.toLowerCase();
  // The intake response combines independent limits. Only blame fields whose
  // submitted values fail validation; a valid draft retains a global error.
  if (message === INTAKE_VALIDATION_MESSAGE) {
    const { title, description } = validateTriageCreateInput(input);
    return {
      ...(title ? { title } : {}),
      ...(description ? { description } : {}),
    };
  }
  if (
    code === "invalid_source_url" ||
    message === "source_url must be an http or https url"
  ) {
    return { source_url: "invalid_source_url" };
  }
  if (message === "start_date must not be after due_date") {
    return {
      start_date: "start_date_after_due_date",
      due_date: "due_date_before_start_date",
    };
  }
  if (code === "invalid_start_date" || message === "start_date must be yyyy-mm-dd") {
    return { start_date: "invalid_start_date" };
  }
  if (code === "invalid_due_date" || message === "due_date must be yyyy-mm-dd") {
    return { due_date: "invalid_due_date" };
  }
  if (code === "invalid_title") return { title: "invalid_title" };
  if (code === "invalid_description") return { description: "invalid_description" };
  return {};
}

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
  const [fieldErrors, setFieldErrors] = useState<CreateFieldErrors>({});
  const [formError, setFormError] = useState("");
  const titleEditor = useRef<TitleEditorRef>(null);
  const editor = useRef<ContentEditorRef>(null);
  const formBody = useRef<HTMLDivElement>(null);
  const intention = useRef<{ signature: string; id: string } | null>(null);
  const inFlight = useRef(false);
  const { uploadWithToast } = useEditorUpload();
  const translateErrors = (keys: CreateFieldErrorKeys): CreateFieldErrors => {
    const errors: CreateFieldErrors = {};
    for (const field of [
      "title", "description", "source_url", "start_date", "due_date",
    ] as const) {
      const key = keys[field];
      if (key) errors[field] = t(($) => $[key]);
    }
    return errors;
  };
  const focusInvalidField = (errors: CreateFieldErrors) => {
    if (errors.title) titleEditor.current?.focus();
    else if (errors.description) editor.current?.focus();
    else if (errors.start_date) {
      formBody.current?.querySelector<HTMLElement>("#triage-create-start-date-field-control")?.focus();
    } else if (errors.due_date) {
      formBody.current?.querySelector<HTMLElement>("#triage-create-due-date-field-control")?.focus();
    } else if (errors.source_url) {
      formBody.current?.querySelector<HTMLElement>("#triage-source-url-field-control")?.focus();
    }
  };
  const clearFieldError = (field: keyof CreateFieldErrors) => {
    setFieldErrors((current) => {
      if (!current[field]) return current;
      const next = { ...current };
      delete next[field];
      return next;
    });
  };
  const updateFields = (next: TriageFields) => {
    const datesChanged =
      next.start_date !== fields.start_date || next.due_date !== fields.due_date;
    setFields(next);
    setFieldErrors((current) => {
      if (!datesChanged || (!current.start_date && !current.due_date)) {
        return current;
      }
      const nextErrors = { ...current };
      delete nextErrors.start_date;
      delete nextErrors.due_date;
      return nextErrors;
    });
    setFormError("");
  };
  const submit = async () => {
    if (inFlight.current || uploading || editor.current?.hasActiveUploads())
      return;

    const normalizedSourceUrl = sourceUrl.trim();
    const currentDescription = editor.current?.getMarkdown() ?? description;
    // Consume the queued editor update so a callback from before submission
    // cannot clear the validation or server feedback we are about to display.
    editor.current?.flushPendingUpdate();
    setDescription(currentDescription);
    const nextFieldErrors = translateErrors(validateTriageCreateInput({
      title,
      description: currentDescription,
      source_url: normalizedSourceUrl,
      start_date: fields.start_date,
      due_date: fields.due_date,
    }));
    if (Object.keys(nextFieldErrors).length > 0) {
      setFieldErrors(nextFieldErrors);
      setFormError("");
      focusInvalidField(nextFieldErrors);
      return;
    }

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
      source_url: normalizedSourceUrl || undefined,
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
    setFieldErrors({});
    setFormError("");
    try {
      const item = await mutation.mutateAsync({
        ...payload,
        request_id: intention.current.id,
      });
      onCreated(item);
      onClose();
    } catch (e) {
      const clientMessage = clientErrorMessage(e);
      const nextErrors = translateErrors(triageCreateServerFieldErrors(e, payload));
      if (Object.keys(nextErrors).length > 0) {
        setFieldErrors(nextErrors);
        setFormError("");
        focusInvalidField(nextErrors);
      } else {
        setFormError(
          clientMessage?.toLowerCase() === INTAKE_VALIDATION_MESSAGE
            ? t(($) => $.create_failed)
            : clientMessage ?? t(($) => $.create_failed),
        );
      }
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
      <DialogContent
        closeLabel={t(($) => $.close)}
        className="flex max-h-[90dvh] flex-col overflow-hidden sm:max-w-2xl pointer-coarse:[&_[data-slot=dialog-close]]:min-h-11 pointer-coarse:[&_[data-slot=dialog-close]]:min-w-11"
      >
        <DialogHeader className="shrink-0 pr-10">
          <DialogTitle>{t(($) => $.create)}</DialogTitle>
          <DialogDescription>{t(($) => $.create_hint)}</DialogDescription>
        </DialogHeader>
        <div ref={formBody} className="min-h-0 flex-1 space-y-5 overflow-y-auto overscroll-contain pr-1">
          <div className="space-y-1.5">
            <label
              id="triage-create-title-label"
              className="block text-caption font-medium text-muted-foreground"
            >
              {t(($) => $.title_field)}{" "}
              <span aria-hidden="true">({t(($) => $.required)})</span>
            </label>
            <TitleEditor
              ref={titleEditor}
              autoFocus
              onChange={(value) => {
                setTitle(value);
                clearFieldError("title");
                setFormError("");
              }}
              aria-labelledby="triage-create-title-label"
              aria-describedby={fieldErrors.title ? "triage-create-title-error" : undefined}
              aria-invalid={fieldErrors.title ? true : undefined}
              aria-required
              className={`rounded-lg border border-input px-2.5 py-1 text-title-sm md:text-body [overflow-wrap:anywhere] focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50 aria-invalid:border-destructive ${TRIAGE_CONTROL}`}
              placeholder={t(($) => $.title_placeholder)}
              onSubmitShortcut={() => void submit()}
            />
            {fieldErrors.title && (
              <p
                id="triage-create-title-error"
                role="alert"
                className="text-caption text-destructive"
              >
                {fieldErrors.title}
              </p>
            )}
          </div>
          <div className="space-y-1.5">
            <label
              id="triage-create-description-label"
              className="block text-caption font-medium text-muted-foreground"
            >
              {t(($) => $.description)}
            </label>
            <ContentEditor
              ref={editor}
              defaultValue=""
              onUpdate={(value) => {
                setDescription(value);
                clearFieldError("description");
                setFormError("");
              }}
              debounceMs={0}
              aria-labelledby="triage-create-description-label"
              aria-describedby={fieldErrors.description ? "triage-create-description-error" : undefined}
              aria-invalid={fieldErrors.description ? true : undefined}
              placeholder={t(($) => $.description_placeholder)}
              className="min-h-32 rounded-lg border border-input p-2.5 focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50 aria-invalid:border-destructive"
              onUploadingChange={setUploading}
              onUploadFile={async (file) => {
                const result = await uploadWithToast(file);
                if (result) setUploads((all) => [...all, result]);
                return result;
              }}
            />
            {fieldErrors.description && (
              <p
                id="triage-create-description-error"
                role="alert"
                className="text-caption text-destructive"
              >
                {fieldErrors.description}
              </p>
            )}
          </div>
          <FileUploadButton
            showLabel
            onSelect={(file) => editor.current?.uploadFile(file)}
            className="[@media(pointer:coarse)]:min-h-11"
          />
          <TriageFieldsEditor
            wsId={wsId}
            idPrefix="triage-create"
            fields={fields}
            onChange={updateFields}
            showStatus={false}
            candidateMode
            pickerAffordance
            errors={{
              start_date: fieldErrors.start_date,
              due_date: fieldErrors.due_date,
            }}
          />
          <TriageField
            id="triage-source-url-field"
            controlId="triage-source-url-field-control"
            label={t(($) => $.source_url)}
            error={fieldErrors.source_url}
          >
            <Input
              id="triage-source-url-field-control"
              type="url"
              className={TRIAGE_CONTROL}
              placeholder={t(($) => $.source_url_placeholder)}
              aria-labelledby="triage-source-url-field-label"
              aria-describedby={fieldErrors.source_url ? "triage-source-url-field-error" : undefined}
              aria-invalid={fieldErrors.source_url ? true : undefined}
              value={sourceUrl}
              onChange={(e) => {
                setSourceUrl(e.target.value);
                clearFieldError("source_url");
                setFormError("");
              }}
            />
          </TriageField>
          {formError && (
            <p role="alert" className="text-caption text-destructive">
              {formError}
            </p>
          )}
        </div>
        <DialogFooter className="shrink-0">
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
