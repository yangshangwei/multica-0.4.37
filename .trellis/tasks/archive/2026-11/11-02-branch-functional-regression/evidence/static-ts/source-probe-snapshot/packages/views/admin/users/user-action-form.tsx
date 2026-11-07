"use client";

import { useId, useRef, useState, type FormEvent } from "react";
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
  const [reasonError, setReasonError] = useState(false);
  const reasonErrorId = useId();
  const [passwordMismatch, setPasswordMismatch] = useState(false);
  const [showTemporaryPasswords, setShowTemporaryPasswords] = useState(false);
  const passwordFieldsId = useId();
  const [conflict, setConflict] = useState(false);
  const working = mutation.isPending || checking;
  const isPasswordReset = action === "recover-password";
  const titles = { disable: t(($) => $.users.disable), restore: t(($) => $.users.restore), "recover-password": t(($) => $.users.recover), role: t(($) => $.users.roleAction) };

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = new FormData(form);
    const password = String(values.get("password") ?? "");
    const reason = String(values.get("reason") ?? "").trim();
    setFeedback("");
    if (!reason) {
      setReasonError(true);
      const input = form.elements.namedItem("reason");
      if (input instanceof HTMLElement) input.focus();
      return;
    }
    setReasonError(false);
    if (isPasswordReset && values.get("temporary_password") !== values.get("confirm_temporary_password")) {
      setPasswordMismatch(true);
      const input = form.elements.namedItem("confirm_temporary_password");
      if (input instanceof HTMLElement) input.focus();
      return;
    }
    setPasswordMismatch(false);
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
      } else if (error instanceof ApiError && errorCode(error) === "username_taken") {
        setFeedback(t(($) => $.users.usernameTaken));
      } else if (error instanceof ApiError && (errorCode(error) === "invalid_username" || errorCode(error) === "username_required")) {
        setFeedback(t(($) => $.users.invalidUsername));
      } else if (error instanceof ApiError && errorCode(error) === "invalid_temporary_password") {
        setFeedback(t(($) => $.users.invalidTempPassword));
      } else if (error instanceof ApiError && error.status === 409) {
        setConflict(true);
        setFeedback(t(($) => $.users.conflict));
      } else {
        setFeedback(isPasswordReset ? t(($) => $.users.resetFailed) : t(($) => $.users.failed));
      }
    } finally {
      for (const name of ["password", "temporary_password", "confirm_temporary_password"]) {
        const input = form.elements.namedItem(name);
        if (input instanceof HTMLInputElement) input.value = "";
      }
      setShowTemporaryPasswords(false);
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
      <p className="font-medium">{receipt.state === "applied" ? isPasswordReset ? t(($) => $.users.resetSucceeded) : t(($) => $.users.succeeded) : t(($) => $.users.uncertain)}</p>
      {isPasswordReset && receipt.state === "applied" && <>
        <p className="max-w-prose text-body text-muted-foreground">{t(($) => $.users.recoveryNotice)}</p>
        {user.status === "disabled" && <p className="max-w-prose text-body text-muted-foreground">{t(($) => $.users.resetDisabledNotice)}</p>}
      </>}
      <p className="break-all text-caption text-muted-foreground">{t(($) => $.users.key)}: {key.current}</p>
      <p className="text-body text-muted-foreground">{t(($) => $.users.processNotice)}</p>
      <Button variant="outline" onClick={onClose}>{t(($) => $.users.close)}</Button>
    </section>
  );
  return (
    <section aria-labelledby="account-action-title" className="space-y-4 rounded-md border border-surface-border p-5">
      <h2 id="account-action-title" className="break-words text-body-lg font-semibold">{titles[action]}: {user.name}</h2>
      <p className="max-w-prose text-body text-muted-foreground">{action === "role" ? t(($) => $.users.roleNotice) : isPasswordReset ? t(($) => $.users.resetNotice) : t(($) => $.users.actionNotice)}</p>
      {isPasswordReset && <div className="max-w-prose space-y-2 text-body text-muted-foreground">
        <p>{t(($) => $.users.processNotice)}</p>
        {user.status === "disabled" && <p>{t(($) => $.users.resetDisabledNotice)}</p>}
        <p>{t(($) => $.users.recoveryNotice)}</p>
      </div>}
      <form onSubmit={submit} className="grid max-w-xl gap-4">
        {action === "role" && uncertain && <input type="hidden" name="role" value={roleSnapshot.current} />}
        {action === "role" && <label className="space-y-1 text-caption">{t(($) => $.users.role)}<select name="role" defaultValue={user.platformRole ?? ""} disabled={working || uncertain} className="h-10 w-full rounded-md border border-input bg-background px-3 text-body focus-visible:outline-ring"><option value="">{t(($) => $.users.roleRemove)}</option><option value="platform_observer">{t(($) => $.users.observer)}</option><option value="super_admin">{t(($) => $.users.superAdmin)}</option></select></label>}
        <div className="space-y-1">
          <label className="space-y-1 text-caption">{t(($) => $.users.reason)}<Textarea autoFocus name="reason" required maxLength={1000} readOnly={working || uncertain} aria-invalid={reasonError || undefined} aria-describedby={reasonError ? reasonErrorId : undefined} onChange={() => setReasonError(false)} /></label>
          {reasonError && <p id={reasonErrorId} role="alert" className="text-caption text-destructive">{t(($) => $.operations.reasonRequired)}</p>}
        </div>
        {isPasswordReset && !user.username && <div className="space-y-1">
          <label className="space-y-1 text-caption">{t(($) => $.users.initialUsername)}<Input name="username" required minLength={3} maxLength={32} pattern="[A-Za-z0-9_]+" readOnly={working || uncertain} autoComplete="off" aria-describedby={`${passwordFieldsId}-username-hint`} /></label>
          <p id={`${passwordFieldsId}-username-hint`} className="text-caption text-muted-foreground">{t(($) => $.users.initialUsernameHint)}</p>
        </div>}
        {isPasswordReset && <div className="space-y-3">
          <div className="space-y-1">
            <label className="space-y-1 text-caption">{t(($) => $.users.tempPassword)}<Input id={`${passwordFieldsId}-temporary`} name="temporary_password" type={showTemporaryPasswords ? "text" : "password"} required minLength={6} maxLength={128} disabled={working} autoComplete="new-password" aria-describedby={`${passwordFieldsId}-hint`} onChange={() => setPasswordMismatch(false)} /></label>
            <p id={`${passwordFieldsId}-hint`} className="text-caption text-muted-foreground">{t(($) => $.users.tempPasswordHint)}</p>
          </div>
          <div className="space-y-1">
            <label className="space-y-1 text-caption">{t(($) => $.users.confirmTempPassword)}<Input id={`${passwordFieldsId}-confirmation`} name="confirm_temporary_password" type={showTemporaryPasswords ? "text" : "password"} required minLength={6} maxLength={128} disabled={working} autoComplete="new-password" aria-invalid={passwordMismatch || undefined} aria-describedby={passwordMismatch ? `${passwordFieldsId}-mismatch` : undefined} onChange={() => setPasswordMismatch(false)} /></label>
            {passwordMismatch && <p id={`${passwordFieldsId}-mismatch`} role="alert" className="text-caption text-destructive">{t(($) => $.users.tempPasswordMismatch)}</p>}
          </div>
          <Button type="button" variant="outline" disabled={working} aria-controls={`${passwordFieldsId}-temporary ${passwordFieldsId}-confirmation`} onClick={() => setShowTemporaryPasswords((shown) => !shown)}>{showTemporaryPasswords ? t(($) => $.users.hideTempPasswords) : t(($) => $.users.showTempPasswords)}</Button>
        </div>}
        <div className="space-y-1">
          <label className="space-y-1 text-caption">{isPasswordReset ? t(($) => $.users.adminPassword) : t(($) => $.users.password)}<Input name="password" type="password" required maxLength={128} disabled={working} autoComplete="current-password" aria-describedby={isPasswordReset ? `${passwordFieldsId}-admin-hint` : undefined} /></label>
          {isPasswordReset && <p id={`${passwordFieldsId}-admin-hint`} className="text-caption text-muted-foreground">{t(($) => $.users.adminPasswordHint)}</p>}
        </div>
        {feedback && <p role="alert" className="text-body">{feedback}</p>}
        {uncertain && isPasswordReset && <p className="text-body text-muted-foreground">{t(($) => $.users.resetRetryNotice)}</p>}
        {uncertain && <p className="break-all text-caption">{t(($) => $.users.key)}: {key.current}</p>}
        <div className="flex flex-wrap gap-3">
          <Button type="submit" disabled={working || conflict}>{working ? t(($) => $.users.working) : isPasswordReset ? t(($) => $.users.recover) : t(($) => $.users.submit)}</Button>
          {uncertain && <Button type="button" variant="outline" disabled={working} onClick={() => void check()}>{t(($) => $.users.checkStatus)}</Button>}
          {conflict && <Button type="button" variant="outline" onClick={() => { onRefresh(); onClose(); }}>{t(($) => $.users.refresh)}</Button>}
          <Button type="button" variant="ghost" disabled={working || uncertain} onClick={onClose}>{t(($) => $.users.cancel)}</Button>
        </div>
      </form>
    </section>
  );
}
