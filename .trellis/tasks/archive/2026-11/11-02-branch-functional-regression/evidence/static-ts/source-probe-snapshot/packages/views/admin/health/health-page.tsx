"use client";
import { useAdminHealth } from "@multica/core/admin";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@multica/ui/components/ui/table";
import { useT } from "../../i18n";
import { AppLink, useNavigation } from "../../navigation";
import { formatAdminTime } from "../executions/list-controls";
import { ObservationHeader, ObservationState } from "../observability/common";
export function AdminHealthPage() {
  const { t } = useT("admin");
  const nav = useNavigation();
  const timezone = nav.searchParams.get("timezone") ?? "UTC";
  const query = useAdminHealth();
  const data = query.data;
  return <section className="space-y-6"><ObservationHeader title={t($ => $.health.title)} description={t($ => $.health.description)}><AppLink href={`/admin/alerts?${new URLSearchParams({ timezone })}`} className="inline-flex min-h-11 min-w-11 items-center text-body underline underline-offset-4">{t($ => $.alerts.title)}</AppLink></ObservationHeader>
    <ObservationState pending={query.isPending} error={query.isError} quality={data?.dataQuality} asOf={data?.asOf} timezone={timezone} retry={() => void query.refetch()}>{data && <>
      <dl><dt className="text-caption text-muted-foreground">{t($ => $.health.detector)}</dt><dd className="text-body font-medium">{t($ => $.health.states[data.detectorState])}</dd></dl>
      <div className="overflow-x-auto rounded-md border border-surface-border"><Table><TableHeader><TableRow><TableHead>{t($ => $.health.source)}</TableHead><TableHead>{t($ => $.health.state)}</TableHead><TableHead>{t($ => $.health.checked)}</TableHead></TableRow></TableHeader><TableBody>{data.sources.map((source, index) => <TableRow key={`${source.name}-${index}`}><TableCell className="whitespace-normal">{t($ => $.health.sources[source.name])}</TableCell><TableCell>{t($ => $.health.states[source.state])}</TableCell><TableCell className="whitespace-normal">{source.checkedAt ? <time dateTime={source.checkedAt}>{formatAdminTime(source.checkedAt, timezone)}</time> : t($ => $.observability.unknown)}</TableCell></TableRow>)}</TableBody></Table></div>
    </>}</ObservationState></section>;
}
