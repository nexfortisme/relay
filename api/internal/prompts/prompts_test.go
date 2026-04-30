package prompts

import (
	"strings"
	"testing"
)

func TestLoadFindsResourceFilesFromPackageWorkingDirectory(t *testing.T) {
	got, err := Load(CiteSources)
	if err != nil {
		t.Fatalf("load cite sources prompt: %v", err)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("expected prompt content")
	}
}

func TestStripHeadingsRemovesLeadingMarkdownTitles(t *testing.T) {
	got := stripHeadings("# Title\n\n## Subtitle\n\nKeep this\n# and keep this too")
	want := "Keep this\n# and keep this too"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
