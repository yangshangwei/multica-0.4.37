"use client";
import { useId, useState } from "react";
import { CalendarRange, ChevronRight } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, isIterationAccessDenied } from "@multica/core/api";
import {
  iterationCapabilitiesOptions,
  protectIterationRead,
  iterationChoicesOptions,
  iterationSettingsOptions,
  useIterationCommand,
  definitelyRejected,
  type IterationPreview,
} from "@multica/core/iterations";
import { issueBehavesAs } from "@multica/core/issues";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@multica/ui/components/ui/dialog";
import { PropRow } from "../common/prop-row";
import { IssuePickerModal } from "../modals/issue-picker-modal";
import {
  PropertyPicker,
  PickerItem,
  PickerEmpty,
} from "../issues/components/pickers/property-picker";
import { matchesPinyin } from "../editor/extensions/pinyin-match";
import { useIterationLabels } from "./labels";
import { IterationError } from "./iteration-error";
import { iterationDisclosureClass } from "./iteration-presentation";
import { useT } from "../i18n";
export function IterationAssignment({
  wsId,
  issueId,
  targetId,
  issueIds,
  currentIterationId,
  rolloverCount,
  expanded = false,
  onAssigned,
  available = true,
}: {
  wsId: string;
  issueId?: string;
  targetId?: string;
  issueIds?: string[];
  currentIterationId?: string | null;
  rolloverCount?: number;
  expanded?: boolean;
  onAssigned?: () => void;
  available?: boolean;
}) {
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  if (capability.error && isIterationAccessDenied(capability.error))
    return <IterationError error={capability.error} />;
  if (capability.data?.enabled !== true && !expanded) return null;
  return (
    <Assignment
      key={JSON.stringify([wsId, issueId ?? null, targetId ?? null, issueIds ?? null])}
      wsId={wsId}
      issueId={issueId}
      targetId={targetId}
      issueIds={issueIds}
      currentIterationId={currentIterationId}
      rolloverCount={rolloverCount}
      expanded={expanded}
      onAssigned={onAssigned}
      available={available && capability.data?.enabled === true}
    />
  );
}
function Assignment({
  wsId,
  issueId,
  targetId,
  issueIds,
  currentIterationId,
  rolloverCount,
  expanded = false,
  onAssigned,
  available = true,
}: {
  wsId: string;
  issueId?: string;
  targetId?: string;
  issueIds?: string[];
  currentIterationId?: string | null;
  rolloverCount?: number;
  expanded?: boolean;
  onAssigned?: () => void;
  available?: boolean;
}) {
  const { t } = useT("projects");
  const client = useQueryClient();
  const labels = useIterationLabels();
  const allowCompletedId = useId();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [ids, setIds] = useState(issueId ?? issueIds?.join(" ") ?? "");
  const [target, setTarget] = useState<string | null>(targetId ?? null);
  const [reason, setReason] = useState("");
  const [allowCompleted, setAllowCompleted] = useState(false);
  const [preview, setPreview] = useState<IterationPreview | null>(null);
  const settings = useQuery(iterationSettingsOptions(wsId));
  const targets = useQuery(iterationChoicesOptions(wsId));
  const apply = useIterationCommand(
    wsId,
    `assign:${issueId ?? targetId ?? "batch"}`,
  );
  const pending = apply.pending;
  const selectedTarget = target ?? currentIterationId ?? "";
  const currentIterationName = targets.data?.find((item) => item.id === currentIterationId)?.name
    ?? currentIterationId
    ?? t(($) => $.iterations.unassignedState);
  const prepare = useMutation({
    mutationKey: ["iterations", wsId, "assignment-preview"],
    mutationFn: () =>
      protectIterationRead(client, wsId, async () => {
        const currentSettings = await api.getIterationSettings(wsId);
        const selected = new Set<string>();
        for (const token of ids.split(/[\s,]+/).filter(Boolean)) {
          if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(token)) selected.add(token.toLowerCase());
          else {
            const issue = await api.getIssue(token);
            if (issue.workspace_id !== wsId) throw new Error("Task workspace mismatch");
            selected.add(issue.id);
          }
        }
        const draft = {
          operation: "move" as const,
          iteration_id: null,
          expected_iteration_revision: null,
          expected_scope_revision: null,
          expected_settings_revision: currentSettings.revision,
          reason: reason || null,
          moves: [...selected].map((id) => ({ issue_id: id, expected_issue_revision: 1, expected_source_id: null as string | null, target_id: selectedTarget || null, allow_completed: allowCompleted })),
          start: null,
        };
        const preliminary = await api.previewIteration(wsId, draft);
        const facts = new Map(preliminary.issues.map((issue) => [issue.issue_id, issue]));
        if (preliminary.complete !== true || facts.size !== selected.size || preliminary.issues.length !== selected.size || [...selected].some((id) => !facts.has(id))) {
          throw new Error("Incomplete assignment preview");
        }
        const result = await api.previewIteration(wsId, {
          ...draft,
          moves: draft.moves.map((move) => ({ ...move, expected_issue_revision: facts.get(move.issue_id)!.revision, expected_source_id: facts.get(move.issue_id)!.source_id })),
        });
        if (result.issues.some((issue) => !selected.has(issue.issue_id)) || new Set(result.issues.map((issue) => issue.issue_id)).size !== result.issues.length) throw new Error("Unexpected assignment preview members");
        setPreview(result);
      }),
  });
  const locked = !available || pending !== null || prepare.isPending || apply.isPending;
  function changed() {
    setPreview(null);
  }
  async function submit() {
    if ((!preview && !pending) || (!available && !pending)) return;
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
      setPreview(null);
      if (onAssigned) {
        setIds(issueId ?? issueIds?.join(" ") ?? "");
        setReason("");
        onAssigned();
      }
    } catch (error) {
      if (definitelyRejected(error)) setPreview(null);
    }
  }

  return (
    <>
      {issueId && currentIterationId !== undefined && (
        <PropRow label={t(($) => $.iterations.currentGroup)} interactive={false}>
          <div className="min-w-0 py-1.5">
            <div className="flex min-w-0 items-center gap-1.5">
              <CalendarRange className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
              <span
                className={`truncate ${currentIterationId ? "" : "text-muted-foreground"}`}
                title={currentIterationName}
              >
                {currentIterationName}
              </span>
            </div>
            <p className="mt-0.5 whitespace-normal pl-5 text-caption text-muted-foreground">
              {t(($) => $.iterations.rollover)}: {rolloverCount ?? t(($) => $.iterations.unknownHistory)}
            </p>
          </div>
        </PropRow>
      )}
    <details open={expanded || undefined} className="group/assignment col-span-2 min-w-0">
      <summary className={expanded ? "hidden" : `${iterationDisclosureClass} -mx-2 flex list-none items-center gap-1 px-2 transition-colors hover:bg-accent/50 focus-visible:bg-accent/50 group-open/assignment:text-foreground [&::-webkit-details-marker]:hidden`}>
        <span className="min-w-0">{t(($) => $.iterations.assign)}</span>
        <ChevronRight className="size-3 shrink-0 stroke-[2.5] transition-transform group-open/assignment:rotate-90 motion-reduce:transition-none" aria-hidden />
      </summary>
      <div className={`min-w-0 space-y-3 text-caption ${expanded ? "" : "pb-3 pt-2"}`}>
        {(rolloverCount ?? 0) >= 3 && <p className="text-muted-foreground">{t(($) => $.iterations.rolloverReview)}</p>}
        {issueIds && <p className="text-muted-foreground">{t(($) => $.iterations.affected)}: <span className="font-medium tabular-nums text-foreground">{issueIds.length}</span></p>}
        <fieldset
          disabled={!available || pending !== null || prepare.isPending || apply.isPending}
          className="min-w-0 space-y-3"
        >
          {!issueId && !issueIds && (
            <>
              <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" onClick={() => setPickerOpen(true)}>
                {t(($) => $.iterations.selectTasks)}
              </Button>
              <IssuePickerModal
                open={pickerOpen}
                onOpenChange={setPickerOpen}
                title={t(($) => $.iterations.selectTasks)}
                description={t(($) => $.iterations.assign)}
                excludeIds={ids.split(/[\s,]+/)}
                filterIssue={(issue) =>
                  issue.workspace_id === wsId &&
                  !issueBehavesAs(issue, "cancelled") &&
                  (!issueBehavesAs(issue, "done") || (allowCompleted && targets.data?.find((item) => item.id === selectedTarget)?.status === "active")) &&
                  [undefined, "not_required", "accepted"].includes(
                    issue.admission_status,
                  )
                }
                onSelect={(issue) => {
                  setIds((previous) =>
                    [previous, issue.id].filter(Boolean).join(" "),
                  );
                  changed();
                  setPickerOpen(false);
                }}
              />
            </>
          )}
          {!issueId && !issueIds && (
            <label className="grid min-w-0 gap-1.5 text-caption text-muted-foreground">
              {t(($) => $.iterations.issueId)}
              <Input
                className="text-caption text-foreground md:text-caption pointer-coarse:min-h-11"
                value={ids}
                onChange={(e) => {
                  setIds(e.target.value);
                  changed();
                }}
              />
            </label>
          )}
          {!targetId && (
            <IterationSelect
              disabled={locked}
              value={selectedTarget}
              onChange={(value) => {
                setTarget(value);
                changed();
              }}
              items={targets.data ?? []}
            />
          )}
          <label className="grid min-w-0 gap-1.5 text-caption text-muted-foreground">
            {t(($) => $.iterations.reason)}
            <Input
              className="text-caption text-foreground md:text-caption pointer-coarse:min-h-11"
              value={reason}
              onChange={(e) => {
                setReason(e.target.value);
                changed();
              }}
            />
          </label>
          <label htmlFor={allowCompletedId} className="flex min-h-8 cursor-pointer items-start gap-2 py-1.5 text-caption text-muted-foreground pointer-coarse:min-h-11">
            <Checkbox
              id={allowCompletedId}
              className="mt-0.5 pointer-coarse:after:-inset-3.5"
              disabled={locked}
              checked={allowCompleted}
              onCheckedChange={(checked) => {
                setAllowCompleted(checked);
                changed();
              }}
            />
            {t(($) => $.iterations.allowCompleted)}
          </label>
          <Button
            size="sm"
            className="pointer-coarse:min-h-11"
            disabled={!available || prepare.isPending || !settings.data || !ids.trim()}
            onClick={() => { if (available) prepare.mutate(); }}
          >
            {t(($) => $.iterations.preview)}
          </Button>
        </fieldset>
        {(prepare.error || apply.error) && (
          <div className="text-destructive">
            <IterationError error={prepare.error ?? apply.error} />
          </div>
        )}
        {preview && (
          <div className="min-w-0 space-y-3">
            <p className="text-muted-foreground">
              {t(($) => $.iterations.affected)}: <span className="font-medium tabular-nums text-foreground">{preview.total_affected}</span>
            </p>
            <ul className="max-h-48 space-y-2 overflow-y-auto">
              {preview.issues.map((issue) => (
                <li key={issue.issue_id} className="min-w-0 break-words">
                  <span className="text-muted-foreground">{issue.identifier}</span> · {issue.title}
                  <p className="mt-0.5 text-muted-foreground">{t(($) => $.iterations.source)}: {targets.data?.find((item) => item.id === issue.source_id)?.name ?? issue.source_id ?? t(($) => $.iterations.unassignedState)}</p>
                </li>
              ))}
            </ul>
            {preview.invalid_items.map((item, index) => <p role="alert" className="text-destructive" key={`${item.issue_id}:${index}`}>{labels.validation(item.code)}</p>)}
            <Button
              size="sm"
              className="pointer-coarse:min-h-11"
              disabled={
                !available ||
                apply.isPending ||
                prepare.isPending ||
                preview.complete !== true ||
                preview.invalid_items.length > 0
              }
              onClick={() => void submit()}
            >
            {t(($) => $.iterations.assign)}
            </Button>
          </div>
        )}
        {apply.isPending && <p role="status" className="text-muted-foreground">{t(($) => $.iterations.processing)}</p>}
        {prepare.isPending && <p role="status" className="text-muted-foreground">{t(($) => $.iterations.preparing)}</p>}
        {pending && !apply.isPending && <p role="status" className="text-muted-foreground">{t(($) => $.iterations.unknownResult)}</p>}
        {pending && (
          <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" disabled={apply.isPending} onClick={() => void submit()}>
            {t(($) => $.iterations.retry)}
          </Button>
        )}
      </div>
    </details>
    </>
  );
}
export function IterationSelect({
  value,
  onChange,
  items,
  label,
  disabled = false,
  descriptionId,
}: {
  value: string;
  onChange: (id: string) => void;
  items: { id: string; name: string; status: string }[];
  label?: string;
  disabled?: boolean;
  descriptionId?: string;
}) {
  const { t } = useT("projects");
  const id = useId();
  const options = [
    { value: "", label: t(($) => $.iterations.unassigned) },
    ...items.filter((item) => ["planned", "active"].includes(item.status)).map((item) => ({ value: item.id, label: item.name })),
  ];
  return (
    <div className="grid min-w-0 gap-1.5 text-caption text-muted-foreground">
      <label htmlFor={id} className="[overflow-wrap:anywhere]">{label ?? t(($) => $.iterations.title)}</label>
      <Select
        items={options}
        disabled={disabled}
        value={value}
        onValueChange={(value) => onChange(value ?? "")}
      >
        <SelectTrigger id={id} aria-describedby={descriptionId} className="w-full min-w-0 text-caption text-foreground pointer-coarse:min-h-11"><SelectValue /></SelectTrigger>
        <SelectContent>
          {options.map((option) => <SelectItem value={option.value} key={option.value} className="pointer-coarse:min-h-11 [&>span:first-child]:min-w-0 [&>span:first-child]:shrink [&>span:first-child]:whitespace-normal"><span className="[overflow-wrap:anywhere]">{option.label}</span></SelectItem>)}
        </SelectContent>
      </Select>
    </div>
  );
}
export function IterationCandidate({
  wsId,
  value,
  onChange,
  triggerRender,
}: {
  wsId: string;
  value?: string | null;
  onChange: (id: string | null, revision?: number) => void;
  triggerRender?: React.ReactElement;
}) {
  const { t } = useT("projects");
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  const list = useQuery({
    ...iterationChoicesOptions(wsId),
    enabled: capability.data?.enabled === true,
  });
  if (capability.data?.enabled !== true) return null;

  function select(id: string | null) {
    onChange(id, list.data?.find((item) => item.id === id)?.revision);
    setOpen(false);
  }

  if (!triggerRender) {
    return (
      <IterationSelect
        value={value ?? ""}
        onChange={(id) => select(id || null)}
        items={list.data ?? []}
      />
    );
  }

  const current = list.data?.find((item) => item.id === value);
  const label = current?.name ?? t(($) => value ? $.iterations.noSelection : $.iterations.noIteration);
  const triggerLabel = `${t(($) => $.iterations.title)}: ${label}`;
  const query = filter.trim().toLowerCase();
  const filtered = (list.data ?? []).filter(
    (item) => item.name.toLowerCase().includes(query) || matchesPinyin(item.name, query),
  );

  return (
    <div className="inline-flex min-w-0">
      <PropertyPicker
        open={open}
        onOpenChange={setOpen}
        triggerRender={triggerRender}
        trigger={
          <>
            <CalendarRange className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
            <span className="sr-only">{triggerLabel}</span>
            <span className="truncate" aria-hidden>{label}</span>
          </>
        }
        tooltip={triggerLabel}
        width="w-64"
        align="start"
        searchable
        searchPlaceholder={t(($) => $.iterations.search)}
        onSearchChange={setFilter}
        navigationResetKey={JSON.stringify([wsId, filtered.map((item) => [item.id, item.revision])])}
      >
        <PickerItem hoverClassName="hover:bg-accent pointer-coarse:min-h-11" emptyValue selected={!value} onClick={() => select(null)}>
          <CalendarRange className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
          <span className="truncate text-muted-foreground">{t(($) => $.iterations.noIteration)}</span>
        </PickerItem>
        {list.isPending ? (
          <div role="status" className="space-y-2 px-2 py-3">
            <span className="sr-only">{t(($) => $.iterations.loading)}</span>
            <Skeleton className="h-8 w-full" /><Skeleton className="h-8 w-3/4" />
          </div>
        ) : list.error ? (
          <div className="space-y-2 px-2 py-3 text-body">
            <IterationError error={list.error} />
            <Button variant="outline" size="sm" className="pointer-coarse:min-h-11" onClick={() => void list.refetch()} disabled={list.isFetching}>
              {t(($) => $.iterations.retry)}
            </Button>
          </div>
        ) : (
          <>
            {filtered.map((item) => (
              <PickerItem
                hoverClassName="hover:bg-accent pointer-coarse:min-h-11"
                key={item.id}
                selected={item.id === value}
                onClick={() => select(item.id)}
                tooltip={item.name}
              >
                <CalendarRange className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
                <span className="truncate">{item.name}</span>
              </PickerItem>
            ))}
            {filtered.length === 0 && <PickerEmpty />}
          </>
        )}
      </PropertyPicker>
    </div>
  );
}

export function IterationBatchAssignment({ wsId, issueIds }: { wsId: string; issueIds: string[] }) {
  const { t } = useT("projects");
  const [open, setOpen] = useState(false);
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  if (capability.data?.enabled !== true) return null;
  return <>
    <Button variant="ghost" size="sm" className="pointer-coarse:min-h-11" onClick={() => setOpen(true)}>{t(($) => $.iterations.title)}</Button>
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="max-h-[85vh] overflow-auto [&_[data-slot=dialog-close]]:pointer-coarse:min-h-11 [&_[data-slot=dialog-close]]:pointer-coarse:min-w-11">
        <DialogTitle>{t(($) => $.iterations.assign)}</DialogTitle>
        <DialogDescription>{t(($) => $.iterations.affected)}: {issueIds.length}</DialogDescription>
        <IterationAssignment wsId={wsId} issueIds={issueIds} />
      </DialogContent>
    </Dialog>
  </>;
}
