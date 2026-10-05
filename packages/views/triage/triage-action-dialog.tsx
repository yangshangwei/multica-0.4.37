"use client";
import { useRef, useState } from "react";
import { ApiError, errorCode } from "@multica/core/api";
import { useQuery } from "@tanstack/react-query";
import { useCurrentWorkspace } from "@multica/core/paths";
import {
  createTriageRequestId,
  isFormalAdmission,
  triageDetailOptions,
  useTriageAction,
  type TriageItem,
  type TriageActionName,
  type TriageFields,
  type TriageActionResult,
  type TriageSettings,
} from "@multica/core/triage";
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
import { Textarea } from "@multica/ui/components/ui/textarea";
import { IssuePickerModal } from "../modals/issue-picker-modal";
import {
  TriageFieldsEditor,
  TriageField,
  ReviewerSelect,
} from "./triage-fields";
import { duplicateReference, snoozePresets, TRIAGE_CONTROL } from "./triage-ui";
import { useT } from "../i18n";
import {
  TriageExecutionSummary,
  useTriageExecutionPrerequisites,
} from "./triage-execution-preview";

export function SnoozeInput({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const { t, i18n } = useT("triage");
  const [presets] = useState(() => snoozePresets());
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const format = (d: Date) =>
    new Intl.DateTimeFormat(i18n.language, {
      dateStyle: "medium",
      timeStyle: "short",
      timeZone: zone,
    }).format(d);
  const localValue = value
    ? new Date(
        new Date(value).getTime() - new Date(value).getTimezoneOffset() * 60000,
      )
        .toISOString()
        .slice(0, 16)
    : "";
  return (
    <div className="space-y-3">
      <p className="text-caption text-muted-foreground">
        {t(($) => $.timezone, { zone })}
      </p>
      <div className="grid gap-2">
        {(["hour", "tomorrow", "week"] as const).map((key) => (
          <Button
            key={key}
            variant={
              value === presets[key].toISOString() ? "secondary" : "outline"
            }
            className={`${TRIAGE_CONTROL} h-auto justify-between gap-4 whitespace-normal py-2 text-left`}
            onClick={() => onChange(presets[key].toISOString())}
          >
            <span>{t(($) => $[key])}</span>
            <span className="text-caption text-muted-foreground">
              {format(presets[key])}
            </span>
          </Button>
        ))}
      </div>
      <TriageField label={t(($) => $.custom_time)}>
        <Input
          type="datetime-local"
          className={TRIAGE_CONTROL}
          aria-label={t(($) => $.custom_time)}
          value={localValue}
          onChange={(e) => {
            const d = new Date(e.target.value);
            onChange(Number.isNaN(d.getTime()) ? "" : d.toISOString());
          }}
        />
      </TriageField>
    </div>
  );
}

export function TriageActionDialog({
  wsId,
  item,
  action,
  settings,
  onClose,
  onSuccess,
}: {
  wsId: string;
  item: TriageItem;
  action: TriageActionName;
  settings: TriageSettings;
  onClose: () => void;
  onSuccess: (result: TriageActionResult) => void;
}) {
  const { t } = useT("triage");
  const workspace = useCurrentWorkspace();
  const mutation = useTriageAction(wsId);
  const current = useQuery(triageDetailOptions(wsId, item.issue.id));
  const [revision, setRevision] = useState(item.issue.revision);
  const [reason, setReason] = useState("");
  const [target, setTarget] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);
  const [until, setUntil] = useState("");
  const [reviewer, setReviewer] = useState(item.reviewer_id);
  const [fields, setFields] = useState<TriageFields>({
    status: settings.acceptance_status,
    priority: item.issue.priority,
    project_id: item.candidate_project_id,
    assignee_type: item.candidate_assignee_type,
    assignee_id: item.candidate_assignee_id,
    label_ids: item.issue.labels?.map((l) => l.id),
    start_date: item.issue.start_date,
    due_date: item.issue.due_date,
  });
  const execution = useTriageExecutionPrerequisites(
    wsId,
    fields,
    action === "accept_and_execute",
  );
  const [error, setError] = useState("");
  const [revisionConflict, setRevisionConflict] = useState(false);
  const reasonInput = useRef<HTMLTextAreaElement>(null);
  const intention = useRef<{ signature: string; id: string } | null>(null);
  const inFlight = useRef(false);
  const accepts = action === "accept" || action === "accept_and_execute";
  const needsReason = action === "reject" || action === "reopen";
  const submit = async () => {
    if (
      inFlight.current ||
      (action === "accept_and_execute" && !execution.canExecute)
    )
      return;
    setError("");
    setRevisionConflict(false);
    if (needsReason && (!reason.trim() || reason.trim().length > 2000)) {
      setError(t(($) => $.reason_required));
      reasonInput.current?.focus();
      return;
    }
    if (
      accepts &&
      settings.require_priority &&
      (!fields.priority || fields.priority === "none")
    ) {
      setError(t(($) => $.priority_required));
      document
        .getElementById("triage-priority-field")
        ?.querySelector<HTMLButtonElement>("button")
        ?.focus();
      return;
    }
    if (
      action === "snooze" &&
      (!until ||
        new Date(until).getTime() <= Date.now() ||
        new Date(until).getTime() > Date.now() + 90 * 86400000)
    ) {
      setError(t(($) => $.invalid_snooze));
      return;
    }
    let duplicateId: string | undefined;
    if (action === "duplicate") {
      try {
        duplicateId = duplicateReference(target, workspace?.slug ?? "");
        if (
          !duplicateId ||
          duplicateId === item.issue.id ||
          duplicateId === item.issue.identifier
        )
          throw new Error();
      } catch {
        setError(t(($) => $.invalid_duplicate_link));
        return;
      }
    }
    const payload = {
      expected_revision: revision,
      action,
      reason: reason.trim() || undefined,
      fields: accepts ? fields : undefined,
      duplicate_issue_id: duplicateId,
      snoozed_until: action === "snooze" ? until : undefined,
      reviewer_id: action === "assign_reviewer" ? reviewer : undefined,
    };
    const signature = JSON.stringify(payload);
    if (intention.current?.signature !== signature)
      intention.current = { signature, id: createTriageRequestId() };
    inFlight.current = true;
    try {
      const result = await mutation.mutateAsync({
        id: item.issue.id,
        input: { ...payload, request_id: intention.current.id },
      });
      onSuccess(result);
      onClose();
    } catch (e) {
      setRevisionConflict(
        errorCode(e) === "revision_conflict" ||
          (e instanceof ApiError &&
            e.status === 409 &&
            /triage item changed|resource changed concurrently|item is no longer awaiting triage/i.test(
              e.message,
            )),
      );
      setError(e instanceof Error ? e.message : t(($) => $.failed));
    } finally {
      inFlight.current = false;
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) onClose();
      }}
    >
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $[action])}</DialogTitle>
          <DialogDescription>
            {item.issue.identifier} · {item.issue.title}
          </DialogDescription>
        </DialogHeader>
        {accepts && (
          <>
            {action === "accept" && (
              <p className="text-body text-muted-foreground">
                {t(($) => $.accept_hint)}
              </p>
            )}
            <TriageFieldsEditor
              wsId={wsId}
              fields={fields}
              onChange={setFields}
            />
            {action === "accept_and_execute" && (
              <>
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.execute_hint)}
                </p>
                <TriageExecutionSummary prerequisites={execution} />
              </>
            )}
          </>
        )}
        {action === "duplicate" && (
          <>
            <p className="text-body text-muted-foreground">
              {t(($) => $.duplicate_hint)}
            </p>
            <TriageField label={t(($) => $.duplicate_target)}>
              <Input
                aria-label={t(($) => $.duplicate_target)}
                value={target}
                onChange={(e) => setTarget(e.target.value)}
              />
            </TriageField>
            <Button
              variant="outline"
              className={TRIAGE_CONTROL}
              onClick={() => setSearchOpen(true)}
            >
              {t(($) => $.duplicate_search)}
            </Button>
            <IssuePickerModal
              open={searchOpen}
              onOpenChange={setSearchOpen}
              title={t(($) => $.duplicate_search)}
              description={t(($) => $.duplicate_hint)}
              excludeIds={[item.issue.id]}
              filterIssue={(issue) => isFormalAdmission(issue.admission_status)}
              onSelect={(issue) => {
                setTarget(issue.identifier);
                setSearchOpen(false);
              }}
            />
          </>
        )}
        {(needsReason || accepts || action === "duplicate") && (
          <TriageField
            label={`${t(($) => $.reason)}${needsReason ? ` · ${t(($) => $.required)}` : ""}`}
          >
            <Textarea
              ref={reasonInput}
              aria-label={t(($) => $.reason)}
              value={reason}
              maxLength={2000}
              onChange={(e) => setReason(e.target.value)}
              autoFocus={needsReason}
            />
          </TriageField>
        )}
        {action === "snooze" && (
          <SnoozeInput value={until} onChange={setUntil} />
        )}
        {action === "assign_reviewer" && (
          <ReviewerSelect wsId={wsId} value={reviewer} onChange={setReviewer} />
        )}
        {error && (
          <div role="alert" className="space-y-2 text-body">
            <p className="text-destructive">{error}</p>
            {revisionConflict && (
              <>
                <p>{t(($) => $.refresh_conflict)}</p>
                <Button
                  variant="outline"
                  className={TRIAGE_CONTROL}
                  onClick={async () => {
                    const result = await current.refetch();
                    if (result.data) {
                      setRevision(result.data.issue.revision);
                      intention.current = null;
                      setRevisionConflict(false);
                      setError("");
                    }
                  }}
                >
                  {t(($) => $.refresh)}
                </Button>
              </>
            )}
          </div>
        )}
        <DialogFooter>
          <Button
            variant="outline"
            className={TRIAGE_CONTROL}
            disabled={mutation.isPending}
            onClick={onClose}
          >
            {t(($) => $.cancel)}
          </Button>
          <Button
            className={TRIAGE_CONTROL}
            disabled={
              mutation.isPending ||
              (action === "accept_and_execute" && !execution.canExecute)
            }
            onClick={() => void submit()}
          >
            {mutation.isPending ? t(($) => $.submitting) : t(($) => $[action])}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
