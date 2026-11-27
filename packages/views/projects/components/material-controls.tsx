"use client";

import { useEffect, useRef, useState } from "react";
import { ApiError, errorCode } from "@multica/core/api";
import { setMaterialPendingRequest, useMaterialDraftStore } from "@multica/core/materials";
import { Button } from "@multica/ui/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { useT } from "../../i18n";

export function MaterialSelect({ id, label, value, options, onChange, disabled }: {
  id: string; label: string; value: string; options: { value: string; label: string }[];
  onChange: (value: string) => void; disabled?: boolean;
}) {
  return <div className="min-w-0 space-y-2">
    <label className="text-caption font-medium" htmlFor={id}>{label}</label>
    <Select items={options} value={value} disabled={disabled} onValueChange={(next) => { if (next !== null) onChange(next); }}>
      <SelectTrigger id={id} className="w-full"><SelectValue /></SelectTrigger>
      <SelectContent>{options.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
    </Select>
  </div>;
}

// An uncertain write keeps its original body and request ID until reconciled.
// Definite HTTP refusals release the form; transport and malformed success do not.
export function useMaterialAction<T, R>(write: (input: T) => Promise<R>, key: string, decode: (value: unknown) => T | null, onRecovered?: (result: R) => void, holdIncompatible = false) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const stored = useMaterialDraftStore((state) => state.draft.requests[key]);
  const pending = decode(stored);
  const incompatible = holdIncompatible && stored !== undefined && pending === null;
  const lock = useRef(false);
  const mounted = useRef(true);
  const onSuccess = useRef<((result: R) => void) | undefined>(undefined);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const execute = async (input: T) => {
    if (lock.current) return;
    lock.current = true;
    setMaterialPendingRequest(key, input);
    setBusy(true); setError(null);
    try {
      const result = await write(input);
      if (!mounted.current) return;
      setMaterialPendingRequest(key, null);
      (onSuccess.current ?? onRecovered)?.(result);
    } catch (failure) {
      if (!mounted.current) return;
      setError(failure);
      const definite = failure instanceof ApiError && failure.status >= 400 && failure.status < 500 && failure.status !== 408;
      setMaterialPendingRequest(key, definite ? null : input);
    } finally {
      lock.current = false;
      if (mounted.current) setBusy(false);
    }
  };
  return { busy, incompatible, error: error ?? (pending !== null && !busy ? new Error("Unconfirmed request") : null), pending, locked: busy || pending !== null || incompatible,
    submit: (input: T, success?: (result: R) => void) => {
      if (pending !== null || incompatible || lock.current) return;
      onSuccess.current = success;
      void execute(input);
    },
    retry: () => { if (pending !== null) void execute(pending); },
  };
}

export function MaterialActionNotice({ error, uncertain, busy, onRetry }: { error: unknown; uncertain: boolean; busy: boolean; onRetry: () => void }) {
  const { t } = useT("projects");
  const labels = useMaterialLabels();
  if (!error) return null;
  return <div role="alert" className="space-y-2 text-caption text-destructive">
    <p>{uncertain ? t(($) => $.materials.unknownResult) : errorCode(error) ? labels.blocker(errorCode(error)!) : t(($) => $.materials.writeError)}</p>
    {uncertain && <Button type="button" size="sm" variant="outline" disabled={busy} onClick={onRetry}>{t(($) => $.materials.retryOriginal)}</Button>}
  </div>;
}

export function MaterialLoadError({ onRetry, unsupported = false, busy = false }: { onRetry: () => void; unsupported?: boolean; busy?: boolean }) {
  const { t } = useT("projects");
  return <div role="alert" className="space-y-3 py-4 text-body text-muted-foreground">
    <p>{unsupported ? t(($) => $.materials.unsupported) : t(($) => $.materials.loadError)}</p>
    <Button type="button" size="sm" variant="outline" disabled={busy} onClick={onRetry}>{t(($) => $.materials.refresh)}</Button>
  </div>;
}

export function useMaterialLabels() {
  const { t } = useT("projects");
  const states = t(($) => $.materials.states, { returnObjects: true });
  const plans = t(($) => $.materials.planStates, { returnObjects: true });
  const operations = t(($) => $.materials.operationStates, { returnObjects: true });
  const publications = t(($) => $.materials.publicationStates, { returnObjects: true });
  const label = (map: Record<string, string>, value: string) => Object.hasOwn(map, value) ? map[value] : t(($) => $.materials.unknownState);
  const blocker = (value: string) => {
    if (/runtime_required|runtime_unavailable|runtime_changed/.test(value)) return t(($) => $.materials.blockerMessages.runtime);
    if (/runtime_offline/.test(value)) return t(($) => $.materials.runtimeOffline);
    if (/protocol|capability|daemon_upgrade/.test(value)) return t(($) => $.materials.blockerMessages.runtimeUpgrade);
    if (/git_auth|authentication|git_tool|cli_missing/.test(value)) return t(($) => $.materials.blockerMessages.runtimeAuth);
    if (/not_reviewable|invalid_material_(revision|source)/.test(value)) return t(($) => $.materials.blockerMessages.notReady);
    if (/reviewer_required|human_required/.test(value)) return t(($) => $.materials.blockerMessages.reviewer);
    if (/agent_unavailable|plan_unavailable|plan_start_pending/.test(value)) return t(($) => $.materials.blockerMessages.agent);
    if (/source_unavailable/.test(value)) return t(($) => $.materials.blockerMessages.sources);
    if (/not_approved|approval/.test(value)) return t(($) => $.materials.blockerMessages.approval);
    if (/permission_denied|credential_revoked|manager_required/.test(value)) return t(($) => $.materials.blockerMessages.permissions);
    if (/configuration|connection|repository_mismatch|resource_changed|resource_unavailable/.test(value)) return t(($) => $.materials.blockerMessages.configuration);
    if (/head_changed|baseline_changed|version_changed|revision_changed|review_changed/.test(value)) return t(($) => $.materials.blockerMessages.version);
    if (/file_not_regular|path_not_directory|path_not_allowed|unexpected_files/.test(value)) return t(($) => $.materials.blockerMessages.unsafe);
    if (/conflict|diverged|closed/.test(value)) return t(($) => $.materials.blockerMessages.conflict);
    if (/pipeline|checks|ci_|checking|native_merge_blocked/.test(value)) return t(($) => $.materials.blockerMessages.checks);
    if (/not_confirmed|transport|merge_incomplete/.test(value)) return t(($) => $.materials.blockerMessages.uncertain);
    if (/disabled|not_configured/.test(value)) return t(($) => $.materials.blockerMessages.disabled);
    return t(($) => $.materials.blockerMessages.unavailable);
  };
  return { operation: (value: string) => label(operations, value), state: (value: string) => label(states, value), plan: (value: string) => label(plans, value), publication: (value: string) => label(publications, value), blocker };
}
