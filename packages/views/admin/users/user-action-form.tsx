"use client";

import { useRef, useState, type FormEvent } from "react";
import { ApiError, errorCode } from "@multica/core/api";
import {
  AdminUserOperationUncertainError, findAdminUserOperation, useAdminUserMutation,
  type AdminAccountAction, type AdminScope, type AdminUser, type AdminUserOperation,
} from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../../i18n";

export function AdminUserActionForm({ user, action, scope, onClose, onRefresh }: {
  user: AdminUser;
  action: AdminAccountAction | "role";
  scope: AdminScope;
  onClose(): void;
  onRefresh(): void;
}) {
  const { t } = useT("admin");
  const key = useRef(crypto.randomUUID());
  const roleSnapshot = useRef(user.platformRole ?? "");
  const mutation = useAdminUserMutation(scope);
  const [uncertain, setUncertain] = useState(false);
  const [checking, setChecking] = useState(false);
  const [receipt, setReceipt] = useState<AdminUserOperation | null>(null);
  const [feedback, setFeedback] = useState("");
  const [conflict, setConflict] = useState(false);
  const working = mutation.isPending || checking;
  const titles = { disable: t(($) => $.users.disable), restore: t(($) => $.users.restore), "recover-password": t(($) => $.users.recover), role: t(($) => $.users.roleAction) };

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = new FormData(form);
    const password = String(values.get("password") ?? "");
    const reason = String(values.get("reason") ?? "");
    setFeedback("");
    try {
      let result: AdminUserOperation;
      if (action === "role") {
        const selected = String(values.get("role") ?? "");
        roleSnapshot.current = selected;
        const role = selected === "super_admin" || selected === "platform_observer" ? selected : null;
        result = await mutation.mutateAsync({ id: user.id, key: key.current, action, body: { role, expectedRole: user.platformRole, expectedAuthVersion: user.authVersion, reason, password } });
      } else {
        result = await mutation.mutateAsync({ id: user.id, key: key.current, action, body: {
          expectedAuthVersion: user.authVersion, reason, password,
          ...(action === "recover-password" ? { temporaryPassword: String(values.get("temporary_password") ?? ""), username: String(values.get("username") ?? "") || undefined } : {}),
        } });
      }
      setReceipt(result);
      setUncertain(false);
    } catch (error) {
      if (error instanceof AdminUserOperationUncertainError) {
        setUncertain(true);
        setFeedback(t(($) => $.users.uncertain));
      } else if (error instanceof ApiError && (errorCode(error) === "password_verification_failed" || errorCode(error) === "password_verification_stale")) {
        setFeedback(t(($) => $.users.passwordFailed));
      } else if (error instanceof ApiError && errorCode(error) === "account_remains_disabled") {
        setFeedback(t(($) => $.users.targetUnavailable));
      } else if (error instanceof ApiError && error.status === 409) {
        setConflict(true);
        setFeedback(t(($) => $.users.conflict));
      } else {
        setFeedback(t(($) => $.users.failed));
      }
    } finally {
      for (const name of ["password", "temporary_password"]) {
        const input = form.elements.namedItem(name);
        if (input instanceof HTMLInputElement) input.value = "";
      }
      mutation.reset();
    }
  }
  async function check() {
    setChecking(true);
    try {
      const found = await findAdminUserOperation(scope, key.current);
      const expectedKind = action === "role" ? "user.role" : action === "recover-password" ? "user.password.recover" : `user.${action}`;
      if (found && found.targetId === user.id && found.kind === expectedKind) { setReceipt(found); setUncertain(false); onRefresh(); }
      else setFeedback(t(($) => $.users.notRecorded));
    } catch { setFeedback(t(($) => $.users.uncertain)); }
    finally { setChecking(false); }
  }
  if (receipt) return (
    <section role="status" aria-live="polite" className="space-y-3 rounded-md border border-surface-border p-5">
      <p className="font-medium">{receipt.state === "applied" ? t(($) => $.users.succeeded) : t(($) => $.users.uncertain)}</p>
      <p className="break-all text-caption text-muted-foreground">{t(($) => $.users.key)}: {key.current}</p>
      <p className="text-body text-muted-foreground">{t(($) => $.users.processNotice)}</p>
      <Button variant="outline" onClick={onClose}>{t(($) => $.users.close)}</Button>
    </section>
  );
  return (
    <section aria-labelledby="account-action-title" className="space-y-4 rounded-md border border-surface-border p-5">
      <h2 id="account-action-title" className="text-body-lg font-semibold">{titles[action]}: {user.name}</h2>
      <p className="max-w-prose text-body text-muted-foreground">{action === "role" ? t(($) => $.users.roleNotice) : t(($) => $.users.actionNotice)}</p>
      {action === "recover-password" && <p className="max-w-prose text-body text-muted-foreground">{t(($) => $.users.recoveryNotice)}</p>}
      <form onSubmit={submit} className="grid max-w-xl gap-4">
        {action === "role" && uncertain && <input type="hidden" name="role" value={roleSnapshot.current} />}
        {action === "role" && <label className="space-y-1 text-caption">{t(($) => $.users.role)}<select name="role" defaultValue={user.platformRole ?? ""} disabled={working || uncertain} className="h-10 w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-ring"><option value="">{t(($) => $.users.roleRemove)}</option><option value="platform_observer">{t(($) => $.users.observer)}</option><option value="super_admin">{t(($) => $.users.superAdmin)}</option></select></label>}
        <label className="space-y-1 text-caption">{t(($) => $.users.reason)}<Textarea autoFocus name="reason" required maxLength={1000} readOnly={working || uncertain} /></label>
        {action === "recover-password" && !user.username && <label className="space-y-1 text-caption">{t(($) => $.users.initialUsername)}<Input name="username" required minLength={3} maxLength={32} pattern="[A-Za-z0-9_]+" readOnly={working || uncertain} autoComplete="off" /></label>}
        {action === "recover-password" && <label className="space-y-1 text-caption">{t(($) => $.users.tempPassword)}<Input name="temporary_password" type="password" required minLength={6} maxLength={128} disabled={working} autoComplete="new-password" /></label>}
        <label className="space-y-1 text-caption">{t(($) => $.users.password)}<Input name="password" type="password" required maxLength={128} disabled={working} autoComplete="current-password" /></label>
        {feedback && <p role="alert" className="text-body">{feedback}</p>}
        {uncertain && <p className="break-all text-caption">{t(($) => $.users.key)}: {key.current}</p>}
        <div className="flex flex-wrap gap-3">
          <Button type="submit" disabled={working || conflict}>{working ? t(($) => $.users.working) : t(($) => $.users.submit)}</Button>
          {uncertain && <Button type="button" variant="outline" disabled={working} onClick={() => void check()}>{t(($) => $.users.checkStatus)}</Button>}
          {conflict && <Button type="button" variant="outline" onClick={() => { onRefresh(); onClose(); }}>{t(($) => $.users.refresh)}</Button>}
          <Button type="button" variant="ghost" disabled={working || uncertain} onClick={onClose}>{t(($) => $.users.cancel)}</Button>
        </div>
      </form>
    </section>
  );
}
