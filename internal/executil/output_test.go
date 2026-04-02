package executil

import (
	"bytes"
	"strings"
	"testing"
)

func TestNormalizeCapturedOutputReturnsPlaceholderForEmptyText(t *testing.T) {
	got := NormalizeCapturedOutput([]byte(" \n\t "))
	if got != "(no output)" {
		t.Fatalf("NormalizeCapturedOutput() = %q, want %q", got, "(no output)")
	}
}

func TestNormalizeCapturedOutputTrimsOuterWhitespace(t *testing.T) {
	got := NormalizeCapturedOutput([]byte("\n  hello world  \n"))
	if got != "hello world" {
		t.Fatalf("NormalizeCapturedOutput() = %q, want %q", got, "hello world")
	}
}

func TestNormalizeCapturedOutputTruncatesAfterLimit(t *testing.T) {
	limit := 16
	input := append(bytes.Repeat([]byte("a"), limit+4), '\n')

	got := normalizeCapturedOutput(input, limit)
	want := strings.Repeat("a", limit) + "\n[output truncated]"
	if got != want {
		t.Fatalf("normalizeCapturedOutput() = %q, want %q", got, want)
	}
}
