import { useState } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationCommentsOptions, collaborationMemoryOptions, collaborationMemoryVersionsOptions, collaborationRunRecordsOptions,
  knowledgeSourceSelectionKey, type CollaborationIssue, type CollaborationScope, type KnowledgeSourceSelection } from "@multica/core/collaboration";
import { Button } from "@multica/ui/components/ui/button";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { useT } from "../i18n";
import { KnowledgeSelect, KnowledgeSourceLabel } from "./knowledge-controls";

export function KnowledgeSourcePicker({ scope, issue, selected, onChange, disabled }: {
  scope: CollaborationScope; issue: CollaborationIssue; selected: KnowledgeSourceSelection[];
  onChange: (next: KnowledgeSourceSelection[]) => void; disabled: boolean;
}) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const [kind, setKind] = useState("memory"), [runId, setRunId] = useState("");
  const memory = useQuery({ ...collaborationMemoryOptions(client, scope, issue.id), enabled: kind === "memory" });
  const versions = useInfiniteQuery({ ...collaborationMemoryVersionsOptions(client, scope, issue.id), enabled: kind === "memory" });
  const comments = useInfiniteQuery({ ...collaborationCommentsOptions(client, scope, issue.id), enabled: kind === "comment" });
  const records = useInfiniteQuery({ ...collaborationRunRecordsOptions(client, scope, issue.id, runId), enabled: kind === "run_record" && !!runId });
  const rows: { source: KnowledgeSourceSelection; preview?: string; name?: string }[] = kind === "memory" ? [...new Set([
    ...(!memory.error && memory.data?.version !== "0" && memory.data ? [memory.data.version] : []),
    ...(!versions.error ? versions.data?.pages.flatMap((page) => page.items.filter((item) => !item.purged).map((item) => item.version)) ?? [] : []),
  ])].map((version) => ({ source: { kind: "memory", issue_id: issue.id, version } }))
    : kind === "comment" ? (!comments.error ? comments.data?.pages.flatMap((page) => page.items.filter((item) => item.deleted_at === null).map((item) => ({
      source: { kind: "comment" as const, issue_id: issue.id, comment_id: item.id, version: item.comment_revision }, preview: item.content.slice(0, 160), name: item.author?.name,
    }))) ?? [] : [])
      : (!records.error ? records.data?.pages.flatMap((page) => page.items.map((item) => ({ source: { kind: "run_record" as const, issue_id: issue.id, run_id: runId, record_id: item.id }, preview: item.body.slice(0, 160) }))) ?? [] : []);
  const query = kind === "memory" ? versions : kind === "comment" ? comments : records;
  const selectedKeys = new Set(selected.map(knowledgeSourceSelectionKey));
  function toggle(source: KnowledgeSourceSelection) {
    const key = knowledgeSourceSelectionKey(source);
    onChange(selectedKeys.has(key) ? selected.filter((item) => knowledgeSourceSelectionKey(item) !== key) : [...selected, source]);
  }
  return <fieldset disabled={disabled} className="min-w-0 space-y-3">
    <legend className="mb-2 text-body font-medium">{t(($) => $.knowledge.sources)}</legend>
    <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.sourcesHint)}</p>
    <KnowledgeSelect label={t(($) => $.knowledge.sources)} value={kind} onChange={setKind} disabled={disabled}
      items={(["memory", "comment", "run_record"] as const).map((value) => ({ value, label: t(($) => $.knowledge[value]) }))} />
    {kind === "run_record" && <KnowledgeSelect label={t(($) => $.knowledge.chooseRun)} value={runId} onChange={setRunId} disabled={disabled}
      items={[{ value: "", label: t(($) => $.knowledge.chooseRun) }, ...(issue.execution?.runs ?? []).map((run) => ({ value: run.id, label: `${t(($) => $.execution[run.step_kind])} · ${run.id.slice(-8)}` }))]} />}
    {query.isFetching && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.knowledge.loading)}</p>}
    {query.isError && <div role="alert" className="space-y-2 text-caption"><p>{t(($) => $.knowledge.sourceChanged)}</p><Button type="button" variant="outline" size="sm" onClick={() => void query.refetch()}>{t(($) => $.knowledge.refresh)}</Button></div>}
    {!query.isFetching && !query.error && rows.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.knowledge.sourceEmpty)}</p>}
    <div className="max-h-64 space-y-3 overflow-y-auto p-1">
      {rows.map(({ source, preview, name }) => <label key={knowledgeSourceSelectionKey(source)} className="flex min-w-0 cursor-pointer items-start gap-3 py-1 text-caption pointer-coarse:min-h-11">
        <Checkbox checked={selectedKeys.has(knowledgeSourceSelectionKey(source))} disabled={disabled || selected.length >= 32 && !selectedKeys.has(knowledgeSourceSelectionKey(source))} onCheckedChange={() => toggle(source)} />
        <span className="min-w-0 flex-1"><KnowledgeSourceLabel source={source} />{name && <span> · {name}</span>}{preview && <span className="mt-1 block text-muted-foreground [overflow-wrap:anywhere]">{preview}</span>}</span>
      </label>)}
    </div>
    {query.hasNextPage && <Button type="button" variant="outline" size="sm" disabled={query.isFetchingNextPage || disabled} onClick={() => void query.fetchNextPage()}>{t(($) => $.knowledge.more)}</Button>}
    {selected.length > 0 && <div className="space-y-2"><p className="text-caption font-medium">{t(($) => $.knowledge.selected)}</p><ul className="space-y-2">
      {selected.map((source) => <li key={knowledgeSourceSelectionKey(source)} className="flex min-w-0 items-center justify-between gap-3 text-caption"><KnowledgeSourceLabel source={source} />
        <Button type="button" variant="ghost" size="sm" onClick={() => toggle(source)} disabled={disabled}>{t(($) => $.knowledge.removeSource)}</Button></li>)}
    </ul></div>}
  </fieldset>;
}
