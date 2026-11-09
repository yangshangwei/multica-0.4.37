import { readFileSync, writeFileSync, chmodSync } from 'node:fs';
import { createRequire } from 'node:module';
import { randomBytes, randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';
const root=process.cwd();
const require=createRequire(root+'/apps/web/package.json');
const {parse}=require('dotenv');
const pg=createRequire(root+'/package.json')('pg');
const allocationFile=root+'/artifacts/qa/collaboration-multi-account-20261109/environment/allocation.json';
const a=JSON.parse(readFileSync(allocationFile));
const source=parse(readFileSync(root+'/.env'));
if (!source.DATABASE_URL) throw Error('Source database endpoint is missing');
const target=new URL(source.DATABASE_URL);target.pathname='/'+a.database;
const admin=new URL(source.DATABASE_URL);admin.pathname='/postgres';
if (!['localhost','127.0.0.1'].includes(admin.hostname)||!/^multica_qa_20261109_[0-9a-f]+$/.test(a.database)) throw Error('Task database boundary rejected');
const client=new pg.Client({connectionString:admin.href,connectionTimeoutMillis:5000});
try {
 await client.connect();
 const capacity=(await client.query("SELECT current_database() AS database, current_setting('max_connections')::int AS maximum, (SELECT count(*)::int FROM pg_stat_activity) AS used")).rows[0];
 if(capacity.maximum-capacity.used<15) throw Error('Insufficient connection headroom');
 const existing=await client.query('SELECT 1 FROM pg_database WHERE datname=$1',[a.database]);
 if(existing.rowCount) throw Error('Refuse existing database');
 await client.query('CREATE DATABASE "'+a.database+'"');
 a.capacity_before=capacity;a.database_created=true;
} catch(e) { console.error('Task database allocation failed:',e.code??e.constructor.name);process.exitCode=2; } finally {await client.end();}
if(process.exitCode) process.exit();
const check=new pg.Client({connectionString:target.href,connectionTimeoutMillis:5000});
await check.connect();const actual=(await check.query('SELECT current_database() AS name')).rows[0].name;await check.end();
if(actual!==a.database) throw Error('Task database identity mismatch');
a.database_verified=actual;
const goEnv=JSON.parse(execFileSync('go',['env','-json','GOCACHE','GOMODCACHE','GOPATH'],{encoding:'utf8'}));
const api='http://127.0.0.1:'+a.api_port,web='http://127.0.0.1:'+a.web_port,renderer='http://127.0.0.1:'+a.renderer_port;
const env={PATH:process.env.PATH,HOME:a.private_root+'/home',USER:process.env.USER??'artisan',TMPDIR:process.env.TMPDIR??'/tmp',...goEnv,
 PLAYWRIGHT_BROWSERS_PATH:process.env.HOME+'/Library/Caches/ms-playwright',DATABASE_URL:target.href,TASK_DATABASE_NAME:a.database,
 DATABASE_MAX_CONNS:'8',DATABASE_MIN_CONNS:'1',APP_ENV:'development',NODE_ENV:'production',PORT:String(a.api_port),FRONTEND_PORT:String(a.web_port),FRONTEND_ORIGIN:web,
 JWT_SECRET:randomBytes(32).toString('hex'),ALLOW_SIGNUP:'true',MULTICA_AUTH_MODE:'password',MULTICA_PASSWORD_LIMITER_MODE:'single',MULTICA_DEVICE_AUTH_ENABLED:'false',MULTICA_CLOUD_URL:'',
 MULTICA_MANAGED_INSTALLATIONS_ENABLED:'true',MULTICA_DEPLOYMENT_ID:randomUUID(),MULTICA_PLATFORM_ADMIN_ENABLED:'false',DISABLE_WORKSPACE_CREATION:'false',
 MULTICA_APP_URL:web,MULTICA_PUBLIC_URL:api,MULTICA_DAEMON_SERVER_URL:api,LOCAL_UPLOAD_DIR:a.private_root+'/uploads',LOCAL_UPLOAD_BASE_URL:api+'/uploads',
 CORS_ALLOWED_ORIGINS:web+','+renderer,ALLOWED_ORIGINS:web+','+renderer,ANALYTICS_DISABLED:'true',MULTICA_MESSAGING_INTEGRATIONS_ENABLED:'false',MULTICA_VCS_INTEGRATION_ENABLED:'false',
 NEXT_PUBLIC_API_URL:api,NEXT_PUBLIC_WS_URL:api.replace('http:','ws:')+'/ws',REMOTE_API_URL:api,DOCS_URL:'',
 COLLABORATION_E2E_API_URL:api,PLAYWRIGHT_BASE_URL:web,E2E_PASSWORD_AUTH:'1',COLLABORATION_ELECTRON_RENDERER_URL:renderer,COLLABORATION_ELECTRON_SOURCE_ROOT:a.snapshot,
 COLLABORATION_HOST_DAEMON:a.private_root+'/bin/host-daemon',COLLABORATION_HOST_PROVIDER:a.private_root+'/bin/host-provider',COLLABORATION_HOST_CLI:a.private_root+'/bin/multica',
 MULTICA_DEV_HOME:a.private_root+'/registry',NEXT_TELEMETRY_DISABLED:'1'};
const envFile=a.private_root+'/environment.json';writeFileSync(envFile,JSON.stringify(env,null,2)+'\n',{mode:0o600});chmodSync(envFile,0o600);
a.env_file=envFile;a.deployment_id=env.MULTICA_DEPLOYMENT_ID;a.api_url=api;a.web_url=web;a.renderer_url=renderer;a.state='database_created';
writeFileSync(allocationFile,JSON.stringify(a,null,2)+'\n');console.log(JSON.stringify({database:a.database,database_verified:a.database_verified,capacity_before:a.capacity_before,private_root:a.private_root,api,web,renderer}));
