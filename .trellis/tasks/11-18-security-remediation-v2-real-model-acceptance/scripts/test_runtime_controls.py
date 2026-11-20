"""Credential-free controls. All executables and HTTP peers are test-created."""

from concurrent.futures import ThreadPoolExecutor
import hashlib
from http.client import HTTPConnection
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

import gateway
import runtime_guard as guard


class Controls(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.root.chmod(0o700)
        self.executable = self.root / "fake-agent"
        self.executable.write_text("#!/bin/sh\nexit 0\n")
        self.executable.chmod(0o700)
        self.registration = {
            "schema_version": 1, "mode": "fake", "enabled": False,
            "model": "synthetic-model",
            "max_starts": 2, "max_model_requests": 3,
            "max_seconds": 30, "per_start_seconds": 10, "expires_at": time.time() + 60,
            "ledger": str(self.root / "ledger.json"), "argv": [str(self.executable)],
            "executable_sha256": guard.digest(self.executable),
            "cwd": str(self.root), "source_hashes": {}, "environment": {"PATH": "/usr/bin:/bin"},
        }

    def registered(self):
        path = self.root / "registration.json"
        path.write_text(json.dumps(self.registration))
        path.chmod(0o600)
        return guard.load_registration(str(path), guard.digest(path))

    def test_reservation_cap_and_ambiguous_predecessor(self):
        registration = self.registered()
        sequence, _ = guard.reserve_start(registration)
        self.assertEqual(sequence, 1)
        with self.assertRaises(guard.Refused):
            guard.reserve_start(registration)
        with guard.ledger(registration) as ledger:
            ledger["starts"][0]["finished_at"] = time.time()
        self.assertEqual(guard.reserve_start(registration)[0], 2)
        with guard.ledger(registration) as ledger:
            ledger["starts"][1]["finished_at"] = time.time()
        with self.assertRaises(guard.Refused):
            guard.reserve_start(registration)

    def test_concurrent_reservations_admit_one(self):
        registration = self.registered()

        def attempt(_):
            try:
                guard.reserve_start(registration)
                return True
            except guard.Refused:
                return False

        with ThreadPoolExecutor(max_workers=4) as pool:
            self.assertEqual(sum(pool.map(attempt, range(4))), 1)

    def test_failed_launch_debits_and_stops(self):
        self.executable.write_text("#!/bin/sh\nexit 7\n")
        self.registration["executable_sha256"] = guard.digest(self.executable)
        registration = self.registered()
        self.assertEqual(guard.execute(registration), 7)
        with guard.ledger(registration) as ledger:
            self.assertTrue(ledger["stopped"])
            self.assertEqual(len(ledger["starts"]), 1)
            self.assertEqual(ledger["starts"][0]["outcome"], "failed")
        with self.assertRaises(guard.Refused):
            guard.reserve_start(registration)

    def test_timeout_is_recorded_and_stops(self):
        self.executable.write_text("#!/bin/sh\nsleep 10\n")
        self.registration["executable_sha256"] = guard.digest(self.executable)
        self.registration["per_start_seconds"] = 0.15
        registration = self.registered()
        self.assertEqual(guard.execute(registration), 124)
        with guard.ledger(registration) as ledger:
            self.assertEqual(ledger["starts"][0]["outcome"], "timed_out")
            self.assertTrue(ledger["stopped"])

    def test_expiry_hash_and_disabled_real_gate(self):
        self.registration["expires_at"] = time.time() - 1
        with self.assertRaises(guard.Refused):
            self.registered()
        self.registration["expires_at"] = time.time() + 60
        self.registration["mode"] = "real"
        with self.assertRaises(guard.Refused):
            self.registered()
        self.registration["mode"] = "fake"
        registration = self.registered()
        self.executable.write_text("#!/bin/sh\nexit 9\n")
        with self.assertRaises(guard.Refused):
            self.registered()
        self.assertFalse(Path(registration["ledger"]).exists())

    def test_request_time_and_no_active_process_are_refused(self):
        registration = self.registered()
        with self.assertRaises(guard.Refused):
            guard.reserve_model_request(registration)
        guard.reserve_start(registration)
        with guard.ledger(registration) as ledger:
            ledger["starts"][0]["started_at"] = time.time() - 11
        with self.assertRaises(guard.Refused):
            guard.reserve_model_request(registration)
        with guard.ledger(registration) as ledger:
            self.assertEqual(ledger["model_requests"], [])

    def test_apparently_complete_real_registration_is_still_disabled(self):
        self.registration.update(
            mode="real", enabled=True, account_alias="synthetic-account",
            integration_commit="f" * 40, isolation_evidence_sha256="e" * 64,
            source_hashes={str(self.executable): guard.digest(self.executable)},
        )
        with patch.dict(os.environ, {"MULTICA_RUN_REAL_AGENT_SMOKE": "1"}):
            with self.assertRaisesRegex(guard.Refused, "Preparation build"):
                self.registered()
        self.assertFalse(Path(self.registration["ledger"]).exists())

    def test_private_ledger_rejects_another_registration(self):
        registration = self.registered()
        guard.reserve_start(registration)
        self.registration["max_starts"] = 3
        changed = self.registered()
        with self.assertRaises(guard.Refused):
            guard.reserve_start(changed)

    def test_gateway_allowlist_auth_identity_redirect_and_request_cap(self):
        observed = []

        class Upstream(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                self.respond()

            def do_POST(self):
                self.respond()

            def respond(self):
                observed.append((self.command, self.path, self.headers.get("Authorization"), self.headers.get("X-Workspace-ID")))
                if self.path.endswith("/compact"):
                    self.send_response(302)
                    self.send_header("Location", "http://unapproved.invalid/")
                    self.end_headers()
                    return
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(b'{"synthetic":true}')

        upstream = ThreadingHTTPServer(("127.0.0.1", 0), Upstream)
        threading.Thread(target=upstream.serve_forever, daemon=True).start()
        self.addCleanup(upstream.server_close)
        self.addCleanup(upstream.shutdown)
        registration = self.registered()
        guard.reserve_start(registration)
        with guard.ledger(registration) as ledger:
            ledger["starts"][0]["started_at"] = time.time()
        config = {
            "model_base": f"http://127.0.0.1:{upstream.server_port}/v1",
            "platform_base": f"http://127.0.0.1:{upstream.server_port}",
            "proxy_token": "synthetic-role-token", "model_token": "synthetic-gateway-token",
        }
        proxy = gateway.Gateway(("127.0.0.1", 0), config, registration)
        threading.Thread(target=proxy.serve_forever, daemon=True).start()
        self.addCleanup(proxy.server_close)
        self.addCleanup(proxy.shutdown)

        def request(method, path, token="synthetic-role-token", model="synthetic-model"):
            client = HTTPConnection("127.0.0.1", proxy.server_port, timeout=3)
            body = json.dumps({"model": model}) if method == "POST" else None
            client.request(method, path, body=body, headers={"Authorization": "Bearer " + token, "X-Workspace-ID": "synthetic-workspace"})
            response = client.getresponse()
            response.read()
            code = response.status
            client.close()
            return code

        for method, path in [("CONNECT", "unapproved.invalid:443"), ("GET", "http://unapproved.invalid/"), ("GET", "/model/../api/secret"), ("GET", "/model/%2e%2e/api/secret"), ("GET", "/model/models?redirect=x"), ("POST", "/model/arbitrary"), ("GET", "/health")]:
            self.assertIn(request(method, path), {403, 405})
        self.assertEqual(observed, [])
        self.assertEqual(request("POST", "/model/responses", "wrong"), 401)
        self.assertEqual(request("POST", "/model/responses", model="unregistered-model"), 403)
        self.assertEqual(request("GET", "/model/models"), 200)
        self.assertEqual(request("POST", "/model/responses"), 200)
        self.assertEqual(request("POST", "/model/responses/compact"), 502)
        self.assertEqual(request("POST", "/model/responses"), 429)
        self.assertEqual(len(observed), 3)
        self.assertTrue(all(row[2] == "Bearer synthetic-gateway-token" for row in observed))
        self.assertEqual(request("GET", "/api/issues?limit=1", "synthetic-task-token"), 200)
        self.assertEqual(observed[-1][2:], ("Bearer synthetic-task-token", "synthetic-workspace"))
        self.assertEqual(request("GET", "/api/issues"), 401)
        with guard.ledger(registration) as ledger:
            self.assertEqual(len(ledger["model_requests"]), 3)
            ledger["stopped"] = True
        self.assertEqual(request("GET", "/api/issues", "synthetic-task-token"), 429)


if __name__ == "__main__":
    unittest.main()
