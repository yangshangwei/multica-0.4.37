"""Capture a bounded verification command and the concurrent worktree source."""

import datetime
import hashlib
import json
import platform
from pathlib import Path
import subprocess
import sys

root = Path.cwd()
out = Path(__file__).resolve().parent
label, *command = sys.argv[1:]
sources = sorted((root / "server/internal/daemon/collaborationrun").glob("*.go"))
sources += sorted((root / "server/pkg/protocol").glob("*.go"))
metadata = {
    "command": command,
    "cwd": str(root),
    "time": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "platform": platform.platform(),
    "database": "none; test-owned executables and HTTP/ledger fixtures only",
    "head": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
    "source_sha256": {
        str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest()
        for path in sources
    },
    "dirty_diff_sha256": hashlib.sha256(
        subprocess.check_output(["git", "diff", "--binary"])
    ).hexdigest(),
}
result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
(out / f"{label}.log").write_text(result.stdout)
metadata["exit_code"] = result.returncode
metadata["source_changed_during_check"] = [
    name for name, digest in metadata["source_sha256"].items()
    if hashlib.sha256((root / name).read_bytes()).hexdigest() != digest
]
(out / f"{label}.json").write_text(json.dumps(metadata, indent=2) + "\n")
print(result.stdout)
print(f"exit_code={result.returncode}")
sys.exit(result.returncode)
