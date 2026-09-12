# Grounded skill outputs

## Request

Apply the approved improvements to the three skills whose real Claude outputs
contained unsupported ADR background, incorrect CSV safety advice, or inaccurate
documentation summaries. Preserve successful skill discovery and role boundaries.

The latest user instruction is: "先完成修订，稍后切到codex复测". This child
delivers the revisions, workspace synchronization and local verification. Real
model regression is deferred to a later, explicitly resumed Codex evaluation.

## Acceptance criteria for this revision

- ADR instructions require evidence for background facts and constraints, keep
  missing facts unknown, and prevent framework conventions from becoming claims
  about the current project. Preserve draft status and observer permissions.
- Requirement clarification instructions separate known requirements from
  unverified implementation advice. CSV guidance distinguishes structural quoting
  from formula neutralization and links a verified, narrowly scoped reference.
- Documentation instructions require searches of the final files, with actual
  paths, lines and coverage limits. Distinguish obsolete recommendations from
  legitimate compatibility, history and test mentions.
- The real-agent fixture provides read-only literal search with explicit scope,
  complete/truncated reporting and safe path boundaries. Supporting skill files
  are included in evaluation snapshots and embedded role templates. Local tests
  must not access real agent accounts.
- Synchronize the three approved skill updates to the intended existing
  workspace copies, preserving unrelated content and files. Other workspaces
  and unrelated E2E failures remain outside this task.
- No new project dependencies. Keep skill names, discovery descriptions and
  unrelated role permissions unchanged; version material skill behavior changes.

## Deferred behavioral acceptance

Preserve the original failing prompts, new scenarios, negative controls, baseline
snapshots and all failed execution evidence. A later Codex evaluation must collect
native skill-loading evidence and independent semantic review. Compare baseline
and revised skills under the same Codex model and tools; previous Claude output
is not a controlled Codex baseline. Local checks and a successful process exit do
not prove that generated content improved. The current harness is Claude-only;
native Codex invocation still needs an adapter and fresh execution evidence.
