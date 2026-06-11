package llm

type apiUsage struct {
	PromptTokens            int `json:"prompt_tokens"`
	CompletionTokens        int `json:"completion_tokens"`
	TotalTokens             int `json:"total_tokens"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func (u apiUsage) tokenUsagePtr() *TokenUsage {
	usage := u.tokenUsage()
	if usage.IsZero() {
		return nil
	}
	return &usage
}

func (u apiUsage) tokenUsage() TokenUsage {
	reasoningTokens := max(0, u.CompletionTokensDetails.ReasoningTokens)
	outputTokens := max(0, u.CompletionTokens-reasoningTokens)
	totalTokens := u.PromptTokens + outputTokens
	if totalTokens == 0 && u.TotalTokens > 0 {
		totalTokens = max(0, u.TotalTokens-reasoningTokens)
	}
	return TokenUsage{
		InputTokens:     max(0, u.PromptTokens),
		OutputTokens:    outputTokens,
		ReasoningTokens: reasoningTokens,
		TotalTokens:     totalTokens,
	}
}
