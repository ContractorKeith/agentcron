// seed prepares deterministic persisted fixture state for scripts/accept.sh.
// Production CLI writes deliberately refuse the --now preview flag.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ContractorKeith/agentcron/internal/app"
	"github.com/ContractorKeith/agentcron/internal/clock"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: seed <RFC3339> <paused-job>")
		os.Exit(2)
	}
	now, err := time.Parse(time.RFC3339, os.Args[1])
	if err == nil {
		err = app.PauseJob(context.Background(), app.Options{}, clock.NewFake(now), os.Args[2], true)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
