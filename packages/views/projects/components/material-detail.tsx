"use client";

import { useEffect, useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, ExternalLink } from "lucide-react";
import { materialRuntimeOptions, canReviewMaterialRevision, isKnownMaterialState, materialDetailOptions, useMaterialCommand, decodeMaterialCommand, materialDraftKey, setMaterialEditorDraft, type MaterialCommand, type MaterialDetail, type MaterialKind, type MaterialRevision } from "@multica/core/materials";
import { runtimeDisplayLabel } from "@multica/core/runtimes";
import { useAuthStore } from "@multica/core/auth";
import { memberListOptions } from "@multica/core/workspace/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Markdown } from "@multica/ui/markdown";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { getSettingsQueryState } from "../../settings/components/settings-query-feedback";
import { MaterialActionNotice, MaterialLoadError, MaterialSelect, useMaterialAction, useMaterialLabels } from "./material-controls";
import { MaterialRevisionEditor } from "./material-revision-editor";

export function MaterialDetailView({ workspaceId, projectId, materialId, onBack }: { workspaceId: string; projectId: string; materialId: string; onBack: () => void }) {
  const { t } = useT("projects");
  const query = useQuery(materialDetailOptions(workspaceId, projectId, materialId));
  const { hasData, failed } = getSettingsQueryState(query);
  const refresh = () => { if (!query.isFetching) void query.refetch({ cancelRefetch: false }); };
  return <div className="space-y-5">
    <Button type="button" size="sm" variant="ghost" onClick={onBack}><ArrowLeft />{t(($) => $.materials.back)}</Button>
    {failed && <MaterialLoadError onRetry={refresh} busy={query.isFetching} />}
    {!hasData && !failed && <p role="status" className="text-body text-muted-foreground">{t(($) => $.materials.loading)}</p>}
    {hasData && query.data && <MaterialDetailContent key={materialId} detail={query.data} onRefresh={refresh} />}
  </div>;
}

function RevisionDocument({ revision }: { revision: MaterialRevision }) {
  const { t } = useT("projects");
  return <div className="min-w-0 space-y-5">
    <div className="min-w-0 overflow-x-auto break-words"><Markdown mode="full">{revision.content}</Markdown></div>
    {revision.plan_content && <section className="min-w-0 space-y-3"><h4 className="text-body font-semibold">{t(($) => $.materials.planContent)}</h4><div className="overflow-x-auto break-words"><Markdown mode="full">{revision.plan_content}</Markdown></div></section>}
    {revision.acceptance_criteria.length > 0 && <section className="space-y-2"><h4 className="text-body font-semibold">{t(($) => $.materials.acceptance)}</h4><ul className="list-disc space-y-2 pl-5 text-body">{revision.acceptance_criteria.map((criterion, index) => <li key={index} className="break-words">{criterion}</li>)}</ul></section>}
    {revision.blocking_questions.length > 0 && <section className="space-y-2"><h4 className="text-body font-semibold">{t(($) => $.materials.blockers)}</h4><ul className="list-disc space-y-2 pl-5 text-body">{revision.blocking_questions.map((question, index) => <li key={index} className="break-words">{question}</li>)}</ul></section>}
    {revision.sources.length > 0 && <details className="text-caption"><summary className="cursor-pointer font-medium">{t(($) => $.materials.sources)}</summary><ul className="mt-3 space-y-3">{revision.sources.map((source, index) => <li key={index} className="space-y-1 break-all text-muted-foreground"><p>{source.path || source.resource_id}</p><code>{source.commit}</code></li>)}</ul></details>}
  </div>;
}

function MaterialDetailContent({ detail, onRefresh }: { detail: MaterialDetail; onRefresh: () => void }) {
  const { t } = useT("projects");
  const id = useId();
  const labels = useMaterialLabels();
  const paths = useWorkspacePaths();
  const { material, baseline, plan } = detail;
  const members = useQuery(memberListOptions(material.workspace_id));
  const runtimes = useQuery({ ...materialRuntimeOptions(material.workspace_id), enabled: !!detail.operation });
  const operationRuntime = runtimes.data?.find((runtime) => runtime.id === detail.operation?.execution_runtime_id);
  const names = new Map(members.data?.map((member) => [member.user_id, member.name]));
  const mutation = useMaterialCommand(material.workspace_id, material.project_id, material.id);
  const userId = useAuthStore((state) => state.user?.id);
  const action = useMaterialAction(mutation.mutateAsync, materialDraftKey(userId, material.workspace_id, material.project_id, material.id), decodeMaterialCommand, onRefresh);
  const [editor, setEditor] = useState<MaterialKind | null>(null);
  const [selected, setSelected] = useState<string | null>(() => detail.revisions.at(-1)?.id ?? null);
  const [reviewSnapshot, setReviewSnapshot] = useState<{ id: string; head: string; digest: string } | null>(null);
  const [compare, setCompare] = useState(false);
  const [reason, setReason] = useState("");
  const [comment, setComment] = useState("");
  const ordered = [...detail.revisions].reverse();
  const revision = ordered.find((item) => item.id === selected) ?? ordered[0];
  useEffect(() => {
    // Freeze the first published version shown. A later remote-head change
    // needs an explicit new review, even when the revision ID stays the same.
    if (!reviewSnapshot && revision?.head_sha) setReviewSnapshot({ id: revision.id, head: revision.head_sha, digest: revision.content_sha256 });
  }, [reviewSnapshot, revision?.id, revision?.head_sha, revision?.content_sha256]);
  const approved = detail.revisions.find((item) => item.id === baseline?.revision_id);
  const previous = revision && ordered.slice(ordered.indexOf(revision) + 1).find((item) => item.kind === revision.kind);
  const comparison = revision?.kind === "requirement" && approved && approved.id !== revision.id ? approved : previous;
  const known = isKnownMaterialState(material.status);
  const canEdit = detail.capabilities.can_edit === true && known && !action.locked;
  const sameReview = revision && reviewSnapshot?.id === revision.id && reviewSnapshot.head === revision.head_sha && reviewSnapshot.digest === revision.content_sha256;
  const canReview = revision && sameReview && canReviewMaterialRevision(detail, revision) && !action.locked;
  const submit = (command: MaterialCommand, success?: (result: MaterialDetail) => void) => action.submit(command, (result) => { success?.(result); onRefresh(); });
  const decide = (decision: "approve" | "request_changes" | "withdraw") => {
    if (!revision?.head_sha || !canReview) return;
    submit({ action: "decisions", data: { request_id: crypto.randomUUID(), expected_version: material.version,
      revision_id: revision.id, head_sha: revision.head_sha, decision, reason } }, () => setReason(""));
  };
  return <div className="space-y-6">
    <header className="space-y-3"><div className="flex flex-wrap items-start justify-between gap-3"><h2 className="min-w-0 break-words text-title font-semibold">{material.title}</h2><Button type="button" size="sm" variant="outline" onClick={onRefresh} disabled={action.busy}>{t(($) => $.materials.refresh)}</Button></div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-caption"><span className="font-medium">{labels.state(material.status)}</span><span className="min-w-0 break-all text-muted-foreground">{material.path}</span></div>
      {material.blocker && <p role="status" className="break-words text-body text-muted-foreground">{labels.blocker(material.blocker)}</p>}
    </header>
    {detail.operation && <section className="space-y-2 text-caption" aria-labelledby={`${id}-operation`}>
      <h3 id={`${id}-operation`} className="font-medium">{t(($) => $.materials.operation)}</h3>
      <p role="status">{labels.operation(detail.operation.status)}</p>
      {detail.operation.execution_runtime_id && <p className="break-all text-muted-foreground">{t(($) => $.materials.executionRuntime)}: {operationRuntime ? runtimeDisplayLabel(operationRuntime) : t(($) => $.materials.runtimeHistoryUnavailable)}</p>}
      {detail.operation.blocker && <p className="text-muted-foreground">{labels.blocker(detail.operation.blocker)}</p>}
    </section>}
    <MaterialActionNotice error={action.error} uncertain={action.pending !== null} busy={action.busy} onRetry={action.retry} />
    <section className="space-y-3 rounded-lg bg-muted/40 p-4" aria-labelledby={`${id}-baseline`}>
      <h3 id={`${id}-baseline`} className="text-body font-semibold">{t(($) => $.materials.baseline)}</h3>
      {baseline ? <div className="space-y-2 text-caption"><p className="break-all"><span className="text-muted-foreground">{t(($) => $.materials.baselineCommit)}</span>{" "}<code>{baseline.commit}</code></p>
        <Button type="button" size="sm" variant="outline" onClick={() => { setSelected(baseline.revision_id); setReviewSnapshot(null); setEditor(null); }}>{t(($) => $.materials.content)}</Button>
      </div> : <p className="text-body text-muted-foreground">{t(($) => $.materials.noBaseline)}</p>}
      {plan && <div className="space-y-3 pt-2"><p className="text-body"><span className="font-medium">{t(($) => $.materials.planning)}</span>{" · "}{labels.plan(plan.status)}</p>
        {plan.blocker && <p role="status" className="break-words text-caption text-muted-foreground">{labels.blocker(plan.blocker)}</p>}
        <div className="flex flex-wrap gap-2">{plan.issue_id && <AppLink href={paths.issueDetail(plan.issue_id)} className="inline-flex items-center rounded-md px-3 py-2 text-caption font-medium underline underline-offset-4">{t(($) => $.materials.openTask)}</AppLink>}
          {detail.capabilities.can_start_plan === true && <Button type="button" size="sm" disabled={action.locked || !known} onClick={() => submit({ action: "start-plan", data: { request_id: crypto.randomUUID(), expected_version: material.version } })}>{t(($) => $.materials.startPlan)}</Button>}
        </div>
      </div>}
    </section>
    <div className="flex flex-wrap gap-2">{detail.capabilities.can_edit === true && <>
      <Button type="button" variant="outline" size="sm" disabled={!canEdit} onClick={() => setEditor("requirement")}>{t(($) => $.materials.edit)}</Button>
      {baseline && <Button type="button" variant="outline" size="sm" disabled={!canEdit} onClick={() => setEditor("plan")}>{t(($) => $.materials.editPlan)}</Button>}
    </>}{detail.capabilities.can_retry === true && <Button type="button" variant="outline" size="sm" disabled={action.locked || !known} onClick={() => submit({ action: "retry", data: { request_id: crypto.randomUUID(), expected_version: material.version } })}>{t(($) => $.materials.resume)}</Button>}</div>
    {editor ? <MaterialRevisionEditor key={editor} detail={detail} kind={editor} locked={action.locked || detail.capabilities.can_edit !== true} onSave={(command) => submit(command, (result) => { setMaterialEditorDraft(materialDraftKey(userId, material.workspace_id, material.project_id, `${material.id}:${editor}`), null); setEditor(null); setReviewSnapshot(null); setSelected(result.revisions.at(-1)?.id ?? null); })} onCancel={() => setEditor(null)} />
      : revision && <>
        <div className="grid items-end gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
          <MaterialSelect id={`${id}-revision`} label={t(($) => $.materials.revision)} value={revision.id} disabled={action.locked} options={ordered.map((item) => ({ value: item.id, label: `${item.kind === "plan" ? t(($) => $.materials.planKind) : t(($) => $.materials.requirement)} · ${item.created_at} · ${item.id.slice(0, 8)}` }))} onChange={(value) => { setSelected(value); setReviewSnapshot(null); setReason(""); setComment(""); }} />
          <Button type="button" variant={compare ? "secondary" : "outline"} disabled={!comparison} onClick={() => setCompare(!compare)}>{approved && revision.kind === "requirement" && approved.id !== revision.id ? t(($) => $.materials.compareBaseline) : t(($) => $.materials.compare)}</Button>
        </div>
        <div className="flex flex-wrap items-center gap-4 text-caption text-muted-foreground"><span>{labels.publication(revision.publication_status)}</span><code className="break-all">{revision.head_sha ?? revision.content_sha256}</code>
          {revision.review_url && <a href={revision.review_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 underline underline-offset-4">{t(($) => $.materials.openReview)}<ExternalLink className="size-3" /></a>}
        </div>
        <div className={compare && comparison ? "grid min-w-0 gap-8 xl:grid-cols-2" : "min-w-0 max-w-prose"}>
          {compare && comparison && <section className="min-w-0 space-y-4"><h3 className="text-body font-semibold">{t(($) => $.materials.previous)}</h3><RevisionDocument revision={comparison} /></section>}
          <section className="min-w-0 space-y-4"><h3 className="text-body font-semibold">{baseline?.revision_id === revision.id ? t(($) => $.materials.baseline) : t(($) => $.materials.candidate)}</h3><RevisionDocument revision={revision} /></section>
        </div>
        <section className="max-w-3xl space-y-4 pt-4" aria-labelledby={`${id}-review`}><h3 id={`${id}-review`} className="text-title font-semibold">{t(($) => $.materials.review)}</h3>
          {canReview || action.locked ? <>
            <p className="max-w-prose text-body text-muted-foreground">{t(($) => $.materials.approveHint)}</p>
            <div className="space-y-2"><label htmlFor={`${id}-reason`} className="text-caption font-medium">{t(($) => $.materials.reason)}</label><Textarea id={`${id}-reason`} value={reason} onChange={(event) => setReason(event.target.value)} disabled={action.locked} /></div>
            <div className="flex flex-wrap gap-2"><Button type="button" disabled={!canReview} onClick={() => decide("approve")}>{t(($) => $.materials.approve)}</Button><Button type="button" variant="outline" disabled={!canReview || !reason.trim()} onClick={() => decide("request_changes")}>{t(($) => $.materials.requestChanges)}</Button><Button type="button" variant="ghost" disabled={!canReview} onClick={() => decide("withdraw")}>{t(($) => $.materials.withdraw)}</Button></div>
          </> : <div className="space-y-3"><p className="text-body text-muted-foreground">{t(($) => $.materials.reviewUnavailable)}</p>
            {!sameReview && canReviewMaterialRevision(detail, revision) && <Button type="button" variant="outline" disabled={action.locked} onClick={() => { setReviewSnapshot(null); setReason(""); }}>{t(($) => $.materials.reviewCurrent)}</Button>}
          </div>}
        </section>
        <section className="max-w-3xl space-y-4"><h3 className="text-title font-semibold">{t(($) => $.materials.comments)}</h3>
          <ul className="space-y-4">{detail.comments.filter((item) => item.revision_id === revision.id).map((item) => <li key={item.id} className="space-y-1"><p className="text-caption text-muted-foreground">{names.get(item.actor_id) ?? item.actor_id}{" · "}{item.created_at}</p><p className="whitespace-pre-wrap break-words text-body">{item.body}</p></li>)}</ul>
          {(detail.capabilities.can_edit === true || detail.capabilities.can_review === true) && <form className="space-y-2" onSubmit={(event) => { event.preventDefault(); if (!comment.trim() || action.locked) return; submit({ action: "comments", data: { request_id: crypto.randomUUID(), revision_id: revision.id, body: comment } }, () => setComment("")); }}>
            <label htmlFor={`${id}-comment`} className="text-caption font-medium">{t(($) => $.materials.comment)}</label><Textarea id={`${id}-comment`} value={comment} onChange={(event) => setComment(event.target.value)} disabled={action.locked} /><Button type="submit" variant="outline" size="sm" disabled={!comment.trim() || action.locked}>{t(($) => $.materials.postComment)}</Button>
          </form>}
        </section>
      </>}
    <details className="space-y-3 text-body"><summary className="cursor-pointer font-medium">{t(($) => $.materials.history)}</summary>
      {detail.decisions.length === 0 ? <p className="pt-3 text-muted-foreground">{t(($) => $.materials.noHistory)}</p> : <ul className="space-y-4 pt-3">{detail.decisions.map((decision) => <li key={decision.id} className="space-y-1"><p className="font-medium">{decision.decision === "approve" ? t(($) => $.materials.approve) : decision.decision === "request_changes" ? t(($) => $.materials.requestChanges) : decision.decision === "withdraw" ? t(($) => $.materials.withdraw) : t(($) => $.materials.unknownState)}</p><p className="break-all text-caption text-muted-foreground">{names.get(decision.actor_id) ?? decision.actor_id}{" · "}{decision.created_at}{" · "}{decision.head_sha}</p><p className="whitespace-pre-wrap break-words">{decision.reason}</p></li>)}</ul>}
    </details>
  </div>;
}
