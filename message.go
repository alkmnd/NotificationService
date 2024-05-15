package NotificationService

import "github.com/google/uuid"

type Message struct {
	UserId uuid.UUID `json:"user_id"`
}
