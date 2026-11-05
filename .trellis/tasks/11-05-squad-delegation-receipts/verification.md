# M1-C1 implementation verification

Status: independently reviewed with two local fixes; final scoped checks recorded below. No work commit made by implementer or reviewer.

## Delivered

- Ordinary CLI comment add/update consumes the returned admission outcomes. It warns on stderr about blocked, malformed, unknown or conflicting target receipts; full JSON stdout and successful saved-write exit semantics are preserved, including table mode's existing empty stdout.
- Canonical target type/UUID plus status/reason shape are validated; identical duplicate receipts are counted once, contradictory duplicates stay unconfirmed. Arbitrary response values are not copied into the terminal warning. The CLI does not infer missing intended targets from comment text; the leader knows that set.
- Owner and guest leader briefings require exact live-roster target matching, distinguish queued/coalesced/deferred from execution/completion, and record partial or uncertain delegation as failed with the handled subset. One-comment, status ownership, human done and dual-trigger boundaries remain covered.
- Lost responses cause no automatic second write. Guidance checks existing comments for write occurrence and explicitly states that readback cannot recover the original receipt.
- Runtime mention facts and the two built-in skills/source maps agree. No DB/API schema or queue implementation changed in this lane; no dependency added.

## Executed checks

All named counts include subtests and are scoped inventories, not a full-repository count. Commands and full names are retained in `evidence/*.json`; raw logs remain at the private paths recorded in `evidence/manifest.json` with SHA-256 hashes.

| Check | Result | Evidence run |
| --- | --- | --- |
| CLI add/update receipt matrix and existing write contracts | 62 pass, 0 fail, 0 skip | receipts-cli-green-02 |
| Leader owner/guest protocol and roster | 10 pass, 0 fail, 0 skip | receipts-handler-green-03 |
| Runtime mention loop/receipt contract | 4 pass, 0 fail, 0 skip | receipts-execenv-green-01 |
| Built-in skill format/line budgets/YAML | 20 pass, 0 fail, 0 skip | receipts-skills-green-01 |
| Mention and issue-loop skill contracts | 8 pass, 0 fail, 0 skip | receipts-skill-contract-green-01 |
| Go build and go vet, CLI/handler/execenv | exit 0 for both | static-checks.json |
| gofmt and owned diff whitespace | clean | final local inspection |

The CLI matrix runs real command functions against a fake HTTP server and checks exactly one POST/PUT, unchanged response fields, warning/exit semantics, legacy absent/empty outcomes, known/unknown statuses, invalid target identity, malformed container/entry/field, conflicting duplicates, table output, invalid JSON and connection loss. Tests use a synthetic mat_ token and private HOME.

Handler tests ran on the coordinator-owned isolated PostgreSQL17 database `rnd_m1_receipts_green01`, cloned after migration765. No agent/database was borrowed from other work. Go is the local darwin/arm64 go1.27.0; commands use GOMAXPROCS=2 and -p1. The final handler run adds explicit lost-response readback wording; the prior green-02 is retained, not counted twice.

## Retained failures

1. `receipts-cli-red-01`: test fixture used the general synthetic token despite a daemon-task marker. The existing client guard refused before HTTP (50 named failures). Fixed the fixture to use a synthetic mat_ token; this is setup evidence, not a product regression.
2. `receipts-cli-red-02`: 16 pass, 34 fail. Fake-server writes succeeded but blocked/malformed/unknown receipts produced no warning. This is the real CLI behavioral red.
3. `receipts-handler-red-01`: 1 named failure because both protocol variants lacked the receipt checks.
4. `receipts-execenv-red-01`: 2 pass, 2 fail at the missing admission/receipt facts; preexisting loop controls passed.
5. CLI/handler green-01 attempts failed to compile in parallel lane `internal/auth/task_capability.go` (pgtype.Text Email passed as string). Zero tests ran. That lane fixed it; reruns above passed. No cross-owner edit was made.

## Files and limits

The eleven owned product paths and their SHA-256 values are in `evidence/manifest.json`. This manifest binds this lane's source; the main session owns final integration verification with the least-privilege lane.

Not tested: a real model following these instructions, provider execution, browser flows, hosted CI/Release, durable receipt recovery, idempotent ordinary comment POST, or server-derived squad activity proof. Those are separate milestones; this delivery does not complete M1 and does not accept M2/M3. No paid model, production, remote push or publication ran.

## Independent check follow-up

The reviewer fixed two local issues: malformed receipts with an identifiable target now invalidate that target (instead of counting it in both handled/blocked and unknown); the generic mentioning skill limits `squad activity failed` guidance to squad leaders on delegation-only turns. No other lane was edited.

- Behavioral red `receipts-independent-cli-red-01`:44 named pass,9 fail,0 skip (six failing leaf cases plus their parents). All failures reflect the duplicate-target summary bug; no compilation/setup failure.
- Final independent CLI check:70 named pass,0 fail,0 skip; includes the three malformed-duplicate cases for both add/update and persistent ambiguity after a third duplicate.
- Fresh owned handler database `rnd_m1_receipts_review01`:10 named pass,0 fail,0 skip.
- Runtime mention checks:4 named pass,0 fail,0 skip.
- Skill formatting/YAML and relevant contracts:28 named pass,0 fail,0 skip.
- Independent Go build and vet and owned-path formatting/whitespace results are in `evidence/receipts-independent-*.json` and `independent-review.md`.

Original implementation source fingerprints remain in `evidence/manifest-before-independent-review.json`; `manifest.json` now records final reviewed source fingerprints plus retained independent red/green evidence. Original evidence counts above are historical and are not added to these final counts.
