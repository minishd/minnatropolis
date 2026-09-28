package unconscious

import "time"

type counEvent = int32

// IDs of each event.
// (Explicitly numbered, because these values
// must match what the game expects)
const (
	counEventNone           counEvent = 0
	counEventAnniversary    counEvent = 1
	counEventNewYear        counEvent = 2
	counEventSpringEquinox  counEvent = 3
	counEventAprilFools     counEvent = 4
	counEventSummerSolstice counEvent = 5
	counEventHalloween      counEvent = 6
	counEventWinter         counEvent = 7
)

func getCounEvent() counEvent {
	_, month, day := time.Now().UTC().Date()

	switch {
	case (month == time.December && day >= 30) || (month == time.January && day <= 2):
		return counEventNewYear
	case month == time.March && day >= 19 && day <= 21:
		return counEventSpringEquinox
	case (month == time.March && day >= 31) || (month == time.April && day <= 2):
		return counEventAprilFools
	case month == time.June && day >= 4 && day <= 11:
		return counEventAnniversary
	case month == time.June && day >= 18 && day <= 24:
		return counEventSummerSolstice
	case (month == time.October && day >= 21) || (month == time.November && day <= 4):
		return counEventHalloween
	case month == time.December && day >= 14 && day <= 28:
		return counEventWinter
	default:
		return counEventNone
	}
}
