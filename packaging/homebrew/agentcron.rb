# frozen_string_literal: true

# Local scheduler for recurring agent work.
class Agentcron < Formula
  desc "Local agent scheduler with CLI, daemon, MCP server and TUI"
  homepage "https://github.com/ContractorKeith/agentcron"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/ContractorKeith/agentcron/releases/download/v#{version}/agentcron_v#{version}_darwin_arm64.tar.gz"
      sha256 "REPLACE_AFTER_RELEASE" # darwin_arm64
    end
    on_intel do
      url "https://github.com/ContractorKeith/agentcron/releases/download/v#{version}/agentcron_v#{version}_darwin_amd64.tar.gz"
      sha256 "REPLACE_AFTER_RELEASE" # darwin_amd64
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/ContractorKeith/agentcron/releases/download/v#{version}/agentcron_v#{version}_linux_arm64.tar.gz"
      sha256 "REPLACE_AFTER_RELEASE" # linux_arm64
    end
    on_intel do
      url "https://github.com/ContractorKeith/agentcron/releases/download/v#{version}/agentcron_v#{version}_linux_amd64.tar.gz"
      sha256 "REPLACE_AFTER_RELEASE" # linux_amd64
    end
  end

  def install
    bin.install "agentcron"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/agentcron version")
    assert_match "A local scheduler", shell_output("#{bin}/agentcron --help")
  end
end
