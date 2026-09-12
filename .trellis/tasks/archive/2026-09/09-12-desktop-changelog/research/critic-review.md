# Critic review

Date: 2026-09-13. Scope: sequential review after `architecture-review.md`, against the current `prd.md`, `design.md`, and `implement.md`. No implementation or repository-wide exploration was performed.

**Verdict: APPROVE.** The current plan is sufficiently grounded, bounded, and testable for implementation. No planning blocker remains.

## Verified clarifications

The source-identity ambiguity identified during review is resolved in the current design. A feed can contain entries from at most one fork repository; both the generator and Go reader reject mixed-fork history, and generation checks every fork identity against `--repository`. Attributed upstream entries may coexist. T1/T2 explicitly include the rejection tests. This makes the page's latest-stable selection consistent with R3–R4 without a new API field.

Both architecture corrections are present: the entire read/validate/snapshot operation is serialized, and offline targets invoke the complete publisher through the already-loaded frontend image with `--pull never`. T1 now explicitly requires a controlled concurrent-read/replacement regression and `go test -race ./internal/changelog`; T2 includes first publication, prerelease, stable release, and older-version retry retention. These amendments were re-read before this verdict.

## Acceptance and implementation checkpoints

- The offline fixture must exercise writable destination permissions and a stable deployment directory, as the Architect requested. A stub proving only that Docker was invoked would not prove successful atomic publication by the image's configured user.
- Keep the existing `first release → prerelease → stable → retry an older tag` sequence in release-helper acceptance. A retry must not become a shortened source of cumulative history or silently replace an immutable published asset.
- Complete T7's unchanged-process, generated-file-to-mounted-reader check. Query-option assertions, mocked fetches, or a backend restart are insufficient substitutes for the stated 60-second freshness requirement.

## Approval rationale

1. **Objective coverage:** R1–R10 and A1–A8 cover the Help destination, reference-inspired reader, honest official/fork content, installed/server/published version distinctions, refresh and degraded states, commit generation, cumulative publication, and Trellis completion evidence. The required web wrapper repairs the shared Help destination; it does not expand the product scope arbitrarily.
2. **Principle/option consistency:** Deployment-owned delivery directly supports intranet access and live refresh. The Architect fairly presents the strongest embedded-only alternative and its lower operational cost. Its inability to update an unchanged server explains why the mutable file handoff is justified. External fetch and CMS alternatives have concrete additional costs rather than being dismissed without consideration.
3. **Testability:** The plan names canonical test layers, temporary Git fixtures, strict producer versus tolerant consumer validation, real publication-to-reader acceptance, and workflow-graph inspection. Acceptance does not depend on production credentials, a real release tag, or a public network at client runtime. Actual environment limitations must be recorded without relabeling an unexecuted check as passing.
4. **Ownership and sequencing:** A/B/C have bounded write scopes; D owns seed, deployment integration, and final acceptance. Shared server wiring, package exports, documentation, and dirty-file handling have explicit owners and handoffs. Generate-once artifact delivery precedes all consuming builds; public notes follow required build success.

No broader redesign, new dependency, CMS, mobile work, binary-updater migration, or production publication is requested by this review. This is a planning verdict, not evidence that the feature works.
