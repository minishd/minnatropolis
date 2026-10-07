package room

// Functions relating to message processing,
// game logic can go here

import (
	"log"

	pt "github.com/minishd/minnatropolis/api/room/protocol"
	"github.com/minishd/minnatropolis/api/room/user"
)

func (h *Handler) removePicture(d *user.ClientData, picID int32) {
	delete(d.ActivePictures, picID)
	h.shareToRoom(d, pt.ErasePictureS2C{ID: d.CID, PicID: picID})
}

func (h *Handler) updatePicture(d *user.ClientData, pic *pt.Picture) {
	// If it's not a one-shot effect,
	// we will keep track of it
	if !pic.SpritesheetPlayOnce {
		d.ActivePictures[pic.PicID] = pic
	}
}

func (h *Handler) processMessage(u *user.User, m any) {
	d := u.GetData()

	switch m := m.(type) {

	case pt.SwitchRoomC2S:
		log.Println("change to room", m.RoomID)
		h.changeRoom(u, m.RoomID)

	case pt.MainPlayerPosC2S:
		d.X = m.X
		d.Y = m.Y
		h.shareToRoom(d, pt.MainPlayerPosS2C{ID: d.CID, X: d.X, Y: d.Y})
	case pt.TeleportC2S:
		d.X = m.X
		d.Y = m.Y
		h.shareToRoom(d, pt.MainPlayerPosS2C{ID: d.CID, X: d.X, Y: d.Y})
	case pt.JumpC2S:
		d.X = m.X
		d.Y = m.Y
		h.shareToRoom(d, pt.JumpS2C{ID: d.CID, X: d.X, Y: d.Y})

	case pt.SpeedC2S:
		d.Speed = m.Speed
		h.shareToRoom(d, pt.SpeedS2C{ID: d.CID, Speed: d.Speed})

	case pt.SpriteC2S:
		d.Sprite = m.Name
		d.SpriteIndex = m.Index
		h.shareToRoom(d, pt.SpriteS2C{ID: d.CID, Name: d.Sprite, Index: d.SpriteIndex})

	case pt.FacingC2S:
		d.Facing = m.Direction
		h.shareToRoom(d, pt.FacingS2C{ID: d.CID, Direction: d.Facing})

	case pt.HiddenC2S:
		d.Hidden = m.Hidden
		h.shareToRoom(d, pt.HiddenS2C{ID: d.CID, Hidden: d.Hidden})

	case pt.SysNameC2S:
		d.SysName = m.Name
		h.shareToRoom(d, pt.SysNameS2C{ID: d.CID, Name: d.SysName})

	case pt.TransparencyC2S:
		d.Transparency = m.Transparency
		h.shareToRoom(d, pt.TransparencyS2C{ID: d.CID, Transparency: d.Transparency})

	case pt.SoundEffectC2S:
		h.shareToRoom(d, pt.SoundEffectS2C{ID: d.CID, Name: m.Name, Volume: m.Volume, Tempo: m.Tempo, Balance: m.Balance})

	case pt.FlashC2S:
		h.shareToRoom(d, pt.FlashS2C{ID: d.CID, Flash: m.Flash})
	case pt.RepeatingFlashC2S:
		h.shareToRoom(d, pt.RepeatingFlashS2C{ID: d.CID, Flash: m.Flash})
	case pt.RemoveRepeatingFlashC2S:
		d.Flash = nil
		h.shareToRoom(d, pt.RemoveRepeatingFlashS2C{ID: d.CID})

	case pt.ShowPlayerBattleAnimC2S:
		h.shareToRoom(d, pt.ShowPlayerBattleAnimS2C{ID: d.CID, AnimID: m.AnimID})

	case pt.ShowPictureC2S:
		// If there is already a picture
		// with that ID, we will remove it
		_, ok := d.ActivePictures[m.PicID]
		if ok {
			h.removePicture(d, m.PicID)
		}

		h.updatePicture(d, m.Picture)
		h.shareToRoom(d, pt.ShowPictureS2C{ID: d.CID, Picture: m.Picture})
	case pt.MovePictureC2S:
		pic, ok := d.ActivePictures[m.PicID]
		if !ok {
			// No such picture?
			// That's invalid but I won't do
			// anything about it for now
			return
		}
		pic.BasePicture = m.BasePicture

		h.updatePicture(d, pic)
		h.shareToRoom(d, pt.MovePictureS2C{ID: d.CID, BasePicture: m.BasePicture, Duration: m.Duration})

	case pt.ErasePictureC2S:
		_, ok := d.ActivePictures[m.PicID]
		if !ok {
			// Also no such picture..
			return
		}
		h.removePicture(d, m.PicID)

	default:
		// If we registered a message type,
		// we should also be handling it
		panic("unhandled message type")
	}
}
