"use client";

import {
  BookOpen,
  CircleHelp,
  FileText,
  MessageCircle,
} from "lucide-react";
import type { ReactNode } from "react";
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
import { useT } from "../i18n";

export function HelpLauncher({ versionSlot }: { versionSlot?: ReactNode } = {}) {
  const { t } = useT("layout");
  const serverVersion = useConfigStore((state) => state.serverVersion);
  // Nullable on purpose: this menu lives in the dashboard sidebar, which is
  // always workspace-scoped, but reading the slug defensively keeps the Help
  // menu from being the thing that throws if it is ever mounted elsewhere.
  const workspaceSlug = useWorkspaceSlug();
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
        {/* Reference pages and feedback are served by this deployment. */}
        {workspaceSlug ? (
          <>
          <DropdownMenuItem
            render={
              <AppLink href={paths.workspace(workspaceSlug).docs()} />
            }
          >
            <BookOpen className="h-3.5 w-3.5" />
            {t(($) => $.help.docs)}
          </DropdownMenuItem>
          <DropdownMenuItem
            render={<AppLink href={paths.workspace(workspaceSlug).changelog()} />}
          >
            <FileText aria-hidden="true" className="h-3.5 w-3.5" />
            {t(($) => $.help.changelog)}
          </DropdownMenuItem>
          </>
        ) : null}
        <DropdownMenuItem
          onClick={() => useModalStore.getState().open("feedback")}
        >
          <MessageCircle className="h-3.5 w-3.5" />
          {t(($) => $.help.feedback)}
        </DropdownMenuItem>
        {(versionSlot || serverVersion) && <DropdownMenuSeparator />}
        {/* Platform-supplied build info (desktop passes its app version). The
            slot must render plain elements, NOT Base UI menu parts: their
            required ancestors live in this file, invisible from the app that
            fills the slot — which is how MUL-4819 shipped a version row that
            crashed the app on open. See the DropdownMenuGroup note below. */}
        {versionSlot}
        {serverVersion && (
          /* DropdownMenuLabel renders Base UI's Menu.GroupLabel, which reads a
             Menu.Group context and throws if it has no Group ancestor. It must
             always be wrapped in a DropdownMenuGroup — without it the Help menu
             crashes the whole app on open (no error boundary sits above the
             sidebar). */
          <DropdownMenuGroup>
            <DropdownMenuLabel className="font-normal break-words">
              {t(($) => $.help.server_version, { version: serverVersion })}
            </DropdownMenuLabel>
          </DropdownMenuGroup>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
