package executil

import "strings"

const maxOutputBytes = 50 * 1024 * 1024 // 50 MiB

// NormalizeCapturedOutput trims command output, applies a size cap, and
// returns a friendly placeholder when the command produced nothing.
func NormalizeCapturedOutput(data []byte) string {
	output := strings.TrimSpace(string(data))
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes] + "\n[output truncated]"
	}
	if output == "" {
		return "(no output)"
	}
	return output
}
