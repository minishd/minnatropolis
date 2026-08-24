package room

import (
	"fmt"

	pt "github.com/minishd/minnatropolis/api/room/protocol"
)

const (
	minSpeed = 0
	maxSpeed = 10

	minRoomID = 1
	maxRoomID = 5000

	minFacing = 0
	maxFacing = 3
)

func isValidSpeed(speed int32) bool {
	return speed >= minSpeed && speed <= maxSpeed
}

func isValidRoomID(id int32) bool {
	return id >= minRoomID && id <= maxRoomID
}

func isValidFacingDirection(facing int32) bool {
	return facing >= minFacing && facing <= maxFacing
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

	}

	return nil
}
