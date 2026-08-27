// YNO protocol serializer/deserializer implementation that
// converts between Go structs and the string-based network format.
//
// For the Go structs that represent S->C and C->S packets,
// see [messages.go]
package protocol

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

var (
	paramDelim   = []byte{0xef, 0xbf, 0xbf}
	messageDelim = []byte{0xef, 0xbf, 0xbe}
)

// Deserialize the fields of a struct from the given list of parts
func deserializeFields(packetName string, typ reflect.Type, parts []string) (msg any, numPartsUsed int, err error) {
	// Before we do anything, do we have enough parts
	// to read all of the fields we want?
	// (It's okay to have more, maybe we will need to parse
	// a field that's a struct; NumField doesn't count fields
	// of child structures)
	numField := typ.NumField()
	partsLen := len(parts)
	if partsLen < numField {
		// No, there are not enough
		err = fmt.Errorf("%s: needed %d fields, there were only %d", packetName, numField, partsLen)
		return
	}

	// Create a new value of the type we got
	// and then get its pointer
	msgValueI := reflect.Indirect(reflect.New(typ))

	// Start mapping the packet params
	// to Go struct fields
	for i := range numField {
		// Get field from struct (so we can set it) as a [reflect.Value],
		// and as a normal [any] (so we can `switch` by its type)
		fieldName := typ.Field(i).Name
		fieldValue := msgValueI.Field(i)
		field := fieldValue.Interface()

		// If this field is another struct,
		// there will be no corresponding part
		// Hand off the rest of the parts
		// to be parsed as the struct's type
		switch field.(type) {
		case BasePicture, Picture:
			nextMsg, nextNPU, err_ := deserializeFields(packetName, fieldValue.Type(), parts[i:])
			if err_ != nil {
				err = err_
				return
			}
			fieldValue.Set(reflect.ValueOf(nextMsg))

			// We want to keep track of how many parts we have read
			// That way we can get rid of parts that don't
			// belong to the current struct
			parts = slices.Delete(parts, i+1, i+nextNPU)
			// ^ We also leave one of the fields that aren't ours
			// so that the next iteration reads the correct field
			// (Even though we didn't read any parts ourselves,
			// `i` still got incremented)
			numPartsUsed += nextNPU
			continue
		}

		part := parts[i]
		numPartsUsed++

		switch field.(type) {
		// [string]s don't need any change
		case string:
			fieldValue.SetString(part)
		// [int32]/[int64] need to get parsed from string
		case int32, int64:
			var partInt int64
			partInt, err = strconv.ParseInt(part, 10, fieldValue.Type().Bits())
			if err != nil {
				err = fmt.Errorf("%s/%s: invalid integer", packetName, fieldName)
				return
			}
			fieldValue.SetInt(partInt)

		// [bool]s are represented by a "0" or a "1" character
		case bool:
			partBool := false
			switch part {
			case "0":
				// No change is needed
				break
			case "1":
				partBool = true
			default:
				err = fmt.Errorf("%s/%s: invalid boolean", packetName, fieldName)
				return
			}
			fieldValue.SetBool(partBool)

		case int, uint, uintptr:
			// Only allow ints of specified bitness
			panic("inbound message uses int of unspecified size")
		default:
			panic("inbound message contains unhandled type")
		}
	}

	msg = msgValueI.Interface()

	return
}

// Convert a C2S message from YNO's format
// to its corresponding Go struct
func deserializeOne(msgBytes []byte) (msg any, err error) {
	// Split packet by param/part delimiter
	msgStr := string(msgBytes)
	parts := strings.Split(msgStr, string(paramDelim))
	partsLen := len(parts)
	if partsLen < 1 {
		// There no params, so we can't read the
		// first param to get the packet name
		err = errors.New("name of packet unspecified")
		return
	}

	// Read the name of the packet
	// and look up corresponding [reflect.Type]
	packetName := parts[0]
	typ, ok := packetsC2S[packetName]
	if !ok {
		// We did not find a [reflect.Type] for that name
		err = fmt.Errorf("not a valid packet name: `%s`", packetName)
		return
	}

	// Deserialize parts to struct fields
	namelessParts := parts[1:]
	msg, npu, err := deserializeFields(packetName, typ, namelessParts)
	if err != nil {
		return
	}

	// Make sure the client didn't send any extra fields
	// That would be weird right?
	namelessPartsLen := len(namelessParts)
	if namelessPartsLen != npu {
		err = fmt.Errorf("%s: too many fields (%d instead of %d)", packetName, namelessPartsLen, npu)
		return
	}

	return
}

// Convert one or more messages from YNO's format
func Deserialize(msgsBytes []byte) (msgs []any, err error) {
	// Split by message delimiter
	// Then parse each individual message
	msgStrSeq := strings.SplitSeq(string(msgsBytes), string(messageDelim))
	for msgStr := range msgStrSeq {
		// Parse single message
		msg, err_ := deserializeOne([]byte(msgStr))
		if err_ != nil {
			err = err_
			return
		}

		msgs = append(msgs, msg)
	}

	return
}

// Serialize the fields of a struct into an array of bytes
func serializeFields(msg any) (msgBytes []byte) {
	msgValue := reflect.ValueOf(msg)

	// Start converting fields to string representation
	// and pushing them to the message
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
		case []string:
			numStr := len(field)
			for i, str := range field {
				msgBytes = append(msgBytes, []byte(str)...)
				// If there is another string, we need
				// a param. delimiter to separate it
				if i+1 != numStr {
					msgBytes = append(msgBytes, paramDelim...)
				}
			}

		case int32, int64, PictureListType:
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

		case Picture, BasePicture:
			msgBytes = append(msgBytes, serializeFields(field)...)

		case int, uint, uintptr:
			// We want sizes to be specified
			// (More workable for the future)
			panic("outbound message uses int of unspecified size")
		default:
			panic("outbound message contains unhandled type")
		}

		// If not last field, add param delimiter
		if i+1 != numField {
			msgBytes = append(msgBytes, paramDelim...)
		}
	}

	return
}

// Convert an S2C message from a Go struct
// to network format
func serializeOne(msg any) (msgBytes []byte) {
	msgValue := reflect.ValueOf(msg)

	// Look up the name of the packet by [reflect.Type]
	packetName, ok := packetsS2C[msgValue.Type()]
	if !ok {
		panic("can't serialize unregistered packet")
	}

	// Push name of packet to message + param delimiter
	msgBytes = append(msgBytes, []byte(packetName)...)
	msgBytes = append(msgBytes, paramDelim...)

	// Push serialized struct fields
	msgBytes = append(msgBytes, serializeFields(msg)...)

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
