package runner

import (
	"fmt"
	"github.com/ContractorKeith/agentcron/internal/testutil"
	"os"
	"testing"
)

var fakeBin string

func TestMain(m *testing.M) {
	if dir := os.Getenv("TEST_AGENTCRON_FAKES"); dir != "" {
		fakeBin = dir
		os.Exit(m.Run())
	}
	dir, cleanup, err := testutil.Fakes()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fakeBin = dir
	_ = os.Setenv("TEST_AGENTCRON_FAKES", dir)
	code := m.Run()
	cleanup()
	os.Exit(code)
}
