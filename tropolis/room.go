package tropolis

import (
	"log/slog"
	"sync/atomic"

	"github.com/google/uuid"
	ee "github.com/lxzan/event_emitter"
	"github.com/lxzan/gws"
)

// Data associated with a room client
type clientData struct {
	cID  int32
	name string

	guardKey string

	mapID  int32
	x, y   int32
	facing int32
}

// Implements [ee] subscriber interface onto
// our websocket connections
type Sub gws.Conn

func NewSub(c *gws.Conn) *Sub { return (*Sub)(c) }

func (c *Sub) Conn() *gws.Conn          { return (*gws.Conn)(c) }
func (c *Sub) GetMetadata() ee.Metadata { return c.Conn().Session() }
func (c *Sub) GetData() clientData {
	cd, _ := c.GetMetadata().Load("cd")
	return cd.(clientData)
}

func (c *Sub) GetSubscriberID() int32 { return c.GetData().cID }

// Add a subscriber to a topic of raw websocket messages
func Subscribe(em *ee.EventEmitter[int32, *Sub], s *Sub, topic string) {
	em.Subscribe(s, topic, func(msg any) {
		bc := msg.(*gws.Broadcaster)
		_ = bc.Broadcast(s.Conn())
	})
}

// Send a websocket message to
func Publish(em *ee.EventEmitter[int32, *Sub], topic string, msg []byte) {
	bc := gws.NewBroadcaster(gws.OpcodeBinary, msg)
	defer bc.Close()
	em.Publish(topic, bc)
}

// Shared handler for room websocket events
type roomHandler struct {
	em *ee.EventEmitter[int32, *Sub]
}

// Base chat channel
const topicChat = "chat"

func (h *roomHandler) OnMessage(c *gws.Conn, msg *gws.Message) {
	defer msg.Close()
	// slog.Info(hex.EncodeToString(msg.Bytes()))
}

func onWriteError(err error) {
	if err != nil {
		slog.Error("write", "err", err)
	}
}

// Increments by 1 for each
// connection opened
var cIDCounter atomic.Int32

func (h *roomHandler) OnOpen(c *gws.Conn) {
	s := NewSub(c)
	slog.Info("open", "cID", s.GetSubscriberID())

	// Send initial packets
	sPkt := syncPlayerDataS2C{
		HostID: s.GetData().cID,
		Key:    s.GetData().guardKey,
		UUID:   uuid.NewString(),
	}
	c.WriteAsync(gws.OpcodeBinary, serialize(sPkt), onWriteError)
	riPkt := roomInfoS2C{
		MapID: s.GetData().mapID,
	}
	c.WriteAsync(gws.OpcodeBinary, serialize(riPkt), onWriteError)

	// Subscribe to default topics
	Subscribe(h.em, s, topicChat)
}
func (h *roomHandler) OnClose(c *gws.Conn, err error) {
	s := NewSub(c)
	slog.Info("close", "cID", s.GetSubscriberID())

	// Remove all subscriptions
	h.em.UnSubscribeAll(s)
}

func (h *roomHandler) OnPing(c *gws.Conn, payload []byte) {
	// minnaengine doesn't send pings
	// but respond anyway
	_ = c.WritePong(nil)
}
func (h *roomHandler) OnPong(c *gws.Conn, payload []byte) {}
