#!/usr/bin/env bash
# Bash 3.2 compatible; validate every checksum before replacing the formula.
set -eu
if [ "${1:-}" = "--help" ]; then
  echo "Usage: update-formula.sh vVERSION checksums-file [formula-file]"
  echo "Validates all four local checksums before writing the formula."
  exit 0
fi
if [ "$#" -lt 2 ] || [ "$#" -gt 3 ] || ! printf '%s\n' "$1" | LC_ALL=C grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$'; then
  echo "Usage: update-formula.sh vVERSION checksums-file [formula-file]" >&2
  exit 1
fi
release_version=$1
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
formula=${3:-$script_dir/agentcron.rb}
work_dir=$(mktemp -d)
trap 'rm -r "$work_dir"' EXIT
checksums=$2
# Marker comments associate each hash with its platform; reject missing/duplicate assets.
awk -v version="$release_version" '
  NR == FNR {
    if (NF == 2) { hashes[$2] = $1; counts[$2]++ }
    next
  }
  /^  version / { print "  version \"" substr(version, 2) "\""; next }
  /sha256 / {
    platform = $NF
    asset = "agentcron_" version "_" platform ".tar.gz"
    hash = hashes[asset]
    if (counts[asset] != 1 || length(hash) != 64 || hash ~ /[^0-9a-fA-F]/) {
      print "Invalid or missing checksum: " asset > "/dev/stderr"
      failed = 1
      exit 1
    }
    sub(/sha256 "[^"]*"/, "sha256 \"" hash "\"")
    updated++
  }
  { print }
  END { if (failed || updated != 4) exit 1 }
' "$checksums" "$formula" > "$work_dir/agentcron.rb"
# Keep the existing file intact if validation fails.
cat "$work_dir/agentcron.rb" > "$formula"
echo "Updated $formula for $release_version"
