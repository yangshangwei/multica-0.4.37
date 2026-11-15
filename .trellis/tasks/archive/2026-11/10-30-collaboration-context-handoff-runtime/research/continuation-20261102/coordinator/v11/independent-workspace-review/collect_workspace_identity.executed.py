#!/usr/bin/env python3
"""Read-only stdlib collector; never executes project code or child processes.

Run once from the coordinator's working shell and capture stdout in this review
directory. The reviewer authors the procedure and analyzes the returned facts.
No candidate, dependency, Git, environment, service, or network mutations occur.
"""

import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys


COORDINATOR = Path("/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261102/coordinator")
CANDIDATE = Path("/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103")
EXPECTED_HEAD = "304f74dd3ba4ddea862c94917c5bb0770355a351"
INPUTS = {
    "expectations": (COORDINATOR / "v11/workspace-identity-expectations.json", "579f69cdd7c561a9d65c20fa21cde1443092696fcef860c8a315ce9ba7daf49d"),
    "observation": (COORDINATOR / "v11/workspace-links-observation.json", "f90d90b40111034a67cb7b4b71aa6fecb62a5c61b315f99c589dfcf4e7359191"),
    "diagnosis": (COORDINATOR / "v10/category-timeouts/20261103-readonly-diagnosis.json", "2b7be3a02c890f6afe6aec419a76b228e9261801b15e32c02bc791c447af89a5"),
    "contract": (COORDINATOR / "v10/workspace-identity-contract.md", "89e1498470ec3117d6c8bf53d379866dd781bd59864250880bdcdf8e27ff0d2b"),
}
PRUNED = {"node_modules", ".git", ".next", "dist", "out", "build", ".turbo", "coverage", "test-results", "playwright-report", ".expo", "Pods"}
SOURCE_SUFFIXES = {".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".json", ".jsonc", ".css", ".yaml", ".yml", ".md", ".mdx"}
SHA_RE = re.compile(r"(?<![0-9a-f])[0-9a-f]{64}(?![0-9a-f])")
SAFE_CODE_PATH_RE = re.compile(r"/Volumes/artisan/code/2026/[A-Za-z0-9._/@+%()\[\]:-]+")
IMPORT_RE = re.compile(r"(?:from\s*|import\s*\(|require\s*\(|import\s*)[\"'](@multica/[^\"']+)[\"']")


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def error_name(exc):
    return type(exc).__name__


def sha_file(path):
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while True:
            chunk = handle.read(1024 * 1024)
            if not chunk:
                break
            digest.update(chunk)
    return digest.hexdigest()


def json_file(path):
    return json.loads(path.read_text(encoding="utf-8"))


def fingerprint(path):
    result = {"path": str(path), "exists": path.exists(), "lexists": os.path.lexists(path)}
    if not result["lexists"]:
        return result
    try:
        info = path.lstat()
        result.update({"is_symlink": stat.S_ISLNK(info.st_mode), "realpath": str(path.resolve()), "lstat_device": info.st_dev, "lstat_inode": info.st_ino})
        if result["is_symlink"]:
            result["link_target"] = os.readlink(path)
        if path.is_file():
            result.update({"bytes": path.stat().st_size, "sha256": sha_file(path)})
        if path.exists():
            followed = path.stat()
            result.update({"device": followed.st_dev, "inode": followed.st_ino, "is_directory": path.is_dir()})
    except (OSError, RuntimeError) as exc:
        result["read_error"] = error_name(exc)
    return result


def inside(path, root=CANDIDATE):
    try:
        path.resolve().relative_to(root.resolve())
        return True
    except (ValueError, OSError, RuntimeError):
        return False


def candidate_path(value):
    path = Path(value)
    return path if path.is_absolute() else CANDIDATE / path


def safe_json_inventory(value):
    """Return only keys, code paths, hashes and selected numeric/status facts.

    In particular do not emit HTTP bodies, environment values or provider text.
    """
    paths, hashes, counts = set(), set(), []

    def visit(item, key_path="", depth=0):
        if depth > 20:
            return
        if isinstance(item, dict):
            for key, child in item.items():
                visit(child, key_path + "/" + str(key), depth + 1)
        elif isinstance(item, list):
            for index, child in enumerate(item):
                visit(child, key_path + "/" + str(index), depth + 1)
        elif isinstance(item, str):
            paths.update(SAFE_CODE_PATH_RE.findall(item))
            if re.search(r"sha256|(?:^|/)(?:hash|digest)(?:/|$)", key_path, re.I):
                hashes.update(SHA_RE.findall(item))
        elif isinstance(item, (int, bool)) and re.search(r"count|matched|checked|total|unresolved|passed|module[_-]?id", key_path, re.I):
            if len(counts) < 60:
                counts.append({"key": key_path, "value": item})

    visit(value)
    return {"top_level_keys": sorted(value) if isinstance(value, dict) else [], "code_paths": sorted(paths)[:100], "sha256_values": sorted(hashes)[:100], "numeric_facts": counts}


def redact_contract(text):
    # This is the normative layout contract, not an env file or a provider log.
    # Drop any accidental credential or prompt sections before returning it.
    lines = []
    suppress_prompt = False
    for line in text.splitlines():
        if re.search(r"^#{1,6}\s+.*(?:raw|actual)\s+(?:provider\s+)?prompt", line, re.I):
            suppress_prompt = True
        elif line.startswith("#"):
            suppress_prompt = False
        if suppress_prompt or re.search(r"(?:Bearer\s+\S+|(?:api[_-]?key|password|secret|token)\s*[:=]\s*[^\s<])", line, re.I):
            lines.append("[sensitive line omitted]")
        else:
            lines.append(line)
    rendered = "\n".join(lines)
    return {"text": rendered[:20000], "truncated": len(rendered) > 20000, "original_characters": len(text)}


def read_inputs():
    evidence, loaded = {}, {}
    for name, (path, expected_sha) in INPUTS.items():
        item = fingerprint(path)
        item["expected_sha256"] = expected_sha
        item["matches_pinned_sha256"] = item.get("sha256") == expected_sha
        if path.exists():
            try:
                if name == "contract":
                    loaded[name] = path.read_text(encoding="utf-8")
                    item["normative_contract"] = redact_contract(loaded[name])
                else:
                    loaded[name] = json_file(path)
                    item["safe_inventory"] = safe_json_inventory(loaded[name])
            except (OSError, ValueError) as exc:
                item["parse_error"] = error_name(exc)
        evidence[name] = item
    return evidence, loaded


def declared_packages():
    packages = {}
    for parent in (CANDIDATE / "apps", CANDIDATE / "packages"):
        if not parent.exists():
            continue
        for directory in sorted(parent.iterdir()):
            path = directory / "package.json"
            if not path.exists():
                continue
            try:
                manifest = json_file(path)
                name = manifest.get("name")
                if isinstance(name, str):
                    packages[name] = {"root": directory, "manifest": manifest}
            except (OSError, ValueError):
                pass
    return packages


def package_base(specifier):
    parts = specifier.split("/")
    length = 2 if specifier.startswith("@") else 1
    return "/".join(parts[:length]), "." if len(parts) == length else "./" + "/".join(parts[length:])


def resolve_package(specifier, consumer):
    base, subpath = package_base(specifier)
    attempted_broken = []
    for origin in (consumer, *consumer.parents):
        path = origin / "node_modules" / base
        if os.path.lexists(path) and not path.exists():
            attempted_broken.append(str(path))
        if path.is_dir():
            result = {"consumer_root": str(consumer), "specifier": specifier, "package_name": base, "subpath": subpath, "package_root": fingerprint(path), "broken_links_before_resolution": attempted_broken}
            manifest_path = path / "package.json"
            result["package_manifest"] = fingerprint(manifest_path)
            try:
                manifest = json_file(manifest_path)
                result["resolved_name"] = manifest.get("name")
                result["resolved_version"] = manifest.get("version")
                result["export_targets"] = export_targets(manifest, subpath, path)
            except (OSError, ValueError) as exc:
                result["manifest_error"] = error_name(exc)
            return result
    return {"consumer_root": str(consumer), "specifier": specifier, "resolved": False, "broken_links_before_resolution": attempted_broken}


def export_targets(manifest, subpath, root):
    exports = manifest.get("exports")
    matched_key, wildcard = None, None
    branch = exports
    if isinstance(exports, dict) and any(str(key).startswith(".") for key in exports):
        if subpath in exports:
            matched_key, branch = subpath, exports[subpath]
        else:
            matches = []
            for key, value in exports.items():
                if "*" not in key:
                    continue
                prefix, suffix = key.split("*", 1)
                if subpath.startswith(prefix) and subpath.endswith(suffix) and len(subpath) >= len(prefix) + len(suffix):
                    matches.append((len(prefix), len(suffix), key, value, subpath[len(prefix):len(subpath) - len(suffix) if suffix else None]))
            if matches:
                _, _, matched_key, branch, wildcard = sorted(matches, key=lambda item: (item[0], item[1]), reverse=True)[0]
            else:
                return {"matched_export": None, "exported": False, "targets": []}
    elif exports is None:
        branch = manifest.get("module", manifest.get("main")) if subpath == "." else subpath
    elif subpath != ".":
        return {"matched_export": None, "exported": False, "targets": []}
    targets = []

    def walk(value, conditions):
        if isinstance(value, str):
            target = value.replace("*", wildcard) if wildcard is not None else value
            path = root / target
            targets.append({"conditions": conditions, "declared_target": target, "target": fingerprint(path), "target_inside_candidate": inside(path)})
        elif isinstance(value, dict):
            for condition, child in value.items():
                walk(child, conditions + [condition])
        elif isinstance(value, list):
            for index, child in enumerate(value):
                walk(child, conditions + [str(index)])

    walk(branch, [])
    return {"matched_export": matched_key, "exported": bool(targets), "targets": targets, "method": "static export-map inspection; no Node or bundler execution"}


def lock_summary(path, internal_names):
    result = fingerprint(path)
    if not path.exists():
        return result
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
        in_importers = False
        importer, section, dependency, specifier = None, None, None, None
        importers, edges, snapshots = [], [], 0
        in_snapshots = False
        for line in lines:
            if line.startswith("lockfileVersion:"):
                result["lockfile_version"] = line.split(":", 1)[1].strip().strip("'\"")
            if line == "importers:":
                in_importers = True
                continue
            if line == "snapshots:":
                in_snapshots = True
                in_importers = False
                continue
            if in_snapshots and re.match(r"^  \S.*:\s*$", line):
                snapshots += 1
            if not in_importers:
                continue
            if line and not line.startswith(" "):
                in_importers = False
                continue
            match = re.match(r"^( +)(.+?):\s*(.*)$", line)
            if not match:
                continue
            indent, key, value = len(match.group(1)), match.group(2).strip("'\""), match.group(3).strip().strip("'\"")
            if indent == 2:
                importer, section, dependency, specifier = key, None, None, None
                importers.append(importer)
            elif indent == 4:
                section, dependency, specifier = key, None, None
            elif indent == 6:
                dependency, specifier = key, None
            elif indent == 8 and key == "specifier":
                specifier = value
            elif indent == 8 and key == "version" and (dependency in internal_names or value.startswith("link:")):
                edges.append({"importer": importer, "section": section, "dependency": dependency, "specifier": specifier, "version": value})
        result.update({"importers": importers, "importer_count": len(importers), "internal_link_edges": edges, "snapshot_count": snapshots})
    except (OSError, ValueError) as exc:
        result["read_error"] = error_name(exc)
    return result


def module_metadata(path):
    result = fingerprint(path)
    if not path.exists():
        return result
    allowed = {"layoutVersion", "nodeLinker", "packageManager", "storeDir", "virtualStoreDir", "virtualStoreDirMaxLength", "hoistWorkspacePackages", "prunedAt"}
    try:
        result["profile_fields"] = {}
        for line in path.read_text(encoding="utf-8").splitlines():
            match = re.match(r"^([A-Za-z][A-Za-z0-9]*):\s*(.*)$", line)
            if match and match.group(1) in allowed:
                result["profile_fields"][match.group(1)] = match.group(2).strip("'\"")
    except OSError as exc:
        result["read_error"] = error_name(exc)
    return result


def source_paths():
    paths = []
    for name in ("package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml", "turbo.json", ".npmrc", ".pnpmfile.cjs"):
        path = CANDIDATE / name
        if path.exists():
            paths.append(path)
    for base in (CANDIDATE / "apps/web", CANDIDATE / "apps/desktop", CANDIDATE / "packages"):
        if not base.exists():
            continue
        for directory, folders, files in os.walk(base, followlinks=False):
            folders[:] = sorted(folder for folder in folders if folder not in PRUNED)
            for filename in sorted(files):
                path = Path(directory) / filename
                if path.suffix in SOURCE_SUFFIXES and not filename.startswith(".env"):
                    paths.append(path)
    # Other importer manifests are stable installation inputs too.
    for parent in (CANDIDATE / "apps", CANDIDATE / "packages"):
        if parent.exists():
            paths.extend(path for path in parent.glob("*/package.json") if path.exists())
    for name in (".modules.yaml", ".pnpm/lock.yaml", ".pnpm-workspace-state-v1.json"):
        path = CANDIDATE / "node_modules" / name
        if path.exists():
            paths.append(path)
    return sorted(set(paths))


def stable_snapshot(packages, expected_dependencies):
    files = {}
    for path in source_paths():
        try:
            files[str(path.relative_to(CANDIDATE))] = {"sha256": sha_file(path), "bytes": path.stat().st_size, "realpath": str(path.resolve())}
        except (OSError, RuntimeError) as exc:
            files[str(path.relative_to(CANDIDATE))] = {"read_error": error_name(exc)}
    directories = [CANDIDATE / "node_modules", CANDIDATE / "node_modules/.pnpm"]
    directories.extend(item["root"] / "node_modules" for item in packages.values())
    links = {str(path.relative_to(CANDIDATE)): fingerprint(path) for path in sorted(set(directories))}
    dependencies = {}
    for item in expected_dependencies:
        consumer = candidate_path(item["consumer_root"])
        base, _ = package_base(item["dependency"])
        resolved = resolve_package(base, consumer)
        dependencies[item["consumer"] + "/" + base] = resolved.get("package_root", resolved)
    digest = hashlib.sha256(json.dumps({"files": files, "directories": links, "dependency_origins": dependencies}, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    return {"sha256": digest, "files": files, "directories": links, "dependency_origins": dependencies}


def git_head():
    git_entry = CANDIDATE / ".git"
    result = {"git_entry": fingerprint(git_entry)}
    try:
        git_dir = git_entry
        if git_entry.is_file():
            text = git_entry.read_text(encoding="utf-8").strip()
            if not text.startswith("gitdir: "):
                result["read_error"] = "UnsupportedGitEntry"
                return result
            git_dir = (CANDIDATE / text[8:]).resolve()
        head = (git_dir / "HEAD").read_text(encoding="utf-8").strip()
        result["git_dir"] = str(git_dir)
        result["head_file_sha256"] = sha_file(git_dir / "HEAD")
        if re.fullmatch(r"[0-9a-f]{40}", head):
            result["commit"] = head
        elif head.startswith("ref: "):
            ref = head[5:]
            common = git_dir
            if (git_dir / "commondir").exists():
                common = (git_dir / (git_dir / "commondir").read_text(encoding="utf-8").strip()).resolve()
            result["head_ref"] = ref
            for root in (git_dir, common):
                loose = root / ref
                if loose.exists():
                    value = loose.read_text(encoding="utf-8").strip()
                    if re.fullmatch(r"[0-9a-f]{40}", value):
                        result["commit"] = value
                        break
                packed = root / "packed-refs"
                if packed.exists():
                    for line in packed.read_text(encoding="utf-8").splitlines():
                        if line.endswith(" " + ref):
                            result["commit"] = line.split(" ", 1)[0]
        result["matches_expected_commit"] = result.get("commit") == EXPECTED_HEAD
    except (OSError, RuntimeError) as exc:
        result["read_error"] = error_name(exc)
    return result


def internal_links(packages):
    links = []
    directories = [CANDIDATE / "node_modules/@multica", CANDIDATE / "node_modules/.pnpm/node_modules/@multica"]
    directories.extend(item["root"] / "node_modules/@multica" for item in packages.values())
    virtual = CANDIDATE / "node_modules/.pnpm"
    if virtual.exists():
        directories.extend(virtual.glob("*/node_modules/@multica"))
    for directory in sorted(set(directories)):
        if not directory.exists():
            continue
        for path in sorted(directory.iterdir()):
            item = fingerprint(path)
            item["source_inside_candidate"] = inside(path)
            name = "@multica/" + path.name
            if name in packages:
                item["matches_declared_workspace_root"] = str(path.resolve()) == str(packages[name]["root"].resolve())
            links.append(item)
    return links


def source_overlays():
    symlinks = []
    for base in (CANDIDATE / "apps/web", CANDIDATE / "apps/desktop", CANDIDATE / "packages"):
        if base.is_symlink():
            symlinks.append(fingerprint(base))
        if not base.exists():
            continue
        for directory, folders, files in os.walk(base, followlinks=False):
            folders[:] = sorted(folder for folder in folders if folder not in PRUNED)
            for name in sorted(folders + files):
                path = Path(directory) / name
                if path.is_symlink():
                    item = fingerprint(path)
                    item["source_inside_candidate"] = inside(path)
                    symlinks.append(item)
    return symlinks


def auth_origins(packages):
    core = packages.get("@multica/core", {}).get("root")
    if core is None:
        return {"read_error": "CoreWorkspaceMissing"}
    source_files = [path for path in source_paths() if path.suffix in {".ts", ".tsx", ".js", ".jsx", ".mjs"}]
    auth_specs, symbol_files = {"@multica/core/auth"}, []
    for path in source_files:
        try:
            text = path.read_text(encoding="utf-8")
            auth_specs.update(spec for spec in IMPORT_RE.findall(text) if spec.startswith("@multica/core/auth"))
            if any(symbol in text for symbol in ("WebProviders", "CoreProvider", "useAuthStore")):
                symbol_files.append({"path": str(path), "sha256": sha_file(path)})
        except (OSError, UnicodeError):
            pass
    exports = packages["@multica/core"]["manifest"].get("exports", {})
    if isinstance(exports, dict):
        auth_specs.update("@multica/core" + key[1:] for key in exports if key.startswith("./auth") and "*" not in key)
    contexts = [CANDIDATE / "apps/web", core, packages.get("@multica/views", {}).get("root", CANDIDATE / "packages/views")]
    contexts.extend(Path(item["path"]).parent for item in symbol_files if Path(item["path"]).name in {"providers.tsx", "core-provider.tsx", "auth-store.ts"})
    contexts = sorted(set(contexts))
    probes = [resolve_package(spec, consumer) for consumer in contexts for spec in sorted(auth_specs)]
    auth_sources = [path for path in source_files if inside(path, core) and "auth" in path.parts and ("store" in path.name or path.name == "index.ts")]
    inodes = []
    for consumer in contexts:
        root_result = resolve_package("@multica/core", consumer)
        package_root_info = root_result.get("package_root", {})
        root_path = package_root_info.get("path")
        if root_path:
            for source in auth_sources:
                relative = source.relative_to(core)
                inodes.append({"consumer_root": str(consumer), "relative_auth_source": str(relative), "source": fingerprint(Path(root_path) / relative)})
    return {"actual_auth_import_specifiers_and_exact_exports": sorted(auth_specs), "consumer_contexts": [str(path) for path in contexts], "source_symbol_files": symbol_files[:80], "static_export_probes": probes, "auth_source_inode_probes": inodes}


def critical_external_origins(packages):
    consumers = [CANDIDATE, CANDIDATE / "apps/web", CANDIDATE / "packages/core", CANDIDATE / "packages/views"]
    probes = []
    for consumer in consumers:
        for name in ("react", "react-dom", "zustand", "next", "@tanstack/react-query"):
            result = resolve_package(name, consumer)
            result.pop("export_targets", None)
            probes.append(result)
    desktop = packages.get("@multica/desktop", {}).get("root", CANDIDATE / "apps/desktop")
    electron = resolve_package("electron", desktop)
    electron.pop("export_targets", None)
    if "package_root" in electron:
        root = Path(electron["package_root"]["path"])
        electron["runtime_files"] = [fingerprint(root / name) for name in ("path.txt", "dist/version", "dist/Electron.app/Contents/MacOS/Electron", "dist/electron", "dist/electron.exe")]
    return {"web_shared_singleton_metadata": probes, "desktop_electron": electron, "external_content_cache_sharing_is_permitted": True}


def prior_build_artifacts():
    root = CANDIDATE / "apps/web/.next"
    result = {"directory": fingerprint(root), "build_id": fingerprint(root / "BUILD_ID"), "trace_files_checked": 0, "workspace_source_refs_outside_candidate": []}
    if not root.exists():
        return result
    for path in sorted(root.rglob("*.nft.json")):
        result["trace_files_checked"] += 1
        try:
            payload = json_file(path)
            for entry in payload.get("files", []):
                if not isinstance(entry, str):
                    continue
                target = (path.parent / entry).resolve()
                text = str(target)
                if re.search(r"/packages/(?:core|views|ui|tsconfig|eslint-config)/", text) and not inside(target):
                    if len(result["workspace_source_refs_outside_candidate"]) < 80:
                        result["workspace_source_refs_outside_candidate"].append({"trace_path": str(path), "workspace_source_realpath": text})
        except (OSError, ValueError, RuntimeError):
            pass
    return result


def installation_receipts():
    receipts = []
    root = COORDINATOR / "v11"
    for path in sorted(root.rglob("*")):
        if "independent-workspace-review" in path.parts or not path.is_file():
            continue
        if not re.search(r"install|receipt|freeze", path.name, re.I):
            continue
        result = fingerprint(path)
        if path.suffix == ".json":
            try:
                result["safe_inventory"] = safe_json_inventory(json_file(path))
            except (OSError, ValueError) as exc:
                result["parse_error"] = error_name(exc)
        receipts.append(result)
    return receipts[:60]


def main():
    started = now()
    inputs, loaded = read_inputs()
    expectations = loaded.get("expectations", {})
    expected_dependencies = expectations.get("internal_dependencies", [])
    packages = declared_packages()
    before = stable_snapshot(packages, expected_dependencies)
    head_before = git_head()
    dependency_checks = []
    for item in expected_dependencies:
        probe = resolve_package(item["dependency"], candidate_path(item["consumer_root"]))
        expected = candidate_path(item["expected_workspace_root"])
        probe["expected_workspace_root"] = str(expected)
        probe["declared"] = item.get("declared")
        probe["consumer"] = item["consumer"]
        probe["matches_expected_workspace_root"] = probe.get("package_root", {}).get("realpath") == str(expected.resolve())
        probe["source_inside_candidate"] = bool(probe.get("package_root")) and inside(Path(probe["package_root"]["path"]))
        dependency_checks.append(probe)
    configuration_checks = []
    config_expected = expectations.get("configuration_sha256", {})
    if isinstance(config_expected, dict):
        for name, expected_sha in sorted(config_expected.items()):
            path = candidate_path(name)
            item = fingerprint(path)
            item.update({"expected_sha256": expected_sha, "matches_expected_sha256": item.get("sha256") == expected_sha})
            configuration_checks.append(item)
    manifests = [{"name": name, "root": fingerprint(item["root"]), "package_json": fingerprint(item["root"] / "package.json"), "version": item["manifest"].get("version"), "declared_internal_dependencies": {section: {key: value for key, value in item["manifest"].get(section, {}).items() if key in packages} for section in ("dependencies", "devDependencies", "optionalDependencies", "peerDependencies")}} for name, item in sorted(packages.items())]
    lockfiles = {"committed": lock_summary(CANDIDATE / "pnpm-lock.yaml", set(packages)), "installed": lock_summary(CANDIDATE / "node_modules/.pnpm/lock.yaml", set(packages)), "modules_metadata": module_metadata(CANDIDATE / "node_modules/.modules.yaml"), "workspace_state": fingerprint(CANDIDATE / "node_modules/.pnpm-workspace-state-v1.json")}
    state_path = CANDIDATE / "node_modules/.pnpm-workspace-state-v1.json"
    if state_path.exists():
        try:
            lockfiles["workspace_state"]["safe_inventory"] = safe_json_inventory(json_file(state_path))
        except (OSError, ValueError):
            pass
    links = internal_links(packages)
    overlays = source_overlays()
    auth = auth_origins(packages)
    external = critical_external_origins(packages)
    builds = prior_build_artifacts()
    receipts = installation_receipts()
    after = stable_snapshot(declared_packages(), expected_dependencies)
    head_after = git_head()
    changed_files = [name for name in sorted(set(before["files"]) | set(after["files"])) if before["files"].get(name) != after["files"].get(name)]
    changed_directories = [name for name in sorted(set(before["directories"]) | set(after["directories"])) if before["directories"].get(name) != after["directories"].get(name)]
    changed_origins = [name for name in sorted(set(before["dependency_origins"]) | set(after["dependency_origins"])) if before["dependency_origins"].get(name) != after["dependency_origins"].get(name)]
    result = {
        "schema_version": 1,
        "collector": "reviewer-authored read-only stdlib Python; executed once by coordinator because reviewer exec transport was unavailable",
        "started_at_utc": started,
        "completed_at_utc": now(),
        "candidate_root": str(CANDIDATE),
        "expected_candidate_head": EXPECTED_HEAD,
        "operations": {"filesystem_reads": True, "project_code_executed": False, "subprocesses": False, "node": False, "tests": False, "build": False, "install": False, "git_mutations": False, "candidate_mutations": False, "environment_contents": False, "network": False, "services": False, "provider": False},
        "pinned_inputs": inputs,
        "expectation_candidate_root": expectations.get("candidate_root"),
        "expectation_declared_packages": safe_json_inventory(expectations.get("declared_packages", [])),
        "head_before": head_before,
        "head_after": head_after,
        "declared_workspaces": manifests,
        "declared_dependency_checks": dependency_checks,
        "declared_dependency_summary": {"checked": len(dependency_checks), "matches_expected_workspace_root": sum(item["matches_expected_workspace_root"] for item in dependency_checks), "all_inside_candidate": all(item["source_inside_candidate"] for item in dependency_checks)},
        "configuration_checks": configuration_checks,
        "installation_lock_and_profile": lockfiles,
        "own_node_modules_directories": before["directories"],
        "all_discovered_internal_links": links,
        "source_symlinks": overlays,
        "nested_auth_and_consumer_resolution": auth,
        "external_package_origins": external,
        "existing_build_artifacts": builds,
        "installation_receipts_found": receipts,
        "stable_inputs": {"scope": "root install/config inputs; all Web/Desktop/shared package source/config/docs files; all importer manifests; installed lock/profile/workspace state; own node_modules directory and declared internal package origins", "before_sha256": before["sha256"], "after_sha256": after["sha256"], "before_file_count": len(before["files"]), "after_file_count": len(after["files"]), "unchanged": before["sha256"] == after["sha256"], "changed_files": changed_files, "changed_directories": changed_directories, "changed_dependency_origins": changed_origins},
        "limits": ["No actual Node resolver, webpack graph, fresh build or SSR was executed.", "Static export-map branches are inspected without choosing runtime conditions.", "Shared/cache-backed external package content is reported separately from internal workspace source origins.", "Absent installation receipts do not turn this filesystem observation into a completed installation freeze.", "No wholecheck, E5/E6, paid-provider or task-completion clearance."]
    }
    print(json.dumps(result, ensure_ascii=False, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
