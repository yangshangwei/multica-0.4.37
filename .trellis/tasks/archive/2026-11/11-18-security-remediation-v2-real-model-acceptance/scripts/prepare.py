#!/usr/bin/env python3
"""No-model preparation: validate synthetic sources and probe disposable containers.

This deliberately has no real execution mode, provider discovery or credential
handling. Its network-none probes do not prove the later API/model allowlist.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import uuid


TASK = Path(__file__).resolve().parents[1]
FIXTURES = TASK / "fixtures"
IMAGE = "sha256:9d2e5553305c7c7b0097999bb17187c69b921ccd6bc9d40e4bb5ebe652c00285"


def command(argv, **kwargs):
    return subprocess.run(argv, text=True, capture_output=True, timeout=20, **kwargs)


def inventory():
    paths = [FIXTURES / "expected-actions.json"] + sorted(
        p for p in (FIXTURES / "repos").rglob("*") if p.is_file()
    )
    return {
        str(p.relative_to(TASK)): hashlib.sha256(p.read_bytes()).hexdigest()
        for p in paths
    }


def validate_sources():
    frozen = json.loads((FIXTURES / "freeze.sha256.json").read_text())
    if inventory() != frozen:
        raise RuntimeError("Frozen fixture inventory differs")
    expectations = json.loads((FIXTURES / "expected-actions.json").read_text())
    if len(expectations["cases"]) != 4 or expectations["real_model_calls"] != 0:
        raise RuntimeError("Invalid scenario inventory or pretrial claim")
    dependency = FIXTURES / "repos/dependency"
    advisory = json.loads((dependency / "advisory.json").read_text())
    lock = json.loads((dependency / "dependency.lock.json").read_text())
    for version, release in advisory["releases"].items():
        source = dependency / release["source"]
        if source.parent != dependency / "vendor":
            raise RuntimeError("Dependency path escapes vendor directory")
        if hashlib.sha256(source.read_bytes()).hexdigest() != release["sha256"]:
            raise RuntimeError("Vendored source digest differs")
        if version == lock["version"]:
            if any(lock[k] != release[k] for k in ("source", "sha256")):
                raise RuntimeError("Lockfile and advisory differ")
    missing = json.loads((FIXTURES / "repos/other/supply-chain.json").read_text())
    if any(missing[k] is not None for k in ("artifact_sha256", "provenance", "sbom")):
        raise RuntimeError("Missing evidence was filled in")
    return {"files": len(frozen), "hashes_match": True, "real_model_calls": 0}


def docker_args():
    return [
        "docker", "run", "--rm", "--pull=never", "--network=none",
        "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges",
        "--user=65534:65534", "--pids-limit=64", "--memory=128m", "--cpus=1",
        "--tmpfs=/tmp:rw,noexec,nosuid,size=16m", "-e", "PYTHONDONTWRITEBYTECODE=1",
    ]


def run_container(argv):
    name = "secrem-v2-offline-" + uuid.uuid4().hex[:12]
    argv[2:2] = ["--name", name, "--label", "task=security-remediation-v2-offline"]
    try:
        return command(argv)
    except subprocess.TimeoutExpired as error:
        try:
            cleanup = command(["docker", "rm", "-f", name])
            cleaned = cleanup.returncode == 0
        except subprocess.TimeoutExpired:
            cleaned = False
        raise RuntimeError(f"Owned container timed out: {name}; cleanup_confirmed={cleaned}") from error


def local_baseline_tests():
    results = {}
    for case, count, exit_code in [("code", 2, 0), ("dependency", 3, 1)]:
        run = command(
            [sys.executable, "-B", "-m", "unittest", "discover", "-s", "tests", "-v"],
            cwd=FIXTURES / "repos" / case,
            env={"PATH": os.defpath, "PYTHONDONTWRITEBYTECODE": "1"},
        )
        output = run.stdout + run.stderr
        if run.returncode != exit_code or f"Ran {count} tests" not in output:
            raise RuntimeError(f"Unexpected {case} local baseline: {output}")
        if case == "dependency" and "FAILED (failures=1)" not in output:
            raise RuntimeError("Dependency baseline must have one expected failure")
        results[case] = {"tests": count, "exit": run.returncode, "output": output}
    return {"results": results, "isolation_proved": False, "scope": "fixture validity only; no OS network boundary"}


def container_probe():
    image = command(["docker", "image", "inspect", IMAGE, "--format", "{{.Id}} {{.Os}} {{.Architecture}}"])
    if image.returncode != 0:
        raise RuntimeError("Pinned local Python image unavailable; no pull attempted")
    baseline = {}
    for scenario, expected_count, expected_exit in [("code", 2, 0), ("dependency", 3, 1)]:
        argv = docker_args() + [
            "--mount", f"type=bind,src={FIXTURES / 'repos' / scenario},dst=/fixture,readonly",
            "--workdir=/fixture", IMAGE, "python3", "-m", "unittest", "discover", "-s", "tests", "-v",
        ]
        run = run_container(argv)
        output = run.stdout + run.stderr
        if run.returncode != expected_exit or f"Ran {expected_count} tests" not in output:
            raise RuntimeError(f"Unexpected {scenario} fixture result: {output}")
        if scenario == "dependency" and "FAILED (failures=1)" not in output:
            raise RuntimeError("Dependency baseline must have exactly one intended failure")
        baseline[scenario] = {
            "tests": expected_count, "exit": run.returncode,
            "expected_failure": scenario == "dependency", "output": output,
        }

    roles = ["leader", "verifier", "implementer", "qa", "security-reviewer", "code-reviewer", "migration-reviewer"]
    probe = """import json,pathlib,socket
p=pathlib.Path('/work')
assert (p/'own-sentinel').read_text() == 'synthetic-role-only'
assert not pathlib.Path('/other-role/other-sentinel').exists()
assert not pathlib.Path('/var/run/docker.sock').exists()
try:
    assert not pathlib.Path('/root/.codex/auth.json').exists()
except PermissionError:
    pass
routes=pathlib.Path('/proc/net/route').read_text().strip().splitlines()
assert len(routes)==1, routes
s=socket.socket(); s.settimeout(0.3)
try:
    s.connect(('198.51.100.1',443))
except OSError:
    pass
else:
    raise AssertionError('unexpected external reachability')
finally:
    s.close()
(p/'own-write').write_text('synthetic')
print(json.dumps({'own_read_write':True,'peer_unmounted':True,'external_route_absent':True,'external_connect_denied':True,'docker_socket_absent':True}))
"""
    probes = {}
    with tempfile.TemporaryDirectory(prefix="isolation-", dir=TASK / "research") as temp:
        root = Path(temp)
        for role in roles:
            own = root / role
            own.mkdir(mode=0o777)
            own.chmod(0o777)
            (own / "own-sentinel").write_text("synthetic-role-only")
        for role in roles:
            run = run_container(docker_args() + [
                "--mount", f"type=bind,src={root / role},dst=/work",
                IMAGE, "python3", "-c", probe,
            ])
            if run.returncode != 0:
                raise RuntimeError(f"Isolation probe failed for {role}: {run.stderr}")
            probes[role] = json.loads(run.stdout)
    return {
        "image": image.stdout.strip(), "baselines": baseline, "roles": probes,
        "scope": "offline disposable container probes only; production gateway and daemon not started",
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--container-probe", action="store_true")
    parser.add_argument("--local-tests", action="store_true")
    args = parser.parse_args()
    result = {"evidence_type": "fixture-and-environment-preflight", "sources": validate_sources()}
    if args.local_tests:
        result["local_baselines"] = local_baseline_tests()
    if args.container_probe:
        result["containers"] = container_probe()
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
