package tropolis

import (
	"reflect"
	"strconv"
	"strings"
)

type syncPlayerDataS2C_s struct {
	HostID     int32
	Key        int32 // (Gets read as a string in minnaengine)
	UUID       string
	Rank       int32
	AccountBin int32
	Badge      string
	Medals     [5]int32
}

type roomInfoS2C_ri struct {
	roomID uint32
}

var delim = []byte{0xef, 0xbf, 0xbf}

// Serialize a message into YNO's format
func serialize(msg any) (msgBytes []byte) {
	msgValue := reflect.ValueOf(msg)

	if msgValue.Kind() != reflect.Struct {
		panic("serializer is only for structs")
	}

	// Extract packet name from type name
	typeName := msgValue.Type().Name()
	_, packetName, found := strings.Cut(typeName, "_")
	if !found {
		panic("couldn't get packet name from struct type")
	}

	msgBytes = append(msgBytes, []byte(packetName)...)
	msgBytes = append(msgBytes, delim...)

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
					msgBytes = append(msgBytes, delim...)
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
			msgBytes = append(msgBytes, delim...)
		}
	}

	return
}
