# Overview coverage sources

- Help: packages/views/layout/help-launcher.tsx; workspace docs route; server embeds Chinese bundle. Desktop versionSlot and config serverVersion differ.
- Primary navigation: packages/views/layout/app-sidebar.tsx personalNav/workNav/aiTeamNav/resourceNav; settings-nav.ts groups account/workspace/issue/connections/desktop.
- Global collaboration: packages/views/layout/global-scope-navigation.tsx and collaboration/account-workbench-page.tsx.
- Usage: packages/views/dashboard/components/dashboard-page.tsx, dashboard-filters.tsx, dashboard-shared.tsx and packages/views/locales/zh-Hans/usage.json; server/internal/handler/dashboard.go.
- Settings: packages/views/settings/components/settings-nav.ts and preferences-tab.tsx; billing and plugins require feature flags. Do not imply these are default enabled intranet capabilities.

Guide overview should link every primary navigation function, summarize settings locations, distinguish source code coverage from deployed availability, and keep detailed behavior on its canonical guide.
