import { useEffect } from "react";
import { useLocation } from "react-router-dom";
import { accountPath, parseAccountPath } from "@multica/core/paths";
import { setCurrentWorkspace } from "@multica/core/platform";
import { useAuthStore } from "@multica/core/auth";
import { AccountWorkbenchPage } from "@multica/views/collaboration";
import { useT } from "@multica/views/i18n";

/** Authenticated account scope never synthesizes workspace membership. */
export function DesktopAccountWorkbenchRoute() {
  const location = useLocation();
  const user = useAuthStore((state) => state.user);
  const { t } = useT("collaboration");
  const title = t(($) => $.page.title);
  useEffect(() => { document.title = title; }, [title]);
  setCurrentWorkspace(null, null);
  const destination = parseAccountPath(`${location.pathname}${location.search}${location.hash}`);
  if (!user || user.requires_account_setup || user.requires_password_change || !destination) return null;
  return <AccountWorkbenchPage destination={destination} administrationHref={accountPath({ administration: true })} />;
}
