package jobs

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentCreate(t *testing.T) {
	dir := isolated(t)
	start := make(chan struct{})
	results := make(chan string, 24)
	var wg sync.WaitGroup
	for i := range 24 {
		wg.Go(func() {
			<-start
			j := valid()
			j.Prompt = fmt.Sprintf("winner-%d", i)
			if err := Create(dir, j); err == nil {
				results <- j.Prompt
			} else if !errors.Is(err, os.ErrExist) {
				t.Errorf("create: %v", err)
			}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	winners := []string{}
	for prompt := range results {
		winners = append(winners, prompt)
	}
	if len(winners) != 1 {
		t.Fatalf("successful creates = %d, want 1", len(winners))
	}
	got, err := Read(dir, "morning")
	if err != nil || got.Prompt != winners[0] {
		t.Fatalf("disk=%q winner=%q err=%v", got.Prompt, winners[0], err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary files remain: %v %v", files, err)
	}
}
func TestEmptyCommandAllowed(t *testing.T) {
	dir := isolated(t)
	j, err := Parse(dir, "morning", []byte(`schedule="@daily"
runner="claude"
prompt="hi"
command=[]
`))
	if err != nil || len(j.Command) != 0 {
		t.Fatal(j, err)
	}
}
func TestRelativeRepoRejected(t *testing.T) {
	dir := isolated(t)
	for _, repo := range []string{"some/where", "~root/x", "."} {
		j := valid()
		j.Repo = repo
		if err := j.Validate(dir); err == nil || !strings.Contains(err.Error(), "repo") {
			t.Fatalf("repo=%q err=%v", repo, err)
		}
	}
}
