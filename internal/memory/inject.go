package memory

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

func HasMCPConfig(args []string) bool {
	for _, arg := range args {
		if arg == "--mcp-config" || strings.HasPrefix(arg, "--mcp-config=") {
			return true
		}
	}
	return false
}

// Inject returns runner flags; command runners receive only the memory environment.
func (c Client) Inject(runner, dir string, args []string) ([]string, error) {
	switch runner {
	case "claude":
		if HasMCPConfig(args) {
			return nil, nil
		}
		server := struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}{c.Binary, c.Args("mcp", "--allow-write")}
		config := map[string]any{"mcpServers": map[string]any{"mdkb": server}}
		data, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dir, "mcp.json")
		if err = os.WriteFile(path, data, 0600); err != nil {
			return nil, err
		}
		return []string{"--mcp-config", path, "--strict-mcp-config"}, nil
	case "codex":
		// Use the TOML encoder rather than Go quoting: TOML escapes differ for controls.
		var buf bytes.Buffer
		values := struct {
			Command string   `toml:"command"`
			Args    []string `toml:"args"`
		}{c.Binary, c.Args("mcp", "--allow-write")}
		if err := toml.NewEncoder(&buf).Encode(values); err != nil {
			return nil, err
		}
		var flags []string
		for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			flags = append(flags, "-c", "mcp_servers.mdkb."+strings.Replace(line, " = ", "=", 1))
		}
		return flags, nil
	}
	return nil, nil
}

func Preamble(job string, ownConfig bool) string {
	text := "Memory note: agentcron/" + job + "\nRead it with the mdkb MCP tools before you start; append what you learned and what to do next time at the end of the run.\n"
	if ownConfig {
		text += "Using the job's own --mcp-config; agentcron did not inject mdkb MCP.\n"
	}
	return text
}
