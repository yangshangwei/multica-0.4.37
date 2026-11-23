# Resource catalog configuration: read-only verification

The shared v6 run stopped in `cmd/server` with two top-level skill-catalog
scenarios returning503. This note verifies the source boundary requested by the
sole execution owner; it performs no Go tests, SQL, services or configuration
changes. The actual failed run remains in `shared-check-v6-result.json`.

- `server/internal/handler/handler.go:573` creates `ResourcePublisher` from
  `cfg.ResourcePublishDir`; its organization getter at581 calls
  `GetInternalOrganization` and rejects missing/inactive organization.
- `server/internal/service/skill_template_dir.go:54` reads embedded/mounted
  templates, then only enters the managed publisher when `Root` is nonempty.
  `resource_catalog.go:356` resolves organization before reading managed skill
  bundles. MCP's managed path does the same at402. The missing manual directory
  case at41 deliberately returns an empty list; it alone is not this503 cause.
- `server/cmd/server/main.go:415` runs `preparePlatformOrganization` only for
  password mode with platform administration enabled. The prepare function at
  `platform_admin.go:88` ensures the organization. Merely setting a publication
  directory in a legacy/admin-disabled API does not run this startup step.
- The real human workspace-create handler nevertheless calls
  `EnsureInternalOrganization` in its transaction at `workspace.go:256` before
  `CreateWorkspace`; device onboarding also ensures it. The default router
  fixture at `cmd/server/integration_test.go:100` inserts user/workspace/member
  rows directly, bypassing these real producer entry points.
- Real browser gates depend on the catalog contract: `skill-template-creation`
  reads the actual template API and requires15 entries at145;
  `skill-market:114` requires the real catalog to be nonempty before adding its
  bounded interception. The native MCP baseline keeps its actual built-in
  template expectations. Global removal of the API's publication/mount
  configuration would invalidate separate publication/catalog coverage.

The source-backed correction is to keep API/non-test/compile configuration and
explicit test-owned recursive overrides intact, while the outer ordinary-Go
package environment does not inherit the API-only publication directory. Each
publication-specific test remains responsible for its own configured fixtures
and internal organization. Do not add a synthetic organization to the failed
default fixture or weaken catalog assertions merely to relabel this run.

Fresh dedicated-database red/green evidence, exact new adapter/launcher hashes,
independent review and a new complete check remain the execution owner's work.
This note proves source wiring only, not the correction or any passing run.
