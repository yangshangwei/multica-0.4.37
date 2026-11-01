#!/usr/bin/env python3
"""Bind a real-model registration to the exact checked-out source.

Writes source-manifest.json next to this script: HEAD, git tree ids, the
tracked/untracked dirty state, and SHA-256 of every tracked file under the
product roots. source_id = SHA-256 over sorted "path\0sha256\n" lines, so a
later run can recompute it from the same checkout and compare.
"""
import hashlib
import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOTS = ["server", "apps", "packages", "scripts", "Makefile", "package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml"]


def git(repo, *args):
    return subprocess.run(["git", "-C", repo, *args], check=True, capture_output=True, text=True).stdout


def main():
    repo = sys.argv[1] if len(sys.argv) > 1 else "."
    out = Path(sys.argv[2]) if len(sys.argv) > 2 else Path(__file__).with_name("source-manifest.json")
    head = git(repo, "rev-parse", "HEAD").strip()
    files = [p for p in git(repo, "ls-files", "-z", "--", *ROOTS).split("\0") if p]
    sha = {}
    for path in sorted(files):
        full = Path(repo, path)
        if full.is_symlink() or not full.is_file():
            continue
        sha[path] = hashlib.sha256(full.read_bytes()).hexdigest()
    digest = hashlib.sha256()
    for path in sorted(sha):
        digest.update(f"{path}\0{sha[path]}\n".encode())
    dirty = git(repo, "status", "--porcelain=v1", "--untracked-files=all", "--", *ROOTS).splitlines()
    manifest = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "head": head,
        "branch": git(repo, "rev-parse", "--abbrev-ref", "HEAD").strip(),
        "tree": {root: git(repo, "rev-parse", f"HEAD:{root}").strip() for root in ["server", "apps", "packages"]},
        "roots": ROOTS,
        "source_id_method": "sha256 over sorted path NUL sha256 LF lines of tracked regular files under roots",
        "source_id": digest.hexdigest(),
        "file_count": len(sha),
        "dirty_product_paths": dirty,
        "overlay": "none" if not dirty else "dirty paths listed; registration is not source-bound until they are hashed separately",
        "sha256": sha,
    }
    out.write_text(json.dumps(manifest, ensure_ascii=False, indent=1) + "\n")
    print(json.dumps({k: manifest[k] for k in ("head", "tree", "source_id", "file_count", "dirty_product_paths")}, ensure_ascii=False))


if __name__ == "__main__":
    main()
