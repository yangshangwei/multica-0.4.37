# Committed source disposition

Target baseline: e541cfb43. All local branch tips and detached heads now have independent archival refs (takeover-preservation-manifest.json).

- 28 unique non-merge commits are patch-equivalent (`git cherry`); skip reapplication. Branch tips remain preserved, not deleted or rewritten.
- c90bc965a is an empty baseline; no product delta.
- fc920522f,5696ee916,d6a70ed29,835e7b046,cc530a558,6d2be85fb,f033051ca only change .trellis task/spec/research/journal paths. Preserve them in original branches+backup refs. They bind historical experiments/acceptance evidence and are not current product increments; do not transplant old task status or execution authority into this merge task.
- 4aa77e08e recipient picker E2E repair is semantically superseded: current test retains NEXT_PUBLIC_API_URL fallback, page-level portalled Recipient listbox, Assign to me row, Filter options input and exact lookup footer, and adds scoped popup/creation/viewport assertions.
- 32b741d0a context experiment harness is superseded by current context runner/fixture/scoring and strict source identity tests. Its scenario/evidence contracts remain current; the separately uncommitted E5 early-stop guard is integrated from context-guard source.
- 0041aa4b2 is a merge aggregate of recipient history, not an extra feature patch; its effective product path is covered by the recipient branch decisions.
- 92bb6d86b native verification snapshot:176/178 non-.trellis changed-path blobs occur in target history at the same product path. The two remaining files are handled below.

## Native snapshot remaining files

`.github/workflows/collaboration-host-native-verification.yml`: archive, intentional branch-only workflow whose own first line excludes product-branch merge. It triggers only the old verification branch.

`e2e/fixtures/collaboration-host-runtime.ts`: superseded. Current fixture retains source host execution flows and adds exported shared fixtures, knowledge observation, full squad/input author context, provider action receipts, workdir file hashes, owned admission-gate open/restore and stricter environment bindings. Source reapplication would remove those behaviors. Keep current fixture and current native-script recovery requirements.

No whole-branch cherry-pick is warranted. This task's actual changes come from uncommitted working-tree deltas, not these preserved divergent histories.
