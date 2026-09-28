"use client";

import { useT } from "../i18n";
import { PageHeader } from "../layout/page-header";
import { McpTab } from "../settings/components/mcp-tab";

export function McpPage() {
  const { t } = useT("layout");

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <PageHeader>
        <h1 className="text-body font-medium">{t(($) => $.nav.mcp)}</h1>
      </PageHeader>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-3xl p-4 sm:p-6 md:p-8">
          <McpTab />
        </div>
      </div>
    </div>
  );
}
