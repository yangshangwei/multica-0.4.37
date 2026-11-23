# V9 final catalogue failure diagnosis

The authoritative final JSON contains **13 catalogue-related failures out of 16**. The same two task-environment preparation gaps are now corroborated by complete errors, source and filesystem evidence. V9 remains failed: 334 pass/16 fail/89 skip, 0 flaky. No original result was reclassified.

## Actual causes

**BCD-1 — missing configured managed publication root.** Actual v9 configuration sets `MULTICA_RESOURCE_PUBLISH_DIR` to task-owned `catalog/published`, which did not exist when diagnosed. Candidate 64a `ResourcePublisher.openRoot` requires an existing real directory (`resource_store.go:49–68`); consumer merging calls it before reading the managed index (`resource_catalog.go:147–155`). Both Skill and MCP catalogues intentionally propagate this error and suppress embedded rows. The API evidence shows **48 MCP catalogue GET 503 and 19 Skill catalogue GET 503**. All 10 apparent 200s are OPTIONS preflights, not successful reads.

The final localized-specialist, skill-market and independent skill-copy cases fail directly on `/api/skills/templates` with 503 at lines 229, 114 and 145, before their intended journeys. The capability 3, MCP's other 4, stale-session and workspace-defaults cases wait for missing templates/categories/cards. Retained Skill/MCP DOM snapshots show catalogue loading errors. This source/error/GET evidence supports a setup defect; no product fail-closed behavior or assertion should be weakened.

**BCD-2 — missing deployment MCP fixture parent.** `E2E_MCP_TEMPLATE_DIR` and API MCP directory point to the same absent `catalog/mcp`. The final deployment MCP case has an actual ENOENT/mkdtemp error. `e2e/fixtures/mcp-deployment.ts:17–19` creates a temporary child before its try/finally and assumes its parent exists. Missing manual directories alone are treated as empty by catalogue readers; that fact does not explain the other built-in failures. The configured missing managed root does.

## Minimal setup remedy and replay

V9 is now complete; preserve its original failed database/logs/JSON. Before a fresh owned API/browser replay, prepare and record private owner/mode/inode identities for three separate directories: manual MCP, manual Skill and managed published storage. API/test MCP paths must match. Keep managed publication configured; use the existing public workspace fixture that requires an active internal organization, rather than inventing SQL seed data or changing auth flags.

Require actual catalogue GET 200 and expected built-in content before replay. Re-run the 13 exact failing catalogue titles from `capability-market.spec.ts`, `mcp-market.spec.ts`, `missing-user-session.spec.ts`, `localized-template-defaults.spec.ts`, `skill-market.spec.ts`, `skill-template-creation.spec.ts` and `workspace-defaults.spec.ts`, one worker/zero retries/raw JSON. The skill-copy case currently asserts 15 templates; preserve that and the 16 MCP recipes/source/retention/visibility checks. Serialize shared catalogue mutation with native MCP/admin publication cases.

No product-source patch is justified by these errors. If initialized setup still returns 503 or reveals another assertion failure, diagnose that new evidence separately. Targeted successes cannot convert the old v9 run into complete acceptance.

## Scope and limits

Actual organization rows and precise internal API error branches were not queried. Public workspace creation requires an active organization before commit, and the absent configured root independently fails. The stale-session case failed at the final Playwright-template button after earlier login/route/tab assertions; final later auth-error assertions were not reached. Some DOM artifacts are empty after cleanup, so their exact disappearance mechanism is unknown.

The unrelated memory/settings/invitation failures are excluded and remain assigned to other root-owned slices. No tests, database/HTTP/API/server/CLI/Provider execution, source/env/catalogue changes or commits ran here. Only this report and [browser-catalog-diagnosis.json](browser-catalog-diagnosis.json) were written. Raw errors and credentials remain in the private authoritative diagnostics; the report records their hashes and sanitized categories.
