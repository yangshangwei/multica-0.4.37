import hashlib
import json
import pathlib
import re
import shlex
import urllib.parse

root = pathlib.Path(__file__).parent

def read(name):
    return json.loads((root / name).read_text())

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def keyed(rows):
    result = {row["id"]: row for row in rows}
    if len(result) != len(rows):
        raise ValueError("duplicate snapshot identity")
    return result

def delta(first, last):
    before, after = keyed(first), keyed(last)
    return {"added": sorted(after.keys() - before.keys()),
            "removed": sorted(before.keys() - after.keys()),
            "changed": sorted(key for key in before.keys() & after.keys() if before[key] != after[key])}

def command_args(command):
    outer = shlex.split(command)
    if len(outer) != 3 or outer[:2] != ["/bin/zsh", "-lc"]:
        return []
    return shlex.split(outer[2])

def response_json(item):
    text = item.get("aggregated_output", "")
    lines = text.splitlines()
    for i, line in enumerate(lines):
        if line.startswith("{"):
            return json.loads("\n".join(lines[i:]))
    raise ValueError("command has no retained JSON response")

trial = read("leader-trial.json")
expected = trial["expected"]
before = read("leader-before-state.json")
after = read("leader-after-state.json")
ledger = read("model-trials.json")
run = [row for row in ledger if row["role"] == "leader"]
assert len(run) == 1
run = run[0]
events = [json.loads(line) for line in (root / "leader-codex-events.jsonl").read_text().splitlines()]
commands = [event["item"] for event in events if event.get("type") == "item.completed" and event.get("item", {}).get("type") == "command_execution"]
started_commands = [event["item"] for event in events if event.get("type") == "item.started" and event.get("item", {}).get("type") == "command_execution"]
trace_all = [json.loads(line) for line in (root / "live-http-trace.jsonl").read_text().splitlines()]
trace = trace_all[before["trace_lines"]:after["trace_lines"]]
issue = expected["current_issue"]
agent = trial["env"]["MULTICA_AGENT_ID"]
task = trial["claim"]["id"]
expected_routes = [("GET", "/api/issues/" + issue, 200),
                   ("POST", "/api/issues/" + issue + "/comments", 201),
                   ("POST", "/api/issues/" + issue + "/squad-evaluated", 201)]
request_ok = len(trace) == 3 and [(row["method"], urllib.parse.unquote(urllib.parse.urlsplit(row["path"]).path), row["status"]) for row in trace] == expected_routes and all(row["role"] == "leader" and not urllib.parse.urlsplit(row["path"]).query for row in trace)
changes = {name: delta(before[name], after[name]) for name in ("issues", "comments", "tasks", "evaluations")}
comments = [keyed(after["comments"])[key] for key in changes["comments"]["added"]]
tasks = [keyed(after["tasks"])[key] for key in changes["tasks"]["added"]]
evaluations = [keyed(after["evaluations"])[key] for key in changes["evaluations"]["added"]]
args = [command_args(item["command"]) for item in commands]
cli = str(root / "bin/multica")
command_ok = len(commands) == 3 and len(started_commands) == 3 and all(item.get("exit_code") == 0 for item in commands)
command_ok = command_ok and args[0] == [cli, "issue", "get", issue, "--output", "json"]
command_ok = command_ok and len(args[1]) == 9 and args[1][:6] == [cli, "issue", "comment", "add", issue, "--content"] and args[1][7:] == ["--output", "json"]
command_ok = command_ok and len(args[2]) == 9 and args[2][:6] == [cli, "squad", "activity", issue, "failed", "--reason"] and args[2][7:] == ["--output", "json"]
responses = [response_json(item) for item in commands]
receipts = responses[1].get("trigger_outcomes", [])
receipt_ok = len(receipts) == 2 and {tuple(row.get(k) for k in ("target_type", "target_id", "status", "reason_code")) for row in receipts} == {
    ("agent", expected["allowed_target"], "queued", "queued"),
    ("agent", expected["blocked_target"], "blocked", "invocation_not_allowed")}
warning_ok = "1 handled, 1 blocked, 0 unconfirmed" in commands[1].get("aggregated_output", "") and "Do not repost" in commands[1].get("aggregated_output", "")
comment_ok = len(comments) == 1 and not changes["comments"]["removed"] and not changes["comments"]["changed"]
if comment_ok:
    comment = comments[0]
    mentions = re.findall(r"\]\(mention://agent/([0-9a-f-]+)\)", comment["content"])
    comment_ok = comment["issue_id"] == issue and comment["author_type"] == "agent" and comment["author_id"] == agent and comment["source_task_id"] == task and comment["dispatch_eligible"] is True and sorted(mentions) == sorted([expected["allowed_target"], expected["blocked_target"]])
    comment_ok = comment_ok and responses[1].get("id") == comment["id"] and responses[1].get("content") == comment["content"] == args[1][6]
    comment_ok = comment_ok and responses[1].get("author_id") == agent and responses[1].get("source_task_id") == task
else:
    comment = {}
task_ok = len(tasks) == 1 and not changes["tasks"]["removed"] and not changes["tasks"]["changed"]
if task_ok:
    child = tasks[0]
    parent = keyed(before["tasks"])[task]
    task_ok = child["agent_id"] == expected["allowed_target"] and child["issue_id"] == issue and child["trigger_comment_id"] == comment.get("id") and child["status"] == "queued" and child["originator_user_id"] == parent["originator_user_id"]
evaluation_ok = len(evaluations) == 1 and not changes["evaluations"]["removed"] and not changes["evaluations"]["changed"]
if evaluation_ok:
    evaluation = evaluations[0]
    details = evaluation.get("details", {})
    reason = details.get("reason", "")
    evaluation_ok = evaluation["actor_type"] == "agent" and evaluation["actor_id"] == agent and evaluation["action"] == "squad_leader_evaluated"
    evaluation_ok = evaluation_ok and details.get("outcome") == "failed" and details.get("task_id") == task and details.get("squad_id") == expected["squad_id"]
    evaluation_ok = evaluation_ok and responses[2].get("id") == evaluation["id"] and responses[2].get("action") == "squad_leader_evaluated" and args[2][6] == reason
    evaluation_ok = evaluation_ok and all(text in reason for text in [expected["allowed_target"], expected["blocked_target"], "queued", "blocked", "invocation_not_allowed", "admission only", "not work started or completed", "not retried"])
else:
    reason = ""
issue_ok = not any(changes["issues"].values()) and keyed(after["issues"])[issue]["status"] == "in_progress"
prompt_ok = before["prompt_sha256"] == after["prompt_sha256"] == run["prompt_sha256"] == trial["input_selection"]["prompt_sha256"] == digest(root / "leader-prompt.md")
claim_ok = trial["claim"]["is_leader_task"] is True and trial["claim"]["agent"]["instructions"] in (root / "leader-prompt.md").read_text() and hashlib.sha256(trial["claim"]["agent"]["instructions"].encode()).hexdigest() == trial["input_selection"]["agent_instructions_sha256"]
cli_ok = trial["cli_hash"] == digest(root / "bin/multica")
event_ok = run["event_sha256"] == digest(root / "leader-codex-events.jsonl")
cwd_ok = before["cwd_files"] == after["cwd_files"]
completed_turns = [event for event in events if event.get("type") == "turn.completed"]
final_exists = (root / "leader-codex-final.txt").exists()
observed_actions = all([request_ok, command_ok, receipt_ok, warning_ok, comment_ok, task_ok, evaluation_ok, issue_ok, prompt_ok, claim_ok, cli_ok, event_ok, cwd_ok])
process_completed = run.get("exit") == 0 and not run.get("timeout") and len(completed_turns) == 1 and final_exists
budget_used = len(ledger)
report = {
    "role": "leader", "observed_business_actions_verdict": "pass" if observed_actions else "fail",
    "process_completion_verdict": "pass" if process_completed else "fail",
    "overall_complete_trial_verdict": "pass" if observed_actions and process_completed else "fail",
    "process": {key: run.get(key) for key in ["exit", "timeout", "timeout_seconds", "elapsed_seconds", "status"]},
    "turn_completed_count": len(completed_turns), "final_response_exists": final_exists,
    "usage": completed_turns[0].get("usage") if completed_turns else "unknown: no completed-turn usage event",
    "invocation_budget": {"used": budget_used, "maximum": 3, "exhausted": budget_used == 3, "additional_run_permitted": False},
    "checks": {"requests": request_ok, "commands": command_ok, "actual_receipts": receipt_ok, "cli_partial_warning": warning_ok,
               "one_current_leader_comment": comment_ok, "one_allowed_queued_task_with_lineage": task_ok,
               "one_exact_actor_task_squad_failed_evaluation": evaluation_ok, "all_recorded_issue_fields_unchanged": issue_ok,
               "prompt_identity": prompt_ok, "actual_claim_instructions": claim_ok, "compiled_cli_identity": cli_ok,
               "event_stream_identity": event_ok, "recorded_cwd_files_unchanged": cwd_ok},
    "full_record_deltas": changes,
    "actual_receipts": receipts, "evaluation_reason": reason,
    "request_offsets_seconds": [round(row["time"] - run["started_at"], 3) for row in trace],
    "seconds_from_evaluation_response_to_forced_stop": round(run["started_at"] + run["elapsed_seconds"] - trace[-1]["time"], 3),
    "snapshot_scope": {"issues": "selected business fields across synthetic workspace", "comments_tasks_evaluations": "current leader issue only"},
    "limitations": ["Guided partial-receipt sample: task request explicitly prescribed both workers and partial-failed behavior.",
                    "No turn.completed/final response; committed API actions do not convert timeout into completed-model success.",
                    "No supplied evidence attributes final15 seconds to a Multica bug, model stall or provider transport; cause remains unknown.",
                    "Only captured fields are compared; final cwd equality is not a host sandbox or transient-write guarantee.",
                    "Queued worker task was not claimed or executed; admission is not worker completion.",
                    "No leader final usage telemetry; unknown consumption must not be reported as zero cost."],
    "evidence_hashes": {name: digest(root / name) for name in ["leader-before-state.json", "leader-after-state.json", "leader-codex-events.jsonl", "leader-codex-stderr.log", "leader-prompt.md", "leader-oracle.py", "leader-oracle.json", "prepare-leader-fixture.py", "model-trials.json", "run-codex-trial.py"]},
    "independent_oracle_sha256": digest(pathlib.Path(__file__)),
}
output = root / "leader-independent-oracle01.json"
with output.open("x") as file:
    json.dump(report, file, ensure_ascii=False, indent=2)
    file.write("\n")
output.chmod(0o600)
print(json.dumps({key: report[key] for key in ["role", "observed_business_actions_verdict", "process_completion_verdict", "overall_complete_trial_verdict", "turn_completed_count", "final_response_exists", "invocation_budget"]}))
