package protocol

import (
	"reflect"
)

type SyncPlayerDataS2C struct {
	HostID     int32
	Key        uint32
	UUID       string
	Rank       int32
	AccountBin int32 // binary
	Badge      string
	Medals     [5]int32
}

type RoomInfoS2C struct {
	RoomID int32
}

type ConnectS2C struct {
	ID         int32
	UUID       string
	Rank       int32
	AccountBin int32 // binary
	Badge      string
	Medals     [5]int32
}

type DisconnectS2C struct {
	ID int32
}

type SwitchRoomC2S struct {
	RoomID int32
}

type MainPlayerPosC2S struct {
	X, Y int32
}

type SpeedC2S struct {
	Speed int32
}

type SpriteC2S struct {
	Name  string
	Index int32
}

type FacingC2S struct {
	Direction int32
}

type HiddenC2S struct {
	HiddenBin int32 // binary
}

type SysNameC2S struct {
	Name string
}

var (
	packetsS2C = make(map[reflect.Type]string)
	packetsC2S = make(map[string]reflect.Type)
)

func registerS2C[T any](name string) {
	typ := reflect.TypeFor[T]()
	packetsS2C[typ] = name
}
func registerC2S[T any](name string) {
	typ := reflect.TypeFor[T]()
	packetsC2S[name] = typ
}

func RegisterAllPackets() {
	registerS2C[SyncPlayerDataS2C]("s")
	registerS2C[RoomInfoS2C]("ri")
	registerS2C[ConnectS2C]("c")
	registerS2C[DisconnectS2C]("d")

	registerC2S[SwitchRoomC2S]("sr")
	registerC2S[MainPlayerPosC2S]("m")
	registerC2S[SpeedC2S]("spd")
	registerC2S[SpriteC2S]("spr")
	registerC2S[FacingC2S]("f")
	registerC2S[HiddenC2S]("h")
	registerC2S[SysNameC2S]("sys")
}
