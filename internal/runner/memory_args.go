package runner

import "strings"

// Merge the server allowance into existing Claude flags so repeated options
// cannot replace either the job's permissions or the injected server allowance.
func memoryPermissions(flags, args []string) ([]string, []string) {
	if len(flags) < 2 || flags[len(flags)-2] != "--allowedTools" {
		return flags, args
	}
	allowance := flags[len(flags)-1]
	result := append([]string{}, args...)
	merged := false
	for i, arg := range result {
		switch {
		case arg == "--allowedTools" || arg == "--allowed-tools":
			if i+1 < len(result) && !strings.HasPrefix(result[i+1], "-") {
				result[i+1] += "," + allowance
				merged = true
			}
		case strings.HasPrefix(arg, "--allowedTools=") || strings.HasPrefix(arg, "--allowed-tools="):
			result[i] += "," + allowance
			merged = true
		}
	}
	if merged {
		flags = flags[:len(flags)-2]
	}
	return flags, result
}
