package service

import (
	"strings"
	"testing"
)

func TestCapturedEnvironment(t *testing.T) {
	t.Setenv("MDKB_ROOT", "/kb & tools")
	o, err := Current("/agentcron", "/bin:/tools", "/config", "/data")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"linux", "darwin"} {
		o.OS = target
		d, err := Render(o)
		if err != nil {
			t.Fatal(err)
		}
		if d.Environment["PATH"] != "/bin:/tools" || d.Environment["MDKB_ROOT"] != "/kb & tools" {
			t.Fatal(d.Environment)
		}
		if !strings.Contains(d.Unit, "MDKB_ROOT") {
			t.Error(d.Unit)
		}
	}
}
