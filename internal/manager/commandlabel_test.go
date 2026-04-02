package manager

import "testing"

func TestSummarizeCommandLabelKeepsSimpleSingleLineCommands(t *testing.T) {
	got := summarizeCommandLabel("$ ", "hostname")
	if got != "$ hostname" {
		t.Fatalf("summarizeCommandLabel() = %q, want %q", got, "$ hostname")
	}
}

func TestSummarizeCommandLabelSkipsShebangAndMarksMultilineCommands(t *testing.T) {
	got := summarizeCommandLabel("SSH: ", "#!/bin/sh\napt-get update\napt-get upgrade -y")
	if got != "SSH: apt-get update ..." {
		t.Fatalf("summarizeCommandLabel() = %q, want %q", got, "SSH: apt-get update ...")
	}
}
