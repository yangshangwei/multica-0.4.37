#!/usr/bin/env bash
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
GUARD_SCRIPT="$SCRIPT_DIR/go-test-with-agent-cli-guard.sh"

usage() {
  echo "usage: $0 [--race]" >&2
}

# internal/service alone takes ~13 min under -race, past go test's 10 min default.
go_test_args=(test -timeout=40m)
case "$#" in
  0) ;;
  1)
    if [ "$1" != "--race" ]; then
      usage
      exit 2
    fi
    go_test_args+=(-race)
    ;;
  *)
    usage
    exit 2
    ;;
esac

cd "$REPO_ROOT/server"
packages=$(go list ./...)
regular_packages=()
for package in $packages; do
  case "$package" in
    */pkg/agent|*/pkg/agent/*) ;;
    *) regular_packages+=("$package") ;;
  esac
done

# Database-backed packages share DATABASE_URL and mutate singleton rows, such
# as the internal organization. Serialize packages to keep their fixtures apart.
"$GUARD_SCRIPT" -- go "${go_test_args[@]}" -p 1 -parallel 2 "${regular_packages[@]}"
# Subprocess-backed agent tests have hard deadlines. Limit both package and
# within-package parallelism so race builds do not starve their parent loops.
"$GUARD_SCRIPT" -- go "${go_test_args[@]}" -p 2 -parallel 2 ./pkg/agent/...
