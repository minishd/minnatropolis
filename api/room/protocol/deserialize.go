package protocol

import (
	"errors"
	"log"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func deserializeSlice(typ reflect.Type, parts []string) (val reflect.Value, consumed int, err error) {
	val = reflect.Indirect(reflect.New(typ))
	typElem := typ.Elem()
	nParts := len(parts)

	for {
		// Stop parsing if we ran out of parts
		// YNO protocol has no slices smaller than
		// the entire remainder of a message, so
		// that is the only check we have
		if consumed == nParts {
			break
		}

		// Deserialize slice element, then add the number of
		// parts it consumed to our total
		eS, eConsumed, eErr := deserializeAny(typElem, parts[consumed:])
		if err = eErr; err != nil {
			// We don't care about consumed, no deserializer
			// will continue parsing if it sees an error
			return
		}
		consumed += eConsumed
		// Append to slice
		val = reflect.Append(val, eS)
	}

	return
}

func deserializeStruct(typ reflect.Type, parts []string) (val reflect.Value, consumed int, err error) {
	val = reflect.Indirect(reflect.New(typ))

	for f, fVal := range val.Fields() {
		fS, fConsumed, fErr := deserializeAny(f.Type, parts[consumed:])
		if err = fErr; err != nil {
			return
		}
		fVal.Set(fS)
		consumed += fConsumed
	}

	return
}

func deserializeAny(typ reflect.Type, parts []string) (val reflect.Value, consumed int, err error) {
	// Handle types containing other types
	switch typ.Kind() {
	case reflect.Struct:
		return deserializeStruct(typ, parts)
	case reflect.Slice:
		return deserializeSlice(typ, parts)
	}

	val = reflect.Indirect(reflect.New(typ))
	part := parts[0]

	switch val.Interface().(type) {
	// [string]s don't need any change
	case string:
		val.SetString(part)
	// [int32]/[int64] and similar need to get parsed from string
	case int32, int64, PictureListType:
		n, dErr := strconv.ParseInt(part, 10, typ.Bits())
		if err = dErr; err != nil {
			return
		}
		val.SetInt(n)
	// [uint32]/[uint64] need to get parsed from string
	case uint32, uint64:
		n, dErr := strconv.ParseUint(part, 10, typ.Bits())
		if err = dErr; err != nil {
			return
		}
		val.SetUint(n)

	// [bool]s are represented by a "0" or a "1" character
	case bool:
		b := false
		switch part {
		case "0":
			// No change is needed
			break
		case "1":
			b = true
		default:
			err = errors.New("invalid boolean")
			return
		}
		val.SetBool(b)

	// [uuid.UUID] must be parsed from string
	case uuid.UUID:
		uuid, dErr := uuid.Parse(part)
		if err = dErr; err != nil {
			return
		}
		val.Set(reflect.ValueOf(uuid))

	case int, uint, uintptr:
		// Only allow ints of specified bitness
		panic("deserialize int of unspecified size")
	default:
		log.Println(typ.String())
		panic("deserialize unhandled type")
	}

	// Primitive protocol types consume one part,
	// so add that to our total
	consumed++

	return
}

// Convert a C2S message from YNO's format
// to its corresponding Go struct
func deserializeOne(msgBytes []byte) (msg any, err error) {
	// Split packet by delimiter
	parts := strings.Split(string(msgBytes), string(paramDelim))

	// Extract name of packet
	if len(parts) < 1 {
		err = errors.New("packet has no name")
		return
	}
	name := parts[0]

	// Look up packet type
	typ, ok := packetsC2S[name]
	if !ok {
		err = errors.New("no such packet type")
		return
	}

	// Deserialize packet
	partsNameless := parts[1:]
	val, consumed, dErr := deserializeAny(typ, partsNameless)
	if err = dErr; err != nil {
		return
	}
	msg = val.Interface()

	// Check if too many fields were sent
	nPartsNameless := len(partsNameless)
	if consumed != nPartsNameless {
		err = errors.New("too many fields sent")
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
		var msg any
		msg, err = deserializeOne([]byte(msgStr))
		if err != nil {
			return
		}

		msgs = append(msgs, msg)
	}

	return
}
