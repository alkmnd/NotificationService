package service

import "github.com/google/uuid"

const (
	Notification = "Notification"
)

type Message struct {
	UserId uuid.UUID `json:"user_id"`
}
