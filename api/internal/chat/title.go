package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/prompts"
	"github.com/nexfortisme/relay/internal/tools"
)

func (s *Service) SuggestConversationTitle(ctx context.Context, userID, conversationID string) (string, error) {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return "", err
	}
	history, err := s.store.GetMessages(ctx, conversationID)
	if err != nil {
		return "", err
	}
	if len(history) == 0 {
		return "New chat", nil
	}
	settings := s.LoadRuntimeSettings(ctx, userID)
	provider := llm.NewHTTPProvider(settings.LLMURL, settings.LLMModel, settings.LLMAPIKey, s.responseTimeout)
	titlePrompt, err := prompts.Load(prompts.SuggestTitle)
	if err != nil {
		return "", fmt.Errorf("load title prompt: %w", err)
	}
	prompt := llm.ChatMessage{
		Role:    "system",
		Content: titlePrompt,
	}
	llmMessages := append([]llm.ChatMessage{prompt}, toLLMMessages(history)...)
	stream := provider.GenerateStream(ctx, llmMessages, tools.NoopRuntime{})
	var titleBuilder strings.Builder
	for event := range stream {
		if event.Err != nil {
			return "", event.Err
		}
		if event.Token != "" {
			titleBuilder.WriteString(event.Token)
		}
	}
	title := clampConversationTitle(titleBuilder.String())
	if title == "" {
		for _, message := range history {
			if message.Role == "user" {
				title = deriveTitle(message.Content)
				break
			}
		}
	}
	if title == "" {
		title = "New chat"
	}
	return title, nil
}

func (s *Service) ensureConversationTitle(ctx context.Context, conversationID string, firstMessage string) error {
	conversation, err := s.store.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	if conversation.Title != "New chat" {
		return nil
	}
	autoTitle := deriveTitle(firstMessage)
	return s.store.UpdateConversationTitle(ctx, conversationID, autoTitle)
}

func deriveTitle(message string) string {
	trimmed := normalizeConversationTitle(message)
	if trimmed == "" {
		return "New chat"
	}
	if len(trimmed) <= maxConversationTitleLength {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:maxConversationTitleLength]) + "..."
}

func clampConversationTitle(title string) string {
	normalized := normalizeConversationTitle(title)
	if len(normalized) <= maxConversationTitleLength {
		return normalized
	}
	return strings.TrimSpace(normalized[:maxConversationTitleLength])
}

func normalizeConversationTitle(title string) string {
	return strings.TrimSpace(strings.ReplaceAll(title, "\n", " "))
}
