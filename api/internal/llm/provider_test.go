package llm

import "testing"

func TestExtractChunkUsageExcludesReasoningTokens(t *testing.T) {
	raw := `{"choices":[],"usage":{"prompt_tokens":120,"completion_tokens":30,"total_tokens":150,"completion_tokens_details":{"reasoning_tokens":12}}}`

	_, _, _, usage, ok := extractChunk(raw)
	if !ok {
		t.Fatal("expected usage-only chunk to parse")
	}
	if usage == nil {
		t.Fatal("expected token usage")
	}
	if usage.InputTokens != 120 {
		t.Fatalf("expected input tokens 120, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 18 {
		t.Fatalf("expected output tokens 18, got %d", usage.OutputTokens)
	}
	if usage.ReasoningTokens != 12 {
		t.Fatalf("expected reasoning tokens 12, got %d", usage.ReasoningTokens)
	}
	if usage.TotalTokens != 138 {
		t.Fatalf("expected total tokens 138, got %d", usage.TotalTokens)
	}
}
