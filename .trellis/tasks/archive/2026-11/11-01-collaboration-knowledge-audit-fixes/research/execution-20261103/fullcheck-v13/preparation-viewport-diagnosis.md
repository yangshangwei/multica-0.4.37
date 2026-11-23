# Preparation viewport failure: offline diagnosis

The failed capture is **legacy/wide (1280×1000)**. It is not caused by an unexpanded disclosure: the legacy run records are open, and the Not recorded / No business result / Termination unconfirmed assertions passed before the capture.

The original trace records two scrolling ancestors. After the hash navigation (`call187`), document HTML scrollTop is **220** and the account-detail root is **6715**. After the capture's manual detail-root adjustment and two animation frames (`call203`/`call205`), HTML remains **220**, detail becomes **7258**, and the intersection ratio is repeatedly **0**. The screenshot shows the selected preparation section above the viewport while later handoff/usage content is visible.

The capture currently changes only `root.scrollTop` using element and root client rectangles. It does **not** call native `scrollIntoView` across scroll ancestors. This is a demonstrated coordinate mismatch at capture time, not evidence that the details were closed. The source's deep-link effect also adjusts only the account-detail root and uses focus with preventScroll.

The available trace does not persist the target/root client rectangles, because the assertion occurs before `boundingBox()`. It therefore does not establish whether the user-visible deep-link was already misplaced before the test's own scroll. A fresh directed replay should record document/root/target geometry on both sides of the hash jump and capture adjustment, verify the target without test scrolling, and retain the ratio1 assertion. Do not label this a product regression or a stale-selector repair without that comparison.

No browser, database, source change or replay was performed in this bounded diagnosis. Exact trace/source hashes and snapshot references are in preparation-viewport-diagnosis.json.
