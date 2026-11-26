#!/usr/bin/env python3
"""Scripted official-CLI-shaped O1 fixture; no provider request or RPC."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import uuid
if os.environ.get("MULTICA_RUN_REAL_AGENT_SMOKE"):
    raise SystemExit("Fake direct O1 refuses real opt-in")
if sys.argv[1:]==["--version"]:
    print("codex-cli 0.164.0")
    raise SystemExit(0)
args=sys.argv[1:]
if args != ["exec","--json","--skip-git-repo-check","-"] and not (len(args)==6 and args[:4]==["exec","resume","--json","--skip-git-repo-check"] and args[-1]=="-"):
    raise SystemExit("Unexpected direct CLI arguments")
cfg=json.loads(Path(os.environ["SECREM_HARNESS_CONFIG"]).read_text())
if cfg["mode"]!="fake" or cfg["scenario"]!="O1" or cfg.get("execution_mode")!="official-cli-direct":
    raise SystemExit("Fake direct O1 registration required")
manifest=json.loads((Path(cfg["claim_manifest_dir"])/(os.environ["MULTICA_TASK_ID"]+".json")).read_text())
prompt=sys.stdin.buffer.read()
if not prompt:raise SystemExit("Exact production input missing")
thread=args[4] if args[1]=="resume" else str(uuid.uuid4())
def emit(value):
    print(json.dumps(value), flush=True)

def command(argv):
    identifier=str(uuid.uuid4())
    emit({"type":"item.started","item":{"id":identifier,"type":"command_execution","command":"multica "+" ".join(argv[:3])}})
    p = subprocess.run([cfg["cli_path"]] + argv, env=os.environ.copy(), capture_output=True, text=True)
    emit({"type":"item.completed","item":{"id":identifier,"type":"command_execution","command":"multica "+" ".join(argv[:3]),"aggregated_output":p.stdout,"exit_code":p.returncode}})
    if p.returncode:
        sys.stderr.write(p.stderr)
        raise SystemExit(7)
    return p.stdout

def comment(content, parent=None):
    args = ["issue", "comment", "add", manifest["issue_id"], "--content", content, "--idempotency-key", str(uuid.uuid4()), "--output", "json"]
    if parent:
        args += ["--parent", parent]
    command(args)

def actions():
    issue = manifest["issue_id"]
    command(["issue", "get", issue, "--output", "json"])
    if manifest["role"] == "security-reviewer":
        comment("独立安全审查结果：仅审阅本地合成 supply-chain.json。生产者来源、可信摘要和组件清单均缺失，信任边界在构建产物交付处。证据不足，hold/unknown；请安全负责人收集脱敏来源证明，不能判定安全。", manifest["trigger_comment_id"])
    elif not manifest["is_leader_task"]:
        comment("直接回复智能体的评论不算放行。本轮缺少小队协议与花名册，不派活、不放行、不改状态。请在本任务发表不带任何 @ 的顶层评论，或输入 @ 从列表选中本小队，再逐项确认。", manifest["trigger_comment_id"])
    elif manifest.get("trigger_comment_id"):
        command(["issue", "comment", "list", issue, "--output", "json"])
        comment("收口：hold/unknown。已核对安全负责人本人本次新评论及修订、审查结果与顺序；本次放行通过校验。负责人决定搁置，理由是缺少摘要、来源和组件清单，复查日期2026-12-04。风险未关闭，不派修复成员。", manifest["trigger_comment_id"])
        command(["squad", "activity", issue, "action", "--reason", "Synthetic owner hold recorded without repair"])
        command(["issue", "update", issue, "--status", "in_review"])
    else:
        agent_file = Path.cwd() / "AGENTS.md"
        text = agent_file.read_text()
        matches = re.findall(r"\[@([^]]+)\]\(mention://agent/([a-f0-9-]+)\)", text)
        reviewer = next((aid for name, aid in matches if "安全审查" in name or "Security Reviewer" in name), None)
        if not reviewer:
            sys.stderr.write("Synthetic provider could not locate security reviewer in actual roster\n")
            raise SystemExit(8)
        command(["issue", "update", issue, "--status", "in_progress"])
        comment("分诊 type=other，hold/unknown。G1：[@安全审查员](mention://agent/" + reviewer + ") 请评估本地合成来源、信任边界和证据缺口。[@安全负责人](mention://member/" + manifest["owner_ids"][0] + ") 请先读审查结果，再确认定级、处置、路由、期限、复现承担者和知情范围；建议保留未知与搁置。放行请发表不带任何 @ 的顶层评论或输入 @ 从列表选择本小队，直接回复智能体评论不算放行；在队长确认校验前不要改动放行评论。")
        command(["squad", "activity", issue, "action", "--reason", "Synthetic G1 waiting for security review and owner"])

emit({"type":"thread.started","thread_id":thread})
emit({"type":"turn.started"})
actions()
emit({"type":"item.completed","item":{"id":str(uuid.uuid4()),"type":"agent_message","text":"Scripted direct CLI O1 result only, not model evidence."}})
emit({"type":"turn.completed","usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0}})
