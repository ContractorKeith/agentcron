package jobs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolated(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME"} {
		t.Setenv(key, dir)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "gitconfig"))
	return dir
}
func valid() Job {
	j := Defaults("morning")
	j.Schedule = "@daily"
	j.Runner = "claude"
	j.Prompt = "hi"
	return j
}
func TestInvalidFields(t *testing.T) {
	dir := isolated(t)
	for _, tc := range []struct {
		field  string
		change func(*Job)
	}{
		{"name", func(j *Job) { j.Name = "../escape" }},
		{"name", func(j *Job) { j.Name = strings.Repeat("a", 64) }},
		{"schedule", func(j *Job) { j.Schedule = "every 30s" }},
		{"runner", func(j *Job) { j.Runner = "other" }},
		{"prompt", func(j *Job) { j.Prompt = "" }},
		{"prompt", func(j *Job) { j.PromptFile = "p.md" }},
		{"command", func(j *Job) { j.Runner = "command" }},
		{"command", func(j *Job) { j.Command = []string{"cat"} }},
		{"command", func(j *Job) { j.Runner = "command"; j.Command = []string{""} }},
		{"timeout", func(j *Job) { j.Timeout = "0s" }},
		{"timeout", func(j *Job) { j.Timeout = "-1m" }},
		{"timeout", func(j *Job) { j.Timeout = "bad" }},
		{"expect", func(j *Job) { j.Expect = "[" }},
		{"timezone", func(j *Job) { j.Timezone = "Mars/Olympus" }},
		{"catchup", func(j *Job) { j.Catchup = "all" }},
		{"base", func(j *Job) { j.Base = "main" }},
		{"created_by", func(j *Job) { j.CreatedBy = "other" }},
	} {
		t.Run(tc.field, func(t *testing.T) {
			j := valid()
			tc.change(&j)
			if err := j.Validate(dir); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := Parse(dir, "morning", []byte("typo = true")); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatal(err)
	}
	j := valid()
	j.Repo = "~/repo"
	if err := j.Validate(dir); err != nil || j.Repo != filepath.Join(dir, "repo") {
		t.Fatal(j, err)
	}
}
func TestParsedRepoExpansion(t *testing.T) {
	dir := isolated(t)
	j, err := Parse(dir, "morning", []byte(`schedule="@daily"
runner="claude"
prompt="hi"
repo="~/repo"
`))
	if err != nil || j.Repo != filepath.Join(dir, "repo") {
		t.Fatal(j, err)
	}
}

func TestPromptContainment(t *testing.T) {
	root := isolated(t)
	dir := filepath.Join(root, "jobs")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.md")
	if err := os.WriteFile(outside, []byte("hi"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../outside.md", "link.md", outside, "missing.md", "."} {
		j := valid()
		j.Prompt = ""
		j.PromptFile = path
		if err := j.Validate(dir); err == nil || !strings.Contains(err.Error(), "prompt_file") {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "inside.md"), []byte("hi"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dir, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	j := valid()
	j.Prompt = ""
	j.PromptFile = "inside.md"
	if err := j.Validate(filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
}
func TestFiles(t *testing.T) {
	dir := isolated(t)
	j := valid()
	for range 2 {
		if err := Write(dir, j); err != nil {
			t.Fatal(err)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 || files[0].Name() != "morning.toml" {
		t.Fatal(files, err)
	}
	got, err := Read(dir, "morning")
	if err != nil || got.Prompt != "hi" {
		t.Fatal(got, err)
	}
	for name, data := range map[string]string{"bad.toml": "timeout = 4", "ignored.txt": "ignored"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := List(dir)
	if err != nil || len(entries) != 2 || entries[0].Error == "" || entries[1].Job == nil {
		t.Fatal(entries, err)
	}
	if err := Remove(dir, "morning"); err != nil {
		t.Fatal(err)
	}
}
