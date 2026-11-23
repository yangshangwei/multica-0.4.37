# Preparation capture patch artifact

Only `e2e/collaboration-preparation-records-web.spec.ts` changes. Replace the single-inner-root scroll offset with native `scrollIntoView({ block: "start", inline: "nearest", behavior: "instant" })`, retaining the actual account-detail root guard, two animation frames, ratio1, geometry and all state assertions.

The original trace records HTML scrollTop220 alongside detail scrollTop7258 when the legacy capture fails. This patch addresses capture coordination across ancestors; it does not assert that production deep-link navigation regressed.

`git apply --check` passes against85ef5d205. The patch is not applied and no browser or repository tests were run. Root must independently review it and run the original directed wide/narrow proof on the new candidate.
