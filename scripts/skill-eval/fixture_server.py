#!/usr/bin/env python3
"""Local-only MCP adapter for disposable skill evaluation fixtures."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time

TOOLS = [
    {"name": "list_files", "description": "List the actual files available in this local fixture workspace. No external files or network are available.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
    {"name": "read_file", "description": "Read a complete UTF-8 file from the local fixture workspace. Accepts a workspace-relative path or an absolute path inside that workspace.", "inputSchema": {"type": "object", "properties": {"path": {"type": "string"}}, "required": ["path"], "additionalProperties": False}},
    {"name": "write_file", "description": "Write UTF-8 text to a file inside the local fixture workspace. This tool does not grant task authority: respect the user's scope and role. Skill source files are immutable.", "inputSchema": {"type": "object", "properties": {"path": {"type": "string"}, "content": {"type": "string"}}, "required": ["path", "content"], "additionalProperties": False}},
    {"name": "list_checks", "description": "List the actual build/test commands provided by the local fixture. This does not run them or predict their results.", "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False}},
    {"name": "run_check", "description": "Execute one provided local build/test command and return its real exit code, stdout, and stderr. A nonzero exit can be a test failure; preserve and report the output. Arbitrary commands and network access are unavailable.", "inputSchema": {"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"], "additionalProperties": False}},
    {"name": "search", "description": "Read-only, case-sensitive literal search of current UTF-8 fixture files. Returns actual path, line, text and file SHA-256 evidence, declared scope, limits and completeness. No regex interpretation or expected answers. Check complete, truncated and skipped_files before claiming absence or full coverage.", "annotations": {"readOnlyHint": True}, "inputSchema": {"type": "object", "properties": {"query": {"type": "string", "minLength": 1, "maxLength": 2000}, "paths": {"type": "array", "items": {"type": "string"}, "minItems": 1, "maxItems": 32, "default": ["."]}, "max_results": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 100}, "max_files": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 200}, "max_bytes": {"type": "integer", "minimum": 1, "maximum": 1000000, "default": 200000}}, "required": ["query"], "additionalProperties": False}},
]


def resolve_path(value, *, relative_only=False):
    if not isinstance(value, str) or not value:
        raise ValueError("path must be a nonempty string")
    candidate = Path(value)
    if ".." in candidate.parts or (relative_only and candidate.is_absolute()):
        raise PermissionError("search paths must be fixture-relative, without traversal")
    if not candidate.is_absolute():
        candidate = ROOT / candidate
    if not candidate.is_relative_to(ROOT):
        raise PermissionError("path is outside the isolated fixture workspace")
    current = ROOT
    for part in candidate.relative_to(ROOT).parts:
        current /= part
        if current.is_symlink():
            raise PermissionError("symlinks are not supported in fixture paths")
    candidate = candidate.resolve()
    if not candidate.is_relative_to(ROOT):
        raise PermissionError("path is outside the isolated fixture workspace")
    return candidate


def iter_files(path):
    if path.is_symlink():
        raise PermissionError("symlinks are not supported in search scope")
    if path.is_file():
        yield path
    elif path.is_dir():
        for child in sorted(path.iterdir()):
            yield from iter_files(child)
    else:
        raise FileNotFoundError("search scope is not an existing regular file or directory")


def search_files(arguments):
    defaults = {"max_results": 100, "max_files": 200, "max_bytes": 200000}
    caps = {"max_results": 1000, "max_files": 1000, "max_bytes": 1000000}
    if set(arguments) - {"query", "paths", *defaults}:
        raise ValueError("unsupported search argument")
    query = arguments.get("query")
    if not isinstance(query, str) or not query or len(query) > 2000 or "\n" in query or "\r" in query:
        raise ValueError("query must be a nonempty single-line literal of at most 2000 characters")
    paths = arguments.get("paths", ["."])
    if not isinstance(paths, list) or not 1 <= len(paths) <= 32:
        raise ValueError("paths must contain between one and 32 fixture-relative scopes")
    scopes = [resolve_path(path, relative_only=True) for path in paths]
    limits = {key: arguments.get(key, default) for key, default in defaults.items()}
    for key, value in limits.items():
        if type(value) is not int or not 1 <= value <= caps[key]:
            raise ValueError(key + " must be an integer between 1 and " + str(caps[key]))
    selected = []
    seen = set()
    reasons = []
    for scope in scopes:
        for path in iter_files(scope):
            stat = path.stat(follow_symlinks=False)
            identity = (stat.st_dev, stat.st_ino)
            if identity in seen:
                continue
            seen.add(identity)
            selected.append(path)
            if len(selected) > limits["max_files"]:
                reasons.append("max_files")
                break
        if reasons:
            break
    matches = []
    files = []
    skipped = []
    byte_count = 0
    matched_lines = 0
    for path in selected[:limits["max_files"]]:
        relative = path.relative_to(ROOT).as_posix()
        remaining = limits["max_bytes"] - byte_count
        if path.stat().st_size > remaining:
            skipped.append({"path": relative, "reason": "max_bytes"})
            if "max_bytes" not in reasons:
                reasons.append("max_bytes")
            continue
        with path.open("rb") as stream:
            data = stream.read(remaining + 1)
        if len(data) > remaining:
            skipped.append({"path": relative, "reason": "max_bytes"})
            if "max_bytes" not in reasons:
                reasons.append("max_bytes")
            continue
        byte_count += len(data)
        try:
            lines = data.decode("utf-8").splitlines()
        except UnicodeDecodeError:
            skipped.append({"path": relative, "reason": "non_utf8"})
            continue
        sha256 = hashlib.sha256(data).hexdigest()
        files.append({"path": relative, "sha256": sha256, "bytes": len(data), "lines": len(lines)})
        for line_number, text in enumerate(lines, 1):
            if query not in text:
                continue
            matched_lines += 1
            if len(matches) < limits["max_results"]:
                matches.append({"path": relative, "line": line_number, "text": text, "sha256": sha256})
            elif "max_results" not in reasons:
                reasons.append("max_results")
    return {"query": query, "match_mode": "literal", "case_sensitive": True,
            "scope": {"paths": paths, "recursive": True, "encoding": "utf-8"},
            "matches": matches, "matched_lines": matched_lines, "files": files,
            "files_scanned": len(files), "files_discovered_lower_bound": len(selected),
            "bytes_scanned": byte_count, "skipped_files": skipped, "limits": limits,
            "limit_reasons": reasons, "complete": not reasons and not skipped, "truncated": bool(reasons)}


def handle_tool(name, arguments):
    if name == "search":
        return search_files(arguments)
    if name == "list_files":
        return {"files": sorted(str(p.relative_to(ROOT)) for p in ROOT.rglob("*") if p.is_file())}
    if name == "read_file":
        path = resolve_path(arguments["path"])
        data = path.read_bytes()
        if len(data) > 200000:
            raise ValueError("fixture file is too large")
        return {"path": str(path.relative_to(ROOT)), "sha256": hashlib.sha256(data).hexdigest(), "text": data.decode("utf-8")}
    if name == "write_file":
        path = resolve_path(arguments["path"])
        relative = path.relative_to(ROOT)
        if relative.parts and relative.parts[0].casefold() == ".claude":
            raise PermissionError("the original skill files are immutable")
        content = arguments["content"]
        if len(content.encode("utf-8")) > 200000:
            raise ValueError("fixture write exceeds size limit")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        return {"path": str(path.relative_to(ROOT)), "bytes": len(content.encode("utf-8")), "written": True}
    if name == "list_checks":
        return {"checks": [{"name": label, "command": info["display_command"]} for label, info in CHECKS["checks"].items()]}
    if name == "run_check":
        label = arguments["name"]
        if label not in CHECKS["checks"]:
            raise PermissionError("check is not one of the fixture's fixed commands")
        for relative, expected in CHECKS["frozen_execution_inputs"].items():
            path = resolve_path(relative)
            if hashlib.sha256(path.read_bytes()).hexdigest() != expected:
                raise PermissionError("execution input changed; arbitrary modified code cannot be executed")
        info = CHECKS["checks"][label]
        command = ["/usr/bin/sandbox-exec", "-f", CHECKS["sandbox_profile"], *info["argv"]]
        started = time.monotonic()
        result = subprocess.run(command, cwd=ROOT, env={"PATH": "/usr/bin:/bin", "LANG": "en_US.UTF-8", "TMPDIR": str(ROOT)}, capture_output=True, text=True, timeout=25)
        return {"command": info["display_command"], "exit_code": result.returncode, "stdout": result.stdout, "stderr": result.stderr, "duration_seconds": round(time.monotonic() - started, 3)}
    raise ValueError("unknown local fixture tool")


def record(value):
    with (EVIDENCE / "fixture-tool-events.jsonl").open("a") as stream:
        stream.write(json.dumps(value, ensure_ascii=False) + "\n")


def main():
    global ROOT, EVIDENCE, CHECKS
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--evidence", required=True)
    parser.add_argument("--checks", required=True)
    args = parser.parse_args()
    ROOT = Path(args.root).resolve()
    EVIDENCE = Path(args.evidence).resolve()
    CHECKS = json.loads(Path(args.checks).read_text())
    record({"type": "server_start", "cwd": str(ROOT), "environment_keys": sorted(os.environ), "network": "no network tools; fixed Node checks use an OS sandbox with network denied"})
    for line in sys.stdin:
        request = {}
        try:
            request = json.loads(line)
            method = request.get("method")
            params = request.get("params", {})
            if "id" not in request:
                continue
            if method == "initialize":
                result = {"protocolVersion": params.get("protocolVersion", "2024-11-05"), "capabilities": {"tools": {}}, "serverInfo": {"name": "multica-local-fixture", "version": "1.0"}}
            elif method == "ping":
                result = {}
            elif method == "tools/list":
                result = {"tools": TOOLS}
            elif method == "tools/call":
                tool = params.get("name")
                arguments = params.get("arguments", {})
                event = {"type": "tool_call", "name": tool, "arguments": arguments}
                try:
                    actual = handle_tool(tool, arguments)
                    event.update({"result": actual, "is_error": False})
                    result = {"content": [{"type": "text", "text": json.dumps(actual, ensure_ascii=False)}], "isError": False}
                except Exception as error:
                    actual = {"error": type(error).__name__, "message": str(error)}
                    event.update({"result": actual, "is_error": True, "boundary_denial": isinstance(error, PermissionError)})
                    result = {"content": [{"type": "text", "text": json.dumps(actual, ensure_ascii=False)}], "isError": True}
                record(event)
            else:
                raise ValueError("unsupported MCP method")
            print(json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}, ensure_ascii=False), flush=True)
        except Exception as error:
            print(json.dumps({"jsonrpc": "2.0", "id": request.get("id"), "error": {"code": -32603, "message": type(error).__name__ + ": " + str(error)}}), flush=True)


if __name__ == "__main__":
    main()
