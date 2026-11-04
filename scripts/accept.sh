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
  for owner in ${daemon_owner-}; do kill "$owner" 2>/dev/null || :; done
  if test -d "$scratch/bin"; then pkill -f "$scratch/bin/claude" 2>/dev/null || :; fi
  for owner in ${daemon_owner-} ${cancel_owner-} ${overlap_owner-} ${queue_owner-}; do wait "$owner" 2>/dev/null || :; done
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
MDKB_ROOT= golden doctor doctor
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
"$bin" tui </dev/null >/dev/null 2>&1 && exit 1 || test $? -eq 2
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
"$bin" show roundtrip --json | grep '"status":"orphaned"' >/dev/null
"$bin" show macro --json --now "$now" | grep '"paused":true' >/dev/null
"$bin" resume macro >/dev/null
"$bin" show macro --json | grep '"paused":false' >/dev/null
printf 'schedule = "@daily"
runner = "claude"
prompt = "hi"
timeout = "0s"
' >"$XDG_CONFIG_HOME/agentcron/jobs/invalid.toml"
"$bin" show invalid --json >"$scratch/out"
grep '"status":"invalid"' "$scratch/out" >/dev/null
grep timeout "$scratch/out" >/dev/null
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
FAKE_MODE=spawn-child FAKE_SLEEP=20s non_success "$bin" run short --now
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
  -e 's/r_[0-9]{8}T[0-9]{6}Z_[A-Z2-7]{8}/RUN/g' \
  -e 's/"scheduled_for":"[^"]*"/"scheduled_for":"TIME"/g' | diff - testdata/golden/runs.json
"$bin" show success --json | grep '"state":"succeeded"' >/dev/null
"$bin" ls --json | grep '"last_state":"queued"' >/dev/null

echo "== daemon, tick and services"
export AGENTCRON_CONFIG_DIR="$scratch/m3-config" AGENTCRON_DATA_DIR="$scratch/m3-data"
"$bin" init >/dev/null
# Render both platforms without ever invoking the host service manager.
for target in linux darwin; do
  PATH=/fake/bin "$bin" service install --dry-run --os "$target" --json |
    sed -e "s|/private$scratch|/SCRATCH|g" -e "s|$scratch|/SCRATCH|g" -e 's|gui/[0-9][0-9]*|gui/UID|g' >"$scratch/service-$target.json"
  diff "$scratch/service-$target.json" "testdata/golden/service-$target.json"
done
# A real daemon fires a near-future one-shot with the fake runner.
fire="$(date -u -d '+3 seconds' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v+3S +%Y-%m-%dT%H:%M:%SZ)"
"$bin" add due --schedule "at $fire" --runner claude --prompt hello >/dev/null
"$bin" daemon >"$scratch/daemon-out" 2>"$scratch/daemon-err" &
daemon_owner=$!
attempts=0
until "$bin" doctor --json | grep '"daemon_pid":[1-9]' >/dev/null; do
  attempts=$((attempts+1)); test "$attempts" -lt 100; sleep 0.05
done
for action in daemon tick; do
  "$bin" "$action" >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 4
  grep "$daemon_owner" "$scratch/err" >/dev/null
done
attempts=0
until "$bin" runs due --json | grep '"state":"succeeded"' >/dev/null; do
  attempts=$((attempts+1)); test "$attempts" -lt 400; sleep 0.1
done
"$bin" show due --json | grep '"status":"done"' >/dev/null
kill -TERM "$daemon_owner"; wait "$daemon_owner"; daemon_owner=
# Queued runs execute in a one-pass tick and queued --wait observes the result.
"$bin" run due --wait >"$scratch/wait-id" &
queue_owner=$!
attempts=0
until "$bin" runs due --state queued --json | grep '"state":"queued"' >/dev/null; do
  attempts=$((attempts+1)); test "$attempts" -lt 100; sleep 0.05
done
"$bin" tick >"$scratch/tick-out" 2>"$scratch/tick-err"
wait "$queue_owner"
expect_state due succeeded
# Killing the owner leaves an unknown outcome, including when its child is live.
"$bin" add restart --schedule @daily --runner claude --prompt hello --timeout 2m >/dev/null
"$bin" run restart >/dev/null
FAKE_MODE=sleep FAKE_SLEEP=90s "$bin" daemon >"$scratch/daemon-out" 2>"$scratch/daemon-err" &
daemon_owner=$!
wait_running restart
kill -KILL "$daemon_owner"; wait "$daemon_owner" 2>/dev/null || :; daemon_owner=
"$bin" tick >"$scratch/tick-out" 2>"$scratch/tick-err"
expect_state restart unknown
"$bin" tick >"$scratch/tick-out" 2>"$scratch/tick-err"
expect_state restart unknown
test "$("$bin" runs restart --json | grep -o '"id":' | wc -l | tr -d ' ')" = 1
pkill -f "$scratch/bin/claude" 2>/dev/null || :
# SIGTERM drains the running process group and records the shutdown reason.
"$bin" run restart >/dev/null
FAKE_MODE=sleep FAKE_SLEEP=90s "$bin" daemon >"$scratch/daemon-out" 2>"$scratch/daemon-err" &
daemon_owner=$!
wait_running restart
kill -TERM "$daemon_owner"; wait "$daemon_owner"; daemon_owner=
expect_state restart cancelled
"$bin" runs restart --limit 1 --json | grep '"error":"[^"]*daemon stopped' >/dev/null
if pgrep -f "$scratch/bin/claude" >"$scratch/survivors"; then echo "surviving fake process"; exit 1; fi
# no-wait keeps an inherited lock in a detached worker until the run ends.
"$bin" run due >/dev/null
no_wait_output="$(FAKE_MODE=sleep FAKE_SLEEP=1s "$bin" tick --no-wait 2>&1)"
test -z "$no_wait_output"
wait_running due
"$bin" tick >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 4
attempts=0
until "$bin" runs due --limit 1 --json | grep '"state":"succeeded"' >/dev/null; do
  attempts=$((attempts+1)); test "$attempts" -lt 100; sleep 0.05
done
# A detached worker's runner must not inherit the daemon lock descriptor.
"$bin" run restart >/dev/null
no_wait_output="$(FAKE_MODE=sleep FAKE_SLEEP=90s "$bin" tick --no-wait 2>&1)"
test -z "$no_wait_output"
wait_running restart
worker_pid="$("$bin" doctor --json | sed -E 's/.*"daemon_pid":([0-9]+).*/\1/')"
kill -KILL "$worker_pid"
attempts=0
until "$bin" doctor --json | grep '"daemon_pid":0' >/dev/null; do
  attempts=$((attempts+1)); test "$attempts" -lt 100; sleep 0.05
done
"$bin" tick >"$scratch/tick-out" 2>"$scratch/tick-err"
expect_state restart unknown
pkill -f "$scratch/bin/claude" 2>/dev/null || :
# Reconcile must drain a remaining group even after its leader is reaped.
process_live() {
  process_state="$(ps -p "$1" -o stat= 2>/dev/null)" || return 1
  case "$process_state" in *Z*) return 1 ;; *) return 0 ;; esac
}
for mode in background spawn-child; do
  "$bin" run restart >/dev/null
  FAKE_MODE="$mode" FAKE_CHILD_PID="$scratch/orphan-child" FAKE_SLEEP=90s "$bin" daemon >"$scratch/daemon-out" 2>"$scratch/daemon-err" &
  daemon_owner=$!
  wait_running restart
  attempts=0
  until test -s "$scratch/orphan-child"; do
    attempts=$((attempts+1)); test "$attempts" -lt 100; sleep 0.05
  done
  child_pid="$(cat "$scratch/orphan-child")"
  leader_pid="$("$bin" runs restart --state running --limit 1 --json | sed -E 's/.*"pid":([0-9]+).*/\1/')"
  kill -KILL "$daemon_owner"; wait "$daemon_owner" 2>/dev/null || :; daemon_owner=
  kill -KILL "$leader_pid" 2>/dev/null || :
  attempts=0
  while ps -p "$leader_pid" >/dev/null 2>&1; do
    attempts=$((attempts+1)); test "$attempts" -lt 100; sleep 0.05
  done
  "$bin" tick >"$scratch/tick-out" 2>"$scratch/tick-err"
  expect_state restart unknown
  attempts=0
  while process_live "$child_pid"; do
    attempts=$((attempts+1)); test "$attempts" -lt 100; sleep 0.05
  done
  rm "$scratch/orphan-child"
done

echo "== memory"
unset AGENTCRON_CONFIG_DIR AGENTCRON_DATA_DIR
for f in claude codex mdkb; do
  test "$(command -v "$f")" = "$scratch/bin/$f"
done
mkdir -p "$MDKB_ROOT"
"$bin" doctor --json | grep '"kb_initialised":false' >/dev/null
mkdir "$MDKB_ROOT/.mdkb"
export FAKE_RECORD="$scratch/memory-record"
"$bin" add memory-claude --schedule @daily --runner claude --prompt remember >/dev/null
"$bin" run memory-claude --now >"$scratch/memory-id"
mcp_file="$XDG_DATA_HOME/agentcron/runs/$(cat "$scratch/memory-id")/mcp.json"
grep '"command":"'"$scratch/bin/mdkb"'"' "$mcp_file" >/dev/null
grep '"args":\["--root","'"$MDKB_ROOT"'","mcp","--allow-write"\]' "$mcp_file" >/dev/null
"$bin" run memory-claude --now >/dev/null
test "$(grep -c '"new"' "$FAKE_RECORD")" -eq 1
test "$(grep -c '"cat"' "$FAKE_RECORD")" -eq 2
rm "$MDKB_ROOT/agentcron/memory-claude.md"
"$bin" run memory-claude --now >/dev/null
test "$(grep -c '"new"' "$FAKE_RECORD")" -eq 2
grep 'mcp__mdkb__\*' "$FAKE_RECORD" >/dev/null
"$bin" doctor --json | grep '"kb_initialised":true' >/dev/null
"$bin" memory memory-claude --json | grep '"note":' >/dev/null
"$bin" memory memory-claude | grep 'Created by agentcron' >/dev/null
"$bin" add memory-fresh --schedule @daily --runner claude --prompt remember >/dev/null
"$bin" memory memory-fresh >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 2
grep 'no memory note yet' "$scratch/err" >/dev/null
"$bin" add memory-off --schedule @daily --runner claude --prompt hi --no-memory >/dev/null
"$bin" memory memory-off >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 2
grep 'memory is off for this job' "$scratch/err" >/dev/null
"$bin" memory unknown >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 2
"$bin" add memory-codex --schedule @daily --runner codex --prompt remember >/dev/null
"$bin" run memory-codex --now >/dev/null
grep 'mcp_servers.mdkb.command=' "$FAKE_RECORD" >/dev/null
grep 'mcp_servers.mdkb.args=' "$FAKE_RECORD" >/dev/null
grep 'AGENTCRON_MEMORY_NOTE' "$FAKE_RECORD" >/dev/null
# An isolated PATH prevents falling through to a real installed mdkb.
mkdir "$scratch/memory-path"
cp "$scratch/bin/claude" "$scratch/memory-path/claude"
PATH="$scratch/memory-path" "$bin" run memory-claude --now >/dev/null
PATH="$scratch/memory-path" "$bin" doctor --json | grep '"mdkb":false' >/dev/null
PATH="$scratch/memory-path" "$bin" memory memory-claude >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 1
grep 'mdkb not available' "$scratch/err" >/dev/null
PATH="$scratch/memory-path" "$bin" doctor --json | grep 'mdkb is missing' >/dev/null
PATH="$scratch/memory-path" "$bin" add memory-default-off --schedule @daily --runner claude --prompt hi >/dev/null
"$bin" show memory-default-off --json | grep '"memory":false' >/dev/null
"$bin" add memory-budget --schedule @daily --runner claude --prompt hi --memory --timeout 150ms >/dev/null
FAKE_MDKB_CAT_SLEEP=10s FAKE_RECORD="$scratch/memory-budget-record" non_success "$bin" run memory-budget --now
expect_state memory-budget timed_out
"$bin" runs memory-budget --limit 1 --json | grep 'setup timed out' >/dev/null
if grep '"argv":\["'"$scratch/bin/claude"'"' "$scratch/memory-budget-record"; then echo 'agent launched during expired setup'; exit 1; fi
unset FAKE_RECORD

echo "== MCP and approval"
export AGENTCRON_CONFIG_DIR="$scratch/m5-config" AGENTCRON_DATA_DIR="$scratch/m5-data"
"$bin" init >/dev/null
"$bin" add readable --schedule @daily --runner command --command cat --prompt hello >/dev/null
read_id="$("$bin" run readable --now)"
MCP_INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"accept","version":"1"}}}'
MCP_READY='{"jsonrpc":"2.0","method":"notifications/initialized"}'
mcp_exchange() {
  # EOF cancels SDK requests; leave transport open while responses are written.
  (printf '%s\n' "$MCP_INIT" "$MCP_READY" "$1"; sleep 2) | "$bin" mcp ${2-}
}
MCP_LIST='{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
for write_flag in '' --allow-write; do
  mcp_exchange "$MCP_LIST" "$write_flag" >"$scratch/mcp-list"
  tool_count="$(grep '"id":2,' "$scratch/mcp-list" | grep -o '"inputSchema"' | wc -l | tr -d ' ')"
  if test -z "$write_flag"; then test "$tool_count" = 5; else test "$tool_count" = 9; fi
done
read_calls='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_jobs","arguments":{}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_job","arguments":{"name":"readable"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_runs","arguments":{"job":"readable","limit":1}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"next_fires","arguments":{"count":1}}}'
read_calls="$read_calls
{\"jsonrpc\":\"2.0\",\"id\":6,\"method\":\"tools/call\",\"params\":{\"name\":\"get_run_output\",\"arguments\":{\"id\":\"$read_id\",\"stream\":\"stdout\"}}}"
mcp_exchange "$read_calls" >"$scratch/mcp-reads"
for response in 2 3 4 5 6; do grep "\"id\":$response," "$scratch/mcp-reads" | grep '"structuredContent"' >/dev/null; done
if grep '"isError":true' "$scratch/mcp-reads"; then exit 1; fi
create_call='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_job","arguments":{"name":"agent","schedule":"every 1m","runner":"command","command":["cat"],"prompt":"hello"}}}'
mcp_exchange "$create_call" --allow-write >"$scratch/mcp-create"
grep '"status":"unapproved"' "$scratch/mcp-create" >/dev/null
test -f "$AGENTCRON_CONFIG_DIR/jobs/agent.toml"
grep 'created_by = "mcp"' "$AGENTCRON_CONFIG_DIR/jobs/agent.toml" >/dev/null
"$bin" ls | grep 'unapproved' >/dev/null
"$bin" run agent >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 5
"$bin" run agent --now >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 5
test "$("$bin" runs agent --json)" = '[]'
"$bin" tick >/dev/null 2>"$scratch/mcp-tick-err"
test "$("$bin" runs agent --json)" = '[]'
"$bin" approve absent >"$scratch/out" 2>"$scratch/err" && exit 1 || test $? -eq 2
"$bin" approve agent >/dev/null
"$bin" approve agent >/dev/null
"$bin" run agent --now >/dev/null
expect_state agent succeeded
AGENTCRON_RUN_ID=x mcp_exchange "$create_call" --allow-write >"$scratch/mcp-guard"
grep '"isError":true' "$scratch/mcp-guard" >/dev/null
grep allow_agent_scheduling "$scratch/mcp-guard" >/dev/null
echo "accept: ok"
