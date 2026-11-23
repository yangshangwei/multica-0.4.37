"use client";

import { useEffect } from "react";
import { accountPath, parseAccountPath } from "@multica/core/paths";
import { AccountWorkbenchPage } from "@multica/views/collaboration";
import { currentPath, useNavigation } from "@multica/views/navigation";
import { useT } from "@multica/views/i18n";
import { formatDocumentTitle, SITE_TITLE } from "./document-title";

export function AccountWorkbenchRoute() {
  const { t } = useT("collaboration");
  const navigation = useNavigation();
  const path = currentPath(navigation);
  const destination = parseAccountPath(path);
  const valid = destination !== null;
  const title = formatDocumentTitle(t(($) => $.page.title));
  useEffect(() => {
    document.title = title;
    return () => { if (document.title === title) document.title = SITE_TITLE; };
  }, [title, path]);
  useEffect(() => {
    if (!valid) navigation.replace(accountPath());
  }, [valid, navigation]);
  return destination ? <AccountWorkbenchPage destination={destination} administrationHref="/admin/collaboration" /> : null;
}
