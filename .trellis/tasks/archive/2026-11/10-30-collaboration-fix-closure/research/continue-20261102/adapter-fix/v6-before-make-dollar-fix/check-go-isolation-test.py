import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

WRAPPER = Path(os.environ.get("CHECK_ISOLATION_WRAPPER", str(Path(__file__).parent / "check-bin/go")))

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
 with open(os.environ['CHECK_ISOLATION_FAKE_RECORD'],'a') as f:f.write(json.dumps({'go':sys.argv[1:],'database':os.environ['DATABASE_URL'],'mode':os.environ['MULTICA_TASK_PACKAGE_DB_MODE'],'allocation_evidence':os.environ['MULTICA_TASK_PACKAGE_DB_EVIDENCE'],'pool_max':os.environ.get('DATABASE_MAX_CONNS'),'pool_min':os.environ.get('DATABASE_MIN_CONNS')})+'\\n')
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

    def test_api_pool_overrides_removed_for_package_tests(self):
        result = self.run_wrapper("test", "./...", DATABASE_MAX_CONNS="10", DATABASE_MIN_CONNS="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [call for call in self.calls() if "go" in call]
        self.assertEqual(len(calls), 2)
        for call in calls:
            self.assertIsNone(call["pool_max"], "API max override must not change unset/default tests")
            self.assertIsNone(call["pool_min"], "API min override must not change unset/default tests")
            self.assertIn("/ccheck_", call["database"])
            self.assertEqual(call["mode"], "0")
            self.assertEqual(call["allocation_evidence"], str(self.root / "evidence"))

    def test_api_pool_overrides_removed_for_recursive_tests(self):
        database = "ccheck_20261101235959999999_abcdef123456"
        url = "postgres://fixture@127.0.0.1/" + database
        evidence = self.root / "evidence"
        evidence.mkdir()
        (evidence / (database + ".json")).write_text(json.dumps({
            "database": database, "state": "running", "template": "multica_check_fixture_go"
        }))
        result = self.run_wrapper("test", "./fixture", MULTICA_TASK_PACKAGE_DB_MODE="0",
                                  DATABASE_URL=url, DATABASE_MAX_CONNS="10", DATABASE_MIN_CONNS="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        self.assertEqual(len(calls), 1)
        self.assertEqual(calls[0]["database"], url)
        self.assertEqual(calls[0]["mode"], "0")
        self.assertEqual(calls[0]["allocation_evidence"], str(evidence))
        self.assertIsNone(calls[0]["pool_max"], "Recursive tests own their unset/default max setting")
        self.assertIsNone(calls[0]["pool_min"], "Recursive tests own their unset/default min setting")

    def test_api_pool_overrides_preserved_for_non_test_calls(self):
        self.go.write_text("""#!/usr/bin/env python3
import json, os
print(json.dumps({key: os.environ.get(key) for key in ("DATABASE_MAX_CONNS", "DATABASE_MIN_CONNS")}))
""")
        result = self.run_wrapper("version", DATABASE_MAX_CONNS="10", DATABASE_MIN_CONNS="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout), {"DATABASE_MAX_CONNS": "10", "DATABASE_MIN_CONNS": "1"})
        self.assertEqual(self.calls(), [])

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
