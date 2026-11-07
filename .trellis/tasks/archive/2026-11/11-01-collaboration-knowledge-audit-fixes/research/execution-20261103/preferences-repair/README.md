# Restore previously verified dialog waits

Patch restores exactly three waits from304f74dd3 /118fc53ea and no other bytes. `git apply --check` passes against85ef5d205. All five owning product files match v11; the same v11 browser case passed.

No source was changed and no test was rerun. Root must independently review and replay the original case on the corrected candidate; all original timeouts, ordinary clicks and persistence/API assertions remain.
