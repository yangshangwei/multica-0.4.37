// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseCollaborationStageView } from "./stage-schema";
import { collaborationStageScopeSelection } from "./stage-presentation";
import { stageChildFixture, stageViewManagedFixture, stageDigests, stageIds } from "./stage-test-fixtures";
const view = (child: unknown) => ({ ...stageViewManagedFixture, children: { items: [child], truncated: false } });
describe("section13.14 child digest confidentiality", () => {
  it("accepts only null for a source-unreadable child and never selects it for fresh A5", () => {
    const hidden = parseCollaborationStageView(view({ ...stageChildFixture, can_open: false, content_digest: null }))!;
    expect(hidden?.children.items[0]?.content_digest).toBeNull();
    const selection = collaborationStageScopeSelection(hidden);
    expect(selection.candidates).toEqual([]);expect(selection.entries([stageIds.child1], { [stageIds.child1]: null })).toBeNull();
    expect(selection.entries([], {})).toEqual([]);
  });
  it.each([
    { can_open: false, content_digest: stageDigests.child1 }, { can_open: true, content_digest: null },
    { can_open: false, content_digest: undefined }, { can_open: true, content_digest: "" },
  ])("rejects a mismatched or absent digest %j", (change) => { expect(parseCollaborationStageView(view({ ...stageChildFixture, ...change }))).toBeNull(); });
});
