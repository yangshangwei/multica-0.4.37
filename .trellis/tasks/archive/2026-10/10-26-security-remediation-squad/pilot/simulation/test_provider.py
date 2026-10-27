"""Local simulator checks, with no daemon, API account, or model calls."""

import copy
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
import uuid


ROOT = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("simulation_provider", ROOT / "provider.py")
provider = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(provider)
VALIDATOR_SPEC = importlib.util.spec_from_file_location("simulation_evidence", ROOT / "validate_evidence.py")
validator = importlib.util.module_from_spec(VALIDATOR_SPEC)
VALIDATOR_SPEC.loader.exec_module(validator)


def config():
    issue = str(uuid.uuid4())
    return {"simulation_id": "test-simulation", "workspace_id": str(uuid.uuid4()),
            "squad_id": str(uuid.uuid4()), "api_url": "http://127.0.0.1:1",
            "lab_path": "/work/lab", "patch_path": "/work/simulation/lab-diff.patch",
            "roles": {key: str(uuid.uuid4()) for key in provider.ROLE_KEYS},
            "cases": {issue: {"key": "other-supply-chain"}}}


class ConfigurationTests(unittest.TestCase):
    def load(self, data):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps(data))
            return provider.load_config(path)

    def test_valid_distinct_roles(self):
        self.assertEqual(len(self.load(config())["roles"]), 8)

    def test_aliasing_implementation_and_qa_is_rejected(self):
        data = config()
        data["roles"]["qa-engineer"] = data["roles"]["implementer"]
        with self.assertRaisesRegex(ValueError, "distinct"):
            self.load(data)

    def test_unknown_case_and_escaping_paths_are_rejected(self):
        for field, value in (("lab_path", "/etc"), ("patch_path", "/tmp/arbitrary.patch")):
            with self.subTest(field=field):
                data = config()
                data[field] = value
                with self.assertRaises(ValueError):
                    self.load(data)
        data = config()
        next(iter(data["cases"].values()))["key"] = "production-deploy"
        with self.assertRaisesRegex(ValueError, "Unknown"):
            self.load(data)

    def test_human_credential_is_rejected_before_network(self):
        data = config()
        env = {"MULTICA_AGENT_ID": data["roles"]["maintenance-lead"],
               "MULTICA_TASK_ID": str(uuid.uuid4()), "MULTICA_TOKEN": "synthetic-human-token"}
        with patch.dict(os.environ, env), self.assertRaisesRegex(ValueError, "task-scoped"):
            provider.SimulationTurn(data, "unused")


class ProvenanceTests(unittest.TestCase):
    def make_turn(self):
        turn = object.__new__(provider.SimulationTurn)
        turn.config = config()
        turn.case = "other-supply-chain"
        task_id, trigger_id, leader_task = (str(uuid.uuid4()) for _ in range(3))
        member = turn.config["roles"]["security-reviewer"]
        turn.tasks = [{"id": task_id, "agent_id": member, "status": "completed",
                       "trigger_comment_id": trigger_id, "attribution": {"source": "delegation"}}]
        turn.comments = [{"id": trigger_id, "author_type": "agent",
            "author_id": turn.config["roles"]["maintenance-lead"],
            "content": provider.record_marker("test-simulation", "dispatch", "security-reviewer", 0, leader_task)},
            {"id": str(uuid.uuid4()), "author_type": "agent", "author_id": member,
             "content": provider.record_marker("test-simulation", "result", "security-reviewer", 0, task_id)}]
        return turn

    def test_matching_actor_task_and_dispatch_are_required(self):
        turn = self.make_turn()
        self.assertEqual(turn.completed_stages(), {0: turn.tasks[0]["id"]})
        for mutation in ("author", "source", "task", "leader"):
            broken = self.make_turn()
            if mutation == "author": broken.comments[1]["author_type"] = "member"
            if mutation == "source": broken.tasks[0]["attribution"]["source"] = "direct_human"
            if mutation == "task": broken.tasks[0]["status"] = "failed"
            if mutation == "leader": broken.comments[0]["author_id"] = str(uuid.uuid4())
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                broken.completed_stages()

    def test_duplicate_results_are_not_silently_counted(self):
        turn = self.make_turn()
        duplicate = copy.deepcopy(turn.comments[-1])
        duplicate["id"] = str(uuid.uuid4())
        turn.comments.append(duplicate)
        with self.assertRaisesRegex(ValueError, "Duplicate"):
            turn.completed_stages()


class ProtocolTests(unittest.TestCase):
    def test_comment_file_respects_cli_workdir_boundary(self):
        turn = object.__new__(provider.SimulationTurn)
        turn.issue = str(uuid.uuid4())
        turn.parent = str(uuid.uuid4())
        turn.audit = lambda *args, **kwargs: None
        seen = []
        def cli(*args):
            content_file = Path(args[-1])
            self.assertEqual(content_file.parent.resolve(), Path.cwd().resolve())
            self.assertEqual(content_file.read_text(), "simulation comment")
            seen.append(content_file)
            return {"id": str(uuid.uuid4())}
        turn.cli = cli
        original = Path.cwd()
        with tempfile.TemporaryDirectory() as directory, patch.dict(os.environ, {"TMPDIR": "/tmp"}):
            try:
                os.chdir(directory)
                turn.comment("simulation comment")
                self.assertFalse(seen[0].exists())
            finally:
                os.chdir(original)

    def test_discovery_and_handshake_do_not_need_credentials_or_config(self):
        result = subprocess.run([sys.executable, str(ROOT / "provider.py"), "--version"],
                                text=True, capture_output=True, check=True)
        self.assertIn("SCRIPTED SIMULATION", result.stdout)
        requests = [{"id": 1, "method": "initialize", "params": {}},
                    {"method": "initialized", "params": {}},
                    {"id": 2, "method": "thread/start", "params": {}},
                    {"id": 3, "method": "model/list", "params": {}}]
        result = subprocess.run([sys.executable, str(ROOT / "provider.py"), "app-server"],
            input="".join(json.dumps(x) + "\n" for x in requests), text=True, capture_output=True, check=True)
        responses = [json.loads(line) for line in result.stdout.splitlines()]
        self.assertEqual([r["id"] for r in responses], [1, 2, 3])
        self.assertIn("scripted", responses[2]["result"]["data"][0]["model"])

    def test_unknown_protocol_method_fails_closed(self):
        result = subprocess.run([sys.executable, str(ROOT / "provider.py"), "app-server"],
            input=json.dumps({"id": 1, "method": "execute/arbitrary"}) + "\n",
            text=True, capture_output=True, check=True)
        self.assertEqual(json.loads(result.stdout)["error"]["code"], -32601)


class FixtureReplayTests(unittest.TestCase):
    def test_ordered_repair_retains_diagnostic_failure_and_independent_suite(self):
        fixtures = ROOT.parent / "fixtures"
        patch_path = ROOT.parent / "evidence/lab-diff-20261027.patch"
        with tempfile.TemporaryDirectory(prefix="security-simulation-test-") as directory:
            lab = Path(directory) / "lab"
            shutil.copytree(fixtures, lab)
            def run(*args):
                return subprocess.run(args, cwd=lab, text=True, capture_output=True, timeout=15)
            baseline = run(sys.executable, "-B", "-m", "unittest", "test_dependency", "test_authorization")
            self.assertEqual(baseline.returncode, 1)
            self.assertIn("FAILED (failures=2)", baseline.stderr)
            applied = run("git", "apply", "--include=application.py", "--include=dependency.lock.json", str(patch_path))
            self.assertEqual(applied.returncode, 0, applied.stderr)
            self.assertEqual(run(sys.executable, "-B", "-m", "unittest", "test_dependency").returncode, 0)
            diagnostic = run(sys.executable, "-B", "-m", "unittest", "test_authorization")
            self.assertEqual(diagnostic.returncode, 1)
            self.assertIn("FAILED (failures=1)", diagnostic.stderr)
            applied = run("git", "apply", "--include=workspace_reader.py", str(patch_path))
            self.assertEqual(applied.returncode, 0, applied.stderr)
            final = run(sys.executable, "-B", "-m", "unittest", "test_dependency", "test_authorization")
            self.assertEqual(final.returncode, 0, final.stderr)
            self.assertIn("Ran 6 tests", final.stderr)
            self.assertEqual(run(sys.executable, "-B", "validate_fixture.py").returncode, 0)


class EvidenceValidationTests(unittest.TestCase):
    def setUp(self):
        source = ROOT.parent / "evidence/scripted-simulation-20261027"
        temporary = tempfile.TemporaryDirectory(prefix="simulation-evidence-test-")
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        names = ["simulation-state.json", "simulation-actions.jsonl", "simulation-boundary.json"]
        names += [f"simulation-{case}.json" for case in validator.SEQUENCES]
        for name in names:
            shutil.copyfile(source / name, self.directory / name)
        self.snapshot_path = self.directory / "simulation-other-supply-chain.json"
        self.snapshot = json.loads(self.snapshot_path.read_text())
        self.result_id = next(comment["id"] for comment in self.snapshot["comments"]
            if provider.marker(comment, "security-squad-sim-20261027", "result"))
        self.member_id = next(comment["source_task_id"] for comment in self.snapshot["comments"]
                              if comment["id"] == self.result_id)
        self.return_id = next(task["id"] for task in self.snapshot["task_runs"]
            if task.get("trigger_comment_id") == self.result_id and task.get("is_leader_task"))

    def test_captured_product_evidence_passes_without_model_claim(self):
        result = validator.validate(self.directory)
        self.assertEqual((result["verified_cases"], result["successful_tasks"]), (4, 29))
        self.assertEqual(len(result["acknowledged_failed_attempts"]), 2)
        self.assertIs(result["model_behavior_verified"], False)
        self.assertIs(result["real_runtime_invoked"], False)

    def test_mutated_attribution_and_terminal_outcome_are_rejected(self):
        mutations = ("wrong_actor", "direct_member", "missing_return", "wrong_no_action",
                     "missing_source_task", "imprecise_return", "direct_return",
                     "wrong_return_evidence", "nonleader_final", "wrong_workspace")
        for mutation in mutations:
            data = copy.deepcopy(self.snapshot)
            comment = next(c for c in data["comments"] if c["id"] == self.result_id)
            member = next(t for t in data["task_runs"] if t["id"] == self.member_id)
            returned = next(t for t in data["task_runs"] if t["id"] == self.return_id)
            if mutation == "wrong_actor":
                comment["author_id"] = str(uuid.uuid4())
            elif mutation == "direct_member":
                member["attribution"]["source"] = "direct_human"
            elif mutation == "missing_return":
                data["task_runs"].remove(returned)
            elif mutation == "wrong_no_action":
                for event in data["timeline"]:
                    if event.get("action") == "squad_leader_evaluated" and event["details"].get("outcome") == "no_action":
                        event["details"]["outcome"] = "action"
            elif mutation == "missing_source_task":
                comment["source_task_id"] = None
            elif mutation == "imprecise_return":
                returned["attribution"]["precise"] = False
            elif mutation == "direct_return":
                returned["attribution"]["source"] = "direct_human"
            elif mutation == "wrong_return_evidence":
                returned["attribution"]["evidence"]["ref_id"] = str(uuid.uuid4())
            elif mutation == "nonleader_final":
                returned["is_leader_task"] = False
            elif mutation == "wrong_workspace":
                member["workspace_id"] = str(uuid.uuid4())
            self.snapshot_path.write_text(json.dumps(data))
            with self.subTest(mutation=mutation), self.assertRaises((AssertionError, KeyError)):
                validator.validate(self.directory)

    def test_nonzero_model_calls_are_rejected(self):
        path = self.directory / "simulation-state.json"
        state = json.loads(path.read_text())
        state["real_model_calls"] = 1
        path.write_text(json.dumps(state))
        with self.assertRaises(AssertionError):
            validator.validate(self.directory)

    def test_diagnostic_evidence_must_belong_to_the_actual_actor(self):
        path = self.directory / "simulation-actions.jsonl"
        events = [json.loads(line) for line in path.read_text().splitlines()]
        diagnostic = next(event for event in events if event.get("operation") == "fixture_command"
                          and event.get("role") == "diagnostician")
        diagnostic["agent_id"] = str(uuid.uuid4())
        path.write_text("".join(json.dumps(event) + "\n" for event in events))
        with self.assertRaises(AssertionError):
            validator.validate(self.directory)


if __name__ == "__main__":
    unittest.main(verbosity=2)
