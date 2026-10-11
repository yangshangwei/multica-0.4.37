import React from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { I18nProvider } from '../../../packages/core/i18n/react';
import { RESOURCES } from '../../../packages/views/locales';
import { ApiClient, setApiInstance } from '../../../packages/core/api';
import { createAuthStore, registerAuthStore } from '../../../packages/core/auth';
import { WorkspaceSlugProvider } from '../../../packages/core/paths/hooks';
import { NavigationProvider } from '../../../packages/views/navigation/context';
import { ProjectOverviewPanel } from '../../../packages/views/projects/components/project-overview';
import { p1Project, p1Overview, p1Revision, p1Member } from '../../../packages/core/projects/test-fixtures/p1';
import '@fontsource-variable/inter';
import '../../../apps/desktop/src/renderer/src/globals.css';

// The production component and styles render synthetic API responses in memory.
// This fixture never mutates a live project or requires a signed-in account.
const flags = new URLSearchParams(location.search);
const en = flags.get('lang') === 'en';
const locale = en ? 'en' : 'zh-Hans';
const long = flags.has('long');
const state = flags.get('state') || 'risk';
document.documentElement.classList.toggle('dark', flags.get('theme') === 'dark');
const project = { ...p1Project, status: 'completed' as const, priority: 'medium' as const, lead_type: 'member' as const,
  title: en ? 'Release verification' : '手测-发布闭环', description_revision: 3 };
const author = { ...p1Member, availability: 'active' as const, name: long ? (en ? 'A very long historical member name '.repeat(4) : '用于检查长名称换行的历史成员'.repeat(4)) : en ? 'Alex Yang' : '杨尚伟' };
const body = en ? 'Stage acceptance: A and E meet the requirements. C and D are not delivered; B was removed from scope.' : '阶段验收：A、E 满足本次验收要求；C、D 尚未交付，B 已从范围中取消。';
const overview = structuredClone(p1Overview);
overview.statistics.counts = { ...overview.statistics.counts, total: 5, completed: 1, cancelled: 1, open: 3, overdue: 2 };
overview.statistics.closure_ratio = .4;
overview.statistics.reasons = ['blocked_issues', 'overdue_issues', 'unassigned_issues', 'in_review_issues', 'ended_project_open_issues'];
overview.statistics.calculated_at = new Date().toISOString();
overview.statistics.reference_date = new Intl.DateTimeFormat('en-CA', { timeZone: overview.statistics.timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date());
if (['clear', 'empty'].includes(state)) {
  overview.statistics.health = state;
  overview.statistics.reasons = [];
  for (const key of Object.keys(overview.statistics.counts)) overview.statistics.counts[key] = 0;
  overview.statistics.counts.total = state === 'empty' ? 0 : 5;
  overview.statistics.counts.completed = state === 'empty' ? 0 : 5;
  overview.statistics.closure_ratio = state === 'empty' ? null : 1;
}
if (state === 'unavailable') {
  overview.statistics.health = 'unavailable'; overview.statistics.complete = false;
  overview.statistics.counts.overdue = null;
  overview.statistics.incomplete_reasons = ['unknown_status'];
}
if (state === 'attention') {
  overview.statistics.health = 'attention'; overview.statistics.reasons = ['stale_progress'];
}
if (state === 'stale') { overview.statistics.health = 'clear'; overview.statistics.reasons = []; }
const published = '2026-10-08T10:46:39Z';
const acceptance = { ...p1Revision, kind: 'acceptance', body: long ? Array.from({ length: 12 }, () => body).join('\n\n') : body,
  created_at: published, editor: author, acceptance: { conclusion: 'partial', scope: en ? 'Deliveries A and E.' : 'A、E 的交付结果；其余任务不在本次通过范围。',
    explanation: en ? 'Verified in Electron. Preserve the historical statistics and evidence.' : '在 Electron 中核对交付结果，保留历史统计与证据。',
    description_revision: 2, description_snapshot: long ? body.repeat(15) : '## Goal\nShip the release' },
  evidence: [{ input: { kind: 'url', url: 'https://example.test/evidence', id: null }, availability: 'unverified',
    label: long ? `https://example.test/${'release-evidence-'.repeat(35)}` : en ? 'Release evidence' : '交付验收证据', href: 'https://example.test/evidence' }], statistics_snapshot: p1Overview.statistics };
overview.latest_acceptance = { update_id: acceptance.update_id, revision: 1, published_at: published, conclusion: 'partial', description_revision: 2, applicable_to_current_description: false, author };
const update = (revision, id, timestamp) => ({ workspace_id: project.workspace_id, project_id: project.id, id, author,
  published_at: timestamp, current_revision: 1, current: { ...revision, update_id: id } });
const entries = [
  update(acceptance, acceptance.update_id, published),
  update({ ...p1Revision, kind: 'risk', body: en ? 'Chinese export filenames still need verification.' : '中文文件名导出仍需验证。', health_judgment: 'attention', editor: author }, 'risk-record', '2026-10-08T08:00:00Z'),
  update({ ...p1Revision, body: en ? 'Release checks recorded. Continue testing the remaining tasks.' : '已记录本轮交付检查，继续验证剩余任务。', editor: author }, 'progress-record', '2026-10-07T08:00:00Z'),
];
const api = new ApiClient('http://127.0.0.1:1');
let overviewReads = 0;
Object.assign(api, {
  getProject: async () => project,
  getProjectOverview: async () => { if (state === 'stale' && overviewReads++ > 0) throw new Error('Fixture refresh failed'); return overview; },
  listProjectUpdates: async (_ws, _id, options) => ({ items: flags.has('empty-updates') ? [] : options?.cursor ? entries.slice(1) : entries, next_cursor: null }),
  listProjectUpdateRevisions: async () => ({ items: [acceptance], next_cursor: null }),
  listWorkspaces: async () => [{ id: project.workspace_id, slug: 'overview-review', name: 'Presentation fixture', issue_prefix: 'QA' }],
  listMembers: async () => [], listAgents: async () => [], listSquads: async () => [], listProjects: async () => [project], listLabels: async () => [],
});
setApiInstance(api);
const auth = createAuthStore({ api, storage: { getItem: () => null, setItem: () => {}, removeItem: () => {} } });
auth.setState({ user: { id: author.id, name: author.name, email: 'reviewer@example.test' }, status: 'authenticated', isLoading: false });
registerAuthStore(auth);
const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity, refetchOnWindowFocus: false } } });
const navigation = { pathname: '/overview-review/projects/review', searchParams: flags, hash: '', push: url => window.__destination = url,
  replace: url => window.__destination = url, back: () => {}, getShareableUrl: value => new URL(value, location.origin).href };
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={client}><I18nProvider locale={locale} resources={RESOURCES}>
  <WorkspaceSlugProvider slug="overview-review"><NavigationProvider value={navigation}>
    <main className="flex h-dvh min-w-0 flex-col overflow-hidden bg-background text-foreground">
      <div className="shrink-0 border-b px-6 py-3 text-body font-medium">{project.title}</div>
      <ProjectOverviewPanel project={project} updatesSupported onRisk={(signal, version) => { window.__riskSelection = { signal, version }; }} onProtectedError={() => {}} />
    </main>
  </NavigationProvider></WorkspaceSlugProvider>
</I18nProvider></QueryClientProvider>);
