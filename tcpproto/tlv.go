package tcpproto

import (
	"errors"
	"fmt"
	"io"
	"math"
)

// AppendVarint encodes a uint64 into a varint byte sequence and appends it to the buffer.
func AppendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v|0x80))
		v >>= 7
	}
	return append(buf, byte(v))
}

// ReadVarint decodes a varint from an io.ByteReader.
// This matches the 10-byte protection limit found in the Objective-C code.
func ReadVarint(r io.ByteReader) (uint64, error) {
	var v uint64
	var shift uint

	for i := 0; i < 10; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}

		// Check for potential overflow on the 10th byte (shift == 63)
		if shift == 63 && (b&0x7E) != 0 {
			return 0, errors.New("varint overflow")
		}

		v |= uint64(b&0x7F) << shift
		if (b & 0x80) == 0 {
			return v, nil
		}
		shift += 7
	}

	return 0, errors.New("malformed varint: too many bytes")
}

// ZigZagEncode converts a signed int64 into an unsigned uint64.
// This makes negative numbers small integers, optimizing them for Varint compression.
func ZigZagEncode(n int64) uint64 {
	return uint64((n << 1) ^ (n >> 63))
}

// ZigZagDecode converts an unsigned uint64 back into a signed int64.
func ZigZagDecode(u uint64) int64 {
	return int64(u>>1) ^ -int64(u&1)
}

func ConvertToTLV(data interface{}) []byte {
	payload := []byte{}
	switch v := data.(type) {
	case map[string]interface{}:
		mapData := []byte{}
		for key, value := range v {
			dV := len(key)
			mapData = AppendVarint(mapData, uint64(dV))
			addToPayload(&mapData, key)
			mapData = append(mapData, ConvertToTLV(value)...)

		}
		payload = []byte{0x01}
		payload = AppendVarint(payload, uint64(len(mapData)))
		addToPayload(&payload, mapData)
	case []interface{}:
		mapData := []byte{}
		for _, value := range v {
			mapData = append(mapData, ConvertToTLV(value)...)
		}
		payload = []byte{0x02}
		payload = AppendVarint(payload, uint64(len(mapData)))
		addToPayload(&payload, mapData)
	case string:
		payload = []byte{0x03}
		payload = AppendVarint(payload, uint64(len(v)))
		addToPayload(&payload, v)
	case int:
		payload = []byte{0x04}
		payload = AppendVarint(payload, ZigZagEncode(int64(v)))
	case bool:
		payload = []byte{0x06}
		if v {
			payload = append(payload, 0x01)
		} else {
			payload = append(payload, 0x00)
		}
	case nil:
		payload = []byte{0x07}
	case float64:
		payload = []byte{0x05}
		addToPayload(&payload, math.Float64bits(v))
	default:
		fmt.Printf("type of %T is not implemented in TLV! ignoring...\n", data)
	}

	// fmt.Printf("incoming data is type of: %T\n", data)
	return payload
}
