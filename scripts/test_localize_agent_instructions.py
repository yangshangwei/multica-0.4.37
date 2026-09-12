import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location(
    "localize_agent_instructions", Path(__file__).with_name("localize_agent_instructions.py")
)
subject = importlib.util.module_from_spec(spec)
spec.loader.exec_module(subject)

WORKSPACE = "00000000-0000-4000-8000-000000000001"
AGENT = "00000000-0000-4000-8000-000000000002"
OTHER = "00000000-0000-4000-8000-000000000003"
BEFORE_TIME = "2026-09-12T08:30:00.123456Z"
AFTER_TIME = "2026-09-12T08:30:01.654321Z"
SOURCE = "# Code Reviewer\n\nRead the diff. Keep `done` unchanged."
TARGET = "# 代码审查员\n\n阅读 diff，保留 `done` 状态值。"


def row(**changes):
    result = {
        "id": AGENT,
        "workspace_id": WORKSPACE,
        "name": "Reviewer",
        "template_key": "code-reviewer",
        "template_version": 1,
        "instructions": SOURCE,
        "updated_at": BEFORE_TIME,
    }
    result.update(changes)
    return result


class FakeAPI:
    server_url = "http://localhost:18572"

    def __init__(self, agents=None, squads=None):
        self.agents = agents if agents is not None else [row()]
        self.squads = squads or []
        self.writes = []
        self.supported = True
        self.conflict_during_write = False

    def request(self, method, path, body=None):
        headers = {"X-Multica-Instructions-Precondition": "1"} if self.supported else {}
        if path == f"/api/workspaces/{WORKSPACE}":
            return {"id": WORKSPACE, "name": "测试工作区"}, headers
        if path == "/api/agents":
            return copy.deepcopy(self.agents), headers
        if path == "/api/squads":
            return copy.deepcopy(self.squads), headers
        records = self.squads if path.startswith("/api/squads/") else self.agents
        current = next(item for item in records if item["id"] == path.rsplit("/", 1)[1])
        if method == "GET":
            return copy.deepcopy(current), headers
        self.writes.append(copy.deepcopy(body))
        if self.conflict_during_write:
            current.update(instructions="另一个成员的新规则", updated_at=AFTER_TIME)
        if (current["instructions"] != body["expected_instructions"] or
                current["updated_at"] != body["expected_updated_at"]):
            raise subject.APIError(409)
        current.update(instructions=body["instructions"], updated_at=AFTER_TIME)
        return copy.deepcopy(current), headers


class LocalizationTests(unittest.TestCase):
    def catalog(self):
        return {("agent", "code-reviewer"): {
            "source": SOURCE, "target": TARGET, "source_version": 1, "target_version": 1,
        }}

    def plan(self, api):
        return subject.build_plan(api, WORKSPACE, self.catalog())

    def test_only_exact_known_default_is_selected(self):
        api = FakeAPI(agents=[
            row(),
            row(id=OTHER, instructions=SOURCE + "\nAlso follow our team convention."),
        ])
        plan = self.plan(api)
        self.assertTrue(plan["entries"][0]["selected"])
        self.assertEqual(plan["entries"][0]["after"], TARGET)
        self.assertFalse(plan["entries"][1]["selected"])
        self.assertEqual(plan["entries"][1]["before"], api.agents[1]["instructions"])
        self.assertEqual(api.writes, [])

    def test_name_or_version_alone_cannot_identify_a_default(self):
        for changes in ({"template_key": ""}, {"template_version": 2},
                        {"instructions": SOURCE + "\n"}, {"system_key": "mika"}):
            with self.subTest(changes=changes):
                plan = self.plan(FakeAPI([row(**changes)]))
                self.assertFalse(plan["entries"][0]["selected"])

    def test_translated_or_historically_different_rules_are_not_replaced(self):
        api = FakeAPI([row(instructions=TARGET)])
        self.assertFalse(self.plan(api)["entries"][0]["selected"])
        catalog = self.catalog()
        catalog[("agent", "code-reviewer")]["target_version"] = 2
        self.assertFalse(subject.build_plan(FakeAPI(), WORKSPACE, catalog)["entries"][0]["selected"])

    def test_cross_workspace_response_is_rejected(self):
        with self.assertRaises(ValueError):
            self.plan(FakeAPI([row(workspace_id=OTHER)]))

    def test_dry_run_does_not_write_or_create_a_journal(self):
        api = FakeAPI()
        summary = subject.execute_plan(api, self.plan(api))
        self.assertEqual(summary["ready"], 1)
        self.assertEqual(api.writes, [])

    def test_update_preserves_microseconds_and_only_sends_instruction_fields(self):
        api = FakeAPI()
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / "apply.jsonl"
            summary = subject.execute_plan(api, self.plan(api), journal)
            self.assertEqual(summary["applied"], 1)
            self.assertEqual(api.writes, [{
                "instructions": TARGET,
                "expected_instructions": SOURCE,
                "expected_updated_at": BEFORE_TIME,
            }])
            self.assertEqual(journal.stat().st_mode & 0o777, 0o600)
            reverse = subject.rollback_plan(journal)
            self.assertEqual(reverse["entries"][0]["before"], TARGET)
            self.assertEqual(reverse["entries"][0]["after"], SOURCE)
            self.assertEqual(reverse["entries"][0]["before_updated_at"], AFTER_TIME)

    def test_older_server_without_atomic_preconditions_is_never_written(self):
        api = FakeAPI()
        plan = self.plan(api)
        api.supported = False
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ValueError):
                subject.execute_plan(api, plan, Path(directory) / "apply.jsonl")
        self.assertEqual(api.writes, [])

    def test_concurrent_edit_is_preserved_by_server_side_condition(self):
        api = FakeAPI()
        plan = self.plan(api)
        api.conflict_during_write = True
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / "apply.jsonl"
            summary = subject.execute_plan(api, plan, journal)
            self.assertEqual(summary["conflict"], 1)
            self.assertEqual(subject.rollback_plan(journal)["entries"], [])
        self.assertEqual(api.agents[0]["instructions"], "另一个成员的新规则")

    def test_repeat_apply_is_a_noop_and_rollback_respects_later_edits(self):
        api = FakeAPI()
        plan = self.plan(api)
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / "apply.jsonl"
            subject.execute_plan(api, plan, journal)
            count = len(api.writes)
            self.assertEqual(subject.execute_plan(api, plan)["already_matches"], 1)
            self.assertEqual(len(api.writes), count)
            reverse = subject.rollback_plan(journal)
            api.agents[0].update(instructions="迁移后的团队定制", updated_at="2026-09-12T09:00:00Z")
            result = subject.execute_plan(api, reverse, Path(directory) / "rollback.jsonl")
            self.assertEqual(result["conflict"], 1)
            self.assertEqual(len(api.writes), count)
            self.assertEqual(api.agents[0]["instructions"], "迁移后的团队定制")

    def test_mismatched_server_duplicate_ids_and_existing_journal_fail_before_write(self):
        for alteration in ("server", "duplicates", "journal"):
            with self.subTest(alteration=alteration), tempfile.TemporaryDirectory() as directory:
                api = FakeAPI()
                plan = self.plan(api)
                journal = Path(directory) / "apply.jsonl"
                if alteration == "server":
                    plan["server_url"] = "https://different.example"
                elif alteration == "duplicates":
                    plan["entries"].append(copy.deepcopy(plan["entries"][0]))
                else:
                    journal.write_text("existing backup")
                with self.assertRaises((ValueError, FileExistsError)):
                    subject.execute_plan(api, plan, journal)
                self.assertEqual(api.writes, [])

    def test_squad_uses_the_same_atomic_round_trip(self):
        squad = row(template_key="review-gate")
        api = FakeAPI(agents=[], squads=[squad])
        catalog = {("squad", "review-gate"): {
            "source": SOURCE, "target": TARGET, "source_version": 1, "target_version": 1,
        }}
        plan = subject.build_plan(api, WORKSPACE, catalog)
        with tempfile.TemporaryDirectory() as directory:
            result = subject.execute_plan(api, plan, Path(directory) / "apply.jsonl")
        self.assertEqual(result["applied"], 1)
        self.assertEqual(api.squads[0]["instructions"], TARGET)

    def test_successful_rollback_restores_only_our_recorded_write(self):
        api = FakeAPI()
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / "apply.jsonl"
            subject.execute_plan(api, self.plan(api), journal)
            reverse = subject.rollback_plan(journal)
            summary = subject.execute_plan(api, reverse, Path(directory) / "rollback.jsonl")
        self.assertEqual(summary["applied"], 1)
        self.assertEqual(api.agents[0]["instructions"], SOURCE)
        self.assertEqual(api.writes[-1]["expected_updated_at"], AFTER_TIME)

    def test_uncertain_write_stops_later_updates_and_blocks_automatic_rollback(self):
        api = FakeAPI([row(), row(id=OTHER)])
        plan = self.plan(api)
        original = api.request

        def fail_after_commit(method, path, body=None):
            result = original(method, path, body)
            if method == "PUT":
                raise OSError("connection lost after server committed")
            return result

        api.request = fail_after_commit
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / "apply.jsonl"
            with self.assertRaises(OSError):
                subject.execute_plan(api, plan, journal)
            records = [json.loads(line) for line in journal.read_text().splitlines()]
            self.assertEqual(records[1]["entry"]["before"], SOURCE)
            self.assertEqual(records[-1]["type"], "uncertain")
            with self.assertRaisesRegex(ValueError, "uncertain"):
                subject.rollback_plan(journal)
        self.assertEqual(len(api.writes), 1)
        self.assertEqual(api.agents[1]["instructions"], SOURCE)

    def test_malformed_resource_and_backup_are_rejected_before_any_write(self):
        for changes in ({"instructions": None}, {"updated_at": None}, {"id": "not-a-uuid"}):
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                self.plan(FakeAPI([row(**changes)]))
        api = FakeAPI()
        plan = self.plan(api)
        plan["entries"][0]["before"] = "accidentally edited backup"
        with self.assertRaises(ValueError):
            subject.execute_plan(api, plan)
        self.assertEqual(api.writes, [])

    def test_no_write_happens_if_backup_cannot_be_flushed(self):
        api = FakeAPI()
        with tempfile.TemporaryDirectory() as directory, patch.object(subject.os, "fsync", side_effect=OSError):
            with self.assertRaises(OSError):
                subject.execute_plan(api, self.plan(api), Path(directory) / "apply.jsonl")
        self.assertEqual(api.writes, [])

    def test_corrupted_rollback_backup_is_rejected(self):
        api = FakeAPI()
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / "apply.jsonl"
            subject.execute_plan(api, self.plan(api), journal)
            records = [json.loads(line) for line in journal.read_text().splitlines()]
            records[-1]["entry"]["before"] = "corrupted original instructions"
            journal.write_text("\n".join(json.dumps(record) for record in records))
            with self.assertRaises(ValueError):
                subject.rollback_plan(journal)
        self.assertEqual(len(api.writes), 1)
        self.assertEqual(api.agents[0]["instructions"], TARGET)

    def test_deleted_resource_is_identified_in_the_result_journal(self):
        api = FakeAPI()
        plan = self.plan(api)
        api.request = lambda *args, **kwargs: (_ for _ in ()).throw(subject.APIError(404))
        with tempfile.TemporaryDirectory() as directory:
            journal = Path(directory) / "apply.jsonl"
            summary = subject.execute_plan(api, plan, journal)
            self.assertEqual(summary["conflict"], 1)
            records = [json.loads(line) for line in journal.read_text().splitlines()]
            self.assertEqual(records[-1].get("id"), AGENT)
            self.assertEqual(records[-1].get("type"), "conflict")
        self.assertEqual(api.writes, [])


if __name__ == "__main__":
    unittest.main()
