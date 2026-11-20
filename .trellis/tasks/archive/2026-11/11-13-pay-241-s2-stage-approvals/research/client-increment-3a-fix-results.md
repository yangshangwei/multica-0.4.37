# S2 client 3a independent-review corrections

Status: **UNIT FIXES COMPLETE — INDEPENDENT RECHECK PENDING** (2026-11-14T04:40:16.160853+00:00). C01–C07 were verified against source, reproduced before correction and addressed in this client increment. This is not a passed independent check, S2 or A01–A12 acceptance.

- Review: `client-increment-3a-check.md` (SHA-256 `17ab152d90d3ec9eb2da954daabd2d54ec648bd1a872c1e6d2d28654f11455ce`) and its JSON. Neither the original review nor the original 3a/A5 results were edited.
- HEAD: `8e824b3035c4f17a9a9dfa0504f1294fbcc9b20b` plus the uncommitted client overlay and other owners' work. No git writes, server edits, dependencies, real API/DB/browser/Desktop/provider or build commands.
- Started against wire `73a3498c34bf205e0cd78d7226b0ca32e35ed7b438b54213a750141115032c82`. End wire `29f07b2747fafed8df7944afd5a0e4deade456f77a11b538956bf1d59c708fce` includes coordinator/server section 13.11 (A5's committed scope does not expire with JWT/PAT). I read that addition and the revised A5 paragraph: the existing client already has no A5 credential-expiry field or rule, so no extra wire change was needed.
- The S5 measurement hold was respected. Only read/edit work began during the hold; tests started after the coordinator explicitly released it.

## Findings and corrections

| Finding | Implemented correction | Canonical evidence |
| --- | --- | --- |
| C01 | Same storage prefix now holds either a sendable full intent or an issue-scoped recovery-only identity. Issue denial erases body/prose, retains original key/target/action/minimal matching identity; both use the same confirmation matcher. Recovery-only records and stale full handles cannot POST. Denied detail mounts GET-only recovery. Account switch/logout clears both. | stage-actions.test.ts C01; stage-intent.test.ts C01 and scoped erasure; stage-recovery.test.tsx; collaboration-detail.test.tsx C01; stage-schema/code-schema identity matcher parity |
| C02 | Explicit source workspace in R4 Query keys, metadata-only monotonic generation per QueryClient/Server/viewer/workspace. Exact source denial cancels without rollback and erases private sibling/cross-issue R4 data. Older success/error cannot republish or revoke unrelated issue identity. Already-anonymous R4 identity keeps its original read/error state; fresh authorized reads alone restore private data. | stage-source-cache.test.ts (3); stage-code-panel.test.tsx C02 sibling-cache race |
| C03 | Current proper-role return derives from coherent R1/R4 identity, separately from source readability/signable approval. Withheld baseline permits reason-only return with null expiry/all-null evidence/no dependencies. Optional hidden references and approval stay refused; withheld-source form contains no evidence pickers. | stage-code-presentation.test.ts C03; stage-code-actions.test.ts C03; stage-code-panel.test.tsx C03 |
| C04 | Fresh R1 exact candidate ref/state must agree with R4 for approval/advance. Selected R4 refetches when discovery changes. Independent revocation remains available for superseded unconsumed decisions. | stage-code-presentation.test.ts C04; stage-code-panel.test.tsx C04 open form |
| C05 | Existing memberListOptions query function supplies the current source roster; only one matching workspace/user row becomes a minimal role projection under Server/viewer scope. Owner/admin plus successful non-fetching authority read required; preparation validates target/account/role. No public P1 field added. | stage-actions.test.ts C05 (5); stage-panel.test.tsx C05 owner/admin/member/unknown/denied; C06 authority refresh failure |
| C06 | Parent issue/R1, P1/source authority and R3 errors disable new writes/resends, preserve form input and leave original GET recovery usable. Proposal revision and decision forms subscribe to their required read; explicit successful refresh unlocks retained forms. | stage-panel.test.tsx C06 R1/R3/P1/authority refresh failures; stage-recovery.test.tsx GET-only recovery |
| C07 | R4 schema rejects state valid with a non-null revocation; validDecisions excludes terminal revocations defensively. | stage-code-schema.test.ts C07; stage-code-presentation.test.ts C07 |

### C01 storage and receipt boundary

The existing `multica_collaboration_stage:` key still owns one record per original request. A full sendable intent remains the same format. On denied issue access, its private JSON body and source/prose fields are replaced with a strict recovery-only record carrying the original account/server, issue, action, request key and only the IDs/versions/digests/expiry the receipt matcher actually needs. The same matcher handles full and reduced identities; no full request body is reconstructed. Recovery-only objects cannot enter send or resend, and an old full handle cannot repersist its body while the issue is denied.

The denied detail branch mounts a named GET-only recovery region without an issue DTO. GET404 remains unknown; exact confirmation retires the original key. Scoped denial leaves other issue/project records alone. Account switch/logout removes full and reduced records. A fresh restored issue read does not make the reduced record sendable again.

### C02 workspace scope and races

R4 keys now include the explicit source workspace supplied by the authorized issue DTO. A monotonic metadata-only generation is scoped by QueryClient, Server/viewer session and source workspace. The exact source-denied projection advances that generation before cancellation, removes private R4 data across all candidates and issues in that workspace, and replaces cancellation rollback snapshots using the established knowledge/access pattern. Other workspaces and issue/R1/history/recovery caches are unchanged.

Both old successful responses and old errors are fenced: a late 404 from a superseded source read cannot clear legitimate collaboration access. Private evidence can return only from a read started after the fence. Already-anonymous, server-issued R4 identities are preserved with their original read/error state; another denial does not turn an old error into success or repeatedly tear down safe reason-only return UI.

### C05 current source role

No `can_manage` field was invented. `memberListOptions` supplies its existing explicit-workspace query function, wrapped by the account/session guard and mapped to only the current viewer's matching workspace role. Ambiguous, missing or unknown rows yield no administration right. The fresh authority query and P1 error state govern the form, and the prepare function independently requires matching account/workspace plus owner/admin. An ordinary member can still read policy facts. The backend remains the final authority at write time.

## Red, green and final checks

Logs live in `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/s2-client-fix3a-*.log`; the JSON preserves exact commands, exit status and log SHA-256.

| Log suffix | Exit | Result |
| --- | --- | --- |
| `c01-c03-c04-c07-red` | 1 | 5 failed | 68 skipped (73) |
| `c02-red` | 1 | 1 failed (1) |
| `c05-c06-red` | 1 | 5 failed | 2 passed | 19 skipped (26) |
| `types1` | 2 | typecheck / lint |
| `core-green1` | 0 | 84 passed (84) |
| `views-green1` | 1 | 2 failed | 46 passed (48) |
| `views-green2` | 0 | 54 passed (54) |
| `types2` | 0 | typecheck / lint |
| `core-all1` | 0 | 291 passed (291) |
| `views-all1` | 0 | 127 passed (127) |
| `types3` | 2 | typecheck / lint |
| `actions-final` | 0 | 45 passed (45) |
| `core-lint1` | 0 | typecheck / lint |
| `views-lint1` | 0 | typecheck / lint |
| `types-final` | 0 | typecheck / lint |
| `source-identity-red` | 1 | 1 failed | 2 passed (3) |
| `source-final` | 0 | 3 passed (3) |
| `source-views-final` | capture error | 2 passed | 12 skipped (14) |
| `types-final2` | 0 | typecheck / lint |
| `core-lint-final` | 0 | typecheck / lint |
| `source-views-final2` | 0 | 2 passed | 12 skipped (14) |

- Initial C01/C03/C04/C07 run: **5 counterexamples failed**. C02 sibling/cross-issue cache run: **1 failed**. C05/C06 UI run: **5 failed**, owner/admin controls passed. Filtered tests were intentional, not accepted skips.
- Later C02 repeated-denial identity counterexample failed, then its complete source-cache suite passed after preserving only already-anonymous data and original error state.
- Full affected Core stage/draft run: **12 suites / 291 passed / 0 failed or skipped**. After the final test-only typing correction and source-cache refinement, the changed suites passed again: **45 stage-actions** and **3 source-cache** tests. The new source identity case is included in the latter.
- Full affected Views/detail/locales run: **6 suites / 127 passed / 0 failed or skipped**. Final source-cache/UI refinement recheck: **2 C02/C03 tests passed**, with 12 deliberately filtered cases.
- Final Core/Views typecheck passed. All affected Core/Views source/tests passed eslint with zero code warnings. Existing package-manager configuration warnings are unchanged. Scoped tracked-file `git diff --check` passed.
- Early typecheck failures concerned the new full/recovery union at old test/consumer sites and a duplicated test-fixture role property; they were corrected. The first Views green attempt needed `waitFor` to await Query observer rendering after `refetchQueries`; both actual error locks then passed and retained inputs were asserted.
- `source-views-final` printed passing tests but its exit-line append had a mistyped `pay241-20161113-01` directory. I verified the missing path, kept that original log and reran the same two tests with one correct log-path variable. `source-views-final2` has the direct captured exit 0; the first log is explicitly marked a capture error, not used as final exit proof.

## Changed files and source binding

26 code/test/locale files changed against `logs/s2-client-fix3a-before.json`. The JSON records before and final hashes; a null before-hash denotes a new file. API transport, server, platform routes, unrelated creation-page work and dependencies were not changed in this fix pass.

| File | Final SHA-256 |
| --- | --- |
| `packages/core/collaboration/index.ts` | `2a530d53951098a8b2d9ac92b13064686492f8f1f9d1f275b7c514ee9c048bc9` |
| `packages/core/collaboration/stage-actions.test.ts` | `4cbbc17945f815234e35d096cb8edac163f944db81232c1cdab020b1373ed58c` |
| `packages/core/collaboration/stage-actions.ts` | `71774b76ac4f1b69134d2caa102277021676f6c8a9047df79dda64d37ae510e7` |
| `packages/core/collaboration/stage-code-actions.test.ts` | `974cbf84f1ac4ac6f1612766f2a9de54656b7ba9f44a83148a3472da06565a2c` |
| `packages/core/collaboration/stage-code-presentation.test.ts` | `707d9acfbb5a39f69e138d7309116f80cd7210f599a23230be36b95b44f4cc26` |
| `packages/core/collaboration/stage-code-presentation.ts` | `b3ded1dc4b36c32412fea06327672248ffc4b53f105584f3b554bc94d916e9f0` |
| `packages/core/collaboration/stage-code-schema.test.ts` | `d649ecf1814355457eb9a7df0f08452b3922b230239c81650b0e2a36cba5bef7` |
| `packages/core/collaboration/stage-intent.test.ts` | `de60ef83047d51fb82b89a46d430cc0292af92630e5a90bd95b3dc6e70c3af95` |
| `packages/core/collaboration/stage-intent.ts` | `bd205750510a16b8ca8bab54dd968691dae932a802b95676fc072c80cfc9b670` |
| `packages/core/collaboration/stage-queries.test.ts` | `a62fb1a29400cd4eda0033847cec183e2e08d79bea23dae513bef93766a749ca` |
| `packages/core/collaboration/stage-queries.ts` | `0de8a66fca4d9416b290f959a3ca837be56be3f683a3a55f5d628f4bcda4de17` |
| `packages/core/collaboration/stage-schema.test.ts` | `22ecf3a91694f9ac8bfe18004db2243b1da22118dd5a710c3cb319ecd5984766` |
| `packages/core/collaboration/stage-schema.ts` | `ff76c8c554710c627132ff8fe70dcf2ebe148ac08cd6da2500392d4960c7f518` |
| `packages/core/collaboration/stage-source-cache.test.ts` | `673780b61fe3d04f37f67ff0eb1e5b26e4f6456e97c56cf45b05204213997388` |
| `packages/core/collaboration/stage-source-cache.ts` | `033711eda34e7fd06295abb6316a34f41abe476d57c47e84d81fdb7ed51fa461` |
| `packages/views/collaboration/collaboration-detail.test.tsx` | `7d913ca33324548097b2d3c0dc83edf84ae29992d0a4ffdb5ab91544fadb80e3` |
| `packages/views/collaboration/collaboration-detail.tsx` | `6c3a3167988fc485737f3ea4f4631c9cbcf4e926836b5719c7bacde994dddc2b` |
| `packages/views/collaboration/stage-code-forms.tsx` | `403f8003aadb61dd4ab6b3c2dd9e183901cc2da046033bccd1c97feb6249288e` |
| `packages/views/collaboration/stage-code-panel.test.tsx` | `778093a4c94ddf2891b103131bb2350040b8da5ff5d52b57a49bccf66e490c25` |
| `packages/views/collaboration/stage-code-panel.tsx` | `fee486c0acd34c6f933d8d220fb3122633d1cf74b888996a4fbe4b902ed53da9` |
| `packages/views/collaboration/stage-forms.tsx` | `b49b1cda0df7666ab59e8c83cc9c682f7cf838f940dd1865b172c62e8eb22332` |
| `packages/views/collaboration/stage-panel.test.tsx` | `595e48b3138f8b90f9f7a6e5093769ce526b597f5091ca31ec4a118d5cac1da2` |
| `packages/views/collaboration/stage-panel.tsx` | `5934ab6348a507c847deed285ae33ca735c53ecf562f33a3ef8159b0d13a21e3` |
| `packages/views/collaboration/stage-recovery.test.tsx` | `93d5781c6dadc1192412e490ec97a6237da28b8d8736a68bdf098abfa858507a` |
| `packages/views/locales/en/collaboration.json` | `ed9c2b72b2b97e3eb7d63f805ea682c2414f88184212fbfc0c1cbe9f6943c835` |
| `packages/views/locales/zh-Hans/collaboration.json` | `e6a86f54cd5748f13fb248c1182a3f8afa9695211ac247e9c06fea38f614bdb5` |

These result artifacts are `research/client-increment-3a-fix-results.md` and `.json`.

## Remaining coordination

- Return to the same independent checker for recheck; no additional client implementation should begin before that boundary is reviewed.
- Root owns spec synchronization: document issue-denial payload erasure versus GET-only recovery, complete account cleanup, and the workspace-scoped R4 source fence. Existing dirty spec files were left untouched.
- Client3b still owns T4 detail after server freeze, M1 collaboration completion, generic completion blockers and activity/notifications. Future private T4 queries must join the same source-evidence fence; this pass does not invent their DTO or implement that batch.
- Actual API/DB, production Web/Electron and full A01–A12 verification remain outside this bounded unit pass.
