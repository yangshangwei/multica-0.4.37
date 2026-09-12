# Execution and verification

- [x] Preserve baseline skill snapshots and original failing prompts/results.
- [x] Verify CSV facts against official references; revise three skill bodies,
  behavior versions and the repository spec.
- [x] Add bounded literal search and test file scopes, symlinks, line evidence,
  limits, searches after edits, complete reference delivery and account opt-in.
- [x] Preserve the three attempted baseline calls: all returned provider HTTP 402
  before model tool use. Stop real calls following the user's deferral.
- [x] Independently review skill/CSV changes and the harness; fix the two
  filesystem-alias findings and verify both macOS and Linux behavior.
- [x] Run focused Go/template, frontend presentation, Python, formatting,
  type and static checks. Keep unrelated full E2E failures out of this revision.
- [x] Update the three authorized skills in workspace 毕宿五, add the CSV reference,
  and reopen all content to verify persistence.
- [ ] Record final evidence, commit with Lore trailers, and archive this revision
  child. Keep the parent validation task in review.

## Deferred to the later Codex evaluation

Implement a native Codex adapter when the user resumes testing. Run baseline and
revised skill snapshots with the same model/tools, repeat original and new cases,
and run ordinary negative controls. Review actual facts, search evidence, safety
advice and role boundaries independently. Preserve failures and raw native events;
do not report content problems eliminated before that review.
