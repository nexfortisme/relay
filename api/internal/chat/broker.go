package chat

import "sync"

type Event struct {
	Type            string `json:"type"`
	MessageID       string `json:"messageId,omitempty"`
	Token           string `json:"token,omitempty"`
	Thinking        string `json:"thinking,omitempty"`
	Error           string `json:"error,omitempty"`
	ElapsedMs       int64  `json:"elapsedMs,omitempty"`
	InputTokens     int    `json:"inputTokens,omitempty"`
	OutputTokens    int    `json:"outputTokens,omitempty"`
	ReasoningTokens int    `json:"reasoningTokens,omitempty"`
	TotalTokens     int    `json:"totalTokens,omitempty"`
}

type Broker struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{
		subs: make(map[string]map[chan Event]struct{}),
	}
}

func (b *Broker) Subscribe(conversationID string) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Event, 64)
	if b.subs[conversationID] == nil {
		b.subs[conversationID] = make(map[chan Event]struct{})
	}
	b.subs[conversationID][ch] = struct{}{}

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[conversationID], ch)
		close(ch)
		if len(b.subs[conversationID]) == 0 {
			delete(b.subs, conversationID)
		}
	}

	return ch, unsub
}

func (b *Broker) Publish(conversationID string, event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for sub := range b.subs[conversationID] {
		select {
		case sub <- event:
		default:
		}
	}
}
