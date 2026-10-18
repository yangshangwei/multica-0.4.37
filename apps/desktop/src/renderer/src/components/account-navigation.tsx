import { accountPath, paths } from "@multica/core/paths";
import { useWorkspaceList } from "@multica/core/workspace";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink, useNavigation } from "@multica/views/navigation";
import { useT } from "@multica/views/i18n";
import { useTabStore } from "../stores/tab-store";

/** Workspace selection stays optional in the authenticated account shell. */
export function DesktopAccountNavigation({ onRetry }: { onRetry: () => void }) {
  const { t } = useT("collaboration");
  const { t: layout } = useT("layout");
  const list = useWorkspaceList();
  const nav = useNavigation();
  const activeScope = useTabStore((state) => state.activeScope);
  return <nav aria-label={t(($) => $.page.title)} className="flex min-w-0 flex-wrap items-center gap-3 px-4 py-2 text-body" style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}>
    <AppLink href={accountPath()} aria-current={activeScope === "account" ? "page" : undefined} className="rounded-md px-3 py-2 hover:bg-accent aria-[current=page]:font-semibold focus-visible:ring-2 focus-visible:ring-ring">{t(($) => $.page.title)}</AppLink>
    <select aria-label={layout(($) => $.sidebar.workspaces_label)} value="" onChange={(event) => { if (event.target.value) nav.push(paths.workspace(event.target.value).issues()); }} className="max-w-64 rounded-md bg-background px-3 py-2 focus-visible:ring-2 focus-visible:ring-ring">
      <option value="">{layout(($) => $.sidebar.workspaces_label)}</option>
      {list.workspaces.map((workspace) => <option key={workspace.id} value={workspace.slug}>{workspace.name}</option>)}
    </select>
    {list.unavailable && <Button variant="ghost" disabled={list.isFetching} onClick={onRetry}>{t(($) => $.page.refresh)}</Button>}
    <Button variant="ghost" onClick={() => nav.push("/workspaces/new")}>{layout(($) => $.sidebar.create_workspace)}</Button>
  </nav>;
}
