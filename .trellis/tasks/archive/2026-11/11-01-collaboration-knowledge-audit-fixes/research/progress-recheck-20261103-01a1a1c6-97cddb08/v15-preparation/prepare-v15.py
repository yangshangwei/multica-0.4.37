"""Create successor templates only; this script executes no prepared command."""
import ast
import datetime
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shlex
import tokenize


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PROJECT = Path('/Volumes/artisan/code/2026/multica-0.4.37')
OLD_OUT = ROOT / '.omx/private/final-check-v14'
OUT = ROOT / '.omx/private/final-check-v15'
OLD_HERE = PROJECT / '.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261103-2100/candidate-v14'
HERE = Path(__file__).resolve().parent
OLD_OWNER = '01a1a186-9a5c-7b00-96f8-0430b47edb4c'
OWNER = '01a1a1c6-9f9b-7521-8341-f57c97cddb08'
OLD_SHA = 'd0b132fff0c477eb2d744933e13db730240231aa'
DEPLOYMENT_UUID = '725f8e4c-8979-4fc2-b105-36a6ce89a2e5'
ROWS = []
PROTECTED = {}


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write(path, data, mode=0o600):
    assert path.is_relative_to(OUT) or path.is_relative_to(HERE)
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
    with os.fdopen(descriptor, 'wb') as target:
        target.write(data)


def rebase(source, python=True):
    source = re.sub(r'/private/tmp/multica-final-check-v14-[A-Za-z0-9_]+',
                    'PENDING_ROOT_BUILD_TEMP_DIRECTORY', source)
    source = source.replace(str(OLD_HERE), str(HERE)).replace(str(OLD_OUT), str(OUT))
    source = source.replace(OLD_OWNER, OWNER).replace(OLD_SHA, 'PENDING_ROOT_FINAL_SHA')
    source = source.replace('v14', 'v15').replace('V14', 'V15')
    source = source.replace('multica_closure_v15_seed_01a1a186', 'multica_closure_v15_seed_01a1a1c6')
    if python:
        output = []
        for token in tokenize.generate_tokens(io.StringIO(source).readline):
            if token.type == tokenize.NUMBER and token.string in ('55131', '55132'):
                marker = 'PENDING_ROOT_REDIS_PORT' if token.string == '55131' else 'PENDING_ROOT_RENDERER_PORT'
                token = token._replace(string='int("' + marker + '")')
            elif token.type == tokenize.STRING:
                token = token._replace(string=token.string.replace('55131', 'PENDING_ROOT_REDIS_PORT')
                                       .replace('55132', 'PENDING_ROOT_RENDERER_PORT'))
            output.append(token)
        source = tokenize.untokenize(output)
        ast.parse(source)
    else:
        source = source.replace('55131', 'PENDING_ROOT_REDIS_PORT').replace('55132', 'PENDING_ROOT_RENDERER_PORT')
    return source


def template(original, destination, final, change='Version/evidence path and root-supplied pending identity rebinding', transform=None):
    PROTECTED[str(original)] = digest(original)
    content = original.read_text()
    if transform:
        content = transform(content)
    else:
        content = rebase(content, python=original.suffix == '.py')
    if final.suffix == '.py' or original.name == 'pnpm':
        ast.parse(content)
    write(destination, content.encode())
    ROWS.append({'original': str(original), 'original_sha256': digest(original),
                 'prepared_template': str(destination), 'prepared_sha256': digest(destination),
                 'final_path': str(final), 'materialize': True, 'mode': '0700' if original.name in ('go', 'pnpm') else '0600',
                 'change': change})


def launcher(source):
    source = rebase(source)
    source = source.replace('from urllib.parse import urlsplit\n', 'from urllib.parse import urlsplit\nimport uuid\n', 1)
    validator = '''\n\ndef deployment_uuid(values):
    raw = values.get("MULTICA_DEPLOYMENT_ID", "")
    try:
        parsed = uuid.UUID(raw)
    except (AttributeError, TypeError, ValueError):
        raise RuntimeError("Task deployment requires a canonical nonzero UUID") from None
    require(raw == str(parsed) and parsed.int != 0,
            "Task deployment requires a canonical nonzero UUID")
    return raw
'''
    source = source.replace('\n\ndef renderer_url(values):', validator + '\n\ndef renderer_url(values):', 1)
    original_read = '    source = out / "check-source-v15.env"\n    source_bytes = source.read_bytes()\n'
    assert source.count(original_read) == 1
    source = source.replace(original_read, '', 1)
    original_parse = '    values = parse_generated_environment(source_bytes.decode())\n'
    assert source.count(original_parse) == 1
    source = source.replace(original_parse, '', 1)
    first_subprocess = '    head = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()\n'
    assert first_subprocess in source
    source = source.replace(first_subprocess,
                            '    # Reject malformed deployment data before every subprocess, including metadata.\n'
                            + original_read + original_parse + '    deployment_id = deployment_uuid(values)\n' + first_subprocess, 1)
    source = source.replace('    # Parse the reviewed source as data. The unchanged make scripts receive a\n'
                            '    # canonical, shell-quoted copy, never the original unparsed dotenv bytes.\n',
                            '    # The reviewed data will become a canonical, shell-quoted Make copy.\n', 1)
    source = source.replace('        "renderer": renderer, "knowledge_default": False, "real_provider_allowed": False,',
                            '        "renderer": renderer, "deployment_id": deployment_id, "knowledge_default": False, "real_provider_allowed": False,', 1)
    return source


def pnpm(source):
    insertion = '''\ncanonical_turbo_tests = [
    "exec", "turbo", "run", "test", "--filter=!@multica/mobile",
    "--concurrency=1", "--force", "--", "--maxWorkers=2",
]
if args == canonical_turbo_tests:
    args.append("--reporter=json")
    output = Path(environment["MULTICA_TASK_PACKAGE_DB_EVIDENCE"]).parent / "makecheck-vitest-reporting-v15.json"
    record = {"original_argv": canonical_turbo_tests, "effective_argv": args,
              "candidate": environment["MULTICA_TASK_CANDIDATE_ROOT"],
              "reporter": "json", "transport": "per-package-prefixed stdout in the actual full raw log",
              "strict_turbo_environment_unchanged": True, "scope": "reporting only; no synthetic test results"}
    with output.open("x") as target:
        output.chmod(0o600)
        target.write(json.dumps(record, indent=2) + "\\n")
'''
    anchor = 'real = environment.get("MULTICA_TASK_REAL_PNPM") or "/opt/homebrew/bin/pnpm"\n'
    assert source.count(anchor) == 1
    return source.replace(anchor, anchor + insertion, 1)


def fake_launcher(source):
    source = rebase(source)
    anchor = '            "MULTICA_PLATFORM_ADMIN_ENABLED=false",\n'
    assert source.count(anchor) == 1
    source = source.replace(anchor, anchor + '            "MULTICA_DEPLOYMENT_ID=2c019f6e-fab7-4aec-862d-09c20cc097bc",\n', 1)
    addition = '''\n    def test_invalid_deployment_uuid_refuses_before_any_subprocess(self):
        values = ["collaboration-closure-v14-01a1a186", "00000000-0000-0000-0000-000000000000",
                  "2C019F6E-FAB7-4AEC-862D-09C20CC097BC", "2c019f6efab74aec862d09c20cc097bc",
                  "{2c019f6e-fab7-4aec-862d-09c20cc097bc}", "", None]
        original = self.source_env.read_text()
        for value in values:
            with self.subTest(value=value):
                kept = [line for line in original.splitlines() if not line.startswith("MULTICA_DEPLOYMENT_ID=")]
                if value is not None:
                    kept.append("MULTICA_DEPLOYMENT_ID=" + shlex.quote(value))
                self.source_env.write_text("\\n".join(kept) + "\\n")
                module = runpy.run_path(str(self.launcher), run_name="private_uuid_probe")
                with patch.object(subprocess, "check_output", side_effect=AssertionError("Unexpected subprocess before UUID refusal")) as output, \\
                     patch.object(subprocess, "run", side_effect=AssertionError("Unexpected subprocess before UUID refusal")) as run, \\
                     patch.object(subprocess, "Popen", side_effect=AssertionError("Unexpected child before UUID refusal")) as popen:
                    with self.assertRaisesRegex(RuntimeError, "canonical nonzero UUID"):
                        module["main"]()
                    output.assert_not_called()
                    run.assert_not_called()
                    popen.assert_not_called()
                self.assertFalse((self.out / "make-check-v15.json").exists())
                self.assertFalse((self.out / "check-source-v15.safe.env").exists())
        self.source_env.write_text(original)
'''
    anchor = '\n    def prepare_owned_fixture(self):'
    assert source.count(anchor) == 1
    return source.replace(anchor, addition + anchor, 1)


def adapter_controls(source):
    source = rebase(source)
    addition = '''\n    def test_exact_turbo_test_scope_adds_only_json_reporting(self):
        fake = self.executable("fake-pnpm", "import json,sys\\nprint(json.dumps({'argv':sys.argv[1:]}))\\n")
        evidence = self.fixture / "synthetic-go-evidence"
        self.env.update(MULTICA_TASK_REAL_PNPM=str(fake), MULTICA_TASK_PACKAGE_DB_EVIDENCE=str(evidence),
                        MULTICA_TASK_CANDIDATE_ROOT=str(self.fixture / "synthetic-candidate"))
        arguments = ["exec", "turbo", "run", "test", "--filter=!@multica/mobile", "--concurrency=1", "--force", "--", "--maxWorkers=2"]
        result = self.invoke("pnpm", arguments)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["argv"], arguments + ["--reporter=json"])
        record = json.loads((self.fixture / "makecheck-vitest-reporting-v15.json").read_text())
        self.assertEqual(record["original_argv"], arguments)
        self.assertEqual(record["effective_argv"], arguments + ["--reporter=json"])
        self.assertEqual(record["transport"], "per-package-prefixed stdout in the actual full raw log")

    def test_other_turbo_invocation_is_not_rewritten(self):
        fake = self.executable("fake-pnpm", "import json,sys\\nprint(json.dumps({'argv':sys.argv[1:]}))\\n")
        self.env['MULTICA_TASK_REAL_PNPM'] = str(fake)
        arguments = ["exec", "turbo", "run", "test", "--filter=!@multica/mobile", "--concurrency=1", "--force", "--", "--maxWorkers=1"]
        result = self.invoke("pnpm", arguments)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["argv"], arguments)
        self.assertFalse((self.fixture / "makecheck-vitest-reporting-v15.json").exists())
'''
    anchor = '\n    def fake_go_and_sql(self):'
    assert source.count(anchor) == 1
    return source.replace(anchor, addition + anchor, 1)


def environment(source):
    values = {}
    for line in source.splitlines():
        if not line.strip() or line.lstrip().startswith('#'):
            continue
        key, raw = line.split('=', 1)
        tokens = shlex.split(raw)
        assert len(tokens) <= 1
        values[key] = rebase(tokens[0] if tokens else '', python=False)
    values.update(DATABASE_URL='PENDING_ROOT_DATABASE_URL', JWT_SECRET='PENDING_ROOT_PRIVATE_JWT_SECRET',
                  MULTICA_DEPLOYMENT_ID=DEPLOYMENT_UUID)
    return '# Root-supplied private values remain pending; this is not a runnable environment.\n' + ''.join(
        key + '=' + shlex.quote(value) + '\n' for key, value in values.items())


def allocator(source):
    source = rebase(source).replace('multica-v15-01a1a186-redis', 'multica-v15-01a1a1c6-redis')
    source = source.replace("          'previous_failed_preflight': str(HERE / 'resource-preflight-failure-01.json'),\n"
                            "          'previous_failure_boundary': 'absent Unix socket; read-only connection failed before CREATE; no DB reached',\n", '')
    return source


for source, transform in [
    ('run-make-check-v14-provenance.py', launcher),
    ('fake-preflight-v14-provenance.py', fake_launcher),
    ('adapter-env-preflight-v14.py', adapter_controls),
    ('check-source-v14.env', environment),
]:
    final = OUT / source.replace('v14', 'v15')
    template(OLD_OUT / source, final.with_name(final.name + '.in'), final, transform=transform)

final = OUT / 'check-bin/pnpm'
template(OLD_OUT / 'check-bin/pnpm', final.with_name('pnpm.in'), final,
         change='Exact canonical Turbo test argv appends JSON reporter; other argv and Playwright logic unchanged', transform=pnpm)
for name in ['go']:
    original = OLD_OUT / 'check-bin' / name
    PROTECTED[str(original)] = digest(original)
    final = OUT / 'check-bin' / name
    write(final, original.read_bytes(), 0o700)
    ROWS.append({'original': str(original), 'original_sha256': digest(original), 'prepared_template': str(final),
                 'prepared_sha256': digest(final), 'final_path': str(final), 'materialize': False,
                 'mode': '0700', 'change': 'Byte-identical reviewed Go adapter'})

for name in ['checkout-source-id-v14.cjs', 'resolve-workspaces-v14.mjs', 'electron.vite.config.cts',
             'cli-build-offline.sb', 'desktop-build-offline.sb', 'private-preflight-offline.sb']:
    final = OUT / name.replace('v14', 'v15')
    template(OLD_OUT / name, final.with_name(final.name + '.in'), final)

for name in ['prepare-bindings-v14.py', 'run-private-preflight.py',
             'allocate-fresh-db-redis.py', 'migrate-seed.py', 'start-renderer.py',
             'candidate-setup/build-current-v14.py', 'candidate-setup/source-snapshot.py',
             'candidate-setup/capture-artifacts-v14.py',
             'final-state-audit-v14/final_state_audit.py', 'final-state-audit-v14/final_state_audit_test.py']:
    final = HERE / name.replace('v14', 'v15')
    template(OLD_HERE / name, final.with_name(final.name + '.in'), final,
             transform=allocator if name == 'allocate-fresh-db-redis.py' else None)

failed_materializer = OLD_HERE / 'prepare-current-resources.py'
PROTECTED[str(failed_materializer)] = digest(failed_materializer)
reference = HERE / 'historical-input-prepare-current-resources-v14.txt'
write(reference, failed_materializer.read_bytes())
historical = [{'path': str(reference), 'sha256': digest(reference),
               'reason': 'Retained failed mixed materializer/URI allocation path as historical reference; use Root materialization plus successful explicit endpoint allocator.'}]

changed = [path for path, expected in PROTECTED.items() if digest(Path(path)) != expected]
assert not changed
manifest = {'recorded_at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'coordinator': OWNER,
            'candidate_root': str(ROOT), 'candidate_sha': None, 'state': 'templates_only_not_frozen_not_runnable',
            'root_supplied_deployment_uuid': DEPLOYMENT_UUID, 'seed_database_proposal': 'multica_closure_v15_seed_01a1a1c6',
            'files': ROWS, 'protected_donor_hashes': PROTECTED, 'changed_donors': changed,
            'historical_nonoperational_references': historical, 'operational_template_files': len(ROWS),
            'pending_fields': ['PENDING_ROOT_FINAL_SHA', 'PENDING_ROOT_RENDERER_PORT', 'PENDING_ROOT_REDIS_PORT',
                               'PENDING_ROOT_PRIVATE_JWT_SECRET', 'PENDING_ROOT_DATABASE_URL', 'PENDING_ROOT_BUILD_TEMP_DIRECTORY'],
            'build_or_test_or_database_or_services_started': False, 'fullcheck_started': False,
            'knowledge_budget_allocated': 0, 'knowledge_real_calls': 0, 'real_provider_allowed': False,
            'validation_here': 'Python AST parsing and donor byte comparison only; prepared controls not executed'}
write(HERE / 'prepared-template-manifest.json', (json.dumps(manifest, indent=2) + '\n').encode())
print(json.dumps({'prepared_files': len(ROWS), 'changed_donors': changed, 'manifest': str(HERE / 'prepared-template-manifest.json'),
                  'runtime_tests_executed': 0, 'fullcheck_started': False}))
