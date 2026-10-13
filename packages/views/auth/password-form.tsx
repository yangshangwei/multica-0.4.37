"use client";

import { useEffect, useRef, useState, type ComponentProps, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Eye, EyeOff } from "lucide-react";
import { useAuthStore, userAuthStatus } from "@multica/core/auth";
import { api, ApiError } from "@multica/core/api";
import { useConfigStore } from "@multica/core/config";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@multica/ui/components/ui/input-group";
import { Label } from "@multica/ui/components/ui/label";
import { Card, CardHeader, CardDescription, CardContent, CardFooter } from "@multica/ui/components/ui/card";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { passwordErrorDetails } from "./password-error";

function PasswordInput({ id, disabled, ...props }: Omit<ComponentProps<typeof Input>, "type"> & { id: string }) {
  const { t } = useT("auth");
  const [visible, setVisible] = useState(false);
  const visibilityLabel = visible ? t(($) => $.password.hide) : t(($) => $.password.show);

  return <InputGroup className="h-11 border-muted-foreground/75 has-[[data-slot=input-group-control]:focus-visible]:border-foreground">
    <InputGroupInput {...props} className="h-full" id={id} disabled={disabled} type={visible ? "text" : "password"} />
    <InputGroupAddon align="inline-end" className="p-0 has-[>button]:mr-0">
      <InputGroupButton
        type="button"
        size="icon-sm"
        className="size-11"
        aria-label={visibilityLabel}
        aria-describedby={`${id}-label`}
        aria-controls={id}
        aria-pressed={visible}
        title={visibilityLabel}
        disabled={disabled}
        onMouseDown={(event) => event.preventDefault()}
        onClick={() => setVisible((value) => !value)}
      >
        {visible ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
      </InputGroupButton>
    </InputGroupAddon>
  </InputGroup>;
}

export function PasswordForm({ mode = "login", onSuccess, footer, logo, title: loginTitle, description: loginDescription, className, onTokenObtained, cliCallback, skipWorkspaceBootstrap = false }: {
  cliCallback?: {url: string; state: string};
  skipWorkspaceBootstrap?: boolean;
  mode?: "login" | "setup" | "change";
  onSuccess: () => void;
  footer?: ReactNode;
  logo?: ReactNode;
  title?: ReactNode;
  description?: ReactNode;
  className?: string;
  onTokenObtained?: () => void;
}) {
  const { t } = useT("auth");
  const qc = useQueryClient();
  const [register, setRegister] = useState(false);
  const [username, setUsername] = useState("");
  const [name, setName] = useState(mode === "setup" ? useAuthStore.getState().user?.name ?? "" : "");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [confirmationTouched, setConfirmationTouched] = useState(false);
  const confirmationRef = useRef<HTMLInputElement>(null);
  const [current, setCurrent] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const pending = useRef(false);
  const [retryAfter, setRetryAfter] = useState(0);
  useEffect(() => {
    if (retryAfter <= 0) return;
    const timer = setTimeout(() => setRetryAfter((seconds) => Math.max(0, seconds - 1)), 1000);
    return () => clearTimeout(timer);
  }, [retryAfter]);
  const signup = useConfigStore((s) => s.passwordSignupAvailable);
  const creating = register || mode === "setup";
  const passwordsMismatch = register && password !== confirmPassword;
  const showConfirmationError = passwordsMismatch && confirmationTouched;
  const title = mode === "change" ? t(($) => $.password.change_title) : mode === "setup" ? t(($) => $.password.setup_title) : register ? t(($) => $.password.register_title) : t(($) => $.password.login);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (pending.current || retryAfter > 0) return;
    if (passwordsMismatch) {
      setConfirmationTouched(true);
      confirmationRef.current?.focus();
      return;
    }
    pending.current = true;
    setLoading(true); setError("");
    try {
      const store = useAuthStore.getState();
      const user = mode === "change" ? await store.changePassword(current, password)
        : mode === "setup" ? await store.setupPassword(username, password, name)
        : register ? await store.registerPassword(username, password, name)
        : await store.loginWithPassword(username, password);
      setPassword(""); setConfirmPassword(""); setConfirmationTouched(false); setCurrent("");
      onTokenObtained?.();
      if (userAuthStatus(user) !== "authenticated") return;
      if (cliCallback) {
        const { token } = await api.issueCliToken();
        const url = new URL(cliCallback.url);
        url.searchParams.set("token", token);
        url.searchParams.set("state", cliCallback.state);
        window.location.href = url.toString();
        return;
      }
      if (!skipWorkspaceBootstrap) {
        const list = await api.listWorkspaces();
        qc.setQueryData(workspaceKeys.list(), list);
      }
      onSuccess();
    } catch (err) {
      if (err instanceof ApiError && err.retryAfterSeconds) setRetryAfter(err.retryAfterSeconds);
      const detail = passwordErrorDetails(err, { username, name, password, registering: register });
      setError(t(($) => $.password[detail.key], { length: detail.length }));
    } finally { pending.current = false; setLoading(false); }
  }

  return <div className={cn("flex min-h-svh flex-col items-center overflow-y-auto px-4 py-6", className)}>
    <Card className="my-auto w-full max-w-sm shrink-0 gap-5 py-6">
      <CardHeader className="gap-2 px-6 text-center">
        {logo && <div className="mx-auto mb-2">{logo}</div>}
        <h1 className="font-heading text-display-sm font-medium [overflow-wrap:anywhere]">{mode === "login" && !register ? loginTitle ?? title : title}</h1>
        <CardDescription>{mode === "setup" ? t(($) => $.password.setup_description) : mode === "change" ? t(($) => $.password.change_description) : register ? t(($) => $.password.register_description) : loginDescription ?? t(($) => $.password.description)}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 px-6">
        <form id="password-form" onSubmit={submit} className="space-y-4" aria-busy={loading}>
          {mode !== "change" && <div className="space-y-2">
            <Label htmlFor="password-username">{t(($) => $.password.username)}</Label>
            <Input id="password-username" className="h-11 border-muted-foreground/75 focus-visible:border-foreground" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required autoCapitalize="none" spellCheck={false} aria-describedby={creating ? "username-help" : undefined} disabled={loading} />
            {creating && <p id="username-help" className="text-caption text-muted-foreground">{t(($) => $.password.username_help)}</p>}
          </div>}
          {creating && <div className="space-y-2">
            <Label htmlFor="password-name">{t(($) => $.password.name)}</Label>
            <Input id="password-name" className="h-11 border-muted-foreground/75 focus-visible:border-foreground" autoComplete="name" value={name} onChange={(e) => setName(e.target.value)} required disabled={loading} aria-describedby="name-help" />
            <p id="name-help" className="text-caption text-muted-foreground">{t(($) => $.password.name_help)}</p>
          </div>}
          {mode === "change" && <div className="space-y-2">
            <Label id="password-current-label" htmlFor="password-current">{t(($) => $.password.current)}</Label>
            <PasswordInput id="password-current" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required disabled={loading} />
          </div>}
          <div className="space-y-2">
            <Label id="password-value-label" htmlFor="password-value">{mode === "change" ? t(($) => $.password.new_password) : t(($) => $.password.password)}</Label>
            <PasswordInput key={register ? "register" : mode} id="password-value" autoComplete={creating || mode === "change" ? "new-password" : "current-password"} value={password} onChange={(e) => setPassword(e.target.value)} required disabled={loading} aria-describedby={[creating || mode === "change" ? "password-help" : "", error ? "password-error" : ""].filter(Boolean).join(" ") || undefined} />
            {(creating || mode === "change") && <p id="password-help" className="text-caption text-muted-foreground">{t(($) => $.password.password_help)}</p>}
          </div>
          {register && <div className="space-y-2">
            <Label id="password-confirm-label" htmlFor="password-confirm">{t(($) => $.password.confirm_password)}</Label>
            <PasswordInput
              ref={confirmationRef}
              id="password-confirm"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              onBlur={() => setConfirmationTouched(true)}
              required
              disabled={loading}
              aria-invalid={showConfirmationError || undefined}
              aria-describedby={showConfirmationError ? "password-confirm-error" : undefined}
            />
            {showConfirmationError && <p id="password-confirm-error" role="alert" className="text-body text-destructive">{t(($) => $.password.passwords_mismatch)}</p>}
          </div>}
          {error && <p id="password-error" role="alert" className="text-body text-destructive">{error}</p>}
          <Button type="submit" className="min-h-11 w-full whitespace-normal" disabled={loading || retryAfter > 0}>{retryAfter > 0 ? t(($) => $.password.retry_after, { seconds: retryAfter }) : loading ? t(($) => $.password.submitting) : register ? t(($) => $.password.register) : title}</Button>
        </form>
        <div className="flex flex-col items-center gap-1 text-center">
          {mode === "login" ? <>
            {signup && <Button variant="link" className="min-h-11 whitespace-normal" disabled={loading} onClick={() => {setRegister(!register);setPassword("");setConfirmPassword("");setConfirmationTouched(false);setError("");}}>{register ? t(($) => $.password.back_to_login) : t(($) => $.password.register_title)}</Button>}
            {!register && <p className="text-caption text-muted-foreground">{t(($) => $.password.recovery)}</p>}
          </> : <Button variant="link" className="min-h-11" disabled={loading} onClick={() => useAuthStore.getState().logout()}>{t(($) => $.password.logout)}</Button>}
        </div>
      </CardContent>
      {footer && <CardFooter className="px-6">{footer}</CardFooter>}
    </Card>
  </div>;
}
