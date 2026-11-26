# Research: CLI01-v2 closure and Linux-only preparation

- Query: Targeted recheck of CLI cleanup blocker and exact Linux-only preparation request.
- Scope: internal, static review; no adapters executed.
- Date: 2026-11-25

## Findings

**Full CLI01-v2 accepted, no remaining blocker.** Request `12bee39797306f955d2dfa79d4c951eec113dc13f0fef65554fe5fa839383352`; executable decision in `native-cli-review-cli01-v2.json`.

`run_cli.py:63–102` now handles cleanup errors per child, handles process-exit races, bounds15s/5s waits, closes logs independently, continues Docker/gate/source/listener cleanup and always publishes terminal receipts through its outer finally. Gate epochs are recorded and restored booleans checked. The API executable is newly pinned and matches. All other request bindings match; original ten operations,240-second deadline and zero retries remain. This approves actual execution, not a predicted business PASS.

**Linux preparation accepted independently of macOS protocol cleanup.** Request `27e6f02466642e28c5bbef124da376f065d81cfaf380399d5bdba7f7abfc5b6e`; decision in `native-linux-package-review.json`. The small new wrapper calls only the already-reviewed Linux build branch and always records its terminal disposition. It does not invoke macOS protocol capture/restore or prepare a Mac app. All declared hashes and cached Electron39.8.7 executable/version match. Main launch remains separate.

## Files found / related specs

- Native `cli-review-request.json`, `run_cli.py` — repaired CLI1 and exact original journey binding.
- Native `linux-package-review-request.json`, `prepare_linux.py` — narrow Linux-only entry into unchanged accepted package preparation.
- Previously reviewed guard/cleanup/freeze/package modules — declared hashes unchanged except the intended runner/wrapper.
- Parent native/CLI reviews and verification-environment/E2E isolation contracts — applicable scope and required actual outcome evidence.

## Caveats / Not Found

- No source edits, tests, Docker/API/SQL operations or adapters executed. Research outputs only; parent may copy exact accepted JSONs.
- No optional research or broader review performed. Both decisions are restricted to the recorded requests and bytes.
