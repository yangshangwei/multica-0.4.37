# Running Multica Locally

How to start and stop the three local surfaces — **backend** (Go API), **web** (Next.js), and
**desktop** (Electron) — for a development checkout.

中文版：[RUNNING.zh.md](RUNNING.zh.md) · Self-hosting with Docker: [SELF_HOSTING.md](SELF_HOSTING.md)

---

## TL;DR

```bash
make up C=api,web,desktop   # start everything
make status                 # what is running, and is it mine
make down                   # stop the processes, keep the database
make destroy                # stop, and delete database + profile + slot
```

`make up` is the recommended entry point for every checkout, including git worktrees. It
allocates ports, a database name and a CLI profile under a lock, records them in
`~/.multica/dev/`, and verifies the database through `DATABASE_URL` — not through
`docker exec`, which lands in the wrong server whenever a native PostgreSQL owns 5432.

---

## Prerequisites

- Node 22 + pnpm, Go 1.26, Docker (for the shared PostgreSQL container)
- `pnpm install` once per checkout
- An env file: `.env` in the main checkout, `.env.worktree` in a worktree
  (`make worktree-env` generates one with unique ports)

---

## Managed environments (recommended)

### Start

```bash
make up                       # api + web (default)
make up C=api,web,daemon      # add the local agent daemon
make up C=desktop             # Electron against this environment's backend
make up C=api,web,daemon,desktop
make up ARGS=--ephemeral      # agent-owned, expires, collected by `make gc`
```

Component names: `api` (Go backend), `web` (Next.js), `daemon` (agent daemon),
`desktop` (Electron). **Selecting anything implies `api`.**

`make up` also runs migrations before launching. It reuses a running API only when `/health`
proves the listener's pid, process group and commit belong to this checkout.

### Inspect

```bash
make status          # this environment: ports, pids, commit proof
make list            # every registered environment on this machine
```

Logs live in `~/.multica/dev/envs/<name>/logs/` (`api.log`, `web.log`, `desktop.log`,
`migrate.log`). The daemon logs to its profile directory instead:
`~/.multica/profiles/<profile>/daemon.log`.

### Stop

```bash
make down                       # stop all components of this environment
make down ARGS="--components web,desktop"   # stop just some of them
```

`make down` keeps the database, the CLI profile and the allocated slot, so the next
`make up` returns to the same environment with the same ports and data.

### Tear down

```bash
make destroy        # stop, then drop the database, profile, daemon workspaces,
                    # Desktop userData, and the registry entry
make gc             # collect environments whose directory is gone or whose TTL expired
```

`destroy` is irreversible: the local database is dropped.

---

## Running one surface at a time

Use these when you already have an environment up and want to restart a single piece in the
foreground (to watch its output, or to attach a debugger). They read the current env file and
skip the database preflight, because `make up` already proved the database is reachable.

| Surface | Start | URL |
| --- | --- | --- |
| Backend (Go API) | `make api-dev` | `http://localhost:$BACKEND_PORT` (default 8080) |
| Web (Next.js) | `make web-dev` | `http://localhost:$FRONTEND_PORT` (default 3000) |
| Desktop (Electron) | `make up C=desktop` | Electron window |

Stop any of them with `Ctrl-C` in their terminal.

### Backend only

```bash
make server     # ensures PostgreSQL, then runs the Go server
make api-dev    # runs the Go server directly, with commit stamped into /health
```

### Web only

```bash
make web-dev    # == pnpm dev:web
pnpm dev:web    # same, without the env file wiring
```

### Desktop only

Prefer `make up C=desktop`: it hands Electron the registry-allocated renderer port, app name
and userData directory, so Desktop shares the same environment ledger as the API and web.

```bash
make up C=desktop
```

`pnpm dev:desktop` also works, but it self-isolates by deriving its own identity from the
checkout path instead of using the allocated slot. Use it only when you deliberately want a
standalone desktop instance.

To run an arbitrary command with this environment's variables:

```bash
make env-exec ARGS="-- pnpm dev:desktop"
```

### Agent daemon

```bash
make up C=daemon      # start it as part of the environment
make daemon           # restart the local daemon using the CLI's stored auth
```

---

## Legacy single-checkout commands

These predate the environment registry. They operate on the current env file directly and do
not register anything, so `make status` / `make list` will not see them. Prefer `make up`.

```bash
make setup      # install deps, ensure DB, run migrations
make start      # migrations, then backend + frontend in the foreground
make stop       # kill whatever listens on $PORT and $FRONTEND_PORT
make dev        # bootstrap end-to-end, then start in the foreground
```

`make start` runs both processes under one `trap 'kill 0' EXIT`, so `Ctrl-C` stops both.
`make stop` kills by port, which will also kill an unrelated process that happens to hold
that port.

Worktree variants: `make setup-worktree`, `make start-worktree`, `make stop-worktree`
(all pinned to `.env.worktree`); main-checkout variants: `make setup-main`, `make start-main`,
`make stop-main`.

---

## Database

The shared PostgreSQL container is separate from the app processes and survives `make down`.

```bash
make db-up       # start the shared PostgreSQL container
make db-down     # stop it (the Docker volume is kept)
make db-drop     # permanently drop this checkout's database (asks first)
make db-reset    # drop, recreate, re-run all migrations
make migrate-up  # apply migrations
```

Worktrees share one container and get isolated database names and ports via `.env.worktree`.
Remove a linked worktree together with its database:

```bash
make remove-worktree WORKTREE=../path
```

---

## Ports

Ports come from the env file (`.env` or `.env.worktree`), or from the registry allocation
when started through `make up`.

| Variable | Default | Used by |
| --- | --- | --- |
| `BACKEND_PORT` / `PORT` | 8080 | Go API, WebSocket at `/ws` |
| `FRONTEND_PORT` | 3000 | Next.js web |
| `POSTGRES_PORT` | 5432 | shared PostgreSQL |
| desktop renderer | allocated | Electron renderer dev server |

---

## Troubleshooting

**"No environment registered for …"** — run `make up` first; `status` / `down` / `destroy`
need a registry entry.

**Port already in use** — `make list` shows every environment on the machine and which ports
it owns. Stop the owner with `make down`, or `make gc` if its directory is gone.

**Backend starts but the frontend cannot reach it** — check `NEXT_PUBLIC_API_URL` and
`NEXT_PUBLIC_WS_URL` in the env file; they must point at `BACKEND_PORT`.

**Migrations failed on start** — read `~/.multica/dev/envs/<name>/logs/migrate.log`.

**A stale environment survived a deleted checkout** — `make gc`.
