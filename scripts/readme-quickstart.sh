#!/usr/bin/env bash
# Execute the README's exact quick start with isolated configuration and fake agents.
set -eu
cd "$(dirname "$0")/.."
scratch=$(mktemp -d)
trap 'rm -r "$scratch"' EXIT
mkdir -p "$scratch/bin" "$scratch/tmp"
# Acceptance can supply its existing binary and fake directory before HOME isolation.
if [ "$#" -eq 2 ]; then
  cp "$1" "$scratch/bin/agentcron"
  for runner in claude codex mdkb; do
    cp "$2/$runner" "$scratch/bin/$runner"
  done
elif [ "$#" -eq 0 ]; then
  go build -o "$scratch/bin/agentcron" ./cmd/agentcron
  for runner in claude codex mdkb; do
    go build -o "$scratch/bin/$runner" "./testdata/fakes/$runner"
  done
else
  echo 'Usage: readme-quickstart.sh [agentcron-binary fake-bin-directory]' >&2
  exit 1
fi
export PATH="$scratch/bin:$PATH"
for runner in agentcron claude codex mdkb; do
  test "$(command -v "$runner")" = "$scratch/bin/$runner" || exit 1
done
# All mktemp directories made by the verbatim block belong to the cleanup root.
export TMPDIR="$scratch/tmp" FAKE_MODE=ok FAKE_STDIN= FAKE_RECORD="$scratch/record"
export HOME="$scratch/home" XDG_CONFIG_HOME="$scratch/config" XDG_DATA_HOME="$scratch/data"
export XDG_CACHE_HOME="$scratch/cache" MDKB_ROOT="$scratch/kb"
export GIT_CONFIG_GLOBAL="$scratch/gitconfig" GIT_CONFIG_NOSYSTEM=1
unset AGENTCRON_CONFIG_DIR AGENTCRON_DATA_DIR AGENTCRON_RUN_ID
awk '
  /^<!-- quickstart:start -->$/ { section=1; starts++; next }
  /^<!-- quickstart:end -->$/ { section=0; ends++; next }
  section && /^```bash$/ { code=1; next }
  section && /^```$/ { code=0; next }
  section && code { print; lines++ }
  END { if (starts != 1 || ends != 1 || code || !lines) exit 1 }
' README.md > "$scratch/quickstart.sh"
bash -eu "$scratch/quickstart.sh" > "$scratch/output"
cat "$scratch/output"
grep '"state": "succeeded"' "$scratch/output" >/dev/null
grep 'Hello from agentcron' "$scratch/output" >/dev/null
grep '"mcp","add","--scope","local","agentcron","--"' "$scratch/record" >/dev/null
echo 'README quick start: ok'
