"""Private adapter regressions; all executable and SQL boundaries are mocked."""
import contextlib
import hashlib
import io
import json
import os
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parent
WRAPPER = Path(os.environ.get("CHECK_ISOLATION_WRAPPER", str(ROOT / "check-bin/go")))
FLAG_REGISTRY = json.loads((ROOT / "go-flag-registry.json").read_text())
OWNER, OTHER = "example/first", "example/second"
CLONE = "ccheck_20261102010101000000_" + hashlib.sha256(OWNER.encode()).hexdigest()[:12]
OBSERVATIONS = []
TEST_VALUE_FLAGS = ("-bench", "-benchtime", "-blockprofile", "-blockprofilerate", "-count", "-cpu", "-cpuprofile",
                    "-fuzz", "-fuzzminimizetime", "-fuzztime", "-list", "-memprofile", "-memprofilerate",
                    "-mutexprofile", "-mutexprofilefraction", "-outputdir", "-parallel", "-run", "-shuffle",
                    "-skip", "-timeout", "-trace")


class Forwarded(Exception):
    pass


class PackageBindingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="binding-", dir=ROOT / ".tmp")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.module = self.root / "module"
        for directory in (self.module, self.module / "first", self.module / "second"):
            directory.mkdir()
        self.evidence = self.root / "evidence"
        self.evidence.mkdir()
        self.record = {"database": CLONE, "template": "multica_check_fixture_go",
                       "state": "running", "package": OWNER}
        self.env = {"MULTICA_TASK_REAL_GO": "/fixture/fake-go", "MULTICA_TASK_PSQL": "/fixture/fake-psql",
                    "MULTICA_TASK_PACKAGE_DB_MODE": "0", "MULTICA_TASK_PACKAGE_DB_EVIDENCE": str(self.evidence),
                    "DATABASE_URL": "postgres://127.0.0.1:5432/" + CLONE}

    def invoke(self, argv, *, cwd=None, record=None, remove_record_key=None, changes=None,
               remove_env=None, listed=None, list_status=0, test_status=0):
        allocation = {**self.record, **(record or {})}
        if remove_record_key:
            allocation.pop(remove_record_key, None)
        (self.evidence / (CLONE + ".json")).write_text(json.dumps(allocation))
        environment = {**self.env, **(changes or {})}
        if remove_env:
            environment.pop(remove_env, None)
        calls = []
        stdout, stderr = io.StringIO(), io.StringIO()

        def exec_recorder(executable, arguments):
            calls.append({"kind": "exec", "executable": executable, "argv": arguments,
                          "database_name": urlsplit(os.environ.get("DATABASE_URL", "")).path.lstrip("/"),
                          "mode": os.environ.get("MULTICA_TASK_PACKAGE_DB_MODE"),
                          "pool_max": os.environ.get("DATABASE_MAX_CONNS"),
                          "pool_min": os.environ.get("DATABASE_MIN_CONNS")})
            raise Forwarded()

        def run_recorder(arguments, **kwargs):
            if arguments[0] == "/fixture/fake-psql":
                database = urlsplit(arguments[1]).path.lstrip("/")
                statement = arguments[-1]
                calls.append({"kind": "sql", "database_name": database, "statement": statement})
                answer = database + "\n" if statement == "SELECT current_database()" else ""
                return subprocess.CompletedProcess(arguments, 0, answer, "")
            # The historical fallback is deliberately captured, never executed.
            if arguments[0] not in ("/fixture/fake-go", "/opt/homebrew/bin/go", "", " \t"):
                raise AssertionError("Unexpected executable; no real processes are permitted")
            position, effective = 1, Path.cwd()
            while position < len(arguments):
                if arguments[position] in ("-C", "--C"):
                    effective = (effective / arguments[position + 1]).resolve()
                    position += 2
                elif arguments[position].startswith(("-C=", "--C=")):
                    effective = (effective / arguments[position].split("=", 1)[1]).resolve()
                    position += 1
                else:
                    break
            self.assertEqual(arguments[position], "list")
            selectors = arguments[position + 1:]
            calls.append({"kind": "list", "argv": arguments, "cwd": str(Path.cwd()),
                          "effective_cwd": str(effective), "selectors": selectors})
            packages = []
            for selector in selectors:
                if "..." in selector:
                    packages.extend((OWNER, OTHER))
                elif selector in (OWNER, OTHER):
                    packages.append(selector)
                else:
                    packages.append({self.module / "first": OWNER, self.module / "second": OTHER}.get(
                        (effective / selector).resolve(), "example/root"))
            answer = packages if listed is None else listed
            return subprocess.CompletedProcess(arguments, list_status, "\n".join(answer) + "\n" if answer else "", "fake list failure" if list_status else "")

        def test_recorder(arguments, **kwargs):
            selected = kwargs["env"]
            calls.append({"kind": "test", "argv": arguments,
                          "database_name": urlsplit(selected["DATABASE_URL"]).path.lstrip("/"),
                          "mode": selected["MULTICA_TASK_PACKAGE_DB_MODE"],
                          "pool_max": selected.get("DATABASE_MAX_CONNS"), "pool_min": selected.get("DATABASE_MIN_CONNS")})
            class FakeTest:
                stdout = io.StringIO("\n".join(json.dumps(event) for event in (
                    {"Action": "run", "Test": "TestFakeFixture"},
                    {"Action": "pass", "Test": "TestFakeFixture"},
                    {"Action": "pass", "Package": arguments[-1]},
                )) + "\n")
                def wait(self):
                    return test_status
            return FakeTest()

        previous = Path.cwd()
        selected_cwd = cwd or self.module
        try:
            os.chdir(selected_cwd)
            with mock.patch.dict(os.environ, environment, clear=True), \
                 mock.patch.object(sys, "argv", [str(WRAPPER), *argv]), \
                 mock.patch.object(os, "execv", exec_recorder), \
                 mock.patch.object(os, "execve", side_effect=AssertionError("Unexpected execve")), \
                 mock.patch.object(os, "execvpe", side_effect=AssertionError("Unexpected execvpe")), \
                 mock.patch.object(subprocess, "run", run_recorder), \
                 mock.patch.object(subprocess, "Popen", test_recorder), \
                 contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                terminal, code = "returned", 0
                try:
                    runpy.run_path(str(WRAPPER), run_name="__main__")
                except Forwarded:
                    terminal = "forwarded"
                except SystemExit as error:
                    terminal, code = "refused", error.code
                except Exception as error:
                    terminal, code = type(error).__name__, 1
                    stderr.write(str(error))
        finally:
            os.chdir(previous)
        result = {"test": self.id(), "argv": argv, "cwd": str(selected_cwd), "allocation": allocation,
                  "terminal": terminal, "exit_code": code, "calls": calls,
                  "stdout": stdout.getvalue(), "stderr": stderr.getvalue(),
                  "actual_subprocesses": 0, "actual_sql_connections": 0}
        OBSERVATIONS.append(result)
        return result

    def assert_forwarded(self, result, argv, selectors):
        self.assertEqual(result["terminal"], "forwarded", result)
        self.assertEqual([call["kind"] for call in result["calls"]], ["list", "exec"], result)
        self.assertEqual(result["calls"][0]["selectors"], selectors)
        self.assertEqual(result["calls"][1]["argv"], ["/fixture/fake-go", *argv])
        self.assertEqual(result["calls"][1]["database_name"], CLONE)

    def assert_refused(self, result, *, no_calls=False):
        self.assertEqual(result["exit_code"], 2, result)
        self.assertFalse(any(call["kind"] in ("exec", "test", "sql") for call in result["calls"]), result)
        if no_calls:
            self.assertEqual(result["calls"], [], result)

    def test_canonical_selector_resolves_before_forwarding(self):
        argv = ["test", OWNER]
        self.assert_forwarded(self.invoke(argv), argv, [OWNER])

    def test_relative_selector_compares_canonical_import_path(self):
        argv = ["test", "./first"]
        self.assert_forwarded(self.invoke(argv), argv, ["./first"])

    def test_default_selector_uses_the_current_package(self):
        argv = ["test", "-race", "-count", "1"]
        self.assert_forwarded(self.invoke(argv, cwd=self.module / "first"), argv, ["."])

    def test_explicit_dot_is_permitted_in_owner_directory(self):
        argv = ["test", "."]
        self.assert_forwarded(self.invoke(argv, cwd=self.module / "first"), argv, ["."])

    def test_default_selector_in_another_directory_refuses(self):
        self.assert_refused(self.invoke(["test"], cwd=self.module / "second"))

    def test_leading_C_and_C_equals_keep_identical_list_and_test_semantics(self):
        for prefix in (["-C", "first"], ["-C=first"]):
            with self.subTest(prefix=prefix):
                argv = [*prefix, "test"]
                result = self.invoke(argv)
                self.assert_forwarded(result, argv, ["."])
                self.assertEqual(result["calls"][0]["argv"], ["/fixture/fake-go", *prefix, "list", "."])
                self.assertEqual(result["calls"][0]["effective_cwd"], str(self.module / "first"))

    def test_changed_C_and_C_equals_default_package_refuse(self):
        for prefix in (["-C", "second"], ["-C=second"]):
            with self.subTest(prefix=prefix):
                self.assert_refused(self.invoke([*prefix, "test"]))

    def test_relative_selector_under_C_is_resolved_by_go_list(self):
        argv = ["-C", "second", "test", "../first"]
        self.assert_forwarded(self.invoke(argv), argv, ["../first"])

    def test_other_canonical_and_relative_packages_refuse(self):
        for selector in (OTHER, "./second"):
            with self.subTest(selector=selector):
                self.assert_refused(self.invoke(["test", selector]))

    def test_multiple_selectors_refuse_even_if_they_resolve_to_one_owner(self):
        for selectors in ([OWNER, OTHER], [OWNER, OWNER], [OWNER, "./first"]):
            with self.subTest(selectors=selectors):
                self.assert_refused(self.invoke(["test", *selectors], listed=[OWNER]))

    def test_wildcard_selectors_never_use_the_parent_clone(self):
        for selector in ("./...", "example/...", "./first/..."):
            with self.subTest(selector=selector):
                self.assert_refused(self.invoke(["test", selector]))

    def test_wildcard_refuses_even_when_list_would_return_only_owner(self):
        self.assert_refused(self.invoke(["test", "./..."], listed=[OWNER]))

    def test_missing_empty_and_malformed_package_identities_refuse_before_calls(self):
        self.assert_refused(self.invoke(["test", OWNER], remove_record_key="package"), no_calls=True)
        for package in (None, "", " \t", 0, [], {}):
            with self.subTest(package=package):
                self.assert_refused(self.invoke(["test", OWNER], record={"package": package}), no_calls=True)

    def test_noncanonical_allocation_package_refuses_before_calls(self):
        for package in (".", "./first", "../first", "/example/first", "example/...", "example//first",
                        "example/../first", "example/first\n", " example/first", "-example/first", "example/first:tmp"):
            with self.subTest(package=package):
                self.assert_refused(self.invoke(["test", OWNER], record={"package": package}), no_calls=True)

    def test_go_list_failure_refuses_before_test_execution(self):
        self.assert_refused(self.invoke(["test", OWNER], list_status=1))

    def test_empty_multiple_and_different_list_results_refuse(self):
        for listed in ([], [OWNER, OTHER], [OTHER]):
            with self.subTest(listed=listed):
                self.assert_refused(self.invoke(["test", OWNER], listed=listed))

    def test_value_flags_do_not_become_package_selectors(self):
        flags = ["-timeout", "40m", "-p", "1", "-parallel", "2", "-run", OTHER, "-skip", OTHER,
                 "-count", "1", "-tags", "fixture", "-cpu", "2", "-coverprofile", "coverage.out",
                 "-coverpkg", OTHER, "-vet", "off", "-o", "output",
                 "-ldflags", "-s -w", "-gcflags", "all=-l", "-asmflags", "all=-v", "-toolexec", "fake-tool",
                 "-mod", "readonly", "-pkgdir", "packages", "-exec", "fake-exec", "-list", "TestFixture"]
        argv = ["test", *flags, "./first"]
        self.assert_forwarded(self.invoke(argv), argv, ["./first"])

    def test_equals_value_flags_and_test_arguments_remain_exact(self):
        for boundary in ("-args", "--"):
            with self.subTest(boundary=boundary):
                argv = ["test", "-run=" + OTHER, OWNER, boundary, OTHER, "-c", "-C", "second"]
                self.assert_forwarded(self.invoke(argv), argv, [OWNER])

    def test_default_selector_after_args_boundary_is_not_compile_only(self):
        for boundary in ("-args", "--"):
            with self.subTest(boundary=boundary):
                argv = ["test", boundary, "-c"]
                self.assert_forwarded(self.invoke(argv, cwd=self.module / "first"), argv, ["."])

    def test_flag_values_and_false_compile_override_require_package_binding(self):
        for flags in (["-run", "-c"], ["-c", "-c=false"], ["-list", "-c"]):
            with self.subTest(flags=flags):
                argv = ["test", *flags, OWNER]
                self.assert_forwarded(self.invoke(argv), argv, [OWNER])

    def test_real_compile_flags_forward_without_isolation_context(self):
        for flag in ("-c", "-c=true", "-c=1", "-c=T"):
            with self.subTest(flag=flag):
                argv = ["test", "-run", "TestFixture", flag, OWNER]
                result = self.invoke(argv, changes={"MULTICA_TASK_PACKAGE_DB_MODE": "", "DATABASE_URL": ""})
                self.assertEqual(result["terminal"], "forwarded", result)
                self.assertEqual([call["kind"] for call in result["calls"]], ["exec"])
                self.assertEqual(result["calls"][0]["argv"], ["/fixture/fake-go", *argv])

    def test_stripped_non_test_forwarding_keeps_fallback_and_pool_configuration(self):
        for argv in (["version"], ["build", "./..."], ["run", "./fixture.go"], ["-C=first", "version"]):
            with self.subTest(argv=argv):
                result = self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO", changes={
                    "MULTICA_TASK_PACKAGE_DB_MODE": "", "DATABASE_URL": "", "DATABASE_MAX_CONNS": "10", "DATABASE_MIN_CONNS": "1"})
                self.assertEqual(result["terminal"], "forwarded", result)
                self.assertEqual(result["calls"][0]["argv"], ["/opt/homebrew/bin/go", *argv])
                self.assertEqual(result["calls"][0]["pool_max"], "10")
                self.assertEqual(result["calls"][0]["pool_min"], "1")
                self.assertEqual(len(result["calls"]), 1)

    def test_unmarked_test_commands_refuse_before_all_calls(self):
        for argv in (["test", OWNER], ["-C=first", "test"], ["test", "-args", "-c"],
                     ["test", "--", "-c"], ["test", "-run", "-c"], ["test", "-c", "-c=false"]):
            with self.subTest(argv=argv):
                self.assert_refused(self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                    changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""}), no_calls=True)

    def test_mode1_requires_nonblank_explicit_go_psql_and_evidence_before_any_call(self):
        for key in ("MULTICA_TASK_REAL_GO", "MULTICA_TASK_PSQL", "MULTICA_TASK_PACKAGE_DB_EVIDENCE"):
            for value in (None, "", " \t"):
                with self.subTest(key=key, value=value):
                    changes = {"MULTICA_TASK_PACKAGE_DB_MODE": "1", "DATABASE_URL": "postgres://127.0.0.1/multica_check_fixture_go"}
                    if value is not None:
                        changes[key] = value
                    self.assert_refused(self.invoke(["test", OWNER], changes=changes,
                        remove_env=key if value is None else None), no_calls=True)

    def test_mode0_requires_nonblank_explicit_go_and_evidence_before_any_call(self):
        for key in ("MULTICA_TASK_REAL_GO", "MULTICA_TASK_PACKAGE_DB_EVIDENCE"):
            for value in (None, "", " \t"):
                with self.subTest(key=key, value=value):
                    self.assert_refused(self.invoke(["test", OWNER], changes={} if value is None else {key: value},
                        remove_env=key if value is None else None), no_calls=True)

    def test_inherited_pool_overrides_are_removed_for_permitted_recursive_tests(self):
        argv = ["test", OWNER]
        result = self.invoke(argv, changes={"DATABASE_MAX_CONNS": "10", "DATABASE_MIN_CONNS": "1"})
        self.assert_forwarded(result, argv, [OWNER])
        self.assertIsNone(result["calls"][1]["pool_max"])
        self.assertIsNone(result["calls"][1]["pool_min"])

    def test_mode1_keeps_fresh_per_package_allocation_and_original_check_flags(self):
        argv = ["-C", str(self.module), "test", "-race", "-timeout=40m", "-p", "1", "-parallel", "2", "./..."]
        result = self.invoke(argv, changes={"MULTICA_TASK_PACKAGE_DB_MODE": "1",
            "DATABASE_URL": "postgres://127.0.0.1/multica_check_fixture_go", "DATABASE_MAX_CONNS": "10", "DATABASE_MIN_CONNS": "1"})
        self.assertEqual(result["exit_code"], 0, result)
        tests = [call for call in result["calls"] if call["kind"] == "test"]
        self.assertEqual(len(tests), 2)
        self.assertNotEqual(tests[0]["database_name"], tests[1]["database_name"])
        for package, call in zip((OWNER, OTHER), tests):
            self.assertEqual(call["argv"], ["/fixture/fake-go", "-C", str(self.module), "test",
                "-race", "-timeout=40m", "-p", "1", "-parallel", "2", "-json", package])
            self.assertEqual(call["mode"], "0")
            self.assertIsNone(call["pool_max"])
            self.assertIsNone(call["pool_min"])
            allocation = json.loads((self.evidence / (call["database_name"] + ".json")).read_text())
            self.assertEqual(allocation["package"], package)
            self.assertEqual(allocation["counts"], {"run": 1, "pass": 1, "fail": 0, "skip": 0})

    def test_go_value_flags_and_test_aliases_never_exempt_unmarked_tests(self):
        for flag in (*TEST_VALUE_FLAGS, *("-test." + flag[1:] for flag in TEST_VALUE_FLAGS)):
            for value in ("-c", "-c=true"):
                with self.subTest(flag=flag, value=value):
                    self.assert_refused(self.invoke(["test", flag, value, OWNER],
                        remove_env="MULTICA_TASK_REAL_GO", changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""}), no_calls=True)

    def test_go_value_flags_and_test_aliases_keep_permitted_package_binding(self):
        for flag in (*TEST_VALUE_FLAGS, *("-test." + flag[1:] for flag in TEST_VALUE_FLAGS)):
            with self.subTest(flag=flag):
                argv = ["test", flag, "-c", OWNER]
                self.assert_forwarded(self.invoke(argv), argv, [OWNER])

    def test_compile_scanner_preserves_last_real_flag_and_args_boundaries(self):
        for argv in (["test", "-c=true", "-c=false", OWNER],
                     ["test", "-c=false", "-bench", "-c", OWNER],
                     ["test", "-c=false", "-test.run", "-c=true", OWNER],
                     ["test", OWNER, "-args", "-bench", "-c"],
                     ["test", OWNER, "--", "-test.run", "-c=true"]):
            with self.subTest(argv=argv):
                self.assert_refused(self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                    changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""}), no_calls=True)
        for argv in (["test", "-c=false", "-c=true", OWNER],
                     ["test", "-c", "-test.run", "-c=false", OWNER],
                     ["test", "-bench", "-c", "-c=true", OWNER],
                     ["test", "-c", OWNER, "-args", "-c=false"],
                     ["test", "-c=true", OWNER, "--", "-c=false"]):
            with self.subTest(argv=argv):
                result = self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                    changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""})
                self.assertEqual(result["terminal"], "forwarded", result)
                self.assertEqual([call["kind"] for call in result["calls"]], ["exec"])
                self.assertEqual(result["calls"][0]["argv"], ["/opt/homebrew/bin/go", *argv])

    def test_complete_go_value_registry_never_exempts_unmarked_test_execution(self):
        for flag in FLAG_REGISTRY["value_flags"]:
            for spelling in (flag, "-" + flag):
                for value in ("-c", "-c=true"):
                    with self.subTest(flag=spelling, value=value):
                        self.assert_refused(self.invoke(["test", spelling, value, OWNER],
                            remove_env="MULTICA_TASK_REAL_GO", changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""}), no_calls=True)

    def test_known_boolean_registry_keeps_standalone_compile_flag(self):
        for flag in FLAG_REGISTRY["boolean_flags"]:
            for spelling in (flag, "-" + flag):
                with self.subTest(flag=spelling):
                    argv = ["test", spelling, "-c", OWNER]
                    result = self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                        changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""})
                    self.assertEqual(result["terminal"], "forwarded", result)
                    self.assertEqual([call["kind"] for call in result["calls"]], ["exec"])
                    self.assertEqual(result["calls"][0]["argv"], ["/opt/homebrew/bin/go", *argv])

    def test_unknown_options_never_grant_compile_or_touch_go_or_sql(self):
        for mode in ("", "0", "1"):
            for option in ("-future-output", "--future-output", "-test.installsuffix", "-test.covermode"):
                for argv in (["test", option, "-c", OWNER], ["test", "-c", option, OWNER],
                             ["test", option, "-c=true", OWNER], ["test", option + "=-c", "-c", OWNER]):
                    with self.subTest(mode=mode, argv=argv):
                        result = self.invoke(argv, changes={"MULTICA_TASK_PACKAGE_DB_MODE": mode,
                            "DATABASE_URL": "postgres://127.0.0.1/multica_check_fixture_go" if mode == "1" else self.env["DATABASE_URL"]})
                        self.assert_refused(result, no_calls=True)

    def test_inline_known_values_and_last_compile_setting_remain_exact(self):
        positive = (["test", "-installsuffix=-c", "-c", OWNER],
                    ["test", "--gccgoflags=-c", "--c=true", OWNER],
                    ["test", "-c=false", "--c=true", "-race=false", OWNER],
                    ["test", "--c", "-tags=fixture", "-o=output", OWNER],
                    ["test", "-test.run=-c", "-c=true", OWNER])
        for argv in positive:
            with self.subTest(argv=argv):
                result = self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                    changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""})
                self.assertEqual(result["terminal"], "forwarded", result)
                self.assertEqual([call["kind"] for call in result["calls"]], ["exec"])
                self.assertEqual(result["calls"][0]["argv"], ["/opt/homebrew/bin/go", *argv])
        for argv in (["test", "--c=true", "-c=false", OWNER],
                     ["test", "-c=true", "--c=false", OWNER],
                     ["test", "-c", "--c=false", OWNER],
                     ["test", "-installsuffix=-c", OWNER],
                     ["test", "--test.coverprofile=-c", OWNER]):
            with self.subTest(argv=argv):
                self.assert_refused(self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                    changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""}), no_calls=True)

    def test_leading_C_aliases_and_command_local_C_use_the_same_package_identity(self):
        for argv in (["--C", "first", "test"], ["--C=first", "test"],
                     ["test", "-C", "first"], ["test", "-C=first"],
                     ["test", "--C", "first"], ["test", "--C=first"]):
            with self.subTest(argv=argv):
                result = self.invoke(argv)
                self.assert_forwarded(result, argv, ["."])
                self.assertEqual(result["calls"][0]["effective_cwd"], str(self.module / "first"))
        for argv in (["--C", "second", "test"], ["--C=second", "test"],
                     ["test", "-C", "second"], ["test", "--C=second"]):
            with self.subTest(argv=argv):
                self.assert_refused(self.invoke(argv))
        for argv in (["--C=first", "test", "-c=false"], ["test", "--C", "first", "-c=false"]):
            with self.subTest(argv=argv):
                self.assert_refused(self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                    changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""}), no_calls=True)

    def test_mode0_source_resolution_options_refuse_before_list_sql_or_execution(self):
        for flag in ("-modfile", "-overlay", "--modfile", "--overlay"):
            for argv in (["test", flag, "alternate.mod", OWNER], ["test", flag + "=alternate.mod", OWNER],
                         ["-C", "first", "test", "-c=false", flag, "alternate.mod"]):
                with self.subTest(argv=argv):
                    self.assert_refused(self.invoke(argv), no_calls=True)

    def test_source_resolution_names_as_values_or_binary_arguments_are_preserved(self):
        for flag in ("-run", "--run", "-test.run", "--test.run", "-coverprofile", "-test.coverprofile"):
            for value in ("-modfile", "-overlay"):
                with self.subTest(flag=flag, value=value):
                    argv = ["test", flag, value, OWNER]
                    self.assert_forwarded(self.invoke(argv), argv, [OWNER])
        for boundary in ("-args", "--args", "--"):
            with self.subTest(boundary=boundary):
                argv = ["test", OWNER, boundary, "-modfile", "alternate.mod", "-overlay", "overlay.json", "-c"]
                self.assert_forwarded(self.invoke(argv), argv, [OWNER])
        for argv in (["test", "-modfile", "alternate.mod", "-c", OWNER],
                     ["test", "--overlay=overlay.json", "--c=true", OWNER]):
            with self.subTest(argv=argv):
                result = self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                    changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""})
                self.assertEqual(result["terminal"], "forwarded", result)
                self.assertEqual([call["kind"] for call in result["calls"]], ["exec"])

    def test_package_positional_tail_and_args_boundaries_do_not_reclassify_binary_c(self):
        for argv in (["test", OWNER, "-v", "literal", "-c"],
                     ["test", OWNER, "-run", "TestFixture", "literal", "-c=true"],
                     ["test", OWNER, "--args", "-installsuffix", "-c"],
                     ["test", OWNER, "--", "-gccgoflags", "-c=true"]):
            with self.subTest(argv=argv):
                self.assert_refused(self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
                    changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""}), no_calls=True)
        argv = ["test", "-c", OWNER, "-v", "literal", "-c=false"]
        result = self.invoke(argv, remove_env="MULTICA_TASK_REAL_GO",
            changes={"MULTICA_TASK_PACKAGE_DB_MODE": ""})
        self.assertEqual(result["terminal"], "forwarded", result)
        self.assertEqual([call["kind"] for call in result["calls"]], ["exec"])


if __name__ == "__main__":
    ROOT.joinpath(".tmp").mkdir(exist_ok=True)
    result = unittest.main(verbosity=2, exit=False).result
    report = {"wrapper": str(WRAPPER), "wrapper_sha256": hashlib.sha256(WRAPPER.read_bytes()).hexdigest(),
              "tests_run": result.testsRun, "failure_count": len(result.failures), "error_count": len(result.errors),
              "failure_test_ids": [test.id() for test, _ in result.failures], "observations": OBSERVATIONS,
              "method": "Exact wrapper with execv/execve/execvpe/run/Popen mocked; no real executable or SQL calls"}
    output = os.environ.get("CHECK_BINDING_EVIDENCE")
    if output:
        Path(output).write_text(json.dumps(report, indent=2) + "\n")
    raise SystemExit(0 if result.wasSuccessful() else 1)
