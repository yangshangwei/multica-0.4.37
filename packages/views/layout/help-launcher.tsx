"use client";

import {
  ArrowUpRight,
  BookOpen,
  CircleHelp,
  Download,
  History,
  MessageCircle,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { useModalStore } from "@multica/core/modals";
import { useConfigStore } from "@multica/core/config";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { isDesktopShell } from "../platform/local-directory";
import { useT } from "../i18n";

const CHANGELOG_URL = "https://multica.ai/changelog";
// Absolute, including on self-hosted deployments: the installers we ship are
// the same binaries either way, and the desktop client can point at a
// self-hosted backend once installed. A self-host-relative /download would
// only serve a copy of this page that still has to reach our release assets.
const DOWNLOAD_URL = "https://multica.ai/download";

export function HelpLauncher() {
  const { t } = useT("layout");
  const serverVersion = useConfigStore((state) => state.serverVersion);
  // Nullable on purpose: this menu lives in the dashboard sidebar, which is
  // always workspace-scoped, but reading the slug defensively keeps the Help
  // menu from being the thing that throws if it is ever mounted elsewhere.
  const workspaceSlug = useWorkspaceSlug();
  // Web-only: offering "download the desktop app" inside the desktop app is
  // nonsense, and this sidebar is shared — apps/desktop renders the same
  // AppSidebar as the web dashboard, so the entry has to be gated here.
  //
  // No `mounted` deferral (cf. browser-notification-setting.tsx): the desktop
  // renderer is a locally-bundled SPA with no SSR pass, and on web
  // `isDesktopShell()` is false both on the server and after hydration. The
  // markup matches either way, so the link can ship in the SSR payload instead
  // of popping in a frame late.
  const desktop = isDesktopShell();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={t(($) => $.help.trigger)}
        title={t(($) => $.help.trigger)}
        className="inline-flex size-7 items-center justify-center rounded-full text-muted-foreground transition-colors cursor-pointer hover:bg-accent hover:text-foreground data-popup-open:bg-accent data-popup-open:text-foreground"
      >
        <CircleHelp className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        side="top"
        sideOffset={8}
        className="min-w-40 max-w-56"
      >
        {!desktop && (
          <>
            <DropdownMenuItem
              render={
                <a
                  href={DOWNLOAD_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                />
              }
            >
              <Download className="h-3.5 w-3.5" />
              {t(($) => $.help.download_desktop)}
              <ArrowUpRight className="size-3 translate-y-px text-faint-foreground" />
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </>
        )}
        {/* Documentation is served by the deployment this client is connected
            to, so this is an in-app destination rather than a link off to
            multica.ai — an intranet install cannot reach the public site at
            all. No ArrowUpRight for the same reason: nothing leaves the app.
            "Change log" and "Desktop app" below stay external, because the
            release assets they point at genuinely are not in this
            deployment. */}
        {workspaceSlug ? (
          <DropdownMenuItem
            render={
              <AppLink href={paths.workspace(workspaceSlug).docs()} />
            }
          >
            <BookOpen className="h-3.5 w-3.5" />
            {t(($) => $.help.docs)}
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem
          render={
            <a
              href={CHANGELOG_URL}
              target="_blank"
              rel="noopener noreferrer"
            />
          }
        >
          <History className="h-3.5 w-3.5" />
          {t(($) => $.help.changelog)}
          <ArrowUpRight className="size-3 translate-y-px text-faint-foreground" />
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => useModalStore.getState().open("feedback")}
        >
          <MessageCircle className="h-3.5 w-3.5" />
          {t(($) => $.help.feedback)}
        </DropdownMenuItem>
        {serverVersion && (
          <>
            <DropdownMenuSeparator />
            {/* DropdownMenuLabel renders Base UI's Menu.GroupLabel, which reads
                a Menu.Group context and throws if it has no Group ancestor. It
                must always be wrapped in a DropdownMenuGroup — without it the
                Help menu crashes the whole app on open (no error boundary sits
                above the sidebar). */}
            <DropdownMenuGroup>
              <DropdownMenuLabel className="font-normal break-words">
                {t(($) => $.help.server_version, { version: serverVersion })}
              </DropdownMenuLabel>
            </DropdownMenuGroup>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
