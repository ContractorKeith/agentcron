package service

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeCommander struct {
	calls  [][]string
	output string
	loaded bool
}

func (f *fakeCommander) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if name == "launchctl" {
		switch args[0] {
		case "bootout":
			f.loaded = false
		case "bootstrap":
			f.loaded = true
		case "print":
			if !f.loaded {
				return "Could not find service", errors.New("not found")
			}
		}
	}
	return f.output, nil
}
func TestServiceLifecycle(t *testing.T) {
	for _, target := range []string{"linux", "darwin"} {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			o := Options{OS: target, Home: home, Binary: "/opt/my tools/agentcron", Path: "/bin:/a&b", ConfigDir: "/config path", DataDir: filepath.Join(home, "data"), UID: 501}
			f := &fakeCommander{output: "active\n"}
			ctx := context.Background()
			d, err := Apply(ctx, o, "install", true, f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(d.Path); !os.IsNotExist(err) || len(f.calls) != 0 {
				t.Fatal("dry run mutated state")
			}
			if target == "linux" {
				if !strings.Contains(d.Unit, `ExecStart="/opt/my tools/agentcron" "--config" "/config path"`) {
					t.Fatal(d.Unit)
				}
			} else {
				dec := xml.NewDecoder(strings.NewReader(d.Unit))
				for {
					_, err := dec.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				if !strings.Contains(d.Unit, "/a&amp;b") {
					t.Fatal(d.Unit)
				}
			}
			_, err = Apply(ctx, o, "install", false, f)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(f.calls, o.Commands("install")) {
				t.Fatal(f.calls)
			}
			if target == "darwin" {
				f.output = "state = running\n"
			}
			state, err := Status(ctx, o, f)
			if err != nil || !state.Installed || !state.Active {
				t.Fatal(state, err)
			}
			_, err = Apply(ctx, o, "uninstall", false, f)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(d.Path); !os.IsNotExist(err) {
				t.Fatal(err)
			}
		})
	}
}
