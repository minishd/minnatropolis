package room

import (
	"sync"

	"github.com/google/uuid"
	"github.com/lxzan/gws"
	pt "github.com/minishd/minnatropolis/api/room/protocol"
)

// Data associated with a room client
type clientData struct {
	cID  int32
	name string

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

// Wrapper around a [gws.Conn].
type User gws.Conn

func NewUser(c *gws.Conn) *User { return (*User)(c) }

// Get underlying [gws.Conn].
func (u *User) Conn() *gws.Conn { return (*gws.Conn)(u) }

const kClientData = "cd"

func (u *User) getData() *clientData {
	cd, _ := u.Conn().Session().Load(kClientData)
	return cd.(*clientData)
}

// Serialize and send a YNO message.
func (u *User) Send(msgs ...any) {
	data := pt.Serialize(msgs...)
	u.Conn().WriteAsync(gws.OpcodeBinary, data, nil)
}

// Get the packets for our initial state.
func (u *User) GetIntroMessages() (msgs []any) {
	d := u.getData()

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
