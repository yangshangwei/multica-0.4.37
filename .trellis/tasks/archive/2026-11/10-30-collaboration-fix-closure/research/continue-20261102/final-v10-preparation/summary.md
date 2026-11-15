# V10 complete-check preparation

Prepared the private launcher and data-only configuration on clean `118fc53ea8eb8ebc53b4f9c1131e3e7910eacce9`. The Go/pnpm v9 adapters and the inherited parser functions are unchanged. Only six source-env paths changed; three fresh catalog directories and a Git-tree-external build directory are owned mode0700.

[Preparation JSON](preparation.json) contains every source/launcher/adapter/artifact hash, allocated path and the root execution command. [Fake preflight](fake-preflight.json) passed 32/32 with 0 fail/error/skip. [Offline checks](static-checks.json) passed syntax, hash, configuration, path and equal-product-tree checks. Prior v9 evidence remains preserved.

Root still owns independent clearance, installing staged renderer/preload into the checkout, live renderer/API/catalog/database readiness, the shared service window and one complete `make check`. No actual clearance, run record, HOME/registry/test database state, Go TestMain, build, repository test, service, network, Provider call or commit was created by this preparation. Freeze these selected code/configuration bytes before independent review and execution.
