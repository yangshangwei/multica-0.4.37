# Keep requirement clarification independent of CSV evaluation fixtures

## Goal

Retain generic evidence rules, move CSV guidance to reviewer-only evaluation references, update tests and synchronize the approved workspace skill without altering custom content.

## Requirements

- Keep requirement clarification focused on open questions, observable acceptance
  criteria, evidence-backed risk analysis and work breakdown. Preserve its name,
  description, invocation policy, role bindings and authority boundaries.
- Retain the generic evidence rules and the sentence limiting this skill to
  translating risks into verifiable requirements; remove its CSV-specific routing.
- Move the CSV reference unchanged to `scripts/skill-eval/references/` for semantic
  reviewers. Do not include reviewer guidance in skill bundles or case fixtures.
- Preserve the original `P01_requirement` and `G02_csv_targets` inputs and all
  historical evaluation evidence. This edit does not authorize real-account tests.
- Increment only the requirement role-skill version from 2 to 3. Existing workspace
  copies still require an explicit update and must not be automatically overwritten.
- Back up and update the affected skill in the previously documented `aldebaran-96xe`
  workspace after confirming its identity and current content. Preserve custom
  content, metadata, bindings and unrelated files. Remove the old reference only
  after its content matches the known source and is retained in the backup.

## Cleanup Plan

1. Run existing role/template tests. Replace the CSV-specific delivery assertion
   with a generic source-to-bundle comparison across all role skills, including
   main content, relative paths, duplicate/missing files and exact contents. Run it
   before moving the reference to cover the current nonempty attachment bundle.
2. Remove the two CSV-specific lines, move the reference without changing its bytes,
   bump the skill version and update the current template spec and evaluation README.
   Do not change the loader, evaluation runner, fixture inputs or unrelated skills.
3. Run focused Go tests, `go vet`, formatting/diff checks, skill validation and the
   offline evaluation harness tests. Check that fixture inputs and reference bytes
   are unchanged and the shipped requirement skill contains no CSV attachment.
4. Back up the live copy, apply the same narrow change, save and reread the body and
   file list. Record the workspace update separately from code/test verification.

The independent read-only plan review accepted this scope and recommended source
enumeration rather than checking only delivered files, which could miss omissions.

## Acceptance Criteria

- [x] Requirement clarification retains all generic evidence and role boundaries.
- [x] New requirement-skill bundles contain no CSV reference or CSV-specific routing.
- [x] CSV reviewer guidance is preserved outside the distributed skill; original
      evaluation inputs and historical records remain unchanged.
- [x] Generic bundle tests pass before and after the move; role bindings stay intact.
- [x] Relevant Go/static checks and offline evaluation tests pass, with gaps recorded.
- [x] The identified workspace copy is backed up, narrowly updated and reread.

## Notes

- User approved the recommendation in the preceding investigation. No further design
  approval is needed for this bounded change.
- Other active work concerns intranet messaging and template-catalog presentation;
  do not alter, stage or revert those unrelated files or their task state.
- Local/static checks cannot establish model-output quality. Native real-model
  regression remains a separate, explicitly authorized evaluation.
