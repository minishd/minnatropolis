package protocol

import (
	"reflect"

	"github.com/google/uuid"
)

// ********** General Types **********
// These types may be shared between
// S->C, C->S, and general backend
// code

type BasePicture struct {
	PicID                   int32
	PosX, PosY              int32
	MapX, MapY              int32
	PanX, PanY              int32
	Magnify                 int32
	TopTransp, BottomTransp int32
	R, G, B, Saturation     int32
	EffectMode              int32
	EffectPower             int32
}
type Picture struct {
	BasePicture

	PicName        string
	UseTranspColor bool
	FixedToMap     bool

	SpritesheetCols, SpritesheetRows int32
	SpritesheetFrame                 int32
	SpritesheetSpeed                 int32
	SpritesheetPlayOnce              bool

	MapLayer    int32
	BattleLayer int32
	Flags       int32
	BlendMode   int32

	FlipX, FlipY bool

	Origin int32
}

type Flash struct {
	R, G, B       int32
	Power, Frames int32
}

// ********** Server -> Client **********
// These packets are sent by the server
// to clients (minnaengine).

type SyncPlayerDataS2C struct {
	HostID     int32
	Key        uint32
	UUID       uuid.UUID
	Rank       int32
	IsLoggedIn bool
	Badge      string
}

type RoomInfoS2C struct {
	RoomID int32
}

type ConnectS2C struct {
	ID         int32
	UUID       uuid.UUID
	Rank       int32
	IsLoggedIn bool
	Badge      string
}

type DisconnectS2C struct {
	ID int32
}

type NameS2C struct {
	ID   int32
	Name string
}

type MainPlayerPosS2C struct {
	ID   int32
	X, Y int32
}

type JumpS2C struct {
	ID   int32
	X, Y int32
}

type SpriteS2C struct {
	ID    int32
	Name  string
	Index int32
}

type FacingS2C struct {
	ID        int32
	Direction int32
}

type SpeedS2C struct {
	ID    int32
	Speed int32
}

type HiddenS2C struct {
	ID     int32
	Hidden bool
}

type TransparencyS2C struct {
	ID           int32
	Transparency int32
}

type SysNameS2C struct {
	ID   int32
	Name string
}

type SoundEffectS2C struct {
	ID      int32
	Name    string
	Volume  int32
	Tempo   int32
	Balance int32
}

type FlashS2C struct {
	ID    int32
	Flash Flash
}

type RepeatingFlashS2C struct {
	ID    int32
	Flash Flash
}

type RemoveRepeatingFlashS2C struct {
	ID int32
}

type ShowPlayerBattleAnimS2C struct {
	ID     int32
	AnimID int32
}

type BattleAnimSyncListS2C struct {
	IDs []int32
}

type PictureListType int32

const (
	PictureListName PictureListType = iota
	PictureListPrefix
)

type PictureSyncListS2C struct {
	Type PictureListType
	List []string
}

type ShowPictureS2C struct {
	ID int32
	Picture
}

type MovePictureS2C struct {
	ID int32
	BasePicture

	Duration int32
}

type ErasePictureS2C struct {
	ID    int32
	PicID int32
}

// ********** Client -> Server **********
// These are packets sent by the client
// and handled by the server.

type SwitchRoomC2S struct {
	RoomID int32
}

type MainPlayerPosC2S struct {
	X, Y int32
}

type TeleportC2S struct {
	X, Y int32
}

type JumpC2S struct {
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
	Hidden bool
}

type SysNameC2S struct {
	Name string
}

type TransparencyC2S struct {
	Transparency int32
}

type SoundEffectC2S struct {
	Name    string
	Volume  int32
	Tempo   int32
	Balance int32
}

type FlashC2S struct {
	Flash Flash
}

type RepeatingFlashC2S struct {
	Flash Flash
}

type RemoveRepeatingFlashC2S struct{}

type ShowPlayerBattleAnimC2S struct {
	AnimID int32
}

type ShowPictureC2S struct {
	Picture
}

type MovePictureC2S struct {
	BasePicture

	Duration int32
}

type ErasePictureC2S struct {
	PicID int32
}

// ********** Message Registry **********

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

func init() {
	registerS2C[SyncPlayerDataS2C]("s")
	registerS2C[RoomInfoS2C]("ri")
	registerS2C[ConnectS2C]("c")
	registerS2C[DisconnectS2C]("d")
	registerS2C[NameS2C]("name")
	registerS2C[MainPlayerPosS2C]("m")
	registerS2C[JumpS2C]("jmp")
	registerS2C[SpriteS2C]("spr")
	registerS2C[FacingS2C]("f")
	registerS2C[SpeedS2C]("spd")
	registerS2C[HiddenS2C]("h")
	registerS2C[TransparencyS2C]("tr")
	registerS2C[SysNameS2C]("sys")
	registerS2C[SoundEffectS2C]("se")
	registerS2C[FlashS2C]("fl")
	registerS2C[RepeatingFlashS2C]("rfl")
	registerS2C[RemoveRepeatingFlashS2C]("rrfl")
	registerS2C[ShowPlayerBattleAnimS2C]("ba")
	registerS2C[BattleAnimSyncListS2C]("bas")
	registerS2C[PictureSyncListS2C]("pns")
	registerS2C[ShowPictureS2C]("ap")
	registerS2C[MovePictureS2C]("mp")
	registerS2C[ErasePictureS2C]("rp")

	registerC2S[SwitchRoomC2S]("sr")
	registerC2S[MainPlayerPosC2S]("m")
	registerC2S[TeleportC2S]("tp")
	registerC2S[JumpC2S]("jmp")
	registerC2S[SpeedC2S]("spd")
	registerC2S[SpriteC2S]("spr")
	registerC2S[FacingC2S]("f")
	registerC2S[HiddenC2S]("h")
	registerC2S[SysNameC2S]("sys")
	registerC2S[TransparencyC2S]("tr")
	registerC2S[SoundEffectC2S]("se")
	registerC2S[FlashC2S]("fl")
	registerC2S[RepeatingFlashC2S]("rfl")
	registerC2S[RemoveRepeatingFlashC2S]("rrfl")
	registerC2S[ShowPlayerBattleAnimC2S]("ba")
	registerC2S[ShowPictureC2S]("ap")
	registerC2S[MovePictureC2S]("mp")
	registerC2S[ErasePictureC2S]("rp")
}
