package app

import (
	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/store"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLogReadersRefuseSpecialFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo"} {
		for _, reader := range []string{"logs", "tail"} {
			t.Run(kind+"/"+reader, func(t *testing.T) {
				opts := isolated(t)
				c := clock.Real{}
				if err := Init(t.Context(), opts); err != nil {
					t.Fatal(err)
				}
				db, err := store.Open(t.Context(), filepath.Join(opts.DataDir, "agentcron.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				r, err := store.NewRun("job", "manual", c.Now())
				if err != nil {
					t.Fatal(err)
				}
				if err = db.CreateRun(t.Context(), r); err != nil {
					t.Fatal(err)
				}
				dir := filepath.Join(opts.DataDir, "runs", r.ID)
				if err = os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "stdout.log")
				if kind == "symlink" {
					secret := filepath.Join(opts.DataDir, "secret")
					if err := os.WriteFile(secret, []byte("secret"), 0600); err != nil {
						t.Fatal(err)
					}
					err = os.Symlink(secret, path)
				} else {
					err = syscall.Mkfifo(path, 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
				done := make(chan error, 1)
				go func() {
					var e error
					if reader == "logs" {
						e = Logs(t.Context(), opts, c, r.ID, false, false, io.Discard)
					} else {
						_, e = LogTail(t.Context(), opts, c, r.ID, false)
					}
					done <- e
				}()
				select {
				case err = <-done:
				case <-time.After(time.Second):
					peer, e := os.OpenFile(path, os.O_WRONLY, 0)
					if e == nil {
						peer.Close()
					}
					<-done
					t.Fatal("log reader blocked on FIFO")
				}
				if err == nil || !strings.Contains(err.Error(), "log is not a regular file") {
					t.Fatal(err)
				}
			})
		}
	}
}
