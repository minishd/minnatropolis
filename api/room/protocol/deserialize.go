package protocol

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
)

func deserializeSlice(typ reflect.Type, parts []string) (s any, consumed int, err error) {
	val := reflect.Indirect(reflect.New(typ))
	nParts := len(parts)

	for {
		// Stop parsing if we ran out of parts
		// YNO protocol has no slices smaller than
		// the entire remainder of a message, so
		// that is the only check we have
		if consumed == nParts {
			break
		}

		eS, eConsumed, eErr := deserializeAny(typ, parts[consumed:])
		if err = eErr; err != nil {
			// We don't care about consumed, no deserializer
			// will continue parsing if it sees an error
			return
		}
		val.Index(consumed).Set(reflect.ValueOf(eS))
		consumed += eConsumed
		// ^ Can never exceed nParts, no deserializer
		// will consume more parts than are available to us
	}

	s = val.Interface()
	return
}

func deserializeStruct(typ reflect.Type, parts []string) (s any, consumed int, err error) {
	val := reflect.Indirect(reflect.New(typ))

	for f, fVal := range val.Fields() {
		fS, fConsumed, fErr := deserializeAny(f.Type, parts[consumed:])
		if err = fErr; err != nil {
			return
		}
		fVal.Set(reflect.ValueOf(fS))
		consumed += fConsumed
	}

	s = val.Interface()
	return
}

func deserializeAny(typ reflect.Type, parts []string) (s any, consumed int, err error) {
	// Handle types containing other types
	switch typ.Kind() {
	case reflect.Struct:
		return deserializeStruct(typ, parts)
	case reflect.Slice:
		return deserializeSlice(typ, parts)
	}

	val := reflect.Indirect(reflect.New(typ))
	part := parts[0]

	switch val.Interface().(type) {
	// [string]s don't need any change
	case string:
		val.SetString(part)
	// [int32]/[int64] need to get parsed from string
	case int32, int64:
		n, dErr := strconv.ParseInt(part, 10, typ.Bits())
		if err = dErr; err != nil {
			return
		}
		val.SetInt(n)

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

	case int, uint, uintptr:
		// Only allow ints of specified bitness
		panic("deserialize int of unspecified size")
	default:
		panic("deserialize unhandled type")
	}

	consumed++
	s = val.Interface()
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
	var consumed int
	msg, consumed, err = deserializeAny(typ, partsNameless)
	if err != nil {
		return
	}

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
