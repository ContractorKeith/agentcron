package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIErrorBound(t *testing.T) {
	scratch := isolate(t)
	binary := filepath.Join(scratch, "agentcron")
	build(t, binary, ".")
	if out, err := testCommand(context.Background(), binary, "init").CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	out, err := testCommand(context.Background(), binary, "show", strings.Repeat("x", 8192)).CombinedOutput()
	if err == nil || len(out) > 240 {
		t.Fatalf("error=%v output bytes=%d", err, len(out))
	}
}
