"""Reject missing, skipped, failed, duplicated, or stale-source test evidence."""

import hashlib
import json
from pathlib import Path


HERE = Path(__file__).resolve().parent
ROOT = next(parent for parent in HERE.parents if (parent / "server/go.mod").is_file())
CHECKPOINT = "TestCollaborationCheckpointAuthorityTransactionOrder"
WAKEUP = "TestCollaborationHandoffTwoMembersAndClaimBoundary"
CHECKPOINT_TESTS = {CHECKPOINT} | {
    f"{CHECKPOINT}/{action}/{first}_first"
    for action in ("stop", "revoke_grant", "author_leave")
    for first in ("checkpoint", "authority")
}

for name, expected in (
    ("checkpoint-first", CHECKPOINT_TESTS),
    ("wakeup-first", {WAKEUP}),
    ("matrix-race", CHECKPOINT_TESTS | {WAKEUP}),
):
    events = [json.loads(line) for line in (HERE / f"{name}.jsonl").read_text().splitlines()]
    assert (HERE / f"{name}.exit").read_text().strip() == "0", name
    assert not any(event["Action"] in ("fail", "skip") for event in events), name
    for action in ("run", "pass"):
        names = [event["Test"] for event in events if event["Action"] == action and "Test" in event]
        assert set(names) == expected and len(names) == len(expected), (name, action, names)
    assert len([event for event in events if event["Action"] == "pass" and "Test" not in event]) == 1, name
    assert not any("no tests to run" in event.get("Output", "") for event in events), name

source = json.loads((HERE / "race-source.json").read_text())
for filename, digest in source["test_sha256"].items():
    assert hashlib.sha256((ROOT / filename).read_bytes()).hexdigest() == digest, filename
assert (HERE / "service-vet.exit").read_text().strip() == "0"
assert (HERE / "gofmt.exit").read_text().strip() == "0"
print("PASS: exact run/pass inventory, zero skips/failures, and tested source hashes")
