# R24 Submit result selector implementation

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.
Implementer: `/root/r24_submit_selector_implement`.

**Result: the one-site fixture repair and static checks pass. Actual browser acceptance remains main-owned.** The retained R23 four-case result stays **3 PASS / 1 FAIL / 0 NOT_RUN**, with no automatic retries; its failure is not relabeled.

## Change

Only `e2e/collaboration-stage-execution.spec.ts` was edited. The fourth case now clicks its existing exact `Submit result` toolbar button directly. The unconditional opening of the unrelated `More actions` modal menu, the button/menuitem union and `.first()` were removed. The now-unused disclosure import was removed.

The current shared workflow puts this action on the toolbar; the observed menu does not contain it. Direct Playwright clicking retains strict uniqueness and normal actionability. No force click, custom backdrop behavior, helper abstraction, product/UI change or dependency was introduced.

The edit passed the original ROOT SHA256/mode guard. Comparing the whole file with its retained pre-edit snapshot confirms that only the three authorized text replacements occurred. Every later submission, status-options, exact M1 binding, Web/Desktop transport, original-operation recovery, administrator and project-exit assertion is byte-identical. The 20-second action timeout and 480-second case timeout are unchanged.

| Source | SHA256 | Mode |
| --- | --- | --- |
| Original ROOT spec | `781c29f1bee79160e82e6c587439870f6cfbe303af3d1fab3aafc65a9972b90e` | 0644 |
| Repaired ROOT spec | `6d08bb644417d3c23b579564b7110a6d234773df7e68e187bfa03ab1175573f4` | 0644 |
| Retained R23 result | `440d404d437fa34b1e808e56ffefa98bca30ea2768cb8970db3466f24db7a21f` | Unchanged |

## Verification

All commands used the installed Node 22 and explicitly constructed private HOME/TMP environments. No dependency was installed. Command, stdout, stderr, exit and selected source/config/tool hashes are retained in the accompanying JSON references.

| Check | Result and actual scope |
| --- | --- |
| Strict TypeScript | Exit0, empty stdout/stderr; ROOT execution spec and its execution/control fixtures, with transitive source checking and the existing strict react-library configuration. NoEmit, incremental/composite disabled, noUnusedLocals/noUnusedParameters/noImplicitReturns/noUncheckedIndexedAccess retained. Dependency declarations remain excluded by the existing skipLibCheck rule. |
| Actual ESLint | Exit0; exact ROOT execution spec entry, 0 errors/warnings/messages/suppressed messages. Reused the repository Core config and error-level dependency rule with the real ROOT package context; no ignored-file success. |
| Corrected Playwright discovery | Exit0, exactly the original four tests in two files. It used a private cwd containing neither dotenv candidate and did not load ROOT dotenv. Test bodies and browser launches: 0. |
| Source preservation | Exact intended delta and ROOT mode0644 confirmed; 16 selected code/config/tool inputs stayed unchanged throughout the first three static commands, with a final repaired-spec recheck after corrected discovery. This is not a whole-ROOT fingerprint claim. |

An initial configuration lookup referenced a nonexistent ROOT ESLint config; the actual package configuration was verified before lint execution. The first discovery command used ROOT cwd and indirectly loaded `.env` through an existing fixture's `e2e/env` import. It made no API requests and printed no credential values. Its command/output/result are retained separately. A fresh discovery command used the unchanged listing config from the empty private cwd; it listed the same four identities without dotenv loading. Strict TypeScript and lint were not repeated because they did not load dotenv and had passed.

## Remaining main-owned gate

The original R23 browser failure is the retained regression RED. Main must bind the repaired spec into a new acceptance source, retain production Web/Desktop and Go consumer provenance, use fresh staging generations and execute the original four cases with workers1/retries0/maxFailures1. Actual Submit result/M1/administrator/project-exit completion, current resource restoration and independent A01–A12 review remain open. No S2, parent, S3, S4 or S5 acceptance is claimed here.

The implementer did not edit retained R23 source/config/results, task metadata, plans/specs, other source, API/DB/artifact state or Git. Private raw evidence is under `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/selector-implementation`; exact references and hashes are in [the JSON report](r24-submit-selector-implementation-20261117.json).
