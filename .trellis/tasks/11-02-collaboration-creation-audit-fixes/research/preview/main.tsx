import { useState } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { I18nProvider } from "@multica/core/i18n/react";
import { collaborationScope, readCollaborationDeliveryRecords } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { defaultStorage } from "@verification/core/platform/storage";
import { CollaborationCreationEntry } from "@verification/views/collaboration/creation-form";
import { PropertyPicker, PickerItem } from "@verification/views/issues/components/pickers/property-picker";
import { NavigationProvider } from "@multica/views/navigation";
import { RESOURCES } from "@multica/views/locales";
import { Button } from "@multica/ui/components/ui/button";
import { Dialog, DialogContent, DialogTitle, DialogDescription, DialogTrigger } from "@multica/ui/components/ui/dialog";
import "./style.css";

const actor = "00000000-0000-4000-8000-000000000001";
const source = "00000000-0000-4000-8000-000000000002";
const secondSource = "00000000-0000-4000-8000-000000000003";
const accounts = [
  { user_id: "00000000-0000-4000-8000-000000000004", name: "李明", avatar_url: "", external: false },
  { user_id: "00000000-0000-4000-8000-000000000005", name: "李明", avatar_url: "", external: false },
  { user_id: "00000000-0000-4000-8000-000000000006", name: "客户张经理", avatar_url: "", external: true },
];
const params = new URLSearchParams(location.search);
const locale = params.get("locale") === "en" ? "en" : "zh-Hans";
const translations = RESOURCES[locale].collaboration;
const memory = new Map<string, string>();
const storage = {
  getItem: (key: string) => memory.get(key) ?? null,
  setItem: (key: string, value: string) => { memory.set(key, value); },
  removeItem: (key: string) => { memory.delete(key); },
  keys: () => [...memory.keys()],
};
// Only this fresh fixture process uses the in-memory adapter.
Object.assign(defaultStorage, storage);
const api = new ApiClient("https://collaboration-fixture.invalid");
setApiInstance(api);
const auth = createAuthStore({ api, storage });
registerAuthStore(auth);
auth.setState({ user: { id: actor, name: "隔离测试账号" }, status: "authenticated", isLoading: false });
const scope = collaborationScope(actor);
const requests: { path: string; search: string; method: string; body?: Record<string, unknown>; key?: string }[] = [];
const navigationCalls: string[] = [];
let mode = "success";
let refusal = "issue_limit_reached";
let pendingCreate: (() => void) | null = null;
let latestInput: Record<string, unknown> = {};
let latestKey = "";
let parentError = false;
const sources = [{ id: source, name: "产品研发", slug: "research" }, { id: secondSource, name: "客户协作", slug: "customers" }];
const parentCandidates = [
  { id: "00000000-0000-4000-8000-000000000011", identifier: "SRC-11", name: "协作任务改进" },
  { id: "00000000-0000-4000-8000-000000000012", identifier: "SRC-12", name: "设计审查" },
];
const response = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
const created = () => ({ ...collaborationIssueFixture,
  id: "00000000-0000-4000-8000-000000000020", title: latestInput.title,
  source_workspace: { id: latestInput.source_workspace_id, name: "产品研发", identifier: "SRC-20" },
  handler: accounts.find((account) => account.user_id === latestInput.handler_user_id) ?? { user_id: actor, name: "自己", avatar_url: "", external: false },
});

window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  const headers = new Headers(init.headers);
  const body = typeof init.body === "string" ? JSON.parse(init.body) : undefined;
  requests.push({ path: url.pathname, search: url.search, method: init.method ?? "GET", body, key: headers.get("Idempotency-Key") ?? undefined });
  if (url.hostname !== "collaboration-fixture.invalid") throw new Error("Fixture forbids external API transport");
  if (url.pathname.endsWith("creation-options")) {
    const id = url.searchParams.get("source_workspace_id");
    const search = url.searchParams.get("parent_search") ?? "";
    if (search && parentError) return response({ error: "Fixture parent query failed" }, 500);
    const choice = sources.find((item) => item.id === id) ?? null;
    return response({ creation_enabled: true, sources, source: choice, max_attachment_bytes: 104857600,
      ...(choice ? { fields: { statuses: [{ key: "todo", name: "待办", category: "todo" }, { key: "scheduled", name: "已排期", category: "backlog" }], default_status: "todo",
        labels: [], properties: [], source_options: { projects: [{ id: "00000000-0000-4000-8000-000000000010", name: "产品迭代" }],
          parents: parentCandidates.filter((parent) => !search || `${parent.identifier} ${parent.name}`.includes(search)), parents_truncated: false,
          iterations: [], iterations_enabled: false } } } : {}),
    });
  }
  if (url.pathname.endsWith("recipients/search")) {
    const query = body?.q ?? url.searchParams.get("query") ?? url.searchParams.get("q") ?? url.searchParams.get("search") ?? "";
    return response({ issue_id: null, source_workspace_id: body?.source_workspace_id ?? url.searchParams.get("source_workspace_id") ?? source,
      scope: "directory", source_member: true, recent: [], recipients: accounts.filter((account) => !query || `${account.name} ${account.user_id}`.includes(query)), next_cursor: null });
  }
  if (url.pathname.endsWith("recipients/resolve")) return response({ issue_id: null, source_workspace_id: body?.source_workspace_id ?? source, recipient: accounts[2] });
  if (url.pathname.includes("/attachments/drafts/")) return response({ draft_id: url.pathname.split("/").pop(), attachments: [] });
  if (url.pathname === "/api/collaboration/issues" && init.method === "POST") {
    latestInput = body;
    latestKey = headers.get("Idempotency-Key") ?? "";
    if (mode === "delayed") return new Promise<Response>((resolve) => { pendingCreate = () => { resolve(response(created())); pendingCreate = null; }; });
    if (mode === "reject") return response({ code: refusal, error: "Fixture confirmed business rejection" }, 402);
    if (mode === "unknown") throw new TypeError("Fixture loses the creation response");
    return response(created());
  }
  if (url.pathname.endsWith("/result")) {
    if (mode === "unknown") return response({ outcome: "unknown" }, 202);
    return response({ request_id: latestKey, source_workspace_id: latestInput.source_workspace_id, actor_user_id: actor, outcome: "rejected", code: refusal });
  }
  if (url.pathname.includes("/creations/")) {
    if (mode === "delayed-recovery") return new Promise<Response>((resolve) => { pendingCreate = () => { resolve(response(created())); pendingCreate = null; }; });
    return response(created());
  }
  throw new Error(`Fixture route not implemented: ${url.pathname}`);
};

const fixture = {
  actor, source, secondSource, accounts, translations, locale, requests, navigationCalls,
  setMode: (next: string) => { mode = next; }, setRefusal: (next: string) => { refusal = next; },
  finish: () => pendingCreate?.(), hasPending: () => pendingCreate !== null,
  records: () => readCollaborationDeliveryRecords(scope),
  parentError: (next: boolean) => { parentError = next; },
};
Object.assign(window, { fixture });
const navigation = { push: (path: string) => { navigationCalls.push(path); }, replace: (path: string) => { navigationCalls.push(path); },
  back: () => {}, pathname: "/account", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path: string) => path };
const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });

function App() {
  const [genericOpen, setGenericOpen] = useState(false);
  const [genericSelection, setGenericSelection] = useState("First");
  return <QueryClientProvider client={client}><I18nProvider locale={locale} resources={RESOURCES}><NavigationProvider value={navigation}>
    <main className="space-y-4 p-8"><p className="text-caption text-muted-foreground">原始组件 / 模拟 API / 隔离浏览器</p>
      <CollaborationCreationEntry scope={scope} sourceWorkspaceId={source} />
      <div><PropertyPicker open={genericOpen} onOpenChange={setGenericOpen} searchable trigger={<span>Shared picker compatibility</span>}>
        {["First", "Second"].map((value) => <PickerItem key={value} selected={genericSelection === value} onClick={() => { setGenericSelection(value); setGenericOpen(false); }}>{value}</PickerItem>)}
      </PropertyPicker></div>
      <Dialog><DialogTrigger render={<Button variant="outline" />}>Shared dialog compatibility</DialogTrigger><DialogContent><DialogTitle>Shared dialog</DialogTitle><DialogDescription>Unchanged primitive outside the creation surface.</DialogDescription></DialogContent></Dialog>
    </main>
  </NavigationProvider></I18nProvider></QueryClientProvider>;
}
createRoot(document.getElementById("root")!).render(<App />);
