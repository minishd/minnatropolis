package room

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/google/uuid"
	ee "github.com/lxzan/event_emitter"
	"github.com/lxzan/gws"
	pt "github.com/minishd/minnatropolis/tropolis/protocol"
)

// Shared handler for room websocket events
type Handler struct {
	guardPSK []byte

	em *ee.EventEmitter[int32, *User]
}

func NewHandler(guardPSK []byte) *Handler {
	return &Handler{
		guardPSK: guardPSK,

		em: ee.New[int32, *User](&ee.Config{
			BucketNum:  128,
			BucketSize: 128,
		}),
	}
}

func (h *Handler) Subscribe(s *User, topic string) {
	h.em.Subscribe(s, topic, func(msg any) {
		bc := msg.(*gws.Broadcaster)
		_ = bc.Broadcast(s.Conn())
	})
}
func (h *Handler) Publish(topic string, msg []byte) {
	bc := gws.NewBroadcaster(gws.OpcodeBinary, msg)
	defer bc.Close()
	h.em.Publish(topic, bc)
}

// Increments by 1 for each
// connection opened
var cIDCounter atomic.Int32

// In the future, this should check tokens probably
// (Either here or in [Handler.OnOpen])
func Authorize(r *http.Request, session gws.SessionStorage) bool {
	// Get room ID
	roomID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return false
	}
	if roomID < 1 || roomID > 5000 {
		return false
	}

	// Get token
	token := r.URL.Query().Get("token")

	// Set up data
	session.Store("cd", &clientData{
		cID:      cIDCounter.Add(1),
		name:     token, // Temporary
		guardKey: rand.Uint32(),
		roomID:   int32(roomID),
	})

	// Authorize connection
	return true
}

func (h *Handler) OnOpen(c *gws.Conn) {
	s := NewUser(c)
	slog.Info("open", "cID", s.GetSubscriberID())

	// Send initial packet
	d := s.GetData()
	s.Send(pt.SyncPlayerDataS2C{
		HostID: d.cID,
		Key:    d.guardKey,
		UUID:   uuid.NewString(),
	})
}

func (h *Handler) OnMessage(c *gws.Conn, msg *gws.Message) {
	defer msg.Close()

	s := NewUser(c)
	d := s.GetData()

	m := msg.Bytes()
	if len(m) < 8 {
		// Missing guard data
		return
	}

	// Verify HMAC
	// ..
	guardKeyBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(guardKeyBytes, d.guardKey)

	hash := sha1.New()
	hash.Write(h.guardPSK)
	hash.Write(guardKeyBytes)
	hash.Write(m[4:])

	if !bytes.Equal(hash.Sum(nil)[:4], m[:4]) {
		// Invalid HMAC
		slog.Warn("declined HMAC")
		return
	}

	// Verify counter
	// ..
	count := binary.BigEndian.Uint32(m[4:8])
	if count <= d.guardCount {
		// The sent count should only increase
		slog.Warn("declined count")
		return
	}
	d.guardCount = count

	// Message handling
	// ..
	msgs := pt.Deserialize(m[8:])
	for _, msg := range msgs {
		slog.Info("received", "msg", msg)
	}
}

func (h *Handler) OnClose(c *gws.Conn, err error) {
	s := NewUser(c)
	slog.Info("close", "cID", s.GetSubscriberID())

	// Remove all subscriptions
	h.em.UnSubscribeAll(s)
}

func (h *Handler) OnPing(c *gws.Conn, payload []byte) {
	// minnaengine doesn't send pings
	// but respond anyway
	_ = c.WritePong(nil)
}
func (h *Handler) OnPong(c *gws.Conn, payload []byte) {}
