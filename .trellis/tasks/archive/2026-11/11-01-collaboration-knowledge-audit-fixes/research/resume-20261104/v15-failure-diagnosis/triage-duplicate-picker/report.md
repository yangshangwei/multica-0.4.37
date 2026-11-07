# V15 triage duplicate picker diagnosis

The actual English failure is confirmed: Reason text appears in the duplicate-target combobox while the Reason textarea is empty. The initial empty-target assertion fails at fixture line111. Saved screenshot and DOM agree; the failure precedes pagination or duplicate submission.

The strongest hypothesis is a dialog initial-focus race, with medium causal confidence. Installed Base UI1.3.0 queues first-control focus through a microtask and requestAnimationFrame. Installed Playwright1.58.2 textarea.fill selects/focuses the textarea, then later sends keyboard.insertText. The fixture immediately fills another field after opening. Product reason and target states/setters are separate; labels/roles identify distinct controls.

This run contains no trace or focus/input timeline. Final combobox focus is observed after input.click. The actual focus-stealing event is not yet proven. No production fix or immediate test change is recommended during the shared run. Allfour relevant picker cases passed in v14, with relevant fixture/modal/picker/UI sources unchanged.

After the sole full window releases, retain original actions/assertions and enable trace plus scoped focusin/focusout/beforeinput/input/change and action-stage recording. Confirm whether textarea focus/select is followed by default combobox focus before insertText. If confirmed, the smallest fix belongs to the shared E2E fixture: await intended initial combobox focus before filling Reason and immediately verify Reason value. Keep every original empty-target, count/list, keyboard/focus, target identity, durable decision and history assertion.

No tests, DB/services/processes, source changes, commits or models were run by this diagnosis. Knowledge0/0/defaultfalse remain unchanged. report.json binds actual source/library/artifact hashes, the immutable failure excerpt and full diagnostic/control proposal.
