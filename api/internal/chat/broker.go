package chat

import "sync"

// Event is one WebSocket frame sent to clients streaming a conversation.
// Type is one of: token, thinking, done, stopped, error, ping. Delta events
// (token/thinking) carry only the new fragment; terminal events (done/stopped)
// carry the full final content so clients that missed deltas can recover.
type Event struct {
	Type            string `json:"type"`
	MessageID       string `json:"messageId,omitempty"`
	Token           string `json:"token,omitempty"`
	Content         string `json:"content,omitempty"`
	Thinking        string `json:"thinking,omitempty"`
	Model           string `json:"model,omitempty"`
	Error           string `json:"error,omitempty"`
	ElapsedMs       int64  `json:"elapsedMs,omitempty"`
	InputTokens     int    `json:"inputTokens,omitempty"`
	OutputTokens    int    `json:"outputTokens,omitempty"`
	ReasoningTokens int    `json:"reasoningTokens,omitempty"`
	TotalTokens     int    `json:"totalTokens,omitempty"`
}

// Broker fans generation events out to every WebSocket subscribed to a
// conversation, decoupling the goroutine producing tokens from the handlers
// streaming them. Multiple tabs can subscribe to the same conversation.
type Broker struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{
		subs: make(map[string]map[chan Event]struct{}),
	}
}

// Subscribe registers a listener for a conversation's events and returns the
// event channel plus an unsubscribe func the caller must invoke when done.
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

// Publish sends event to all subscribers of a conversation. Delivery is
// best-effort: a subscriber whose buffer is full is skipped rather than
// blocking generation, which is safe because terminal events repeat the full
// message content.
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
