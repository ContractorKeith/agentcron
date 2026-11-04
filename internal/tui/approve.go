package tui

import (
	"fmt"
	"github.com/ContractorKeith/agentcron/internal/app"
	"strings"
)

func approvalSummary(j app.Job) string {
	prompt, _, _ := strings.Cut(plain(j.Prompt), "\n")
	return fmt.Sprintf("runner=%s | command=%q args=%q | repo=%s | prompt=%s", j.Runner, j.Command, j.Args, j.Repo, prompt)
}
