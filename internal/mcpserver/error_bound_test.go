package mcpserver_test

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"strings"
	"testing"
)

func TestMCPErrorBound(t *testing.T) {
	cs, _, _ := fixture(t, false)
	r := call(t, cs, "get_job", map[string]any{"name": strings.Repeat("x", 8192)}, true)
	text := r.Content[0].(*mcp.TextContent).Text
	if len(text) > 200 {
		t.Fatalf("error bytes=%d", len(text))
	}
}
