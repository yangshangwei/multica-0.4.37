"use client";

import { AdminCollaborationPage } from "@multica/views/admin";
import { useNavigation } from "@multica/views/navigation";

export function AdminCollaborationRoute() {
  const navigation = useNavigation();
  return <AdminCollaborationPage issueId={navigation.searchParams.get("issue")}
    hrefForIssue={(id) => id ? `/admin/collaboration?${new URLSearchParams({ issue: id })}` : "/admin/collaboration"} />;
}
