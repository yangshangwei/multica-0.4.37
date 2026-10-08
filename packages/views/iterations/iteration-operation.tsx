"use client";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, isIterationAccessDenied } from "@multica/core/api";
import {
  iterationChoicesOptions,
  protectIterationRead,
  prepareIterationDraft,
  useIterationCommand,
  type Iteration,
  type IterationDraft,
  type IterationPreview,
} from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@multica/ui/components/ui/dialog";
import { useIterationLabels } from "./labels";
import { IterationError } from "./iteration-error";
import { useT, useLocale } from "../i18n";
import { formatInTimeZone } from "../common/format-in-time-zone";
export function IterationOperation({
  wsId,
  iteration,
  operation,
  available = true,
}: {
  wsId: string;
  available?: boolean;
  iteration: Iteration | null;
  settingsRevision: number;
  operation: IterationDraft["operation"];
}) {
  const { t } = useT("projects");
  const locale = useLocale();
  const labels = useIterationLabels();
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [target, setTarget] = useState("");
  const [mode, setMode] = useState<"scheduled" | "today">("scheduled");
  const [retain, setRetain] = useState(false);
  const [preview, setPreview] = useState<IterationPreview | null>(null);
  const [draft, setDraft] = useState<IterationDraft | null>(null);
  const targets = useQuery({
    ...iterationChoicesOptions(wsId),
    enabled: open && available,
  });
  const apply = useIterationCommand(
    wsId,
    `${operation}:${iteration?.id ?? "workspace"}`,
  );
  const pending = apply.pending;
  const showsTarget =
    operation === "end" ||
    operation === "handoff" ||
    (operation === "cancel" && iteration?.status === "active");
  const previewMutation = useMutation({
    mutationKey: ["iterations", wsId, "preview"],
    mutationFn: () =>
      protectIterationRead(client, wsId, async () => {
        const currentSettings = await api.getIterationSettings(wsId);
        const currentIteration = iteration
          ? (await api.getIteration(wsId, iteration.id)).iteration
          : null;
        const input = await prepareIterationDraft(
          wsId,
          currentIteration,
          currentSettings.revision,
          operation,
          reason,
          target || null,
          mode,
          retain,
        );
        if (draft) {
          input.moves = input.moves.map((move) => {
            const previous = draft.moves.find(
              (item) => item.issue_id === move.issue_id,
            );
            return previous ? { ...move, target_id: previous.target_id } : move;
          });
          if (input.start)
            input.start.terminal_choices = input.start.terminal_choices.map(
              (choice) => ({
                ...choice,
                retain:
                  draft.start?.terminal_choices.find(
                    (previous) => previous.issue_id === choice.issue_id,
                  )?.retain ?? choice.retain,
              }),
            );
        }
        const result = await api.previewIteration(wsId, input);
        setDraft(result.draft);
        setPreview(result);
        return { ...result, effectiveTimezone: currentSettings.effective_timezone };
      }),
  });
  async function submit() {
    if (!preview && !pending) return;
    const command = pending ?? {
      kind: "operation" as const,
      body: {
        request_id: crypto.randomUUID(),
        draft: preview!.draft,
        preview_hash: preview!.preview_hash,
      },
    };
    try {
      await apply.mutateAsync({ command, recover: pending !== null });
      setOpen(false);
      setPreview(null);
      setDraft(null);
    } catch {
      if (!apply.pending) setPreview(null);
    }
  }
  const accessDenied = [previewMutation.error, apply.error].some(
    isIterationAccessDenied,
  );
  const visibleIssues = accessDenied
    ? []
    : (previewMutation.data?.issues ?? []);
  const previewTimezones = iteration
    ? [preview?.iterations.find((item) => item.id === iteration.id)?.timezone ?? iteration.timezone]
    : [...new Set(preview?.iterations.length ? preview.iterations.map((item) => item.timezone) : previewMutation.data ? [previewMutation.data.effectiveTimezone] : [])];
  const label = t(($) =>
    operation === "disable"
      ? $.iterations.disable
      : operation === "start"
        ? $.iterations.start
        : operation === "end"
          ? $.iterations.end
          : operation === "cancel"
            ? $.iterations.cancel
            : operation === "delete"
              ? $.iterations.delete
              : operation === "handoff"
                ? $.iterations.handoff
                : $.iterations.assign,
  );
  function changed() {
    setDraft(null);
    setPreview(null);
  }
  if (!available && !pending && !apply.isPending && !apply.error) return null;
  return (
    <>
      {available && (
        <Button variant="outline" onClick={() => setOpen(true)}>
          {label}
        </Button>
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>{label}</DialogTitle>
            <DialogDescription>
              {accessDenied
                ? t(($) => $.iterations.settings)
                : (iteration?.name ?? t(($) => $.iterations.settings))}
            </DialogDescription>
          </DialogHeader>
          <fieldset
            disabled={
              !available ||
              pending !== null ||
              apply.isPending ||
              previewMutation.isPending
            }
            className="space-y-4"
          >
            <label className="block">
              {t(($) => $.iterations.reason)}
              <Input
                value={reason}
                onChange={(e) => {
                  setReason(e.target.value);
                  changed();
                }}
              />
            </label>
            {!accessDenied && showsTarget && (
              <label className="block">
                {t(($) => $.iterations.target)}
                <select
                  value={target}
                  onChange={(e) => {
                    setTarget(e.target.value);
                    changed();
                  }}
                  className="w-full rounded-md border bg-background p-2"
                >
                  <option value="">{t(($) => $.iterations.unassigned)}</option>
                  {targets.data
                    ?.filter(
                      (item) =>
                        item.id !== iteration?.id && item.status === "planned",
                    )
                    .map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.name}
                      </option>
                    ))}
                </select>
              </label>
            )}
            {!accessDenied && ["start", "handoff"].includes(operation) && (
              <>
                <label className="block">
                  <select
                    value={mode}
                    onChange={(e) => {
                      setMode(
                        e.target.value === "today" ? "today" : "scheduled",
                      );
                      changed();
                    }}
                    className="rounded-md border bg-background p-2"
                  >
                    <option value="scheduled">
                      {t(($) => $.iterations.scheduled)}
                    </option>
                    <option value="today">
                      {t(($) => $.iterations.today)}
                    </option>
                  </select>
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={retain}
                    onChange={(e) => {
                      setRetain(e.target.checked);
                      changed();
                    }}
                  />
                  {t(($) => $.iterations.retain)}
                </label>
              </>
            )}
            {!accessDenied &&
              draft?.moves.map((move) => (
                <label className="block" key={move.issue_id}>
                  <span className="break-words">
                    {visibleIssues.find(
                      (issue) => issue.issue_id === move.issue_id,
                    )?.title ?? move.issue_id}
                  </span>
                  <select
                    className="block w-full rounded-md border bg-background p-2"
                    value={move.target_id ?? ""}
                    onChange={(e) => {
                      setDraft({
                        ...draft,
                        moves: draft.moves.map((item) =>
                          item.issue_id === move.issue_id
                            ? { ...item, target_id: e.target.value || null }
                            : item,
                        ),
                      });
                      setPreview(null);
                    }}
                  >
                    <option value="">
                      {t(($) => $.iterations.unassigned)}
                    </option>
                    {targets.data
                      ?.filter(
                        (item) =>
                          item.id !== iteration?.id &&
                          item.status === "planned",
                      )
                      .map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name}
                        </option>
                      ))}
                  </select>
                </label>
              ))}
            {!accessDenied &&
              draft?.start?.terminal_choices.map((choice) => {
                const issue = visibleIssues.find((item) => item.issue_id === choice.issue_id);
                return (
                <label key={choice.issue_id} className="flex gap-2">
                  <input
                    type="checkbox"
                    checked={choice.retain}
                    onChange={(e) => {
                      setDraft({
                        ...draft,
                        start: {
                          ...draft.start!,
                          terminal_choices: draft.start!.terminal_choices.map(
                            (item) =>
                              item.issue_id === choice.issue_id
                                ? { ...item, retain: e.target.checked }
                                : item,
                          ),
                        },
                      });
                      setPreview(null);
                    }}
                  />
                  <span className="break-words">
                    {t(($) => $.iterations.retainTask)}: {issue ? `${issue.identifier} · ${issue.title}` : choice.issue_id}
                  </span>
                </label>
                );
              })}
            <Button
              disabled={
                previewMutation.isPending ||
                (!reason.trim() && operation !== "start") ||
                (operation === "handoff" && !target)
              }
              onClick={() => previewMutation.mutate()}
            >
              {t(($) => $.iterations.preview)}
            </Button>
          </fieldset>
          {(previewMutation.error || apply.error) && (
            <IterationError error={previewMutation.error ?? apply.error} />
          )}
          {!accessDenied && preview && (
            <section className="space-y-3">
              <h3 className="font-semibold">
                {t(($) => $.iterations.changes)}
              </h3>
              {previewTimezones.map((timezone) => <p key={timezone}>
                {t(($) => $.iterations.previewedAt)}: <time dateTime={preview.previewed_at}>{formatInTimeZone(preview.previewed_at, timezone, locale, { year: "numeric" })}</time> · {timezone}
              </p>)}
              {["end", "cancel", "disable", "handoff"].includes(operation) && <p className="font-medium">{t(($) => $.iterations.executionUnchanged)}</p>}
              <p>
                {t(($) => $.iterations.affected)}: {preview.total_affected} ·{" "}
                {t(($) => $.iterations.notification)}:{" "}
                {preview.recipients.length}
              </p>
              {Object.entries(preview.statistics).map(([id, stats]) => <dl key={id} className="flex flex-wrap gap-4">
                <div><dt>{t(($) => $.iterations.name)}</dt><dd>{preview.iterations.find((item) => item.id === id)?.name ?? id}</dd></div>
                <div><dt>{t(($) => $.iterations.done)}</dt><dd>{stats.completed}</dd></div>
                <div><dt>{t(($) => $.iterations.cancelledCount)}</dt><dd>{stats.cancelled}</dd></div>
                <div><dt>{t(($) => $.iterations.remaining)}</dt><dd>{stats.remaining}</dd></div>
              </dl>)}
              {["end", "cancel", "disable", "handoff"].includes(operation) && <p>{t(($) => $.iterations.carriedCount)}: {preview.draft.moves.filter((move) => move.target_id !== null).length} · {t(($) => $.iterations.removedTasks)}: {operation === "disable" || (operation === "cancel" && iteration?.status === "planned") ? preview.issues.length : preview.draft.moves.filter((move) => move.target_id === null).length}</p>}
              {operation === "handoff" && preview.draft.start && <p>{t(($) => $.iterations.nextBaseline)}: {preview.issues.filter((issue) => {
                const move = preview.draft.moves.find((entry) => entry.issue_id === issue.issue_id);
                if (move) return move.target_id === preview.draft.start!.target_id;
                return issue.source_id === preview.draft.start!.target_id && (!['done', 'cancelled'].includes(issue.status_category) || preview.draft.start!.terminal_choices.some((choice) => choice.issue_id === issue.issue_id && choice.retain));
              }).length}</p>}
              {preview.start_preview && (
                <p>
                  {preview.start_preview.effective_start_date} –{" "}
                  {preview.start_preview.effective_end_date} ·{" "}
                  {preview.start_preview.timezone}
                </p>
              )}
              <ul className="max-h-64 overflow-auto">
                {preview.issues.map((issue) => (
                  <li key={issue.issue_id} className="py-2 break-words">
                    {issue.identifier} · {issue.title} ·{" "}
                    {issue.project.name ?? t(($) => $.iterations.none)} ·{" "}
                    {issue.assignee.name ?? t(($) => $.iterations.none)} ·{" "}
                    {labels.category(issue.status_category)} ·{" "}
                    {t(($) => $.iterations.running)}:{" "}
                    {issue.running_execution_count} ·{" "}
                    {t(($) => $.iterations.rollover)}: {issue.rollover_count}
                    {issue.rollover_count >= 3 && <span className="block">{t(($) => $.iterations.rolloverReview)}</span>}
                  </li>
                ))}
              </ul>
              {preview.invalid_items.map((item, i) => (
                <p key={i} role="alert">
                  {labels.validation(item.code)}
                </p>
              ))}
              <Button
                disabled={
                  !available ||
                  preview.complete !== true ||
                  preview.invalid_items.length > 0 ||
                  previewMutation.isPending ||
                  apply.isPending
                }
                onClick={() => void submit()}
              >
                {t(($) => $.iterations.confirm)}
              </Button>
            </section>
          )}
          {apply.isPending && <p role="status">{t(($) => $.iterations.processing)}</p>}
          {previewMutation.isPending && <p role="status">{t(($) => $.iterations.preparing)}</p>}
          {pending && !apply.isPending && (
            <div role="status">
              <p>{t(($) => $.iterations.unknownResult)}</p>
              <Button disabled={apply.isPending} onClick={() => void submit()}>
                {t(($) => $.iterations.recover)}
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
