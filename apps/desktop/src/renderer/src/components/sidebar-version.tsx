import { cn } from "@multica/ui/lib/utils";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import { useT } from "@multica/views/i18n";
import { useNavigation } from "@multica/views/navigation";

/**
 * The running desktop build, rendered as a row inside the sidebar's help menu
 * directly above the server version. Desktop-only: the version comes from the
 * preload bridge, and web has no equivalent (its slot stays empty).
 *
 * Deliberately a plain button rather than a DropdownMenuItem: the Base UI menu
 * parts require ancestors that live in HelpLauncher, invisible from here, and a
 * version row that got those wrong is exactly how MUL-4819 black-screened the
 * app. The styling matches the server-version label next to it.
 *
 * A packaged build reports the release tag; a dev build reports the full
 * `git describe` string, which is wider than the menu, so the label wraps and
 * the tooltip carries the whole thing plus the call to action.
 */
/**
 * The validated running build, or null when the preload bridge has nothing
 * usable: `fetchAppInfo` answers the literal "unknown" when its synchronous
 * IPC fails, and an empty string if main ever answers with one. Neither is
 * worth a menu row.
 *
 * Exported because the layout gates the help menu's version slot on it —
 * HelpLauncher cannot tell an opaque element that renders null from a real
 * row, so without this check a build whose version failed to load would hang
 * the version block's separator over nothing.
 */
export function desktopAppVersion(): string | null {
  // The bridge is guaranteed in the packaged renderer, but test environments
  // and any non-preload host may lack it; degrade to "no version row" rather
  // than crash the layout.
  const reported = window.desktopAPI?.appInfo?.version;
  return reported && reported !== "unknown" ? reported : null;
}

export function SidebarVersion() {
  const { t } = useT("layout");
  const { push } = useNavigation();
  // Nullable read, like HelpLauncher around it: the footer only mounts under a
  // resolved workspace, but nothing above the sidebar is an error boundary, and
  // a version row is not worth risking a blank window over.
  const workspaceSlug = useWorkspaceSlug();

  const version = desktopAppVersion();
  if (!version) return null;

  const label = t(($) => $.desktop.version.label, { version });
  // Same padding and type scale as the server-version label below it.
  const rowClassName = "min-w-0 px-1.5 py-1 text-caption break-words text-muted-foreground";

  if (!workspaceSlug) return <span className={rowClassName}>{label}</span>;

  return (
    <button
      type="button"
      title={t(($) => $.desktop.version.tooltip, { version })}
      onClick={() =>
        push(`${paths.workspace(workspaceSlug).settings()}?tab=updates`)
      }
      className={cn(
        rowClassName,
        "w-full cursor-pointer rounded-md text-left transition-colors hover:bg-accent hover:text-foreground",
      )}
    >
      {label}
    </button>
  );
}
