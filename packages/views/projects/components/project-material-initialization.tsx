"use client";

import { useId, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import { canPublishInitializedMaterial, canReviewInitialization, decodeMaterialInitializationCommand, initializationReviewKey, materialDraftKey, materialInitializationOptions, supportsMaterialInitialization, useMaterialInitializationCommand, type MaterialInitialization, type MaterialInitializationCommand } from "@multica/core/materials";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";
import { getSettingsQueryState } from "../../settings/components/settings-query-feedback";
import { MaterialActionNotice, MaterialSelect, useMaterialAction, useMaterialLabels } from "./material-controls";

export function ProjectMaterialInitialization({ workspaceId, projectId, settingsVersion }: { workspaceId: string; projectId: string; settingsVersion?: number }) {
  const { t } = useT("projects");
  const query = useQuery(materialInitializationOptions(workspaceId, projectId));
  const { hasData, failed } = getSettingsQueryState(query);
  const unsupported = query.error instanceof ApiError && [404, 405, 501].includes(query.error.status);
  return <section className="space-y-4 rounded-lg border p-4" aria-label={t(($) => $.materials.initialization.title)}>
    <div className="flex flex-wrap items-center justify-between gap-3"><h3 className="text-body font-semibold">{t(($) => $.materials.initialization.title)}</h3>
      <Button size="sm" variant="ghost" disabled={query.isFetching} onClick={() => { void query.refetch({ cancelRefetch: false }); }}>{t(($) => $.materials.refresh)}</Button>
    </div>
    {failed && <p role="alert" className="text-caption text-muted-foreground">{unsupported ? t(($) => $.materials.initialization.unsupported) : t(($) => $.materials.initialization.loadError)}</p>}
    {!hasData && !failed && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.materials.loading)}</p>}
    {hasData && query.data && <InitializationContent key={`${workspaceId}:${projectId}`} value={query.data} settingsVersion={settingsVersion} readFailed={failed || query.isError} />}
  </section>;
}

function InitializationContent({ value, settingsVersion, readFailed }: { value: MaterialInitialization; settingsVersion?: number; readFailed: boolean }) {
  const { t } = useT("projects");
  const id = useId();
  const userId = useAuthStore(state => state.user?.id);
  const mutation = useMaterialInitializationCommand(value.workspace_id, value.project_id);
  const action = useMaterialAction(mutation.mutateAsync, materialDraftKey(userId, value.workspace_id, value.project_id, "initialization"), decodeMaterialInitializationCommand, undefined, true);
  const [reason, setReason] = useState("");
  const [reviewKey, setReviewKey] = useState<string | null>(() => initializationReviewKey(value));
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [bundle, setBundle] = useState("");
  const labels = useMaterialLabels();
  const states = t(($) => $.materials.initialization.states, { returnObjects: true });
  const fileActions = t(($) => $.materials.initialization.fileActions, { returnObjects: true });
  const label = (map: Record<string, string>, key: string) => Object.hasOwn(map, key) ? map[key] : t(($) => $.materials.unknownState);
  const supported = supportsMaterialInitialization(value);
  const current = value.settings_version === (settingsVersion ?? 0);
  const disabled = action.locked || readFailed || !supported || !current;
  const candidate = value.candidate;
  const sameReview = reviewKey !== null && reviewKey === initializationReviewKey(value);
  const canReview = sameReview && canReviewInitialization(value) && !disabled;
  const file = candidate?.files.find(item => item.path === selectedFile) ?? candidate?.files[0];
  const submit = (command: MaterialInitializationCommand) => action.submit(command, () => setReason(""));
  const identity = () => ({ request_id: crypto.randomUUID(), expected_version: value.version, expected_settings_version: value.settings_version });
  const decide = (decision: "approve" | "request_changes" | "withdraw") => {
    if (!canReview || !candidate?.head_sha) return;
    submit({ action: "decisions", data: { ...identity(), candidate_id: candidate.id, head_sha: candidate.head_sha, manifest_sha256: candidate.manifest_sha256, decision, reason } });
  };
  const publishAllowed = canPublishInitializedMaterial(value, settingsVersion);
  return <div className="space-y-4">
    <p role="status" className="text-body font-medium">{label(states, value.status)}</p>
    <p className="text-caption text-muted-foreground">{publishAllowed ? t(($) => $.materials.initialization.publishAllowed) : t(($) => $.materials.initialization.draftAllowed)}</p>
    {!supported && <p role="alert" className="text-caption text-muted-foreground">{t(($) => $.materials.initialization.unsupported)}</p>}
    {!current && <p role="alert" className="text-caption text-muted-foreground">{t(($) => $.materials.configChanged)}</p>}
    {value.blocker && <p role="status" className="text-caption text-muted-foreground">{labels.blocker(value.blocker)}</p>}
    {value.operation && <p className="text-caption text-muted-foreground">{t(($) => $.materials.operation)}: {labels.operation(value.operation.status)}{value.operation.blocker && ` · ${labels.blocker(value.operation.blocker)}`}</p>}
    {value.readiness.verified_commit && <div className="space-y-1 text-caption text-muted-foreground"><p>{t(($) => $.materials.initialization.acceptedBundle)}: {value.readiness.bundle_version}</p><p className="break-all">{t(($) => $.materials.baselineCommit)}: <code>{value.readiness.verified_commit}</code></p></div>}
    <MaterialActionNotice error={action.error} uncertain={action.pending !== null} busy={action.busy || readFailed || !supported} onRetry={action.retry} />
    {action.incompatible && <p role="alert" className="text-caption text-destructive">{t(($) => $.materials.initialization.incompatibleRequest)}</p>}
    <div className="flex flex-wrap gap-2">
      {value.capabilities.can_check && <Button size="sm" variant="outline" disabled={disabled} onClick={() => submit({ action: "check", data: identity() })}>{t(($) => $.materials.initialization.check)}</Button>}
      {value.capabilities.can_retry && <Button size="sm" variant="outline" disabled={disabled} onClick={() => submit({ action: "retry", data: identity() })}>{t(($) => $.materials.resume)}</Button>}
    </div>
    {candidate && <details className="space-y-4" open={value.status === "in_review" || undefined}>
      <summary className="cursor-pointer text-body font-medium">{candidate.kind === "maintenance" ? t(($) => $.materials.initialization.maintenance) : t(($) => $.materials.initialization.candidate)}</summary>
      <div className="space-y-2 break-all text-caption text-muted-foreground"><p>{t(($) => $.materials.initialization.bundle)}: {candidate.bundle_version}</p><p>{t(($) => $.materials.initialization.baseCommit)}: <code>{candidate.base_commit}</code></p><p>{t(($) => $.materials.initialization.candidateHead)}: <code>{candidate.head_sha ?? t(($) => $.materials.initialization.unpublished)}</code></p><p>{t(($) => $.materials.initialization.manifest)}: <code>{candidate.manifest_sha256}</code></p>{candidate.review_url && <a className="underline underline-offset-4" href={candidate.review_url} target="_blank" rel="noopener noreferrer">{t(($) => $.materials.openReview)}</a>}</div>
      <p className="text-caption text-muted-foreground">{t(($) => $.materials.initialization.diffHint)}</p>
      <MaterialSelect id={`${id}-file`} label={t(($) => $.materials.initialization.file)} value={file?.path ?? ""} options={candidate.files.map(item => ({ value: item.path, label: `${item.path} · ${label(fileActions, item.action)}` }))} onChange={setSelectedFile} />
      <ul className="flex flex-wrap gap-x-4 gap-y-1 text-caption text-muted-foreground">{candidate.files.map(item => <li key={item.path} className="break-all">{item.path} · {label(fileActions, item.action)}</li>)}</ul>
      {file && <div className="space-y-3">
        {file.explanation && <p className="text-caption text-muted-foreground">{file.explanation}</p>}
        <div className="grid min-w-0 gap-4 xl:grid-cols-2">{[{ label: t(($) => $.materials.initialization.before), content: file.before_content, digest: file.before_sha256 }, { label: t(($) => $.materials.initialization.after), content: file.after_content, digest: file.after_sha256 }].map(side => <section key={side.label} className="min-w-0 space-y-2"><h4 className="text-caption font-medium">{side.label}</h4>{side.content === null ? <p className="text-caption text-muted-foreground">{side.digest ? t(($) => $.materials.initialization.unreadBody) : t(($) => $.materials.initialization.noFile)}</p> : <pre className="max-h-[32rem] overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-3 text-caption"><code>{side.content}</code></pre>}</section>)}</div>
      </div>}
      {value.capabilities.can_review && <div className="space-y-3">
        <p className="text-caption text-muted-foreground">{t(($) => $.materials.initialization.approvalHint)}</p>
        {!sameReview && <Button variant="outline" size="sm" disabled={disabled || !canReviewInitialization(value)} onClick={() => { setReviewKey(initializationReviewKey(value)); setReason(""); }}>{t(($) => $.materials.initialization.reviewCurrent)}</Button>}
        <label htmlFor={`${id}-reason`} className="text-caption font-medium">{t(($) => $.materials.initialization.reason)}</label><Textarea id={`${id}-reason`} value={reason} disabled={action.locked} onChange={event => setReason(event.target.value)} />
        <div className="flex flex-wrap gap-2"><Button size="sm" disabled={!canReview} onClick={() => decide("approve")}>{t(($) => $.materials.initialization.approve)}</Button><Button size="sm" variant="outline" disabled={!canReview || !reason.trim()} onClick={() => decide("request_changes")}>{t(($) => $.materials.requestChanges)}</Button><Button size="sm" variant="ghost" disabled={!canReview} onClick={() => decide("withdraw")}>{t(($) => $.materials.withdraw)}</Button></div>
      </div>}
    </details>}
    {value.capabilities.can_maintain && <div className="space-y-3"><MaterialSelect id={`${id}-bundle`} label={t(($) => $.materials.initialization.targetBundle)} value={bundle} options={[{ value: "", label: t(($) => $.materials.choose) }, ...value.available_bundle_versions.map(version => ({ value: version, label: version }))]} disabled={disabled} onChange={setBundle} /><p className="text-caption text-muted-foreground">{t(($) => $.materials.initialization.maintenanceHint)}</p><Button size="sm" variant="outline" disabled={disabled || !value.available_bundle_versions.includes(bundle)} onClick={() => submit({ action: "maintenance", data: { ...identity(), target_bundle_version: bundle } })}>{t(($) => $.materials.initialization.maintain)}</Button></div>}
    {(value.history?.length ?? 0) > 0 && <details className="space-y-3 text-caption"><summary className="cursor-pointer font-medium">{t(($) => $.materials.initialization.proofHistory)}</summary><p className="text-muted-foreground">{t(($) => $.materials.initialization.proofHistoryHint)}</p><ul className="space-y-2">{value.history?.map((proof, index) => <li key={index} className="break-all">{proof.bundle_version} · {proof.verified_at}<br /><code>{proof.verified_commit}</code></li>)}</ul></details>}
    {value.decisions.length > 0 && <details className="space-y-3 text-caption"><summary className="cursor-pointer font-medium">{t(($) => $.materials.initialization.history)}</summary><ul className="space-y-3">{value.decisions.map(decision => <li key={decision.id} className="space-y-1 break-all"><p>{decision.actor_id} · {decision.created_at} · {decision.decision === "approve" ? t(($) => $.materials.approve) : decision.decision === "request_changes" ? t(($) => $.materials.requestChanges) : decision.decision === "withdraw" ? t(($) => $.materials.withdraw) : t(($) => $.materials.unknownState)}</p><code>{decision.head_sha}</code><p className="whitespace-pre-wrap break-words">{decision.reason}</p></li>)}</ul></details>}
  </div>;
}
