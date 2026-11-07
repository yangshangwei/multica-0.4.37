# Execution

1. Record baseline screenshot/layout evidence and reproduce Close hit-testing in an owned Electron fixture if available. Keep read-only native evidence separate from Chromium interaction evidence.
2. Implement expanded-only flex growth with description as sole grow item, preserving all nodes/state. Fix proven close hit boundary locally; test actual event behavior and native region property as appropriate.
3. Run focusedcreation/AI/focus suites and typecheck/scopedlint. No new tests mirroring arbitrary classes; browser asserts textarea height growth and lackofunusedbottomspace.
4. One batched actualcomponentbrowser/Electron verification: screenshot901x1042,1280x900,390/320both themes; empty/longtext, openadvanced/AIquestions/files, toggle-retention, Close/draft/reopen, pointer+keyboard. Persist visualverdictbefore any correctionbatch; confirmonce.
5. Independenttrellis-check and final scoped suites serially, Views typecheck/lint, Impeccablelayoutdetector, exactsourceprovenance, cleanupownedresources. Syncspec, scopedcommit, archive.

pnpm --filter @multica/views exec vitest run collaboration/creation-form.test.tsx collaboration/creation-description-assist.test.tsx collaboration/focus.test.tsx collaboration/attachment-picker.test.tsx modals/issue-description-assist.test.tsx modals/issue-description-assist-editor.test.tsx --maxWorkers=1
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
