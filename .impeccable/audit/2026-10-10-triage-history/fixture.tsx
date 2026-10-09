import React, { useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import i18next from 'i18next';
import { I18nextProvider, initReactI18next } from 'react-i18next';
import { ApiClient, setApiInstance } from '../../../packages/core/api';
import { createAuthStore, registerAuthStore } from '../../../packages/core/auth';
import { WorkspaceSlugProvider } from '../../../packages/core/paths/hooks';
import { NavigationProvider } from '../../../packages/views/navigation/context';
import { TriagePage } from '../../../packages/views/triage/triage-page';
import '@fontsource-variable/inter';
import '../../../apps/desktop/src/renderer/src/globals.css';

// This fixture renders the actual shared page, router intent hook and controls.
// Only its API responses and identity are synthetic; no account or server is used.
const flags = new URLSearchParams(window.location.search);
const language = flags.get('lang') === 'en' ? 'en' : 'zh-Hans';
const dark = flags.get('theme') === 'dark';
document.documentElement.lang = language;
document.documentElement.classList.toggle('dark', dark);
const localeModules = import.meta.glob('../../../packages/views/locales/{en,zh-Hans}/*.json', { eager: true, import: 'default' });
const resources: Record<string, Record<string, unknown>> = { en: {}, 'zh-Hans': {} };
for (const [filename, value] of Object.entries(localeModules)) {
  const parts = filename.split('/');
  resources[parts.at(-2)!][parts.at(-1)!.replace('.json', '')] = value;
}
const i18n = i18next.createInstance();
await i18n.use(initReactI18next).init({ lng: language, fallbackLng: 'en', resources, interpolation: { escapeValue: false }, react: { useSuspense: false } });

const wsId = '11111111-1111-4111-8111-111111111111';
const actorId = '22222222-2222-4222-8222-222222222222';
const isEnglish = language === 'en';
const user = { id: actorId, name: isEnglish ? 'Alex Yang' : '杨尚伟', email: 'reviewer@example.test' };
const members = [{ id: 'member-1', workspace_id: wsId, user_id: actorId, name: user.name, email: user.email, role: 'member' }];
const rejectionReason = isEnglish ? 'Desktop batch review: keep these test examples and retain their review history.' : '桌面批量审核验收：这两项作为测试样例归档，保留审核历史。';
function decision(id: string, number: number, action: string, created_at: string, source: string, reason: string | null = null) {
  const title = isEnglish ? ({ 4: 'Rejection and another review', 10: 'Successful CSV import', 6: 'Review later', 8: 'Clarify the acceptance criteria' }[number]) : ({ 4: '手测-R 拒绝与重新审核', 10: '手测-CSV 正常导入', 6: '手测-Z 稍后处理', 8: '确认任务验收标准' }[number]);
  const issue = { id: `issue-${number}`, identifier: `DQA-${number}`, title, admission_status: 'pending', status: 'backlog' };
  return {
    id, kind: 'action', issue_id: issue.id, identifier: issue.identifier, title, action,
    actor_id: actorId, created_at, reason, batch_id: null, filename: null, counts: null,
    before: { issue, source, snoozed_until: null },
    after: { issue: { ...issue, admission_status: action === 'reject' ? 'rejected' : action === 'accept' ? 'accepted' : 'pending' }, source, snoozed_until: action === 'snooze' ? '2026-10-09T01:00:00Z' : null },
  };
}
const entries = [
  decision('action-4', 4, 'snooze', '2026-10-08T15:09:59Z', 'manual'),
  decision('action-10', 10, 'reject', '2026-10-08T11:38:25Z', 'csv', rejectionReason),
  decision('action-6', 6, 'reject', '2026-10-08T11:38:25Z', 'manual', rejectionReason),
  { id: 'import-1', kind: 'import', issue_id: null, identifier: null, title: 'desktop-retained-import.csv', action: 'import', actor_id: actorId, created_at: '2026-10-08T11:28:37Z', reason: null, before: {}, after: {}, batch_id: 'batch-1', filename: 'desktop-retained-import.csv', counts: { created: 1, skipped: 0, failed: 0 } },
  decision('action-8', 8, 'accept', '2026-10-07T08:10:00Z', 'manual'),
];
if (flags.has('long')) {
  const longTitle = isEnglish ? 'Review the requirements and acceptance criteria with all affected collaborators '.repeat(5) : '请核对跨日期审核记录与重复提交场景中的全部验收标准，并保留每次处理原因。'.repeat(6);
  entries[0].title = longTitle;
  entries[0].reason = `${rejectionReason}\n${isEnglish ? 'Keep the original audit event even if the task is reviewed again.' : '任务重新审核之后，仍需保留这一次处理记录及其原始原因。'}`;
}

const fixtureApi = new ApiClient('http://127.0.0.1:1');
Object.assign(fixtureApi, {
  listWorkspaces: async () => [{ id: wsId, slug: 'triage-review', name: 'Review fixture', issue_prefix: 'DQA' }],
  listMembers: async () => members,
  listAgents: async () => [],
  listSquads: async () => [],
  listProjects: async () => [],
  listLabels: async () => [],
  listIssueStatuses: async () => ({ statuses: [] }),
  getTriageSettings: async () => ({ enabled: true, supported: true, acceptance_status: 'todo', responsibility_mode: 'none', require_priority: false }),
  listTriageItems: async () => ({ items: [], counts: { pending: 1, ready: 1, snoozed: 0 }, total: 0, limit: 50, offset: 0 }),
  listTriageHistory: async (_wsId: string, params: Record<string, string>) => {
    let rows = flags.has('empty') ? [] : entries;
    if (params.q) rows = rows.filter((entry) => `${entry.identifier ?? ''} ${entry.title}`.toLowerCase().includes(params.q.toLowerCase()));
    if (params.result) rows = rows.filter((entry) => entry.action === params.result);
    if (params.source) rows = rows.filter((entry) => entry.kind === 'import' ? params.source === 'csv' : entry.after.source === params.source);
    if (params.processed_by) rows = rows.filter((entry) => entry.actor_id === params.processed_by);
    if (params.processed_after) rows = rows.filter((entry) => entry.created_at >= params.processed_after);
    if (params.processed_before) rows = rows.filter((entry) => entry.created_at.slice(0, 10) <= params.processed_before);
    const offset = Number(params.offset) || 0;
    return { entries: rows.slice(offset, offset + 50), total: rows.length, limit: 50, offset };
  },
});
setApiInstance(fixtureApi);
const auth = createAuthStore({ api: fixtureApi, storage: { getItem: () => null, setItem: () => {}, removeItem: () => {} } });
auth.setState({ user, status: 'authenticated', isLoading: false });
registerAuthStore(auth);
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity, refetchOnWindowFocus: false } } });
queryClient.setQueryData(['workspaces', 'list'], [{ id: wsId, slug: 'triage-review', name: 'Review fixture', issue_prefix: 'DQA' }]);

function Fixture() {
  const [search, setSearch] = useState(() => {
    const next = new URLSearchParams(window.location.search);
    next.set('view', 'history');
    return next.toString();
  });
  const navigation = useMemo(() => ({
    pathname: '/triage-review/triage', hash: '', searchParams: new URLSearchParams(search),
    push: (value: string) => { window.__triageDestination = value; },
    replace: (value: string) => {
      const next = new URL(value, window.location.origin);
      window.history.replaceState(null, '', next.pathname + next.search);
      setSearch(next.searchParams.toString());
    },
    back: () => {}, getShareableUrl: (value: string) => new URL(value, window.location.origin).href,
  }), [search]);
  return <QueryClientProvider client={queryClient}><I18nextProvider i18n={i18n}><WorkspaceSlugProvider slug="triage-review"><NavigationProvider value={navigation}><main className="flex h-dvh min-w-0 flex-col overflow-hidden bg-background text-foreground"><TriagePage /></main></NavigationProvider></WorkspaceSlugProvider></I18nextProvider></QueryClientProvider>;
}
createRoot(document.getElementById('root')!).render(<Fixture />);
