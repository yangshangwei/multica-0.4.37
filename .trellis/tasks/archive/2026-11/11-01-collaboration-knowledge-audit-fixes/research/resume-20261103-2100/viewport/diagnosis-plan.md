# Viewport repair boundary

The current 17-case directed run has one failure: a preparation section reaches an IntersectionObserver ratio of 0.9991987347602844 after native `scrollIntoView({block: "start"})`. The source assertion still requires complete visibility. The candidate product is unchanged; this is a screenshot positioning investigation confined to the Web preparation E2E file.

Hypothesis to measure before fixing: start alignment plus fractional layout/integer scroll offsets leaves the section slightly outside a clipping ancestor. Capture the section rectangle, clipping ancestor rectangles/scroll offsets and actual IntersectionObserver intersection rectangle on the unchanged start-aligned behavior. Preserve the failing ratio, timeout and screenshot geometry assertions. If proved, use native centering to leave genuine space inside the clipping viewport rather than relax the assertions.

Only the assigned Web preparation spec may change. Diagnostic and new numbered runs are task-owned; all prior failures and the other six reviewed files stay unchanged. Product files, runtime configuration, DB identity, model gates and original test thresholds are immutable. Reuse the parent's explicitly authorized isolated directed environment, then run the complete 17-case directed selection once the specific case passes.
