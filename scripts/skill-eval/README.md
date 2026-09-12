# Local real-skill evaluation

This small harness runs the configured Claude Code CLI against disposable local
fixtures. It uses the native `Skill` tool and a local MCP adapter for file access,
literal search and fixed Node checks. It does not configure accounts or models.

## Offline tests

```bash
python3 -m unittest discover -s scripts/skill-eval -p 'test_*.py' -v
```

These tests only launch the local Python MCP adapter. Importing the runner or
running without opt-in must not resolve or execute an installed agent CLI.

## Authorized real runs

Real runs consume the existing Claude account's quota. They require
`MULTICA_RUN_REAL_AGENT_SMOKE=1` before executable lookup or account access.
Use a current sanitized native `system/init` inventory containing only `skills`
and plugin `source` fields; this lets the session disable unrelated host skills
and plugins. Never supply an authentication or user configuration file as inventory.

```bash
MULTICA_RUN_REAL_AGENT_SMOKE=1 python3 scripts/skill-eval/runner.py \
  --skills server/internal/service/builtin_role_skills \
  --inventory /path/to/sanitized-host-inventory.json \
  --output /path/to/local-evidence \
  --phase patched-v1 --case P01_requirement --repeat 2
```

`--cases` defaults to the adjacent `cases.json`. Each case contains its raw prompt,
fixture files and optional fixed Node check arguments. Expected skill identifiers
are evaluation metadata and are never copied into the model's fixture. The runner
does not grade final prose. Review actual outputs independently.

Use a new `--phase` for another run: existing snapshots and attempts are never
overwritten. A phase copies the complete skill tree, including references, and
records hashes. Each repetition receives a fresh fixture. Maximum concurrency is
two; the default per-case timeout is 240 seconds. Node checks currently use the
macOS `sandbox-exec` path; all other fixture operations use standard Python.
A failed native invocation stops queued cases; cases already running remain
bounded by their timeout. The phase result records the cases that were not run.
An incomplete phase exits with status 1 after preserving its evidence.
The runner currently supports Claude Code only and has no automatic provider
fallback. Reusing these fixtures in a future Codex run requires an explicit
native invocation adapter and fresh execution evidence.

## Requirement clarification regression review

Keep `P01_requirement` and `G02_csv_targets` as CSV regression cases with their
original prompts and fixture files. Review their actual requirements, questions
and safety claims against [CSV export safety](references/csv-export-safety.md).
The reference is reviewer-only material: it is not part of any distributed role
skill, and the runner does not copy it into a model's fixture or skill snapshot.

The requirement-clarification skill keeps the general rules for evidence-backed
advice and explicit unknowns. CSV-specific facts belong to this review material
or the task's own relevant sources, rather than the default role skill. A model
without enough evidence should leave a safety claim unverified; matching wording
from the reference is not an acceptance criterion.

## Search evidence and boundaries

`search` is read-only, literal and case-sensitive. Its `paths` are explicit
fixture-relative files or directories, not globs; directories recurse. Symlinks
and `..` traversal are rejected. File identity deduplication prevents case aliases
or hardlinks from inflating counts; different files on a case-sensitive filesystem
remain distinct. Results contain real path/line/text/SHA-256
evidence plus searched files, byte/file/result limits, skips, `complete` and
`truncated`. An empty result with incomplete coverage does not prove absence.
The tool searches current files after edits; it does not classify retained
history or deprecated mentions or fabricate a desired zero-match result.

The model receives no general shell, web, agent or external MCP tools. File tools
cannot leave the fixture, and case variants of the reserved `.claude` tree cannot
be overwritten. Fixed Node commands use clean environments, immutable
execution-input hashes, denied network access and fixture-only writes. System
runtime reads remain available; account/home data and the real workspace volume
are denied, apart from the existing Node installation. No credential values are
copied to subprocess arguments, fixtures or evidence.

Inspect each run's `native-events.jsonl`, `fixture-tool-events.jsonl`, final answer,
file manifests and changed files. A successful native process is not proof of
semantic correctness, role compliance or improved performance over another language.
