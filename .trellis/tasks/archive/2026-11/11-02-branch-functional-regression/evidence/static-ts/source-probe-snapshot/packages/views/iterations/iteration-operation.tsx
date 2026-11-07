"use client";
import { useId, useState } from "react";
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
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@multica/ui/components/ui/dialog";
import { useIterationLabels } from "./labels";
import { IterationError } from "./iteration-error";
import { IterationSelect } from "./iteration-assignment";
import { useT, useLocale } from "../i18n";
import { formatInTimeZone } from "../common/format-in-time-zone";
export function IterationOperation({
  wsId,
  iteration,
  operation,
  available = true,
  open: controlledOpen,
  onOpenChange,
  hideTrigger = false,
  description,
}: {
  wsId: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  hideTrigger?: boolean;
  description?: string;
  available?: boolean;
  iteration: Iteration | null;
  settingsRevision: number;
  operation: IterationDraft["operation"];
}) {
  const { t } = useT("projects");
  const locale = useLocale();
  const labels = useIterationLabels();
  const client = useQueryClient();
  const fieldId = useId();
  const [internalOpen, setInternalOpen] = useState(false);
  const open = controlledOpen ?? internalOpen;
  function setOpen(next: boolean) {
    setInternalOpen(next);
    onOpenChange?.(next);
  }
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
  const issueFacts = new Map(visibleIssues.map((issue) => [issue.issue_id, issue]));
  const destinationChoices = (targets.data ?? []).filter((item) => item.id !== iteration?.id && item.status === "planned");
  const locked = !available || pending !== null || apply.isPending || previewMutation.isPending;
  const reasonRequired = operation !== "start";
  const reasonMissing = reasonRequired && !reason.trim();
  const targetMissing = operation === "handoff" && !target;
  const modeOptions = [
    { value: "scheduled", label: t(($) => $.iterations.scheduled) },
    { value: "today", label: t(($) => $.iterations.today) },
  ];
  const destructive = operation === "cancel" || operation === "delete" || operation === "disable";
  const previewTimezones = iteration
    ? [preview?.iterations.find((item) => item.id === iteration.id)?.timezone ?? iteration.timezone]
    : [...new Set(preview?.iterations.length ? preview.iterations.map((item) => item.timezone) : previewMutation.data ? [previewMutation.data.effectiveTimezone] : [])];
  const previewIterations = new Map((preview?.iterations ?? []).map((item) => [item.id, item]));
  const previewMoves = new Map((preview?.draft.moves ?? []).map((move) => [move.issue_id, move]));
  const retainedTerminalIssues = new Set(preview?.draft.start?.terminal_choices.filter((choice) => choice.retain).map((choice) => choice.issue_id));
  const nextScopeIssues = new Set(preview?.draft.start ? preview.issues.filter((issue) => {
    const move = previewMoves.get(issue.issue_id);
    if (move) return move.target_id === preview.draft.start!.target_id;
    return issue.source_id === preview.draft.start!.target_id && (!['done', 'cancelled'].includes(issue.status_category) || retainedTerminalIssues.has(issue.issue_id));
  }).map((issue) => issue.issue_id) : []);
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
  if (!open && !available && !pending && !apply.isPending && !apply.error) return null;
  return (
    <>
      {available && !hideTrigger && (
        <Button variant={destructive ? "destructive" : operation === "start" ? "default" : "outline"} className="h-auto min-h-8 whitespace-normal [overflow-wrap:anywhere] pointer-coarse:min-h-11" onClick={() => setOpen(true)}>
          {label}
        </Button>
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[85vh] min-w-0 overflow-y-auto sm:max-w-3xl [&_[data-slot=dialog-close]]:pointer-coarse:min-h-11 [&_[data-slot=dialog-close]]:pointer-coarse:min-w-11">
          <DialogHeader>
            <DialogTitle className="pr-8 [overflow-wrap:anywhere]">{label}</DialogTitle>
            <DialogDescription className="[overflow-wrap:anywhere]">
              {accessDenied
                ? t(($) => $.iterations.settings)
                : (description ?? iteration?.name ?? t(($) => $.iterations.settings))}
            </DialogDescription>
          </DialogHeader>
          <fieldset
            disabled={locked}
            className="min-w-0 space-y-4"
          >
            <div className="grid gap-1.5">
              <div className="flex flex-wrap items-baseline gap-2">
                <label htmlFor={`${fieldId}-reason`}>{t(($) => $.iterations.reason)}</label>
                <span aria-hidden className="text-caption text-muted-foreground">{t(($) => reasonRequired ? $.iterations.audit.required : $.iterations.audit.optional)}</span>
              </div>
              <Input
                id={`${fieldId}-reason`}
                className="pointer-coarse:min-h-11"
                required={reasonRequired}
                aria-describedby={reasonMissing ? `${fieldId}-reason-hint` : undefined}
                value={reason}
                onChange={(e) => {
                  setReason(e.target.value);
                  changed();
                }}
              />
            </div>
            {reasonMissing && <p id={`${fieldId}-reason-hint`} className="text-caption text-muted-foreground">{t(($) => $.iterations.audit.reasonRequiredHint)}</p>}
            {!accessDenied && showsTarget && (
              <div className="space-y-1.5">
                <IterationSelect label={t(($) => $.iterations.target)} disabled={locked} value={target} items={destinationChoices} descriptionId={targetMissing ? `${fieldId}-target-hint` : undefined} onChange={(value) => { setTarget(value); changed(); }} />
                {targetMissing && <p id={`${fieldId}-target-hint`} className="text-caption text-muted-foreground">{t(($) => $.iterations.audit.handoffTargetHint)}</p>}
              </div>
            )}
            {!accessDenied && ["start", "handoff"].includes(operation) && (
              <>
                <div className="grid gap-1.5">
                  <label htmlFor={`${fieldId}-mode`}>{t(($) => $.iterations.audit.startMode)}</label>
                  <Select
                    items={modeOptions}
                    disabled={locked}
                    value={mode}
                    onValueChange={(value) => {
                      setMode(value === "today" ? "today" : "scheduled");
                      changed();
                    }}
                  >
                    <SelectTrigger id={`${fieldId}-mode`} className="w-full pointer-coarse:min-h-11"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {modeOptions.map((option) => <SelectItem key={option.value} value={option.value} className="pointer-coarse:min-h-11 [&>span:first-child]:min-w-0 [&>span:first-child]:shrink [&>span:first-child]:whitespace-normal">{option.label}</SelectItem>)}
                    </SelectContent>
                  </Select>
                </div>
                <label htmlFor={`${fieldId}-retain`} className="flex min-h-8 cursor-pointer items-center gap-2 pointer-coarse:min-h-11">
                  <Checkbox
                    id={`${fieldId}-retain`}
                    disabled={locked}
                    className="pointer-coarse:after:-inset-3.5"
                    checked={retain}
                    onCheckedChange={(checked) => {
                      setRetain(checked);
                      changed();
                    }}
                  />
                  {t(($) => $.iterations.retain)}
                </label>
              </>
            )}
            {!accessDenied &&
              draft?.moves.map((move) => (
                <IterationSelect
                    key={move.issue_id}
                    label={t(($) => $.iterations.audit.taskDestination, { task: issueFacts.get(move.issue_id)?.title ?? move.issue_id })}
                    disabled={locked}
                    items={destinationChoices}
                    value={move.target_id ?? ""}
                    onChange={(value) => {
                      setDraft({
                        ...draft,
                        moves: draft.moves.map((item) =>
                          item.issue_id === move.issue_id
                            ? { ...item, target_id: value || null }
                            : item,
                        ),
                      });
                      setPreview(null);
                    }}
                />
              ))}
            {!accessDenied &&
              draft?.start?.terminal_choices.map((choice) => {
                const issue = issueFacts.get(choice.issue_id);
                return (
                <label htmlFor={`${fieldId}-terminal-${choice.issue_id}`} key={choice.issue_id} className="flex min-h-8 cursor-pointer items-start gap-2 py-1.5 pointer-coarse:min-h-11">
                  <Checkbox
                    id={`${fieldId}-terminal-${choice.issue_id}`}
                    disabled={locked}
                    className="mt-0.5 pointer-coarse:after:-inset-3.5"
                    checked={choice.retain}
                    onCheckedChange={(checked) => {
                      setDraft({
                        ...draft,
                        start: {
                          ...draft.start!,
                          terminal_choices: draft.start!.terminal_choices.map(
                            (item) =>
                              item.issue_id === choice.issue_id
                                ? { ...item, retain: checked }
                                : item,
                          ),
                        },
                      });
                      setPreview(null);
                    }}
                  />
                  <span className="min-w-0 [overflow-wrap:anywhere]">
                    {t(($) => $.iterations.retainTask)}: {issue ? `${issue.identifier} · ${issue.title}` : choice.issue_id}
                  </span>
                </label>
                );
              })}
            <Button
              variant={preview ? "outline" : "default"}
              className="pointer-coarse:min-h-11"
              aria-describedby={[reasonMissing ? `${fieldId}-reason-hint` : "", targetMissing ? `${fieldId}-target-hint` : ""].filter(Boolean).join(" ") || undefined}
              disabled={
                previewMutation.isPending ||
                reasonMissing || targetMissing
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
            <div className="min-w-0 space-y-6">
              <section aria-labelledby={`${fieldId}-summary`} className="min-w-0 space-y-3">
                <h3 id={`${fieldId}-summary`} className="text-body font-semibold">{t(($) => $.iterations.audit.changeSummary)}</h3>
                <dl className="grid gap-x-8 gap-y-3 text-body sm:grid-cols-2 [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:font-medium [&_dd]:[overflow-wrap:anywhere]">
                  <div><dt>{t(($) => $.iterations.affected)}</dt><dd className="tabular-nums">{preview.total_affected}</dd></div>
                  <div><dt>{t(($) => $.iterations.notification)}</dt><dd className="tabular-nums">{preview.recipients.length}</dd></div>
                  {previewTimezones.map((timezone) => <div key={timezone}><dt>{t(($) => $.iterations.previewedAt)}</dt><dd><time dateTime={preview.previewed_at}>{formatInTimeZone(preview.previewed_at, timezone, locale, { year: "numeric" })}</time><span className="block text-caption font-normal text-muted-foreground">{timezone}</span></dd></div>)}
                  {["end", "cancel", "disable", "handoff"].includes(operation) && <>
                    <div><dt>{t(($) => $.iterations.carriedCount)}</dt><dd className="tabular-nums">{preview.draft.moves.filter((move) => move.target_id !== null).length}</dd></div>
                    <div><dt>{t(($) => $.iterations.removedTasks)}</dt><dd className="tabular-nums">{operation === "disable" || (operation === "cancel" && iteration?.status === "planned") ? preview.issues.length : preview.draft.moves.filter((move) => move.target_id === null).length}</dd></div>
                  </>}
                  {operation === "handoff" && preview.draft.start && <div><dt>{t(($) => $.iterations.nextBaseline)}</dt><dd className="tabular-nums">{nextScopeIssues.size}</dd></div>}
                  {preview.start_preview && <>
                    <div><dt>{t(($) => $.iterations.startDate)}</dt><dd>{preview.start_preview.effective_start_date}</dd></div>
                    <div><dt>{t(($) => $.iterations.endDate)}</dt><dd>{preview.start_preview.effective_end_date}<span className="block text-caption font-normal text-muted-foreground">{preview.start_preview.timezone}</span></dd></div>
                  </>}
                </dl>
                {Object.entries(preview.statistics).map(([id, stats]) => <div key={id} className="space-y-2 pt-2">
                  <h4 className="text-body font-medium [overflow-wrap:anywhere]">{previewIterations.get(id)?.name ?? id}</h4>
                  <dl className="grid grid-cols-3 gap-4 text-body [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:mt-1 [&_dd]:font-medium [&_dd]:tabular-nums">
                    <div><dt>{t(($) => $.iterations.done)}</dt><dd>{stats.completed}</dd></div>
                    <div><dt>{t(($) => $.iterations.cancelledCount)}</dt><dd>{stats.cancelled}</dd></div>
                    <div><dt>{t(($) => $.iterations.remaining)}</dt><dd>{stats.remaining}</dd></div>
                  </dl>
                </div>)}
                {["end", "cancel", "disable", "handoff"].includes(operation) && <p className="text-caption text-muted-foreground">{t(($) => $.iterations.executionUnchanged)}</p>}
              </section>
              <section aria-labelledby={`${fieldId}-tasks`} className="min-w-0 space-y-2">
                <h3 id={`${fieldId}-tasks`} className="text-body font-semibold">{t(($) => $.iterations.audit.taskChanges)}</h3>
                <ul className="max-h-64 min-w-0 divide-y divide-border overflow-y-auto">
                  {preview.issues.map((issue) => {
                    const move = previewMoves.get(issue.issue_id);
                    const destination = move ? move.target_id : operation === "handoff" && nextScopeIssues.has(issue.issue_id) ? preview.draft.start!.target_id : null;
                    return <li key={issue.issue_id} className="min-w-0 space-y-2 py-3">
                      <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1 [overflow-wrap:anywhere]"><span className="text-caption text-muted-foreground">{issue.identifier}</span><span className="min-w-0 text-body font-medium">{issue.title}</span></div>
                      <dl className="grid gap-x-6 gap-y-2 text-caption sm:grid-cols-2 [&_dt]:text-muted-foreground [&_dd]:[overflow-wrap:anywhere]">
                        <div><dt>{t(($) => $.iterations.status)}</dt><dd>{labels.category(issue.status_category)}</dd></div>
                        <div><dt>{t(($) => $.iterations.project)}</dt><dd>{issue.project.name ?? t(($) => $.iterations.none)}</dd></div>
                        <div><dt>{t(($) => $.iterations.assignee)}</dt><dd>{issue.assignee.name ?? t(($) => $.iterations.none)}</dd></div>
                        <div><dt>{t(($) => $.iterations.source)}</dt><dd>{issue.source_id ? previewIterations.get(issue.source_id)?.name ?? issue.source_id : t(($) => $.iterations.unassignedState)}</dd></div>
                        {["end", "cancel", "disable", "handoff"].includes(operation) && <div><dt>{t(($) => $.iterations.destination)}</dt><dd>{destination ? previewIterations.get(destination)?.name ?? destination : t(($) => $.iterations.unassigned)}</dd></div>}
                        <div><dt>{t(($) => $.iterations.running)}</dt><dd className="tabular-nums">{issue.running_execution_count}</dd></div>
                        <div><dt>{t(($) => $.iterations.rollover)}</dt><dd className="tabular-nums">{issue.rollover_count}</dd></div>
                      </dl>
                      {issue.rollover_count >= 3 && <p className="text-caption text-muted-foreground">{t(($) => $.iterations.rolloverReview)}</p>}
                    </li>;
                  })}
                </ul>
              </section>
              {preview.invalid_items.map((item, i) => (
                <p key={i} role="alert">
                  {labels.validation(item.code)}
                </p>
              ))}
              <Button
                variant={destructive ? "destructive" : "default"}
                className="h-auto min-h-8 whitespace-normal [overflow-wrap:anywhere] pointer-coarse:min-h-11"
                disabled={
                  !available ||
                  preview.complete !== true ||
                  preview.invalid_items.length > 0 ||
                  previewMutation.isPending ||
                  apply.isPending
                }
                onClick={() => void submit()}
              >
                {label}
              </Button>
            </div>
          )}
          {apply.isPending && <p role="status">{t(($) => $.iterations.processing)}</p>}
          {previewMutation.isPending && <div role="status" className="space-y-3"><span className="sr-only">{t(($) => $.iterations.preparing)}</span><Skeleton className="h-5 w-1/3" /><Skeleton className="h-16 w-full" /><Skeleton className="h-16 w-full" /></div>}
          {pending && !apply.isPending && (
            <div role="status">
              <p>{t(($) => $.iterations.unknownResult)}</p>
              <Button variant="outline" className="pointer-coarse:min-h-11" disabled={apply.isPending} onClick={() => void submit()}>
                {t(($) => $.iterations.recover)}
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
