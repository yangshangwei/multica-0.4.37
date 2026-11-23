#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env}"
create_remote=false
case "${2:-}" in
  '') ;;
  --create-database) create_remote=true ;;
  *) echo "Usage: ensure-postgres.sh [env-file] [--create-database]" >&2; exit 1 ;;
esac

if [ "$ENV_FILE" != --current-environment ]; then
  if [ ! -f "$ENV_FILE" ]; then
    echo "Missing env file: $ENV_FILE"
    echo "Create .env from .env.example, or run 'make worktree-env' and use .env.worktree."
    exit 1
  fi

  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi

POSTGRES_DB="${POSTGRES_DB:-multica}"
POSTGRES_USER="${POSTGRES_USER:-multica}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-multica}"
DATABASE_URL="${DATABASE_URL:-}"

wait_for_database() {
  local attempt
  for attempt in {1..10}; do
    if "$@" > /dev/null 2>&1; then return 0; fi
    [ "$attempt" -eq 10 ] || sleep 1
  done
  echo "PostgreSQL is not reachable with the configured connection. No alternate database was provisioned." >&2
  return 1
}

if [ -z "$DATABASE_URL" ]; then
  # Legacy setup without a connection URL explicitly selects shared Docker.
  export PGPASSWORD="$POSTGRES_PASSWORD"
  echo "==> Ensuring shared PostgreSQL container is running on localhost:5432..."
  docker compose up -d postgres

  echo "==> Waiting for PostgreSQL to be ready..."
  wait_for_database docker compose exec -T postgres pg_isready -U "$POSTGRES_USER" -d postgres

  echo "==> Ensuring database '$POSTGRES_DB' exists..."
  db_exists="$(docker compose exec -T postgres \
    psql -U "$POSTGRES_USER" -d postgres -Atqc "SELECT 1 FROM pg_database WHERE datname = '$POSTGRES_DB'")"

  if [ "$db_exists" != "1" ]; then
    docker compose exec -T postgres \
      psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 \
      -c "CREATE DATABASE \"$POSTGRES_DB\"" \
      > /dev/null
  fi

  echo "✓ PostgreSQL ready (local Docker). Database: $POSTGRES_DB"
  exit 0
fi

for tool in node psql; do
  command -v "$tool" >/dev/null 2>&1 || { echo "Missing prerequisite: $tool" >&2; exit 1; }
done

# Preserve credentials and non-routing options for libpq. Query parameters can
# override a URL's apparent endpoint, so refuse those before any connection.
connection="$(node <<'JS'
try {
  const url = new URL(process.env.DATABASE_URL);
  const database = decodeURIComponent(url.pathname.slice(1));
  const routing = new Set(["host", "hostaddr", "port", "dbname", "service"]);
  if (!["postgres:", "postgresql:"].includes(url.protocol) || !url.hostname ||
      !database || url.hash || /[\x00-\x1f\x7f]/.test(database) ||
      [...url.searchParams.keys()].some(key => routing.has(key.toLowerCase()))) throw new Error();
  const local = ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname);
  const shared = ["localhost", "127.0.0.1"].includes(url.hostname) && (!url.port || url.port === "5432");
  // URLSearchParams serialization turns spaces into +, which libpq treats
  // literally. Preserve original option encoding while bounding waits.
  const options = url.search.slice(1).split("&").filter(part =>
    part && decodeURIComponent(part.split("=", 1)[0]).toLowerCase() !== "connect_timeout");
  url.search = [...options, "connect_timeout=3"].join("&");
  const target = url.toString();
  url.pathname = "/postgres";
  process.stdout.write([local ? "local" : "remote", shared ? "shared" : "explicit", database, url.toString(), target].join("\n"));
} catch {
  console.error("DATABASE_URL must be a PostgreSQL URL with an explicit database and no routing overrides.");
  process.exit(1);
}
JS
)"
db_kind="$(printf '%s\n' "$connection" | sed -n '1p')"
db_startup="$(printf '%s\n' "$connection" | sed -n '2p')"
db_name="$(printf '%s\n' "$connection" | sed -n '3p')"
admin_url="$(printf '%s\n' "$connection" | sed -n '4p')"
target_url="$(printf '%s\n' "$connection" | sed -n '5p')"

if [ "$db_kind" = remote ] && [ "$create_remote" = false ]; then
  echo "==> Verifying the configured remote database. Skipping Docker and database creation."
  wait_for_database psql "$target_url" -X -Atqc 'SELECT 1'
  echo "✓ Configured remote database is reachable."
  exit 0
fi

if ! PGCONNECT_TIMEOUT=3 psql "$admin_url" -X -Atqc 'SELECT 1' > /dev/null 2>&1; then
  if [ "$db_startup" = shared ]; then
    command -v pg_isready >/dev/null 2>&1 || { echo "Missing prerequisite: pg_isready" >&2; exit 1; }
    readiness=0
    pg_isready -d "$admin_url" -t 3 > /dev/null 2>&1 || readiness=$?
    # Authentication failure is not permission to start a different server.
    # pg_isready=2 means no response; 0/1 mean an existing server answered.
    if [ "$readiness" -eq 2 ]; then
      echo "==> Starting shared PostgreSQL for the configured default endpoint..."
      docker compose up -d postgres
    fi
  fi
  wait_for_database env PGCONNECT_TIMEOUT=3 psql "$admin_url" -X -Atqc 'SELECT 1'
fi

if ! psql "$admin_url" -X -v ON_ERROR_STOP=1 -v database_name="$db_name" > /dev/null 2>&1 <<'SQL'
SELECT format('CREATE DATABASE %I', :'database_name')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'database_name')
\gexec
SQL
then
  echo "Could not provision the database through DATABASE_URL. Check its database and role permissions." >&2
  exit 1
fi
wait_for_database psql "$target_url" -X -Atqc 'SELECT 1'
echo "✓ PostgreSQL ready through DATABASE_URL."
