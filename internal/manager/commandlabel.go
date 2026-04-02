package manager

import (
	"strings"
	"unicode/utf8"
)

const commandLabelLimit = 60

func summarizeCommandLabel(prefix, command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return strings.TrimSpace(prefix)
	}

	lines := strings.Split(trimmed, "\n")
	summary := ""
	nonEmptyLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmptyLines++
		if summary == "" && !strings.HasPrefix(line, "#!") {
			summary = strings.Join(strings.Fields(line), " ")
		}
	}
	if summary == "" {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			summary = strings.Join(strings.Fields(line), " ")
			break
		}
	}
	if summary == "" {
		return strings.TrimSpace(prefix)
	}

	wasTruncated := false
	if utf8.RuneCountInString(summary) > commandLabelLimit {
		runes := []rune(summary)
		summary = strings.TrimSpace(string(runes[:commandLabelLimit]))
		wasTruncated = true
	}
	if nonEmptyLines > 1 || wasTruncated {
		summary += " ..."
	}

	return prefix + summary
}
