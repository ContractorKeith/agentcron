#!/usr/bin/env bash
# scripts/accept.sh — the acceptance for agentcron (docs/PLAN.md §8). Worker, reviewer and
# CI run this exact script; the orchestrator never retypes it. Keep it under a minute.
# Bash 3.2 compatible (macOS): no mapfile, no ${var,,}, no associative arrays.
# Each milestone extends the goldens and live checks below.
set -eu

cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
cleanup() {
  # Join every inline owner even if a live assertion fails.
  for owner in ${cancel_owner-} ${overlap_owner-}; do wait "$owner" 2>/dev/null || :; done
  rm -r "$scratch"
}
trap cleanup EXIT
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

echo "== runner and ledger"
add_run_job() {
  "$bin" add "$1" --schedule @daily --runner claude --prompt hello --timeout 5s >/dev/null
}
expect_state() {
  "$bin" runs "$1" --limit 1 --json | grep "\"state\":\"$2\"" >/dev/null
}
non_success() {
  "$@" >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 6
}
wait_running() {
  attempts=0
  until "$bin" runs "$1" --state running --json | grep '"pid":[0-9]' >/dev/null; do
    attempts=$((attempts + 1)); test "$attempts" -lt 100
    sleep 0.05
  done
}
for job in success failure attention timeout cancel overlap child clean dirty; do add_run_job "$job"; done
FAKE_MODE=ok "$bin" run success --now >"$scratch/success-id"
expect_state success succeeded
"$bin" logs "$(cat "$scratch/success-id")" --follow | grep DONE >/dev/null
FAKE_MODE=fail non_success "$bin" run failure --now
expect_state failure failed
"$bin" add miss --schedule @daily --runner claude --prompt hi --expect missing >/dev/null
non_success "$bin" run miss --now
expect_state miss attention
"$bin" add blocked --schedule @daily --runner command --command agentcron-missing-runner --prompt hi >/dev/null
non_success "$bin" run blocked --now
expect_state blocked blocked_config
# Missing agent binary, with an isolated PATH that cannot reach real agents.
PATH="$scratch/empty-path" non_success "$bin" run attention --now
expect_state attention blocked_config
"$bin" add short --schedule @daily --runner claude --prompt hi --timeout 150ms >/dev/null
FAKE_MODE=ignore-term FAKE_SLEEP=20s non_success "$bin" run short --now
expect_state short timed_out
FAKE_MODE=spawn-child FAKE_SLEEP=2s non_success "$bin" run short --now
expect_state short timed_out
if pgrep -f "$scratch/bin/claude" >"$scratch/survivors"; then echo "surviving fake process"; exit 1; fi
FAKE_MODE=sleep FAKE_SLEEP=4s "$bin" run cancel --now >"$scratch/cancel-id" 2>"$scratch/cancel-err" &
cancel_owner=$!
wait_running cancel
cancel_id="$("$bin" runs cancel --limit 1 --json | sed -E 's/.*"id":"([^"]*)".*/\1/')"
"$bin" cancel "$cancel_id"
wait "$cancel_owner" && exit 1 || test $? -eq 6
expect_state cancel cancelled
FAKE_MODE=sleep FAKE_SLEEP=2s "$bin" run overlap --now >"$scratch/overlap-id" &
overlap_owner=$!
wait_running overlap
non_success "$bin" run overlap --now --wait
expect_state overlap skipped
wait "$overlap_owner"
# Real disposable repositories exercise clean removal and dirty pruning.
mkdir "$scratch/repo"
git -C "$scratch/repo" init -q
git -C "$scratch/repo" -c user.name=Test -c user.email=test@example.com commit -q --allow-empty -m initial
for job in clean dirty; do
  "$bin" rm "$job" >/dev/null
  "$bin" add "$job" --schedule @daily --runner claude --prompt hi --repo "$scratch/repo" >/dev/null
done
"$bin" run clean --now >/dev/null
expect_state clean succeeded
test -z "$(git -C "$scratch/repo" branch --list 'agentcron/*')"
FAKE_MODE=dirty "$bin" run dirty --now >/dev/null
"$bin" worktrees --json | grep '"kept":true' >/dev/null
"$bin" prune --dry-run >/dev/null
"$bin" worktrees --json | grep '"kept":true' >/dev/null
"$bin" prune >/dev/null
test "$("$bin" worktrees --json)" = '[]'
test -z "$(git -C "$scratch/repo" branch --list 'agentcron/*')"
"$bin" run success >"$scratch/queued-id"
"$bin" runs success --limit 1 --json | sed -E \
  -e 's/r_[0-9]{8}T[0-9]{6}Z_[A-Z2-7]{4}/RUN/g' \
  -e 's/"scheduled_for":"[^"]*"/"scheduled_for":"TIME"/g' | diff - testdata/golden/runs.json
"$bin" show success --json | grep '"state":"succeeded"' >/dev/null
"$bin" ls --json | grep '"last_state":"queued"' >/dev/null

echo "accept: ok"
