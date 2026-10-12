package app

import (
	"time"
)

// ChangeMessage is retained in its private proposal conversation.
type ChangeMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
