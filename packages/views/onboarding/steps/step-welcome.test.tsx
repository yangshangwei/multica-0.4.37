import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithI18n } from "../../test/i18n";
import { StepWelcome } from "./step-welcome";

// Keep the illustration's human approval and review gates localized.
describe("StepWelcome status labels", () => {
  it("renders the handoff and approval stages in English", () => {
    renderWithI18n(<StepWelcome onNext={vi.fn()} />);

    for (const label of ["Awaiting requirements confirmation", "Design complete", "Awaiting review", "Review failed · Merge blocked", "Awaiting your release approval"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(screen.getByText("Requirements Agent")).toBeInTheDocument();
    expect(screen.getByText("QA Agent")).toBeInTheDocument();
  });

  it("localizes the gates and human decisions in Chinese", () => {
    renderWithI18n(<StepWelcome onNext={vi.fn()} />, { locale: "zh-Hans" });

    for (const label of ["等待需求确认", "设计已完成", "等待审核", "审核未通过 · 合并已阻止", "等待你批准发布"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(screen.getByText("需求智能体")).toBeInTheDocument();
    expect(screen.getByText("测试智能体")).toBeInTheDocument();
    expect(screen.queryByText("Awaiting your release approval")).not.toBeInTheDocument();
  });
});
