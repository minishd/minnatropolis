package protocol

import (
	"reflect"
	"strconv"

	"github.com/google/uuid"
)

func serializeSlice(val reflect.Value) (msgBytes []byte) {
	// Iterate over all elements
	n := val.Len()
	for i := range n {
		// Serialize it
		eVal := val.Index(i)
		msgBytes = append(msgBytes, serializeAny(eVal)...)

		// If not last element, add delimiter
		if i+1 != n {
			msgBytes = append(msgBytes, paramDelim...)
		}
	}

	return
}
func serializeStruct(val reflect.Value) (msgBytes []byte) {
	// Iterate over all fields
	n := val.NumField()
	for i := range n {
		// Serialize it
		fVal := val.Field(i)
		msgBytes = append(msgBytes, serializeAny(fVal)...)

		// If not last field, add delimiter
		if i+1 != n {
			msgBytes = append(msgBytes, paramDelim...)
		}
	}

	return
}
func serializeAny(val reflect.Value) (msgBytes []byte) {
	// In the serializer, we want to offer a fast path
	// for []byte, so the type switch statement is put
	// before the struct/slice check

	field := val.Interface()
	switch field := field.(type) {
	case byte:
		msgBytes = append(msgBytes, field)
	case []byte:
		msgBytes = append(msgBytes, field...)
	case string:
		msgBytes = append(msgBytes, []byte(field)...)

	case bool:
		digit := '0'
		if field {
			digit = '1'
		}
		msgBytes = append(msgBytes, byte(digit))

	case int32, int64, PictureListType:
		msgBytes = append(msgBytes, []byte(strconv.FormatInt(val.Int(), 10))...)
	case uint32, uint64:
		msgBytes = append(msgBytes, []byte(strconv.FormatUint(val.Uint(), 10))...)

	case uuid.UUID:
		uuidStr := field.String()
		msgBytes = append(msgBytes, []byte(uuidStr)...)

	case int, uint, uintptr:
		// We want sizes to be specified
		// (More workable for the future)
		panic("serialize int of unspecified size")

	default:
		switch val.Kind() {
		case reflect.Struct:
			return serializeStruct(val)
		case reflect.Slice:
			return serializeSlice(val)
		default:
			panic("serialize unhandled type")
		}
	}

	return
}

// Convert an S2C message from a Go struct
// to network format
func serializeOne(msg any) (msgBytes []byte) {
	val := reflect.ValueOf(msg)
	name, ok := packetsS2C[val.Type()]
	if !ok {
		panic("unregistered packet type")
	}

	msgBytes = []byte(name)
	msgBytes = append(msgBytes, paramDelim...)
	msgBytes = append(msgBytes, serializeAny(val)...)

	return
}

// Convert one or more messages into YNO's format
func Serialize(msgs ...any) (msgsBytes []byte) {
	msgsLen := len(msgs)
	for i, msg := range msgs {
		// Serialize and push single message
		msgBytes := serializeOne(msg)
		msgsBytes = append(msgsBytes, msgBytes...)

		// If it's not the last message,
		// add a message delimiter
		if i+1 != msgsLen {
			msgsBytes = append(msgsBytes, messageDelim...)
		}
	}

	return
}
