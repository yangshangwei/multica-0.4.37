#!/usr/bin/env python3
"""Reuse installed exact packages while keeping workspace aliases checkout-local.

Usage: python3 dependency-links.py SOURCE_INSTALLATION TARGET_CHECKOUT
No installation, package manifest, lockfile, or source node_modules is modified.
"""
import json
import sys
from pathlib import Path

source, target = (Path(value).resolve() for value in sys.argv[1:])
assert source != target and (target / ".git").exists()
packages = {
    json.loads(manifest.read_text())["name"]: manifest.parent.relative_to(source)
    for group in ("apps", "packages")
    for manifest in (source / group).glob("*/package.json")
}


def local_directory(path):
    # Remove only this checkout's alias, never write through a shared symlink.
    if path.is_symlink():
        path.unlink()
    path.mkdir(parents=True, exist_ok=True)


def external_alias(path, destination):
    if path.exists() or path.is_symlink():
        return
    path.symlink_to(destination)


def workspace_alias(path, destination):
    if path.is_symlink():
        if path.resolve() == destination:
            return
        path.unlink()
    elif path.exists():
        raise RuntimeError(f"Refusing non-symlink workspace dependency: {path}")
    path.symlink_to(destination)


for relative in [Path("."), *packages.values()]:
    installed = source / relative / "node_modules"
    if not installed.exists():
        continue
    modules = target / relative / "node_modules"
    local_directory(modules)
    for item in installed.iterdir():
        alias = modules / item.name
        if item.name.startswith("@") and item.is_dir():
            local_directory(alias)
            for dependency in item.iterdir():
                name = f"{item.name}/{dependency.name}"
                if name in packages:
                    workspace_alias(alias / dependency.name, target / packages[name])
                else:
                    external_alias(alias / dependency.name, dependency.resolve())
        else:
            external_alias(alias, item.resolve())
    # The root test runner also imports shared types; all workspace aliases
    # resolve to this checkout while external dependencies retain exact bytes.
    for name, package in packages.items():
        alias = modules / name
        local_directory(alias.parent)
        workspace_alias(alias, target / package)

proof = {}
for relative in [".", "apps/web", "apps/desktop", "packages/core", "packages/views"]:
    modules = target / relative / "node_modules"
    zod = modules / "zod/package.json"
    proof[relative] = {
        "zod": json.loads(zod.read_text())["version"] if zod.exists() else None,
        "core_alias": str((modules / "@multica/core").resolve()),
    }
print(json.dumps({"source_installation": str(source), "target": str(target), "proof": proof}, indent=2))
