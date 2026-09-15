import { cn } from "@multica/ui/lib/utils";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import { useT } from "@multica/views/i18n";
import { useNavigation } from "@multica/views/navigation";

/**
 * The running desktop build, parked at the left edge of the sidebar footer
 * opposite HelpLauncher. Desktop-only: the version comes from the preload
 * bridge, and web has no equivalent (its footer keeps the slot empty).
 *
 * A packaged build reports the release tag; a dev build reports the full
 * `git describe` string, which is far wider than the footer, so the label
 * truncates and the tooltip carries the whole thing.
 */
export function SidebarVersion() {
  const { t } = useT("layout");
  const { push } = useNavigation();
  // Nullable read, like HelpLauncher next to it: the footer only mounts under a
  // resolved workspace, but nothing above the sidebar is an error boundary, and
  // a version row is not worth risking a blank window over.
  const workspaceSlug = useWorkspaceSlug();

  // `fetchAppInfo` in the preload answers the literal "unknown" when its
  // synchronous IPC fails. "vunknown" reads as a broken build, so drop the row.
  const reported = window.desktopAPI.appInfo.version;
  const version = reported && reported !== "unknown" ? reported : null;
  if (!version) return null;

  const label = `v${version}`;
  const textClassName = "min-w-0 truncate text-caption text-muted-foreground";

  if (!workspaceSlug) return <span className={textClassName}>{label}</span>;

  return (
    <button
      type="button"
      title={t(($) => $.desktop.version.tooltip, { version })}
      onClick={() =>
        push(`${paths.workspace(workspaceSlug).settings()}?tab=updates`)
      }
      className={cn(
        textClassName,
        "cursor-pointer rounded px-1.5 py-0.5 transition-colors hover:bg-accent hover:text-foreground",
      )}
    >
      {label}
    </button>
  );
}
