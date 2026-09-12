"use client";

import { useId } from "react";
import { ChevronRight, Clock } from "lucide-react";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { browserTimezone } from "../../common/timezone-select";
import { useT } from "../../i18n";
import { useNavigation } from "../../navigation";
import { autopilotTemplateHref } from "../template-create-defaults";
import { useAutopilotTemplates } from "../use-autopilot-templates";
import { parseCron } from "./schedule-editor/cron-mapping";
import { useDescribeSchedule } from "./schedule-editor/describe";

export function AutopilotTemplateCatalog() {
  const { t } = useT("autopilots");
  const headingId = useId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const describe = useDescribeSchedule();
  const { data: templates = [], isLoading, isError, refetch } = useAutopilotTemplates();

  return (
    <section aria-labelledby={headingId} className="shrink-0 space-y-3 px-4 py-4">
      <div>
        <h2 id={headingId} className="text-body font-medium">{t(($) => $.catalog.title)}</h2>
        <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.catalog.description)}</p>
      </div>
      {isLoading ? (
        <Skeleton className="h-20 w-full" />
      ) : isError ? (
        <div className="flex items-center gap-3">
          <p role="alert" className="text-caption text-muted-foreground">{t(($) => $.template_picker.load_failed)}</p>
          <Button size="sm" variant="ghost" onClick={() => void refetch()}>{t(($) => $.page.retry)}</Button>
        </div>
      ) : templates.length === 0 ? (
        <p className="text-caption text-muted-foreground">{t(($) => $.template_picker.empty)}</p>
      ) : (
        <div className="grid max-h-64 gap-x-4 gap-y-1 overflow-y-auto sm:grid-cols-2 xl:grid-cols-4">
          {templates.map((template) => (
            <button key={template.key} type="button" onClick={() => navigation.push(autopilotTemplateHref(paths.newAutopilotTemplate(), {}, template.key))}
              className="group flex min-w-0 flex-col rounded-md px-2 py-2.5 text-left transition-colors hover:bg-accent/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
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
          ))}
        </div>
      )}
    </section>
  );
}
