"use client";
import { useRef, useState } from "react";
import {
  createTriageRequestId,
  usePreviewTriageBatch,
  useCommitTriageBatch,
  type TriageItem,
  type TriageBatchActionName,
  type TriageBatchInput,
  type TriageBatchPreview,
  type TriageBatchRowResult,
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
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { TriageSelect, ReviewerSelect } from "./triage-fields";
import { SnoozeInput } from "./triage-action-dialog";
import { TRIAGE_CONTROL } from "./triage-ui";
import { useT } from "../i18n";

export function TriageBatchDialog({
  wsId,
  items,
  onClose,
  onSuccess,
}: {
  wsId: string;
  items: TriageItem[];
  onClose: () => void;
  onSuccess: (ids: string[]) => void;
}) {
  const { t } = useT("triage");
  // This is the explicit review snapshot: later queue refreshes must not replace it.
  const [selection] = useState(items);
  const previewMutation = usePreviewTriageBatch(wsId);
  const commit = useCommitTriageBatch(wsId);
  const [action, setAction] = useState<TriageBatchActionName>("accept");
  const [reason, setReason] = useState("");
  const [until, setUntil] = useState("");
  const [reviewer, setReviewer] = useState<string | null>(null);
  const [preview, setPreview] = useState<TriageBatchPreview | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [results, setResults] = useState<TriageBatchRowResult[]>([]);
  const requests = useRef<TriageBatchInput | null>(null);
  const [error, setError] = useState("");
  const busy = previewMutation.isPending || commit.isPending;
  const previewSelected = async () => {
    if (action === "reject" && !reason.trim()) {
      setError(t(($) => $.reason_required));
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
    setError("");
    const shared = {
      action,
      reason: action === "reject" ? reason.trim() : undefined,
      snoozed_until: action === "snooze" ? until : undefined,
      reviewer_id: action === "assign_reviewer" ? reviewer : undefined,
    };
    try {
      const response = await previewMutation.mutateAsync({
        ...shared,
        items: selection.map((item) => ({
          issue_id: item.issue.id,
          expected_revision: item.issue.revision,
        })),
      });
      setPreview(response);
      setSelected(
        new Set(
          response.items.filter((row) => row.valid).map((row) => row.issue_id),
        ),
      );
      requests.current = {
        items: response.items
          .filter((row) => row.valid)
          .map((row) => ({
            ...shared,
            issue_id: row.issue_id,
            expected_revision: row.expected_revision,
            request_id: createTriageRequestId(),
          })),
      };
    } catch (e) {
      setError(e instanceof Error ? e.message : t(($) => $.failed));
    }
  };
  const submit = async () => {
    if (!requests.current || busy) return;
    const succeeded = new Set(
      results
        .filter((row) => row.status === "success")
        .map((row) => row.issue_id),
    );
    const pending = requests.current.items.filter(
      (row) => selected.has(row.issue_id) && !succeeded.has(row.issue_id),
    );
    if (!pending.length) return;
    try {
      const response = await commit.mutateAsync({ items: pending });
      setResults((old) => [
        ...old.filter(
          (row) =>
            !response.results.some((next) => next.issue_id === row.issue_id),
        ),
        ...response.results,
      ]);
      onSuccess(
        response.results
          .filter((row) => row.status === "success")
          .map((row) => row.issue_id),
      );
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : t(($) => $.failed));
    }
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose();
      }}
    >
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t(($) => $.batch)}</DialogTitle>
          <DialogDescription>
            {t(($) => $.batch_hint)}{" "}
            {t(($) => $.selected, { count: selection.length })}
          </DialogDescription>
        </DialogHeader>
        {!preview && (
          <>
            <TriageSelect
              label={t(($) => $.result)}
              value={action}
              onChange={(v) => {
                if (
                  v === "accept" ||
                  v === "reject" ||
                  v === "snooze" ||
                  v === "assign_reviewer"
                )
                  setAction(v);
              }}
              options={(
                ["accept", "reject", "snooze", "assign_reviewer"] as const
              ).map((v) => ({ value: v, label: t(($) => $[v]) }))}
            />
            {action === "reject" && (
              <Textarea
                aria-label={t(($) => $.reason)}
                value={reason}
                maxLength={2000}
                onChange={(e) => setReason(e.target.value)}
              />
            )}
            {action === "snooze" && (
              <SnoozeInput value={until} onChange={setUntil} />
            )}
            {action === "assign_reviewer" && (
              <ReviewerSelect
                wsId={wsId}
                value={reviewer}
                onChange={setReviewer}
              />
            )}
          </>
        )}
        {preview && (
          <div className="space-y-2">
            <p role="status">
              {t(($) => $.preview_valid, { count: preview.valid_count })}
            </p>
            {preview.items.map((row) => {
              const item = selection.find((i) => i.issue.id === row.issue_id);
              const outcome = results.find((r) => r.issue_id === row.issue_id);
              const known = (
                [
                  "success",
                  "conflict",
                  "invalid",
                  "forbidden",
                  "failed",
                ] as const
              ).find((v) => v === outcome?.status);
              return (
                <label
                  key={row.issue_id}
                  className="flex min-h-11 items-start gap-3 py-2"
                >
                  <Checkbox
                    aria-label={t(($) => $.select_task, {
                      title: item?.issue.title ?? row.issue_id,
                    })}
                    checked={selected.has(row.issue_id)}
                    disabled={
                      !row.valid || busy || outcome?.status === "success"
                    }
                    onCheckedChange={(checked) =>
                      setSelected((old) => {
                        const next = new Set(old);
                        if (checked) next.add(row.issue_id);
                        else next.delete(row.issue_id);
                        return next;
                      })
                    }
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block break-words">
                      {item?.issue.identifier} {item?.issue.title}
                    </span>
                    {(row.error || outcome?.error) && (
                      <span className="block text-caption text-destructive">
                        {outcome?.error ?? row.error}
                      </span>
                    )}
                    {outcome && (
                      <span className="block text-caption">
                        {known ? t(($) => $[known]) : outcome.status}
                      </span>
                    )}
                  </span>
                </label>
              );
            })}
          </div>
        )}
        {error && (
          <p role="alert" className="text-destructive">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button
            variant="outline"
            className={TRIAGE_CONTROL}
            onClick={onClose}
            disabled={busy}
          >
            {t(($) => $.close)}
          </Button>
          {!preview ? (
            <Button
              className={TRIAGE_CONTROL}
              onClick={() => void previewSelected()}
              disabled={busy || !selection.length}
            >
              {busy ? t(($) => $.processing) : t(($) => $.preview)}
            </Button>
          ) : (
            <Button
              className={TRIAGE_CONTROL}
              disabled={
                busy ||
                ![...selected].some(
                  (id) =>
                    !results.some(
                      (r) => r.issue_id === id && r.status === "success",
                    ),
                )
              }
              onClick={() => void submit()}
            >
              {busy
                ? t(($) => $.processing)
                : results.length
                  ? t(($) => $.retry_unfinished)
                  : t(($) => $.commit_selected)}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
