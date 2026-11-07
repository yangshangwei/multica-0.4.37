import React from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { I18nProvider } from '@multica/core/i18n/react';
import { WorkspaceSlugProvider } from '@multica/core/paths';
import { ApiClient } from '@multica/core/api/client';
import { setApiInstance } from '@multica/core/api';
import { RESOURCES } from '@multica/views/locales';
import { NavigationProvider } from '@multica/views/navigation';
import { DocsPage } from '@multica/views/docs';
import '@fontsource-variable/inter';
import '@fontsource-variable/geist-mono';
import './globals.css';

const params = new URLSearchParams(location.search);
const slug = params.get('slug') || 'concepts';
document.documentElement.classList.toggle('dark', params.get('theme') === 'dark');
setApiInstance(new ApiClient(location.origin));
const client = new QueryClient(); client.getQueryCache().subscribe(event => { if (event.query.state.error) console.error(event.query.state.error); });
const nav = { pathname: '/test/docs/' + slug, searchParams: new URLSearchParams(), hash: '',
  push: (path: string) => { location.href = '/docs-visual-check.html?slug=' + encodeURIComponent(path.replace('/test/docs/', '')); },
  replace: () => {}, back: () => {}, getShareableUrl: (path: string) => location.origin + path };
const bundleRoot = '/@fs/Volumes/artisan/code/2026/multica-0.4.37/server/internal/docs/content/';
const pageUrl = params.has('baseline') ? '/@fs/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-07-docs-visual-guides/research/baseline-page.json' : bundleRoot + 'pages/' + slug + '.json';
const [manifest, pageData] = await Promise.all([fetch(bundleRoot + 'manifest.json').then(r=>r.json()), fetch(pageUrl).then(r=>r.json())]);
client.setQueryData(['docs', 'manifest'], manifest);
if (params.has('broken')) pageData.body = '## 图示错误回退\n\n```mermaid\nthis is invalid mermaid\n```\n\n后续正文仍然可读。';
client.setQueryData(['docs', 'page', slug], pageData);
createRoot(document.getElementById('root')!).render(
 <I18nProvider locale="zh-Hans" resources={RESOURCES}>
  <QueryClientProvider client={client}><WorkspaceSlugProvider slug="test"><NavigationProvider value={nav}>
   <div className="flex h-screen min-w-0 bg-background text-foreground"><DocsPage slug={slug}/></div>
  </NavigationProvider></WorkspaceSlugProvider></QueryClientProvider>
 </I18nProvider>
);
