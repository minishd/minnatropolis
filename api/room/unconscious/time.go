package unconscious

import "time"

// Amounts of real-world time that
// will pass per in-game time
const (
	perCounHour  = time.Minute
	perCounDay   = perCounHour * 20
	perCounCycle = perCounDay * 12
)

func getCounTime() int32 {
	now := time.Now().UTC()
	epoch := time.UnixMicro(0).UTC()
	sinceEpoch := now.Sub(epoch)
	counHour := (sinceEpoch % perCounCycle) / perCounHour

	return int32(counHour)
}
