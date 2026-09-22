// YNO protocol serializer/deserializer implementation that
// converts between Go structs and the string-based network format.
//
// For the Go structs that represent S->C and C->S packets,
// see [messages.go]
package protocol

var (
	paramDelim   = []byte{0xef, 0xbf, 0xbf}
	messageDelim = []byte{0xef, 0xbf, 0xbe}
)
