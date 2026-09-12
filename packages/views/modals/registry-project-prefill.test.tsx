import { afterEach, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { useModalStore } from "@multica/core/modals";

vi.mock("./create-project", () => ({ CreateProjectModal: ({ data }: { data?: object }) => <output aria-label="Project prefill">{JSON.stringify(data)}</output> }));
vi.mock("./create-issue-dialog", () => ({ CreateIssueDialog: () => null }));
vi.mock("./create-squad", () => ({ CreateSquadModal: () => null }));
vi.mock("./staff-squad-template", () => ({ StaffSquadTemplateModal: () => null }));
vi.mock("./feedback", () => ({ FeedbackModal: () => null }));
vi.mock("./set-parent-issue", () => ({ SetParentIssueModal: () => null }));
vi.mock("./add-child-issue", () => ({ AddChildIssueModal: () => null }));
vi.mock("./delete-issue-confirm", () => ({ DeleteIssueConfirmModal: () => null }));
vi.mock("./run-confirm", () => ({ RunConfirmModal: () => null }));
vi.mock("./issue-limit-upgrade-dialog", () => ({ IssueLimitUpgradeDialog: () => null }));

import { ModalRegistry } from "./registry";

afterEach(() => act(() => useModalStore.getState().close()));

it("forwards catalog data into project creation", () => {
  useModalStore.getState().open("create-project", { squad_template_key: "feature-delivery" });
  render(<ModalRegistry />);
  expect(screen.getByLabelText("Project prefill")).toHaveTextContent('"squad_template_key":"feature-delivery"');
});
