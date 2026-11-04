#!/usr/bin/env bash
# Build and inspect the release assets without tagging or publishing. Bash 3.2.
set -eu
if [ "$#" -ne 1 ] || ! printf '%s\n' "$1" | LC_ALL=C grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$'; then
  echo 'Usage: release-dry-run.sh vVERSION' >&2
  exit 1
fi
release_tag=$1
cd "$(dirname "$0")/.."
release_root=$(pwd)
scratch=$(mktemp -d)
trap 'rm -r "$scratch"' EXIT
host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
for target_os in linux darwin; do
  for target_arch in amd64 arm64; do
    stage="$scratch/${target_os}_${target_arch}"
    mkdir "$stage"
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" go build -trimpath \
      -ldflags "-s -w -X main.version=$release_tag" -o "$stage/agentcron" ./cmd/agentcron
    cp README.md LICENSE "$stage/"
    if [ "$target_os/$target_arch" = "$host_os/$host_arch" ]; then
      reported=$("$stage/agentcron" version)
      test "$reported" = "$release_tag"
      echo "host binary: $reported"
    else
      file "$stage/agentcron"
      go version -m "$stage/agentcron" > "$stage/build-info"
      # Go omits linker flags from build info with -trimpath; inspect the embedded tag.
      LC_ALL=C grep -aF "$release_tag" "$stage/agentcron" >/dev/null
      echo "embedded tag: $release_tag"
      grep -F "GOOS=$target_os" "$stage/build-info"
      grep -F "GOARCH=$target_arch" "$stage/build-info"
    fi
    tar -czf "$scratch/agentcron_${release_tag}_${target_os}_${target_arch}.tar.gz" -C "$stage" agentcron README.md LICENSE
  done
done
cd "$scratch"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum agentcron_*.tar.gz > checksums.txt
else
  shasum -a 256 agentcron_*.tar.gz > checksums.txt
fi
cat checksums.txt
# Prove the generated names and hashes can fill the release formula, without changing it.
cp "$release_root/packaging/homebrew/agentcron.rb" "$scratch/agentcron.rb"
bash "$release_root/packaging/homebrew/update-formula.sh" "$release_tag" checksums.txt "$scratch/agentcron.rb"
echo 'release dry run: ok (four archives verified; temporary assets removed on exit)'
