"""Offline fixture scanner, not a real CVE feed, SBOM or provenance attestation."""

import hashlib
import json
from pathlib import Path


def scan(root):
    lock = json.loads((root / "dependency.lock.json").read_text())
    advisory = json.loads((root / "advisory.json").read_text())
    source = (root / lock["source"]).resolve()
    if source.parent != (root / "vendor").resolve():
        raise ValueError("Synthetic dependency must be vendored locally")
    actual = hashlib.sha256(source.read_bytes()).hexdigest()
    if actual != lock["sha256"]:
        raise ValueError("Locked source hash mismatch")
    return {
        "synthetic": True,
        "scanner": "local-fixture-only-v1",
        "advisory_id": advisory["id"],
        "package": lock["name"],
        "resolved_version": lock["version"],
        "source_sha256": actual,
        "affected_by_fixture_advisory": lock["version"] in advisory["affected_versions"],
        "dependency_path": ["application", lock["name"]],
        "indirect_dependencies": [],
        "limitations": "Only the local synthetic advisory and lockfile; no real vulnerability coverage or producer trust assertion",
    }


if __name__ == "__main__":
    print(json.dumps(scan(Path(__file__).resolve().parent), indent=2))
