package room

import (
	"fmt"

	pt "github.com/minishd/minnatropolis/api/room/protocol"
)

// Ranges come from ynoproject/ynoserver
const (
	minRoomID = 1
	maxRoomID = 5000

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
)

// No upper positions as the server doesn't know them.
func isValidSpriteIndex(index int32) bool {
	return index >= 0
}

func isValidPosition(x, y int32) bool {
	return x >= 0 && y >= 0 || x <= 999_999 && y <= 999_999
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

func isValidRoomID(id int32) bool {
	return id >= minRoomID && id <= maxRoomID
}

// Validates requests and returns an error if
// the message was tampered.
func (h *Handler) validateMessage(m any) error {
	switch m := m.(type) {

	case pt.SwitchRoomC2S:
		if !isValidRoomID(m.RoomID) {
			return fmt.Errorf("room id %d out of range", m.RoomID)
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
		if !isValidPosition(m.X, m.Y) {
			return fmt.Errorf("positions %d, %d out of range", m.X, m.Y)
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

	}

	return nil
}
