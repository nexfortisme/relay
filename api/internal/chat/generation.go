package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexfortisme/relay/internal/auth"
	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/prompts"
	"github.com/nexfortisme/relay/internal/tools"
)

func (s *Service) generateAssistant(userID string, conversationID string, assistantMessageID string, messages []llm.ChatMessage, settings RuntimeSettings) {
	s.generateAssistantWithRuntime(userID, conversationID, assistantMessageID, messages, settings, s.tools)
}

func (s *Service) generateAssistantWithRuntime(userID string, conversationID string, assistantMessageID string, messages []llm.ChatMessage, settings RuntimeSettings, toolRuntime tools.Runtime) {
	ctx, cancel := context.WithCancel(auth.ContextWithUserID(context.Background(), userID))
	defer cancel()
	s.registerCancel(conversationID, cancel)
	defer s.unregisterCancel(conversationID)

	startTime := time.Now()
	provider := llm.NewHTTPProvider(settings.LLMURL, settings.LLMModel, settings.LLMAPIKey, s.responseTimeout)
	stream := provider.GenerateStream(ctx, messages, toolRuntime)
	accumulator := assistantAccumulator{}

	for event := range stream {
		if event.Err != nil {
			if errorsIsContextDone(event.Err) {
				s.finishStoppedGeneration(conversationID, assistantMessageID, startTime, &accumulator)
				return
			}
			s.finishFailedGeneration(conversationID, assistantMessageID, startTime, event.Err)
			return
		}

		s.publishGenerationDelta(conversationID, assistantMessageID, event, &accumulator)

		if event.Done {
			s.finishCompletedGeneration(ctx, conversationID, assistantMessageID, startTime, provider, messages, &accumulator)
			return
		}
	}
}

type assistantAccumulator struct {
	content  strings.Builder
	thinking strings.Builder
	usage    llm.TokenUsage
}

type assistantFinalState struct {
	content   string
	thinking  string
	elapsedMs int64
	usage     llm.TokenUsage
}

func (a *assistantAccumulator) finalContent() string {
	return strings.TrimSpace(a.content.String())
}

func (a *assistantAccumulator) finalThinking() string {
	return strings.TrimSpace(a.thinking.String())
}

func finalAssistantState(startTime time.Time, accumulator *assistantAccumulator) assistantFinalState {
	return assistantFinalState{
		content:   accumulator.finalContent(),
		thinking:  accumulator.finalThinking(),
		elapsedMs: time.Since(startTime).Milliseconds(),
		usage:     accumulator.usage,
	}
}

func (s *Service) publishGenerationDelta(conversationID string, assistantMessageID string, event llm.TokenEvent, accumulator *assistantAccumulator) {
	if event.Usage != nil {
		accumulator.usage.Add(*event.Usage)
	}
	if event.Token != "" {
		accumulator.content.WriteString(event.Token)
		s.broker.Publish(conversationID, Event{
			Type:      "token",
			MessageID: assistantMessageID,
			Token:     event.Token,
		})
	}
	if event.Thinking != "" {
		accumulator.thinking.WriteString(event.Thinking)
		s.broker.Publish(conversationID, Event{
			Type:      "thinking",
			MessageID: assistantMessageID,
			Thinking:  event.Thinking,
		})
	}
}

func (s *Service) finishStoppedGeneration(conversationID string, assistantMessageID string, startTime time.Time, accumulator *assistantAccumulator) {
	state := finalAssistantState(startTime, accumulator)
	_ = s.persistAssistantFinalState(context.Background(), assistantMessageID, state)
	s.publishAssistantFinalEvent(conversationID, assistantMessageID, "stopped", state)
}

func (s *Service) finishFailedGeneration(conversationID string, assistantMessageID string, startTime time.Time, err error) {
	elapsedMs := time.Since(startTime).Milliseconds()
	s.logger.Error("generation failed", "conversation_id", conversationID, "error", err)
	if persistErr := s.store.SetLatestUserMessageError(context.Background(), conversationID, true); persistErr != nil {
		s.logger.Error("failed to persist user message error state", "conversation_id", conversationID, "error", persistErr)
	}
	if persistErr := s.store.SetMessageElapsedMs(context.Background(), assistantMessageID, elapsedMs); persistErr != nil {
		s.logger.Error("failed to persist errored assistant elapsed", "message_id", assistantMessageID, "error", persistErr)
	}
	s.broker.Publish(conversationID, Event{
		Type:      "error",
		MessageID: assistantMessageID,
		Error:     err.Error(),
		ElapsedMs: elapsedMs,
	})
}

func (s *Service) finishCompletedGeneration(
	ctx context.Context,
	conversationID string,
	assistantMessageID string,
	startTime time.Time,
	provider llm.Provider,
	messages []llm.ChatMessage,
	accumulator *assistantAccumulator,
) {
	if accumulator.finalContent() == "" {
		s.requestFallbackAssistantResponse(ctx, conversationID, assistantMessageID, provider, messages, accumulator)
	}

	state := finalAssistantState(startTime, accumulator)
	if err := s.persistAssistantFinalState(ctx, assistantMessageID, state); err != nil {
		s.broker.Publish(conversationID, Event{
			Type:      "error",
			MessageID: assistantMessageID,
			Error:     fmt.Sprintf("failed to persist message: %v", err),
		})
		return
	}
	s.publishAssistantFinalEvent(conversationID, assistantMessageID, "done", state)
}

func (s *Service) persistAssistantFinalState(ctx context.Context, assistantMessageID string, state assistantFinalState) error {
	contentErr := s.store.SetMessageContent(ctx, assistantMessageID, state.content)
	if contentErr != nil {
		s.logger.Error("failed to persist assistant message", "message_id", assistantMessageID, "error", contentErr)
	}
	if err := s.store.SetMessageThinking(ctx, assistantMessageID, state.thinking); err != nil {
		s.logger.Error("failed to persist assistant thinking", "message_id", assistantMessageID, "error", err)
	}
	if err := s.store.SetMessageElapsedMs(ctx, assistantMessageID, state.elapsedMs); err != nil {
		s.logger.Error("failed to persist assistant elapsed", "message_id", assistantMessageID, "error", err)
	}
	if !state.usage.IsZero() {
		if err := s.store.SetMessageTokenUsage(ctx, assistantMessageID, state.usage.InputTokens, state.usage.OutputTokens, state.usage.ReasoningTokens, state.usage.TotalTokens); err != nil {
			s.logger.Error("failed to persist assistant token usage", "message_id", assistantMessageID, "error", err)
		}
	}
	return contentErr
}

func (s *Service) publishAssistantFinalEvent(conversationID string, assistantMessageID string, eventType string, state assistantFinalState) {
	s.broker.Publish(conversationID, Event{
		Type:            eventType,
		MessageID:       assistantMessageID,
		Content:         state.content,
		Thinking:        state.thinking,
		ElapsedMs:       state.elapsedMs,
		InputTokens:     state.usage.InputTokens,
		OutputTokens:    state.usage.OutputTokens,
		ReasoningTokens: state.usage.ReasoningTokens,
		TotalTokens:     state.usage.TotalTokens,
	})
}

func (s *Service) requestFallbackAssistantResponse(
	ctx context.Context,
	conversationID string,
	assistantMessageID string,
	provider llm.Provider,
	messages []llm.ChatMessage,
	accumulator *assistantAccumulator,
) {
	// Some tool-capable local models finish with only tool/thinking output.
	// Ask once more, without tools, so the UI gets a visible assistant reply.
	fallback, _ := prompts.Load(prompts.FallbackResponse)
	if fallback == "" {
		fallback = "Please provide a response. If you need more information from the user to answer, ask them directly."
	}
	followUp := append(append([]llm.ChatMessage(nil), messages...), llm.ChatMessage{
		Role:    "user",
		Content: fallback,
	})
	for event := range provider.GenerateStream(ctx, followUp, tools.NoopRuntime{}) {
		if event.Err != nil {
			return
		}
		s.publishGenerationDelta(conversationID, assistantMessageID, event, accumulator)
		if event.Done {
			return
		}
	}
}

func (s *Service) registerCancel(conversationID string, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.cancels[conversationID] = cancel
}

func (s *Service) unregisterCancel(conversationID string) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	delete(s.cancels, conversationID)
}
