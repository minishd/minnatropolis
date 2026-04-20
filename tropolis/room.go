package tropolis

import (
	"fmt"
	"log/slog"
	"math/rand/v2"

	ee "github.com/lxzan/event_emitter"
	"github.com/lxzan/gws"
)

// Data associated with a room client
type clientData struct {
	cID uint64

	mapID uint16
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

func (c *Sub) GetSubscriberID() uint64 { return c.GetData().cID }

// Add a subscriber to a topic of raw websocket messages
func Subscribe(em *ee.EventEmitter[uint64, *Sub], s *Sub, topic string) {
	em.Subscribe(s, topic, func(msg any) {
		bc := msg.(*gws.Broadcaster)
		_ = bc.Broadcast(s.Conn())
	})
}

// Send a websocket message to
func Publish(em *ee.EventEmitter[uint64, *Sub], topic string, msg []byte) {
	bc := gws.NewBroadcaster(gws.OpcodeBinary, msg)
	defer bc.Close()
	em.Publish(topic, bc)
}

// Shared handler for room websocket events
type roomHandler struct {
	em *ee.EventEmitter[uint64, *Sub]
}

// Base chat channel
const topicChat = "chat"

func (h *roomHandler) OnMessage(c *gws.Conn, msg *gws.Message) {
	defer msg.Close()

	// slog.Info(hex.Dump(msg.Bytes()))
}

func (h *roomHandler) OnOpen(c *gws.Conn) {
	// Set up data
	c.Session().Store("cd", clientData{
		cID: rand.Uint64(),
	})

	s := NewSub(c)

	slog.Info("open", slog.Uint64("cID", s.GetSubscriberID()))

	// Subscribe to default topics
	Subscribe(h.em, s, topicChat)
}
func (h *roomHandler) OnClose(c *gws.Conn, err error) {
	s := NewSub(c)

	fmt.Println("close", slog.Uint64("cID", s.GetSubscriberID()))

	// Remove all subscriptions
	h.em.UnSubscribeAll(s)
}

func (h *roomHandler) OnPing(c *gws.Conn, payload []byte) {
	// minnaengine doesn't send pings
	// but respond anyway
	_ = c.WritePong(nil)
}
func (h *roomHandler) OnPong(c *gws.Conn, payload []byte) {}
