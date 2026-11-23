# Preferences failure: retained wait regression

The final candidate dropped exactly three waits present in `118fc53ea` / `304f74dd3`: wait for the New Issue dialog, wait for its required title editor focus, and wait for the Set priority menu item. Restoring them makes the entire file byte-identical to the known v11 source. All five owning production dialog/menu files are unchanged from v11, and the same v11 browser case actually passed.

In v13 the zero-count assertion can succeed before the manual panel settles. The menu item was briefly found, reported unstable, then detached; the click waited until the existing60s test budget expired. Trace `call1427` starts at1625710ms and the fixture's failure screenshot at1683028ms still shows New Issue on the workspace issues URL. The later `/setup` error-context follows fixture cleanup; it is not evidence of an earlier authentication redirect. The trace does not capture the exact focused element, so the animated Switch to Agent border is not used as focus evidence.

Recommendation: restore the original three condition-based waits, preserving ordinary clicks, all time budgets and persistence/API assertions. No force click, sleep or timeout increase. A new directed run must verify the restoration. The patch artifact is under `../preferences-repair/`; it has not been applied.

Original trace, extracted failure screenshot, four screencast frames, source diff, five production hashes and the retained v11 result are indexed in diagnosis.json. No browser, DB or source mutation ran in this diagnosis.
