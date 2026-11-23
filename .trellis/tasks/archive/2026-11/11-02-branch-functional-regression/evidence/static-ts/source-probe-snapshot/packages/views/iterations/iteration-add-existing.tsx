"use client";
import { useId, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Search } from "lucide-react";
import { api } from "@multica/core/api";
import {
  classifyIterationCandidate,
  iterationCandidateSearchOptions,
  iterationUnplannedCandidatesOptions,
  ISSUE_REFERENCE_LIMIT,
  UNPLANNED_CANDIDATE_LIMIT,
  iterationChoicesOptions,
  iterationSettingsOptions,
  protectIterationRead,
  useIterationCommand,
  type IterationCandidateState,
  type IterationPreview,
} from "@multica/core/iterations";
import { issueStatusCategory } from "@multica/core/issues";
import type { Issue } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { DialogFooter } from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { useDebouncedValue } from "../common/use-debounced-value";
import { StatusIcon } from "../issues/components/status-icon";
import { useT } from "../i18n";
import { IterationError } from "./iteration-error";
import { useIterationLabels } from "./labels";

type Target = { id: string; status: string };
type Outcome =
  | { kind: "changed"; issues: Issue[] }
  | { kind: "invalid"; issues: Issue[]; invalid: IterationPreview["invalid_items"] }
  | { kind: "ready"; issues: Issue[]; preview: IterationPreview };

const sameState = (a: IterationCandidateState, b: IterationCandidateState) =>
  a.blocked === b.blocked && a.needsReason === b.needsReason && a.needsCompletedConfirmation === b.needsCompletedConfirmation;

/**
 * Picks existing tasks for one iteration. Eligibility, the reason field and
 * completed-work consent are derived from the selection (see
 * core/iterations/candidates.ts); one click previews and applies, stopping
 * only when the preview disagrees with what the user saw.
 */
export function IterationAddExisting({
  wsId,
  iteration,
  open,
  available,
  onAdded,
}: {
  wsId: string;
  iteration: Target;
  /** The dialog stays mounted while closed; reads run only while it is open. */
  open: boolean;
  available: boolean;
  onAdded: () => void;
}) {
  const { t } = useT("projects");
  const labels = useIterationLabels();
  const client = useQueryClient();
  const fieldId = useId();
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Issue[]>([]);
  const [reason, setReason] = useState("");
  const [confirmedFor, setConfirmedFor] = useState<string | null>(null);
  const [invalid, setInvalid] = useState<IterationPreview["invalid_items"]>([]);
  const [changed, setChanged] = useState(false);
  const debounced = useDebouncedValue(query.trim(), 300);
  const searchOptions = iterationCandidateSearchOptions(wsId, debounced);
  const search = useQuery({ ...searchOptions, enabled: open && searchOptions.enabled !== false, placeholderData: keepPreviousData });
  const browse = useQuery({ ...iterationUnplannedCandidatesOptions(wsId), enabled: open });
  const settings = useQuery(iterationSettingsOptions(wsId));
  const choices = useQuery(iterationChoicesOptions(wsId));
  const apply = useIterationCommand(wsId, `assign:${iteration.id}`);
  const pending = apply.pending;
  const openIterations = choices.data ? new Set(choices.data.map((item) => item.id)) : null;
  const classify = (issue: Issue) => classifyIterationCandidate(issue, iteration, openIterations);
  const ready = selected.filter((issue) => classify(issue).blocked === null);
  const needsReason = ready.some((issue) => classify(issue).needsReason);
  const completedIds = ready.filter((issue) => classify(issue).needsCompletedConfirmation).map((issue) => issue.id).sort().join(" ");
  // Consent covers the exact completed set it was given for.
  const confirmed = completedIds !== "" && confirmedFor === completedIds;
  const showsReason = needsReason || iteration.status === "active";

  function edited() {
    setInvalid([]);
    setChanged(false);
  }
  function toggle(issue: Issue, on: boolean) {
    setSelected((list) => (on ? [...list.filter((item) => item.id !== issue.id), issue] : list.filter((item) => item.id !== issue.id)));
    edited();
  }

  const prepare = useMutation({
    mutationKey: ["iterations", wsId, "assignment-preview", iteration.id],
    mutationFn: (input: { issues: Issue[]; reason: string | null; allowCompleted: boolean; open: ReadonlySet<string> | null }) =>
      protectIterationRead(client, wsId, async (): Promise<Outcome> => {
        const currentSettings = await api.getIterationSettings(wsId);
        const check = (issue: Issue) => classifyIterationCandidate(issue, iteration, input.open);
        const draft = {
          operation: "move" as const,
          iteration_id: null,
          expected_iteration_revision: null,
          expected_scope_revision: null,
          expected_settings_revision: currentSettings.revision,
          reason: input.reason,
          moves: input.issues.map((issue) => ({ issue_id: issue.id, expected_issue_revision: 1, expected_source_id: null as string | null, target_id: iteration.id, allow_completed: false })),
          start: null,
        };
        // The first preview only reads current facts; its validation is ignored.
        const preliminary = await api.previewIteration(wsId, draft);
        const facts = new Map(preliminary.issues.map((fact) => [fact.issue_id, fact]));
        if (preliminary.complete !== true || preliminary.issues.length !== input.issues.length || input.issues.some((issue) => !facts.has(issue.id)))
          throw new Error("Incomplete assignment preview");
        const issues = input.issues.map((issue) => {
          const fact = facts.get(issue.id)!;
          const category = issueStatusCategory({ status: fact.status_category });
          return { ...issue, revision: fact.revision, current_iteration_id: fact.source_id, ...(category ? { status_category: category } : {}) };
        });
        if (issues.some((issue, index) => !sameState(check(issue), check(input.issues[index]!)))) return { kind: "changed", issues };
        const result = await api.previewIteration(wsId, {
          ...draft,
          moves: issues.map((issue) => ({
            issue_id: issue.id,
            expected_issue_revision: issue.revision!,
            expected_source_id: issue.current_iteration_id ?? null,
            target_id: iteration.id,
            allow_completed: input.allowCompleted && check(issue).needsCompletedConfirmation,
          })),
        });
        const members = new Set(input.issues.map((issue) => issue.id));
        if (result.issues.some((fact) => !members.has(fact.issue_id)) || new Set(result.issues.map((fact) => fact.issue_id)).size !== result.issues.length)
          throw new Error("Unexpected assignment preview members");
        if (result.invalid_items.length > 0) return { kind: "invalid", issues, invalid: result.invalid_items };
        if (result.complete !== true) throw new Error("Incomplete assignment preview");
        return { kind: "ready", issues, preview: result };
      }),
  });

  function reset() {
    setQuery("");
    setSelected([]);
    setReason("");
    setConfirmedFor(null);
    edited();
  }
  async function send(command: NonNullable<typeof pending>, recover: boolean) {
    try {
      await apply.mutateAsync({ command, recover });
      reset();
      onAdded();
    } catch {
      // apply.error shows the failure; an unknown outcome stays pending for
      // retry, and a rejection keeps the draft for the user to adjust.
    }
  }
  async function submit() {
    if (!available || pending || ready.length === 0) return;
    let outcome: Outcome;
    try {
      outcome = await prepare.mutateAsync({ issues: ready, reason: showsReason ? reason.trim() || null : null, allowCompleted: confirmed, open: openIterations });
    } catch {
      return;
    }
    const refreshed = new Map(outcome.issues.map((issue) => [issue.id, issue]));
    setSelected((list) => list.map((issue) => refreshed.get(issue.id) ?? issue));
    if (outcome.kind === "changed") return setChanged(true);
    if (outcome.kind === "invalid") return setInvalid(outcome.invalid);
    await send({ kind: "operation", body: { request_id: crypto.randomUUID(), draft: outcome.preview.draft, preview_hash: outcome.preview.preview_hash } }, false);
  }

  const locked = !available || pending !== null || prepare.isPending || apply.isPending;
  const canSubmit = !locked && !!settings.data && ready.length > 0 && (!needsReason || reason.trim() !== "") && (completedIds === "" || confirmed);
  // An empty query browses unplanned work; any text searches the workspace.
  const searchMode = query.trim() !== "";
  const results = (searchMode ? search.data?.issues : browse.data?.issues) ?? [];
  const addable = results.filter((issue) => classify(issue).blocked === null && !selected.some((item) => item.id === issue.id));
  const busy = searchMode ? query.trim() !== debounced || search.isFetching : browse.isFetching && browse.data !== undefined;
  // Selected tasks already visible above are not repeated below.
  const visible = new Set(results.map((issue) => issue.id));
  const offscreen = selected.filter((issue) => !visible.has(issue.id));
  const rowErrors = new Map<string, string>();
  for (const item of invalid) if (item.issue_id) rowErrors.set(item.issue_id, labels.validation(item.code));
  const generalErrors = invalid.filter((item) => !item.issue_id);

  function hint(state: IterationCandidateState) {
    switch (state.blocked) {
      case "member": return t(($) => $.iterations.picker.member);
      case "cancelled": return t(($) => $.iterations.picker.cancelled);
      case "triage": return t(($) => $.iterations.picker.triage);
      case "closed_source": return t(($) => $.iterations.picker.closedSource);
      case "completed_needs_active": return t(($) => $.iterations.picker.completedNeedsActive);
    }
    const notes: string[] = [];
    if (state.needsCompletedConfirmation) notes.push(t(($) => $.iterations.picker.completed));
    if (state.sourceId) {
      const name = choices.data?.find((item) => item.id === state.sourceId)?.name ?? t(($) => $.iterations.picker.otherIteration);
      notes.push(t(($) => $.iterations.picker.moveFrom, { name }));
    }
    return notes.join(" · ") || null;
  }
  const row = (prefix: string, issue: Issue) => {
    const state = classify(issue);
    const checked = selected.some((item) => item.id === issue.id);
    return (
      <CandidateRow
        key={issue.id}
        id={`${fieldId}-${prefix}-${issue.id}`}
        issue={issue}
        hint={hint(state)}
        error={rowErrors.get(issue.id) ?? null}
        checked={checked}
        disabled={locked || (state.blocked !== null && !checked)}
        onChange={(on) => toggle(issue, on)}
      />
    );
  };

  const failure = (message: string, retry: () => void) => (
    <div role="alert" className="flex flex-wrap items-center gap-2 py-2 text-caption text-destructive">
      {message}
      <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" onClick={retry}>{t(($) => $.iterations.retry)}</Button>
    </div>
  );
  const selectAll = (
    <Button
      variant="ghost"
      size="sm"
      className="pointer-coarse:min-h-11"
      onClick={() => {
        setSelected((current) => [...current, ...addable]);
        edited();
      }}
    >
      {t(($) => $.iterations.picker.selectAll, { count: addable.length })}
    </Button>
  );
  // Previous results stay while the next read loads, dimmed so they read as stale.
  const list = (
    <ul aria-busy={busy} className={`max-h-60 overflow-y-auto transition-opacity motion-reduce:transition-none ${busy ? "opacity-60" : ""}`}>
      {results.map((issue) => row(searchMode ? "result" : "unplanned", issue))}
    </ul>
  );

  return (
    <>
      <fieldset disabled={locked} className="min-w-0 space-y-4">
        <div className="space-y-2">
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" aria-hidden />
            <Input
              autoFocus
              aria-label={t(($) => $.iterations.audit.searchTasks)}
              placeholder={t(($) => $.iterations.picker.placeholder)}
              className="pl-8 pointer-coarse:min-h-11"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          {searchMode ? (
            <section aria-label={t(($) => $.iterations.picker.results)} className="space-y-1">
              {search.error && !busy ? (
                failure(t(($) => $.iterations.picker.searchFailed), () => void search.refetch())
              ) : busy && results.length === 0 ? (
                <p role="status" className="py-2 text-caption text-muted-foreground">{t(($) => $.iterations.picker.searching)}</p>
              ) : results.length === 0 && (search.data?.missing.length ?? 0) === 0 ? (
                <p className="py-2 text-caption text-muted-foreground">{t(($) => $.iterations.picker.noResults)}</p>
              ) : (
                <>
                  {addable.length > 1 && <div className="flex justify-end">{selectAll}</div>}
                  {list}
                  {(search.data?.missing.length ?? 0) > 0 && <p className="text-caption text-muted-foreground [overflow-wrap:anywhere]">{t(($) => $.iterations.picker.missing, { ids: search.data!.missing.join(", ") })}</p>}
                  {search.data?.truncated && <p className="text-caption text-muted-foreground">{t(($) => $.iterations.picker.truncated, { limit: ISSUE_REFERENCE_LIMIT })}</p>}
                </>
              )}
            </section>
          ) : (
            <section aria-labelledby={`${fieldId}-unplanned`} className="space-y-1">
              <div className="flex min-h-8 flex-wrap items-center justify-between gap-x-2">
                <h3 id={`${fieldId}-unplanned`} className="text-caption font-medium text-muted-foreground">{t(($) => $.iterations.picker.unplanned)}</h3>
                {addable.length > 1 && selectAll}
              </div>
              {browse.error && !busy ? (
                failure(t(($) => $.iterations.picker.unplannedFailed), () => void browse.refetch())
              ) : browse.isPending ? (
                <p role="status" className="py-2 text-caption text-muted-foreground">{t(($) => $.iterations.picker.unplannedLoading)}</p>
              ) : results.length === 0 ? (
                <p className="py-2 text-caption text-muted-foreground">{t(($) => $.iterations.picker.unplannedEmpty)}</p>
              ) : (
                <>
                  {list}
                  {browse.data?.more && <p className="text-caption text-muted-foreground">{t(($) => $.iterations.picker.unplannedMore, { limit: UNPLANNED_CANDIDATE_LIMIT })}</p>}
                </>
              )}
            </section>
          )}
        </div>
        {offscreen.length > 0 && (
          <section aria-labelledby={`${fieldId}-selected`} className="space-y-1">
            <h3 id={`${fieldId}-selected`} className="text-caption font-medium text-muted-foreground">
              {t(($) => $.iterations.picker.otherSelected)} <span className="tabular-nums">{offscreen.length}</span>
            </h3>
            <ul className="max-h-48 overflow-y-auto">{offscreen.map((issue) => row("selected", issue))}</ul>
          </section>
        )}
        {showsReason && (
          <div className="grid gap-1.5">
            <div className="flex flex-wrap items-baseline gap-2">
              <label htmlFor={`${fieldId}-reason`}>{t(($) => $.iterations.reason)}</label>
              <span aria-hidden className="text-caption text-muted-foreground">{t(($) => needsReason ? $.iterations.audit.required : $.iterations.audit.optional)}</span>
            </div>
            <Input
              id={`${fieldId}-reason`}
              className="pointer-coarse:min-h-11"
              required={needsReason}
              aria-describedby={needsReason ? `${fieldId}-reason-hint` : undefined}
              value={reason}
              onChange={(e) => {
                setReason(e.target.value);
                edited();
              }}
            />
            {needsReason && <p id={`${fieldId}-reason-hint`} className="text-caption text-muted-foreground">{t(($) => $.iterations.picker.reasonHint)}</p>}
          </div>
        )}
        {completedIds !== "" && (
          <label htmlFor={`${fieldId}-completed`} className="flex min-h-8 cursor-pointer items-start gap-2 py-1.5 text-caption pointer-coarse:min-h-11">
            <Checkbox
              id={`${fieldId}-completed`}
              className="mt-0.5 pointer-coarse:after:-inset-3.5"
              checked={confirmed}
              onCheckedChange={(checked) => {
                setConfirmedFor(checked ? completedIds : null);
                edited();
              }}
            />
            {t(($) => $.iterations.picker.confirmCompleted, { count: completedIds.split(" ").length })}
          </label>
        )}
      </fieldset>
      {changed && <p role="alert" className="text-caption text-warning-foreground">{t(($) => $.iterations.picker.changed)}</p>}
      {generalErrors.map((item, index) => <p role="alert" className="text-caption text-destructive" key={`${item.code}:${index}`}>{labels.validation(item.code)}</p>)}
      {(prepare.error || apply.error) && (
        <div className="text-caption text-destructive">
          <IterationError error={prepare.error ?? apply.error} />
        </div>
      )}
      {apply.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.iterations.processing)}</p>}
      {prepare.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.iterations.preparing)}</p>}
      {pending && !apply.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.iterations.unknownResult)}</p>}
      <DialogFooter>
        {pending ? (
          <Button variant="outline" className="pointer-coarse:min-h-11" disabled={apply.isPending} onClick={() => void send(pending, true)}>
            {t(($) => $.iterations.retry)}
          </Button>
        ) : (
          <Button className="pointer-coarse:min-h-11" disabled={!canSubmit} onClick={() => void submit()}>
            {ready.length > 0 ? t(($) => $.iterations.picker.add, { count: ready.length }) : t(($) => $.iterations.pages.addExisting)}
          </Button>
        )}
      </DialogFooter>
    </>
  );
}

function CandidateRow({
  id,
  issue,
  hint,
  error,
  checked,
  disabled,
  onChange,
}: {
  id: string;
  issue: Issue;
  hint: string | null;
  error: string | null;
  checked: boolean;
  disabled: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <li>
      <label
        htmlFor={id}
        className={`flex min-h-9 items-start gap-2.5 rounded-md px-2 py-1.5 pointer-coarse:min-h-11 ${disabled ? "cursor-not-allowed" : "cursor-pointer hover:bg-accent/50"}`}
      >
        <Checkbox id={id} className="mt-0.5 pointer-coarse:after:-inset-3.5" checked={checked} disabled={disabled} onCheckedChange={onChange} />
        <span className={`min-w-0 flex-1 ${disabled && !checked ? "text-muted-foreground" : ""}`}>
          <span className="flex min-w-0 items-center gap-2">
            <StatusIcon status={issue.status} category={issueStatusCategory(issue) ?? undefined} className="size-3.5 shrink-0" />
            <span className="shrink-0 text-caption text-muted-foreground tabular-nums">{issue.identifier}</span>
            <span className="truncate" title={issue.title}>{issue.title}</span>
          </span>
          {hint && <span className="mt-0.5 block text-caption text-muted-foreground">{hint}</span>}
          {error && <span role="alert" className="mt-0.5 block text-caption text-destructive">{error}</span>}
        </span>
      </label>
    </li>
  );
}
