import { useState } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { I18nProvider } from "@multica/core/i18n/react";
import { collaborationScope, readCollaborationDeliveryRecords } from "@multica/core/collaboration";
import { newCollaborationDeliveryForm, saveCollaborationDeliveryRecord } from "@verification/core/collaboration/delivery-draft";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { setCurrentWorkspace } from "@verification/core/platform/workspace-storage";
import { defaultStorage } from "@verification/core/platform/storage";
import { CollaborationCreationEntry } from "@verification/views/collaboration/creation-form";
import { PropertyPicker, PickerItem } from "@verification/views/issues/components/pickers/property-picker";
import { NavigationProvider } from "@multica/views/navigation";
import { RESOURCES } from "@multica/views/locales";
import { Button } from "@multica/ui/components/ui/button";
import { Dialog, DialogContent, DialogTitle, DialogDescription, DialogTrigger } from "@multica/ui/components/ui/dialog";
import "./style.css";

const actor = "1e486130-bb06-4e6e-8be9-dd58c8eb83be";
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
setCurrentWorkspace("ambient-other", "00000000-0000-4000-8000-000000000099");
const auth = createAuthStore({ api, storage });
registerAuthStore(auth);
auth.setState({ user: { id: actor, name: "隔离测试账号" }, status: "authenticated", isLoading: false });
const scope = collaborationScope(actor);
const requests: { path: string; search: string; method: string; body?: Record<string, unknown>; key?: string; workspace?: string | null }[] = [];
const navigationCalls: string[] = [];
let mode = "success";
let refusal = "issue_limit_reached";
let pendingCreate: (() => void) | null = null;
let latestInput: Record<string, unknown> = {};
let latestKey = "";
let parentError = false;
let pendingSourceResponses: (() => void)[] = [];
let releaseSourceResponses = !params.has("delaySource");
let uploadMode = "success";
let aiMode = "stream";
const aiStreams: { controller: ReadableStreamDefaultController<Uint8Array>; signal?: AbortSignal | null; cancelled: boolean }[] = [];
const encoder = new TextEncoder();
function emitAI(event: string, data: unknown, index = aiStreams.length - 1) {
  const stream = aiStreams[index];
  if (!stream || stream.cancelled) return false;
  try { stream.controller.enqueue(encoder.encode(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`)); return true; } catch { return false; }
}
const attachmentRecords = new Map<string, Record<string, unknown>>();
const uploadOperations = new Map<string, Record<string, unknown>>();
let attachmentSequence = 100;
const fileMetadata = (draftId: string, file: File) => ({
  id: `00000000-0000-4000-8000-${String(++attachmentSequence).padStart(12, "0")}`,
  draft_id: draftId, issue_id: null, intended_issue_id: null, source_workspace_id: source, uploader_user_id: actor,
  name: file.name, content_type: file.type || "text/plain", size_bytes: file.size, version: "1",
  created_at: "2026-11-07T00:00:00Z", delivered_at: null,
});
function seedAttachments(count: number) {
  const form = readCollaborationDeliveryRecords(scope).find((record) => record.kind === "form")
    ?? newCollaborationDeliveryForm({ server: scope.server, userId: actor, context: { kind: "creation", sourceWorkspaceId: source } }, "creation");
  const attachments = Array.from({ length: count }, (_, index) => fileMetadata(form.draftId,
    new File(["fixture"], `${String(index + 1).padStart(2, "0")}-跨团队协作设计说明-${"LongFilename".repeat(7)}.txt`, { type: "text/plain" })));
  attachments.forEach((file) => attachmentRecords.set(file.id as string, file));
  saveCollaborationDeliveryRecord({ ...form, selectedAttachmentIds: attachments.map((file) => file.id as string) });
}

const sources = [{ id: source, name: params.has("longSource") ? "协作测试 11-06（保留数据）" : "产品研发", slug: "research" }, { id: secondSource, name: "客户协作", slug: "customers" }];
const parentCandidates = [
  { id: "00000000-0000-4000-8000-000000000011", identifier: "SRC-11", name: "协作任务改进" },
  { id: "00000000-0000-4000-8000-000000000012", identifier: "SRC-12", name: "设计审查" },
];
const response = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
const created = () => ({ ...collaborationIssueFixture,
  creator: { ...collaborationIssueFixture.creator, user_id: actor },
  id: "00000000-0000-4000-8000-000000000020", title: latestInput.title,
  source_workspace: { id: latestInput.source_workspace_id, name: "产品研发", identifier: "SRC-20" },
  handler: accounts.find((account) => account.user_id === latestInput.handler_user_id) ?? { user_id: actor, name: "自己", avatar_url: "", external: false },
});

window.fetch = async (input, init = {}) => {
  const url = new URL(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  const headers = new Headers(init.headers);
  const body = typeof init.body === "string" ? JSON.parse(init.body) : undefined;
  requests.push({ path: url.pathname, search: url.search, method: init.method ?? "GET", body, key: headers.get("Idempotency-Key") ?? undefined, workspace: headers.get("X-Workspace-ID") });
  if (url.hostname !== "collaboration-fixture.invalid") throw new Error("Fixture forbids external API transport");
  if (url.pathname === "/api/issues/optimize-description") {
    if (aiMode === "unavailable") return response({ code: "ai_unavailable", error: "Fixture AI is not configured" }, 503);
    if (aiMode === "missing") return response({ error: "Fixture AI route absent" }, 404);
    if (aiMode === "error") return response({ error: "Fixture generation failed" }, 500);
    let record: typeof aiStreams[number];
    const stream = new ReadableStream<Uint8Array>({
      start(controller) { record = { controller, signal: init.signal, cancelled: false }; aiStreams.push(record); },
      cancel() { record.cancelled = true; },
    });
    return new Response(stream, { headers: { "Content-Type": "text/event-stream" } });
  }
  if (url.pathname.endsWith("creation-options")) {
    const id = url.searchParams.get("source_workspace_id");
    const search = url.searchParams.get("parent_search") ?? "";
    if (search && parentError) return response({ error: "Fixture parent query failed" }, 500);
    if (id && !releaseSourceResponses) await new Promise<void>((resolve) => pendingSourceResponses.push(resolve));
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
  if (url.pathname.endsWith("/attachments/drafts") && init.method === "POST") {
    const formData = init.body as FormData;
    const file = formData.get("file") as File;
    const metadata = fileMetadata(String(formData.get("draft_id")), file);
    attachmentRecords.set(metadata.id as string, metadata);
    uploadOperations.set(headers.get("Idempotency-Key") ?? "", metadata);
    if (uploadMode === "delayed") return new Promise<Response>((resolve, reject) => {
      init.signal?.addEventListener("abort", () => reject(new DOMException("Fixture upload cancelled", "AbortError")), { once: true });
    });
    if (uploadMode === "lost") throw new TypeError("Fixture upload response lost");
    return response(metadata);
  }
  if (url.pathname.includes("/attachments/drafts/") && url.pathname.includes("/operations/")) {
    if (uploadMode === "recovery-error") return response({ code: "attachment_not_found", error: "Fixture recovery unavailable" }, 404);
    return response(uploadOperations.get(url.pathname.split("/").pop()!) ?? null);
  }
  if (url.pathname.includes("/attachments/drafts/")) {
    const draftId = url.pathname.split("/").pop();
    return response({ draft_id: draftId, attachments: [...attachmentRecords.values()].filter((file) => file.draft_id === draftId) });
  }
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
  assistTranslations: RESOURCES[locale].modals.create_issue.ai_optimize,
  setAIMode: (next: string) => { aiMode = next; }, emitAI,
  aiState: () => aiStreams.map((stream) => ({ cancelled: stream.cancelled, aborted: stream.signal?.aborted })),
  pendingSourceCount: () => pendingSourceResponses.length,
  releaseSources: () => { releaseSourceResponses = true; pendingSourceResponses.splice(0).forEach((resolve) => resolve()); },
  setMode: (next: string) => { mode = next; }, setRefusal: (next: string) => { refusal = next; },
  finish: () => pendingCreate?.(), hasPending: () => pendingCreate !== null,
  records: () => readCollaborationDeliveryRecords(scope),
  parentError: (next: boolean) => { parentError = next; },
  seedAttachments, setUploadMode: (next: string) => { uploadMode = next; },
  setInitialFields: (fields: Record<string, unknown>) => {
    const form = readCollaborationDeliveryRecords(scope).find((record) => record.kind === "form");
    if (form) saveCollaborationDeliveryRecord({ ...form, initialFields: fields });
  },
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
      <label>Background fixture input<input data-background-focus aria-label="Background fixture input" /></label>
      <CollaborationCreationEntry scope={scope} sourceWorkspaceId={params.has("noSource") ? undefined : source} />
      <div><PropertyPicker open={genericOpen} onOpenChange={setGenericOpen} searchable trigger={<span>Shared picker compatibility</span>}>
        {["First", "Second"].map((value) => <PickerItem key={value} selected={genericSelection === value} onClick={() => { setGenericSelection(value); setGenericOpen(false); }}>{value}</PickerItem>)}
      </PropertyPicker></div>
      <Dialog><DialogTrigger render={<Button variant="outline" />}>Shared dialog compatibility</DialogTrigger><DialogContent><DialogTitle>Shared dialog</DialogTitle><DialogDescription>Unchanged primitive outside the creation surface.</DialogDescription></DialogContent></Dialog>
    </main>
  </NavigationProvider></I18nProvider></QueryClientProvider>;
}
createRoot(document.getElementById("root")!).render(<App />);
