// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { ProjectDescriptionSave } from "./description-save";
import { p1Project } from "./test-fixtures/p1";
import type { Project } from "../types/project";
it("serializes queued descriptions with the last acknowledged revision", async () => {
  let resolve!: (project: Project) => void;
  const save = vi.fn().mockImplementationOnce(() => new Promise<Project>((r) => { resolve = r; }))
    .mockResolvedValue({ ...p1Project, description: "second", description_revision: 4 });
  const controller = new ProjectDescriptionSave(p1Project, save, vi.fn());
  controller.enqueue("first"); controller.enqueue("second");
  expect(save).toHaveBeenCalledTimes(1);
  resolve({ ...p1Project, status: "in_progress", priority: "medium", lead_type: "member", description: "first", description_revision: 3 });
  await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(2));
  expect(save.mock.calls[1]).toEqual(["second", 3]);
});
describe("conflict recovery", () => {
  it("does not automatically overwrite a conflicting server revision", async () => {
    const changed = vi.fn(); const save = vi.fn().mockRejectedValue(new Error("conflict"));
    const controller = new ProjectDescriptionSave(p1Project, save, changed);
    controller.enqueue("first"); await vi.waitFor(() => expect(changed).toHaveBeenCalled());
    controller.enqueue("second"); expect(save).toHaveBeenCalledTimes(1);
    controller.adopt("server", 8); controller.enqueue("merged");
    await vi.waitFor(() => expect(save).toHaveBeenCalledTimes(2));
    expect(save.mock.calls[1]).toEqual(["merged", 8]);
  });
});
