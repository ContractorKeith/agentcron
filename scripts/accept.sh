#!/usr/bin/env bash
# scripts/accept.sh — the acceptance for agentcron (docs/PLAN.md §8). Worker, reviewer and
# CI run this exact script; the orchestrator never retypes it. Keep it under a minute.
# Bash 3.2 compatible (macOS): no mapfile, no ${var,,}, no associative arrays.
# Each milestone extends the goldens and live checks below.
set -eu

cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
trap 'rm -r "$scratch"' EXIT
bin="$scratch/agentcron"

echo "== gate"
go test -race ./...
test -z "$(gofmt -l .)"
go vet ./...
staticcheck ./...

echo "== build"
go build -trimpath -ldflags "-s -w -X main.version=v0.0.0-accept" -o "$bin" ./cmd/agentcron
"$bin" version

echo "== fakes"
mkdir -p "$scratch/bin"
for f in claude codex mdkb; do
  go build -o "$scratch/bin/$f" "./testdata/fakes/$f"
done
export PATH="$scratch/bin:$PATH"
for f in claude codex mdkb; do
  test "$(command -v "$f")" = "$scratch/bin/$f" || { echo "fake $f not first on PATH"; exit 1; }
done

echo "== fixture"
# Isolate everything that could touch real user data.
export HOME="$scratch/home" XDG_CONFIG_HOME="$scratch/config" XDG_DATA_HOME="$scratch/data"
export XDG_CACHE_HOME="$scratch/cache" GIT_CONFIG_GLOBAL="$scratch/gitconfig" GIT_CONFIG_NOSYSTEM=1
export MDKB_ROOT="$scratch/kb"
unset AGENTCRON_CONFIG_DIR AGENTCRON_DATA_DIR AGENTCRON_RUN_ID
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_CACHE_HOME"
"$bin" init >/dev/null

echo "== goldens"
golden() { # name, args...
  name="$1"; shift
  "$bin" "$@" --json | diff - "testdata/golden/$name.json" >/dev/null || { echo "golden mismatch: $name"; exit 1; }
}
golden doctor doctor

echo "== live checks"
"$bin" init >/dev/null                                                   # init is idempotent
"$bin" --data-dir "$scratch/nowhere" doctor >/dev/null 2>&1 && exit 1 || test $? -eq 3  # uninitialised exits 3

echo "accept: ok"
