package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestResolutionAndInjection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("MDKB_ROOT", filepath.Join(dir, "env-root"))
	t.Setenv("PATH", dir)
	binary := filepath.Join(dir, "mdkb")
	if err := os.WriteFile(binary, []byte("fake"), 0700); err != nil {
		t.Fatal(err)
	}
	c, err := Resolve("", os.Getenv("MDKB_ROOT"))
	if err != nil || c.Binary != binary || c.Root != os.Getenv("MDKB_ROOT") {
		t.Fatalf("%+v %v", c, err)
	}
	configured := filepath.Join(dir, "configured")
	if err = os.WriteFile(configured, []byte("fake"), 0700); err != nil {
		t.Fatal(err)
	}
	c, err = Resolve(configured, filepath.Join(dir, "configured-root"))
	if err != nil || c.Binary != configured || c.Root != filepath.Join(dir, "configured-root") {
		t.Fatalf("%+v %v", c, err)
	}
	c.Root = ""
	c.Binary = "/path/with \"quotes\" and \\slashes"
	flags, err := c.Inject("codex", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		MCPServers map[string]struct {
			Command string
			Args    []string
		} `toml:"mcp_servers"`
	}
	combined := ""
	for i := 0; i < len(flags); i += 2 {
		combined += flags[i+1] + "\n"
	}
	if _, err = toml.Decode(combined, &parsed); err != nil {
		t.Fatal(err)
	}
	server := parsed.MCPServers["mdkb"]
	if server.Command != c.Binary || len(server.Args) != 2 || server.Args[1] != "--allow-write" {
		t.Fatalf("%+v", server)
	}
	c, err = Resolve("", "")
	if err != nil || c.Root != "" {
		t.Fatalf("%+v %v", c, err)
	}
}
