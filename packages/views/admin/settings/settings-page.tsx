"use client";
import { useAdminSettings } from "@multica/core/admin";
import { useT } from "../../i18n";
import { ObservationHeader, ObservationState, MetricRows } from "../observability/common";
export function AdminSettingsPage() {
  const { t } = useT("admin");
  const query = useAdminSettings();
  const data = query.data;
  const flag = (value: boolean | null) => value === null ? t($ => $.observability.unknown) : value ? t($ => $.settings.enabled) : t($ => $.settings.disabled);
  const policy = data?.configuration.registrationPolicy;
  const policyLabel = policy === "self_service" ? t($ => $.users.registrationOpen) : policy === "disabled" ? t($ => $.users.registrationClosed) : t($ => $.observability.unknown);
  return <section className="max-w-4xl space-y-6"><ObservationHeader title={t($ => $.settings.title)} description={t($ => $.settings.description)} />
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} asOf={data?.asOf} retry={() => void query.refetch()}>{data && <>
      <section className="space-y-5"><h2 className="text-body-lg font-semibold">{t($ => $.settings.configuration)}</h2><MetricRows layout="settings" rows={[
        { label: t($ => $.settings.authMode), value: data.configuration.authMode }, { label: t($ => $.settings.registration), value: flag(data.configuration.registrationEnabled) }, { label: t($ => $.settings.registrationPolicy), value: policyLabel },
        { label: t($ => $.settings.managed), value: flag(data.configuration.managedInstallationsEnabled) }, { label: t($ => $.settings.workspaceCreation), value: flag(data.configuration.workspaceCreationEnabled) }, { label: t($ => $.settings.source), value: data.configuration.source === "deployment" ? t($ => $.settings.deployment) : t($ => $.observability.unknown) },
      ]} /></section>
      <section className="space-y-5 border-t border-surface-border pt-6"><div className="space-y-1.5"><h2 className="text-body-lg font-semibold">{t($ => $.settings.retention)}</h2><p className="max-w-prose text-caption text-muted-foreground">{t($ => $.settings.retentionHint)}</p></div><MetricRows layout="settings" rows={[
        { label: t($ => $.settings.operationsDays), value: data.retention.confirmedOperationsDays }, { label: t($ => $.settings.alertsDays), value: data.retention.alertsDays }, { label: t($ => $.settings.auditDays), value: data.retention.auditDays },
      ]} /><p className="text-caption text-muted-foreground">{t($ => $.settings.deletionDisabled)}</p></section>
      <section className="space-y-5 border-t border-surface-border pt-6"><h2 className="text-body-lg font-semibold">{t($ => $.settings.refresh)}</h2><MetricRows layout="settings" rows={[{ label: t($ => $.settings.listSeconds), value: data.refreshIntervalsSeconds.list }, { label: t($ => $.settings.detailSeconds), value: data.refreshIntervalsSeconds.detail }]} /></section>
    </>}</ObservationState></section>;
}
