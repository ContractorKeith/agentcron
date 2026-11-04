package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type noBus struct{}

func (noBus) Run(context.Context, string, ...string) (string, error) {
	return "Failed to connect to user bus", errors.New("no user bus")
}
func TestLinuxUninstallRemovesUnitWithoutBus(t *testing.T) {
	o := Options{OS: "linux", Home: t.TempDir()}
	if err := os.MkdirAll(filepath.Dir(o.file()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(o.file(), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	d, err := Apply(t.Context(), o, "uninstall", false, noBus{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(o.file()); !os.IsNotExist(err) {
		t.Fatal("unit remains", err)
	}
	// Warnings must be visible in the command's normal JSON output.
	raw, _ := json.Marshal(d)
	if !strings.Contains(string(raw), "no user bus") {
		t.Fatal(string(raw))
	}
}
