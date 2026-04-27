package room

import (
	ee "github.com/lxzan/event_emitter"
	"github.com/lxzan/gws"
	pt "github.com/minishd/minnatropolis/tropolis/protocol"
)

// Data associated with a room client
type clientData struct {
	cID  int32
	name string

	guardKey, guardCount uint32

	roomID int32
	x, y   int32
	facing int32
}

// Wrapper around a [gws.Conn].
//
// Implements [ee.Subscriber] and provides
// relevant utility functions.
type User gws.Conn

func NewUser(c *gws.Conn) *User { return (*User)(c) }

func (u *User) GetMetadata() ee.Metadata { return u.Conn().Session() }
func (u *User) GetSubscriberID() int32   { return u.GetData().cID }

// Get underlying [gws.Conn].
func (u *User) Conn() *gws.Conn { return (*gws.Conn)(u) }

func (u *User) GetData() *clientData {
	cd, _ := u.GetMetadata().Load("cd")
	return cd.(*clientData)
}

func onWriteError(err error) {}

// Serialize and send a YNO message.
func (u *User) Send(msgs ...any) {
	data := pt.Serialize(msgs...)
	u.Conn().WriteAsync(gws.OpcodeBinary, data, onWriteError)
}
