# Windows renderer file URL comparison

Run45678396231 public evidence proves API/Web process identity and all three API probes passed (all flags true; HTTP200). The next failure is controller launch at line429, before boot/runtime/login. Existing launch compares URL.pathname against a raw platform path twice: initial window and reload. OnWindows pathname starts /D:/ and uses slash/percent encoding, while packagePath is D:\... with backslashes. This is a fixture assertion error; it does not prove a product loading defect.

Scope: retain file protocol check and all installed-file/hash/source validation. Both launch comparisons use nativeLayout.rendererUrl (normalized href) for nativeWindows/macOS; Linux container retains existing pathname expectation. Add a focused helper in the existing fixture only so the two sites share one condition and actual condition can be exercised through the established vm test pattern. No production code, timeouts, retries or business flow edits.

## Reproduction and verification

- Real `nativeInstalledLayout` reproduction on local Node shows `D:\a\_temp\installed\resources\app.asar/out/renderer/index.html` differs from URL.pathname `/D:/a/_temp/installed/resources/app.asar/out/renderer/index.html`. With spaces/literal percent/hash, URL additionally contains `%20`, `%2520`, `%23`; raw string comparison cannot succeed.
- Added existing-controller Node tests exercising the actual new assertion via vm (same technique already used for controller helpers). Before implementation the two tests failed; after implementation Windows plain and space/%20/# paths plus macOS encoded paths pass. Wrong HTML, wrong ASAR, foreign file host, query/hash and double-decoded percent paths are rejected.
- Linux `/opt/multica/resources/app.asar` and `/opt/Multica/resources/app.asar` keep exact pathname acceptance and reject a different renderer file. Initial explicit file: protocol assertion remains unchanged.
- Related installed-main/layout/windows-config/workflow suites:31PASS/0FAIL/0SKIP. `node --check` and `git diff --check`: pass.
- No Electron binary rebuilt or executed, no Windows GUI acceptance claim, no commit/push. Next full native Windows run must prove the actual page URL passes both first-window and reload checks.
