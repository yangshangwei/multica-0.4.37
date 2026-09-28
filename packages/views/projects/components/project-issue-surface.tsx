"use client";

import { useCallback, useState } from "react";
import { Link2, ListTodo, Plus } from "lucide-react";
import { toast } from "sonner";
import type { Issue, Project } from "@multica/core/types";
import type { IssueScope } from "@multica/core/issues/surface/scope";
import { useUpdateIssue } from "@multica/core/issues/mutations";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { IssueSurface } from "../../issues/surface/issue-surface";
import type { IssueCreateDefaults } from "../../issues/surface/types";
import { IssuePickerModal } from "../../modals/issue-picker-modal";

const NO_EXCLUDED_ISSUES: string[] = [];

export function ProjectIssueSurface({ project, scope, fallbackCreateDefaults }: {
  project: Project;
  scope: Extract<IssueScope, { type: "project" }>;
  fallbackCreateDefaults?: IssueCreateDefaults;
}) {
  const { t } = useT("projects");
  const [linkOpen, setLinkOpen] = useState(false);
  const { mutateAsync: updateIssue } = useUpdateIssue();
  const isEmptyProject = project.issue_count === 0;
  const canLinkIssue = useCallback((issue: Issue) =>
    issue.workspace_id === project.workspace_id && issue.project_id === null,
  [project.workspace_id]);
  const linkIssue = async (issue: Issue) => {
    if (!canLinkIssue(issue)) {
      toast.error(t(($) => $.detail.link_failed));
      return false;
    }
    try {
      await updateIssue({ id: issue.id, project_id: project.id });
      toast.success(t(($) => $.detail.link_success, { identifier: issue.identifier }));
      return true;
    } catch {
      toast.error(t(($) => $.detail.link_failed));
      return false;
    }
  };

  return <>
    <IssueSurface
      scope={scope}
      modes={["board", "list", "table", "swimlane", "gantt"]}
      fallbackCreateDefaults={fallbackCreateDefaults}
      isScopeEmpty={isEmptyProject}
      headerActions={({ controller }) => !isEmptyProject && <>
        <Button
          size="sm"
          variant="ghost"
          aria-label={t(($) => $.detail.link_existing)}
          title={t(($) => $.detail.link_existing)}
          onClick={() => setLinkOpen(true)}
        >
          <Link2 className="size-3.5" aria-hidden="true" />
          <span className="hidden lg:inline">{t(($) => $.detail.link_existing)}</span>
        </Button>
        <Button size="sm" onClick={() => controller.openCreateIssue()}>
          <Plus className="size-3.5" aria-hidden="true" />
          {t(($) => $.detail.empty_issues_new_button)}
        </Button>
      </>}
      renderEmpty={({ controller }) => <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 overflow-y-auto px-6 py-8 text-center">
        <ListTodo className="size-10 shrink-0 text-faint-foreground" aria-hidden="true" />
        <h2 className="text-title font-medium">
          {isEmptyProject ? t(($) => $.detail.first_issue_title) : t(($) => $.detail.empty_view_title)}
        </h2>
        <p className="max-w-prose text-body text-muted-foreground">
          {isEmptyProject ? t(($) => $.detail.first_issue_hint) : t(($) => $.detail.empty_view_hint)}
        </p>
        {isEmptyProject && <div className="mt-2 flex flex-col items-center gap-2">
          <Button onClick={() => controller.openCreateIssue()}>
            <Plus className="size-4" aria-hidden="true" />
            {t(($) => $.detail.empty_issues_new_button)}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setLinkOpen(true)}>
            <Link2 className="size-3.5" aria-hidden="true" />
            {t(($) => $.detail.link_existing)}
          </Button>
        </div>}
      </div>}
    />
    <IssuePickerModal
      open={linkOpen}
      onOpenChange={setLinkOpen}
      title={t(($) => $.detail.link_existing_title)}
      description={t(($) => $.detail.link_existing_hint)}
      excludeIds={NO_EXCLUDED_ISSUES}
      filterIssue={canLinkIssue}
      onSelect={linkIssue}
    />
  </>;
}
