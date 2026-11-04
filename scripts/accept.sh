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
go build -o "$scratch/seed" ./testdata/seed

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
export TZ=UTC
now=2026-11-04T07:00:00-05:00
cp testdata/jobs/*.toml "$XDG_CONFIG_HOME/agentcron/jobs/"
"$scratch/seed" "$now" macro
golden ls ls --now "$now"
for job in cron macro interval once zoned; do
  golden "show-$job" show "$job" --now "$now"
done
golden next next --now "$now"
golden next-spring next --schedule "30 2 * * *" --timezone America/New_York --count 3 --now 2026-03-07T12:00:00Z
golden next-fall next --schedule "30 1 * * *" --timezone America/New_York --count 3 --now 2026-11-01T04:00:00Z

echo "== live checks"
"$bin" init >/dev/null                                                   # init is idempotent
"$bin" --data-dir "$scratch/nowhere" doctor >/dev/null 2>&1 && exit 1 || test $? -eq 3  # uninitialised exits 3

"$bin" add preview --schedule @daily --runner command --command cat --prompt hi --dry-run >"$scratch/preview"
test ! -e "$XDG_CONFIG_HOME/agentcron/jobs/preview.toml"
for action in pause resume rm; do
  "$bin" "$action" macro --now "$now" >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 2
  grep now "$scratch/err" >/dev/null
done
"$bin" add pinned --schedule @daily --runner claude --prompt hi --now "$now" >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 2
test ! -e "$XDG_CONFIG_HOME/agentcron/jobs/pinned.toml"
"$bin" add roundtrip --schedule @daily --runner claude --prompt hi >/dev/null
"$bin" add roundtrip --schedule @daily --runner claude --prompt hi >/dev/null 2>&1 && exit 1 || test $? -eq 2
"$bin" rm roundtrip --dry-run >/dev/null
"$bin" show roundtrip --json >/dev/null
"$bin" rm roundtrip >/dev/null
"$bin" show roundtrip --json >/dev/null 2>&1 && exit 1 || test $? -eq 2
"$bin" show macro --json --now "$now" | grep '"paused":true' >/dev/null
"$bin" resume macro >/dev/null
"$bin" show macro --json | grep '"paused":false' >/dev/null
printf 'schedule = "@daily"
runner = "claude"
prompt = "hi"
timeout = "0s"
' >"$XDG_CONFIG_HOME/agentcron/jobs/invalid.toml"
"$bin" show invalid --json >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 2
grep timeout "$scratch/err" >/dev/null
"$bin" ls --json | grep '"error":' >/dev/null
"$bin" rm invalid >/dev/null

echo "accept: ok"
