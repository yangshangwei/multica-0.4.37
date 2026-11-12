# Main code review before platform acceptance

## Open B2 finding

`knowledge-panel.tsx` retains `intent` for every thrown error and uses `intent !== null` to lock all edit/decision controls. A first request that receives a definitive unsealed proposal validation/permission rejection cannot be corrected: GET recovery returns not-found and retry resends the same invalid payload. This contradicts the approved pre-persistence error boundary and 409/428 rereview flow.

The correction must distinguish definitive no-write results from prior network/commit ambiguity. Do not clear/re-key a prior uncertain operation solely because a later retry receives4xx, or because one recovery GET returns not-found. Preserve original recovery identities for uncertain results. Storage failures also need stage-aware handling: save may fail before POST, but retirement can fail after a known successful receipt; never treat both as proof of no commit.

Add interaction regressions for first definitive validation failure→edit→explicit new intent, prior lost response→later refusal remains original pending, and storage save/retirement differences. Preserve exact expected revision/hash and source-body clearing.

## Already inspected

Shared source/target mounts and current DTO use; selected state stores IDs rather than server detail. Review confirmation resets on changed identity. Source bodies render through restricted RichContent; references use navigation adapter paths. A single mechanical Impeccable pass over eight production view files returned no findings. This is not a screenshot/platform verdict.
