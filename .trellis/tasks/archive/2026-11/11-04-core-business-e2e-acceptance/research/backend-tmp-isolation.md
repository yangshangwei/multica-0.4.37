# Backend test temp-root isolation incident

The reviewedv3 runner created per-package HOME/TMP underneath the original Git checkout's ignored .omx/private directory. Original execenv full package recorded655PASS/5FAIL. cursorProjectRoot walks parents to the first .git, so t.TempDir workdirs falsely belonged to the original repository; Git similarly found the original tree for the non-Git negative test. The failure log names the original root and an unrelated untracked-file budget.

Root's .cursor/mcp.json existed when observed, mtime inside this test run. Its hash/keys/metadata are captured privately; no pre-run snapshot exists, so prior contents are not asserted. No speculative cleanup occurred. This is test-runner environment contamination, not evidence of a product behavior regression.

Parent runner66973 was precisely paused after detection; the already-spawned slack package completed121PASS and is recorded separately. Full result history remains unchanged. v4 moves per-package HOME/TMP and vetHOME to an exact root-authorized directory outside all Git trees and actively rejects a Git-contained root. Explicit Go caches and all source/tool/DB/CLI protections remain unchanged. The failed execenv package needs a fresh root-allocated DB and full unchanged-assertion replay;39 unexecuted packages retain original unused allocations.
