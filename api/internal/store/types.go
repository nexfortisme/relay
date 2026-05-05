package store

import (
	"errors"
	"time"
)

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Message struct {
	ID              string        `json:"id"`
	ConversationID  string        `json:"conversationId"`
	Role            string        `json:"role"`
	Content         string        `json:"content"`
	UserContent     string        `json:"userContent,omitempty"`
	LLMContent      string        `json:"llmContent,omitempty"`
	Attachments     []MessageFile `json:"attachments,omitempty"`
	Thinking        string        `json:"thinking,omitempty"`
	HasError        bool          `json:"hasError,omitempty"`
	ElapsedMs       int64         `json:"elapsedMs,omitempty"`
	InputTokens     int           `json:"inputTokens,omitempty"`
	OutputTokens    int           `json:"outputTokens,omitempty"`
	ReasoningTokens int           `json:"reasoningTokens,omitempty"`
	TotalTokens     int           `json:"totalTokens,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
}

// MessageFile is the lightweight projection of a file attached to a message.
// ID is empty when the file did not make it to the database (failed uploads).
type MessageFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type File struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	Data        []byte    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Session struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userId"`
	RefreshTokenHash string     `json:"-"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	RememberMe       bool       `json:"rememberMe"`
	CreatedAt        time.Time  `json:"createdAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
}

var ErrNotFound = errors.New("not found")
