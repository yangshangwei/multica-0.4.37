# Execution

1. Run all regular/CLI/desktop/changelog cases with one worker and zero retries.
2. Correct confirmed fixture configuration and stale skill-library navigation; retain localization, no-write and keyboard/viewport assertions.
3. Rerun failures, then run device auth against the isolated API with that mode enabled.
4. Run the full suite again when failures are fixed, record exact results and limits, commit only owned test/report changes and archive.

Initial run: 107 passed, 6 failed, 1 mode-gated case. Four desktop cases lacked renderer CORS. Changelog new context lacked PLAYWRIGHT_BASE_URL. One localized-skill case referenced the replaced compact entry.
