package unconscious

import (
	"math"
	"math/rand/v2"
)

func weatherDelta(cur int32) int32 {
	// Pick random number in range [-10,10]
	random := rand.Int32N(21) - 10

	// If the previous temperature was of a high magnitude,
	// apply a correction of 4 in the opposite direction
	var sign float64 = 1
	if cur < 0 {
		sign = -1
	}
	scaled := float64(cur) / 100.0
	large := math.Round(scaled * scaled)
	correction := int32(large * -sign * 4)

	return random + correction
}

func clamp(n, low, high int32) int32 {
	return min(max(n, low), high)
}
