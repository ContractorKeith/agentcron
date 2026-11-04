package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemorySettingsResolveThroughConfigLayer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("MDKB_ROOT", filepath.Join(dir, "environment"))
	file := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(file, []byte("mdkb_path = '~/bin/mdkb'\nmdkb_root = '~/notes'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(Options{ConfigDir: dir, DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if c.MDKBRoot != filepath.Join(dir, "notes") {
		t.Fatalf("root not resolved: %+v", c)
	}
	if c.MDKBPath != filepath.Join(dir, "bin", "mdkb") {
		t.Fatalf("binary not resolved: %+v", c)
	}
	if !strings.Contains(Defaults, "mdkb_path = \"\"") {
		t.Fatal("default mdkb_path missing")
	}
	if err := os.WriteFile(file, []byte("mdkb_root = ''\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(Options{ConfigDir: dir, DataDir: dir})
	if err != nil || c.MDKBRoot != os.Getenv("MDKB_ROOT") {
		t.Fatalf("%+v %v", c, err)
	}
	t.Setenv("MDKB_ROOT", "")
	c, err = Load(Options{ConfigDir: dir, DataDir: dir})
	if err != nil || c.MDKBRoot != "" {
		t.Fatalf("%+v %v", c, err)
	}
}
