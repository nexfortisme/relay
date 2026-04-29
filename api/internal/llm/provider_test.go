package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexfortisme/relay/internal/tools"
)

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

func TestRepetitionDetectorCatchesRepeatedSuffix(t *testing.T) {
	detector := repetitionDetector{}
	repeated := "alpha beta gamma delta epsilon zeta eta theta. "
	if !detector.Accept(repeated) {
		t.Fatal("first span should be accepted")
	}
	if !detector.Accept(repeated) {
		t.Fatal("second span should be accepted")
	}
	if detector.Accept(repeated) {
		t.Fatal("third repeated span should be rejected")
	}
}

func TestRepetitionDetectorCatchesRepeatedParagraphBlock(t *testing.T) {
	detector := repetitionDetector{}
	firstParagraph := numberedWords("alpha", 45)
	secondParagraph := numberedWords("beta", 45)
	block := firstParagraph + "\n\n" + secondParagraph + "\n\n"

	if !detector.Accept(block) {
		t.Fatal("first paragraph block should be accepted")
	}
	if detector.Accept(block) {
		t.Fatal("second repeated paragraph block should be rejected")
	}
}

func numberedWords(prefix string, count int) string {
	words := make([]string, 0, count)
	for i := range count {
		words = append(words, fmt.Sprintf("%s%d", prefix, i))
	}
	return strings.Join(words, " ") + "."
}

func TestRepetitionDetectorAllowsNormalText(t *testing.T) {
	detector := repetitionDetector{}
	parts := []string{
		"alpha beta gamma delta epsilon zeta eta theta. ",
		"iota kappa lambda mu nu xi omicron pi. ",
		"rho sigma tau upsilon phi chi psi omega.",
	}
	for _, part := range parts {
		if !detector.Accept(part) {
			t.Fatalf("normal text was rejected at %q", part)
		}
	}
}

func TestGenerateStreamRetriesAfterRepeatedResponse(t *testing.T) {
	const loop = "alpha beta gamma delta epsilon zeta eta theta. "

	var mu sync.Mutex
	var requests []chatRequest
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/chat/completions" {
			return nil, fmt.Errorf("unexpected path %s", r.URL.Path)
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, fmt.Errorf("decode request: %w", err)
		}
		mu.Lock()
		requests = append(requests, req)
		requestNumber := len(requests)
		mu.Unlock()

		switch requestNumber {
		case 1:
			return sseResponse(sseToken(t, loop) + sseToken(t, loop) + sseToken(t, loop)), nil
		case 2:
			return sseResponse(sseToken(t, "continued without looping") + "data: [DONE]\n\n"), nil
		default:
			return nil, fmt.Errorf("unexpected request %d", requestNumber)
		}
	})

	provider := NewHTTPProvider("http://llm.test", "test-model", time.Second)
	provider.client = &http.Client{Transport: transport}
	stream := provider.GenerateStream(context.Background(), []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: ""},
	}, tools.NoopRuntime{})

	var got strings.Builder
	for event := range stream {
		if event.Err != nil {
			t.Fatalf("unexpected stream error: %v", event.Err)
		}
		got.WriteString(event.Token)
	}

	if !strings.Contains(got.String(), "continued without looping") {
		t.Fatalf("expected retry response to be streamed, got %q", got.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	if len(requests[1].Messages) < 3 {
		t.Fatalf("expected retry request to include prior messages and continuation prompt, got %d messages", len(requests[1].Messages))
	}
	if len(requests[1].Messages) != 3 {
		t.Fatalf("expected retry request to drop empty assistant placeholder, got %d messages", len(requests[1].Messages))
	}
	assistant := requests[1].Messages[len(requests[1].Messages)-2]
	if assistant.Role != "assistant" || assistant.ContentString() != loop+loop {
		t.Fatalf("expected partial assistant content in retry request, got role=%q content=%q", assistant.Role, assistant.ContentString())
	}
	prompt := requests[1].Messages[len(requests[1].Messages)-1]
	if prompt.Role != "user" || !strings.Contains(prompt.ContentString(), "without restating or repeating") {
		t.Fatalf("expected anti-repetition retry prompt, got role=%q content=%q", prompt.Role, prompt.ContentString())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func sseResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func sseToken(t *testing.T, token string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{
				"delta": map[string]string{
					"content": token,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal token: %v", err)
	}
	return fmt.Sprintf("data: %s\n\n", raw)
}
