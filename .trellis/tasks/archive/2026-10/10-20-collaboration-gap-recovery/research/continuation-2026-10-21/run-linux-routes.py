"""Run current test-built providers and supervisors in an isolated Linux container."""

import datetime
import json
import os
import pathlib
import subprocess
import urllib.parse


directory = pathlib.Path(__file__).resolve().parent
environment = os.environ.copy()
url = urllib.parse.urlsplit(environment["DATABASE_URL"])
credentials = url.netloc.rsplit("@", 1)[0]
environment["DATABASE_URL"] = urllib.parse.urlunsplit(url._replace(netloc=f"{credentials}@192.168.65.254:{url.port}"))
pattern = "^TestCollaboration(SingleAgentRuntimeRoutes|HeldSquadAllowsDirectAgentRoutes|RuntimeProofSurvivesOwnerDisable|SquadCapacityOneRoutes|PartialResumeRoutes|SquadCLIAndDaemonCapacityOneRoutes)Linux$"
container_name = "multica-gap-continuation-20261021-routes"
command = [
    "docker", "create", "--rm", "--name", container_name,
    "--privileged", "--security-opt", "seccomp=unconfined", "--security-opt", "apparmor=unconfined",
    "--env", "DATABASE_URL", "--env", "MULTICA_RUN_COLLAB_RUNTIME_E2E=1",
    "--env", "MULTICA_TEST_COLLAB_CLI=/multica-cli",
    "--env", "MULTICA_TEST_COLLAB_DAEMON=/daemon.test",
    "--entrypoint", "/router.test", "redis:7-alpine",
    "-test.run", pattern, "-test.v", "-test.count=1", "-test.parallel=1", "-test.timeout=5m",
]
summary = {"command": command, "database_alias": "service_s6_linux_routes", "os_arch": "linux/arm64",
           "binary_transport": "docker create, copy explicit test-built binaries, start; no host mounts",
           "started_at": datetime.datetime.now(datetime.timezone.utc).isoformat()}
subprocess.run(command, env=environment, check=True, stdout=subprocess.DEVNULL)
for name in ["router.test", "daemon.test", "multica-cli"]:
    subprocess.run(["docker", "cp", f"/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/{name}",
                    f"{container_name}:/{name}"], check=True, stdout=subprocess.DEVNULL)
with (directory / "linux-router.log").open("w") as output:
    result = subprocess.run(["docker", "start", "--attach", container_name], env=environment,
                            stdout=output, stderr=subprocess.STDOUT)
summary.update(exit_code=result.returncode, finished_at=datetime.datetime.now(datetime.timezone.utc).isoformat())
(directory / "linux-router-command.json").write_text(json.dumps(summary, indent=2) + "\n")
with (directory / "linux-router.log").open() as source, (directory / "linux-router.jsonl").open("w") as target:
    subprocess.run(["go", "tool", "test2json", "-p", "github.com/multica-ai/multica/server/cmd/server", "-t"],
                   stdin=source, stdout=target, check=True)
print(json.dumps({key: value for key, value in summary.items() if key != "command"}))
raise SystemExit(result.returncode)
