package app

import (
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/daemon"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestEnqueueWakesDaemon(t *testing.T) {
	opts := isolated(t)
	c := clock.Real{}
	if err := Init(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	j := NewJob("job")
	j.Schedule = "@daily"
	j.Runner = "command"
	j.Command = []string{"cat"}
	j.Prompt = "hi"
	j.Memory = false
	if _, err := AddJob(t.Context(), opts, c, j, false); err != nil {
		t.Fatal(err)
	}
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	lease, err := daemon.Lock(opts.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if _, err := RunJob(t.Context(), opts, c, j.Name, false, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-hup:
	case <-time.After(time.Second):
		t.Fatal("enqueue did not wake lock holder")
	}
}
