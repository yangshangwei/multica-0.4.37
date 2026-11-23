import { expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { parseCollaborationIssue, type CollaborationExecutionRun } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationRunDisclosure } from "./run-records";

vi.mock("../navigation", () => ({ useNavigation: () => ({}), AppLink: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} /> }));
const issue = parseCollaborationIssue(collaborationExecutionIssueFixture)!;
const run = issue.execution!.runs[0]!;
const group = issue.execution!.groups.find(value => value.id === run.group_id)!;
const scope = { server: "https://memory.example", apiScope: "memory", userId: issue.creator.user_id };

function content(memory: CollaborationExecutionRun["memory"]) {
  return <ul><CollaborationRunDisclosure scope={scope} issue={issue} run={{ ...run, memory }} group={group} /></ul>;
}

it("links recorded base and written versions and distinguishes empty from unrecorded bases", () => {
  const view = renderWithI18n(content({ support: "supported", base_version: "2", written_version: "3" }));
  expect(screen.getByRole("link", { name: "Started from v2" })).toHaveAttribute("href", `/account?view=participating&issue=${issue.id}#memory-v2`);
  expect(screen.getByRole("link", { name: "Wrote v3" })).toHaveAttribute("href", `/account?view=participating&issue=${issue.id}#memory-v3`);
  view.rerender(content({ support: "supported", base_version: "0", written_version: null }));
  expect(screen.getByText(/Started from an empty document/)).toBeVisible();
  expect(screen.queryByRole("link")).not.toBeInTheDocument();
  view.rerender(content({ support: "supported", base_version: null, written_version: null }));
  expect(screen.queryByText(/Started from an empty document/)).not.toBeInTheDocument();
  expect(screen.getByText(/Not recorded/)).toBeVisible();
  view.rerender(content({ support: "unsupported", base_version: null, written_version: null }));
  expect(screen.getByText("This runtime does not support shared context")).toBeVisible();
  expect(screen.queryByText(/Shared context not updated/)).not.toBeInTheDocument();
});
