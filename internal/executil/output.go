package executil

import "bytes"

const maxOutputBytes = 50 * 1024 * 1024 // 50 MiB

// NormalizeCapturedOutput trims command output, applies a size cap, and
// returns a friendly placeholder when the command produced nothing.
func NormalizeCapturedOutput(data []byte) string {
	return normalizeCapturedOutput(data, maxOutputBytes)
}

func normalizeCapturedOutput(data []byte, limit int) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "(no output)"
	}
	if len(trimmed) > limit {
		return string(trimmed[:limit]) + "\n[output truncated]"
	}
	return string(trimmed)
}
