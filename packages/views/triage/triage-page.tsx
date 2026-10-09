"use client";
import { useDeferredValue, useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useCurrentMember } from "@multica/core/permissions";
import {
  isEditableShortcutTarget,
  isPortalLayerShortcutTarget,
} from "@multica/core/shortcuts";
import {
  triageSettingsOptions,
  triageCountsOptions,
  triageListOptions,
  triageDetailOptions,
  triageHistoryOptions,
  type TriageListParams,
  type TriageItem,
  type TriageActionName,
  type TriageActionResult,
} from "@multica/core/triage";
import { memberListOptions } from "@multica/core/workspace/queries";
import {
  ArrowLeft,
  Inbox,
  Plus,
  Upload,
  SlidersHorizontal,
} from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { useIsCompact } from "@multica/ui/hooks/use-mobile";
import { cn } from "@multica/ui/lib/utils";
import { ErrorBoundary } from "@multica/ui/components/common/error-boundary";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@multica/ui/components/ui/resizable";
import { AppLink, useNavigation } from "../navigation";
import { PageHeader } from "../layout/page-header";
import { PriorityIcon } from "../issues/components/priority-icon";
import { IssueDetail } from "../issues/components/issue-detail";
import { useT } from "../i18n";
import { TriageCreateDialog } from "./triage-create-dialog";
import { TriageActionDialog } from "./triage-action-dialog";
import { TriageBatchDialog } from "./triage-batch-dialog";
import { TriageImportDialog } from "./triage-import-dialog";
import {
  TriageItemHistory,
  TriageExecutionStatus,
  TriageDuplicateTarget,
} from "./triage-history";
import { TriageHistoryTimeline } from "./triage-history-timeline";
import { TriageHistoryFilterSummary } from "./triage-history-filter-summary";
import { TriageSelect } from "./triage-fields";
import { TriageFilters } from "./triage-filters";
import { useTriageRoute } from "./use-triage-route";
import {
  nextTriageSelection,
  TRIAGE_CONTROL,
  TRIAGE_HISTORY_FILTER_KEYS,
  triageHistoryFilters,
} from "./triage-ui";

const EMPTY_TRIAGE_ITEMS: TriageItem[] = [];

const VIEWS = ["ready", "all", "snoozed", "history"] as const;
const FILTERS = [
  "source",
  "priority",
  "project_id",
  "label_id",
  "reviewer_id",
  "creator_id",
  "entered_after",
  "entered_before",
  "result",
  "processed_by",
  "processed_after",
  "processed_before",
] as const;

export function TriagePage() {
  const wsId = useWorkspaceId();
  return <TriageWorkspacePage key={wsId} wsId={wsId} />;
}

function TriageWorkspacePage({ wsId }: { wsId: string }) {
  const { t, i18n } = useT("triage");
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const { params, changeParams, replaceParams } = useTriageRoute(
    navigation,
    paths.triage(),
  );
  const compact = useIsCompact();
  const settings = useQuery(triageSettingsOptions(wsId));
  const { role } = useCurrentMember(wsId);
  const supported = settings.data?.supported === true;
  const enabled = settings.data?.enabled === true;
  const canReview = enabled && role !== null;
  const view = VIEWS.find((v) => v === params.get("view")) ?? "ready";
  const selectedId = params.get("issue") ?? "";
  const q = useDeferredValue(params.get("q") ?? "");
  const offset = Math.max(0, Number(params.get("offset")) || 0);
  const sort =
    params.get("sort") === "newest"
      ? "newest"
      : params.get("sort") === "priority"
        ? "priority"
        : "oldest";
  const filters = Object.fromEntries(
    FILTERS.flatMap((key) =>
      params.get(key) ? [[key, params.get(key)!]] : [],
    ),
  );
  const queryParams: TriageListParams = {
    ...filters,
    q,
    view,
    sort,
    offset,
    limit: 50,
  };
  const list = useQuery({
    ...triageListOptions(wsId, queryParams),
    enabled: supported && enabled && view !== "history",
  });
  const counts = useQuery({ ...triageCountsOptions(wsId), enabled: supported });
  const history = useQuery({
    ...triageHistoryOptions(wsId, { q, ...filters, offset, limit: 50 }),
    enabled: supported && view === "history",
  });
  const detail = useQuery({
    ...triageDetailOptions(wsId, selectedId),
    enabled: supported && !!selectedId,
  });
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [importOpen, setImportOpen] = useState<{ batchId?: string } | null>(
    null,
  );
  const importSelection = params.get("batch")
    ? { batchId: params.get("batch")! }
    : importOpen;
  const [batchOpen, setBatchOpen] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [action, setAction] = useState<TriageActionName | null>(null);
  const queueRef = useRef<HTMLDivElement>(null);
  const queueScrollRef = useRef(0);
  const historyScrollRef = useRef(0);
  const emptyRef = useRef<HTMLHeadingElement>(null);
  const [focusTarget, setFocusTarget] = useState<string | null>(null);
  const rows = list.data?.items ?? EMPTY_TRIAGE_ITEMS;
  const item = detail.data;
  const pending = item?.issue.admission_status === "pending";
  const focusRow = (id: string) => setFocusTarget(id);
  useEffect(() => {
    if (focusTarget === null || action !== null) return;
    const button = [
      ...(queueRef.current?.querySelectorAll<HTMLButtonElement>(
        "[data-triage-row]",
      ) ?? []),
    ].find((el) => el.dataset.triageRow === focusTarget);
    const target = button ?? (rows.length === 0 ? emptyRef.current : null);
    if (!target) return;
    const frame = requestAnimationFrame(() => {
      target.focus();
      setFocusTarget(null);
    });
    return () => cancelAnimationFrame(frame);
  }, [action, focusTarget, rows]);
  const afterDecision = (result: TriageActionResult) => {
    setSelected((old) => {
      const next = new Set(old);
      next.delete(result.item.issue.id);
      return next;
    });
    if (
      result.action.execution_status === "failed" ||
      result.action.execution_status === "pending" ||
      result.action.action === "assign_reviewer" ||
      result.action.action === "reopen" ||
      result.action.action === "unsnooze"
    )
      return;
    const next = nextTriageSelection(
      rows.map((row) => row.issue.id),
      result.item.issue.id,
    );
    changeParams({ issue: next });
    focusRow(next);
  };
  const filtered = !!q || FILTERS.some((key) => params.has(key));
  const historyFilters = triageHistoryFilters(params);
  const clearHistoryFilters = () =>
    changeParams(
      Object.fromEntries(
        [...TRIAGE_HISTORY_FILTER_KEYS, "offset"].map((key) => [key, ""]),
      ),
    );
  const total =
    view === "history" ? (history.data?.total ?? 0) : (list.data?.total ?? 0);
  const queue = (
    <div
      ref={queueRef}
      role="region"
      aria-label={t(($) => $.queue_label)}
      tabIndex={-1}
      className="flex h-full min-h-0 flex-col"
      onKeyDown={(event) => {
        if (
          event.defaultPrevented ||
          event.nativeEvent.isComposing ||
          event.metaKey ||
          event.ctrlKey ||
          event.altKey ||
          action ||
          batchOpen ||
          createOpen ||
          importSelection ||
          isEditableShortcutTarget(event.target) ||
          isPortalLayerShortcutTarget(event.target)
        )
          return;
        if (
          !queueRef.current?.contains(document.activeElement) ||
          document.querySelector(
            '[role="dialog"], [role="alertdialog"], [data-base-ui-portal]',
          )
        )
          return;
        const key = event.key.toLowerCase();
        if (key === "j" || key === "k") {
          event.preventDefault();
          const index = rows.findIndex((row) => row.issue.id === selectedId);
          const next =
            rows[
              Math.max(
                0,
                Math.min(rows.length - 1, index + (key === "j" ? 1 : -1)),
              )
            ];
          if (next) {
            changeParams({ issue: next.issue.id });
            focusRow(next.issue.id);
          }
        }
        if (!canReview) return;
        if (key === "c") {
          event.preventDefault();
          setCreateOpen(true);
        }
        if (!pending) return;
        const actionKey: Record<string, TriageActionName> = {
          "1": "accept",
          "2": "duplicate",
          "3": "reject",
          h: "snooze",
        };
        if (actionKey[key]) {
          event.preventDefault();
          setAction(actionKey[key]);
        }
      }}
    >
      {selected.size > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-surface-border p-3">
          <span className="text-caption">
            {t(($) => $.selected, { count: selected.size })}
          </span>
          <Button
            variant="outline"
            className={TRIAGE_CONTROL}
            disabled={!canReview}
            onClick={() => setBatchOpen(true)}
          >
            {t(($) => $.batch)}
          </Button>
        </div>
      )}
      <div
        className="min-h-0 flex-1 overflow-y-auto"
        ref={(node) => {
          if (node) node.scrollTop = queueScrollRef.current;
        }}
        onScroll={(event) => {
          queueScrollRef.current = event.currentTarget.scrollTop;
        }}
      >
        {list.isPending && enabled ? (
          <p className="p-6" role="status">
            {t(($) => $.loading)}
          </p>
        ) : list.isError ? (
          <div className="p-6" role="alert">
            <p>{list.error.message}</p>
            <Button onClick={() => void list.refetch()}>
              {t(($) => $.retry)}
            </Button>
          </div>
        ) : rows.length === 0 ? (
          <div className="flex h-full min-h-48 flex-col items-center justify-center gap-3 p-6 text-center">
            <Inbox
              className="size-7 text-muted-foreground"
              aria-hidden="true"
            />
            <h2 ref={emptyRef} tabIndex={-1} className="text-body font-medium">
              {filtered
                ? t(($) => $.no_matches)
                : view === "ready" && (counts.data?.snoozed ?? 0) > 0
                  ? t(($) => $.only_snoozed)
                  : t(($) => $.empty)}
            </h2>
            <p className="max-w-xs text-caption text-muted-foreground">
              {t(($) => $.empty_description)}
            </p>
            {filtered && (
              <Button
                variant="outline"
                onClick={() => {
                  const next = new URLSearchParams();
                  next.set("view", view);
                  replaceParams(next);
                }}
              >
                {t(($) => $.clear_filters)}
              </Button>
            )}
          </div>
        ) : (
          rows.map((row) => {
            const reviewer = members.find((m) => m.user_id === row.reviewer_id);
            return (
              <div
                key={row.issue.id}
                className={cn(
                  "flex items-start gap-2 border-b border-surface-border px-3 py-1 hover:bg-accent/50",
                  selectedId === row.issue.id &&
                    "bg-accent text-accent-foreground hover:bg-accent",
                )}
              >
                <div className="flex min-h-11 items-center">
                  <Checkbox
                    checked={selected.has(row.issue.id)}
                    aria-label={t(($) => $.select_task, {
                      title: row.issue.title,
                    })}
                    disabled={!canReview}
                    onCheckedChange={(checked) =>
                      setSelected((old) => {
                        const next = new Set(old);
                        if (checked) next.add(row.issue.id);
                        else next.delete(row.issue.id);
                        return next;
                      })
                    }
                  />
                </div>
                <button
                  type="button"
                  data-triage-row={row.issue.id}
                  aria-label={`${row.issue.identifier} ${row.issue.title}`}
                  aria-pressed={selectedId === row.issue.id}
                  className="min-h-11 min-w-0 flex-1 space-y-1 rounded-md py-2 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  onClick={() => changeParams({ issue: row.issue.id })}
                >
                  <span className="flex flex-wrap items-baseline gap-x-2">
                    <span className="text-caption text-muted-foreground">
                      {row.issue.identifier}
                    </span>
                    <span
                      className={cn(
                        "break-words text-body",
                        selectedId === row.issue.id && "font-semibold",
                      )}
                    >
                      {row.issue.title}
                    </span>
                  </span>
                  <span className="block text-caption text-muted-foreground">
                    {row.source === "csv"
                      ? t(($) => $.csv)
                      : row.source === "manual"
                        ? t(($) => $.manual)
                        : row.source}{" "}
                    ·{" "}
                    {new Date(row.entered_at).toLocaleDateString(i18n.language)}
                  </span>
                  <span className="flex items-center gap-1.5 text-caption text-muted-foreground">
                    <PriorityIcon priority={row.issue.priority ?? "none"} />
                    {t(($) => $.priorities[row.issue.priority ?? "none"])}
                  </span>
                  {row.reviewer_id && (
                    <span className="block text-caption text-muted-foreground">
                      {row.reviewer_valid
                        ? (reviewer?.name ?? t(($) => $.unknown))
                        : t(($) => $.invalid_reviewer)}
                    </span>
                  )}
                  {row.snoozed_until && (
                    <span className="block text-caption text-muted-foreground">
                      {t(($) => $.snoozed_until, {
                        date: new Date(row.snoozed_until).toLocaleString(
                          i18n.language,
                        ),
                      })}
                    </span>
                  )}
                </button>
              </div>
            );
          })
        )}
      </div>
      <Pagination
        offset={offset}
        total={total}
        onChange={(next) => {
          setSelected(new Set());
          changeParams({ offset: String(next) });
        }}
      />
    </div>
  );
  const detailPane = selectedId ? (
    <div className="flex h-full min-h-0 flex-col">
      {detail.isError && (
        <p role="alert" className="p-4">
          {detail.error.message}
        </p>
      )}
      {item && (
        <div className="shrink-0 space-y-2 border-b border-surface-border p-3">
          <div className="flex flex-wrap items-center gap-2">
            {compact && (
              <Button
                variant="ghost"
                className={TRIAGE_CONTROL}
                onClick={() => changeParams({ issue: "" })}
              >
                <ArrowLeft className="size-4" />
                {t(($) => $.back)}
              </Button>
            )}
            <span className="text-caption text-muted-foreground">
              {t(($) => $.round, { round: item.round })} ·{" "}
              {item.source === "csv" ? t(($) => $.csv) : t(($) => $.manual)}
            </span>
            {item.filename && (
              <span className="break-all text-caption text-muted-foreground">
                {item.filename} ·{" "}
                {t(($) => $.row_number, { row: item.row_number })}
              </span>
            )}
          </div>
          {item.snoozed_until && (
            <p className="text-caption">
              {t(($) => $.snoozed_until, {
                date: new Date(item.snoozed_until).toLocaleString(
                  i18n.language,
                  { timeZoneName: "short" },
                ),
              })}
            </p>
          )}
          {item.duplicate_identifier && (
            <TriageDuplicateTarget
              wsId={wsId}
              identifier={item.duplicate_identifier}
              issueId={item.duplicate_issue_id}
            />
          )}
          <TriageExecutionStatus wsId={wsId} issueId={item.issue.id} />
          <details>
            <summary className="min-h-8 cursor-pointer text-caption text-muted-foreground">
              {t(($) => $.history)}
            </summary>
            <div className="max-h-64 overflow-y-auto">
              <p className="text-caption">
                {t(($) => $.first_entered, {
                  date: new Date(item.first_entered_at).toLocaleString(
                    i18n.language,
                  ),
                })}
              </p>
              {item.external_id && (
                <p className="text-caption">
                  {t(($) => $.external_id)}: {item.external_id}
                </p>
              )}
              {item.source_url && /^https?:\/\//i.test(item.source_url) && (
                <a
                  className="text-caption underline"
                  href={item.source_url}
                  rel="noreferrer"
                  target="_blank"
                >
                  {t(($) => $.source_url)}
                </a>
              )}
              <TriageItemHistory wsId={wsId} issueId={item.issue.id} />
            </div>
          </details>
        </div>
      )}
      <div className="flex min-h-0 flex-1 flex-col">
        <ErrorBoundary key={selectedId}>
          <IssueDetail
            issueId={selectedId}
            defaultSidebarOpen={false}
            layoutId="triage-issue-detail"
            onDelete={() => {
              changeParams({ issue: "" });
              focusRow("");
            }}
          />
        </ErrorBoundary>
      </div>
      {item && (
        <div className="shrink-0 border-t border-surface-border bg-background p-3">
          {" "}
          {pending && canReview && (
            <div className="flex flex-wrap gap-2">
              {(
                [
                  "accept",
                  "duplicate",
                  "reject",
                  "snooze",
                  "assign_reviewer",
                ] as const
              ).map((name) => (
                <Button
                  key={name}
                  variant={name === "accept" ? "default" : "outline"}
                  className={TRIAGE_CONTROL}
                  onClick={() => setAction(name)}
                >
                  {t(($) => $[name])}
                </Button>
              ))}
              <Button
                variant="outline"
                className={TRIAGE_CONTROL}
                onClick={() => setAction("accept_and_execute")}
              >
                {t(($) => $.accept_and_execute)}
              </Button>
              {item.snoozed_until && (
                <Button
                  variant="ghost"
                  className={TRIAGE_CONTROL}
                  onClick={() => setAction("unsnooze")}
                >
                  {t(($) => $.unsnooze)}
                </Button>
              )}
            </div>
          )}
          {(item.issue.admission_status === "rejected" ||
            item.issue.admission_status === "duplicate") && (
            <Button
              variant="outline"
              className={TRIAGE_CONTROL}
              disabled={!canReview}
              onClick={() => setAction("reopen")}
            >
              {t(($) => $.reopen)}
            </Button>
          )}
        </div>
      )}
    </div>
  ) : (
    <div className="flex h-full items-center justify-center p-6 text-body text-muted-foreground">
      {t(($) => $.choose_task)}
    </div>
  );
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader>
        <h1 className="min-w-0 flex-1 text-body font-semibold">
          {t(($) => $.title)}
        </h1>
        {enabled && (
          <>
            <Button
              variant="ghost"
              className={TRIAGE_CONTROL}
              aria-label={t(($) => $.import_csv)}
              onClick={() => setImportOpen({})}
            >
              <Upload className="size-4" />
              <span className="hidden sm:inline">{t(($) => $.import_csv)}</span>
              <span className="sr-only sm:hidden">
                {t(($) => $.import_csv)}
              </span>
            </Button>
            <Button
              className={TRIAGE_CONTROL}
              aria-label={t(($) => $.create)}
              onClick={() => setCreateOpen(true)}
            >
              <Plus className="size-4" />
              <span className="hidden sm:inline">{t(($) => $.create)}</span>
              <span className="sr-only sm:hidden">{t(($) => $.create)}</span>
            </Button>
          </>
        )}
      </PageHeader>
      {settings.isPending ? (
        <p role="status" className="p-6">
          {t(($) => $.loading)}
        </p>
      ) : settings.isError ? (
        <div role="alert" className="p-6">
          <p>{settings.error.message}</p>
          <Button onClick={() => void settings.refetch()}>
            {t(($) => $.retry)}
          </Button>
        </div>
      ) : !supported ? (
        <p className="p-6">{t(($) => $.unsupported)}</p>
      ) : (
        <>
          {!enabled && (
            <div className="space-y-1 border-b border-surface-border px-4 py-3 text-body">
              <p>{t(($) => $.disabled)}</p>
              <AppLink
                className="text-caption underline"
                href={`${paths.settings()}?tab=triage`}
              >
                {t(($) => $.disabled_hint)}
              </AppLink>
            </div>
          )}
          <div
            className="flex shrink-0 flex-wrap items-center gap-1 border-b border-surface-border px-4 py-1"
            role="tablist"
            aria-label={t(($) => $.title)}
          >
            {VIEWS.map((name) => (
              <Button
                key={name}
                role="tab"
                aria-selected={view === name}
                id={`triage-view-${name}`}
                aria-controls="triage-view-panel"
                tabIndex={view === name ? 0 : -1}
                onKeyDown={(event) => {
                  const index = VIEWS.indexOf(name);
                  const nextIndex =
                    event.key === "ArrowRight"
                      ? (index + 1) % VIEWS.length
                      : event.key === "ArrowLeft"
                        ? (index + VIEWS.length - 1) % VIEWS.length
                        : event.key === "Home"
                          ? 0
                          : event.key === "End"
                            ? VIEWS.length - 1
                            : -1;
                  if (nextIndex < 0) return;
                  event.preventDefault();
                  const nextView = VIEWS[nextIndex]!;
                  setSelected(new Set());
                  changeParams({ view: nextView, offset: "", issue: "" });
                  document.getElementById(`triage-view-${nextView}`)?.focus();
                }}
                variant={view === name ? "secondary" : "ghost"}
                className={TRIAGE_CONTROL}
                onClick={() => {
                  setSelected(new Set());
                  changeParams({ view: name, offset: "", issue: "" });
                }}
              >
                {t(($) => $[name])}
                {name !== "history" && (
                  <span className="ml-1 text-caption tabular-nums">
                    {name === "all"
                      ? (counts.data?.pending ?? 0)
                      : (counts.data?.[name] ?? 0)}
                  </span>
                )}
              </Button>
            ))}
          </div>
          <div className="flex shrink-0 items-center gap-2 px-4 py-2">
            <Input
              className={cn("min-w-0 flex-1", TRIAGE_CONTROL)}
              aria-label={t(($) => $.search)}
              placeholder={t(($) => $.search)}
              value={params.get("q") ?? ""}
              onChange={(e) => {
                setSelected(new Set());
                changeParams({ q: e.target.value, offset: "" });
              }}
            />
            <Button
              variant={filtersOpen ? "secondary" : "outline"}
              className={TRIAGE_CONTROL}
              aria-label={t(($) => $.filters)}
              aria-expanded={filtersOpen}
              onClick={() => setFiltersOpen(!filtersOpen)}
            >
              <SlidersHorizontal className="size-4" />
              <span className="hidden sm:inline">{t(($) => $.filters)}</span>
              <span className="sr-only sm:hidden">{t(($) => $.filters)}</span>
            </Button>
            {view !== "history" && (
              <div className="w-36 shrink-0 sm:w-44">
                <TriageSelect
                  label={t(($) => $.sort)}
                  value={sort}
                  onChange={(value) =>
                    changeParams({ sort: value, offset: "" })
                  }
                  options={[
                    { value: "oldest", label: t(($) => $.oldest) },
                    { value: "newest", label: t(($) => $.newest) },
                    { value: "priority", label: t(($) => $.priority_sort) },
                  ]}
                />
              </div>
            )}
          </div>
          {view === "history" && (
            <TriageHistoryFilterSummary
              filters={historyFilters}
              members={members}
              onClear={clearHistoryFilters}
            />
          )}
          {filtersOpen && (
            <TriageFilters
              wsId={wsId}
              params={params}
              history={view === "history"}
              onChange={(key, value) => {
                setSelected(new Set());
                changeParams({ [key]: value, offset: "" });
              }}
            />
          )}
          <div
            id="triage-view-panel"
            role="tabpanel"
            aria-labelledby={`triage-view-${view}`}
            className="min-h-0 flex-1 border-t border-surface-border"
          >
            {view === "history" && !selectedId ? (
              <div className="flex h-full flex-col">
                <div
                  className="min-h-0 flex-1 overflow-y-auto px-4"
                  ref={(node) => {
                    if (node) node.scrollTop = historyScrollRef.current;
                  }}
                  onScroll={(event) => {
                    historyScrollRef.current = event.currentTarget.scrollTop;
                  }}
                >
                  {history.isPending ? (
                    <p role="status" className="p-6">
                      {t(($) => $.loading)}
                    </p>
                  ) : history.isError ? (
                    <p role="alert">{history.error.message}</p>
                  ) : !history.data?.entries.length ? (
                    <div className="flex min-h-48 flex-col items-center justify-center gap-3 p-6 text-center">
                      <h2
                        ref={emptyRef}
                        tabIndex={-1}
                        className="text-body font-medium"
                      >
                        {total > 0
                          ? t(($) => $.history_page_empty)
                          : historyFilters.length > 0
                            ? t(($) => $.history_no_matches)
                            : t(($) => $.history_empty)}
                      </h2>
                      {total > 0 ? (
                        <Button
                          variant="outline"
                          className={TRIAGE_CONTROL}
                          onClick={() => changeParams({ offset: "" })}
                        >
                          {t(($) => $.history_first_page)}
                        </Button>
                      ) : historyFilters.length > 0 ? (
                        <>
                          <p className="text-caption text-muted-foreground">
                            {t(($) => $.history_no_matches_description)}
                          </p>
                          <Button
                            variant="outline"
                            className={TRIAGE_CONTROL}
                            onClick={clearHistoryFilters}
                          >
                            {t(($) => $.clear_filters)}
                          </Button>
                        </>
                      ) : null}
                    </div>
                  ) : (
                    <TriageHistoryTimeline
                      wsId={wsId}
                      entries={history.data.entries}
                      onOpenIssue={(issueId) => changeParams({ issue: issueId })}
                      onOpenImport={(batchId) => setImportOpen({ batchId })}
                    />
                  )}
                </div>
                <Pagination
                  offset={offset}
                  total={total}
                  onChange={(next) => changeParams({ offset: String(next) })}
                />
              </div>
            ) : compact ? (
              selectedId ? (
                detailPane
              ) : (
                queue
              )
            ) : (
              <ResizablePanelGroup orientation="horizontal">
                <ResizablePanel defaultSize="36%" minSize="25%">
                  {view === "history" ? (
                    <div className="p-4">
                      <Button
                        variant="outline"
                        onClick={() => changeParams({ issue: "" })}
                      >
                        {t(($) => $.history)}
                      </Button>
                    </div>
                  ) : (
                    queue
                  )}
                </ResizablePanel>
                <ResizableHandle />
                <ResizablePanel defaultSize="64%" minSize="40%">
                  {detailPane}
                </ResizablePanel>
              </ResizablePanelGroup>
            )}
          </div>
        </>
      )}
      {createOpen && (
        <TriageCreateDialog
          wsId={wsId}
          onClose={() => setCreateOpen(false)}
          onCreated={(created) =>
            changeParams({ issue: created.issue.id, view: "ready" })
          }
        />
      )}
      {action && item && settings.data && (
        <TriageActionDialog
          wsId={wsId}
          item={item}
          action={action}
          settings={settings.data}
          onClose={() => setAction(null)}
          onSuccess={afterDecision}
        />
      )}
      {batchOpen && (
        <TriageBatchDialog
          wsId={wsId}
          items={rows.filter((row) => selected.has(row.issue.id))}
          onClose={() => setBatchOpen(false)}
          onSuccess={(ids) =>
            setSelected(
              (old) => new Set([...old].filter((id) => !ids.includes(id))),
            )
          }
        />
      )}
      {importSelection && (
        <TriageImportDialog
          wsId={wsId}
          batchId={importSelection.batchId}
          onClose={() => {
            setImportOpen(null);
            changeParams({ batch: "" });
          }}
        />
      )}
    </div>
  );
}

function Pagination({
  offset,
  total,
  onChange,
}: {
  offset: number;
  total: number;
  onChange: (offset: number) => void;
}) {
  const { t } = useT("triage");
  return (
    <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-t border-surface-border ps-3 pe-chat-launcher py-2">
      <span className="text-caption text-muted-foreground">
        {t(($) => $.total, { count: total })}
      </span>
      <div className="flex gap-1">
        <Button
          variant="ghost"
          className={TRIAGE_CONTROL}
          disabled={offset === 0}
          onClick={() => onChange(Math.max(0, offset - 50))}
        >
          {t(($) => $.previous)}
        </Button>
        <Button
          variant="ghost"
          className={TRIAGE_CONTROL}
          disabled={offset + 50 >= total}
          onClick={() => onChange(offset + 50)}
        >
          {t(($) => $.next)}
        </Button>
      </div>
    </div>
  );
}
