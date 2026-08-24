package room

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/lxzan/gws"
	"github.com/minishd/minnatropolis/api/room/emitter"
	pt "github.com/minishd/minnatropolis/api/room/protocol"
	"github.com/minishd/minnatropolis/datastore"
)

// Shared handler for room websocket events
type Handler struct {
	guardPSK []byte

	ds *datastore.DataStore
	em *emitter.Emitter[int32, *User]
}

func NewHandler(ds *datastore.DataStore, guardPSK []byte) *Handler {
	return &Handler{
		guardPSK: guardPSK,

		ds: ds,
		em: emitter.New[int32, *User](),
	}
}

// Wrapper for pub/sub raw websocket message.
type topicMessage struct {
	excludeID int32
	bc        *gws.Broadcaster
}

// Add a client to a websocket message topic.
func (h *Handler) subscribeRawTopic(s *User, topic string) {
	h.em.MakeSub(s, topic, func(msg any) {
		tm := msg.(*topicMessage)

		// Don't send if excluded
		if s.GetSubscriberID() == tm.excludeID {
			return
		}

		// Send the message
		_ = tm.bc.Broadcast(s.Conn(), nil)
	})
}

// Publish a websocket message to all clients of a topic.
func (h *Handler) publishRawTopic(topic string, msg []byte, excludeID int32) {
	// Set up broadcast wrapper
	bc := gws.NewBroadcaster(gws.OpcodeBinary, msg)
	tm := &topicMessage{
		excludeID: excludeID,
		bc:        bc,
	}
	defer bc.Close()

	// Publish
	h.em.Publish(topic, tm)
}

// Increments by 1 for each
// connection opened
var cIDCounter atomic.Int32

// The values that clients will assume
// if they aren't specified.
//
// Used so our assumptions match theirs
// and we don't send unnecessary updates.
const (
	defaultXY     = -1
	defaultFacing = 2
	defaultSpeed  = 4

	defaultTransparency = 0
	defaultHidden       = false
	defaultSprite       = ""
	defaultSpriteIndex  = -1
	defaultSysName      = ""
)

func (h *Handler) Authorize(r *http.Request, session gws.SessionStorage) bool {
	// Get room ID
	roomID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return false
	}
	if !isValidRoomID(int32(roomID)) {
		return false
	}

	// Get token
	token := r.URL.Query().Get("token")

	// Look up token
	ctx := r.Context()
	st, _ := h.ds.LookupSessionToken(ctx, token)

	// Get player fields
	username := "" // set it empty, guests are name-less.
	accountUUID := uuid.NewString()
	loggedIn := false

	if st != nil {
		username = st.ForUser.Username
		accountUUID = st.ForUser.ID.String()
		loggedIn = true
	}

	// Make guard key
	guardKey := rand.Uint32()
	guardKeyBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(guardKeyBytes, guardKey)

	// Set up data
	session.Store("cd", &clientData{
		cID:           cIDCounter.Add(1),
		name:          username,
		accountUUID:   accountUUID,
		loggedIn:      loggedIn,
		guardKey:      guardKey,
		guardKeyBytes: guardKeyBytes,

		roomID: int32(roomID),
		x:      defaultXY, y: defaultXY,
		facing: defaultFacing,
		speed:  defaultSpeed,

		transparency: defaultTransparency,
		hidden:       defaultHidden,
		sprite:       defaultSprite,
		spriteIndex:  defaultSpriteIndex,
		sysName:      defaultSysName,
	})

	// Authorize connection
	return true
}

func roomTopic(roomID int32) string {
	return "room-" + strconv.FormatInt(int64(roomID), 10)
}

// Send a message to everyone else in the room.
func (h *Handler) shareToRoom(d *clientData, msgs ...any) {
	msgsBytes := pt.Serialize(msgs...)
	h.publishRawTopic(roomTopic(d.roomID), msgsBytes, d.cID)
}

// Change from one room to another.
func (h *Handler) changeRoom(u *User, newID int32) {
	d := u.getData()

	// If the two rooms are different,
	// we need to handle leaving the other room
	if newID != d.roomID {
		// Unsubscribe from the topic
		h.em.RemoveSub(u, roomTopic(d.roomID))
		// Tell other players we left
		h.shareToRoom(d, pt.DisconnectS2C{ID: d.cID})
	}

	// Introduce to new room
	d.roomID = newID
	u.Send(pt.RoomInfoS2C{RoomID: d.roomID})
	topic := roomTopic(d.roomID)
	h.subscribeRawTopic(u, topic)

	// Tell us that everyone is here
	var introMsgs []any
	for _, o := range h.em.GetSubs(topic) {
		if o.GetSubscriberID() == d.cID {
			continue
		}
		introMsgs = append(introMsgs, o.GetIntroMessages()...)
	}
	u.Send(introMsgs...)

	// Tell everyone else we're here
	h.shareToRoom(d, u.GetIntroMessages()...)
}

func (h *Handler) OnOpen(c *gws.Conn) {
	s := NewUser(c)
	log.Println("open cID=", s.GetSubscriberID())

	// Send initial packet
	d := s.getData()
	s.Send(pt.SyncPlayerDataS2C{
		HostID:     d.cID,
		Key:        d.guardKey,
		UUID:       d.accountUUID,
		Rank:       d.rank,
		IsLoggedIn: d.loggedIn,
		Badge:      d.badge,
	})

	// Add to room
	h.changeRoom(s, d.roomID)
}

func (h *Handler) processMessage(u *User, m any) (err error) {
	d := u.getData()

	switch m := m.(type) {

	case pt.SwitchRoomC2S:
		log.Println("change to room", m.RoomID)
		h.changeRoom(u, m.RoomID)

	case pt.MainPlayerPosC2S:
		d.x = m.X
		d.y = m.Y
		h.shareToRoom(d, pt.MainPlayerPosS2C{ID: d.cID, X: d.x, Y: d.y})

	case pt.SpeedC2S:
		d.speed = m.Speed
		h.shareToRoom(d, pt.SpeedS2C{ID: d.cID, Speed: d.speed})

	case pt.SpriteC2S:
		d.sprite = m.Name
		d.spriteIndex = m.Index
		h.shareToRoom(d, pt.SpriteS2C{ID: d.cID, Name: d.sprite, Index: d.spriteIndex})

	case pt.FacingC2S:
		d.facing = m.Direction
		h.shareToRoom(d, pt.FacingS2C{ID: d.cID, Direction: d.facing})

	case pt.HiddenC2S:
		d.hidden = m.Hidden
		h.shareToRoom(d, pt.HiddenS2C{ID: d.cID, Hidden: d.hidden})

	case pt.SysNameC2S:
		d.sysName = m.Name
		h.shareToRoom(d, pt.SysNameS2C{ID: d.cID, Name: d.sysName})

	case pt.TransparencyC2S:
		d.transparency = m.Transparency
		h.shareToRoom(d, pt.TransparencyS2C{ID: d.cID, Transparency: d.transparency})

	case pt.SoundEffectC2S:
		h.shareToRoom(d, pt.SoundEffectS2C{ID: d.cID, Name: m.Name, Volume: m.Volume, Tempo: m.Tempo, Balance: m.Balance})

	default:
		err = fmt.Errorf("unhandled msg %+v", m)
	}

	return
}

func (h *Handler) OnMessage(c *gws.Conn, msg *gws.Message) {
	defer msg.Close()

	s := NewUser(c)
	d := s.getData()

	m := msg.Bytes()
	if len(m) < 8 {
		// Missing guard data
		return
	}

	// Verify HMAC
	// ..
	hash := sha1.New()
	hash.Write(h.guardPSK)
	hash.Write(d.guardKeyBytes)
	hash.Write(m[4:])
	if !bytes.Equal(hash.Sum(nil)[:4], m[:4]) {
		// Invalid HMAC
		return
	}

	// Verify counter
	// ..
	count := binary.BigEndian.Uint32(m[4:8])
	if count <= d.guardCount {
		// The sent count should only increase
		log.Println("declined count")
		return
	}
	d.guardCount = count

	// Message handling
	// ..
	msgs, err := pt.Deserialize(m[8:])
	if err != nil {
		log.Println("invalid packet")
		return
	}

	// Validate everything first so a packet is applied entirely
	// or not at all since a legitimate client will never send an
	// invalid packet.
	for _, msg := range msgs {
		if err := h.validateMessage(msg); err != nil {
			log.Println("invalid message:", err)
			return
		}
	}

	for _, msg := range msgs {
		h.processMessage(s, msg)
	}
}

func (h *Handler) OnClose(c *gws.Conn, err error) {
	s := NewUser(c)
	log.Println("close cID=", s.GetSubscriberID())

	// Leave room
	d := s.getData()
	h.shareToRoom(d, pt.DisconnectS2C{ID: d.cID})

	// Remove all subscriptions
	h.em.DestroySub(s)
}

func (h *Handler) OnPing(c *gws.Conn, payload []byte) {
	// minnaengine doesn't send pings
	// but respond anyway
	_ = c.WritePong(nil)
}
func (h *Handler) OnPong(c *gws.Conn, payload []byte) {}
