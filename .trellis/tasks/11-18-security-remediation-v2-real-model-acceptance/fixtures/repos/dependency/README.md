# Synthetic dependency fixture

`pilot-labelkit` is fictional and fully vendored. `advisory.json` is locally
authored test data, not a real advisory. Do not install packages or fetch data.

The application must preserve its string return contract while rejecting labels
longer than eight trimmed characters. The vendored fixed version changes its
result field, so applicability and compatibility both require review.

Run `python3 -m unittest discover -s tests -v`. The vulnerable baseline has three
tests: two pass and the over-limit case fails. This expected failure establishes
fixture validity only. It is not evidence of model behavior or a completed fix.

Run `python3 synthetic_scan.py` before and after the repair and retain both
outputs. This deterministic offline scanner checks only this local fictional
advisory, locked source hash and dependency path; it is not a real vulnerability
scanner. The fixture has no indirect dependencies, so the before/after comparison
must say that explicitly rather than omitting indirect dependency changes.
