"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  iterationCapabilitiesOptions,
  iterationChoicesOptions,
  iterationIssuesOptions,
} from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { IterationSelect } from "./iteration-assignment";
import { AppLink } from "../navigation";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
export function ProjectIterations({
  wsId,
  projectId,
}: {
  wsId: string;
  projectId: string;
}) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  const [id, setId] = useState("");
  const [cursor, setCursor] = useState("");
  const capability = useQuery(iterationCapabilitiesOptions(wsId));
  const list = useQuery({
    ...iterationChoicesOptions(wsId),
    enabled: capability.data?.enabled === true,
  });
  const issues = useQuery({
    ...iterationIssuesOptions(wsId, id, {
      project_id: projectId,
      ...(cursor ? { cursor } : {}),
    }),
    enabled: capability.data?.enabled === true && !!id,
  });
  if (capability.data?.enabled !== true) return null;
  return (
    <details className="px-4 py-3">
      <summary>{t(($) => $.iterations.title)}</summary>
      <div className="space-y-3 pt-3">
        <IterationSelect
          value={id}
          onChange={(value) => {
            setId(value);
            setCursor("");
          }}
          items={list.data ?? []}
        />
        {id && (
          <AppLink href={paths.iterationDetail(id)}>
            {t(($) => $.iterations.scope)}
          </AppLink>
        )}
        {issues.error && <p role="alert">{t(($) => $.iterations.error)}</p>}
        <ul>
          {!issues.error &&
            issues.data?.items.map((issue) => (
              <li key={issue.issue_id}>
                <AppLink
                  href={paths.issueDetail(issue.issue_id)}
                  className="break-words"
                >
                  {issue.identifier} · {issue.title}
                </AppLink>
              </li>
            ))}
        </ul>
        {issues.data?.next_cursor && (
          <Button onClick={() => setCursor(issues.data!.next_cursor!)}>
            {t(($) => $.iterations.next)}
          </Button>
        )}
      </div>
    </details>
  );
}
