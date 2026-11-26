# Provider review — 2026-11-27

Scope: all files under `server/internal/materialgit/`, checked against the approved requirement/plan workflow and provider contract. No real repository writes, account access, new dependencies, or database use.

## Fixed

- The resumed implementation had changed `Provider.Merge` to `MergeExpectation`, while six test calls retained the old signature. The package initially did not compile. Updated the calls to carry the frozen review number, source/target branches, HEAD and provider-specific changed-file observations; formatted `provider.go`.
- Added `TestMergeRechecksApprovedTargetAndFiles`: eight GitHub/GitLab cases reject retargeting or an extra file both before the write and when consuming the final merge receipt. The latter cases explicitly retain one actual remote merge effect while returning failure, so callers cannot promote that receipt to a baseline.
- GitHub's `protected` branch flag covers rulesets as well as classic protection. The old code required a successful classic protection lookup whenever that flag was true, permanently blocking ruleset-only branches. It now accepts a classic-protection 404 only after validated active rules and a separate, correctly scoped GraphQL ref return an explicit `branchProtectionRule: null` without errors. Missing fields, wrong/missing refs, permission errors, REST 403 and absent active rules remain blocked.
- Corrected the existing ruleset fixture to use `protected=true`; added nine absence-proof scenarios. The positive cases failed with `native_protection_unavailable` before the fix and passed afterward. README documents the final merge contract and ruleset-only behavior.

## Verification

- `go -C server test ./internal/materialgit -run TestMergeRechecksApprovedTargetAndFiles -count=1 -v` — passed all eight cases.
- `go -C server test ./internal/materialgit -run 'TestGitHubRuleset' -count=1` — demonstrated two expected failures before the ruleset fix, then passed after it.
- `go -C server test -race ./internal/materialgit -count=1` — passed after all changes.
- `go -C server vet ./internal/materialgit` — passed after all changes.
- `gofmt -l server/internal/materialgit/*.go` — no output.
- Type checking is included in the Go compilation above. Frontend lint/typecheck are outside this lane and owned by the main session.

## Remaining boundaries

No unresolved provider code finding within the agreed interface. Existing intentional limits remain: unknown policies and bypass allowances are refused; ambiguous GitLab branch-only creation requires operator resolution; GitLab commits do not offer branch-head CAS; merge APIs fence source HEAD rather than an atomic target snapshot. The service must retain serialization, current authority checks and post-merge byte verification. HTTP fixtures prove adapter contracts, not compatibility with live GitHub/GitLab installations, real credentials or every self-hosted version.

The main session owns synchronizing the provider signature in `contracts.md` and the service caller/fake. These were reported across lanes; this reviewer did not edit those files.

## Primary references

- [GitHub branch API](https://docs.github.com/en/rest/branches/branches): protected branches include branch protection and rulesets.
- [GitHub GraphQL Ref](https://docs.github.com/en/graphql/reference/git#ref): ref identity, nullable classic branch protection and active ruleset fields.
- [GitHub branch protection API](https://docs.github.com/en/rest/branches/branch-protection): classic protection endpoint and permission requirements.
