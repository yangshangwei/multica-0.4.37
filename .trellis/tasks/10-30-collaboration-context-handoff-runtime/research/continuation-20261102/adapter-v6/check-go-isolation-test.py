import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

WRAPPER = Path(os.environ.get("MULTICA_ADAPTER_UNDER_TEST", str(Path(__file__).parent / "check-bin/go")))

class IsolationLauncherTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="multica-check-wrapper-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.record = self.root / "calls.jsonl"
        self.go = self.root / "fake-go"
        self.go.write_text('''#!/usr/bin/env python3
import sys,os,json
if sys.argv[1]=='list': print('example/first\\nexample/second')
elif sys.argv[1]=='version': print('fake-go-version')
elif '-c' in sys.argv or '-c=true' in sys.argv: print('compile-only')
else:
 with open(os.environ['CHECK_ISOLATION_FAKE_RECORD'],'a') as f:f.write(json.dumps({'go':sys.argv[1:],'database':os.environ['DATABASE_URL'],'mode':os.environ['MULTICA_TASK_PACKAGE_DB_MODE']})+'\\n')
 if os.environ.get('CHECK_FAKE_SILENT')=='1': print(json.dumps({'Action':'pass','Package':sys.argv[-1]}))
 elif os.environ.get('CHECK_FAKE_NO_TESTS')=='1':
  print(json.dumps({'Action':'output','Output':'[no test files]'}));print(json.dumps({'Action':'skip','Package':sys.argv[-1]}))
 else:
  print(json.dumps({'Action':'run','Test':'TestFixture'}));print(json.dumps({'Action':'pass','Test':'TestFixture'}));print(json.dumps({'Action':'pass','Package':sys.argv[-1]}))
''')
        self.psql = self.root / "fake-psql"
        self.psql.write_text('''#!/usr/bin/env python3
import sys,os,json,urllib.parse
with open(os.environ['CHECK_ISOLATION_FAKE_RECORD'],'a') as f:f.write(json.dumps({'sql':sys.argv[-1]})+'\\n')
if sys.argv[-1]=='SELECT current_database()':print(urllib.parse.urlsplit(sys.argv[1]).path.lstrip('/'))
''')
        for p in (self.go, self.psql): p.chmod(0o700)
        self.env = {**os.environ, "MULTICA_TASK_REAL_GO":str(self.go), "MULTICA_TASK_PSQL":str(self.psql),
                    "MULTICA_TASK_PACKAGE_DB_MODE":"1", "MULTICA_TASK_PACKAGE_DB_EVIDENCE":str(self.root/"evidence"),
                    "DATABASE_URL":"postgres://fixture:fixture@127.0.0.1:5432/multica_check_fixture_go", "CHECK_ISOLATION_FAKE_RECORD":str(self.record)}

    def run_wrapper(self, *args, **env):
        return subprocess.run([str(WRAPPER), *args], env={**self.env, **env}, text=True, capture_output=True)

    def calls(self):
        return [json.loads(line) for line in self.record.read_text().splitlines()] if self.record.exists() else []

    def test_packages_get_distinct_owned_clones_without_recursive_mode(self):
        result=self.run_wrapper("test","-timeout=40m","-p","1","-parallel","2","./...")
        self.assertEqual(result.returncode,0,result.stderr)
        calls=[x for x in self.calls() if "go" in x]
        self.assertEqual(len(calls),2)
        self.assertNotEqual(calls[0]["database"],calls[1]["database"])
        self.assertTrue(all("/ccheck_" in x["database"] and x["mode"]=="0" for x in calls))
        self.assertEqual(len(list((self.root/"evidence").glob("*.json"))),2)

    def test_default_and_remote_templates_refuse_before_database_commands(self):
        for url in ("", "postgres://fixture@127.0.0.1/multica", "postgres://fixture@remote/multica_check_fixture_go"):
            self.assertEqual(self.run_wrapper("test","./...",DATABASE_URL=url).returncode,2)
        self.assertEqual(self.calls(),[])

    def test_compile_and_non_test_calls_never_touch_database(self):
        self.assertEqual(self.run_wrapper("test","-c","./fixture",DATABASE_URL="").returncode,0)
        self.assertEqual(self.run_wrapper("version",DATABASE_URL="").stdout.strip(),"fake-go-version")
        self.assertEqual(self.calls(),[])

    def test_silent_testmain_success_is_not_named_test_evidence(self):
        self.assertEqual(self.run_wrapper("test","./...",CHECK_FAKE_SILENT="1").returncode,2)

    def test_no_test_files_are_compilation_only(self):
        self.assertEqual(self.run_wrapper("test","./...",CHECK_FAKE_NO_TESTS="1").returncode,0)
        for p in (self.root/"evidence").glob("*.json"):
            record=json.loads(p.read_text())
            self.assertTrue(record["compiled_without_tests"])
            self.assertFalse(record["package_passed"])

    def test_unmarked_test_execution_refuses_before_real_go_or_sql(self):
        env=self.env.copy()
        env.pop("MULTICA_TASK_PACKAGE_DB_MODE",None)
        env.pop("MULTICA_TASK_REAL_GO",None)
        env["DATABASE_URL"]="postgres://fixture@127.0.0.1/multica"
        selections=[("test","./fixture"),("-C",str(self.root),"test","./fixture"),("test","-args","-c"),("test","-run","-c"),("test","-c","-c=false")]
        for args in selections:
            result=subprocess.run([str(WRAPPER),*args],env=env,text=True,capture_output=True)
            self.assertEqual(result.returncode,2,(args,result.stderr))
        self.assertEqual(self.calls(),[])

    def test_unmarked_real_compile_flag_still_compiles_without_a_database(self):
        for args in (("test","-c","./fixture"),("-C",str(self.root),"test","-c=true","./fixture")):
            result=self.run_wrapper(*args,MULTICA_TASK_PACKAGE_DB_MODE="",DATABASE_URL="")
            self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(self.calls(),[])

    def test_recursive_context_refuses_default_or_unowned_database(self):
        for url in ("","postgres://fixture@127.0.0.1/multica","postgres://fixture@remote/ccheck_20261101235959999999_abcdef123456","postgres://fixture@127.0.0.1/ccheck_user_data"):
            result=self.run_wrapper("test","./fixture",MULTICA_TASK_PACKAGE_DB_MODE="0",DATABASE_URL=url)
            self.assertEqual(result.returncode,2,result.stderr)
        self.assertEqual(self.calls(),[])

    def test_recursive_context_forwards_only_its_explicit_owned_clone(self):
        url="postgres://fixture@127.0.0.1/ccheck_20261101235959999999_abcdef123456"
        evidence=self.root/"evidence"
        evidence.mkdir()
        (evidence/"ccheck_20261101235959999999_abcdef123456.json").write_text(json.dumps({"database":"ccheck_20261101235959999999_abcdef123456","state":"running","template":"multica_check_fixture_go"}))
        result=self.run_wrapper("test","./fixture",MULTICA_TASK_PACKAGE_DB_MODE="0",DATABASE_URL=url)
        self.assertEqual(result.returncode,0,result.stderr)
        calls=self.calls()
        self.assertEqual(len(calls),1)
        self.assertEqual(calls[0]["database"],url)
        self.assertEqual(calls[0]["mode"],"0")

    def test_recursive_named_clone_without_allocation_record_refuses(self):
        url="postgres://fixture@127.0.0.1/ccheck_20261101235959999999_abcdef123456"
        result=self.run_wrapper("test","./fixture",MULTICA_TASK_PACKAGE_DB_MODE="0",DATABASE_URL=url)
        self.assertEqual(result.returncode,2,result.stderr)
        self.assertEqual(self.calls(),[])

class PoolEnvironmentTests(unittest.TestCase):
    def setUp(self):
        self.fixture = IsolationLauncherTests("test_packages_get_distinct_owned_clones_without_recursive_mode")
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.fixture.go.write_text('''#!/usr/bin/env python3
import json, os, sys
arguments = sys.argv[1:]
if arguments[:1] == ['list']:
 print('example/first\\nexample/second')
else:
 before = {key: os.environ.get(key) for key in ('DATABASE_MAX_CONNS', 'DATABASE_MIN_CONNS', 'HOME', 'MULTICA_TASK_CONFIG_ROOT', 'MULTICA_TASK_WORKSPACES_ROOT')}
 if os.environ.get('CHECK_FAKE_TEST_OVERRIDES') == '1':
  os.environ['DATABASE_MAX_CONNS'] = '100'
  os.environ['DATABASE_MIN_CONNS'] = '20'
  os.environ['HOME'] = os.environ['CHECK_FAKE_TEST_HOME']
 after = {key: os.environ.get(key) for key in before}
 entry = {'go': arguments, 'database': os.environ.get('DATABASE_URL'), 'before': before, 'after': after}
 with open(os.environ['CHECK_ISOLATION_FAKE_RECORD'], 'a') as sink:
  sink.write(json.dumps(entry) + '\\n')
 if 'test' in arguments and '-c' not in arguments and '-c=true' not in arguments:
  print(json.dumps({'Action': 'run', 'Test': 'TestFixture'}))
  print(json.dumps({'Action': 'pass', 'Test': 'TestFixture'}))
  print(json.dumps({'Action': 'pass', 'Package': arguments[-1]}))
 sys.exit(int(os.environ.get('CHECK_FAKE_EXIT', '0')))
''')
        self.pool_env = {"DATABASE_MAX_CONNS": "10", "DATABASE_MIN_CONNS": "1", "HOME": str(self.fixture.root / "caller-home")}

    def go_calls(self):
        return [entry for entry in self.fixture.calls() if "go" in entry]

    def recursive_env(self):
        name = "ccheck_20261101235959999999_abcdef123456"
        evidence = self.fixture.root / "evidence"
        evidence.mkdir()
        (evidence / (name + ".json")).write_text(json.dumps({"database": name, "state": "running", "template": "multica_check_fixture_go"}))
        return {**self.pool_env, "MULTICA_TASK_PACKAGE_DB_MODE": "0", "DATABASE_URL": "postgres://fixture@127.0.0.1/" + name}

    def assert_default_environment(self, entries):
        for entry in entries:
            self.assertIsNone(entry["before"]["DATABASE_MAX_CONNS"], entry)
            self.assertIsNone(entry["before"]["DATABASE_MIN_CONNS"], entry)
            self.assertEqual(entry["before"]["HOME"], self.pool_env["HOME"])

    def test_fresh_package_children_do_not_inherit_api_pool_defaults(self):
        result = self.fixture.run_wrapper("test", "-parallel", "2", "./...", **self.pool_env)
        self.assertEqual(result.returncode, 0, result.stderr)
        entries = self.go_calls()
        self.assertEqual(len(entries), 2)
        self.assert_default_environment(entries)

    def test_validated_recursive_child_does_not_inherit_api_pool_defaults(self):
        result = self.fixture.run_wrapper("test", "./fixture", **self.recursive_env())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_default_environment(self.go_calls())
        self.assertEqual(len(self.go_calls()), 1)

    def test_non_test_forwarding_preserves_api_budget_and_arguments(self):
        arguments = ("-C", str(self.fixture.root), "build", "-p", "2", "./cmd/server")
        result = self.fixture.run_wrapper(*arguments, **self.pool_env)
        self.assertEqual(result.returncode, 0, result.stderr)
        entry = self.go_calls()[0]
        self.assertEqual(entry["go"], list(arguments))
        self.assertEqual(entry["before"]["DATABASE_MAX_CONNS"], "10")
        self.assertEqual(entry["before"]["DATABASE_MIN_CONNS"], "1")
        self.assertFalse(any("sql" in item for item in self.fixture.calls()))

    def test_compile_only_forwarding_preserves_api_budget_and_arguments(self):
        arguments = ("test", "-c", "-o", "test-owned-output", "./fixture")
        result = self.fixture.run_wrapper(*arguments, **self.pool_env)
        self.assertEqual(result.returncode, 0, result.stderr)
        entry = self.go_calls()[0]
        self.assertEqual(entry["go"], list(arguments))
        self.assertEqual(entry["before"]["DATABASE_MAX_CONNS"], "10")
        self.assertEqual(entry["before"]["DATABASE_MIN_CONNS"], "1")
        self.assertFalse(any("sql" in item for item in self.fixture.calls()))

    def test_test_owned_pool_and_home_overrides_remain_available(self):
        test_home = str(self.fixture.root / "test-home")
        result = self.fixture.run_wrapper("test", "./fixture", **self.pool_env,
                                          CHECK_FAKE_TEST_OVERRIDES="1", CHECK_FAKE_TEST_HOME=test_home)
        self.assertEqual(result.returncode, 0, result.stderr)
        entries = self.go_calls()
        self.assert_default_environment(entries)
        for entry in entries:
            self.assertEqual(entry["after"]["DATABASE_MAX_CONNS"], "100")
            self.assertEqual(entry["after"]["DATABASE_MIN_CONNS"], "20")
            self.assertEqual(entry["after"]["HOME"], test_home)

    def test_forwarded_exit_code_is_preserved(self):
        result = self.fixture.run_wrapper("version", **self.pool_env, CHECK_FAKE_EXIT="43")
        self.assertEqual(result.returncode, 43)


class PlaywrightWiringTests(unittest.TestCase):
    def test_allocated_api_and_renderer_are_explicit_without_network(self):
        with tempfile.TemporaryDirectory(prefix="multica-fixture-wiring-") as temp:
            root = Path(temp)
            scripts = root / "scripts"
            scripts.mkdir()
            guard = scripts / "go-test-with-agent-cli-guard.sh"
            guard.write_text('#!/usr/bin/env bash\nset -eu\n: > "$CHECK_FAKE_GUARD_CALLED"\ntest "$1" = --\nshift\nexec "$@"\n')
            guard.chmod(0o700)
            binary = root / "fake-pnpm"
            binary.write_text('#!/usr/bin/env python3\nimport os,json\nprint(json.dumps({key:os.environ.get(key) for key in ["COLLABORATION_E2E_API_URL","COLLABORATION_ELECTRON_SOURCE_ROOT","COLLABORATION_ELECTRON_RENDERER_URL"]}))\n')
            binary.chmod(0o700)
            env = {**os.environ, "MULTICA_TASK_REAL_PNPM":str(binary), "MULTICA_TASK_CANDIDATE_ROOT":str(root),
                   "MULTICA_TASK_RENDERER_URL":"http://127.0.0.1:19841", "NEXT_PUBLIC_API_URL":"http://127.0.0.1:18081",
                   "DATABASE_URL":"postgres://fixture@127.0.0.1/multica_check_fixture_api",
                   "MULTICA_TASK_PACKAGE_DB_EVIDENCE":str(root/"packages"), "CHECK_FAKE_GUARD_CALLED":str(root/"guard-called")}
            command = [str(WRAPPER.with_name("pnpm")), "exec", "playwright", "test"]
            result = subprocess.run(command, env=env, text=True, capture_output=True)
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual(json.loads(result.stdout), {"COLLABORATION_E2E_API_URL":"http://127.0.0.1:18081",
                "COLLABORATION_ELECTRON_SOURCE_ROOT":str(root),"COLLABORATION_ELECTRON_RENDERER_URL":"http://127.0.0.1:19841"})
            self.assertTrue((root/"makecheck-playwright-environment.json").is_file())
            self.assertTrue((root/"guard-called").is_file())
            wiring=json.loads((root/"makecheck-playwright-environment.json").read_text())
            self.assertEqual(wiring["command"].count("--reporter=line,json"),1)

            env["DATABASE_URL"] = "postgres://fixture@127.0.0.1/multica"
            self.assertNotEqual(subprocess.run(command,env=env,capture_output=True).returncode,0)

if __name__ == "__main__": unittest.main(verbosity=2)
