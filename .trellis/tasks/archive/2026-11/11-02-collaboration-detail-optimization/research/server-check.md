# Server check — collaboration detail optimization

Active task: `.trellis/tasks/11-02-collaboration-detail-optimization`.
Review completed 2026-11-03 against the current shared checkout.

## Findings (fixed)

- File: `server/internal/service/collaboration_delivery.go`
  - Issue: submission projection trusted a handoff whose sender/recipient or event target disagreed with its actual author/initiator. Decision projection accepted a noninitiator actor, and returned decisions did not validate their exact transfer handoff.
  - Fix: validate the acceptance handoff's purpose, sender, initiator recipient and event target; require the actual initiator for ordinary accepted/returned/cancelled decisions; bind a revision to its exact issue/handoff, actor, sender, recipient, reason and recorded time. Decision lifecycle epochs remain positive and bounded. Invalid identities remain unavailable; no timestamp inference or administrative acceptance was added.
- File: `server/internal/service/collaboration_delivery.go`
  - Issue: valid-UUID source-comment references and wrong-source attachment metadata could be presented as available evidence.
  - Fix: validate the retained comment's issue/source relationship and delivered attachment source metadata before exposing evidence. The existing comment getter includes tombstones, and current comment text is never compared with or copied over the result snapshot. Missing relationships are unavailable; database failures remain errors.
- File: `server/internal/handler/collaboration_creation.go`
  - Issue: explicit `ai_policy: null` was treated as omission and created an assisted task; present non-string policies returned a different parser code.
  - Fix: preserve field presence with `json.RawMessage`, then require exactly `manual` or `assisted`. Present null/empty/non-string/unknown values return fixed `invalid_ai_policy` 400 before creation. Omission retains the assisted default and the existing omitted-field service digest.
- Files: `server/internal/service/collaboration_ai_policy_test.go`, `server/internal/service/collaboration_delivery_projection_test.go`, `server/internal/handler/collaboration_delivery_test.go`
  - Issue: existing policy coverage seeded raw persisted policy and did not exercise the actual transition; exact projection and HTTP boundaries lacked regression coverage.
  - Fix: reuse existing managed, workflow, public-creation and testutil fixtures. Added initiator/machine denial, exact-key replay/conflict/no-op, real manual Stop/assisted-no-resume, issued-permit/proof/reservation/waiting-input preservation, required-write rollback, malformed identity/source evidence, strict JSON/UUIDs, escaped 64KiB prose fields with a 1MiB public limit, committed refusal status, and own content-free confirmation after access loss. The confirmation test uses the registered POST participant-removal path.

## Findings (not fixed)

- Package specs still need the new delivery/manual-policy contract synchronized. At review completion, `.trellis/spec/server/collaboration-foundation.md`, `collaboration-content.md`, `collaboration-targets.md` and the Server index did not yet describe `ai_policy`, required basis or exact delivery decisions. This is root-owned documentation scope, so no shared specs were edited here. The root's updated built-in `multica-working-on-issues/references/collaboration-workflow.md` already contains the new human delivery/manual-policy contract.
- No additional production Server issue remains in the assigned review scope. Existing caller adaptations, complete package/repository checks and production Web/Electron acceptance remain separately owned by the root and other lanes.

## Verification

- Lint: **pass** — `gofmt -l` on all reviewer-edited files is empty; `git diff --check` passes; `go -C server vet ./internal/service ./internal/handler ./internal/collaboration ./cmd/server` exits 0.
- TypeCheck: **pass** — `go -C server build ./...` exits 0 after the final production changes.
- Tests: **pass** — serial root-permitted service/handler runners, all final runs with `--race`, no skips:
  - `^TestCollaboration(ManualPolicy|DeliveryProjection|DeliveryRetainsCommentSnapshotAndEncryptedEvidence)`: **33 named passes**, 0 failures, 0 skips, 11 top-level tests.
  - `^TestCollaboration(CreationPolicyHTTP|DeliveryHTTP|AIPolicyHTTP)`: **17 named passes**, 0 failures, 0 skips, 7 top-level tests.
  - The one affected confirmation test was rechecked after aligning its request with the production POST removal route: **1 pass**, 0 failures, 0 skips. This overlaps the previous 17 identities and is not an additional product case.
- The retained-comment/encrypted-evidence canonical test passed after the new relationship checks.
- R38 administration remains unchanged: `admin_collaboration.go` still invokes the existing scoped lifecycle path without preparing an initiator delivery decision. Ordinary decision projection requires the actual initiator.
- Manual admission was traced through explicit input, Resume, registration/resource/consent/Switch mutation kinds, promotion, Claim, Permit and current admission used by continuation/delegation paths. Stop, withdrawal/revocation, original operation reads and sealed proof remain separate.
- The policy migration adds no foreign key or index and guards downgrade while manual rows remain. Additive SQL/generated scanners and new account routes were inspected.
- No database was created, migrated, dropped or switched by this reviewer. No real provider CLI, account or quota was used. Managed proof tests use synthetic protocol proof and do not establish native containment.
- All reviewer DB handles are closed and the shared test slot was explicitly released to `server_callers_v2`.

### Retained run records

All report paths below are relative to `research/continuation-20261103/`:

| Run | Result | Report |
| --- | --- | --- |
| Initial policy/identity red | 14 named pass, 16 fail, 0 skip; 13 missing identity checks reproduced | `server-review-policy-projection-red-1-20261103-100936.json` |
| Scope red | 0 named pass, 2 fail, 0 skip | `server-review-evidence-scope-red-1-20261103-103205.json` |
| Creation enum red | 2 named pass, 5 fail, 0 skip | `server-review-creation-policy-enum-red-1-20261103-103940.json` |
| Final service race green | 33 named pass, 0 fail, 0 skip | `server-review-final-service-green-1-20261103-104403.json` |
| Final handler race green | 17 named pass, 0 fail, 0 skip | `server-review-final-http-green-1-20261103-104911.json` |
| Corrected-route case race green | 1 named pass, 0 fail, 0 skip | `server-review-http-public-remove-path-green-1-20261103-105247.json` |

Each JSON report identifies the exact command, permitted database and retained private JSONL output. The final green handles were 80126, 87528 and 54755; all are closed.

## Production handoff identity

Reviewer production edits are limited to:

| File | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_delivery.go` | `d0807d040c2158914214ae92d010456bce063c9e9643d5557dfdac9c1af309f7` |
| `server/internal/handler/collaboration_creation.go` | `251d122737d23c2e310ddbdce52f0ed5ca0ca253fca5c85b6cc94475266a1f52` |

The following manifest covers the 29 changed/untracked task production Go/SQL/migration files, with tests excluded. Its canonical sorted compact JSON SHA-256 is `efa3e8d14d01084129f9ea11dd8dbac1216cb902eab1baf7869bf88f8b858fd5`. It is a reviewed code manifest, not the root's complete deployed source/artifact fingerprint; root-owned embedded-doc changes still require the normal snapshot sync/rebuild and provenance binding.

```json
{
  "server/cmd/server/router.go": {
    "sha256": "57f90b8fdcb2cc83a4a26c131092629ac402d72e6e93241a05f5d09b1bf09bc8",
    "mode": "0o755"
  },
  "server/internal/collaboration/access.go": {
    "sha256": "e353d02c8b1dfb793492f5551420cd9c1fb2722e90bc41d200898ccb647a6c64",
    "mode": "0o644"
  },
  "server/internal/handler/collaboration.go": {
    "sha256": "d0a141a653cafb514074c0deebcdb0ce2723a65b8b7448d7bea77c80fb311889",
    "mode": "0o644"
  },
  "server/internal/handler/collaboration_ai_policy.go": {
    "sha256": "48dd6cab88176d17e31985f38d0bb5efc99420966b582a1a60d2dcae60443090",
    "mode": "0o644"
  },
  "server/internal/handler/collaboration_creation.go": {
    "sha256": "251d122737d23c2e310ddbdce52f0ed5ca0ca253fca5c85b6cc94475266a1f52",
    "mode": "0o644"
  },
  "server/internal/handler/collaboration_lifecycle.go": {
    "sha256": "5117f38845a38075cb7b94876696f3ffa9f58b97198dae58b3511af5804ae8f3",
    "mode": "0o644"
  },
  "server/internal/handler/collaboration_workflow.go": {
    "sha256": "8e8ccec1cc1b4131919d9a93fd4acb1d29f70f6c83ee21d5ba7a0053e6489430",
    "mode": "0o644"
  },
  "server/internal/service/collaboration.go": {
    "sha256": "3ad137953cd0e2e2e8c491a1c80eeb5b97318233059649efbd90a9e00bdccf19",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_ai_policy.go": {
    "sha256": "523f192b1f83077dcdb99a0e5643d7e5c9d788aaf0ac3f6c22016cd99a7cd1d0",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_authority.go": {
    "sha256": "e067d0f593f6a9482423c641413b08f6eed66bb552dc94020d94800f1f89c52e",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_batches.go": {
    "sha256": "098ee6f0a6ff05dd778ead3e3545077133c0e184503943b6f35448339aacdfd7",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_control.go": {
    "sha256": "b2006df16d203429136e74a8f024feb5fcd489fe438f37d8affcd5528c698e2a",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_delivery.go": {
    "sha256": "d0807d040c2158914214ae92d010456bce063c9e9643d5557dfdac9c1af309f7",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_handoff.go": {
    "sha256": "09f93f41c49d38342be52e1a82019dc47f0d964185a9d184c34f7c6ef82d7aaa",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_lifecycle.go": {
    "sha256": "5d98c9d83a1a4033098263957e1d070412e788d45f00e68584575d0eb67712d7",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_mentions.go": {
    "sha256": "a20b671c1541c93f808881950e387302c5af1e33269c7f98b034fb5d93c2add1",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_queue.go": {
    "sha256": "6fd21ce5d824e0a41951cfa3de83ba2fb2db46cc4366cdea74b45c2931ccd34b",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_runtime.go": {
    "sha256": "3db09f2e93be3c22215da1c87130b9306b484c57a8eb1fc0c26a1c8cbb2ca47b",
    "mode": "0o644"
  },
  "server/internal/service/collaboration_workflow.go": {
    "sha256": "1ef46765e1567a1c762f4647d1553f69b9c8dfd31d1fcc168d77d8795dc28724",
    "mode": "0o644"
  },
  "server/migrations/764_collaboration_ai_policy.down.sql": {
    "sha256": "a3049301fd5f6514a96da66d453f7031f97c653e484f2ca18cf48dd0f6e37f42",
    "mode": "0o644"
  },
  "server/migrations/764_collaboration_ai_policy.up.sql": {
    "sha256": "935800286a9134fcf9fdc74e090d1a485125fa5f601a1b8efa0eec0fe3f396ea",
    "mode": "0o644"
  },
  "server/pkg/db/generated/collaboration.sql.go": {
    "sha256": "ac5c3592ae443ce9360c27709f04d22453bc853ff810e8f77d5a9091263ef1e2",
    "mode": "0o644"
  },
  "server/pkg/db/generated/collaboration_delivery.sql.go": {
    "sha256": "1c91a59d79c2bfaa852f52794a328fb8a9f02a236732dec08167f2d865115871",
    "mode": "0o644"
  },
  "server/pkg/db/generated/collaboration_knowledge_service.sql.go": {
    "sha256": "ca38da913fad927991cfc7f791aa30e0390697b82384fbb657a0513aa8960c6d",
    "mode": "0o644"
  },
  "server/pkg/db/generated/collaboration_lifecycle.sql.go": {
    "sha256": "45a741676b2c890fc2b3957656dcae16570be08654fccd561f1e6eefd78e4b0a",
    "mode": "0o644"
  },
  "server/pkg/db/generated/collaboration_reconciliation.sql.go": {
    "sha256": "dde44fd5bc1288feeac25dd3c4e8679d51ce7a659d418c6c1565cf3039095343",
    "mode": "0o644"
  },
  "server/pkg/db/generated/models.go": {
    "sha256": "5c076a036effd67502f52efb97e43de3a3c9bc1d102e2c2f1f71c7caeb0e1171",
    "mode": "0o755"
  },
  "server/pkg/db/queries/collaboration.sql": {
    "sha256": "52c5d0e1db3ea4456fb98dee9ef88b808c0330edba7cd49bddf9deb1c10d0589",
    "mode": "0o644"
  },
  "server/pkg/db/queries/collaboration_delivery.sql": {
    "sha256": "aa58da207a2533591f2e1d974d036dacf58f3d2569a063f4db677685ded55cd8",
    "mode": "0o644"
  }
}
```

No Core/Views/e2e/spec/commit or existing caller-test file was edited by this reviewer. The fixes extend existing validators/queries and fixtures; they add no result storage system, dependency or scheduler abstraction.
