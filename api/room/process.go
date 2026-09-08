package room

// Functions relating to message processing,
// game logic can go here

import (
	"log"

	pt "github.com/minishd/minnatropolis/api/room/protocol"
)

func (h *Handler) removePicture(d *clientData, picID int32) {
	delete(d.activePictures, picID)
	h.shareToRoom(d, pt.ErasePictureS2C{ID: d.cID, PicID: picID})
}

func (h *Handler) updatePicture(d *clientData, pic pt.Picture) {
	// If it's not a one-shot effect,
	// we will keep track of it
	if !pic.SpritesheetPlayOnce {
		d.activePictures[pic.PicID] = pic
	}
}

func (h *Handler) processMessage(u *User, m any) {
	d := u.getData()

	switch m := m.(type) {

	case pt.SwitchRoomC2S:
		log.Println("change to room", m.RoomID)
		h.changeRoom(u, m.RoomID)

	case pt.MainPlayerPosC2S:
		d.x = m.X
		d.y = m.Y
		h.shareToRoom(d, pt.MainPlayerPosS2C{ID: d.cID, X: d.x, Y: d.y})
	case pt.TeleportC2S:
		d.x = m.X
		d.y = m.Y
		h.shareToRoom(d, pt.MainPlayerPosS2C{ID: d.cID, X: d.x, Y: d.y})
	case pt.JumpC2S:
		d.x = m.X
		d.y = m.Y
		h.shareToRoom(d, pt.JumpS2C{ID: d.cID, X: d.x, Y: d.y})

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

	case pt.FlashC2S:
		h.shareToRoom(d, pt.FlashS2C{ID: d.cID, Flash: m.Flash})
	case pt.RepeatingFlashC2S:
		d.flash = &m.Flash
		h.shareToRoom(d, pt.RepeatingFlashS2C{ID: d.cID, Flash: m.Flash})
	case pt.RemoveRepeatingFlashC2S:
		d.flash = nil
		h.shareToRoom(d, pt.RemoveRepeatingFlashS2C{ID: d.cID})

	case pt.ShowPlayerBattleAnimC2S:
		h.shareToRoom(d, pt.ShowPlayerBattleAnimS2C{ID: d.cID, AnimID: m.AnimID})

	case pt.ShowPictureC2S:
		// If there is already a picture
		// with that ID, we will remove it
		_, ok := d.activePictures[m.PicID]
		if ok {
			h.removePicture(d, m.PicID)
		}

		h.updatePicture(d, m.Picture)
		h.shareToRoom(d, pt.ShowPictureS2C{ID: d.cID, Picture: m.Picture})
	case pt.MovePictureC2S:
		pic, ok := d.activePictures[m.PicID]
		if !ok {
			// No such picture?
			// That's invalid but I won't do
			// anything about it for now
			return
		}
		pic.BasePicture = m.BasePicture

		h.updatePicture(d, pic)
		h.shareToRoom(d, pt.MovePictureS2C{ID: d.cID, BasePicture: m.BasePicture, Duration: m.Duration})

	case pt.ErasePictureC2S:
		_, ok := d.activePictures[m.PicID]
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
