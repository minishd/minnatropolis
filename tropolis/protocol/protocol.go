package protocol

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
)

var (
	paramDelim   = []byte{0xef, 0xbf, 0xbf}
	messageDelim = []byte{0xef, 0xbf, 0xbe}
)

// Deserialize a message from YNO's format
func DeserializeOne(msgBytes []byte) (msg any, err error) {
	msgStr := string(msgBytes)
	parts := strings.Split(msgStr, string(paramDelim))
	partsLen := len(parts)
	if partsLen < 1 {
		err = errors.New("name of packet unspecified")
		return
	}

	packetName := parts[0]
	typ, ok := packetsC2S[packetName]
	if !ok {
		err = errors.New("not a valid packet name")
		return
	}

	numField := typ.NumField()
	if partsLen < numField+1 {
		err = errors.New("there are not enough fields")
		return
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
			var partInt int64
			partInt, err = strconv.ParseInt(part, 10, fieldValue.Type().Bits())
			if err != nil {
				// Invalid integer
				return
			}
			fieldValue.SetInt(partInt)

		case bool:
			partBool := false
			switch part {
			case "0":
				break
			case "1":
				partBool = true
			default:
				// Invalid boolean
				return
			}
			fieldValue.SetBool(partBool)

		case int, uint, uintptr:
			// Only allow ints of specified bitness
			panic("message uses int of unspecified size")
		default:
			panic("message contains unhandled type")
		}
	}

	msg = msgValueI.Interface()

	return
}

// Deserialize one or more messages from YNO's format
func Deserialize(msgsBytes []byte) (msgs []any) {
	msgStrSeq := strings.SplitSeq(string(msgsBytes), string(messageDelim))
	for msgStr := range msgStrSeq {
		msg, err := DeserializeOne([]byte(msgStr))
		if err != nil {
			continue
		}

		msgs = append(msgs, msg)
	}

	return
}

// Serialize a message into YNO's format
func SerializeOne(msg any) (msgBytes []byte) {
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
		case uint32, uint64:
			msgBytes = append(msgBytes, []byte(strconv.FormatUint(fieldValue.Uint(), 10))...)

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

		case bool:
			digit := '0'
			if field {
				digit = '1'
			}
			msgBytes = append(msgBytes, byte(digit))

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

// Serialize one or more messages into YNO's format
func Serialize(msgs ...any) (msgsBytes []byte) {
	msgsLen := len(msgs)
	for i, msg := range msgs {
		msgBytes := SerializeOne(msg)
		msgsBytes = append(msgsBytes, msgBytes...)

		if i+1 != msgsLen {
			msgsBytes = append(msgsBytes, messageDelim...)
		}
	}

	return
}
