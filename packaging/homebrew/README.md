# Publish to Homebrew

The checked-in formula is a template for `ContractorKeith/homebrew-tap`.
`REPLACE_AFTER_RELEASE` hashes deliberately prevent installation before assets exist.

1. After macOS/Linux skeptic review and triage, tag the approved commit `v0.1.0`
   and push the tag. Wait for the release workflow to publish all four tarballs
   and `checksums.txt` on the GitHub Release.
2. Download `checksums.txt` from that release, then run
   `bash packaging/homebrew/update-formula.sh v0.1.0 checksums.txt`. The script
   validates all four entries and rewrites the version and sha256
   lines in `packaging/homebrew/agentcron.rb`. Review the diff, copy it to
   `Formula/agentcron.rb` in a checkout of the tap, and open a PR there. After merge,
   verify `brew install contractorkeith/tap/agentcron` and `brew test agentcron` on macOS
   and Linux.

The script never tags, publishes, or pushes to the tap. Pass the checksums file
as the second argument and an optional output formula path
as the third argument.
Archives include the `v` prefix: `agentcron_v0.1.0_darwin_arm64.tar.gz`.
