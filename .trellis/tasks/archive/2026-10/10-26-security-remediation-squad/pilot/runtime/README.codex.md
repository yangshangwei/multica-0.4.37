# Codex pilot container and fixed egress gateway

This is the historical real-Codex attempt. The user subsequently requested
scripted simulation; these execution containers and temporary credentials have
been removed. The CLI readiness probe succeeded, but genuine Multica SDK turns
remained rejected by the model service after native headers were preserved.
Do not restart real-model work as part of the current simulation continuation.
Use `../simulation/README.md` for the completed model-free workflow.

These are stage-one execution artifacts. They do not change product code, shared
agent bindings, the old OpenCode container, or any acceptance result. The main
session owns the live setup, credential transfer, daemon identity and pilot runs.

## Change boundary

The existing pilot has no Codex adapter and its default Docker bridge allows
arbitrary outbound connections. `Dockerfile.codex` and `codex-pilot` prepare the
existing daemon/provider contract in a disposable container; the nginx template
provides two fixed upstreams for an internal Docker network. Only the synthetic
laboratory, generated task directories and each task's session store become
writable by provider UID 1000. The model bearer stays in the gateway.

`server/internal/daemon/execenv/execenv.go` creates task homes at
`<envRoot>/codex-home` and task CLI configuration at `<envRoot>/multica-config`.
Local-directory session links point into
`/root/.codex/multica-sessions/<profile-namespace>/<agent>/<issue>`
(`codex_home.go`). The namespace is `default` for an empty profile, otherwise
`p_` plus the profile name's SHA-256. The daemon creates the task/home and
session-store parents with mode 0755; task temp and CLI config roots use 0700.
The image grants traversal of `/root` (0711) and the credential-free `.codex`
directory (0755); `/root/.multica` remains 0700. The wrapper grants only the
selected store and task trees to `node`, without following symlinks or changing
the store's parent ownership. It preserves the generated `CODEX_HOME`, task CLI
config pointer and app-server stdio. `runuser` sets the provider's `HOME` to
`/home/node` when switching to UID 1000.

The container is the host-file boundary. Roles still share UID 1000 and the
synthetic lab; this does not establish isolation between squad members.

## Build

Use a separate build context containing exactly `Dockerfile.codex`, `codex-pilot`
and a Linux arm64 `multica` binary built from the recorded source commit. Do not
copy any profile, Codex config or credential into the build context. The image
installs the task-selected CLI version `@openai/codex@0.164.0`.

```bash
docker build --platform linux/arm64 --file "$pilot_context/Dockerfile.codex" \
  --tag multica-security-codex-pilot:20261027 "$pilot_context"
```

Record the source commit, CLI version, image IDs and actual configuration hashes
when starting a run. A floating `node:22-bookworm` or nginx tag is not a promise
of a reproducible supply chain; keep the separate missing-attestation case open.

## Network topology

1. Create a task-specific Docker bridge with `--internal`.
2. Attach only the provider container and gateway to it. Attach the gateway to a
   separate ordinary bridge as well. Do not publish ports or mount host files.
3. Give the provider `--dns 127.0.0.1`; use the gateway's fixed internal IP in its
   API/provider URLs. Embedded Docker name lookup and host-gateway reachability
   must still be checked on the actual Docker installation.
4. Start the gateway with the private rendered config at `/etc/nginx/nginx.conf`.
   Disable its default config-generation entrypoint by using `--entrypoint nginx`
   after the private file is copied into the stopped container.
5. Verify denial of direct internet, host services, unknown gateway paths,
   unsupported model methods and redirects before launching the daemon.

The nginx template preserves incoming API `Authorization`, including task
tokens. It never substitutes the human daemon credential for task traffic.
`/api/` and `/health` terminate at `host.docker.internal:18572`. `/model/` permits
only model listing and Responses/compaction calls to
`https://agentrouter.org/v1`; it verifies TLS and replaces the incoming model
Authorization with the gateway-only bearer. The actual incoming `User-Agent`,
`originator` and `X-Codex-Beta-Features` values are preserved when present; the
gateway must not substitute or fabricate a native-client identity or feature
flag. Content-Type, Accept and OpenAI-Beta are the other forwarded model
headers; unrelated headers are discarded. All other paths, including bare
`/api` and `/model`, are denied. HTTP CONNECT is denied, upstream redirects
return 502, and bodies/headers are not access-logged. WebSocket upgrades are
supported on the fixed API upstream.

The main session should render only `__PILOT_MODEL_BEARER__` into a mode-0600 file
outside the repository, validating the value before nginx interpolation. Reject
newlines, NUL, quotes, backslashes and dollar signs. Copy it privately into the
gateway and remove the temporary host copy. Never use `nginx -T`, shell tracing,
Docker environment variables or command arguments to carry the real bearer.
Do not include the rendered config in evidence. The provider image contains
neither a model bearer nor a model auth file.

## Public Codex configuration

Prepare a minimal, credential-free Codex config in `/root/.codex/config.toml`
for the daemon's task-home seeding and in `/home/node/.codex/config.toml` for
standalone discovery. The source files remain root-owned. The gateway address
and the user-selected model are public settings, not credentials:

```toml
model = "<user-selected-model>"
model_reasoning_effort = "<user-selected-effort>"
model_provider = "pilot_gateway"

[model_providers.pilot_gateway]
name = "Pilot gateway"
base_url = "http://<gateway-internal-ip>:8080/model"
wire_api = "responses"
requires_openai_auth = false
```

Do not inject personal skills, plugins, auth.json, production credentials or an
entire host Codex configuration. The app-server gets its task skills and
instructions from the existing daemon preparation path.

## Daemon and verification

The main session supplies a private developer-only Multica profile under
`/root/.multica/profiles/<pilot-profile>/` (0700 directory, 0600 config), setting
the API URL to `http://<gateway-internal-ip>:8080`. Launch the daemon with
`MULTICA_CODEX_PATH=/usr/local/bin/codex-pilot`, `/work/tasks` as its workspace
root, one task at a time, a bounded timeout, and automatic update/reload disabled.
Use only the fixture copy at `/work/lab` as the project resource.

Use `--security-opt no-new-privileges`, a process/memory limit and no host mounts.
Remove `NET_RAW`; do not grant `NET_ADMIN` or use privileged/host-network mode.
Root still needs ordinary `CHOWN`, `SETUID` and `SETGID` capabilities for the
launch adapter. Verify with a synthetic root-private sentinel that the provider
cannot read the daemon profile, and preserve the actual task's injected role
briefing and identity as separate sanitized evidence.

`test_codex_runtime.py` runs only disposable Docker containers and synthetic
upstreams. It does not start Multica, invoke a model or read account credentials.
The provider uses an internal network; the synthetic upstream uses a separate
bridge reached through the gateway. Checks cover valid and invalid TLS names,
headers and route denial, external route/DNS denial, and both managed and
local-directory task permissions. Its results prove the adapter/gateway
contracts, not model availability or a successful squad route.

At teardown stop the daemon and gateway, verify no pilot task remains active,
remove temporary credential copies and retain redacted runtime/task evidence.
Preserve the old OpenCode history and all shared role contracts.
