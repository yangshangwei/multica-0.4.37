#!/usr/bin/env python3
"""Cross-check captured product records, not the simulator's success message."""

import argparse
import json
from pathlib import Path
import re


SEQUENCES = {
    "other-supply-chain": ["security-reviewer"],
    "dependency-advisory": ["product-analyst", "security-reviewer", "migration-reviewer",
                            "implementer", "qa-engineer", "code-reviewer"],
    "code-authorization": ["security-reviewer", "diagnostician", "implementer",
                           "qa-engineer", "code-reviewer"],
    "production-escalation": [],
}
MARKER = re.compile(r"^SIMULATION_RECORD (\{[^\n]+\})$", re.MULTILINE)


def validate(directory):
    directory = Path(directory)
    state = json.loads((directory / "simulation-state.json").read_text())
    events = [json.loads(line) for line in (directory / "simulation-actions.jsonl").read_text().splitlines() if line]
    roles = state["roles"]
    assert len(roles) == len(set(roles.values())) == 8, "Role identities must remain distinct"
    assert state["mode"] == "scripted_simulation"
    assert type(state["real_model_calls"]) is int and state["real_model_calls"] == 0, "Model calls contradict simulation evidence"
    boundary = json.loads((directory / "simulation-boundary.json").read_text())
    assert boundary["real_agent_binary_installed"] is False
    assert all(boundary[key] is True for key in ("model_route_denied", "external_route_denied", "external_dns_denied"))
    assert boundary["human_profile_readable"] is False and boundary["uid"] == 1000
    summaries, failed_attempts = {}, {}
    all_tasks = {}
    for case, sequence in SEQUENCES.items():
        snapshot = json.loads((directory / ("simulation-" + case + ".json")).read_text())
        issue = snapshot["issue"]
        assert issue["id"] == state["case_ids"][case]["id"] and issue["status"] == "in_review"
        assert issue["workspace_id"] == state["workspace_id"]
        assert issue["assignee_type"] == "squad" and issue["assignee_id"] == state["squad_id"]
        tasks = {task["id"]: task for task in snapshot["task_runs"]}
        assert len(tasks) == len(snapshot["task_runs"])
        assert not tasks.keys() & all_tasks.keys(), "Task reused across simulation cases"
        all_tasks.update(tasks)
        assert all(task["status"] in ("completed", "failed") for task in tasks.values()), "Unsettled task"
        for task in tasks.values():
            assert task["runtime_id"] == state["runtime_id"] and task["agent_id"] in roles.values()
            assert task["issue_id"] == issue["id"] and task["workspace_id"] == state["workspace_id"]
            if task["status"] == "failed":
                failed_attempts[task["id"]] = {"case": case, "error": task["error"]}
        comments = {comment["id"]: comment for comment in snapshot["comments"]}
        assert len(comments) == len(snapshot["comments"]), "Duplicate product comment ID"
        records = {}
        for comment_id, comment in comments.items():
            found = MARKER.findall(comment.get("content", ""))
            if not found:
                continue
            assert len(found) == 1
            record = json.loads(found[0])
            assert record["simulation"] == state["simulation_id"]
            assert comment["author_type"] == "agent"
            assert "SCRIPTED SIMULATION" in comment["content"]
            assert comment["issue_id"] == issue["id"]
            assert comment["source_task_id"] == record["task"], "Marker must match the product source task"
            assert record["task"] in tasks, "Comment source task is missing"
            task = tasks[record["task"]]
            assert task["status"] == "completed" and task["agent_id"] == comment["author_id"]
            audit = [event for event in events if event.get("operation") == "comment" and event.get("comment_id") == comment_id]
            assert len(audit) == 1
            assert audit[0]["task_id"] == task["id"] and audit[0]["agent_id"] == comment["author_id"]
            assert audit[0]["content"] == comment["content"]
            records[comment_id] = record
        completed = {}
        for comment_id, record in records.items():
            if record["kind"] != "result":
                continue
            stage = record["stage"]
            assert type(stage) is int and 0 <= stage < len(sequence)
            assert stage not in completed and sequence[stage] == record["role"]
            comment, member = comments[comment_id], tasks[record["task"]]
            assert member["agent_id"] == roles[sequence[stage]] and not member.get("is_leader_task", False)
            attribution = member["attribution"]
            assert attribution["source"] == "delegation" and attribution["precise"] is True
            trigger_id = member["trigger_comment_id"]
            trigger, dispatch = comments[trigger_id], records[trigger_id]
            assert dispatch["kind"] == "dispatch" and dispatch["stage"] == stage and dispatch["role"] == sequence[stage]
            assert trigger["author_id"] == roles["maintenance-lead"]
            assert attribution["delegated_from_task_id"] == dispatch["task"]
            assert attribution["evidence"]["ref_id"] == trigger_id
            assert tasks[dispatch["task"]]["is_leader_task"] is True
            assert comment["parent_id"] == trigger_id
            returns = [task for task in tasks.values() if task["status"] == "completed"
                and task.get("is_leader_task") is True and task["agent_id"] == roles["maintenance-lead"]
                and task.get("trigger_comment_id") == comment_id
                and task.get("attribution", {}).get("source") == "delegation"
                and task.get("attribution", {}).get("precise") is True
                and task.get("attribution", {}).get("evidence", {}).get("kind") == "comment"
                and task.get("attribution", {}).get("evidence", {}).get("ref_id") == comment_id
                and task.get("attribution", {}).get("delegated_from_task_id") == member["id"]]
            assert len(returns) == 1, "Member result must cause one actual leader return"
            completed[stage] = {"member_task": member["id"], "return_task": returns[0]["id"]}
        assert sorted(completed) == list(range(len(sequence)))
        finals = [(comments[cid], record) for cid, record in records.items() if record["kind"] == "final"]
        assert len(finals) == 1 and finals[0][1]["stage"] == len(sequence)
        final_comment, final_record = finals[0]
        assert final_comment["author_id"] == roles["maintenance-lead"]
        assert final_record["role"] == "maintenance-lead" and tasks[final_record["task"]].get("is_leader_task") is True
        if sequence:
            assert final_record["task"] == completed[len(sequence) - 1]["return_task"]
        evaluations = [item for item in snapshot["timeline"] if item.get("action") == "squad_leader_evaluated"]
        outcomes = {}
        for item in evaluations:
            assert item["actor_type"] == "agent" and item["actor_id"] == roles["maintenance-lead"]
            assert item["details"]["squad_id"] == state["squad_id"]
            task_id = item["details"]["task_id"]
            assert task_id not in outcomes and tasks[task_id]["status"] == "completed"
            outcomes[task_id] = item["details"]["outcome"]
        for record in records.values():
            if record["kind"] == "dispatch":
                assert outcomes[record["task"]] == "action"
        assert outcomes[final_record["task"]] == "no_action"
        assert list(outcomes.values()).count("no_action") == 1
        assert not any(item.get("details", {}).get("to") == "done" for item in snapshot["timeline"])
        if case == "other-supply-chain":
            assert "hold/unknown" in final_comment["content"] and "pilot-build-owner" in final_comment["content"]
            controls = [event for event in events if event.get("issue_id") == issue["id"] and event.get("operation") == "malformed_mention_rejected"]
            assert len(controls) == 1
            assert "NEGATIVE CONTROL" in comments[controls[0]["comment_id"]]["content"]
        if case == "production-escalation":
            assert len(tasks) == 1 and all(task["agent_id"] == roles["maintenance-lead"] for task in tasks.values())
            assert "human incident owner" in final_comment["content"]
            assert not any(event.get("issue_id") == issue["id"] and event.get("operation") == "fixture_command" for event in events)
        summaries[case] = {"identifier": issue["identifier"], "status": issue["status"],
            "verified_member_stages": len(completed), "completed_tasks": sum(task["status"] == "completed" for task in tasks.values()),
            "failed_attempts_retained": sum(task["status"] == "failed" for task in tasks.values()),
            "member_return_links": completed, "activity_outcomes": outcomes}
    assert set(failed_attempts) == set(state["acknowledged_failures"]), "Unexpected failure must not be ignored"
    for task_id, failure in failed_attempts.items():
        assert state["acknowledged_failures"][task_id]["case"] == failure["case"]
        assert state["acknowledged_failures"][task_id]["error"] == failure["error"]
    for event in events:
        task = all_tasks[event["task_id"]]
        assert event["mode"] == "scripted_simulation" and event["simulation"] == state["simulation_id"]
        assert event["agent_id"] == task["agent_id"] == roles[event["role"]], "Audit actor must match the product task"
        assert event["issue_id"] == task["issue_id"]
    commands = [event for event in events if event.get("operation") == "fixture_command"]
    diagnostic = [event for event in commands if event["role"] == "diagnostician" and "unittest" in event["command"]]
    assert len(diagnostic) == 1 and diagnostic[0]["exit_code"] == 1 and "FAILED (failures=1)" in diagnostic[0]["stderr"]
    assert "Ran 3 tests" in diagnostic[0]["stderr"]
    qa = [event for event in commands if event["role"] == "qa-engineer" and "unittest" in event["command"]]
    assert len(qa) == 2
    for event in qa:
        case = next(key for key, value in state["case_ids"].items() if value["id"] == event["issue_id"])
        count = 3 if case == "dependency-advisory" else 6
        assert event["exit_code"] == 0 and f"Ran {count} tests" in event["stderr"] and "\nOK\n" in event["stderr"]
        assert all_tasks[event["task_id"]]["agent_id"] == roles["qa-engineer"]
    repairs = [event for event in commands if event["command"][:2] == ["git", "apply"]]
    assert len(repairs) == 2 and all(event["role"] == "implementer" and event["exit_code"] == 0 for event in repairs)
    assert all(task["agent_id"] == roles["maintenance-lead"] or not task.get("is_leader_task", False) for task in all_tasks.values())
    return {"mode": "scripted_simulation", "verified_cases": len(summaries), "cases": summaries,
            "successful_tasks": sum(task["status"] == "completed" for task in all_tasks.values()),
            "acknowledged_failed_attempts": failed_attempts, "diagnostic_failure_reproduced": True,
            "qa_test_counts": [3, 6], "model_behavior_verified": False, "real_runtime_invoked": False}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("directory", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    result = validate(args.directory)
    text = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.write_text(text)
    else:
        print(text, end="")
