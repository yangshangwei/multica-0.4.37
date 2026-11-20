# Break-loop retrospective: native close control

## Root cause category
Cross-layer contract and test coverage gap. A React control is visually above an Electron native drag region, but visual stacking does not guarantee OS pointer delivery. The top-aligned resource drawer omitted the no-drag contract already used by the creation popup.

## Why the first fix was insufficient
The first defect was real: hidden:undefined overwrote Base UI's retained-node hidden state, leaving an empty shell/backdrop. That repair made state transitions and keyboard/backdrop dismissal work. Verification then activated the X by accessibility element identity; it did not exercise OS hit testing. Treating that as proof of mouse correctness concealed the second defect. The user's subsequent feedback and a coordinate click established the remaining failure.

## Prevention
- Apply the existing no-drag contract to the owning popup, not a substitute click handler or a higher z-index.
- Pin the boundary in a component contract test, with an explicit statement that jsdom/userEvent does not prove native pointer delivery.
- Require actual OS coordinate X clicks for desktop surfaces overlapping shell drag regions, then prove hidden drawer, clear backdrop and working reopen.
- Correct the earlier verification record rather than leaving an unqualified success claim.

## Boundaries
The creation popup already has the correct contract; no shared primitive redesign is needed for this repair. Other overlays are not changed speculatively. No template counterpart exists for the project-specific collaboration detail spec.
