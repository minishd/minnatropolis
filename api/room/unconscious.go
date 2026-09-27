package room

import (
	"time"

	pt "github.com/minishd/minnatropolis/api/room/protocol"
)

type SyncVar = int32

const (
	SyncVarCounEvent SyncVar = 1237
)

type CounEvent = int32

const (
	CounEventNone           CounEvent = 0
	CounEventAnniversary    CounEvent = 1
	CounEventNewYear        CounEvent = 2
	CounEventSpringEquinox  CounEvent = 3
	CounEventAprilFools     CounEvent = 4
	CounEventSummerSolstice CounEvent = 5
	CounEventHalloween      CounEvent = 6
	CounEventWinter         CounEvent = 7
)

func getCounEvent() CounEvent {
	_, month, day := time.Now().UTC().Date()

	switch {
	case (month == time.December && day >= 30) || (month == time.January && day <= 2):
		return CounEventNewYear
	case month == time.March && day >= 19 && day <= 21:
		return CounEventSpringEquinox
	case (month == time.March && day >= 31) || (month == time.April && day <= 2):
		return CounEventAprilFools
	case month == time.June && day >= 4 && day <= 11:
		return CounEventAnniversary
	case month == time.June && day >= 18 && day <= 24:
		return CounEventSummerSolstice
	case (month == time.October && day >= 21) || (month == time.November && day <= 4):
		return CounEventHalloween
	case month == time.December && day >= 14 && day <= 28:
		return CounEventWinter
	default:
		return CounEventNone
	}
}

func getPacketCounEvent() pt.SyncServerVariableS2C {
	return pt.SyncServerVariableS2C{VarID: SyncVarCounEvent, Value: getCounEvent()}
}
