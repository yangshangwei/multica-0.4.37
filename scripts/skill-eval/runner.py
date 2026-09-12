#!/usr/bin/env python3
"""Opt-in Claude Code evaluation with local fixtures; standard library only."""
import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time

GATE = "MULTICA_RUN_REAL_AGENT_SMOKE"
HERE = Path(__file__).resolve().parent
BRIEF = """你在一个一次性的本地样例仓库中处理中文用户请求。只有当前工作目录中的文件与任务属于本次工作范围。使用原生 Skill 工具加载与请求相关的技能，遵守技能及用户的范围；不要批量加载无关技能。
文件操作和真实本地检查通过 fixture 工具完成：list_files、read_file、write_file、search、list_checks、run_check。search 是只读字面搜索，返回实际路径、行号、文件哈希和范围/截断信息。run_check 返回实际进程结果；工具没有网络能力，不允许任意命令或外部路径。工具具备写文件能力不代表任务允许修改文件。不得启动其他智能体、调用外部服务或要求扩大权限。
任务、评论、关联资料都在本地 task.md 与 references/ 中，没有在线任务平台。最终回复就是本地任务评论草稿的交付渠道，宿主会保存原文；不得声称已经发到在线平台。中文回复，直接给出实际交付物，不写未执行的检查结果。
"""


def dump(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def copy_skills(source, destination):
    source = Path(source).resolve()
    entries = sorted(source.rglob("*"))
    if any(path.is_symlink() for path in entries):
        raise ValueError("skill bundles must not contain symlinks")
    manifest = []
    for path in entries:
        if not path.is_file():
            continue
        relative = path.relative_to(source)
        target = destination / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, target)
        manifest.append({"path": relative.as_posix(), "sha256": digest(target), "bytes": target.stat().st_size})
    return manifest


def write_fixture(root, files):
    for relative, text in files.items():
        path = Path(relative)
        if path.is_absolute() or ".." in path.parts or not path.parts or path.parts[0].casefold() == ".claude":
            raise ValueError("fixture data must use relative paths outside the reserved .claude directory")
        target = root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)


def files_manifest(root):
    return {path.relative_to(root).as_posix(): digest(path) for path in root.rglob("*") if path.is_file()}


def parse_events(path):
    events = []
    if path.exists():
        for number, line in enumerate(path.read_text().splitlines(), 1):
            try:
                events.append((number, json.loads(line)))
            except ValueError:
                pass
    return events


def session_settings(inventory, skill_names):
    return {
        "disableAllHooks": True, "autoMemoryEnabled": False,
        "enabledPlugins": {item["source"]: False for item in inventory.get("plugins", [])},
        "skillOverrides": {name: "off" for name in inventory.get("skills", []) if name not in skill_names},
        "claudeMdExcludes": [str(Path.home() / "**"), "/Volumes/**"],
        "env": {"ENABLE_TOOL_SEARCH": "false", "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "0"},
    }


def prepare_checks(case, root, evidence, node):
    node_runtime = str(Path(node).resolve().parents[1]) if node else "/nonexistent-node-runtime"
    profile = evidence / "node-sandbox.sb"
    profile.write_text("\n".join([
        "(version 1)", "(allow default)", "(deny network*)",
        '(deny file-write* (require-all (require-not (subpath ' + json.dumps(str(root)) + ')) (require-not (literal "/dev/null"))))',
        '(deny file-read-data (subpath "/Volumes"))',
        '(deny file-read-data (require-all (subpath "/Users") (require-not (subpath ' + json.dumps(node_runtime) + '))))', "",
    ]))
    checks = {}
    for name, argv in case.get("checks", {}).items():
        if not node or not isinstance(argv, list) or argv[0] != "node":
            raise ValueError("fixed checks require the configured local Node executable")
        arguments = argv[1:]
        if len(arguments) == 2 and arguments[0] == "--test":
            script = arguments[1]
        elif len(arguments) == 1:
            script = arguments[0]
        else:
            raise ValueError("only a fixed Node file or --test file is supported")
        path = Path(script)
        if path.is_absolute() or ".." in path.parts or path.suffix not in [".js", ".mjs"] or not (root / path).is_file():
            raise ValueError("check entrypoint must be an existing fixture JavaScript file")
        checks[name] = {"argv": [node, *arguments], "display_command": " ".join(argv)}
    dump(evidence / "fixed-checks.json", {
        "sandbox_profile": str(profile), "checks": checks,
        "frozen_execution_inputs": {p.relative_to(root).as_posix(): digest(p) for p in root.rglob("*") if p.is_file() and p.suffix in [".js", ".mjs"]},
    })


def collect(case, root, evidence, before, snapshot, code, seconds, timeout):
    events = parse_events(evidence / "native-events.jsonl")
    adapter = [event for _, event in parse_events(evidence / "fixture-tool-events.jsonl")]
    init = next((event for _, event in events if event.get("type") == "system" and event.get("subtype") == "init"), {})
    terminal = next((event for _, event in reversed(events) if event.get("type") == "result"), {})
    tool_uses = []
    for number, event in events:
        if event.get("type") == "assistant":
            for item in event.get("message", {}).get("content", []):
                if item.get("type") == "tool_use":
                    tool_uses.append({"line": number, **item})
    loaded = []
    for path in sorted(snapshot.glob("*/SKILL.md")):
        text = path.read_text()
        body = text.split("---", 2)[-1].strip()
        proof = []
        for number, event in events:
            if event.get("type") != "user":
                continue
            content = event.get("message", {}).get("content", [])
            texts = [content] if isinstance(content, str) else [item.get("text", "") for item in content]
            if any(body in item for item in texts):
                proof.append({"line": number, "source": "native_skill_content"})
        for number, event in enumerate(adapter, 1):
            if event.get("name") == "read_file" and not event.get("is_error") and event.get("result", {}).get("text") == text:
                proof.append({"adapter_line": number, "source": "complete_file_read"})
        if proof:
            loaded.append({"name": path.parent.name, "evidence": proof})
    after = files_manifest(root)
    changed = [name for name in sorted(set(before) | set(after)) if before.get(name) != after.get(name)]
    dump(evidence / "files-after.json", after)
    for name in changed:
        actual = root / name
        if actual.is_file():
            target = evidence / "changed-files" / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(actual, target)
    (evidence / "final-answer.md").write_text(terminal.get("result", "") + "\n")
    result = {
        "id": case["id"], "kind": case.get("kind"), "expected_primary_skills": case.get("expected", []),
        "model": init.get("model"), "completed_successfully": bool(terminal) and terminal.get("subtype") == "success" and not terminal.get("is_error") and code == 0 and not timeout,
        "native_exit_code": code, "timed_out": timeout, "native_status": terminal.get("subtype"), "native_is_error": terminal.get("is_error"),
        "duration_seconds": round(seconds, 3), "cost_usd": terminal.get("total_cost_usd"), "usage": terminal.get("usage"), "model_usage": terminal.get("modelUsage"),
        "native_skill_invocations": [item for item in tool_uses if item["name"] == "Skill"], "observed_full_skill_content": loaded,
        "all_tool_uses": tool_uses, "fixture_tool_calls": [item for item in adapter if item.get("type") == "tool_call"],
        "permission_denials": terminal.get("permission_denials", []), "adapter_denials": [item for item in adapter if item.get("boundary_denial")],
        "plugins": init.get("plugins", []), "native_tools": init.get("tools", []), "mcp_servers": init.get("mcp_servers", []),
        "changed_files": changed, "workspace": str(root), "evidence": str(evidence), "semantic_verdict": "pending independent review",
    }
    dump(evidence / "result.json", result)
    return result


def run_case(case, repeat, options, snapshot, inventory, claude, node, stop):
    if not re.fullmatch(r"[A-Za-z0-9_-]+", case["id"]):
        raise ValueError("case IDs must contain only letters, digits, underscores or hyphens")
    if stop.is_set():
        return {"id": case["id"], "repeat": repeat, "phase": options.phase, "native_status": "not_run", "completed_successfully": False, "not_run_reason": "an earlier native execution failed; no account call was started"}
    evidence = options.output / "runs" / options.phase / (case["id"] + "-r" + str(repeat))
    evidence.mkdir(parents=True, exist_ok=False)
    root = Path(tempfile.mkdtemp(prefix="multica-skill-eval-" + case["id"].split("_")[0] + "-")).resolve()
    copy_skills(snapshot, root / ".claude/skills")
    write_fixture(root, case.get("files", {}))
    dump(evidence / "case.json", {**case, "repeat": repeat, "phase": options.phase, "workspace": str(root)})
    (evidence / "prompt.txt").write_text(case["prompt"] + "\n")
    before = files_manifest(root)
    dump(evidence / "files-before.json", before)
    prepare_checks(case, root, evidence, node)
    skill_names = sorted(path.parent.name for path in snapshot.glob("*/SKILL.md"))
    dump(evidence / "session-settings.json", session_settings(inventory, skill_names))
    dump(evidence / "mcp.json", {"mcpServers": {"fixture": {"command": "/usr/bin/env", "args": ["-i", "PATH=/usr/bin:/bin", sys.executable, str(HERE / "fixture_server.py"), "--root", str(root), "--evidence", str(evidence), "--checks", str(evidence / "fixed-checks.json")]}}})
    allowed = ["Skill(" + name + ")" for name in skill_names] + ["mcp__fixture__*"]
    command = [claude, "--print", "--output-format", "stream-json", "--verbose", "--tools", "Skill", "--allowedTools", ",".join(allowed),
        "--no-session-persistence", "--no-chrome", "--permission-mode", "dontAsk", "--permission-prompts", "none",
        "--strict-mcp-config", "--mcp-config", str(evidence / "mcp.json"), "--settings", str(evidence / "session-settings.json"),
        "--append-system-prompt", BRIEF, "--effort", "medium", case["prompt"]]
    dump(evidence / "invocation.json", {"command": command, "cwd": str(root), "credentials": "existing native Claude settings only; never copied"})
    started = time.monotonic()
    timeout = False
    with (evidence / "native-events.jsonl").open("w") as stdout, (evidence / "native-stderr.log").open("w") as stderr:
        process = subprocess.Popen(command, cwd=root, stdout=stdout, stderr=stderr, text=True, start_new_session=True)
        try:
            code = process.wait(timeout=options.timeout)
        except subprocess.TimeoutExpired:
            timeout = True
            os.killpg(process.pid, signal.SIGTERM)
            try:
                code = process.wait(timeout=8)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                code = process.wait(timeout=5)
    result = collect(case, root, evidence, before, snapshot, code, time.monotonic() - started, timeout)
    result["repeat"] = repeat
    result["phase"] = options.phase
    if not result["completed_successfully"]:
        stop.set()
    dump(evidence / "result.json", result)
    print(json.dumps({key: result[key] for key in ["id", "model", "completed_successfully", "changed_files", "duration_seconds", "cost_usd", "permission_denials"]}, ensure_ascii=False), flush=True)
    return result


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", type=Path, default=HERE / "cases.json")
    parser.add_argument("--case", action="append")
    parser.add_argument("--skills", type=Path, required=True)
    parser.add_argument("--inventory", type=Path, required=True, help="Sanitized current native init metadata: skills and plugin sources only")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--phase", required=True)
    parser.add_argument("--repeat", type=int, default=1)
    parser.add_argument("--concurrency", type=int, choices=[1, 2], default=2)
    parser.add_argument("--timeout", type=int, default=240)
    options = parser.parse_args(argv)
    if os.environ.get(GATE) != "1":
        parser.error("real account access requires " + GATE + "=1")
    if not re.fullmatch(r"[A-Za-z0-9_-]+", options.phase) or not 1 <= options.repeat <= 3 or not 30 <= options.timeout <= 300:
        parser.error("use a simple phase label, one to three repeats and a 30–300 second timeout")
    # Executable lookup happens only after the explicit account-access gate.
    claude = shutil.which("claude")
    if not claude:
        parser.error("Claude Code is not installed on PATH")
    cases = json.loads(options.cases.read_text())
    selected = [case for case in cases if not options.case or case["id"] in options.case]
    if not selected:
        parser.error("no matching cases")
    node = shutil.which("node") if any(case.get("checks") for case in selected) else None
    inventory = json.loads(options.inventory.read_text())
    options.output = options.output.resolve()
    snapshot = options.output / "snapshots" / options.phase
    snapshot.mkdir(parents=True, exist_ok=False)
    manifest = copy_skills(options.skills, snapshot)
    dump(options.output / (options.phase + "-manifest.json"), {"source": str(options.skills.resolve()), "files": manifest})
    dump(options.output / (options.phase + "-runtime.json"), {"version": subprocess.check_output([claude, "--version"], text=True).strip(), "phase": options.phase, "cases": [case["id"] for case in selected], "repeat": options.repeat, "concurrency": options.concurrency})
    stop = threading.Event()
    with concurrent.futures.ThreadPoolExecutor(max_workers=options.concurrency) as pool:
        futures = [pool.submit(run_case, case, repeat, options, snapshot, inventory, claude, node, stop) for case in selected for repeat in range(1, options.repeat + 1)]
        results = [future.result() for future in concurrent.futures.as_completed(futures)]
    dump(options.output / (options.phase + "-results.json"), results)
    return 0 if all(result["completed_successfully"] for result in results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
