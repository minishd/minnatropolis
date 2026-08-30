package room

import (
	"fmt"

	pt "github.com/minishd/minnatropolis/api/room/protocol"
)

// Ranges come from ynoproject/ynoserver
const (
	minSpriteIndex = 0

	minXY = 0
	maxXY = 500

	minFacing = 0
	maxFacing = 3

	minSpeed = 0
	maxSpeed = 10

	minTransparency = 0
	maxTransparency = 7

	minSoundVolume = 0
	maxSoundVolume = 100

	minSoundTempo = 10
	maxSoundTempo = 400

	minSoundBalance = 0
	maxSoundBalance = 100

	minRGB = 0
	maxRGB = 255

	minFlashPower  = 0
	minFlashFrames = 0

	minPicID = 0
	maxPicID = 1000

	minPicMagnify    = 0
	minPicRGBS       = 0
	maxPicRGBS       = 200
	minPicEffectMode = 0

	minPicDuration = 0
)

// No upper positions as the server doesn't know them.
func isValidSpriteIndex(index int32) bool {
	return index >= minSpriteIndex
}

func isValidXY(p int32) bool {
	return p >= minXY && p <= maxXY
}

func isValidSoundBalance(balance int32) bool {
	return balance >= minSoundBalance && balance <= maxSoundBalance
}

func isValidSoundTempo(tempo int32) bool {
	return tempo >= minSoundTempo && tempo <= maxSoundTempo
}

func isValidSoundVolume(volume int32) bool {
	return volume >= minSoundVolume && volume <= maxSoundVolume
}

func isValidTransparency(transparency int32) bool {
	return transparency >= minTransparency && transparency <= maxTransparency
}

func isValidSpeed(speed int32) bool {
	return speed >= minSpeed && speed <= maxSpeed
}

func isValidFacingDirection(facing int32) bool {
	return facing >= minFacing && facing <= maxFacing
}

func isValidRGB(x int32) bool {
	return x >= minRGB && x <= maxRGB
}

func isValidFlashPower(power int32) bool {
	return power >= minFlashPower
}

func isValidFlashFrames(frames int32) bool {
	return frames >= minFlashFrames
}

func isValidPicID(id int32) bool {
	return id >= minPicID && id <= maxPicID
}

func isValidPicMagnify(magnify int32) bool {
	return magnify >= minPicMagnify
}

func isValidPicRGBS(c int32) bool {
	return c >= minPicRGBS && c <= maxPicRGBS
}

func isValidPicEffectMode(effectMode int32) bool {
	return effectMode >= minPicEffectMode
}

func isValidPicDuration(duration int32) bool {
	return duration >= minPicDuration
}

func validateBasePicture(bp pt.BasePicture) error {
	if !isValidPicID(bp.PicID) {
		return fmt.Errorf("pic id %d out of range", bp.PicID)
	}
	if !isValidPicMagnify(bp.Magnify) {
		return fmt.Errorf("pic magnify %d out of range", bp.Magnify)
	}
	if !isValidPicRGBS(bp.R) ||
		!isValidPicRGBS(bp.G) ||
		!isValidPicRGBS(bp.B) ||
		!isValidPicRGBS(bp.Saturation) {
		return fmt.Errorf("pic rgbs %d,%d,%d,%d out of range", bp.R, bp.G, bp.B, bp.Saturation)
	}
	if !isValidPicEffectMode(bp.EffectMode) {
		return fmt.Errorf("pic effect mode %d out of range", bp.EffectMode)
	}

	return nil
}

// Validates requests and returns an error if
// the message was tampered.
func (h *Handler) validateMessage(m any) error {
	switch m := m.(type) {

	case pt.SwitchRoomC2S:
		if !h.filters.HasMap(m.RoomID) {
			return fmt.Errorf("unknown room id %d", m.RoomID)
		}

	case pt.FacingC2S:
		if !isValidFacingDirection(m.Direction) {
			return fmt.Errorf("facing direction %d out of range", m.Direction)
		}

	case pt.SpeedC2S:
		if !isValidSpeed(m.Speed) {
			return fmt.Errorf("speed %d out of range", m.Speed)
		}

	case pt.SysNameC2S:
		if !h.filters.HasSystem(m.Name) {
			return fmt.Errorf("unknown system %q", m.Name)
		}

	case pt.SpriteC2S:
		if !h.filters.HasSprite(m.Name) {
			return fmt.Errorf("unknown sprite %q", m.Name)
		}
		if !isValidSpriteIndex(m.Index) {
			return fmt.Errorf("sprite index %d out of range", m.Index)
		}

	case pt.MainPlayerPosC2S:
		if !isValidXY(m.X) || !isValidXY(m.Y) {
			return fmt.Errorf("move position %d,%d out of range", m.X, m.Y)
		}
	case pt.TeleportC2S:
		if !isValidXY(m.X) || !isValidXY(m.Y) {
			return fmt.Errorf("teleport position %d,%d out of range", m.X, m.Y)
		}
	case pt.JumpC2S:
		if !isValidXY(m.X) || !isValidXY(m.Y) {
			return fmt.Errorf("jump position %d,%d out of range", m.X, m.Y)
		}

	case pt.TransparencyC2S:
		if !isValidTransparency(m.Transparency) {
			return fmt.Errorf("transparency %d out of range", m.Transparency)
		}

	case pt.SoundEffectC2S:
		if !isValidSoundVolume(m.Volume) {
			return fmt.Errorf("sound volume %d out of range", m.Volume)
		}
		if !isValidSoundTempo(m.Tempo) {
			return fmt.Errorf("sound tempo %d out of range", m.Tempo)
		}
		if !isValidSoundBalance(m.Balance) {
			return fmt.Errorf("sound balance %d out of range", m.Balance)
		}

	case pt.FlashC2S:
		if !isValidRGB(m.R) || !isValidRGB(m.G) || !isValidRGB(m.B) {
			return fmt.Errorf("flash rgb %d,%d,%d out of range", m.R, m.G, m.B)
		}
		if !isValidFlashPower(m.Power) {
			return fmt.Errorf("flash power %d out of range", m.Power)
		}
		if !isValidFlashFrames(m.Frames) {
			return fmt.Errorf("flash frames %d out of range", m.Frames)
		}

	case pt.ShowPlayerBattleAnimC2S:
		if !h.filters.HasBattleAnimID(m.AnimID) {
			return fmt.Errorf("unknown battle anim ID %d", m.AnimID)
		}

	case pt.ShowPictureC2S:
		if err := validateBasePicture(m.BasePicture); err != nil {
			return err
		}
		if !h.filters.HasPicture(m.PicName) {
			return fmt.Errorf("unknown picture %s", m.PicName)
		}

	case pt.MovePictureC2S:
		if err := validateBasePicture(m.BasePicture); err != nil {
			return err
		}
		if !isValidPicDuration(m.Duration) {
			return fmt.Errorf("pic duration %d out of range", m.Duration)
		}

	case pt.ErasePictureC2S:
		if !isValidPicID(m.PicID) {
			return fmt.Errorf("erase pic id %d out of range", m.PicID)
		}

	// Unvalidated
	case pt.HiddenC2S:

	default:
		// We should at least add a no-op case
		// for packets we don't need to validate..
		// (So no packets are forgotten)
		panic("packet has no validation")

	}

	return nil
}
