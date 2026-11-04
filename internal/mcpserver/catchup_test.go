package mcpserver_test

import (
	"github.com/ContractorKeith/agentcron/internal/app"
	"testing"
)

func TestCreateCatchup(t *testing.T) {
	cs, opts, c := fixture(t, true)
	call(t, cs, "create_job", map[string]any{"name": "job", "schedule": "@daily", "runner": "command", "command": []string{"cat"}, "prompt": "hi", "memory": false, "catchup": "none"}, false)
	detail, err := app.ShowJob(t.Context(), opts, c, "job")
	if err != nil || detail.Job.Catchup != "none" {
		t.Fatal(detail, err)
	}
}
