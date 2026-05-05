package llm

type TokenUsage struct {
	InputTokens     int `json:"inputTokens,omitempty"`
	OutputTokens    int `json:"outputTokens,omitempty"`
	ReasoningTokens int `json:"reasoningTokens,omitempty"`
	TotalTokens     int `json:"totalTokens,omitempty"`
}

func (u TokenUsage) IsZero() bool {
	return u.InputTokens == 0 && u.OutputTokens == 0 && u.ReasoningTokens == 0 && u.TotalTokens == 0
}

func (u *TokenUsage) Add(other TokenUsage) {
	if other.IsZero() {
		return
	}
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.ReasoningTokens += other.ReasoningTokens
	u.TotalTokens += other.TotalTokens
}
