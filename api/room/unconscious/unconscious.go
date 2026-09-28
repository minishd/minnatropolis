package unconscious

// Helpers for CU-exclusive multiplayer features (syncvars).
// (Currently these function very similarly to how they would on YNO)

import (
	"math/rand/v2"

	pt "github.com/minishd/minnatropolis/api/room/protocol"
)

type syncVar = int32

// Underlying variable IDs for syncvars.
const (
	syncVarCounEvent syncVar = 1237
)

// Shared game state for Collective Unconscious.
type Unconscious struct {
	randInt      int32 // Time's shared random value
	temp, precip int32 // Weather values
}

func New() *Unconscious {
	cu := &Unconscious{}
	cu.onTimeTick()
	cu.onWeatherTick()
	return cu
}

// Called on the start of every 2nd real-world minute.
func (cu *Unconscious) onWeatherTick() {
	tempDelta := weatherDelta(cu.temp)
	precipDelta := weatherDelta(cu.precip)

	cu.temp = clamp(cu.temp+tempDelta, -100, 100)
	cu.precip = clamp(cu.precip+precipDelta, 0, 100)
}

// Called on the start of every real-world minute.
func (cu *Unconscious) onTimeTick() {
	cu.randInt = rand.Int32N(256)
}

//

// Sent when a player switches rooms.
func (*Unconscious) GetEventPacket() pt.SyncServerVariableS2C {
	return pt.SyncServerVariableS2C{VarID: syncVarCounEvent, Value: getCounEvent()}
}

// Sent on initial connection, as well as on every new minute.
// It is sent globally to all players at the same time
// to avoid desync.
func (cu *Unconscious) GetTimePacket() pt.CUTimeS2C {
	return pt.CUTimeS2C{Time: getCounTime(), RandInt: cu.randInt}
}

// Sent on initial connection, as well as every on every 2nd minute.
// It is sent at the same time to all players.
func (cu *Unconscious) GetWeatherPacket() pt.CUWeatherS2C {
	return pt.CUWeatherS2C{Temperature: cu.temp, Precipitation: cu.precip}
}
