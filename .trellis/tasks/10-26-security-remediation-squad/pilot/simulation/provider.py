#!/usr/bin/env python3
"""A finite app-server double; decisions are scripted and no model is called."""

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
import uuid


LABEL = "SCRIPTED SIMULATION — NO MODEL"
DEFAULT_CONFIG = Path("/work/simulation/config.json")
STAGES = {
    "other-supply-chain": ["security-reviewer"],
    "dependency-advisory": ["product-analyst", "security-reviewer", "migration-reviewer",
                            "implementer", "qa-engineer", "code-reviewer"],
    "code-authorization": ["security-reviewer", "diagnostician", "implementer",
                           "qa-engineer", "code-reviewer"],
    "production-escalation": [],
}
ROLE_KEYS = {"maintenance-lead", *sum(STAGES.values(), [])}
MARKER = re.compile(r"^SIMULATION_RECORD (\{[^\n]+\})$", re.MULTILINE)


def load_config(path=DEFAULT_CONFIG):
    config = json.loads(Path(path).read_text())
    roles = config["roles"]
    if set(roles) != ROLE_KEYS or len(set(roles.values())) != len(roles):
        raise ValueError("Exactly eight distinct role identities are required")
    for value in [config["workspace_id"], config["squad_id"], *roles.values(), *config["cases"]]:
        if str(uuid.UUID(value)) != value:
            raise ValueError("Noncanonical UUID in simulation configuration")
    if not re.fullmatch(r"[a-zA-Z0-9_-]{1,80}", config["simulation_id"]):
        raise ValueError("Invalid simulation identifier")
    if any(case["key"] not in STAGES for case in config["cases"].values()):
        raise ValueError("Unknown simulation case")
    if Path(config["lab_path"]).resolve() != Path("/work/lab"):
        raise ValueError("Only the synthetic /work/lab is allowed")
    if Path(config["patch_path"]).resolve() != Path("/work/simulation/lab-diff.patch"):
        raise ValueError("Only the recorded synthetic patch is allowed")
    return config


def record_marker(simulation, kind, role, stage, task):
    return "SIMULATION_RECORD " + json.dumps({"simulation": simulation, "kind": kind,
        "role": role, "stage": stage, "task": task}, sort_keys=True)


def marker(comment, simulation, kind):
    found = MARKER.findall(comment.get("content", ""))
    if len(found) != 1:
        return None
    data = json.loads(found[0])
    return data if data.get("simulation") == simulation and data.get("kind") == kind else None


def as_rows(data, key):
    return data if isinstance(data, list) else data[key]


class SimulationTurn:
    def __init__(self, config, prompt):
        self.config = config
        self.agent = os.environ["MULTICA_AGENT_ID"]
        self.task_id = os.environ["MULTICA_TASK_ID"]
        self.token = os.environ["MULTICA_TOKEN"]
        if not self.token.startswith("mat_"):
            raise ValueError("A real task-scoped credential is required, never a human credential")
        if os.environ["MULTICA_WORKSPACE_ID"] != config["workspace_id"]:
            raise ValueError("Task workspace differs from the simulation")
        self.base = os.environ["MULTICA_SERVER_URL"].rstrip("/")
        if self.base != config["api_url"].rstrip("/"):
            raise ValueError("Only the configured development API gateway is allowed")
        parsed = urllib.parse.urlparse(self.base)
        if parsed.scheme != "http" or parsed.username or parsed.password:
            raise ValueError("Expected credential-free internal HTTP gateway address")
        matches = [issue for issue in config["cases"] if issue in prompt]
        if len(matches) != 1:
            raise ValueError("Prompt must identify exactly one configured simulation issue")
        self.issue = matches[0]
        self.case = config["cases"][self.issue]["key"]
        self.role = next((k for k, v in config["roles"].items() if v == self.agent), None)
        if self.role is None:
            raise ValueError("Task agent is outside the simulation roster")
        self.tasks = as_rows(self.get("task-runs"), "tasks")
        current = [t for t in self.tasks if t["id"] == self.task_id and t["agent_id"] == self.agent]
        if len(current) != 1 or current[0]["status"] != "running":
            raise ValueError("No matching active product task")
        self.task = current[0]
        self.parent = self.task.get("trigger_comment_id")
        if not self.parent:
            raise ValueError("Simulation requires an actual comment-triggered task")
        self.comments = as_rows(self.get("comments"), "comments")
        self.lab = Path(config["lab_path"])
        self.audit_path = Path(config.get("audit_path", "/work/simulation/audit/events.jsonl"))
        if self.audit_path.resolve() != Path("/work/simulation/audit/events.jsonl"):
            raise ValueError("Audit path is outside the simulation")

    def get(self, suffix):
        url = self.base + "/api/issues/" + self.issue + "/" + suffix
        req = urllib.request.Request(url, headers={"Authorization": "Bearer " + self.token,
            "X-Workspace-ID": self.config["workspace_id"], "X-Agent-ID": self.agent,
            "X-Task-ID": self.task_id})
        with urllib.request.urlopen(req, timeout=15) as response:
            data = response.read(4 * 1024 * 1024 + 1)
        if len(data) > 4 * 1024 * 1024:
            raise ValueError("Simulation read exceeded its bound")
        return json.loads(data)

    def audit(self, operation, **detail):
        event = {"mode": "scripted_simulation", "simulation": self.config["simulation_id"],
                 "task_id": self.task_id, "agent_id": self.agent, "issue_id": self.issue,
                 "role": self.role, "operation": operation, **detail}
        encoded = json.dumps(event, ensure_ascii=False)
        if self.token in encoded:
            raise ValueError("Credential must not enter simulation evidence")
        with self.audit_path.open("a") as out:
            out.write(encoded + "\n")

    def cli(self, *args):
        # The CLI owns a 30s transport timeout plus a 5s context grace.
        # Let it report that outcome before the outer process deadline.
        result = subprocess.run(["/usr/local/bin/multica", *args, "--output", "json"],
                                text=True, capture_output=True, timeout=40)
        self.audit("multica_cli", arguments=list(args), exit_code=result.returncode)
        if result.returncode:
            raise RuntimeError("Multica CLI rejected simulation operation: " +
                               result.stderr.replace(self.token, "[REDACTED]")[:1200])
        return json.loads(result.stdout) if result.stdout.strip() else None

    def comment(self, content):
        with tempfile.NamedTemporaryFile(mode="w", prefix=".simulation-comment-", suffix=".md", dir=Path.cwd(),
                                         encoding="utf-8") as out:
            out.write(content)
            out.flush()
            result = self.cli("issue", "comment", "add", self.issue,
                              "--parent", self.parent, "--content-file", out.name)
        self.audit("comment", comment_id=result["id"], parent_id=self.parent,
                   content=content, trigger_outcomes=result.get("trigger_outcomes", []))
        return result

    def triage(self, owner, final=False):
        kind = {"dependency-advisory": "dependency", "code-authorization": "code"}.get(self.case, "other")
        route = {"dependency-advisory": "maintenance", "code-authorization": "bug-fix",
                 "other-supply-chain": "human", "production-escalation": "incident"}[self.case]
        evidence = "confirmed" if final and kind != "other" else "unknown"
        return "\n".join(["## Security triage", "- type: " + kind,
            "- source: synthetic local fixture; scripted assessment",
            "- affected_surface: isolated synthetic lab only", "- severity: unknown",
            "- exploitability: unknown", "- evidence_state: " + evidence,
            "- production_impact: no", "- route: " + route,
            "- blocked_actions: production, secret, external notice, destructive",
            "- next_owner: " + owner])

    def completed_stages(self):
        results = {}
        stages = STAGES[self.case]
        by_id = {task["id"]: task for task in self.tasks}
        comments = {comment["id"]: comment for comment in self.comments}
        for comment in self.comments:
            data = marker(comment, self.config["simulation_id"], "result")
            if not data:
                continue
            stage = data.get("stage")
            if not isinstance(stage, int) or stage < 0 or stage >= len(stages):
                raise ValueError("Out-of-range simulation result stage")
            role = stages[stage]
            task = by_id.get(data.get("task"))
            if data.get("role") != role or comment.get("author_type") != "agent" or \
                    comment.get("author_id") != self.config["roles"][role]:
                raise ValueError("Simulation result author differs from the required role")
            if not task or task.get("agent_id") != comment["author_id"] or task["status"] != "completed":
                raise ValueError("Simulation result has no completed matching product task")
            if task.get("attribution", {}).get("source") != "delegation":
                raise ValueError("A manually dispatched result is not a leader delegation")
            trigger = comments.get(task.get("trigger_comment_id"), {})
            delegated = marker(trigger, self.config["simulation_id"], "dispatch")
            if trigger.get("author_id") != self.config["roles"]["maintenance-lead"] or \
                    not delegated or delegated.get("stage") != stage or delegated.get("role") != role:
                raise ValueError("Member result does not link to the actual leader dispatch")
            if stage in results:
                raise ValueError("Duplicate completed stage requires operator inspection")
            results[stage] = task["id"]
        if sorted(results) != list(range(len(results))):
            raise ValueError("Simulation stages completed out of order")
        return results

    def run(self):
        self.audit("turn_started", case=self.case, parent_id=self.parent)
        stages = STAGES[self.case]
        if self.role == "maintenance-lead":
            if self.task.get("is_leader_task") is not True:
                raise ValueError("Coordinator was not claimed as a genuine squad leader")
            completed = self.completed_stages()
            if len(self.tasks) > 30:
                raise ValueError("Simulation round bound exceeded")
            if len(completed) < len(stages):
                stage = len(completed)
                role = stages[stage]
                if self.case == "other-supply-chain" and stage == 0:
                    broken = LABEL + "\nNEGATIVE CONTROL: intentionally missing Markdown brackets.\n"
                    broken += "@security-reviewer(mention://agent/" + self.config["roles"][role] + ")"
                    result = self.comment(broken)
                    if any(x.get("target_id") == self.config["roles"][role] and
                           x.get("status") in ("queued", "deferred", "coalesced")
                           for x in result.get("trigger_outcomes", [])):
                        raise ValueError("Malformed mention unexpectedly dispatched a member")
                    runs = as_rows(self.get("task-runs"), "tasks")
                    if any(x["agent_id"] == self.config["roles"][role] for x in runs):
                        raise ValueError("Malformed mention produced an actual member task")
                    self.audit("malformed_mention_rejected", comment_id=result["id"])
                body = LABEL + "\n" + record_marker(self.config["simulation_id"], "dispatch", role, stage, self.task_id)
                body += "\n\n" + self.triage(role)
                body += "\n\n[@" + role + "](mention://agent/" + self.config["roles"][role] + ") "
                body += "Execute only your configured scripted fixture stage and post your own marked result in this thread."
                result = self.comment(body)
                outcomes = result.get("trigger_outcomes", [])
                if not any(x.get("target_id") == self.config["roles"][role] and
                           x.get("status") in ("queued", "deferred", "coalesced") for x in outcomes):
                    raise ValueError("Product did not accept the exact member dispatch")
                self.cli("squad", "activity", self.issue, "action", "--reason",
                         LABEL + ": dispatched stage " + str(stage))
                return LABEL + ": dispatched one member and ended the coordinator turn."
            owner = {"other-supply-chain": "pilot-build-owner",
                     "production-escalation": "human incident owner"}.get(self.case, "human reviewer")
            body = LABEL + "\n" + record_marker(self.config["simulation_id"], "final", self.role, len(stages), self.task_id)
            body += "\n\n" + self.triage(owner, final=True)
            body += "\n\nCompleted product task IDs: " + json.dumps(completed, sort_keys=True)
            if self.case == "other-supply-chain":
                body += "\nDecision: hold/unknown. Owner pilot-build-owner must provide trusted digest, producer provenance and component inventory before release."
            elif self.case == "production-escalation":
                body += "\nSimulated incident/release handoff to a human incident owner. No operator task, production command, credential operation or external notification is authorized or executed."
            else:
                body += "\nScripted patch replay and independent fixture checks completed; human review required. Model judgment and instruction following were not tested."
            self.comment(body)
            self.cli("issue", "update", self.issue, "--status", "in_review")
            self.cli("squad", "activity", self.issue, "no_action", "--reason", LABEL + ": finite sequence finished; human review")
            return LABEL + ": sequence settled at in_review; real-model gate remains open."
        trigger = next((x for x in self.comments if x["id"] == self.parent), {})
        dispatch = marker(trigger, self.config["simulation_id"], "dispatch")
        if trigger.get("author_id") != self.config["roles"]["maintenance-lead"] or not dispatch:
            raise ValueError("Member did not receive a genuine scripted leader comment")
        stage = dispatch.get("stage")
        if not isinstance(stage, int) or stage < 0 or stage >= len(stages) or stages[stage] != self.role or dispatch.get("role") != self.role:
            raise ValueError("Member stage/role mismatch")
        if self.task.get("attribution", {}).get("source") != "delegation":
            raise ValueError("Member task is not a real delegation")
        report = self.perform_stage()
        body = LABEL + "\n" + record_marker(self.config["simulation_id"], "result", self.role, stage, self.task_id)
        body += "\n\n" + self.triage("maintenance-lead", final=self.role in ("qa-engineer", "code-reviewer"))
        body += "\n\n" + report
        result = self.comment(body)
        return LABEL + ": role result submitted in comment " + result["id"]

    def read_fixture(self, name):
        path = (self.lab / name).resolve()
        if not path.is_relative_to(self.lab) or not path.is_file():
            raise ValueError("Fixture path escaped the synthetic lab")
        content = path.read_text()
        return {"path": name, "sha256": hashlib.sha256(content.encode()).hexdigest(), "content": content}

    def command(self, command, expected=0):
        result = subprocess.run(command, cwd=self.lab, text=True, capture_output=True, timeout=30)
        evidence = {"command": command, "exit_code": result.returncode, "stdout": result.stdout, "stderr": result.stderr}
        self.audit("fixture_command", **evidence)
        if result.returncode != expected:
            raise RuntimeError("Unexpected fixture command result: " + json.dumps(evidence))
        return evidence

    def perform_stage(self):
        evidence = {"assessment": "predefined simulation logic; not an AI assessment", "role": self.role}
        if self.case == "other-supply-chain":
            fixture = self.read_fixture("supply-chain.json")
            data = json.loads(fixture["content"])
            if not all(data[k] is None for k in ("artifact_sha256", "provenance", "sbom")) or data["network_execution_permitted"] is not False:
                raise ValueError("Supply-chain fixture no longer expresses the expected evidence gap")
            evidence.update(decision="hold", evidence_state="unknown", next_owner=data["next_owner"],
                            required_evidence=data["required_evidence"], source_sha256=fixture["sha256"])
        elif self.role in ("product-analyst", "security-reviewer", "migration-reviewer"):
            files = ["advisory.json", "dependency.lock.json", "application.py"] if self.case == "dependency-advisory" else ["workspace_reader.py", "test_authorization.py"]
            if self.role == "migration-reviewer":
                files += ["vendor/labelkit_v1.py", "vendor/labelkit_v2.py"]
                evidence["contract"] = "Fixture v1 returns .label; v2 returns .text. Caller and lockfile must migrate together. No database/client migration is performed."
            evidence["sources"] = [{k: v for k, v in self.read_fixture(name).items() if k != "content"} for name in files]
            evidence["decision"] = "Proceed only with the configured synthetic diagnostic/repair stages."
        elif self.role == "diagnostician":
            result = self.command(["python3", "-B", "-m", "unittest", "-v", "test_authorization"], expected=1)
            if "Ran 3 tests" not in result["stderr"] or "FAILED (failures=1)" not in result["stderr"]:
                raise ValueError("Diagnostic failure must be the single expected assertion failure")
            evidence.update(commands=[result], diagnosis="Scripted interpretation: cross-workspace access must be rejected before the read audit side effect. No patch made by diagnostician.")
        elif self.role == "implementer":
            files = ["application.py", "dependency.lock.json"] if self.case == "dependency-advisory" else ["workspace_reader.py"]
            result = self.command(["git", "apply", *["--include=" + name for name in files], self.config["patch_path"]])
            evidence.update(commands=[result], changed_scope=files, repair_origin="Replayed previously recorded synthetic patch; not generated by a model.")
        elif self.role == "qa-engineer":
            modules = ["test_dependency"] if self.case == "dependency-advisory" else ["test_dependency", "test_authorization"]
            results = [self.command(["python3", "-B", "validate_fixture.py"]),
                       self.command(["python3", "-B", "-m", "unittest", "-v", *modules])]
            count = 3 if len(modules) == 1 else 6
            if f"Ran {count} tests" not in results[-1]["stderr"] or "\nOK\n" not in results[-1]["stderr"]:
                raise ValueError("QA test count or success output differs from the expected suite")
            evidence.update(commands=results, passed_tests=count, independent_actor=self.agent)
        elif self.role == "code-reviewer":
            expected = {"application.py", "dependency.lock.json"}
            if self.case == "code-authorization":
                expected.add("workspace_reader.py")
            result = self.command(["git", "diff", "--name-only"])
            if set(result["stdout"].splitlines()) != expected:
                raise ValueError("Tracked patch scope differs from the configured repair")
            baseline = json.loads(self.read_fixture("baseline-source-sha256.json")["content"])["sha256"]
            unchanged = [name for name in baseline if name not in expected and name != ".gitignore"]
            for name in unchanged:
                if self.read_fixture(name)["sha256"] != baseline[name]:
                    raise ValueError("Unchanged fixture bytes drifted: " + name)
            evidence.update(commands=[result], unchanged_sources_checked=unchanged, changed_scope=sorted(expected))
        else:
            raise ValueError("No scripted action for this role")
        evidence["code_ref"] = self.command(["git", "rev-parse", "HEAD"])["stdout"].strip()
        evidence["residual_risk"] = "Synthetic checks only; no production vulnerability assessment or model behavior proof."
        self.audit("stage_result", evidence=evidence)
        return "```json\n" + json.dumps(evidence, ensure_ascii=False, indent=2) + "\n```"


def emit(value):
    print(json.dumps(value, ensure_ascii=False), flush=True)


def serve():
    thread = str(uuid.uuid4())
    for line in sys.stdin:
        message = json.loads(line)
        method, request_id = message.get("method"), message.get("id")
        params = message.get("params") or {}
        if method == "initialize":
            result = {"userAgent": "scripted-security-simulation/1", "platformFamily": "unix"}
        elif method in ("thread/start", "thread/resume"):
            thread = params.get("threadId") or thread
            result = {"thread": {"id": thread}, "model": "scripted-simulation-no-model"}
        elif method == "model/list":
            result = {"data": [{"id": "scripted-simulation-no-model", "model": "scripted-simulation-no-model",
                "displayName": LABEL, "description": LABEL, "isDefault": True,
                "defaultReasoningEffort": "low", "supportedReasoningEfforts": [{"reasoningEffort": "low", "description": "Scripted"}]}], "nextCursor": None}
        elif method == "turn/start":
            turn = str(uuid.uuid4())
            emit({"id": request_id, "result": {"turn": {"id": turn, "status": "inProgress"}}})
            emit({"method": "turn/started", "params": {"threadId": thread, "turn": {"id": turn, "status": "inProgress"}}})
            try:
                config = load_config(os.environ.get("SECURITY_SQUAD_SIM_CONFIG", str(DEFAULT_CONFIG)))
                prompt = "\n".join(x.get("text", "") for x in params.get("input", []) if x.get("type") == "text")
                output = SimulationTurn(config, prompt).run()
                emit({"method": "item/completed", "params": {"threadId": thread, "turnId": turn,
                    "item": {"id": "simulation-result", "type": "agentMessage", "text": output}}})
                status, error = "completed", None
            except Exception as exc:
                status, error = "failed", {"message": LABEL + ": " + str(exc)[:1800]}
            emit({"method": "turn/completed", "params": {"threadId": thread,
                "turn": {"id": turn, "status": status, "error": error}}})
            continue
        elif method in ("initialized", "thread/name/set", "thread/unsubscribe", "turn/interrupt", "account/read", "config/read"):
            result = {}
        else:
            if request_id is not None:
                emit({"id": request_id, "error": {"code": -32601, "message": "Unsupported simulation method"}})
            continue
        if request_id is not None:
            emit({"id": request_id, "result": result})


if __name__ == "__main__":
    if "--version" in sys.argv:
        print("codex-cli 0.164.0 (SCRIPTED SIMULATION; no Codex binary)")
    elif "app-server" in sys.argv:
        serve()
    else:
        raise SystemExit("Only simulation version and app-server protocol are supported")
