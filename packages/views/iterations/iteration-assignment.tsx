"use client";
import { useState } from "react";
import { CalendarRange } from "lucide-react";
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
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@multica/ui/components/ui/dialog";
import { IssuePickerModal } from "../modals/issue-picker-modal";
import {
  PropertyPicker,
  PickerItem,
  PickerEmpty,
} from "../issues/components/pickers/property-picker";
import { matchesPinyin } from "../editor/extensions/pinyin-match";
import { useIterationLabels } from "./labels";
import { IterationError } from "./iteration-error";
import { useT } from "../i18n";
export function IterationAssignment({
  wsId,
  issueId,
  targetId,
  issueIds,
  currentIterationId,
  rolloverCount,
}: {
  wsId: string;
  issueId?: string;
  targetId?: string;
  issueIds?: string[];
  currentIterationId?: string | null;
  rolloverCount?: number;
}) {
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  if (capability.error && isIterationAccessDenied(capability.error))
    return <IterationError error={capability.error} />;
  if (capability.data?.enabled !== true) return null;
  return (
    <Assignment
      key={JSON.stringify([wsId, issueId ?? null, targetId ?? null, issueIds ?? null])}
      wsId={wsId}
      issueId={issueId}
      targetId={targetId}
      issueIds={issueIds}
      currentIterationId={currentIterationId}
      rolloverCount={rolloverCount}
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
}: {
  wsId: string;
  issueId?: string;
  targetId?: string;
  issueIds?: string[];
  currentIterationId?: string | null;
  rolloverCount?: number;
}) {
  const { t } = useT("projects");
  const client = useQueryClient();
  const labels = useIterationLabels();
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
  function changed() {
    setPreview(null);
  }
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
      setPreview(null);
    } catch (error) {
      if (definitelyRejected(error)) setPreview(null);
    }
  }

  return (
    <>
      {issueId && currentIterationId !== undefined && <p>{t(($) => $.iterations.currentGroup)}: {targets.data?.find((item) => item.id === currentIterationId)?.name ?? currentIterationId ?? t(($) => $.iterations.unassigned)} · {t(($) => $.iterations.rollover)}: {rolloverCount ?? t(($) => $.iterations.unknownHistory)}</p>}
    <details>
      <summary className="cursor-pointer font-medium">
        {t(($) => $.iterations.assign)}
      </summary>
      <div className="mt-3 space-y-3">
        {(rolloverCount ?? 0) >= 3 && <p>{t(($) => $.iterations.rolloverReview)}</p>}
        {issueIds && <p>{t(($) => $.iterations.affected)}: {issueIds.length}</p>}
        <fieldset
          disabled={pending !== null || prepare.isPending || apply.isPending}
          className="space-y-3"
        >
          {!issueId && !issueIds && (
            <>
              <Button variant="outline" onClick={() => setPickerOpen(true)}>
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
            <label className="block">
              {t(($) => $.iterations.issueId)}
              <Input
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
              value={selectedTarget}
              onChange={(value) => {
                setTarget(value);
                changed();
              }}
              items={targets.data ?? []}
            />
          )}
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
          <label className="flex gap-2">
            <input
              type="checkbox"
              checked={allowCompleted}
              onChange={(e) => {
                setAllowCompleted(e.target.checked);
                changed();
              }}
            />
            {t(($) => $.iterations.allowCompleted)}
          </label>
          <Button
            disabled={prepare.isPending || !settings.data || !ids.trim()}
            onClick={() => prepare.mutate()}
          >
            {t(($) => $.iterations.preview)}
          </Button>
        </fieldset>
        {(prepare.error || apply.error) && (
          <IterationError error={prepare.error ?? apply.error} />
        )}
        {preview && (
          <div>
            <p>
              {t(($) => $.iterations.affected)}: {preview.total_affected}
            </p>
            <ul>
              {preview.issues.map((issue) => (
                <li key={issue.issue_id} className="break-words">
                  {issue.identifier} · {issue.title}
                  <p>{t(($) => $.iterations.source)}: {targets.data?.find((item) => item.id === issue.source_id)?.name ?? issue.source_id ?? t(($) => $.iterations.unassigned)}</p>
                </li>
              ))}
            </ul>
            {preview.invalid_items.map((item, index) => <p role="alert" key={`${item.issue_id}:${index}`}>{labels.validation(item.code)}</p>)}
            <Button
              disabled={
                apply.isPending ||
                prepare.isPending ||
                preview.complete !== true ||
                preview.invalid_items.length > 0
              }
              onClick={() => void submit()}
            >
              {t(($) => $.iterations.confirm)}
            </Button>
          </div>
        )}
        {apply.isPending && <p role="status">{t(($) => $.iterations.processing)}</p>}
        {prepare.isPending && <p role="status">{t(($) => $.iterations.preparing)}</p>}
        {pending && !apply.isPending && <p role="status">{t(($) => $.iterations.unknownResult)}</p>}
        {pending && (
          <Button disabled={apply.isPending} onClick={() => void submit()}>
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
}: {
  value: string;
  onChange: (id: string) => void;
  items: { id: string; name: string; status: string }[];
}) {
  const { t } = useT("projects");
  return (
    <label className="block">
      {t(($) => $.iterations.title)}
      <select
        className="block w-full rounded-md border bg-background p-2"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">{t(($) => $.iterations.unassigned)}</option>
        {items
          .filter((item) => ["planned", "active"].includes(item.status))
          .map((item) => (
            <option value={item.id} key={item.id}>
              {item.name}
            </option>
          ))}
      </select>
    </label>
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
        <PickerItem emptyValue selected={!value} onClick={() => select(null)}>
          <CalendarRange className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
          <span className="truncate text-muted-foreground">{t(($) => $.iterations.noIteration)}</span>
        </PickerItem>
        {list.isPending ? (
          <p role="status" className="px-2 py-3 text-body text-muted-foreground">
            {t(($) => $.iterations.loading)}
          </p>
        ) : list.error ? (
          <div className="space-y-2 px-2 py-3 text-body">
            <IterationError error={list.error} />
            <Button variant="outline" size="sm" onClick={() => void list.refetch()} disabled={list.isFetching}>
              {t(($) => $.iterations.retry)}
            </Button>
          </div>
        ) : (
          <>
            {filtered.map((item) => (
              <PickerItem
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
    <Button variant="ghost" size="sm" onClick={() => setOpen(true)}>{t(($) => $.iterations.title)}</Button>
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="max-h-[85vh] overflow-auto">
        <DialogTitle>{t(($) => $.iterations.assign)}</DialogTitle>
        <DialogDescription>{t(($) => $.iterations.affected)}: {issueIds.length}</DialogDescription>
        <IterationAssignment wsId={wsId} issueIds={issueIds} />
      </DialogContent>
    </Dialog>
  </>;
}
