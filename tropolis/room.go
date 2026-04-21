package tropolis

import (
	"log/slog"
	"math/rand/v2"
	"sync/atomic"

	"github.com/google/uuid"
	ee "github.com/lxzan/event_emitter"
	"github.com/lxzan/gws"
)

// Data associated with a room client
type clientData struct {
	cID int32
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
	// Set up data
	c.Session().Store("cd", clientData{
		cID: cIDCounter.Add(1),
	})

	s := NewSub(c)
	slog.Info("open", "cID", s.GetSubscriberID())

	// Send a placeholder sync
	sPkt := syncPlayerDataS2C_s{
		HostID:     s.GetData().cID,
		Key:        rand.Int32(),
		UUID:       uuid.NewString(),
		Rank:       123,
		AccountBin: 1,
		Badge:      "abc",
		Medals:     [5]int32{},
	}
	c.WriteAsync(gws.OpcodeBinary, serialize(sPkt), onWriteError)

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
