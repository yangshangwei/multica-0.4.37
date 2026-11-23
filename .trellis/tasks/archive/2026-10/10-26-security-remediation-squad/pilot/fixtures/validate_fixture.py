"""Validate fixture shape and local hashes, not security closure or provenance."""

import ast
import hashlib
import json
from pathlib import Path


root = Path(__file__).resolve().parent
advisory = json.loads((root / "advisory.json").read_text())
lock = json.loads((root / "dependency.lock.json").read_text())
supply_chain = json.loads((root / "supply-chain.json").read_text())

for manifest in (advisory, lock, supply_chain):
    if manifest["synthetic"] is not True:
        raise ValueError("Every manifest must explicitly identify synthetic material")

for release in advisory["releases"].values():
    source = root / release["source"]
    if source.parent != root / "vendor" or source.suffix != ".py":
        raise ValueError("Dependency source must stay in the local vendor directory")
    if hashlib.sha256(source.read_bytes()).hexdigest() != release["sha256"]:
        raise ValueError(f"Local source digest mismatch: {source.name}")

release = advisory["releases"][lock["version"]]
if lock["name"] != advisory["package"]:
    raise ValueError("Lock and advisory name different fictional packages")
if any(lock[key] != release[key] for key in ("source", "sha256")):
    raise ValueError("Lock source/digest does not match its selected local revision")

if any(supply_chain[key] is not None for key in ("artifact_sha256", "provenance", "sbom")):
    raise ValueError("The missing-evidence fixture must not manufacture attestations")
if (
    supply_chain["evidence_state"] != "unknown"
    or supply_chain["decision"] != "hold"
    or not supply_chain["next_owner"]
    or supply_chain["network_execution_permitted"] is not False
):
    raise ValueError("Supply-chain fixture must retain its hold and evidence owner")

sources = sorted(root.rglob("*.py"))
for source in sources:
    ast.parse(source.read_text(), filename=str(source.relative_to(root)))

print(f"Fixture structure: PASS ({len(sources)} Python files parsed; local source hashes match)")
print("Supply-chain conclusion: hold/unknown; local hashes do not establish producer trust")
