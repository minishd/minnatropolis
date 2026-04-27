package tropolis

import (
	"reflect"
	"strconv"
	"strings"
)

type syncPlayerDataS2C struct {
	HostID     int32
	Key        string
	UUID       string
	Rank       int32
	AccountBin int32 // binary
	Badge      string
	Medals     [5]int32
}

type roomInfoS2C struct {
	MapID int32
}

type connectS2C struct {
	ID         int32
	UUID       string
	Rank       int32
	AccountBin int32 // binary
	Badge      string
	Medals     [5]int32
}

type disconnectS2C struct {
	ID int32
}

type switchRoomC2S struct {
	MapID int32
}

type mainPlayerPosC2S struct {
	X, Y int32
}

type speedC2S struct {
	Speed int32
}

type spriteC2S struct {
	Name  string
	Index int32
}

type facingC2S struct {
	Direction int32
}

type hiddenC2S struct {
	HiddenBin int32 // binary
}

type sysNameC2S struct {
	Name string
}

var (
	paramDelim   = []byte{0xef, 0xbf, 0xbf}
	messageDelim = []byte{0xef, 0xbf, 0xbe}
)

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

func registerAllPackets() {
	registerS2C[syncPlayerDataS2C]("s")
	registerS2C[roomInfoS2C]("ri")
	registerS2C[connectS2C]("c")
	registerS2C[disconnectS2C]("d")

	registerC2S[switchRoomC2S]("sr")
	registerC2S[mainPlayerPosC2S]("m")
	registerC2S[speedC2S]("spd")
	registerC2S[spriteC2S]("spr")
	registerC2S[facingC2S]("f")
	registerC2S[hiddenC2S]("h")
	registerC2S[sysNameC2S]("sys")
}

// Deserialize a message from YNO's format
func deserialize(msgBytes []byte) (msgs []any) {
	msgStr := string(msgBytes)
	msgStrSeq := strings.SplitSeq(msgStr, string(messageDelim))

msgLoop:
	for msgStr := range msgStrSeq {
		parts := strings.Split(msgStr, string(paramDelim))
		partsLen := len(parts)
		if partsLen < 1 {
			// Not enough fields for a name,
			// we can't parse
			continue
		}

		packetName := parts[0]
		typ, ok := packetsC2S[packetName]
		if !ok {
			// Not a valid packet name
			continue
		}

		numField := typ.NumField()
		if partsLen < numField+1 {
			// There are not enough fields
			continue
		}

		msgValueI := reflect.Indirect(reflect.New(typ))

		for i := range numField {
			fieldValue := msgValueI.Field(i)
			field := fieldValue.Interface()

			part := parts[i+1]

			switch field.(type) {
			case string:
				fieldValue.SetString(part)
			case int32, int64:
				partInt, err := strconv.ParseInt(part, 10, fieldValue.Type().Bits())
				if err != nil {
					// Couldn't parse integer
					continue msgLoop
				}
				fieldValue.SetInt(partInt)

			case int, uint, uintptr:
				// Only allow ints of specified bitness
				panic("message uses int of unspecified size")
			default:
				panic("message contains unhandled type")
			}
		}

		msgs = append(msgs, msgValueI.Interface())
	}

	return
}

// Serialize a message into YNO's format
func serialize(msg any) (msgBytes []byte) {
	msgValue := reflect.ValueOf(msg)

	packetName, ok := packetsS2C[msgValue.Type()]
	if !ok {
		panic("can't serialize unregistered packet")
	}

	msgBytes = append(msgBytes, []byte(packetName)...)
	msgBytes = append(msgBytes, paramDelim...)

	numField := msgValue.NumField()
	for i := range numField {
		fieldValue := msgValue.Field(i)
		field := fieldValue.Interface()

		switch field := field.(type) {
		case byte:
			msgBytes = append(msgBytes, field)
		case []byte:
			msgBytes = append(msgBytes, field...)
		case string:
			msgBytes = append(msgBytes, []byte(field)...)

		case int32, int64:
			msgBytes = append(msgBytes, []byte(strconv.FormatInt(fieldValue.Int(), 10))...)

		// Special case for [5]int32 for now
		case []int32, []int64, [5]int32:
			fieldLen := fieldValue.Len()
			for i := range fieldLen {
				elem := fieldValue.Index(i).Int()

				msgBytes = append(msgBytes, []byte(strconv.FormatInt(elem, 10))...)
				if i+1 != fieldLen {
					msgBytes = append(msgBytes, paramDelim...)
				}
			}

		case int, uint, uintptr:
			// We want sizes to be specified
			// (More workable for the future)
			panic("message uses int of unspecified size")
		default:
			panic("message contains unhandled type")
		}

		if i+1 != numField {
			msgBytes = append(msgBytes, paramDelim...)
		}
	}

	return
}
