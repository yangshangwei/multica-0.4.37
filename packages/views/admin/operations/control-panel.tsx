"use client";
import { useRef, useState } from "react";
import { adminApiScope, canCancelExecution, canControlAdmission, useAdminAccess, useAdminControlDraft, type AdminExecution, type AdminInstallation, type AdminScope } from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { AdminControlForm, type AdminControlTarget } from "./control-form";

export function AdminControlPanel({ id, scope, target = null, label = "", targets, onRefresh }: { id: string; scope: AdminScope; target?: AdminControlTarget | null; label?: string; targets?: { target: AdminControlTarget; label: string }[]; onRefresh(): Promise<unknown> | void }) {
  const { t } = useT("admin");
  const trigger = useRef<HTMLButtonElement>(null);
  const triggerLabel = useRef("");
  const [snapshot, setSnapshot] = useState<AdminControlTarget | null>(null);
  const saved = useAdminControlDraft(scope, id);
  if (saved.loading) return <p role="status" className="text-caption text-muted-foreground">{t($ => $.operations.restoring)}</p>;
  if (saved.error) return <p role="alert" className="text-body">{t($ => $.operations.storageFailed)}</p>;
  const input = saved.draft?.input;
  const restored: AdminControlTarget | null = input ? input.action === "alert" ? { id: input.id, action: "alert", alertAction: input.body.action, version: input.body.expectedVersion, requiresResolution: input.body.action === "close" && !!input.body.resolutionCode } : input.action === "cancel" ? { id: input.id, action: "cancel", fence: input.body.expectedExecutionFence } : { id: input.id, action: "admission", admission: input.body.admission, version: input.body.expectedAdmissionVersion } : null;
  const selected = snapshot ?? restored;
  return selected ? <AdminControlForm key={input?.key ?? "new"} scope={scope} target={selected} restoredInput={input} onRefresh={onRefresh} onClose={() => { setSnapshot(null); saved.reload(); requestAnimationFrame(() => trigger.current?.focus()); }} />
    : <div className="flex flex-wrap gap-3">{(targets ?? (target ? [{ target, label }] : [])).map(item => <Button key={item.label} ref={node => { if (triggerLabel.current === item.label) trigger.current = node; }} variant="outline" onClick={event => { triggerLabel.current = item.label; trigger.current = event.currentTarget; setSnapshot(item.target); }}>{item.label}</Button>)}</div>;
}
export function AdminInstallationControls({ installation, onRefresh }: { installation: AdminInstallation; onRefresh(): Promise<unknown> | void }) {
  const { t } = useT("admin");
  const access = useAdminAccess();
  if (access.status !== "ready" || access.identity?.role !== "super_admin") return null;
  const scope = { apiScope: adminApiScope(), userId: access.identity.userId, organizationId: access.identity.organizationId };
  const admission = canControlAdmission(access.identity?.role, installation);
  const target: AdminControlTarget | null = admission ? { id: installation.id, action: "admission", admission, version: installation.admissionVersion } : null;
  return <AdminControlPanel key={JSON.stringify(scope)} id={installation.id} scope={scope} target={target} label={admission === "accepting" ? t($ => $.operations.resume) : t($ => $.operations.stop)} onRefresh={onRefresh} />;
}
export function AdminExecutionControls({ execution, onRefresh }: { execution: AdminExecution; onRefresh(): Promise<unknown> | void }) {
  const { t } = useT("admin");
  const access = useAdminAccess();
  if (access.status !== "ready" || access.identity?.role !== "super_admin") return null;
  const scope = { apiScope: adminApiScope(), userId: access.identity.userId, organizationId: access.identity.organizationId };
  const target: AdminControlTarget | null = canCancelExecution(access.identity?.role, execution) && execution.executionFence ? { id: execution.id, action: "cancel", fence: execution.executionFence } : null;
  return <AdminControlPanel key={JSON.stringify(scope)} id={execution.id} scope={scope} target={target} label={t($ => $.operations.cancel)} onRefresh={onRefresh} />;
}
