"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { isProjectAccessLost, projectPlanningTimezoneOptions, useProjectPlanningTimezone } from "@multica/core/projects";
import { Button } from "@multica/ui/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { timezoneOptions } from "../../common/timezone-select";
import { useT } from "../../i18n";
import { SettingsCard, SettingsRow, SettingsSection } from "./settings-layout";

export function WorkspacePlanningTimezone({ wsId, canManage }: { wsId: string; canManage: boolean }) {
  const { t } = useT("settings");
  const query = useQuery(projectPlanningTimezoneOptions(wsId));
  const save = useProjectPlanningTimezone(wsId);
  const [draft, setDraft] = useState<string>();
  const saved = query.data;
  const canRead = saved && !isProjectAccessLost(query.error);
  const value = draft ?? saved?.effective_timezone ?? "";
  const changed = draft !== undefined && draft !== saved?.effective_timezone;
  const options = timezoneOptions(value).map((zone) => ({ value: zone, label: zone }));
  const cancel = () => { setDraft(undefined); save.reset(); };
  const submit = (zone: string | null) => {
    if (!canManage || save.isPending) return;
    save.mutate(zone, { onSuccess: () => setDraft(undefined) });
  };

  return <SettingsSection title={t(($) => $.workspace.planning.title)}>
    <SettingsCard>
      <SettingsRow label={t(($) => $.workspace.planning.timezone)} description={t(($) => $.workspace.planning.description)} size="select-wide" align="start">
        {query.isPending ? <p role="status" className="text-caption text-muted-foreground">{t(($) => $.workspace.planning.loading)}</p> :
          canRead ? <div className="space-y-3">
            <Select items={options} value={value} disabled={!canManage || save.isPending} onValueChange={(zone) => { if (zone) { setDraft(zone); save.reset(); } }}>
              <SelectTrigger aria-label={t(($) => $.workspace.planning.timezone)} size="sm" className="w-full max-md:min-h-11 pointer-coarse:min-h-11"><SelectValue>{value}</SelectValue></SelectTrigger>
              <SelectContent align="end" className="max-h-72">
                {options.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
              </SelectContent>
            </Select>
            {!canManage && <p className="text-caption text-muted-foreground">{t(($) => $.workspace.manage_hint)}</p>}
            {canManage && <div className="flex flex-wrap gap-2">
              {changed ? <>
                <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" disabled={save.isPending} onClick={() => submit(value)}>{save.isPending ? t(($) => $.workspace.saving) : t(($) => $.workspace.planning.save)}</Button>
                <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" variant="outline" disabled={save.isPending} onClick={cancel}>{t(($) => $.workspace.confirm_cancel)}</Button>
              </> : saved.configured && <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" variant="outline" disabled={save.isPending} onClick={() => submit(null)}>{t(($) => $.workspace.planning.reset)}</Button>}
            </div>}
            {save.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.workspace.saving)}</p>}
            {save.isSuccess && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.workspace.planning.saved)}</p>}
            {save.error && <p role="alert" className="text-caption text-destructive">{save.error.message || t(($) => $.workspace.toast_save_failed)}</p>}
          </div> : null}
        {query.error && <div role="alert" className="space-y-2 text-caption text-destructive">
          <p>{t(($) => $.workspace.planning.load_error)}</p>
          <Button className="max-md:min-h-11 pointer-coarse:min-h-11" size="sm" variant="outline" disabled={query.isFetching} onClick={() => void query.refetch()}>{t(($) => $.workspace.planning.retry)}</Button>
        </div>}
      </SettingsRow>
    </SettingsCard>
  </SettingsSection>;
}
