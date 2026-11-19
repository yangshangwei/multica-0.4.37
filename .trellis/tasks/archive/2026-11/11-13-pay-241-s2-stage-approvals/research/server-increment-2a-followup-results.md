# S2 server increment 2a follow-up

The three review follow-ups are implemented. Focused normal/race checks passed; independent recheck remains pending. The original 2a and supplemental reports are unchanged.

- **S2A-C01:** Registered all 16 original S2 concurrent-index cleanup hooks. The scoped inventory reads actual migrations and checks the same up hooks; it leaves the global inventory and unrelated eight F2 defects visible. First red: exit1/one named failure; green: one pass.
- **S2A-C02:** R1 source-denied child rows now expose `content_digest:null`. A5 requires current source membership for any nonempty child selection, so a cached digest cannot bypass the read boundary. Source-authorized picker fixtures retain their hex64 identity.
- **V01:** `AdvanceStage` checks retained decision and original signer authority again after the potentially slow artifact observers, using the final database clock. The caller's own credential is independently checked. The deterministic DBTX regression first reproduced both approval expiry and original issuer expiry advancing the stage; caller expiry was the passing control. All three now refuse at the final boundary.

## Verification

| Command selection | Named pass/fail/skip | Exit |
|---|---:|---:|
| cmd/migrate scoped S2 cleanup inventory | 1/0/0 | 0 |
| service `^TestStage` | 90/0/0 | 0 |
| service race, source denial and final clock regressions | 5/0/0 | 0 |
| handler `^TestStageHTTP` | 10/0/0 | 0 |

Raw logs, direct exit sidecars, named inventories and current changed-file hashes are in the adjacent JSON. Each DB test used its exact original task-owned package profile through `run.mjs`. No migration CLI, API, daemon, model, deployment or git write was executed by this lane.

The current migration registry hash also contains the subsequent 827 subject-index registration. That registration is 2b work, not retrospective 2a evidence. Full 2b report/QA implementation, 2d writer guards, enabled policy, production Web/Desktop and A01–A12 acceptance remain outstanding.
