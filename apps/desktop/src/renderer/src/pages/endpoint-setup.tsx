import { useMemo, useState } from "react";
import { Alert, AlertDescription } from "@multica/ui/components/ui/alert";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import { DragStrip } from "@multica/views/platform";
import { CheckCircle2, Loader2, Server, XCircle } from "lucide-react";

type Mode = "cloud" | "private";
type Copy = typeof COPY.en;
const COPY = {
  en: { title: "Connect to your Multica server", description: "Choose Multica Cloud or enter the address of your private deployment.", cloud: "Multica Cloud", private: "Private deployment", address: "Backend address", placeholder: "https://multica.example.com", test: "Test connection", testing: "Testing...", save: "Save and continue", saving: "Saving...", required: "Enter a server address", invalid: "Enter a valid http or https URL", connected: (ms: number) => `Connected in ${ms} ms`, failed: (m: string) => `Connection failed: ${m}`, saveFailed: "Could not save the server configuration." },
  "zh-Hans": { title: "连接到 Multica 服务器", description: "选择 Multica Cloud，或输入私有化部署的地址。", cloud: "Multica Cloud", private: "私有化部署", address: "后端地址", placeholder: "https://multica.example.com", test: "测试连接", testing: "测试中...", save: "保存并继续", saving: "保存中...", required: "请输入服务器地址", invalid: "请输入有效的 http 或 https 地址", connected: (ms: number) => `已连接，用时 ${ms} 毫秒`, failed: (m: string) => `连接失败：${m}`, saveFailed: "无法保存服务器配置。" },
  ko: { title: "Multica 서버에 연결", description: "Multica Cloud를 선택하거나 비공개 배포 주소를 입력하세요.", cloud: "Multica Cloud", private: "비공개 배포", address: "백엔드 주소", placeholder: "https://multica.example.com", test: "연결 테스트", testing: "테스트 중...", save: "저장하고 계속", saving: "저장 중...", required: "서버 주소를 입력하세요", invalid: "유효한 http 또는 https URL을 입력하세요", connected: (ms: number) => `${ms}ms 만에 연결됨`, failed: (m: string) => `연결 실패: ${m}`, saveFailed: "서버 설정을 저장하지 못했습니다." },
  ja: { title: "Multica サーバーに接続", description: "Multica Cloud またはプライベート環境のアドレスを選択してください。", cloud: "Multica Cloud", private: "プライベート環境", address: "バックエンドアドレス", placeholder: "https://multica.example.com", test: "接続をテスト", testing: "テスト中...", save: "保存して続行", saving: "保存中...", required: "サーバーアドレスを入力してください", invalid: "有効な http または https URL を入力してください", connected: (ms: number) => `${ms} ms で接続しました`, failed: (m: string) => `接続に失敗しました: ${m}`, saveFailed: "サーバー設定を保存できませんでした。" },
} as const;

function copyForLocale(locale: string): Copy {
  if (locale.startsWith("zh")) return COPY["zh-Hans"];
  if (locale.startsWith("ko")) return COPY.ko;
  if (locale.startsWith("ja")) return COPY.ja;
  return COPY.en;
}

function validate(value: string, required: string, invalid: string): string | null {
  if (!value.trim()) return required;
  try {
    const url = new URL(value.trim());
    if (url.protocol !== "http:" && url.protocol !== "https:" || url.username || url.password) return invalid;
    return null;
  } catch { return invalid; }
}

export function DesktopEndpointSetupPage({ initialError }: { initialError?: string }) {
  const t = useMemo(() => copyForLocale(window.desktopAPI.systemLocale), []);
  const [mode, setMode] = useState<Mode>("cloud");
  const [address, setAddress] = useState("https://api.multica.ai");
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<{ kind: "success" | "failure"; message: string } | null>(null);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);

  const onModeChange = (next: Mode) => {
    setMode(next);
    setAddress(next === "cloud" ? "https://api.multica.ai" : "");
    setError(null); setStatus(null);
  };
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
      const result = await window.desktopAPI.saveRuntimeConfig({ apiUrl: address });
      if (!result.ok) setStatus({ kind: "failure", message: result.message || t.saveFailed });
    } catch { setStatus({ kind: "failure", message: t.saveFailed }); }
    finally { setSaving(false); }
  };

  return <div className="flex h-screen flex-col bg-background text-foreground">
    <DragStrip />
    <main className="flex flex-1 items-center justify-center p-8">
      <section className="w-full max-w-md">
        <div className="mb-8 text-center"><MulticaIcon bordered size="lg" /><h1 className="mt-5 text-title font-semibold">{t.title}</h1><p className="mt-2 text-body text-muted-foreground">{t.description}</p></div>
        {initialError && <Alert variant="destructive" className="mb-5"><XCircle /><AlertDescription>{initialError}</AlertDescription></Alert>}
        <div className="mb-5 grid grid-cols-2 rounded-lg border bg-muted/40 p-1" role="tablist">
          {([["cloud", t.cloud], ["private", t.private]] as const).map(([value, label]) => <button key={value} type="button" role="tab" aria-selected={mode === value} className={`rounded-md px-3 py-2 text-body font-medium transition ${mode === value ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground"}`} onClick={() => onModeChange(value)}>{label}</button>)}
        </div>
        <div className="space-y-2"><Label htmlFor="runtime-address">{t.address}</Label><div className="relative"><Server className="pointer-events-none absolute left-3 top-2.5 size-4 text-muted-foreground" /><Input id="runtime-address" className="pl-9" value={address} onChange={(e) => { setAddress(e.target.value); setError(null); setStatus(null); }} placeholder={t.placeholder} aria-invalid={Boolean(error)} aria-describedby={error ? "runtime-address-error" : undefined} readOnly={mode === "cloud"} /></div>{error && <p id="runtime-address-error" className="text-caption text-destructive">{error}</p>}</div>
        {status && <Alert className="mt-4" variant={status.kind === "failure" ? "destructive" : "default"}>{status.kind === "success" ? <CheckCircle2 className="text-success" /> : <XCircle />}<AlertDescription>{status.message}</AlertDescription></Alert>}
        <div className="mt-6 flex gap-2"><Button type="button" variant="outline" className="flex-1" disabled={testing || saving} onClick={() => void onTest()}>{testing && <Loader2 className="animate-spin" />}{testing ? t.testing : t.test}</Button><Button type="button" className="flex-1" disabled={testing || saving || Boolean(error)} onClick={() => void onSave()}>{saving && <Loader2 className="animate-spin" />}{saving ? t.saving : t.save}</Button></div>
      </section>
    </main>
  </div>;
}
