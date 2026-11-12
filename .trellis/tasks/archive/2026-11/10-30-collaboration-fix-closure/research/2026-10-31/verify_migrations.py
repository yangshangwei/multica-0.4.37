#!/usr/bin/env python3
"""Rehearse the closure migration in fresh, explicitly task-owned databases."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import urllib.parse
import uuid

ROOT = Path(__file__).resolve().parents[5]
REPORT = Path(__file__).resolve().parent
SCRATCH = ROOT / '.omx/reports/closure-evidence-20261031'
PSQL = '/opt/homebrew/opt/libpq/bin/psql'
HEAD = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
ENV = os.environ.copy()
source = ROOT / '.env'
for line in source.read_text().splitlines():
    if line.startswith('DATABASE_URL='):
        database_url = line.split('=', 1)[1].strip().strip('"').strip("'")
parsed = urllib.parse.urlsplit(database_url)
assert parsed.hostname in ('localhost', '127.0.0.1', '::1')
ENV.update(PGHOST=parsed.hostname, PGPORT=str(parsed.port or 5432),
           PGUSER=urllib.parse.unquote(parsed.username or ''),
           PGPASSWORD=urllib.parse.unquote(parsed.password or ''),
           PGDATABASE='postgres', PGSSLMODE='disable',
           PATH='/opt/homebrew/opt/libpq/bin:' + ENV['PATH'])
RESUME = len(sys.argv) > 1 and sys.argv[1] == '--resume'
previous = json.loads((REPORT / 'migration-summary.json').read_text()) if RESUME else None
DB = previous['upgrade_database'] if previous else 'multica_closure_20261031_' + uuid.uuid4().hex[:10]
FRESH = DB + '_fresh'
summary = {'source_head': HEAD, 'started_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
           'database_host': parsed.hostname, 'database_port': parsed.port or 5432,
           'upgrade_database': DB, 'fresh_database': FRESH, 'checks': []}
if previous:
    assert previous['source_head'] == HEAD
    summary = previous

def run(name, args, cwd=ROOT, env=ENV, stdin=None):
    start = datetime.datetime.now(datetime.timezone.utc).isoformat()
    log = REPORT / (name + '.log')
    with log.open('w') as output:
        output.write(json.dumps({'source_head': HEAD, 'started_at': start, 'cwd': str(cwd),
            'command': args, 'database': env.get('PGDATABASE')}, ensure_ascii=False) + '\n')
        output.flush()
        result = subprocess.run(args, cwd=cwd, env=env, input=stdin, text=True, stdout=output, stderr=subprocess.STDOUT)
        output.write('\nexit_code=' + str(result.returncode) + '\n')
    summary['checks'].append({'name': name, 'exit_code': result.returncode, 'log': log.name})
    (REPORT / 'migration-summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(name, 'exit=' + str(result.returncode), flush=True)
    if result.returncode:
        print(log.read_text()[-5000:], flush=True)
        raise SystemExit(result.returncode)

def sql(name, statement, db=DB):
    env = ENV | {'PGDATABASE': db}
    run(name, [PSQL, '-X', '-v', 'ON_ERROR_STOP=1', '-f', '-'], env=env, stdin=statement)

def migrate(name, stage, db=DB, direction='up'):
    current = parsed._replace(path='/' + db)
    env = ENV | {'PGDATABASE': db, 'DATABASE_URL': urllib.parse.urlunsplit(current)}
    run(name, [str(SCRATCH / 'migrate'), direction], cwd=stage, env=env)

if not RESUME:
    SCRATCH.mkdir(parents=True, exist_ok=True)
    run('build-migration-runner', ['go', 'build', '-o', str(SCRATCH / 'migrate'), './cmd/migrate'], cwd=ROOT / 'server')
    for name, select in [('stage745', lambda p: p.name < '746_'), ('stage746', lambda p: p.name.startswith('746_'))]:
        stage = SCRATCH / name / 'migrations'
        stage.mkdir(parents=True, exist_ok=True)
        for path in (ROOT / 'server/migrations').glob('*.sql'):
            if select(path):
                shutil.copyfile(path, stage / path.name)
    run('database-create', [PSQL, '-X', '-v', 'ON_ERROR_STOP=1', '-c', 'CREATE DATABASE "' + DB + '"', '-c', 'CREATE DATABASE "' + FRESH + '"'])
    sql('database-identity', 'SELECT current_database(), version(), current_user;')
    migrate('baseline-through-745', SCRATCH / 'stage745')
    sql('baseline-inspection', "SELECT max(version), count(*) FROM schema_migrations;\nSELECT column_name,column_default,is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name IN ('agent','collaboration_target') ORDER BY table_name,ordinal_position;")
    migrate('fresh-through-746', ROOT / 'server', db=FRESH)

run('migration-prefix-lint-complete', ['go', 'test', './internal/migrations', '-run', 'Test(MigrationFiles|MigrationNumeric|NewMigrationPrefixes)', '-count=1', '-v'], cwd=ROOT / 'server')
sql('seed-745-history', """
INSERT INTO "user" (id,name,email) VALUES ('11111111-1111-4111-8111-111111111111','Closure migration actor','closure-migration@example.invalid');
INSERT INTO workspace (id,name,slug) VALUES ('22222222-2222-4222-8222-222222222222','Closure migration workspace','closure-migration');
INSERT INTO agent (id,workspace_id,name,owner_id,runtime_mode,instructions)
VALUES ('33333333-3333-4333-8333-333333333333','22222222-2222-4222-8222-222222222222','Retained agent','11111111-1111-4111-8111-111111111111','local','Original instructions');
INSERT INTO collaboration_target (id,session_id,kind,resource_id,roster,configuration_fingerprint)
VALUES ('44444444-4444-4444-8444-444444444444','55555555-5555-4555-8555-555555555555','agent','33333333-3333-4333-8333-333333333333','[]',repeat('a',64));
CREATE TABLE closure_verification_snapshot AS SELECT to_jsonb(t) AS target FROM collaboration_target t;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='agent' AND column_name='collaboration_boundary_epoch') THEN RAISE EXCEPTION 'baseline already has boundary epoch'; END IF;
 IF NOT EXISTS(SELECT 1 FROM schema_migrations WHERE version='745_drop_user_cloud_waitlist') THEN RAISE EXCEPTION '745 not applied'; END IF;
END $$;
""")
migrate('upgrade-745-to-746', ROOT / 'server')
sql('assert-upgrade-and-epoch-contract', """
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM schema_migrations WHERE version='746_collaboration_owner_reconsent') THEN RAISE EXCEPTION '746 not applied'; END IF;
 IF (SELECT to_jsonb(t)-'configuration_boundary_fingerprint' FROM collaboration_target t) IS DISTINCT FROM (SELECT target FROM closure_verification_snapshot) THEN RAISE EXCEPTION 'target history changed during upgrade'; END IF;
 IF EXISTS(SELECT 1 FROM collaboration_target WHERE configuration_boundary_fingerprint IS NOT NULL) THEN RAISE EXCEPTION 'legacy target must remain Switch-only'; END IF;
 IF NOT EXISTS(SELECT 1 FROM agent WHERE name='Retained agent' AND instructions='Original instructions' AND collaboration_config_epoch=1 AND collaboration_boundary_epoch IS NULL) THEN RAISE EXCEPTION 'agent history changed during upgrade'; END IF;
END $$;
UPDATE agent SET instructions='Reviewed instructions' WHERE name='Retained agent';
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM agent WHERE name='Retained agent' AND collaboration_config_epoch=2 AND collaboration_boundary_epoch IS NULL) THEN RAISE EXCEPTION 'instruction edit widened execution boundary'; END IF;
END $$;
UPDATE agent SET custom_env='{"VERIFICATION_VALUE":"fixture"}' WHERE name='Retained agent';
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM agent WHERE name='Retained agent' AND collaboration_config_epoch=3 AND collaboration_boundary_epoch=1) THEN RAISE EXCEPTION 'permission change failed to widen execution boundary'; END IF;
END $$;
UPDATE agent SET collaboration_config_epoch=collaboration_config_epoch+1 WHERE name='Retained agent';
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM agent WHERE name='Retained agent' AND collaboration_config_epoch=4 AND collaboration_boundary_epoch=2) THEN RAISE EXCEPTION 'explicit epoch change failed to widen boundary'; END IF;
 BEGIN
   UPDATE agent SET collaboration_boundary_epoch=3 WHERE name='Retained agent';
   RAISE EXCEPTION 'forged boundary epoch accepted';
 EXCEPTION WHEN check_violation THEN NULL; END;
 BEGIN
   UPDATE agent SET collaboration_config_epoch=6 WHERE name='Retained agent';
   RAISE EXCEPTION 'skipped config epoch accepted';
 EXCEPTION WHEN check_violation THEN NULL; END;
END $$;
SELECT id,collaboration_config_epoch,collaboration_boundary_epoch FROM agent WHERE name='Retained agent';
""")
migrate('down-746-only', SCRATCH / 'stage746', direction='down')
sql('assert-down-retains-history', """
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM information_schema.columns WHERE (table_name='agent' AND column_name='collaboration_boundary_epoch') OR (table_name='collaboration_target' AND column_name='configuration_boundary_fingerprint')) THEN RAISE EXCEPTION 'boundary columns survived rollback'; END IF;
 IF EXISTS(SELECT 1 FROM schema_migrations WHERE version='746_collaboration_owner_reconsent') THEN RAISE EXCEPTION '746 ledger survived rollback'; END IF;
 IF (SELECT to_jsonb(t) FROM collaboration_target t) IS DISTINCT FROM (SELECT target FROM closure_verification_snapshot) THEN RAISE EXCEPTION 'target history changed during rollback'; END IF;
END $$;
UPDATE agent SET instructions='Post-rollback instructions' WHERE name='Retained agent';
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM agent WHERE name='Retained agent' AND collaboration_config_epoch=5 AND custom_env='{"VERIFICATION_VALUE":"fixture"}'::jsonb) THEN RAISE EXCEPTION '704 trigger not restored'; END IF;
END $$;
""")
migrate('up-after-rollback', ROOT / 'server')
sql('assert-up-after-rollback', """
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM agent WHERE name='Retained agent' AND collaboration_config_epoch=5 AND collaboration_boundary_epoch IS NULL AND instructions='Post-rollback instructions') THEN RAISE EXCEPTION 're-upgrade lost retained agent data'; END IF;
 IF (SELECT to_jsonb(t)-'configuration_boundary_fingerprint' FROM collaboration_target t) IS DISTINCT FROM (SELECT target FROM closure_verification_snapshot) THEN RAISE EXCEPTION 're-upgrade changed target history'; END IF;
 IF EXISTS(SELECT 1 FROM collaboration_target WHERE configuration_boundary_fingerprint IS NOT NULL) THEN RAISE EXCEPTION 're-upgrade invented authority'; END IF;
END $$;
CREATE TABLE closure_verification_final AS SELECT to_jsonb(a) AS agent FROM agent a WHERE name='Retained agent';
""")
migrate('reapply-746', ROOT / 'server')
sql('assert-reapply-noop', """
DO $$ BEGIN
 IF (SELECT to_jsonb(a) FROM agent a WHERE name='Retained agent') IS DISTINCT FROM (SELECT agent FROM closure_verification_final) THEN RAISE EXCEPTION 'reapply changed agent'; END IF;
 IF (SELECT to_jsonb(t)-'configuration_boundary_fingerprint' FROM collaboration_target t) IS DISTINCT FROM (SELECT target FROM closure_verification_snapshot) THEN RAISE EXCEPTION 'reapply changed target'; END IF;
END $$;
SELECT current_database(), max(version), count(*) FROM schema_migrations;
""")
summary['completed_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
summary['result'] = 'passed'
(REPORT / 'migration-summary.json').write_text(json.dumps(summary, indent=2) + '\n')
