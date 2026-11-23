# Actual CCG1 pre-execution binding review — 2026-11-04

**Pass with notes; 0 blockers in the actual allocated CCG1 conditions.** Both actual permits remain `ready_to_execute=false`. `root-permission-c1.json` is intentionally absent until Root issues final permission. This review grants no runtime-test acceptance and changed no permission, source, cache, database or service.

## Verified actual bindings

| Input | SHA-256 |
| --- | --- |
| Actual normal false permit | `3f6d63519a055b7ca65d715a017d40f4c93248a17b3406c0814ff91719024358` |
| Actual race false permit | `0033d13871f72ca4d1460d7ea9319f8077a56a50e377d0a38911801942f8e682` |
| CCG1 runner | `4b29de6c452da8f1171b343113297710d055caf5c6aacf41a4ac1794837d2d39` |
| Source inputs | `97bc4ea95f4b5ab628881268c5d7a5f80c1ae9d0a8bcd4eb8ebc7db3a07f3ecf` |
| Root allocation record | `d758d42b4f24a912819ae5b66867fb2462b4cfce3c59eaff19d7e0522371866b` |
| Independent completed shared readiness | `b84ae87408a08e3e2536dfef08935c92b1ec23deddfc8e8a5c88b00d343734a2` |
| Root post-V16 resource release | `ffd84568037732e3642d3a0c661fbd90a34596150086e26460bdc1f034bf9200` |

The actual permits are regular owner-owned 0600 files without redirected paths. Their candidate, runner, source-input, readiness and release hashes match current files. All 11 selected CCG source hashes, four pinned tools and the actual registered review/parser/auditor remain unchanged. Candidate HEAD is `eda0d68fb07852b714f3092aa02ca0523d3ad312`; the sole working-tree difference is the exact registered Next generated import rewrite.

The independent shared readiness record proves the actual complete V16 exit 0, complete named inventory and all 113 explicit skip dispositions. Its 23 evidence references were independently hash-verified, including the original successful final filesystem audit and immutable run/log identities. The Root release witness records released API/Web listeners and retained databases. The existing final auditor was not invoked again.

## Fresh databases and contained temp

The actual normal/race databases are `multica_knowledge_host_composite_20261101_v16n_a3c9` and `multica_knowledge_host_composite_20261101_v16r_a3c9`. Both satisfy the retained literal prefix and PostgreSQL identifier limit. Their Root before/allocation records show absence before creation and distinct fresh clones from the clean migration-764 seed.

Both permit URLs were inspected privately. A read-only psql query through each exact explicit endpoint verified `current_database()`, migration `764_collaboration_ai_policy`, and zero issue/task rows; both returned exit 0 with empty stderr. Connection credentials were kept out of commands, reports and output. The results are in `database-readonly-verification.json`; no database write occurred.

Normal temp `/private/tmp/multica-ccg-v16-normal-pts2beua` and race temp `/private/tmp/multica-ccg-v16-race-yerk581i` are distinct, owner-owned regular 0700 directories with no Git ancestor or redirected path. Each matches its actual allocation and permit.

## Cache provenance

The COW cache handoff is complete. All 10,465 files were independently checked against `cache-before.json`, covering 3,108,203,684 bytes per tree. Source hashes, modes, sizes and original inode identities remain unchanged; target content/modes/sizes match; every source/target inode differs. The target file set is complete and has no symlink, extra or missing file. See `cache-readonly-verification.json`.

CCG16 GOCACHE and its empty GOPATH are owner-owned 0700 directories without redirection. GOMODCACHE remains the separately pinned existing `/Users/artisan/go/pkg/mod`. Normal and race share only this approved CCG cache and execute serially; the original V16 cache is not a writable alias.

## Root's final issuance and execution

Root can now write the final root permission record and flip the two actual permits to ready, retaining their false templates and recording the resulting new hashes. The sole executor then runs normal followed by race with unique labels and the unchanged target:

`TestCollaborationKnowledgeHostCompositeCorrectionReplacesActualInputAndWithdrawalRefusesNextLaunch`, `knowledgeintegration`, guarded by `scripts/go-test-with-agent-cli-guard.sh`, `-p=1`, `-parallel=2`, `-count=1`, JSON reporting and the existing five-minute limit; race adds `-race`.

Each actual result must independently prove exactly one named run/pass, zero fail/skip, the two separate physical inputs, pending-v2 approval/correction, superseded-v1 exclusion, withdrawal-time 409, zero third physical launch and retained original exposure/receipts. No CCG1 test was executed or counted by this review.

CCG2 OFF/ON templates remain false and unchanged; no actual CCG2 permission exists. Their binary/deployment/fresh-resource and runtime-result gates remain separate. Knowledge budget/calls remain **0/0**, real Provider permission remains false, and production injection remains default false. Only this review directory was written.
