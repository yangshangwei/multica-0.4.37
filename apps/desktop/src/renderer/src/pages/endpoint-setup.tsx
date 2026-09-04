import { useMemo, useState, type ReactNode } from "react";
import { Alert, AlertDescription } from "@multica/ui/components/ui/alert";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import { DragStrip } from "@multica/views/platform";
import { RESOURCES } from "@multica/views/locales";
import type { SupportedLocale } from "@multica/core/i18n";
import { CheckCircle2, Loader2, Server, XCircle } from "lucide-react";

interface Copy {
  title: string;
  description: string;
  address: string;
  placeholder: string;
  test: string;
  testing: string;
  save: string;
  saving: string;
  required: string;
  invalid: string;
  connected: (ms: number) => string;
  failed: (message: string) => string;
  saveFailed: string;
}

function copyForLocale(locale: string): Copy {
  const key: SupportedLocale = locale.startsWith("zh")
    ? "zh-Hans"
    : locale.startsWith("ko")
      ? "ko"
      : locale.startsWith("ja")
        ? "ja"
        : "en";
  const auth = RESOURCES[key].auth as unknown as {
    desktop: { runtime_config: Record<string, string> };
  };
  const translated = auth.desktop.runtime_config;
  return {
    title: translated.title,
    description: translated.description,
    address: translated.address,
    placeholder: translated.address_placeholder,
    test: translated.test,
    testing: translated.testing,
    save: translated.save,
    saving: translated.saving,
    required: translated.required,
    invalid: translated.invalid,
    connected: (ms) => translated.success.replace("{{latency}}", String(ms)),
    failed: (message) => translated.test_error.replace("{{message}}", message),
    saveFailed: translated.saved_error,
  };
}

function validate(value: string, required: string, invalid: string): string | null {
  if (!value.trim()) return required;
  try {
    const url = new URL(value.trim());
    if (url.protocol !== "http:" && url.protocol !== "https:" || url.username || url.password) return invalid;
    return null;
  } catch { return invalid; }
}

export function DesktopEndpointSetupPage({
  initialError,
  initialApiUrl,
  embedded = false,
  secondaryAction,
}: {
  initialError?: string;
  initialApiUrl?: string;
  embedded?: boolean;
  /** Rendered under the Test/Save row, inside the same centred column. The
   *  login page's "back to sign-in" lives here rather than as a sibling below
   *  this component: `embedded` claims the full height of its container, so a
   *  sibling would sit below the fold with nothing hinting it is there. */
  secondaryAction?: ReactNode;
}) {
  const t = useMemo(() => copyForLocale(window.desktopAPI.systemLocale), []);
  const initialAddress = initialApiUrl ?? "";
  const [address, setAddress] = useState(initialAddress);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<{ kind: "success" | "failure"; message: string } | null>(null);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);

  const onTest = async () => {
    const validation = validate(address, t.required, t.invalid);
    setError(validation); setStatus(null);
    if (validation) return;
    setTesting(true);
    try {
      const result = await window.desktopAPI.testRuntimeConfig(address);
      setStatus(result.ok ? { kind: "success", message: t.connected(result.latencyMs) } : { kind: "failure", message: t.failed(result.message) });
    } catch { setStatus({ kind: "failure", message: t.failed("Network error") }); }
    finally { setTesting(false); }
  };
  const onSave = async () => {
    const validation = validate(address, t.required, t.invalid);
    setError(validation); if (validation) return;
    setSaving(true);
    try {
      if (embedded) await window.daemonAPI.stop();
      const result = await window.desktopAPI.saveRuntimeConfig({ apiUrl: address });
      if (!result.ok) setStatus({ kind: "failure", message: result.message || t.saveFailed });
    } catch { setStatus({ kind: "failure", message: t.saveFailed }); }
    finally { setSaving(false); }
  };

  return <div className={`flex ${embedded ? "min-h-full" : "h-screen"} flex-col bg-background text-foreground`}>
    {!embedded && <DragStrip />}
    <main className="flex flex-1 items-center justify-center p-8">
      <section className="w-full max-w-md">
        <div className="mb-8 text-center"><MulticaIcon bordered size="lg" /><h1 className="mt-5 text-title font-semibold">{t.title}</h1><p className="mt-2 text-body text-muted-foreground">{t.description}</p></div>
        {initialError && <Alert variant="destructive" className="mb-5"><XCircle /><AlertDescription>{initialError}</AlertDescription></Alert>}
        <div className="space-y-2"><Label htmlFor="runtime-address">{t.address}</Label><div className="relative"><Server className="pointer-events-none absolute left-3 top-2.5 size-4 text-muted-foreground" /><Input id="runtime-address" className="pl-9" value={address} onBlur={() => setError(validate(address, t.required, t.invalid))} onChange={(e) => { setAddress(e.target.value); setError(null); setStatus(null); }} placeholder={t.placeholder} aria-invalid={Boolean(error)} aria-describedby={error ? "runtime-address-error" : undefined} /></div>{error && <p id="runtime-address-error" className="text-caption text-destructive">{error}</p>}</div>
        {status && <Alert className="mt-4" variant={status.kind === "failure" ? "destructive" : "default"}>{status.kind === "success" ? <CheckCircle2 className="text-success" /> : <XCircle />}<AlertDescription>{status.message}</AlertDescription></Alert>}
        <div className="mt-6 flex gap-2"><Button type="button" variant="outline" className="flex-1" disabled={testing || saving} onClick={() => void onTest()}>{testing && <Loader2 className="animate-spin" />}{testing ? t.testing : t.test}</Button><Button type="button" className="flex-1" disabled={testing || saving || Boolean(error) || Boolean(validate(address, t.required, t.invalid))} onClick={() => void onSave()}>{saving && <Loader2 className="animate-spin" />}{saving ? t.saving : t.save}</Button></div>
        {secondaryAction && <div className="mt-3">{secondaryAction}</div>}
      </section>
    </main>
  </div>;
}
