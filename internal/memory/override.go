package memory

import (
	"strings"

	"github.com/BurntSushi/toml"
)

// Override names the job's MCP configuration that agentcron must preserve.
func Override(runner string, args []string) string {
	if runner == "claude" && HasMCPConfig(args) {
		return "--mcp-config"
	}
	if runner != "codex" {
		return ""
	}
	for i, arg := range args {
		value := ""
		switch {
		case arg == "-c" || arg == "--config":
			if i+1 < len(args) {
				value = args[i+1]
			}
		case strings.HasPrefix(arg, "--config="):
			value = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-c"):
			value = strings.TrimPrefix(strings.TrimPrefix(arg, "-c"), "=")
		}
		var settings map[string]any
		meta, err := toml.Decode(value, &settings)
		if err == nil && meta.IsDefined("mcp_servers", "mdkb") {
			return "mcp_servers.mdkb"
		}
	}
	return ""
}
