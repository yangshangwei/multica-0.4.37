"use client";

import { useId, useState } from "react";
import { useIsMutating, useQuery } from "@tanstack/react-query";
import { isIterationAccessDenied } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useCurrentMember } from "@multica/core/permissions";
import {
  iterationCapabilitiesOptions,
  iterationChoicesOptions,
  iterationSettingsOptions,
  useIterationCommand,
  usePendingIterationCommands,
} from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { AppLink } from "../navigation";
import { useT } from "../i18n";
import { SettingsCard, SettingsRow, SettingsSection, SettingsTab } from "../settings/components/settings-layout";
import { IterationError } from "./iteration-error";
import { IterationOperation } from "./iteration-operation";
import { IterationRecovery } from "./iteration-recovery";

export function IterationSettingsTab() {
  const wsId = useWorkspaceId();
  return <WorkspaceIterationSettings key={wsId} wsId={wsId} />;
}

function WorkspaceIterationSettings({ wsId }: { wsId: string }) {
  const { t } = useT("settings");
  const paths = useWorkspacePaths();
  const statusId = useId();
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  const supported = capability.data?.supported === true && capability.data.manual === true;
  const settings = useQuery({ ...iterationSettingsOptions(wsId), enabled: supported });
  const choices = useQuery({ ...iterationChoicesOptions(wsId), enabled: supported && settings.data?.enabled === true });
  const { role, userId } = useCurrentMember(wsId);
  const canManage = !!userId && (role === "owner" || role === "admin");
  const enable = useIterationCommand(wsId, "enable");
  const pending = usePendingIterationCommands(wsId);
  const processing = useIsMutating({ mutationKey: ["iterations", wsId, "command"] }) > 0;
  const [disableOpen, setDisableOpen] = useState(false);
  const timezone = settings.data?.effective_timezone;
  const error = capability.error ?? settings.error;
  const denied = isIterationAccessDenied(error) || isIterationAccessDenied(enable.error);
  const readable = supported && !!settings.data && !denied;
  const enabled = settings.data?.enabled === true;
  const operationBlocked = pending.length > 0 || processing;
  const refreshBlocked = !!error || settings.isFetching || capability.isFetching || !timezone;
  const editable = canManage && readable && !operationBlocked && !refreshBlocked;
  const retry = async () => {
    const current = await capability.refetch();
    if (current.data?.supported === true && current.data.manual === true && !current.error) {
      await settings.refetch();
    }
  };
  async function toggle(next: boolean) {
    if (!editable || !settings.data || !timezone) return;
    if (!next) { setDisableOpen(true); return; }
    try {
      await enable.mutateAsync({ command: {
        kind: "enable",
        body: {
          request_id: crypto.randomUUID(),
          expected_revision: settings.data.revision,
          confirmed_timezone: timezone,
        },
      } });
    } catch { /* The error and durable recovery controls remain on the page. */ }
  }

  return <SettingsTab title={t(($) => $.iterations.title)} description={t(($) => $.iterations.description)}>
    <IterationRecovery wsId={wsId} />
    {error && <div className="space-y-2">
      <IterationError error={error} />
      <Button variant="outline" className="pointer-coarse:min-h-11" disabled={capability.isFetching || settings.isFetching} onClick={() => void retry()}>{t(($) => $.iterations.retry)}</Button>
    </div>}
    {capability.isPending || (supported && settings.isPending) ? <div role="status" className="space-y-5"><span className="sr-only">{t(($) => $.iterations.loading)}</span><Skeleton className="h-16 w-full" /><Skeleton className="h-28 w-full" /></div> :
      !supported && !error ? <div className="space-y-3">
        <p role="status" className="text-body text-muted-foreground">{t(($) => $.iterations.unsupported)}</p>
        <Button variant="outline" className="pointer-coarse:min-h-11" disabled={capability.isFetching} onClick={() => void retry()}>{t(($) => $.iterations.retry)}</Button>
      </div> : null}
    {enable.error && !denied && <IterationError error={enable.error} />}
    {readable && <>
      {!canManage && <p className="text-body text-muted-foreground">{t(($) => $.iterations.readonly)}</p>}
      <SettingsSection>
        <SettingsCard>
          <SettingsRow label={t(($) => $.iterations.enable)} description={t(($) => $.iterations.enable_description)}>
            <Switch aria-label={t(($) => $.iterations.enable)} aria-describedby={statusId} className="after:-inset-y-3.5" checked={enabled} disabled={!editable || disableOpen} onCheckedChange={(next) => void toggle(next)} />
          </SettingsRow>
          <SettingsRow label={<span id={statusId} role="status">{t(($) => enabled ? $.iterations.enabled : $.iterations.disabled)}</span>} description={enabled ? <>
            {choices.error ? <span role="alert">{t(($) => $.iterations.counts_error)} <Button size="sm" variant="ghost" className="pointer-coarse:min-h-11" onClick={() => void choices.refetch()}>{t(($) => $.iterations.retry)}</Button></span> :
              choices.data ? t(($) => $.iterations.counts, { active: choices.data.filter((item) => item.status === "active").length, planned: choices.data.filter((item) => item.status === "planned").length }) : <span role="status">{t(($) => $.iterations.loading)}</span>}
          </> : undefined}>
            <AppLink className="inline-flex min-h-11 items-center text-body font-medium underline underline-offset-4" href={paths.iterations()}>{t(($) => enabled ? $.iterations.manage : $.iterations.history)}</AppLink>
          </SettingsRow>
          <SettingsRow label={t(($) => $.iterations.mode)} description={t(($) => $.iterations.manual_description)}>
            <span className="text-body text-muted-foreground">{t(($) => $.iterations.manual)}</span>
          </SettingsRow>
        </SettingsCard>
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 px-0.5 text-caption text-muted-foreground">
          <p className="min-w-0 break-words leading-5">{t(($) => $.iterations.timezone_description, { timezone })}</p>
          <AppLink className="inline-flex min-h-11 items-center font-medium text-foreground underline underline-offset-4" href={`${paths.settings()}?tab=workspace`}>{t(($) => $.iterations.timezone_settings)}</AppLink>
        </div>
        {canManage && (operationBlocked || refreshBlocked) && <p role="status" className="px-0.5 text-caption text-muted-foreground">{t(($) => operationBlocked ? $.iterations.operation_block : $.iterations.refresh_block)}</p>}
        {refreshBlocked && !error && !settings.isFetching && !capability.isFetching && <Button size="sm" variant="outline" className="pointer-coarse:min-h-11" onClick={() => void retry()}>{t(($) => $.iterations.retry)}</Button>}
      </SettingsSection>
      <p className="text-caption leading-5 text-muted-foreground">{t(($) => $.iterations.footer)}</p>
      <IterationOperation wsId={wsId} iteration={null} settingsRevision={settings.data!.revision} operation="disable" available={canManage && enabled && !operationBlocked && !refreshBlocked} open={disableOpen} onOpenChange={setDisableOpen} hideTrigger description={t(($) => $.iterations.disable_description)} />
    </>}
  </SettingsTab>;
}
