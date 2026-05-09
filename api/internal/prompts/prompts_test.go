package prompts

import (
	"strings"
	"testing"
)

func TestLoadFindsResourceFilesFromPackageWorkingDirectory(t *testing.T) {
	keys := []Key{
		CiteSources,
		SuggestTitle,
		FallbackResponse,
		UnableToFind,
		AttachmentImagesHeader,
		AttachmentImagesSkipped,
		AttachmentDocsInline,
		AttachmentDocsRAG,
		NotebookPDFPageImage,
		FeedSummaryExpanded,
		FeedSummaryPreview,
		FeedSummaryUser,
		FeedNameSystem,
		FeedNameUser,
		RepetitionRetry,
	}
	for _, key := range keys {
		got, err := Load(key)
		if err != nil {
			t.Fatalf("load %s prompt: %v", key, err)
		}
		if strings.TrimSpace(got) == "" {
			t.Fatalf("expected %s prompt content", key)
		}
	}
}

func TestLoadNotebookPDFPageImagePrompt(t *testing.T) {
	got, err := Load(NotebookPDFPageImage)
	if err != nil {
		t.Fatalf("load notebook pdf page image prompt: %v", err)
	}
	if got != "Describe what is shown in this PDF page image. Be concise and factual." {
		t.Fatalf("unexpected prompt content: %q", got)
	}
}

func TestStripHeadingsRemovesLeadingMarkdownTitles(t *testing.T) {
	got := stripHeadings("# Title\n\n## Subtitle\n\nKeep this\n# and keep this too")
	want := "Keep this\n# and keep this too"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
