import { useWorkspaceList } from "@multica/core/workspace";
import { Button } from "@multica/ui/components/ui/button";
import { GlobalScopeNavigation } from "@multica/views/layout";
import { useNavigation } from "@multica/views/navigation";
import { useT } from "@multica/views/i18n";
import { Plus, RotateCw } from "lucide-react";
import { resumeDesktopScope } from "../platform/navigation";
import { useTabStore } from "../stores/tab-store";

/** This shell row survives account entry and workspace-sidebar collapse. */
export function DesktopAccountNavigation({ onRetry }: { onRetry: () => void }) {
  const { t } = useT("collaboration");
  const { t: layout } = useT("layout");
  const list = useWorkspaceList();
  const nav = useNavigation();
  const activeScope = useTabStore((state) => state.activeScope);
  return <div data-slot="global-scope-bar" className="flex h-12 min-w-0 shrink-0 items-center bg-app-shell pr-3 pl-20" style={{ WebkitAppRegion: "drag" } as React.CSSProperties}>
    <div className="min-w-0" style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}>
      <GlobalScopeNavigation
        activeScope={activeScope}
        onWorkspace={() => {
          const slugs = list.ready ? list.workspaces.map((workspace) => workspace.slug) : undefined;
          if (!resumeDesktopScope("workspace", slugs)) onRetry();
        }}
        onCollaboration={() => { resumeDesktopScope("account"); }}
      >
        {list.unavailable && <Button variant="ghost" size="icon" aria-label={t(($) => $.page.refresh)} title={t(($) => $.page.refresh)} disabled={list.isFetching} onClick={onRetry}><RotateCw className="size-4" /></Button>}
        <Button variant="ghost" size="icon" aria-label={layout(($) => $.sidebar.create_workspace)} title={layout(($) => $.sidebar.create_workspace)} onClick={() => nav.push("/workspaces/new")}><Plus className="size-4" /></Button>
      </GlobalScopeNavigation>
    </div>
  </div>;
}
