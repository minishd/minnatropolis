package chat

// Messages sent over the chat websocket as JSON

import (
	"time"

	"github.com/google/uuid"
)

// Same names as the frontend's channels
type Channel string

const (
	ChannelGlobal Channel = "Global"
	ChannelMap    Channel = "Map"
	ChannelParty  Channel = "Party"
)

type messageType string

const (
	typeMessage messageType = "Message"
	typeError   messageType = "Error"
)

// A chat message. Also sent back to whoever sent it
type messageS2C struct {
	Type     messageType
	ID       uuid.UUID
	Channel  Channel
	AuthorID uuid.UUID
	Author   string
	Content  string
	SentAt   time.Time
}

type errorS2C struct {
	Type  messageType
	Error string
}

type sendMessageC2S struct {
	Channel Channel `validate:"oneof=Global Map Party"`
	Content string  `validate:"required,max=150"` // same limit as YNO
}
