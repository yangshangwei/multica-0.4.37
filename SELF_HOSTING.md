# Self-Hosting Guide

Deploy Multica on your own infrastructure in minutes.

## Architecture

| Component | Description | Technology |
|-----------|-------------|------------|
| **Backend** | REST API + WebSocket server | Go (single binary) |
| **Frontend** | Web application | Next.js 16 |
| **Database** | Primary data store | PostgreSQL 17 (`pgcrypto` + `pg_trgm`) |

Each user who runs AI agents locally also installs the **`multica` CLI** and runs the **agent daemon** on their own machine.

## Quick Install (Recommended)

Two commands to set up everything — server, CLI, and configuration.

<details open>
<summary><b>macOS / Linux</b></summary>

<br/>

```bash
# 1. Install CLI + provision the self-host server
curl -fsSL https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.sh | bash -s -- --with-server

# 2. Configure CLI, authenticate, and start the daemon
multica setup self-host
```
</details>
<details>
<summary><b>Windows (PowerShell)</b></summary>

<br/>

```powershell
# 1. Install CLI + provision the self-host server
$env:MULTICA_MODE="with-server"; irm https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.ps1 | iex

# 2. Configure CLI, authenticate, and start the daemon
multica setup self-host
```
</details>

This installs the `multica` CLI, checks out the latest self-host assets, pulls the official Multica images from GHCR, and configures everything for localhost.

Open http://localhost:3000. To log in, configure `RESEND_API_KEY` in `.env` for email-based codes (recommended), or leave Resend unset and copy the generated code from the backend logs. See [Step 2 — Log In](#step-2--log-in) for details.

> **Prerequisites:** Docker and Docker Compose must be installed. The script checks for this and provides install links if missing.
>
> **CLI only?** If the self-host server is already running and you only need the CLI on a macOS/Linux machine, install it with Homebrew:
>
> ```bash
> brew install multica-ai/tap/multica
> ```

---

## Step-by-Step Setup (Alternative)

If you prefer to run each step manually:

### Step 1 — Start the Server

**Prerequisites:** Docker and Docker Compose.

```bash
git clone https://github.com/multica-ai/multica.git
cd multica
make selfhost
```

`make selfhost` automatically creates `.env` from the example, generates a random `JWT_SECRET`, and starts all services via Docker Compose.

By default it pulls the latest stable release images from GHCR. To build the backend/web from your current checkout instead, run `make selfhost-build`.
If the selected GHCR tag has not been published yet, `make selfhost` now tells you to fall back to `make selfhost-build`.
`make selfhost-build` uses local `multica-backend:dev` / `multica-web:dev` tags, so it does not overwrite the pulled `:latest` images.

Once ready:

- **Frontend:** http://localhost:3000
- **Backend API:** http://localhost:8080

> **Note:** If you prefer to run the Docker Compose steps manually, see [Manual Docker Compose Setup](#manual-docker-compose-setup) below.

### Step 2 — Log In

Open http://localhost:3000 in your browser. The Docker self-host stack defaults to `APP_ENV=production` (set in `docker-compose.selfhost.yml`), and there is no fixed verification code by default. Pick one of the following to log in:

- **Recommended (production):** configure `RESEND_API_KEY` in `.env`, then restart the backend. Real verification codes will be sent to the email address you enter. See [Advanced Configuration → Email](SELF_HOSTING_ADVANCED.md#email-required-for-authentication).
- **Without email configured:** the verification code is generated server-side and printed to the backend container logs (look for `[DEV] Verification code for ...:`). Useful for one-off testing on a single machine.
- **Deterministic local/private testing:** set `APP_ENV=development` and `MULTICA_DEV_VERIFICATION_CODE=888888` in `.env`, then restart the backend. This fixed code is ignored when `APP_ENV=production`.

- **Intranet with no mail relay and no reachable OAuth:** none of the above may be possible. See [Intranet Mode — No Login](#intranet-mode--no-login), which lets the desktop app open straight into a shared workspace — at the cost of removing authentication for it.

Changes to `ALLOW_SIGNUP`, `DISABLE_WORKSPACE_CREATION`, and `GOOGLE_CLIENT_ID` also take effect after restarting the backend / compose stack. The web UI reads all three from `/api/config` at runtime, so no web rebuild is needed. See [Advanced Configuration → Signup Controls](SELF_HOSTING_ADVANCED.md#signup-controls-optional) for the recommended sequence to lock down workspace creation.

> **Warning:** do **not** set `MULTICA_DEV_VERIFICATION_CODE` on a publicly reachable instance — anyone who knows an email address can then log in with that fixed code.

### Step 3 — Install CLI & Start Daemon

The daemon runs on your local machine (not inside Docker). It detects installed AI agent CLIs, registers them with the server, and executes tasks when agents are assigned work.

Each team member who wants to run AI agents locally needs to:

### a) Install the CLI and an AI agent

```bash
brew install multica-ai/tap/multica
```

You also need at least one AI agent CLI installed:
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (`claude` on PATH)
- [Antigravity CLI](https://antigravity.google/docs/cli-install) (`agy` on PATH)
- [CodeBuddy Code](https://www.codebuddy.ai/docs/cli/quickstart) (`codebuddy` on PATH)
- [DevEco Code](https://gitcode.com/openharmony-sig/deveco-code) (`deveco` on PATH)
- [Codex](https://github.com/openai/codex) (`codex` on PATH)
- [GitHub Copilot CLI](https://docs.github.com/en/copilot) (`copilot` on PATH)
- [OpenClaw](https://github.com/openclaw/openclaw) (`openclaw` on PATH)
- [OpenCode](https://github.com/anomalyco/opencode) (`opencode` on PATH)
- [Huawei Cloud CodeArts](https://support.huaweicloud.com/qs-codeartssnap/codeartsagent_qs_0004.html) (`codearts` on PATH)
- [Hermes](https://github.com/NousResearch/hermes) (`hermes` on PATH)
- [Pi](https://pi.dev/) (`pi` on PATH)
- [Cursor Agent](https://cursor.com/) (`cursor-agent` on PATH)
- Kimi (`kimi` on PATH)
- [Reasonix](https://github.com/esengine/DeepSeek-Reasonix) (`reasonix` on PATH; run `reasonix setup` first)
- Dim (`dim` on PATH)
- Kiro CLI (`kiro-cli` on PATH)
- Qoder CLI (`qodercli` on PATH)
- Qoder CN CLI (`qoderclicn` on PATH)
- Trae CLI (`traecli` on PATH)
- [Grok Build CLI](https://docs.x.ai/) (`grok` on PATH)
- Qwen Code (`qwen` on PATH)
- [QwenPaw](https://github.com/agentscope-ai/QwenPaw) (`qwenpaw` on PATH; pick its model in QwenPaw's own configuration)
- [MiniMax Code CLI](https://www.npmjs.com/package/@minimax-ai/code) (`mcode` 0.1.2+ on PATH). Install a supported Node.js release (`>=22.19 <23` or `>=24 <27`), run `npm install --global @minimax-ai/code@latest`, then `mcode login`.
- [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) (`dsh` on PATH with the Multica runtime profile installed; set `DEEPSEEK_API_KEY`)

### b) One-command setup

```bash
multica setup self-host
```

This automatically:
1. Configures the CLI to connect to `localhost` (ports 8080/3000)
2. Opens your browser for authentication
3. Discovers your workspaces
4. Starts the daemon in the background

For on-premise deployments with custom domains:

```bash
multica setup self-host --server-url https://api.example.com --app-url https://app.example.com
```

To verify the daemon is running:

```bash
multica daemon status
```

> **Alternative:** If you prefer manual steps, see [Manual CLI Configuration](#manual-cli-configuration) below.

### Step 4 — Verify & Start Using

1. Open your workspace in the web app at http://localhost:3000
2. Navigate to **Settings → Runtimes** — you should see your machine listed
3. Go to **Settings → Agents** and create a new agent
4. Create an issue and assign it to your agent — it will pick up the task automatically

---

## Intranet Mode — No Login

On an isolated intranet, every login flow above may be unavailable: no mail relay to deliver a verification code, no reachable Google OAuth. Device auth closes that gap by trading a per-client identifier for a session, so the desktop app and the web app both boot straight into a shared workspace with nothing to type.

**This is on by default for a self-hosted deployment.** It is off only when the server is serving `multica.ai`. You do not have to set anything to get no-login startup; you have to set something to switch it off.

> **This means the deployment has no authentication.** Anyone who can reach the backend port can mint an identity, read and write every issue in the shared workspace, and queue agent tasks that spend your runtime and model quota. `ALLOW_SIGNUP` does **not** gate it — that flag governs human signup and says nothing about this path. The only thing protecting the deployment is the network in front of it. Keep the backend on a trusted network, or set `MULTICA_DEVICE_AUTH_ENABLED=false` and use the login flows above.

| Variable | Default | Description |
|----------|---------|-------------|
| `MULTICA_DEVICE_AUTH_ENABLED` | on, except on `multica.ai` | Master switch over `POST /auth/device` and the capability advertised through `/api/config`. Leave it empty for the default; `false`, `0`, `no` and `off` all require a login, `true`, `1`, `yes` and `on` all force it on. |
| `MULTICA_DEVICE_AUTH_WORKSPACE` | `intranet` | Slug of the one shared workspace every device joins. Created on first use; an existing workspace with this slug is reused and never modified. A reserved or malformed slug falls back to the default. |
| `MULTICA_DEVICE_AUTH_WORKSPACE_NAME` | `Intranet` | Display name used only when that workspace is created. |
| `MULTICA_DEVICE_AUTH_ROLE` | `member` | Role for every device after the first (`admin` or `member`). The device whose first boot created the workspace becomes its `owner`. |

How it works:

1. Each client generates a random id the first time it runs. The desktop app stores it in its user-data directory as `device-identity.json` with a display name of the form `user@hostname`; a browser stores it in `localStorage` under `multica_device_id` and appears as `web-<os>-<id prefix>`, since it can read neither a username nor a hostname.
2. When a client has no working session it reads `/api/config`. If the deployment declares device auth it posts the id to `/auth/device` and gets a session; if not, it shows the normal login page. The desktop app checks before rendering anything; the web app finds out when its session cookie comes back rejected, which is the only signal it has.
3. The backend maps that id to a user — creating one the first time it sees it — adds it to the shared workspace, and marks it onboarded. Later visits resolve to the same member, so issues, comments and inbox items keep belonging to the same person.

Identity is per client rather than per person, which is what keeps assignment, mentions, inbox and activity meaningful: each one appears in the member list under its own name. Consequences to plan for:

- One person using both the desktop app and a browser is **two** members, and the same person in two browsers is two more. Assign work to the one they actually use.
- Losing or deleting a machine's `device-identity.json`, or clearing site data for the web app, joins as a **new** member next time. Earlier issues stay with the old identity.
- Copying `device-identity.json` to a second machine makes both machines the **same** member.
- Signing out works, and is a one-visit affair: the next launch or page load mints a session again. There is nothing to sign out of on a deployment with no login.

Nothing has to be configured to enable this. Point the desktop app at your server the usual way — `~/.multica/desktop.json` with your own `apiUrl` and `wsUrl` (see [Manual CLI Configuration](#manual-cli-configuration) for where that file lives) — and open the web app at your own origin. Both negotiate the capability from `/api/config`.

To require a login instead, set the switch and restart the backend:

```bash
echo "MULTICA_DEVICE_AUTH_ENABLED=false" >> .env
docker compose -f docker-compose.selfhost.yml up -d backend
```

On Kubernetes the same settings live under `backend.config.deviceAuth.*` in `values.yaml` (`enabled`, `workspaceSlug`, `workspaceName`, `role`); `enabled: ""` takes the default and `enabled: "false"` turns it off. After `helm upgrade` the backend pod rolls automatically because the ConfigMap hash changes.

Turning it off sends clients back to the login page on their next launch, but sessions already issued stay valid until they expire — rotate `JWT_SECRET` if you need them dead immediately. Device users and their data are left in place; they are identifiable by their `@device.multica.local` email suffix.

If the deployment has no outbound access at all, see [Air-gapped / Offline Deployment](#air-gapped--offline-deployment) for how to get the images and installers in.

What this does **not** solve: agents still need to reach a model endpoint. On an intranet with no outbound access, assigning an issue to an agent will not run it until you provide an internal LLM gateway or allow egress to the model provider.

---

## Air-gapped / Offline Deployment

Everything is built on a machine **with** network; only finished artifacts cross into the air gap. Nothing in the build can run offline — the backend image runs `go mod download` and the web image runs `pnpm install`.

Note that `make selfhost` is not an option here twice over: it needs GHCR, and the published images are not your checkout. Build from source.

### On the networked machine

**1. Export the server bundle**

```bash
make offline-bundle                             # images for linux/amd64
make offline-bundle PLATFORM=linux/arm64        # ARM server
make offline-bundle OUT=/media/usb/multica
```

It builds `multica-backend:dev` and `multica-web:dev` from this checkout, pulls the database image the compose file pins, and writes `dist/offline/`:

| File | Purpose |
| --- | --- |
| `multica-images.tar.gz` | All three images, for `docker load` |
| `docker-compose.selfhost.yml` | The stack definition |
| `.env.example` | Template to copy to `.env` offline |
| `README.md` | The import steps, generated with this build's commit |
| `MANIFEST.txt` | Image list, version, commit, target platform, archive checksum |

Images are built for **linux/amd64** by default, not for the build host. That default is deliberate: an arm64 bundle built on an Apple Silicon laptop loads without complaint and then every container exits with `exec format error` on an x86 server — after somebody has already carried it across the air gap. Building for an architecture other than the host's runs under emulation and is slow; the script says so before it starts. Building on a native host of the target architecture is much faster.

**2. Build the desktop installers**

Installers are per platform and signing is site-specific, so they are not part of the bundle. **Build each one on that platform** — this is what CI does (`.github/workflows/desktop-smoke.yml` packages Windows on `windows-latest`, Linux on `ubuntu-latest`):

```bash
cd apps/desktop
CSC_IDENTITY_AUTO_DISCOVERY=false pnpm package -- --win --x64      # on Windows
pnpm package -- --mac --arm64                                      # on macOS
pnpm package -- --linux --x64                                      # on Linux
```

Each build machine needs Node 22, pnpm, **and a Go toolchain** — packaging cross-compiles `server/cmd/multica` for the target and bundles it, so the app ships with its own daemon (`multica.exe` on Windows).

`pnpm package -- --all-platforms` can produce every target from a macOS host, but that path is not what CI exercises; if electron-builder asks for Wine to assemble the Windows NSIS installer, install it or fall back to building on Windows.

Signing:

- Windows: unsigned NSIS installers trigger a SmartScreen warning ("Windows protected your PC" → More info → Run anyway). `CSC_IDENTITY_AUTO_DISCOVERY=false` keeps electron-builder from hunting for a certificate; supply your own code-signing cert to avoid the warning.
- macOS: without `APPLE_TEAM_ID` in the environment, notarization is skipped, and Gatekeeper blocks first launch — users must right-click → Open, or you sign with your own certificate.

**Build the app and the server from the same commit.** Capability negotiation is per-build: an app older than the server's device-auth support will not use it, and a server older than the app will not offer it.

**3. If you want agents, carry their CLI too**

Agents run through an agent CLI (Claude Code, Codex, …) on the runtime machine, normally installed from npm. Bring the package or pre-install it — see the caveat at the end of this section.

### Inside the air gap

**4. Load the images and point Compose at them**

```bash
docker load -i multica-images.tar.gz
cp .env.example .env
```

In `.env`, the overrides that keep Compose away from the registry, plus the usual secrets:

```
MULTICA_BACKEND_IMAGE=multica-backend
MULTICA_WEB_IMAGE=multica-web
MULTICA_IMAGE_TAG=dev

JWT_SECRET=<openssl rand -hex 32>
POSTGRES_PASSWORD=<openssl rand -hex 24>   # keep DATABASE_URL's password in sync

MULTICA_PUBLIC_URL=http://<server-host>:8080
MULTICA_APP_URL=http://<server-host>:3000
```

Device auth needs no line here: it is on by default for a self-hosted deployment, which is what lets these clients start with no login. Add `MULTICA_DEVICE_AUTH_ENABLED=false` if you would rather they didn't.

Leave `RESEND_API_KEY` and `GOOGLE_CLIENT_ID` empty — neither is reachable, and the web UI hides the Google button when the client id is unset. With device auth on you need no mail path at all; if you want email login as a fallback, point `SMTP_HOST` at an internal relay, otherwise verification codes only ever appear in the backend logs.

**5. Start and verify**

```bash
docker compose -f docker-compose.selfhost.yml up -d
curl -sf http://localhost:8080/health
curl -s http://localhost:8080/api/config     # expect "device_auth_available":true
```

Migrations are not a separate step — the backend container runs them before the API starts.

> Compose publishes both ports on `127.0.0.1` by default, so no other machine can reach them yet. Bind them to the LAN or put a reverse proxy in front. Missing this step looks exactly like device auth not working: clients sit on the connecting screen.

**6. Set up the clients**

Install the desktop package, then write `desktop.json` on each machine — `~/.multica/desktop.json` on macOS and Linux, `C:\Users\<you>\.multica\desktop.json` on Windows (the app reads it from the OS home directory):

```json
{
  "schemaVersion": 1,
  "apiUrl": "http://<server-host>:8080",
  "wsUrl": "ws://<server-host>:8080/ws",
  "appUrl": "http://<server-host>:3000"
}
```

`wsUrl` matters as much as `apiUrl`: with only the API pointed at your server, requests work but live updates keep dialling the cloud, so the app looks like it needs a manual refresh for everything.

### What does not work offline

- **Agents cannot run.** This is the significant one. The agent CLI on the runtime machine needs a model endpoint; with no egress, assigning an issue to an agent parks it. The fix is an internal OpenAI-compatible gateway plus a CLI that accepts a base-URL override — and `MULTICA_LLM_BASE_URL` can point the server's own helper calls (chat titles, quick actions) at the same gateway.
- **Desktop auto-update.** The updater feed is GitHub Releases, so upgrades mean rebuilding the bundle and redistributing installers.
- **Third-party integrations.** Slack, Lark, DingTalk, WeCom, Telegram, and GitHub all need egress and a publicly reachable webhook URL. Importing skills from GitHub likewise.
- **Google sign-in**, and any docs/changelog link in the Help menu (they point at multica.ai).

PostHog analytics disables itself — a self-hosted server returns an empty key, so the client ships nothing.

---

## Kubernetes Deployment (Alternative)

If you already run a Kubernetes cluster, you can deploy Multica there instead of Docker Compose using the released OCI Helm chart at `oci://ghcr.io/multica-ai/charts/multica` or the source chart at [`deploy/helm/multica/`](deploy/helm/multica/). It targets a typical k3s / k8s setup with an Ingress controller and a default `ReadWriteOnce` StorageClass — authored against k3s + Traefik + `local-path`, and should work on any cluster with minor tweaks.

The chart creates the following resources in the target namespace:

- `multica-postgres` — `pgvector/pgvector:pg17` backed by a 10Gi PVC
- `multica-backend` — Go API/WS server. Backed by a 5Gi `ReadWriteOnce` uploads PVC by default; set `backend.uploads.persistence.enabled=false` when you have configured S3 (`backend.config.s3Bucket`) and don't want the chart to declare the PVC at all.
- `multica-frontend` — Next.js standalone server
- Two `Ingress` resources: one for the web host, one for the backend host
- `multica-config` ConfigMap (rendered from `values.yaml`)

The `multica-secrets` Secret is **not** managed by the chart — you create it once with `kubectl` so real values never need to land in git.

> **Runtime frontend upstreams:** current `multica-web` images read `REMOTE_API_URL` and `DOCS_URL` when the Next.js server runs, so API/docs upstream changes do not require a web rebuild. The chart defaults `REMOTE_API_URL` to this release's backend Service. `frontend.compatibility.backendAlias` exists only for legacy images that still baked `REMOTE_API_URL=http://backend:8080` at build time.

> **Prerequisites:** `kubectl` and `helm` (v3.13+ for `--take-ownership`, or v4+) configured for the target cluster, an Ingress controller (Traefik / NGINX), and a default StorageClass.

### Step 1 — Point hostnames at the cluster

The chart defaults to `multica.dev.lan` (web) and `api.multica.dev.lan` (backend). Pick one of:

- **`/etc/hosts`** on every machine that needs access (developer laptops + the machine running the daemon):

  ```text
  192.168.1.206  multica.dev.lan api.multica.dev.lan
  ```

  Replace `192.168.1.206` with any node IP where your Ingress controller's Service is reachable.

- **Local DNS** (Pi-hole, Unbound, etc.): add A records for both hostnames pointing at the cluster Ingress IP.

To use different hostnames, override the matching values at install time (see [Step 4](#step-4--install-the-chart)) — `ingress.frontend.host`, `ingress.backend.host`, plus `backend.config.appUrl`, `backend.config.frontendOrigin`, `backend.config.localUploadBaseUrl`, and `backend.config.googleRedirectUri`.

### Step 2 — Create the namespace

```bash
kubectl create namespace multica
```

### Step 3 — Create the `multica-secrets` Secret

The chart references this Secret by name. Create it once with random values:

```bash
kubectl -n multica create secret generic multica-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -hex 32)" \
  --from-literal=POSTGRES_PASSWORD="$(openssl rand -hex 16)" \
  --from-literal=RESEND_API_KEY="" \
  --from-literal=GOOGLE_CLIENT_SECRET="" \
  --from-literal=CLOUDFRONT_PRIVATE_KEY="" \
  --from-literal=MULTICA_DEV_VERIFICATION_CODE=""
```

Leave optional values empty for now — you can fill them in later (see [Step 5 — Log In](#step-5--log-in)).

### Step 4 — Install the chart

```bash
helm install multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica
```

Released chart versions strip the leading `v` from the Git tag. For example, release tag `v0.3.5` publishes chart version `0.3.5`; the chart defaults the backend and frontend image tags to `v0.3.5`.

To override defaults, export the chart values, edit them, and pass them with `-f`:

```bash
helm show values oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> > my-values.yaml
# edit my-values.yaml — e.g. change ingress hosts, image tags, resource limits
helm install multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica \
  -f my-values.yaml
```

When developing from a checkout, use the local chart path instead:

```bash
helm install multica deploy/helm/multica -n multica
```

Watch the pods come up:

```bash
kubectl -n multica get pods -w
```

On a cold cluster the backend can sit `Running` but not `Ready` for a few minutes while it waits on PostgreSQL and runs migrations — a startupProbe absorbs this, so the pod should not restart. Once the backend reports `Ready`, migrations have completed and `/healthz` returns OK:

```bash
curl -H "Host: api.multica.dev.lan" http://<ingress-ip>/healthz
# {"status":"ok","checks":{"db":"ok","migrations":"ok"}}
```

Then open http://multica.dev.lan in your browser.

### Step 5 — Log In

The chart defaults to `APP_ENV=production` (set in `values.yaml` under `backend.config.appEnv`), and there is no fixed verification code by default. Pick one of the following to log in — the same three options as the Docker setup:

- **Recommended (production):** patch the Secret with a real Resend key, then restart the backend:

  ```bash
  kubectl -n multica patch secret multica-secrets --type=merge \
    -p '{"stringData":{"RESEND_API_KEY":"re_xxx"}}'
  kubectl -n multica rollout restart deploy/multica-backend
  ```

  Real verification codes will be sent to the email address you enter. See [Advanced Configuration → Email](SELF_HOSTING_ADVANCED.md#email-required-for-authentication).

- **Without email configured:** the verification code is generated server-side and printed to the backend pod logs (look for `[DEV] Verification code for ...:`). Useful for one-off testing.

  ```bash
  kubectl -n multica logs -f deploy/multica-backend | grep "Verification code"
  ```

- **Deterministic local/private testing:** set `backend.config.appEnv: development` in your values file and `MULTICA_DEV_VERIFICATION_CODE=888888` in the Secret, then `helm upgrade` and restart. This fixed code is ignored when `APP_ENV=production`.

  ```bash
  helm upgrade multica oci://ghcr.io/multica-ai/charts/multica \
    --version <chart-version> \
    -n multica \
    -f my-values.yaml --set backend.config.appEnv=development
  kubectl -n multica patch secret multica-secrets --type=merge \
    -p '{"stringData":{"MULTICA_DEV_VERIFICATION_CODE":"888888"}}'
  kubectl -n multica rollout restart deploy/multica-backend
  ```

`ALLOW_SIGNUP`, `DISABLE_WORKSPACE_CREATION`, and `GOOGLE_CLIENT_ID` likewise live under `backend.config.*` in `values.yaml` (as `allowSignup`, `disableWorkspaceCreation`, and `googleClientId`). After `helm upgrade`, the backend pod will roll automatically because the ConfigMap hash changes; the web UI reads all three from `/api/config` at runtime, so no web rebuild is needed.

> **Warning:** do **not** set `MULTICA_DEV_VERIFICATION_CODE` on a publicly reachable instance — anyone who knows an email address can then log in with that fixed code.

### Step 6 — Install CLI & Start Daemon

The daemon runs on your local machine, not in the cluster. Install the CLI and an AI agent as in [Step 3](#step-3--install-cli--start-daemon) above, then point the CLI at your Ingress hostnames:

```bash
multica setup self-host \
  --server-url http://api.multica.dev.lan \
  --app-url http://multica.dev.lan
```

Make sure the machine running the daemon has the same `/etc/hosts` (or DNS) entries from [Step 1](#step-1--point-hostnames-at-the-cluster).

### Updating

To pull the latest images without changing the chart version when your values still use the mutable `latest` image tag:

```bash
kubectl -n multica rollout restart deploy/multica-backend deploy/multica-frontend
```

To upgrade to a specific Multica release, upgrade to the matching chart version. The released chart defaults its app images to the matching Git tag:

```bash
helm upgrade multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica \
  -f my-values.yaml
```

If you need to override the app images independently from the chart version, set the image tags in your values file:

```yaml
images:
  backend:
    tag: v0.2.4
  frontend:
    tag: v0.2.4
```

Then run the same upgrade command with `-f my-values.yaml`:

```bash
helm upgrade multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica \
  -f my-values.yaml
```

To roll back if an upgrade goes sideways:

```bash
helm -n multica rollback multica
```

> **Upgrading from `v0.3.4` to `v0.3.5+` fails with `refusing to drop legacy daily rollups: ...`?** As of MUL-2957 the `migrate up` command runs an idempotent monthly-slice backfill automatically before applying migration `103`, so a clean upgrade is a single `helm upgrade` + backend rollout. If you are still on a pre-MUL-2957 binary or the auto-hook fails, run the standalone backfill against the same database the chart is using (`kubectl -n multica exec deploy/multica-backend -- ./backfill_task_usage_hourly --sleep-between-slices=2s`), then restart the backend deployment to re-apply migrations. See [Advanced Configuration → Usage Dashboard Rollup](SELF_HOSTING_ADVANCED.md#usage-dashboard-rollup) for the full recovery flow.

### Tearing down

```bash
# Remove the workloads but keep the PVCs and the Secret
helm -n multica uninstall multica

# Wipe everything, including PostgreSQL data and uploads
kubectl delete namespace multica
```

---

## Usage Dashboard Rollup

The Usage / Runtime dashboards read from a derived `task_usage_hourly` table populated by `rollup_task_usage_hourly()`. As of MUL-2957 the backend runs this rollup **in-process** on every replica via a DB-backed scheduler (`sys_cron_executions`); a fresh self-host install needs no operator action and the bundled `pgvector/pgvector:pg17` image works without changes — you do **not** need to swap it for an image that ships `pg_cron`, register an external cron job, set up a systemd timer, or run a Kubernetes `CronJob`.

Multiple backend replicas are safe: each replica ticks every 30 seconds and tries to claim the current 5-minute UTC plan, but the unique key `(job_name, scope_kind, scope_id, plan_time)` means only one wins each plan. Inspect steady-state operation:

> **WeCom (企业微信) smart bot and replica count.** Unlike Slack and Lark, whose outbound is stateless HTTP that any replica can perform, the WeCom smart bot's only outbound path is an in-process WebSocket long connection, held by the replica that owns that bot's lease. What happens to a reply produced on a *different* replica depends on the realtime relay:
>
> - **Sharded or dual relay mode (`REDIS_URL` set, the default with Redis):** the reply or inbox push is forwarded to the lease holder over the relay and delivered. Multi-replica WeCom is supported in this mode.
> - **Legacy relay mode, or no Redis:** the reply is dropped and the WeCom user sees nothing. Run the WeCom-enabled backend as a single replica in this configuration.
>
> In **every** mode there is one residual window: a reply produced while *no* replica holds a live connection to that bot — all of them mid-reconnect — is not delivered. It is **counted**: the replica that routed it checks afterwards whether any replica ever claimed the delivery, and increments `multica_wecom_outbound_dropped_total{reason="no_live_connection"}` when none did, so the window can be measured on a deployment rather than guessed at. If a rare lost reply during reconnects is unacceptable, a single replica remains the most conservative deployment. Everything else (including the rollup scheduler above) is multi-replica safe.

```sql
SELECT plan_time, status, attempt, runner_id,
       error_code, error_msg, started_at, finished_at
  FROM sys_cron_executions
 WHERE job_name = 'rollup_task_usage_hourly'
 ORDER BY plan_time DESC
 LIMIT 20;
```

Full reference (audit table semantics, advisory lock 4246, the standalone backfill command, flag descriptions, the `v0.3.4 → v0.3.5+` migration auto-hook) lives in [Advanced Configuration → Usage Dashboard Rollup](SELF_HOSTING_ADVANCED.md#usage-dashboard-rollup).

> **Upgrading from `v0.3.4` to `v0.3.5+`?** As of MUL-2957 the `migrate up` command runs an idempotent monthly-slice backfill automatically right before applying migration `103`, so the upgrade completes in a single invocation — no operator step required. If you are still on a pre-MUL-2957 binary or the auto-hook fails for an environmental reason, run `backfill_task_usage_hourly` against the same database and re-run the upgrade. See [Advanced Configuration → Usage Dashboard Rollup](SELF_HOSTING_ADVANCED.md#usage-dashboard-rollup) for the recovery flow.

### Compatibility paths (existing deployments only)

External schedulers — **`pg_cron` registered on the database, an external cron job, a systemd timer, or a Kubernetes `CronJob`** — that call `SELECT rollup_task_usage_hourly()` directly were the only option before MUL-2957 and remain a supported compatibility path. They are no longer the recommended setup; new deployments should rely on the in-process scheduler instead. The SQL function holds advisory lock 4246 internally, so the in-process scheduler and any pre-existing external schedule can coexist without ever double-writing the rollup.

If you already have a `pg_cron` job in production, the safe sequence to retire it is:

1. Confirm the in-process scheduler is healthy on at least one backend replica — recent SUCCESS rows should be landing in `sys_cron_executions` for `rollup_task_usage_hourly`:

   ```sql
   SELECT plan_time, status, runner_id, finished_at
     FROM sys_cron_executions
    WHERE job_name = 'rollup_task_usage_hourly'
      AND status = 'SUCCESS'
    ORDER BY plan_time DESC
    LIMIT 5;
   ```

2. Once SUCCESS rows are arriving on schedule, unschedule the redundant `pg_cron` entry:

   ```sql
   SELECT cron.unschedule('rollup_task_usage_hourly')
     FROM cron.job WHERE jobname = 'rollup_task_usage_hourly';
   ```

3. Leave the `pg_cron` extension itself installed unless you are sure no other workload depends on it. The bundled `pgvector/pgvector:pg17` image does **not** ship `pg_cron`, so nothing in Multica's default install needs it; uninstalling `pg_cron` from a custom image that other workloads still use is a separate decision.

External cron / systemd timer / Kubernetes `CronJob` setups that call `SELECT rollup_task_usage_hourly()` directly can be retired the same way — once `sys_cron_executions` shows steady SUCCESS rows from the in-process scheduler, the external job is redundant and can be removed.

## Stopping Services

If you installed via the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.sh | bash -s -- --stop
```

If you cloned the repo manually:

```bash
# Stop the Docker Compose services (backend, frontend, database)
make selfhost-stop

# Stop the local daemon
multica daemon stop
```

## Switching to Multica Cloud

If you've been self-hosting and want to switch your CLI to [Multica Cloud](https://multica.ai):

```bash
multica setup
```

This reconfigures the CLI for multica.ai, re-authenticates, and restarts the daemon. You will be prompted before overwriting the existing configuration.

> Your local Docker services are unaffected. Stop them separately if you no longer need them.

## Upgrading

```bash
docker compose -f docker-compose.selfhost.yml pull
docker compose -f docker-compose.selfhost.yml up -d
```

Pin `MULTICA_IMAGE_TAG` in `.env` to an exact version like `v0.2.4` if you want to stay on a specific release. Migrations run automatically on backend startup.
If the selected GHCR tag has not been published yet, fall back to `make selfhost-build` or `docker compose -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml up -d --build`.

> **Upgrading from `v0.3.4` to `v0.3.5+` fails with `refusing to drop legacy daily rollups: ...`?** That's migration `103`'s fail-closed guard: it requires `task_usage_hourly` to be seeded before the legacy daily rollups are dropped. As of MUL-2957 `migrate up` runs that backfill automatically right before applying `103`, so the upgrade completes in a single invocation. If you are still on a pre-MUL-2957 binary or the auto-hook fails, run `backfill_task_usage_hourly` manually first, then re-run the upgrade. Full instructions in [Advanced Configuration → Usage Dashboard Rollup](SELF_HOSTING_ADVANCED.md#usage-dashboard-rollup).

---

## Manual Docker Compose Setup

If you prefer running Docker Compose steps manually instead of `make selfhost`:

```bash
git clone https://github.com/multica-ai/multica.git
cd multica
cp .env.example .env
```

Edit `.env` — set `JWT_SECRET` (required): docker compose refuses to start without
it, and a production backend refuses to boot on the dev default or any known
placeholder.

```bash
JWT_SECRET=$(openssl rand -hex 32)
```

Then start everything:

```bash
docker compose -f docker-compose.selfhost.yml pull
docker compose -f docker-compose.selfhost.yml up -d
```

## Manual CLI Configuration

If you prefer configuring the CLI step by step instead of `multica setup`:

```bash
# Point CLI to your local server
multica config set server_url http://localhost:8080
multica config set app_url http://localhost:3000

# Login (opens browser)
multica login

# Start the daemon
multica daemon start
```

For production deployments with TLS:

```bash
multica config set app_url https://app.example.com
multica config set server_url https://api.example.com
multica login
multica daemon start
```

## Advanced Configuration

For environment variables, manual setup (without Docker), reverse proxy configuration, database setup, and more, see the [Advanced Configuration Guide](SELF_HOSTING_ADVANCED.md).
