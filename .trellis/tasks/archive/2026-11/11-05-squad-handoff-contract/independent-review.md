# Independent squad contract review

Reviewer: native `trellis-check` (`p1_independent_check`). Worktree: `/Volumes/artisan/code/2026/mc-agent-rnd-p1-20261105`. Reviewed all 17 implementation files and relevant template/docs specs after the implementation lane completed.

## Findings fixed

The new EN/ZH child-stage paragraphs originally promised that the parent assignee is woken whenever a stage closes. The actual path suppresses Backlog, terminal, human-assigned and collaboration parents, then applies existing agent/squad validity and pending-task deduplication. Qualified the new paragraphs to an eligible agent/squad parent that can be woken and explicitly named parked/closed/human cases. No backend routing or invocation policy changed. Source anchors: `server/internal/handler/issue_child_done.go:97–116`, `:274–295`, `:650–729`. The squad path deliberately does not introduce a fresh invocation gate for an already-owned coordination return.

Updated `apps/docs/content/docs/squads.mdx`, `squads.zh.mdx`, and regenerated `server/internal/docs/content/pages/squads.json`. These three review changes supersede only those three bytes in the implementation source manifest; current hashes for all 17 files are in `research/independent-reviewed-sources.json`.

## Findings not fixed

No blocking issue remains in this bounded contract repair.

Existing instance instructions are intentionally not upgraded automatically. The daemon brief and default leader-v3 content require their corresponding deployment/update; static content tests do not demonstrate real model instruction compliance. Backend stage transitions and same-issue comment routing were read, not re-executed with a database. Automatic delivery-before-acceptance and parallel review dispatch remain separate product work.

## Contract review

- The actual generated assignment/comment/leader brief no longer promises `in_review` stage advancement. `done/cancelled` terminal-category barrier and human/integration acceptance remain unchanged.
- Same-issue member result comments retain the current narrow squad-leader wake route (`comment.go:2683–2699`); a child result is not silently promoted to parent acceptance. No new `@parent` bypass was introduced.
- The six Review Gate seats match the leader and squad policies independently: correctness, security, test evidence, Agent/Skill/MCP, experience/accessibility and migration. Specialist evidence gaps remain unknown/hold; must-fix findings return to the author. The first-batch single-member-turn protocol is preserved.
- Only review-gate-lead increments 2→3. Review Gate squad stays3; autonomy, default concurrency and attached skills are unchanged. Source maps, bilingual UI summaries and public/in-app docs agree.
- The template regression separately checks both bodies against the actual roster, so a newer squad body cannot hide a stale leader body. It verifies shipped contract content, not a general model-evaluation guarantee.

## Verification

Independently checked retained JSON inventories: execenv22 named passes, service61 named passes, zero failures/skips in final runs. The initial 501-line skill failure remains in its original evidence file; the final service run passed the canonical body budget and frontmatter validation. No repeat of the unchanged Go template suites was required.

Independently ran `gofmt -l` on the six Go files and scoped `git diff --check`: no findings. After the docs correction, generated the complete Chinese bundle into a Git-external temporary directory; 39 pages/58 assets were produced and only `pages/squads.json` differed (97 other generated files unchanged). Re-ran guarded `go -C server test -p2 -count=1 ./internal/docs`: PASS (1.040s). Evidence: `research/independent-docs-generation.json`, `research/independent-docs-tests.log`.

No database, provider, real credentials, remote application or paid model was accessed. No TS source changed; JSON-only locale key preservation was already checked by the implementation lane. Main owns git/task status and release decisions.
