package room

// Type definitions for room users (players)
// and associated functions

import (
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lxzan/gws"
	pt "github.com/minishd/minnatropolis/api/room/protocol"
)

const (
	// Maximum amount of time we will wait
	// for new outbound messages to be queued
	// before flushing and sending all.
	// This should be the maximum amount of time
	// we can wait between broadcasts without
	// something bad happening (players rubber-banding, etc)
	sendMaxInterval = time.Second / 10

	// The amount of time we will wait for another
	// message to get queued after one is received.
	// We want it to be low (so that observable latency is low),
	// but we also want it to be high enough that the
	// likelihood of catching more queued messages is high.
	// This lets scenarios of low player-count lobbies
	// stay snappy, while larger lobbies can queue up more
	// messages into one packet (keeping packet rate low).
	sendDelay = time.Second / 20
)

// The values that clients will assume
// if they aren't specified.
//
// Used so our assumptions match theirs
// and we don't send unnecessary updates.
const (
	defaultXY     = -1
	defaultFacing = 2
	defaultSpeed  = 3

	defaultTransparency = 0
	defaultHidden       = false
	defaultSprite       = ""
	defaultSpriteIndex  = -1
	defaultSysName      = ""
)

// Data associated with a room client
type clientData struct {
	cID  int32
	name string

	outbox chan []any

	accountUUID uuid.UUID
	rank        int32
	loggedIn    bool
	badge       string
	blocklist   map[uuid.UUID]struct{}
	blocklistMu sync.RWMutex

	guardKey, guardCount uint32
	guardKeyBytes        []byte // so we don't need to recompute

	roomID int32
	x, y   int32
	facing int32
	speed  int32

	transparency int32
	hidden       bool
	sprite       string
	spriteIndex  int32
	sysName      string
	flash        *pt.Flash

	// We need to store what pictures somebody has shown,
	// so that if another player joins, we can sync them
	// those pictures
	activePictures map[int32]pt.Picture
}

// Build a list of packets that sets up our initial state.
// Sent to people when we enter a room, or when other people
// enter a room we're in, so we look how we are meant to look
// on their screen and appear at the position we're standing, etc
func (d *clientData) getIntroMessages() (msgs []any) {
	msgs = append(msgs, pt.ConnectS2C{
		ID: d.cID, UUID: d.accountUUID,
		Rank: d.rank, IsLoggedIn: d.loggedIn,
		Badge: d.badge,
	})

	if d.x != defaultXY || d.y != defaultXY {
		msgs = append(msgs, pt.MainPlayerPosS2C{ID: d.cID, X: d.x, Y: d.y})
	}
	if d.facing != defaultFacing {
		msgs = append(msgs, pt.FacingS2C{ID: d.cID, Direction: d.facing})
	}
	if d.speed != defaultSpeed {
		msgs = append(msgs, pt.SpeedS2C{ID: d.cID, Speed: d.speed})
	}
	if d.name != "" {
		msgs = append(msgs, pt.NameS2C{ID: d.cID, Name: d.name})
	}
	if d.spriteIndex != defaultSpriteIndex && d.sprite != defaultSprite {
		msgs = append(msgs, pt.SpriteS2C{ID: d.cID, Name: d.sprite, Index: d.spriteIndex})
	}
	if d.transparency != defaultTransparency {
		msgs = append(msgs, pt.TransparencyS2C{ID: d.cID, Transparency: d.transparency})
	}
	if d.hidden != defaultHidden {
		msgs = append(msgs, pt.HiddenS2C{ID: d.cID, Hidden: d.hidden})
	}
	if d.sysName != defaultSysName {
		msgs = append(msgs, pt.SysNameS2C{ID: d.cID, Name: d.sysName})
	}
	if d.flash != nil {
		msgs = append(msgs, pt.RepeatingFlashS2C{ID: d.cID, Flash: *d.flash})
	}
	for _, pic := range d.activePictures {
		msgs = append(msgs, pt.ShowPictureS2C{ID: d.cID, Picture: pic})
	}

	return
}

// Wrapper around a [gws.Conn].
type User gws.Conn

func NewUser(c *gws.Conn) *User { return (*User)(c) }

// Get underlying [gws.Conn].
func (u *User) Conn() *gws.Conn { return (*gws.Conn)(u) }

// Key that client data is stored under in
// session storage k/v
const kClientData = "cd"

// Get [clientData] associated with a connection.
func (u *User) getData() *clientData {
	cd, _ := u.Conn().Session().Load(kClientData)
	return cd.(*clientData)
}

// The message loop of a user.
// Does its best to gather many outbound messages
// into a smaller amount of large messages, which it sends.
func (u *User) sendLoop() {
	d := u.getData()

	var pending []any         // re-used buffer of pending messages
	var endMax time.Time      // the latest time current batch could end
	var endC <-chan time.Time // channel that signals end of batch

	for {
		select {
		case msgs, ok := <-d.outbox:
			// did channel close? that means user was disconnected
			if !ok {
				return
			}

			// are we in a batch?
			if endC == nil {
				// no, but we just got a packet so now we are.
				// handle start-of-batch things like picking
				// the latest time this new batch can end.
				endMax = time.Now().Add(sendMaxInterval)
			}

			// set end-channel to new end time,
			// if there's enough time left in this batch
			timeLeft := time.Until(endMax)
			if timeLeft >= sendDelay {
				endC = time.After(sendDelay)
			}

			// add to buffer
			pending = slices.Concat(pending, msgs)

		case <-endC:
			// Serialize and send
			u.SendImmediate(pending...)

			// clear slice, and end batch
			pending = pending[:0]
			endC = nil

		}
	}
}

// Serialize and send a YNO message.
func (u *User) SendImmediate(msgs ...any) {
	data := pt.Serialize(msgs...)
	u.Conn().WriteAsync(gws.OpcodeBinary, data, nil)
}

// Queue a YNO message to be sent.
func (u *User) Send(msgs ...any) {
	u.getData().outbox <- msgs
}
