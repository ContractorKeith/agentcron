package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ContractorKeith/agentcron/internal/memory"
)

func TestMemoryErrorsIncludeBoundedStderrAndRoot(t *testing.T) {
	x, _ := setup(t)
	root := filepath.Join(x.DataDir, "kb")
	t.Setenv("MDKB_ROOT", root)
	message := "permission denied: " + strings.Repeat("x", 600)
	t.Setenv("FAKE_MDKB_FAIL", message)
	c, err := memory.Resolve("", os.Getenv("MDKB_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Run(context.Background(), "cat", "agentcron/fixture")
	if err == nil || !strings.Contains(err.Error(), message[:500]) || strings.Contains(err.Error(), message[:501]) || !strings.Contains(err.Error(), root) {
		t.Fatalf("%v", err)
	}
}

func TestUninitialisedInfoSucceedsButDoctorStatusStaysFalse(t *testing.T) {
	x, _ := setup(t)
	root := filepath.Join(x.DataDir, "kb")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MDKB_ROOT", root)
	c, err := memory.Resolve("", os.Getenv("MDKB_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := c.Run(context.Background(), "info", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Initialised bool `json:"initialized"`
	}
	if err = json.Unmarshal(data, &info); err != nil || info.Initialised {
		t.Fatalf("%s %v", data, err)
	}
	health := c.Status(context.Background())
	if !health.Present || health.Initialised {
		t.Fatal(health)
	}
}
