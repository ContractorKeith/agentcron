package main

import (
	"os"
	"strings"
	"testing"
)

func TestAcceptanceSuccessMarkerIsLast(t *testing.T) {
	raw, err := os.ReadFile("../../scripts/accept.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := strings.TrimSpace(string(raw))
	marker := `echo "accept: ok"`
	if strings.Count(script, marker) != 1 || !strings.HasSuffix(script, marker) {
		t.Fatal("acceptance must print success exactly once, at the end")
	}
}
