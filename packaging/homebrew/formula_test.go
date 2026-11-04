package homebrew

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFormula(t *testing.T) {
	raw, err := os.ReadFile("agentcron.rb")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	formula := filepath.Join(dir, "agentcron.rb")
	sums := filepath.Join(dir, "checksums.txt")
	var checksums strings.Builder
	for n, platform := range []string{"darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64"} {
		checksums.WriteString(strings.Repeat(string(rune('a'+n)), 64) + "  agentcron_v0.2.0_" + platform + ".tar.gz\n")
	}
	run := func(data string, wantOK bool) []byte {
		t.Helper()
		if err := os.WriteFile(formula, raw, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sums, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("bash", "update-formula.sh", "v0.2.0", sums, formula).CombinedOutput()
		if (err == nil) != wantOK {
			t.Fatalf("%s: %v", out, err)
		}
		got, err := os.ReadFile(formula)
		if err != nil {
			t.Fatal(err)
		}
		if !wantOK && !bytes.Equal(got, raw) {
			t.Fatal("failed update changed formula")
		}
		return got
	}
	got := string(run(checksums.String(), true))
	if !strings.Contains(got, `version "0.2.0"`) || strings.Contains(got, "REPLACE_AFTER_RELEASE") {
		t.Fatal(got)
	}
	for n, platform := range []string{"darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64"} {
		if !strings.Contains(got, `sha256 "`+strings.Repeat(string(rune('a'+n)), 64)+`" # `+platform) {
			t.Fatal(got)
		}
	}
	run("", false)
	run(strings.Join(strings.Split(checksums.String(), "\n")[1:], "\n"), false)
	run(strings.Replace(checksums.String(), "agentcron_v0.2.0_linux_amd64", "agentcron_v0.1.0_linux_amd64", 1), false)
	run(checksums.String()+strings.Split(checksums.String(), "\n")[0]+"\n", false)
	run(strings.ReplaceAll(checksums.String(), "aaaa", "xxxx"), false)
}
