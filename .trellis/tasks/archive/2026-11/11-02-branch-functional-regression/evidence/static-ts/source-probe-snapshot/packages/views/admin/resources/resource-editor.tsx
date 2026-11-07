"use client";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import {
  AdminResourceUncertainError, useAdminResourcePreview, useAdminResourceMutation, useAdminResourceLookup,
  type AdminResource, type AdminResourceKind, type AdminResourceLimits, type AdminResourceMutationInput,
  type AdminResourcePreview, type AdminResourceResult, type AdminScope,
} from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import styles from "../admin-visual.module.css";
import { resourceErrorKey } from "./resource-feedback";

type Feedback = ReturnType<typeof resourceErrorKey> | "fileRequired" | "tooLarge" | "pendingReceipt";
export function ResourceEditor({ scope, kind, action, target, targetChanged = false, limits, disabled, onClose, onSuccess, onBusy }: {
  scope: AdminScope; kind: AdminResourceKind; action: "publish" | "withdraw"; target?: AdminResource; targetChanged?: boolean;
  limits: AdminResourceLimits; disabled: boolean; onClose(): void; onSuccess(result: AdminResourceResult): void; onBusy(busy: boolean): void;
}) {
  const { t } = useT("admin");
  const id = useId();
  const [key, setKey] = useState(target?.key ?? "");
  const [file, setFile] = useState<File | null>(null);
  const [reason, setReason] = useState("");
  const [preview, setPreview] = useState<AdminResourcePreview | null>(null);
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const [reasonInvalid, setReasonInvalid] = useState(false);
  const [working, setWorking] = useState<"preview" | "mutation" | "lookup" | null>(null);
  const [uncertain, setUncertain] = useState<AdminResourceMutationInput | null>(null);
  const reasonRef = useRef<HTMLTextAreaElement>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const previewRequest = useRef<AbortController | null>(null);
  const sequence = useRef(0);
  const submitting = useRef(false);
  const previewMutation = useAdminResourcePreview(scope, kind);
  const mutation = useAdminResourceMutation(scope);
  const lookup = useAdminResourceLookup(scope);
  const withdrawalChanged = action === "withdraw" && (targetChanged || feedback === "conflict");
  const frozen = disabled || working === "mutation" || working === "lookup" || !!uncertain;
  useEffect(() => { onBusy(!!working || !!uncertain); return () => onBusy(false); }, [onBusy, working, uncertain]);
  useEffect(() => { headingRef.current?.focus(); }, []);
  useEffect(() => () => { sequence.current += 1; previewRequest.current?.abort(); }, []);

  function invalidatePreview() {
    sequence.current += 1;
    previewRequest.current?.abort();
    setPreview(null); setFeedback(null);
    if (working === "preview") setWorking(null);
    previewMutation.reset();
  }
  async function validate() {
    if (frozen) return;
    if (!file || !/^[A-Za-z0-9_-]+$/.test(key)) { setFeedback("fileRequired"); return; }
    if (file.size > limits.maxUploadBytes || kind === "mcp" && file.size > limits.maxMcpBytes) { setFeedback("tooLarge"); return; }
    invalidatePreview();
    const current = sequence.current;
    const controller = new AbortController(); previewRequest.current = controller;
    setWorking("preview");
    try {
      const result = await previewMutation.mutateAsync({ key, file, filename: file.name, signal: controller.signal });
      if (current !== sequence.current) return;
      // Updating an existing key always starts with its explicit Update action.
      if (result.expectedVersion !== (target?.version ?? null)) { setFeedback("conflict"); return; }
      setPreview(result);
    } catch (error) { if (current === sequence.current) setFeedback(resourceErrorKey(error)); }
    finally { if (current === sequence.current) { setWorking(null); previewMutation.reset(); } }
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (frozen || working || submitting.current || withdrawalChanged) return;
    if (!reason.trim()) { setReasonInvalid(true); reasonRef.current?.focus(); return; }
    if (action === "publish" && (!preview || !file)) return;
    if (action === "publish" && preview?.expectedVersion !== (target?.version ?? null)) { setPreview(null); setFeedback("conflict"); return; }
    if (action === "withdraw" && (!target || target.source !== "managed" || target.state !== "published")) return;
    const operationId = crypto.randomUUID();
    const input: AdminResourceMutationInput = action === "publish" && preview && file
      ? { action, kind, key, operationId, file, filename: file.name, previewDigest: preview.previewDigest, expectedVersion: preview.expectedVersion, reason: reason.trim() }
      : { action: "withdraw", kind, key, operationId, expectedVersion: target!.version, reason: reason.trim() };
    submitting.current = true; setWorking("mutation"); setFeedback(null);
    const current = sequence.current;
    try {
      const result = await mutation.mutateAsync(input);
      if (current === sequence.current) onSuccess(result);
    } catch (error) {
      if (current !== sequence.current) return;
      if (error instanceof AdminResourceUncertainError) setUncertain(input);
      else {
        const message = resourceErrorKey(error); setFeedback(message);
        if (message === "conflict" || message === "previewChanged") setPreview(null);
      }
    } finally {
      submitting.current = false;
      if (current === sequence.current) { setWorking(null); mutation.reset(); }
    }
  }
  async function check() {
    if (!uncertain || working || submitting.current) return;
    submitting.current = true; setWorking("lookup"); setFeedback(null);
    const current = sequence.current;
    try {
      const result = await lookup.mutateAsync(uncertain);
      if (current !== sequence.current) return;
      if (result) onSuccess(result); else setFeedback("pendingReceipt");
    } catch (error) { if (current === sequence.current) setFeedback(resourceErrorKey(error)); }
    finally { submitting.current = false; if (current === sequence.current) { setWorking(null); lookup.reset(); } }
  }
  const confirmLabel = action === "withdraw" ? t($ => $.resources.confirmWithdraw) : target ? t($ => $.resources.confirmUpdate) : t($ => $.resources.confirmPublish);
  const limitLabels = [
    ["limitUpload", limits.maxUploadBytes], ["limitPrimary", limits.maxPrimaryBytes], ["limitFile", limits.maxFileBytes],
    ["limitSupporting", limits.maxSupportingBytes], ["limitTotal", limits.maxTotalBytes], ["limitFiles", limits.maxFiles],
    ["limitArchive", limits.maxArchiveEntries], ["limitMcp", limits.maxMcpBytes],
  ] as const;
  return <section className={cn(styles.panel, "space-y-5")} aria-labelledby={`${id}-heading`}>
    <h2 ref={headingRef} tabIndex={-1} id={`${id}-heading`} className="text-title font-semibold outline-none">{action === "withdraw" ? t($ => $.resources.withdraw) : target ? t($ => $.resources.update) : t($ => $.resources.new)}</h2>
    <form noValidate onSubmit={event => void submit(event)} className="space-y-5">
      {action === "publish" && <>
        <div className="space-y-1.5"><label htmlFor={`${id}-key`} className="text-label font-medium">{t($ => $.resources.key)}</label>
          <Input id={`${id}-key`} value={key} maxLength={128} disabled={frozen} readOnly={!!target} aria-describedby={`${id}-key-hint`} autoComplete="off" onChange={event => { setKey(event.target.value); invalidatePreview(); }} />
          <p id={`${id}-key-hint`} className="text-caption text-muted-foreground">{t($ => $.resources.keyHint)}</p>
        </div>
        <div className="space-y-1.5"><label htmlFor={`${id}-file`} className="text-label font-medium">{t($ => $.resources.file)}</label>
          <Input id={`${id}-file`} type="file" accept={kind === "skill" ? ".md,.zip,.skill" : ".json"} disabled={frozen} aria-describedby={`${id}-file-hint`} className="h-auto min-h-11 py-2" onChange={event => { setFile(event.target.files?.[0] ?? null); invalidatePreview(); }} />
          <p id={`${id}-file-hint`} className="text-caption text-muted-foreground">{kind === "skill" ? t($ => $.resources.skillHint) : t($ => $.resources.mcpHint)}</p>
        </div>
        <details><summary className="cursor-pointer text-caption text-muted-foreground">{t($ => $.resources.limits)}</summary><ul className="mt-2 space-y-1 text-caption text-muted-foreground">{limitLabels.map(([label, value]) => <li key={label}>{t($ => $.resources[label], { value })}</li>)}</ul></details>
        <Button type="button" variant="outline" disabled={frozen || working === "preview"} onClick={() => void validate()}>{working === "preview" ? t($ => $.resources.working) : t($ => $.resources.preview)}</Button>
      </>}
      {preview && <section className="min-w-0 space-y-3" aria-labelledby={`${id}-preview`}>
        <h3 id={`${id}-preview`} className="text-body font-semibold">{t($ => $.resources.previewTitle)}</h3>
        <p className="text-caption text-muted-foreground">{t($ => $.resources.summary, { files: preview.resource.fileCount, bytes: preview.resource.byteCount })}</p>
        <details open><summary className="cursor-pointer text-label font-medium">{t($ => $.resources.inventory)}</summary><ul className={cn(styles.resourceInventory, "mt-2 space-y-1 text-caption")}>{preview.files.map(entry => <li key={entry.path} className="flex min-w-0 justify-between gap-4"><span className="[overflow-wrap:anywhere]">{entry.path}</span><span className="shrink-0 tabular-nums">{entry.size}</span></li>)}</ul></details>
        <details open><summary className="cursor-pointer text-label font-medium">{t($ => $.resources.content)}</summary><pre className={cn(styles.resourcePreview, "mt-2 rounded-md bg-muted p-3 text-caption")}>{preview.preview}</pre></details>
      </section>}
      {(preview || action === "withdraw") && <div className="space-y-4">
        <p className="break-all text-caption">{t($ => $.resources.confirmTarget, { key, version: action === "withdraw" ? target?.version : preview?.expectedVersion ?? t($ => $.resources.createRevision) })}</p>
        <p className="text-body text-muted-foreground">{action === "withdraw" ? t($ => $.resources.withdrawImpact) : t($ => $.resources.publishImpact)}</p>
        <div className="space-y-1.5"><label htmlFor={`${id}-reason`} className="text-label font-medium">{t($ => $.resources.reason)}</label>
          <Textarea id={`${id}-reason`} ref={reasonRef} value={reason} maxLength={1000} disabled={frozen} aria-invalid={reasonInvalid || undefined} aria-describedby={`${id}-reason-hint${reasonInvalid ? ` ${id}-reason-error` : ""}`} onChange={event => { setReason(event.target.value); if (event.target.value.trim()) setReasonInvalid(false); }} />
          <p id={`${id}-reason-hint`} className="text-caption text-muted-foreground">{t($ => $.resources.reasonHint)}</p>
          {reasonInvalid && <p id={`${id}-reason-error`} role="alert" className="text-caption text-destructive">{t($ => $.resources.reasonRequired)}</p>}
        </div>
      </div>}
      {uncertain && <div role="status" className="space-y-2"><p className="text-body">{t($ => $.resources.unknownOutcome)}</p><p className="break-all text-caption">{t($ => $.resources.operation)}: {uncertain.operationId}</p></div>}
      {withdrawalChanged ? <p role="alert" className="text-body">{t($ => $.resources.withdrawChanged)}</p> : feedback && <p role="alert" className="text-body">{t($ => $.resources[feedback])}</p>}
      <div className="flex flex-wrap gap-3">
        {(preview || action === "withdraw") && <Button type="submit" variant={action === "withdraw" ? "destructive" : "default"} disabled={frozen || !!working || withdrawalChanged}>{working === "mutation" ? t($ => $.resources.working) : confirmLabel}</Button>}
        {uncertain && <Button type="button" variant="outline" disabled={!!working} onClick={() => void check()}>{working === "lookup" ? t($ => $.resources.working) : t($ => $.resources.check)}</Button>}
        <Button type="button" variant="ghost" disabled={!!uncertain || working === "mutation" || working === "lookup"} onClick={() => { invalidatePreview(); onClose(); }}>{t($ => $.resources.cancel)}</Button>
      </div>
    </form>
  </section>;
}
