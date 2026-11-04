package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMCPConfigNeverOverwritesExistingFileOrSymlink(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
			t.Fatal(err)
		}
		config := filepath.Join(dir, "mcp.json")
		var err error
		if symlink {
			err = os.Symlink(target, config)
		} else {
			err = os.WriteFile(config, []byte("existing config"), 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
		flags, err := (Client{Binary: "/fake/mdkb"}).Inject("claude", dir, nil)
		if err == nil || len(flags) != 0 {
			t.Fatalf("replaced existing config: %v %v", flags, err)
		}
		data, err := os.ReadFile(target)
		if err != nil || string(data) != "untouched" {
			t.Fatalf("%q %v", data, err)
		}
		if !symlink {
			data, err = os.ReadFile(config)
			if err != nil || string(data) != "existing config" {
				t.Fatalf("%q %v", data, err)
			}
		}
	}
}

func TestClaudeInjectionAllowsOnlyMDKBTools(t *testing.T) {
	flags, err := (Client{Binary: "/fake/mdkb"}).Inject("claude", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 5 || flags[0] != "--mcp-config" || flags[2] != "--strict-mcp-config" || flags[3] != "--allowedTools" || flags[4] != "mcp__mdkb__*" {
		t.Fatalf("%q", flags)
	}
}

func TestCodexOwnMDKBConfigSkipsInjection(t *testing.T) {
	for _, args := range [][]string{
		{"-c", `mcp_servers.mdkb.command="custom"`},
		{"--config", `mcp_servers.mdkb={command="custom"}`},
		{`--config=mcp_servers.mdkb.args=["mcp"]`},
		{`-cmcp_servers.mdkb.command="custom"`},
		{"-c", `mcp_servers."mdkb".command="custom"`},
	} {
		flags, err := (Client{Binary: "/fake/mdkb"}).Inject("codex", t.TempDir(), args)
		if err != nil || len(flags) != 0 {
			t.Fatalf("args=%q flags=%q err=%v", args, flags, err)
		}
	}
	flags, err := (Client{Binary: "/fake/mdkb"}).Inject("codex", t.TempDir(), []string{"-c", `mcp_servers.other.command="custom"`})
	if err != nil || len(flags) != 4 {
		t.Fatalf("%q %v", flags, err)
	}
}
