"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { isProjectAccessLost, projectKeys, projectPlanningTimezoneOptions, useProjectPlanningTimezone } from "@multica/core/projects";
import { iterationKeys } from "@multica/core/iterations";
import { Button } from "@multica/ui/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { timezoneOptions } from "../../common/timezone-select";
import { useT } from "../../i18n";
import { SettingsCard, SettingsRow, SettingsSection } from "./settings-layout";

export function WorkspacePlanningTimezone({ wsId, canManage }: {
  wsId: string;
  canManage: boolean;
}) {
  const { t } = useT("settings");
  const client = useQueryClient();
  const query = useQuery(projectPlanningTimezoneOptions(wsId));
  const save = useProjectPlanningTimezone(wsId);
  const [draft, setDraft] = useState<string>();
  const [confirmation, setConfirmation] = useState<{ zone: string | null }>();
  const [confirmed, setConfirmed] = useState(false);
  const [conflict, setConflict] = useState(false);
  const saved = query.data;
  const canRead = saved && !isProjectAccessLost(query.error) && !isProjectAccessLost(save.error);
  const value = draft ?? saved?.effective_timezone ?? "";
  const changed = draft !== undefined && draft !== saved?.effective_timezone;
  const options = timezoneOptions(value).map((zone) => ({ value: zone, label: zone }));
  const locked = save.isPending || confirmation !== undefined;
  const cancel = () => { setDraft(undefined); save.reset(); setConfirmed(false); setConflict(false); };
  async function confirm(zone: string | null, committed: boolean) {
    const result = await query.refetch();
    if (result.error || !result.data) return;
    const matches = zone === null ? result.data.configured === false : result.data.effective_timezone === zone;
    if (matches) {
      if (!committed) {
        // A recovered write did not run the mutation's success invalidations.
        await Promise.all([
          client.invalidateQueries({ queryKey: projectKeys.all(wsId) }),
          client.invalidateQueries({ queryKey: iterationKeys.all(wsId) }),
        ]);
      }
      setDraft(undefined);
      save.reset();
      setConfirmed(true);
    } else if (committed) {
      setConflict(true);
    }
    setConfirmation(undefined);
  }
  async function submit(zone: string | null) {
    if (!canManage || locked || query.error) return;
    setConfirmed(false);
    setConflict(false);
    setConfirmation({ zone });
    let committed = false;
    try {
      await save.mutateAsync(zone);
      committed = true;
    } catch (error) {
      if (isProjectAccessLost(error)) { setConfirmation(undefined); return; }
    }
    await confirm(zone, committed);
  }

  return <SettingsSection title={t(($) => $.workspace.planning.title)}>
    <SettingsCard>
      <SettingsRow label={t(($) => $.workspace.planning.timezone)} description={t(($) => $.workspace.planning.description)} size="select-wide" align="start">
        {query.isPending ? <p role="status" className="text-caption text-muted-foreground">{t(($) => $.workspace.planning.loading)}</p> :
          canRead ? <div className="space-y-3">
            <Select items={options} value={value} disabled={!canManage || locked} onValueChange={(zone) => { if (zone) { setDraft(zone); save.reset(); setConfirmed(false); setConflict(false); } }}>
              <SelectTrigger aria-label={t(($) => $.workspace.planning.timezone)} size="sm" className="w-full max-md:min-h-11 pointer-coarse:min-h-11"><SelectValue>{value}</SelectValue></SelectTrigger>
              <SelectContent align="end" className="max-h-72">
                {options.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
              </SelectContent>
            </Select>
            {!canManage && <p className="text-caption text-muted-foreground">{t(($) => $.workspace.manage_hint)}</p>}
            {canManage && <div className="flex flex-wrap gap-2">
              {changed ? <>
                <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" disabled={locked} onClick={() => void submit(value)}>{save.isPending ? t(($) => $.workspace.saving) : t(($) => $.workspace.planning.save)}</Button>
                <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" variant="outline" disabled={locked} onClick={cancel}>{t(($) => $.workspace.confirm_cancel)}</Button>
              </> : saved.configured && <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" variant="outline" disabled={locked} onClick={() => void submit(null)}>{t(($) => $.workspace.planning.reset)}</Button>}
            </div>}
            {save.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.workspace.saving)}</p>}
            {confirmed && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.workspace.planning.saved)}</p>}
            {confirmation && !save.isPending && <div className="space-y-2">
              <p role="status" className="text-caption text-muted-foreground">{t(($) => $.workspace.planning.confirming)}</p>
              {!query.error && <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" variant="outline" disabled={query.isFetching} onClick={() => void confirm(confirmation.zone, save.isSuccess)}>{t(($) => $.workspace.planning.retry)}</Button>}
            </div>}
            {conflict && <p role="alert" className="text-caption text-destructive">{t(($) => $.workspace.planning.changed)}</p>}
            {save.error && <p role="alert" className="text-caption text-destructive">{save.error.message || t(($) => $.workspace.toast_save_failed)}</p>}
          </div> : null}
        {query.error && <div role="alert" className="space-y-2 text-caption text-destructive">
          <p>{t(($) => $.workspace.planning.load_error)}</p>
          <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" variant="outline" disabled={query.isFetching} onClick={() => confirmation ? void confirm(confirmation.zone, save.isSuccess) : void query.refetch()}>{t(($) => $.workspace.planning.retry)}</Button>
        </div>}
      </SettingsRow>
    </SettingsCard>
  </SettingsSection>;
}
