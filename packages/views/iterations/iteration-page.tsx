"use client";
import { useState } from "react";
import { ApiError } from "@multica/core/api";
import { toast } from "sonner";
import { copyText } from "@multica/ui/lib/clipboard";
import { memberListOptions } from "@multica/core/workspace/queries";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useProjectPlanningTimezone } from "@multica/core/projects";
import { useCurrentMember } from "@multica/core/permissions";
import {
  iterationCapabilitiesOptions,
  iterationSettingsOptions,
  iterationListOptions,
  iterationDetailOptions,
  iterationEventsOptions,
  iterationIsOverdue,
  useIterationCommand,
} from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { AppLink, useNavigation } from "../navigation";
import { useT, useLocale } from "../i18n";
import { formatInTimeZone } from "../common/format-in-time-zone";
import { useIterationLabels } from "./labels";
import { IterationError } from "./iteration-error";
import { IterationOperation } from "./iteration-operation";
import { IterationForm } from "./iteration-form";
import { IterationAssignment } from "./iteration-assignment";
import { IterationRecovery } from "./iteration-recovery";
import { useIterationCatalogue } from "./iteration-catalogue";
import { IterationEventsView } from "./iteration-events-view";
import { IterationIssueList } from "./iteration-issue-list";
import { IterationHistory } from "./iteration-history";

export function IterationsPage() {
  const wsId = useWorkspaceId();
  return (
    <div className="flex h-full min-h-0 flex-col">
      <IterationRecovery key={wsId} wsId={wsId} />
      <div className="min-h-0 flex-1">
        <IterationPageContent key={wsId} wsId={wsId} />
      </div>
    </div>
  );
}
function IterationPageContent({ wsId }: { wsId: string }) {
  const { t } = useT("projects");
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  if (capability.isPending)
    return (
      <p role="status" className="p-6">
        {t(($) => $.iterations.loading)}
      </p>
    );
  if (capability.error && (!capability.data || isDefinitiveReadError(capability.error)))
    return (
      <div className="p-6">
        <IterationError error={capability.error} />
        <Button onClick={() => capability.refetch()}>
          {t(($) => $.iterations.retry)}
        </Button>
      </div>
    );
  if (capability.data?.supported !== true || capability.data.manual !== true)
    return (
      <section className="p-6">
        <h1 className="text-title font-semibold">
          {t(($) => $.iterations.title)}
        </h1>
        <p className="mt-4 text-muted-foreground">
          {t(($) => $.iterations.unsupported)}
        </p>
      </section>
    );
  return (
    <>
      {capability.error && <IterationRefreshError error={capability.error} onRetry={() => void capability.refetch()} />}
      <IterationWorkspace
        key={wsId}
        wsId={wsId}
        atomicHandoff={capability.data.atomic_handoff === true}
      />
    </>
  );
}
function IterationWorkspace({
  wsId,
  atomicHandoff,
}: {
  wsId: string;
  atomicHandoff: boolean;
}) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const id = navigation.pathname.startsWith(paths.iterations() + "/")
    ? decodeURIComponent(
        navigation.pathname.slice(paths.iterations().length + 1),
      )
    : null;
  const settings = useQuery(iterationSettingsOptions(wsId));
  const { role } = useCurrentMember(wsId);
  const client = useQueryClient();
  const changeTimezone = useProjectPlanningTimezone(wsId);
  const canManage = role === "owner" || role === "admin";
  const [timezone, setTimezone] = useState("");
  const enable = useIterationCommand(wsId, "enable");
  if (settings.isPending)
    return <p role="status">{t(($) => $.iterations.loading)}</p>;
  if (!settings.data || isDefinitiveReadError(settings.error))
    return <IterationError error={settings.error} />;
  if (id)
    return (
      <>
        {settings.error && <IterationRefreshError error={settings.error} onRetry={() => void settings.refetch()} />}
        <IterationDetail
          key={`${wsId}:${id}`}
          wsId={wsId}
          id={id}
          settingsRevision={settings.data.revision}
          enabled={settings.data.enabled}
          atomicHandoff={atomicHandoff}
        />
      </>
    );
  return (
    <main className="h-full overflow-auto p-6 space-y-6">
      {settings.error && <IterationRefreshError error={settings.error} onRetry={() => void settings.refetch()} />}
      <header className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-title font-semibold">
          {t(($) => $.iterations.title)}
        </h1>
      </header>
      <details>
        <summary className="cursor-pointer font-medium">
          {t(($) => $.iterations.settings)}
        </summary>
        <div className="mt-4 space-y-3 max-w-xl">
          <p>{t(($) => $.iterations.manualModeDescription)}</p>
          <label className="block">
            {t(($) => $.iterations.timezone)}
            <Input
              value={timezone || settings.data.effective_timezone}
              onChange={(e) => setTimezone(e.target.value)}
              disabled={!canManage || enable.pending !== null}
            />
          </label>
          {canManage && (
            <Button
              disabled={changeTimezone.isPending || !timezone.trim()}
              onClick={() =>
                changeTimezone.mutate(timezone, {
                  onSuccess: () => {
                    void client.invalidateQueries({
                      queryKey: ["iterations", wsId],
                    });
                  },
                })
              }
            >
              {t(($) => $.iterations.saveTimezone)}
            </Button>
          )}
          {changeTimezone.error && (
            <p role="alert">{t(($) => $.iterations.error)}</p>
          )}
          {!settings.data.enabled && (
            <>
              <p>{t(($) => $.iterations.disabled)}</p>
              {canManage && (
                <Button
                  disabled={enable.isPending}
                  onClick={() =>
                    enable.mutate({
                      command: enable.pending ?? {
                        kind: "enable",
                        body: {
                          request_id: crypto.randomUUID(),
                          expected_revision: settings.data!.revision,
                          confirmed_timezone:
                            timezone || settings.data!.effective_timezone,
                        },
                      },
                      recover: enable.pending !== null,
                    })
                  }
                >
                  {t(($) => $.iterations.enable)}
                </Button>
              )}
            </>
          )}
          {enable.error && <p role="alert">{t(($) => $.iterations.error)}</p>}
          <IterationOperation
            available={settings.data.enabled && canManage}
            wsId={wsId}
            iteration={null}
            settingsRevision={settings.data.revision}
            operation="disable"
          />
        </div>
      </details>
      {settings.data.enabled && (
        <IterationForm
          wsId={wsId}
          timezone={settings.data.effective_timezone}
        />
      )}
      <IterationList wsId={wsId} />
    </main>
  );
}
function IterationList({ wsId }: { wsId: string }) {
  const labels = useIterationLabels();
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [cursor, setCursor] = useState("");
  const choices = useIterationCatalogue(wsId);
  const upcoming = choices.data?.filter((item) => item.status === "planned").sort((a, b) => a.start_date.localeCompare(b.start_date) || a.id.localeCompare(b.id))[0]?.id;
  const list = useQuery(
    iterationListOptions(wsId, {
      search,
      ...(from ? { from } : {}),
      ...(to ? { to } : {}),
      ...(status ? { status } : {}),
      ...(cursor ? { cursor } : {}),
    }),
  );
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap gap-3">
        <Input
          aria-label={t(($) => $.iterations.search)}
          placeholder={t(($) => $.iterations.search)}
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setCursor("");
          }}
          className="max-w-sm"
        />
        <label>
          {t(($) => $.iterations.startDate)}
          <Input
            type="date"
            value={from}
            onChange={(e) => {
              setFrom(e.target.value);
              setCursor("");
            }}
          />
        </label>
        <label>
          {t(($) => $.iterations.endDate)}
          <Input
            type="date"
            value={to}
            onChange={(e) => {
              setTo(e.target.value);
              setCursor("");
            }}
          />
        </label>
        <select
          aria-label={t(($) => $.iterations.status)}
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setCursor("");
          }}
          className="rounded-md border bg-background px-3"
        >
          <option value="">{t(($) => $.iterations.all)}</option>
          {["planned", "active", "completed", "cancelled"].map((s) => (
            <option key={s} value={s}>
              {labels.status(s)}
            </option>
          ))}
        </select>
      </div>
      {list.isPending && <p role="status">{t(($) => $.iterations.loading)}</p>}
      {list.error && (
        <div role="alert">
          <p>{t(($) => $.iterations.error)}</p>
          <Button
            onClick={() => {
              if (cursor) setCursor("");
              else void list.refetch();
            }}
          >
            {t(($) => $.iterations.retry)}
          </Button>
        </div>
      )}
      {choices.isSuccess && !choices.data.some((item) => item.status === "active") && <p role="status">{t(($) => $.iterations.noActive)}</p>}
      {!list.error && [
        { label: t(($) => $.iterations.currentGroup), statuses: ["active"] },
        { label: t(($) => $.iterations.futureGroup), statuses: ["planned"] },
        { label: t(($) => $.iterations.history), statuses: ["completed", "cancelled"] },
        { label: t(($) => $.iterations.unknown), statuses: (list.data?.items ?? []).filter((item) => !["active", "planned", "completed", "cancelled"].includes(item.status)).map((item) => item.status) },
      ].map((group) => {
        const candidates = group.statuses[0] === "active" || group.statuses[0] === "planned" ? choices.data ?? [] : list.data?.items ?? [];
        const items = candidates.filter((item) => group.statuses.includes(item.status) && (!status || item.status === status) && item.name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()) && (!from || item.end_date >= from) && (!to || item.start_date <= to));
        return items.length > 0 && <section key={group.label} className="space-y-2">
          <h2 className="text-subtitle font-semibold">{group.label}</h2>
          <ul className="divide-y">{items.map((item) => <li key={item.id} className="py-4">
            <AppLink href={paths.iterationDetail(item.id)} title={item.name} className="font-medium line-clamp-2 break-words hover:underline">{item.name}</AppLink>
            <p className="text-caption text-muted-foreground">{item.start_date} – {item.end_date} · {labels.status(item.status)} · {item.timezone}{item.id === upcoming && <> · {t(($) => $.iterations.upcoming)}</>}</p>
          </li>)}</ul>
        </section>;
      })}
      {list.data?.items.length === 0 && <p>{t(($) => $.iterations.empty)}</p>}
      {list.data?.next_cursor && (
        <Button onClick={() => setCursor(list.data!.next_cursor!)}>
          {t(($) => $.iterations.next)}
        </Button>
      )}
    </section>
  );
}
function IterationDetail({
  wsId,
  id,
  settingsRevision,
  enabled,
  atomicHandoff,
}: {
  wsId: string;
  id: string;
  settingsRevision: number;
  enabled: boolean;
  atomicHandoff: boolean;
}) {
  const labels = useIterationLabels();
  const locale = useLocale();
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const members = useQuery(memberListOptions(wsId));
  const detail = useQuery(iterationDetailOptions(wsId, id));
  if (detail.isPending)
    return <p role="status">{t(($) => $.iterations.loading)}</p>;
  if (!detail.data || isDefinitiveReadError(detail.error))
    return <IterationError error={detail.error} />;
  const { iteration, statistics, snapshot } = detail.data;
  const known = iteration.mode === "manual" && ["planned", "active", "completed", "cancelled"].includes(
    iteration.status,
  );
  return (
    <main className="h-full overflow-auto p-6 space-y-6">
      <AppLink href={paths.iterations()}>{t(($) => $.iterations.back)}</AppLink>
      {detail.error && <IterationRefreshError error={detail.error} onRetry={() => void detail.refetch()} />}
      <header>
        <h1 className="text-title font-semibold break-words">
          {iteration.name}
        </h1>
        <p className="text-muted-foreground">
          {iteration.start_date} – {iteration.end_date} · {iteration.timezone} ·{" "}
          {labels.status(iteration.status)}
        </p>
        <p>{t(($) => $.iterations.coordinator)}: {members.data?.find((member) => member.user_id === iteration.coordinator_user_id)?.name ?? (iteration.coordinator_user_id ? (members.isSuccess ? t(($) => $.iterations.coordinatorMissing) : iteration.coordinator_user_id) : t(($) => $.iterations.none))} · {iteration.mode === "manual" ? t(($) => $.iterations.manualMode) : iteration.mode}</p>
        <Button variant="outline" onClick={() => { void copyText(navigation.getShareableUrl(paths.iterationDetail(iteration.id))).then((ok) => ok ? toast.success(t(($) => $.iterations.copiedLink)) : toast.error(t(($) => $.iterations.copyFailed))); }}>{t(($) => $.iterations.copyLink)}</Button>
        {iteration.started_at && <p>{t(($) => $.iterations.actualStartedAt)}: <time dateTime={iteration.started_at}>{formatInTimeZone(iteration.started_at, iteration.timezone, locale, { year: "numeric" })}</time> · {iteration.timezone}</p>}
        <p className="whitespace-pre-wrap break-words">
          {iteration.description}
        </p>
        {iterationIsOverdue(
          iteration.status,
          iteration.end_date,
          iteration.timezone,
        ) && (
          <p role="status" className="font-medium">
            {t(($) => $.iterations.overdue)}
          </p>
        )}
      </header>
      {!known && <p>{t(($) => iteration.mode !== "manual" ? $.iterations.unknownMode : $.iterations.unknown)}</p>}
      {known && enabled && (
        <IterationForm
          wsId={wsId}
          timezone={iteration.timezone}
          iteration={iteration}
        />
      )}
      <div className="flex flex-wrap gap-3">
        {(["start", "cancel", "delete", "end", "handoff"] as const).map(
          (operation) => (
            <IterationOperation
              key={operation}
              wsId={wsId}
              iteration={iteration}
              settingsRevision={settingsRevision}
              operation={operation}
              available={
                enabled && known &&
                (operation === "start" || operation === "delete"
                  ? iteration.status === "planned"
                  : operation === "cancel"
                    ? ["planned", "active"].includes(iteration.status)
                    : operation === "handoff"
                      ? iteration.status === "active" && atomicHandoff
                      : iteration.status === "active")
              }
            />
          ),
        )}
      </div>
      {enabled && known && ["planned", "active"].includes(iteration.status) && (
        <IterationAssignment wsId={wsId} targetId={iteration.id} />
      )}
      <IterationHistory
        wsId={wsId}
        statistics={statistics}
        snapshot={snapshot}
        timezone={iteration.timezone}
      />
      {!snapshot && <IterationEvents wsId={wsId} id={id} timezone={iteration.timezone} />}
      <IterationIssueList key={`${wsId}:${id}`} wsId={wsId} id={id} historical={snapshot !== null} />
    </main>
  );
}

function isDefinitiveReadError(error: unknown): boolean {
  // Timeout and rate-limit responses retain cached input just like network/5xx
  // failures. Authentication, authorization and deletion still hide it.
  return error instanceof ApiError && error.status < 500 && ![408, 429].includes(error.status);
}

function IterationRefreshError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useT("projects");
  return <div className="space-y-2">
    <IterationError error={error} />
    <Button onClick={onRetry}>{t(($) => $.iterations.retryRefresh)}</Button>
  </div>;
}

function IterationEvents({ wsId, id, timezone }: { wsId: string; id: string; timezone: string }) {
  const { t } = useT("projects");
  const [open, setOpen] = useState(false);
  const [cursor, setCursor] = useState("");
  const events = useQuery({
    ...iterationEventsOptions(wsId, id, cursor ? { cursor } : {}),
    enabled: open,
  });
  return (
    <details onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary>{t(($) => $.iterations.events)}</summary>
      {events.error ? (
        <p role="alert">{t(($) => $.iterations.error)}</p>
      ) : (
        <IterationEventsView wsId={wsId} events={events.data?.items ?? []} timezone={timezone} />
      )}
      {events.data?.next_cursor && (
        <Button onClick={() => setCursor(events.data!.next_cursor!)}>
          {t(($) => $.iterations.next)}
        </Button>
      )}
    </details>
  );
}
