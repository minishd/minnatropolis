package chat

// Figuring out who gets a chat message

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/minishd/minnatropolis/api/web"
	"github.com/minishd/minnatropolis/datastore"
)

func (h *Handler) processMessage(u *User, channel Channel, content string) {
	d := u.getData()

	// Filter message's text
	content = h.tf.ReplaceAll(content, "♥", true)

	// Figure out who should get it
	var shouldGet func(them *clientData) bool
	if d.punishment == nil {
		// No punishment, handle normally..

		switch channel {

		case ChannelGlobal:
			shouldGet = func(them *clientData) bool { return true }

		case ChannelMap:
			roommates, ok := h.rh.GetRoommates(d.accountUUID)
			if !ok {
				// Not in-game
				u.SendError(web.ErrNotInMap)
				return
			}
			shouldGet = func(them *clientData) bool {
				_, ok := roommates[them.accountUUID]
				return ok
			}

		case ChannelParty:
			partyID, ok := d.getParty()
			if !ok {
				u.SendError(web.ErrNotInParty)
				return
			}
			shouldGet = func(them *clientData) bool {
				theirPartyID, ok := them.getParty()
				return ok && theirPartyID == partyID
			}

		default:
			// If we validated a channel,
			// we should also be handling it
			panic("unhandled chat channel")

		}
	} else {
		// There is a punishment, handle accordingly

		switch d.punishment.PunishmentType {
		// For regular mutes and bans, nobody gets it
		case datastore.PunishmentMute, datastore.PunishmentBan:
			log.Println("a muted user spoke", d.accountUUID)
			shouldGet = func(_ *clientData) bool { return false }

		// For silent ban, user gets their own message
		case datastore.PunishmentBanSilent:
			shouldGet = func(them *clientData) bool { return them.accountUUID == d.accountUUID }

		default:
			panic("unhandled punishment type")
		}
	}

	h.broadcast(d, shouldGet, messageS2C{
		Type:     typeMessage,
		ID:       uuid.Must(uuid.NewV7()),
		Channel:  channel,
		AuthorID: d.accountUUID,
		Author:   d.name,
		Content:  content,
		SentAt:   time.Now(),
	})
}

// Send to everyone who should get it, skipping blocked users
func (h *Handler) broadcast(from *clientData, shouldGet func(them *clientData) bool, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	h.usersMu.RLock()
	defer h.usersMu.RUnlock()
	for u := range h.users {
		them := u.getData()
		if !shouldGet(them) || isBlockedBetween(from, them) {
			continue
		}
		u.sendRaw(data)
	}
}
