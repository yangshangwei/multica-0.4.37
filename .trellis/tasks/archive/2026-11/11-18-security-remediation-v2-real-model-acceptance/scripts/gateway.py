#!/usr/bin/env python3
"""Exact-upstream gateway for an isolated trial; credentials never enter role homes."""

import argparse
import hashlib
import hmac
from http.client import HTTPConnection, HTTPSConnection
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import socket
import ssl
import threading
import time
import uuid
from urllib.parse import unquote, urlsplit

import runtime_guard as guard


MODEL_ROUTES = {("GET", "/model/models"), ("POST", "/model/responses"), ("POST", "/model/responses/compact")}
API_METHODS = {"GET", "POST", "PUT", "PATCH", "DELETE"}
MAX_BODY = 16 << 20
NATIVE_HEADERS = {"content-type", "accept", "openai-beta", "user-agent", "originator", "session_id", "conversation_id"}


def native_protocol_headers(headers):
    """Preserve the official client's own identity; never synthesize it."""
    return {key: value for key, value in headers.items()
            if key.lower() in NATIVE_HEADERS or key.lower().startswith("x-codex-")}


def load_config(path, expected_hash, registration):
    path = guard.private_path(path)
    if guard.digest(path) != expected_hash:
        raise guard.Refused("Gateway configuration changed")
    config = json.loads(path.read_text())
    if config.get("schema_version") != 1 or config.get("mode") != registration["mode"]:
        raise guard.Refused("Gateway mode mismatch")
    for name in ("model", "platform"):
        upstream = urlsplit(config[f"{name}_base"])
        if upstream.scheme not in {"http", "https"} or not upstream.hostname or upstream.username or upstream.password or upstream.query or upstream.fragment:
            raise guard.Refused("Invalid exact upstream")
        if name == "model" and config["mode"] == "real" and upstream.scheme != "https":
            # The user's existing provider is a local authenticated router.
            # This exception never admits cleartext remote model endpoints.
            if upstream.hostname not in {"127.0.0.1", "::1", "localhost"} or config.get("local_router_authorized") is not True:
                raise guard.Refused("Real model upstream requires TLS or the registered local router")
        if config["mode"] == "fake" and upstream.hostname not in {"127.0.0.1", "localhost", "fake-upstream"}:
            raise guard.Refused("Fake gateway permits only a declared fake endpoint")
    for key in ("proxy_token_file", "model_token_file"):
        token = guard.private_path(config[key]).read_text().strip()
        if not token or "\r" in token or "\n" in token:
            raise guard.Refused("Invalid credential file")
        config[key.removesuffix("_file")] = token
    config["model_headers"] = {}
    if config.get("model_headers_file"):
        headers_path = guard.private_path(config["model_headers_file"])
        headers = json.loads(headers_path.read_text())
        if not isinstance(headers, dict) or set(headers) - {"x-openai-actor-authorization"}:
            raise guard.Refused("Unregistered model header")
        if any(not isinstance(v, str) or "\r" in v or "\n" in v for v in headers.values()):
            raise guard.Refused("Invalid model header")
        config["model_headers"] = headers
    if config.get("mode") == "real":
        evidence = Path(config.get("evidence_root", ""))
        if not evidence.is_absolute() or evidence.resolve() != evidence or not evidence.is_dir() or evidence.stat().st_mode & 0o077:
            raise guard.Refused("Private model evidence directory required")
    return config


class Gateway(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, address, config, registration):
        self.config = config
        self.registration = registration
        super().__init__(address, Handler)


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, format, *args):
        pass

    def reject(self, status):
        self.send_response(status)
        self.send_header("Content-Length", "0")
        self.send_header("Connection", "close")
        self.end_headers()
        self.close_connection = True

    def do_CONNECT(self):
        self.reject(405)

    def do_OPTIONS(self):
        self.reject(405)

    def do_TRACE(self):
        self.reject(405)

    def do_HEAD(self):
        self.reject(405)

    def do_GET(self):
        self.forward()

    def do_POST(self):
        self.forward()

    def do_PUT(self):
        self.forward()

    def do_PATCH(self):
        self.forward()

    def do_DELETE(self):
        self.forward()

    def forward(self):
        parsed = urlsplit(self.path)
        decoded = unquote(parsed.path)
        if parsed.scheme or parsed.netloc or parsed.fragment or parsed.path != decoded or "\\" in decoded or any(p in {".", ".."} for p in decoded.split("/")) or "//" in decoded:
            self.reject(403)
            return
        lengths = self.headers.get_all("Content-Length", [])
        if self.headers.get("Transfer-Encoding") or len(lengths) > 1:
            self.reject(400)
            return
        try:
            size = int(lengths[0]) if lengths else 0
        except ValueError:
            self.reject(400)
            return
        if size < 0 or size > MAX_BODY:
            self.reject(413)
            return
        config = self.server.config
        model = (self.command, parsed.path) in MODEL_ROUTES and not parsed.query
        platform = (config.get("mode") != "real" and parsed.path.startswith("/api/")
                    and self.command in API_METHODS)
        if not model and not platform:
            self.reject(403)
            return
        authorization = self.headers.get("Authorization", "")
        if len(self.headers.get_all("Authorization", [])) != 1:
            self.reject(401)
            return
        if model and not hmac.compare_digest(authorization, "Bearer " + config["proxy_token"]):
            self.reject(401)
            return
        if platform and (not authorization.startswith("Bearer ") or authorization == "Bearer " + config["proxy_token"]):
            self.reject(401)
            return
        try:
            self.connection.settimeout(15)
            body = self.rfile.read(size) if size else None
            if body is not None and len(body) != size:
                self.reject(400)
                return
            if model and self.command == "POST":
                try:
                    payload = json.loads(body or b"")
                except (ValueError, UnicodeDecodeError):
                    self.reject(400)
                    return
                if not isinstance(payload, dict) or payload.get("model") != self.server.registration.get("model"):
                    self.reject(403)
                    return
            if model:
                timeout = guard.reserve_model_request(self.server.registration)
            else:
                with guard.ledger(self.server.registration) as ledger:
                    timeout = min(guard.remaining(self.server.registration, ledger), 120)
                    if ledger["stopped"] or timeout <= 0:
                        raise guard.Refused("Gateway expired")
            self.connection.settimeout(min(timeout, 15))
            upstream = urlsplit(config["model_base" if model else "platform_base"])
            path = upstream.path.rstrip("/") + (parsed.path.removeprefix("/model") if model else parsed.path)
            if parsed.query:
                path += "?" + parsed.query
            headers = native_protocol_headers(self.headers)
            headers.update({"Authorization": "Bearer " + config["model_token"] if model else authorization, "Connection": "close"})
            if model:
                headers.update(config.get("model_headers", {}))
            for field in (() if model else ("X-Workspace-ID", "Idempotency-Key")):
                if value := self.headers.get(field):
                    headers[field] = value
            connection = (HTTPSConnection(upstream.hostname, upstream.port, timeout=timeout, context=ssl.create_default_context())
                          if upstream.scheme == "https" else HTTPConnection(upstream.hostname, upstream.port, timeout=timeout))
            def expire():
                try:
                    if connection.sock is not None:
                        connection.sock.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass
                connection.close()
                try:
                    self.connection.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass

            watchdog = threading.Timer(timeout, expire)
            watchdog.daemon = True
            watchdog.start()
            evidence = None
            evidence_stream = None
            response_digest = hashlib.sha256()
            evidence_path = None
            if model and config.get("evidence_root"):
                evidence_path = Path(config["evidence_root"]) / (str(time.time_ns()) + "-" + uuid.uuid4().hex[:8])
                evidence_path.mkdir(mode=0o700)
                request_path = evidence_path / "request.json"
                request_path.write_bytes(body or b"")
                request_path.chmod(0o600)
                evidence = {"started_at": time.time(), "method": self.command, "path": parsed.path,
                            "model_requested": self.server.registration["model"],
                            "native_protocol_headers": native_protocol_headers(self.headers),
                            "request_sha256": hashlib.sha256(body or b"").hexdigest(), "complete": False}
                evidence_stream = (evidence_path / "response.stream").open("wb")
                (evidence_path / "response.stream").chmod(0o600)
            try:
                connection.request(self.command, path, body=body, headers=headers)
                response = connection.getresponse()
                if evidence is not None:
                    evidence.update(status=response.status, content_type=response.getheader("Content-Type"))
                if 300 <= response.status < 400:
                    self.reject(502)
                    return
                self.send_response(response.status)
                if content_type := response.getheader("Content-Type"):
                    self.send_header("Content-Type", content_type)
                self.send_header("Connection", "close")
                self.end_headers()
                self.close_connection = True
                while chunk := response.read1(65536):
                    response_digest.update(chunk)
                    if evidence_stream is not None:
                        evidence_stream.write(chunk)
                    self.wfile.write(chunk)
                    self.wfile.flush()
                if evidence is not None:
                    evidence["complete"] = True
            finally:
                watchdog.cancel()
                connection.close()
                if evidence_stream is not None:
                    evidence_stream.flush()
                    os.fsync(evidence_stream.fileno())
                    evidence_stream.close()
                if evidence is not None:
                    evidence.update(finished_at=time.time(), response_sha256=response_digest.hexdigest())
                    guard.atomic_save(evidence_path / "metadata.json", evidence)
        except guard.Refused:
            self.reject(429)
        except (OSError, ValueError, socket.timeout):
            if model:
                with guard.ledger(self.server.registration) as ledger:
                    ledger["stopped"] = True
                    ledger.setdefault("gateway_failures", []).append({"at": time.time(), "path": parsed.path,
                                                                      "kind": "incomplete_or_evidence_error"})
            self.close_connection = True


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--config-sha256", required=True)
    parser.add_argument("--registration", required=True)
    parser.add_argument("--registration-sha256", required=True)
    parser.add_argument("--bind", default="127.0.0.1")
    parser.add_argument("--port", required=True, type=int)
    args = parser.parse_args()
    registration = guard.load_registration(args.registration, args.registration_sha256)
    config = load_config(args.config, args.config_sha256, registration)
    Gateway((args.bind, args.port), config, registration).serve_forever()
