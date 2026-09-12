"use client";

import { ChevronRight, Clock } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import { BuiltinTemplateCatalog } from "../../common/builtin-template-catalog";
import { browserTimezone } from "../../common/timezone-select";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { autopilotTemplateHref } from "../template-create-defaults";
import { useAutopilotTemplates } from "../use-autopilot-templates";
import { parseCron } from "./schedule-editor/cron-mapping";
import { useDescribeSchedule } from "./schedule-editor/describe";

export function AutopilotTemplateCatalog() {
  const { t } = useT("autopilots");
  const { t: commonT } = useT("common");
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const describe = useDescribeSchedule();
  const { data: templates = [], isLoading, isError, refetch } = useAutopilotTemplates();

  return (
    <BuiltinTemplateCatalog
      copy={{
        title: t(($) => $.catalog.title), description: t(($) => $.catalog.description),
        loading: commonT(($) => $.loading), error: t(($) => $.template_picker.load_failed),
        empty: t(($) => $.template_picker.empty), retry: t(($) => $.page.retry),
      }}
      count={templates.length} loading={isLoading} failed={isError} empty={templates.length === 0}
      onRetry={() => void refetch()}
      listClassName="max-h-64 gap-x-4 gap-y-1 @xl:grid-cols-2 @3xl:grid-cols-4"
    >
      {templates.map((template) => (
        <li key={template.key} className="min-w-0">
          <button type="button" onClick={() => navigation.push(autopilotTemplateHref(paths.newAutopilotTemplate(), {}, template.key))}
            className="group flex w-full min-w-0 flex-col rounded-md px-2 py-2.5 text-left transition-colors hover:bg-accent/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
            <span className="flex w-full items-center gap-2 text-body font-medium">
              <span className="min-w-0 flex-1 truncate">{template.title}</span>
              <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
            </span>
            <span className="mt-1 line-clamp-2 text-caption text-muted-foreground">{template.description}</span>
            <span className="mt-2 flex items-center gap-1.5 text-micro text-muted-foreground">
              <Clock className="size-3 shrink-0" aria-hidden="true" />
              <span>{describe(parseCron(template.cron_expression, browserTimezone())) ?? template.cron_expression}</span>
            </span>
          </button>
        </li>
      ))}
    </BuiltinTemplateCatalog>
  );
}
