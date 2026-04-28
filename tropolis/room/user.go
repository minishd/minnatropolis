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

	// mocks for fields we don't implement yet
	accountUUID string
	rank        int32
	loggedIn    bool
	badge       string
	medals      [5]int32

	guardKey, guardCount uint32

	roomID int32
	x, y   int32
	facing int32
	speed  int32

	transparency int32
	hidden       bool
	sprite       string
	spriteIndex  int32
	sysName      string
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

// Get the packets for our initial state.
func (u *User) GetIntroMessages() (msgs []any) {
	d := u.GetData()

	msgs = append(msgs, pt.ConnectS2C{
		ID: d.cID, UUID: d.accountUUID,
		Rank: d.rank, IsLoggedIn: d.loggedIn,
		Badge: d.badge, Medals: d.medals,
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
	if d.hidden {
		msgs = append(msgs, pt.HiddenS2C{ID: d.cID, Hidden: d.hidden})
	}
	if d.sysName != defaultSysName {
		msgs = append(msgs, pt.SysNameS2C{ID: d.cID, Name: d.sysName})
	}

	return
}
