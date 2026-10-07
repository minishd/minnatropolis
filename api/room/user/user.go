package user

// Type definitions for room users (players)
// and associated functions

import (
	"encoding/binary"
	"math/rand/v2"
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

// Key that client data is stored under in
// session storage k/v
const kClientData = "cd"

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
type ClientData struct {
	CID  int32
	Name string

	outbox chan []any

	AccountUUID uuid.UUID
	Rank        int32
	LoggedIn    bool
	Badge       string
	Blocklist   map[uuid.UUID]struct{}
	BlocklistMu sync.RWMutex

	GuardKey, GuardCount uint32
	GuardKeyBytes        []byte // so we don't need to recompute

	RoomID int32
	X, Y   int32
	Facing int32
	Speed  int32

	Transparency int32
	Hidden       bool
	Sprite       string
	SpriteIndex  int32
	SysName      string
	Flash        *pt.Flash

	// We need to store what pictures somebody has shown,
	// so that if another player joins, we can sync them
	// those pictures
	ActivePictures map[int32]*pt.Picture
}

func InitData(
	session gws.SessionStorage, cID int32, username string, roomID int32,
	accountUUID uuid.UUID, loggedIn bool, blocklist map[uuid.UUID]struct{},
) {
	// Make guard key
	guardKey := rand.Uint32()
	guardKeyBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(guardKeyBytes, guardKey)

	// Make outbox, etc..
	// [User.SendLoop] has its own buffer,
	// we don't allocate one here..
	outbox := make(chan []any)
	activePictures := make(map[int32]*pt.Picture)

	cd := &ClientData{
		CID:           cID,
		Name:          username,
		outbox:        outbox,
		AccountUUID:   accountUUID,
		LoggedIn:      loggedIn,
		Blocklist:     blocklist,
		GuardKey:      guardKey,
		GuardKeyBytes: guardKeyBytes,

		RoomID: roomID,
		X:      defaultXY, Y: defaultXY,
		Facing: defaultFacing,
		Speed:  defaultSpeed,

		Transparency: defaultTransparency,
		Hidden:       defaultHidden,
		Sprite:       defaultSprite,
		SpriteIndex:  defaultSpriteIndex,
		SysName:      defaultSysName,

		ActivePictures: activePictures,
	}

	session.Store(kClientData, cd)
}

// Build a list of packets that sets up our initial state.
// Sent to people when we enter a room, or when other people
// enter a room we're in, so we look how we are meant to look
// on their screen and appear at the position we're standing, etc
func (d *ClientData) GetIntroMessages() (msgs []any) {
	msgs = append(msgs, pt.ConnectS2C{
		ID: d.CID, UUID: d.AccountUUID,
		Rank: d.Rank, IsLoggedIn: d.LoggedIn,
		Badge: d.Badge,
	})

	if d.X != defaultXY || d.Y != defaultXY {
		msgs = append(msgs, pt.MainPlayerPosS2C{ID: d.CID, X: d.X, Y: d.Y})
	}
	if d.Facing != defaultFacing {
		msgs = append(msgs, pt.FacingS2C{ID: d.CID, Direction: d.Facing})
	}
	if d.Speed != defaultSpeed {
		msgs = append(msgs, pt.SpeedS2C{ID: d.CID, Speed: d.Speed})
	}
	if d.Name != "" {
		msgs = append(msgs, pt.NameS2C{ID: d.CID, Name: d.Name})
	}
	if d.SpriteIndex != defaultSpriteIndex && d.Sprite != defaultSprite {
		msgs = append(msgs, pt.SpriteS2C{ID: d.CID, Name: d.Sprite, Index: d.SpriteIndex})
	}
	if d.Transparency != defaultTransparency {
		msgs = append(msgs, pt.TransparencyS2C{ID: d.CID, Transparency: d.Transparency})
	}
	if d.Hidden != defaultHidden {
		msgs = append(msgs, pt.HiddenS2C{ID: d.CID, Hidden: d.Hidden})
	}
	if d.SysName != defaultSysName {
		msgs = append(msgs, pt.SysNameS2C{ID: d.CID, Name: d.SysName})
	}
	if d.Flash != nil {
		msgs = append(msgs, pt.RepeatingFlashS2C{ID: d.CID, Flash: d.Flash})
	}
	for _, pic := range d.ActivePictures {
		msgs = append(msgs, pt.ShowPictureS2C{ID: d.CID, Picture: pic})
	}

	return
}

// Wrapper around a [gws.Conn].
type User gws.Conn

func New(c *gws.Conn) *User { return (*User)(c) }

// Get underlying [gws.Conn].
func (u *User) Conn() *gws.Conn { return (*gws.Conn)(u) }

// Get [ClientData] associated with a connection.
func (u *User) GetData() *ClientData {
	cd, _ := u.Conn().Session().Load(kClientData)
	return cd.(*ClientData)
}

// The message loop of a user.
// Does its best to gather many outbound messages
// into a smaller amount of large messages, which it sends.
func (u *User) SendLoop() {
	d := u.GetData()

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
	u.GetData().outbox <- msgs
}

// Closes a user's outbox.
// This also ends [User.SendLoop].
func (d *ClientData) OnClose() {
	close(d.outbox)
}
