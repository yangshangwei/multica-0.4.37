"""Exercise disposable pilot containers against synthetic upstreams only."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import unittest
import uuid


RUNTIME = Path(__file__).resolve().parent
IMAGE = os.environ.get("PILOT_CODEX_TEST_IMAGE", "multica-security-codex-pilot:20261027")
UPSTREAM = r'''
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json, ssl, threading

class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    def log_message(self, *args):
        pass
    def do_GET(self):
        self.respond()
    def do_POST(self):
        self.respond()
    def respond(self):
        self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if "redirect" in self.path:
            self.send_response(302)
            self.send_header("Location", "https://denied.invalid/private")
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        if self.path == "/api/ws":
            self.send_response(101)
            self.send_header("Upgrade", "websocket")
            self.send_header("Connection", "Upgrade")
            self.end_headers()
            self.wfile.flush()
            if self.rfile.read(4) == b"ping":
                self.wfile.write(b"pong")
                self.wfile.flush()
            return
        data = json.dumps({"path": self.path, "method": self.command,
                           "headers": dict(self.headers)}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

plain = ThreadingHTTPServer(("0.0.0.0", 8000), Handler)
secure = ThreadingHTTPServer(("0.0.0.0", 8443), Handler)
context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
context.load_cert_chain("/tmp/cert.pem", "/tmp/key.pem")
secure.socket = context.wrap_socket(secure.socket, server_side=True)
threading.Thread(target=secure.serve_forever, daemon=True).start()
plain.serve_forever()
'''

CLIENT = r'''
import http.client, json, sys
p = json.load(sys.stdin)
c = http.client.HTTPConnection(p.pop("gateway"), 8080, timeout=5)
c.request(p["method"], p["path"], body=p.get("body"), headers=p.get("headers", {}))
r = c.getresponse()
print(json.dumps({"status": r.status, "headers": dict(r.getheaders()),
                  "body": r.read().decode()}))
c.close()
'''


def run(*args, input_text=None, check=True):
    return subprocess.run(
        args, input=input_text, text=True, capture_output=True, check=check, timeout=40
    )


class CodexPilotTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.prefix = "security-codex-test-" + uuid.uuid4().hex[:10]
        cls.network = cls.prefix + "-net"
        cls.egress = cls.prefix + "-egress"
        cls.gateway = cls.prefix + "-gateway"
        cls.bad_tls_gateway = cls.prefix + "-bad-tls"
        cls.upstream = cls.prefix + "-upstream"
        cls.provider = cls.prefix + "-provider"
        cls.containers = []
        cls.addClassCleanup(cls.cleanup)
        for image in (IMAGE, "nginx:stable-alpine", "python:3.13-slim"):
            run("docker", "image", "inspect", image)
        run("docker", "network", "create", "--internal", cls.network)
        run("docker", "network", "create", cls.egress)
        cls.create(cls.upstream, "python:3.13-slim", "sleep", "infinity",
                   network=cls.egress,
                   extra=["--network-alias", "pilot-upstream"])
        cls.create(cls.provider, IMAGE, "sleep", "infinity",
                   extra=["--dns", "127.0.0.1", "--dns-option", "timeout:1",
                          "--dns-option", "attempts:1"])
        run("docker", "start", cls.upstream, cls.provider)

        with tempfile.TemporaryDirectory(prefix=cls.prefix) as temp:
            root = Path(temp)
            run("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                "-days", "1", "-subj", "/CN=pilot-upstream",
                "-addext", "subjectAltName=DNS:pilot-upstream",
                "-keyout", str(root / "key.pem"), "-out", str(root / "cert.pem"))
            (root / "server.py").write_text(UPSTREAM)
            for name in ("key.pem", "cert.pem", "server.py"):
                run("docker", "cp", str(root / name), cls.upstream + ":/tmp/" + name)
            run("docker", "exec", "-d", cls.upstream, "python", "/tmp/server.py")
            config = (RUNTIME / "nginx-pilot.conf.template").read_text()
            config = config.replace("http://host.docker.internal:18572", "http://pilot-upstream:8000")
            config = config.replace("https://agentrouter.org/v1/", "https://pilot-upstream:8443/v1/")
            config = config.replace("agentrouter.org", "pilot-upstream")
            config = config.replace("/etc/ssl/certs/ca-certificates.crt", "/etc/nginx/test-ca.pem")
            config = config.replace("__PILOT_MODEL_BEARER__", "synthetic-model-bearer")
            for gateway, tls_name in ((cls.gateway, "pilot-upstream"),
                                      (cls.bad_tls_gateway, "wrong-name.invalid")):
                (root / "nginx.conf").write_text(config.replace(
                    "proxy_ssl_name pilot-upstream;", "proxy_ssl_name " + tls_name + ";"))
                cls.create(gateway, "nginx:stable-alpine", "-g", "daemon off;",
                           extra=["--entrypoint", "nginx"])
                run("docker", "network", "connect", cls.egress, gateway)
                run("docker", "cp", str(root / "nginx.conf"), gateway + ":/etc/nginx/nginx.conf")
                run("docker", "cp", str(root / "cert.pem"), gateway + ":/etc/nginx/test-ca.pem")
                run("docker", "start", gateway)
        deadline = time.monotonic() + 15
        while True:
            try:
                if all(cls.request("GET", "/health", gateway=gateway)["status"] == 200
                       for gateway in (cls.gateway, cls.bad_tls_gateway)):
                    break
            except (subprocess.CalledProcessError, json.JSONDecodeError):
                pass
            if time.monotonic() >= deadline:
                raise RuntimeError("synthetic gateway did not become ready")
            time.sleep(0.1)

    @classmethod
    def create(cls, name, image, *command, network=None, extra=()):
        run("docker", "create", "--pull", "never", "--name", name,
            "--network", network or cls.network, "--cap-drop", "NET_RAW",
            "--security-opt", "no-new-privileges", *extra, image, *command)
        cls.containers.append(name)

    @classmethod
    def cleanup(cls):
        for name in reversed(cls.containers):
            run("docker", "rm", "-f", name, check=False)
        run("docker", "network", "rm", cls.egress, check=False)
        run("docker", "network", "rm", cls.network, check=False)

    @classmethod
    def request(cls, method, path, headers=None, body=None, gateway=None):
        payload = {"gateway": gateway or cls.gateway, "method": method, "path": path,
                   "headers": headers or {}, "body": body}
        result = run("docker", "exec", "-i", cls.provider, "python3", "-c", CLIENT,
                     input_text=json.dumps(payload))
        return json.loads(result.stdout)

    def test_api_preserves_task_principal(self):
        response = self.request("POST", "/api/issues/synthetic/comments",
                                {"Authorization": "Bearer synthetic-task-token",
                                 "Host": "denied.invalid", "Content-Type": "application/json"}, "{}")
        self.assertEqual(response["status"], 200)
        echo = json.loads(response["body"])
        self.assertEqual(echo["path"], "/api/issues/synthetic/comments")
        self.assertEqual(echo["headers"]["Authorization"], "Bearer synthetic-task-token")
        self.assertEqual(echo["headers"]["Host"], "localhost:18572")
        without_auth = self.request("GET", "/api/issues/synthetic")
        self.assertEqual(without_auth["status"], 200)
        self.assertNotIn("authorization", {
            k.lower() for k in json.loads(without_auth["body"])["headers"]
        })

    def test_model_uses_only_gateway_bearer(self):
        native_user_agent = "synthetic-native-client/1.0"
        protocol_headers = {
            "originator": "multica-agent-sdk",
            "x-codex-beta-features": "remote_compaction_v2",
        }
        response = self.request("POST", "/model/responses",
                                {"Authorization": "Bearer synthetic-task-token",
                                 "X-Api-Key": "must-not-forward", "Content-Type": "application/json",
                                 "Cookie": "synthetic-cookie=must-not-forward",
                                 "Proxy-Authorization": "Bearer synthetic-proxy-token",
                                 "User-Agent": native_user_agent, **protocol_headers}, "{}")
        self.assertEqual(response["status"], 200)
        echo = json.loads(response["body"])
        self.assertEqual(echo["path"], "/v1/responses")
        self.assertEqual(echo["headers"]["Authorization"], "Bearer synthetic-model-bearer")
        self.assertEqual(echo["headers"]["User-Agent"], native_user_agent)
        echoed_headers = {k.lower(): v for k, v in echo["headers"].items()}
        for name, value in protocol_headers.items():
            self.assertEqual(echoed_headers.get(name), value)
        self.assertFalse({"x-api-key", "cookie", "proxy-authorization"} & echoed_headers.keys())
        without_protocol_headers = self.request("GET", "/model/models")
        self.assertEqual(without_protocol_headers["status"], 200)
        self.assertFalse(protocol_headers.keys() & {
            k.lower() for k in json.loads(without_protocol_headers["body"])["headers"]
        })
        self.assertEqual(self.request("POST", "/model/responses/compact", body="{}")["status"], 200)

    def test_model_tls_certificate_name_is_verified(self):
        response = self.request("GET", "/model/models", gateway=self.bad_tls_gateway)
        self.assertEqual(response["status"], 502)

    def test_unknown_routes_methods_and_redirects_are_denied(self):
        for method, path in (("GET", "/"), ("GET", "/api"), ("POST", "/api"),
                             ("GET", "/model"), ("POST", "/model"), ("GET", "/model/admin"),
                             ("GET", "/model/responses"), ("POST", "/model/models"),
                             ("DELETE", "/model/responses")):
            with self.subTest(method=method, path=path):
                self.assertEqual(self.request(method, path)["status"], 403)
        self.assertIn(self.request("CONNECT", "denied.invalid:443")["status"], (400, 405))
        for method, path in (("GET", "/api/redirect"), ("POST", "/model/responses?redirect=1")):
            with self.subTest(path=path):
                response = self.request(method, path)
                self.assertEqual(response["status"], 502)
                self.assertNotIn("location", {k.lower() for k in response["headers"]})

    def test_api_upgrade_tunnels_to_fixed_upstream(self):
        script = r'''
import http.client, sys
c = http.client.HTTPConnection(sys.argv[1], 8080, timeout=5)
c.request("GET", "/api/ws", headers={"Upgrade": "websocket", "Connection": "Upgrade"})
r = c.getresponse()
assert r.status == 101, r.status
c.sock.sendall(b"ping")
assert c.sock.recv(4) == b"pong"
c.close()
'''
        run("docker", "exec", self.provider, "python3", "-c", script, self.gateway)

    def test_provider_has_no_external_route_or_dns(self):
        upstream = json.loads(run("docker", "inspect", self.upstream).stdout)[0]
        upstream_ip = upstream["NetworkSettings"]["Networks"][self.egress]["IPAddress"]
        script = r'''
from pathlib import Path
import socket, sys
routes = Path("/proc/net/route").read_text().splitlines()[1:]
assert not any(row.split()[1] == "00000000" for row in routes), routes
for destination in (("1.1.1.1", 443), (sys.argv[1], 8000)):
    try:
        socket.create_connection(destination, timeout=1)
    except OSError:
        pass
    else:
        raise AssertionError("direct external TCP unexpectedly succeeded")
try:
    socket.getaddrinfo("example.com", 443)
except socket.gaierror:
    pass
else:
    raise AssertionError("external DNS unexpectedly succeeded")
'''
        run("docker", "exec", "--user", "node", self.provider, "python3", "-c", script, upstream_ip)

    def test_wrapper_grants_task_files_but_not_private_profile(self):
        script = r'''
import json, os, pathlib, subprocess
secret = pathlib.Path("/root/.multica/profiles/synthetic/config.json")
secret.parent.mkdir(parents=True, exist_ok=True)
os.chmod(secret.parent, 0o700)
secret.write_text("synthetic-private-sentinel")
os.chmod(secret, 0o600)
public_config = pathlib.Path("/root/.codex/config.toml")
public_config.write_text("# synthetic public config\n")
public_config.chmod(0o644)
exe = pathlib.Path("/usr/local/bin/codex")
exe.unlink()
exe.write_text("""#!/usr/bin/python3
import os,json,sys,pathlib
assert os.getuid()==1000
assert os.environ['HOME']=='/home/node'
assert os.environ['MULTICA_TOKEN']=='synthetic-task-token'
assert os.environ['TMPDIR']==os.environ['TMP']==os.environ['TEMP']
assert not os.access('/root/.multica/profiles/synthetic/config.json',os.R_OK)
assert not os.access('/root/.codex/config.toml',os.W_OK)
pathlib.Path(os.environ['CODEX_HOME'],'sessions','sentinel').write_text('ok')
pathlib.Path(os.environ['TMPDIR'],'sentinel').write_text('ok')
pathlib.Path('provider-write').write_text('ok')
assert pathlib.Path(os.environ['MULTICA_TASK_CONFIG_ROOT'],'config.json').read_text()=='synthetic-task-config'
sys.stderr.write('synthetic-diagnostic\\n')
print(json.dumps({'uid':os.getuid(),'args':sys.argv[1:],'input':sys.stdin.readline().strip()}))
""")
exe.chmod(0o755)
for mode in ('managed', 'local-directory'):
    root = pathlib.Path('/work/tasks/test-workspace/test-' + mode)
    cwd = pathlib.Path('/work/lab') if mode == 'local-directory' else root / 'workdir'
    codex = root / 'codex-home'
    tmp = pathlib.Path('/tmp/multica-task-' + mode)
    # Match the daemon: task/home/store parents 0755, temp/config roots 0700.
    for path in (cwd, codex, tmp):
        path.mkdir(parents=True, exist_ok=True)
    assert root.stat().st_uid == codex.stat().st_uid == 0
    assert root.stat().st_mode & 0o777 == 0o755
    assert codex.stat().st_mode & 0o777 == 0o755
    (root / '.task_lock').touch()
    task_config = root / 'multica-config'
    task_config.mkdir(mode=0o700)
    (task_config / 'config.json').write_text('synthetic-task-config')
    (task_config / 'config.json').chmod(0o600)
    tmp.chmod(0o700)
    if mode == 'local-directory':
        store = pathlib.Path('/root/.codex/multica-sessions/test-profile/test-agent/test-issue')
        store.mkdir(parents=True)
        for path in (store, store.parent, store.parent.parent):
            assert path.stat().st_uid == 0
            assert path.stat().st_mode & 0o777 == 0o755
        (codex / 'sessions').symlink_to(store)
    else:
        store = codex / 'sessions'
        store.mkdir(mode=0o755)
    (cwd / 'private-file-link').symlink_to(secret)
    (cwd / 'private-dir-link').symlink_to(secret.parent)
    env = dict(os.environ, CODEX_HOME=str(codex), TMPDIR=str(tmp), TMP=str(tmp), TEMP=str(tmp),
               MULTICA_TASK_CONFIG_ROOT=str(task_config), MULTICA_TOKEN='synthetic-task-token')
    p = subprocess.run(['/usr/local/bin/codex-pilot','app-server','--listen','stdio://'],
                       cwd=cwd, env=env, input='synthetic-rpc\n',
                       text=True, capture_output=True, check=True)
    result = json.loads(p.stdout)
    assert result == {'uid':1000,'args':['app-server','--listen','stdio://'],'input':'synthetic-rpc'}, result
    assert p.stderr == 'synthetic-diagnostic\n', p.stderr
    assert (store / 'sentinel').read_text() == 'ok'
    assert task_config.stat().st_mode & 0o777 == 0o700
    assert tmp.stat().st_mode & 0o777 == 0o700
    if mode == 'local-directory':
        assert store.parent.stat().st_uid == store.parent.parent.stat().st_uid == 0

    for key in ('CODEX_HOME', 'TMPDIR'):
        bad_env = dict(env, **{key:'/root/.multica'})
        bad = subprocess.run(['/usr/local/bin/codex-pilot','app-server'], cwd=cwd,
                             env=bad_env, capture_output=True)
        assert bad.returncode != 0 and not bad.stdout
    bad = subprocess.run(['/usr/local/bin/codex-pilot','app-server'], cwd='/tmp',
                         env=env, capture_output=True)
    assert bad.returncode != 0 and not bad.stdout
    (root / '.task_lock').unlink()
    bad = subprocess.run(['/usr/local/bin/codex-pilot','app-server'], cwd=cwd,
                         env=env, capture_output=True)
    assert bad.returncode != 0 and not bad.stdout
    (root / '.task_lock').touch()
    if mode == 'local-directory':
        (codex / 'sessions').unlink()
        (codex / 'sessions').symlink_to(secret.parent)
        bad = subprocess.run(['/usr/local/bin/codex-pilot','app-server'], cwd=cwd,
                             env=env, capture_output=True)
        assert bad.returncode != 0 and not bad.stdout
    assert secret.stat().st_uid == secret.parent.stat().st_uid == 0
    assert secret.stat().st_mode & 0o777 == 0o600
    assert secret.parent.stat().st_mode & 0o777 == 0o700
    assert public_config.stat().st_uid == 0
    assert public_config.read_text() == '# synthetic public config\n'
'''
        run("docker", "exec", self.provider, "python3", "-c", script)


if __name__ == "__main__":
    unittest.main(verbosity=2)
