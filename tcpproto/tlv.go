package tcpproto

import (
	"fmt"
	"math"
)

func ConvertToTLV(data interface{}) []byte {
	payload := []byte{}
	switch v := data.(type) {
	case map[string]interface{}:
		mapData := []byte{}
		for key, value := range v {
			dV := len(key)
			addToPayload(&mapData, uint16(dV))
			addToPayload(&mapData, key)
			mapData = append(mapData, ConvertToTLV(value)...)

		}
		payload = []byte{0x01}
		addToPayload(&payload, uint32(len(mapData)))
		addToPayload(&payload, mapData)
	case []interface{}:
		mapData := []byte{}
		for _, value := range v {
			mapData = append(mapData, ConvertToTLV(value)...)
		}
		payload = []byte{0x02}
		addToPayload(&payload, uint32(len(mapData)))
		addToPayload(&payload, mapData)
	case string:
		payload = []byte{0x03}
		addToPayload(&payload, uint32(len(v)))
		addToPayload(&payload, v)
	case int:
		payload = []byte{0x04, 0x00, 0x00, 0x00, 0x08}
		addToPayload(&payload, int64(v))
	case bool:
		payload = []byte{0x06, 0x00, 0x00, 0x00, 0x01}
		if v {
			payload = append(payload, 0x01)
		} else {
			payload = append(payload, 0x00)
		}
	case nil:
		payload = []byte{0x07, 0x00, 0x00, 0x00, 0x00}
	case float64:
		payload = []byte{0x05, 0x00, 0x00, 0x00, 0x08}
		addToPayload(&payload, math.Float64bits(v))
	default:
		fmt.Printf("type of %T is not implemented in TLV! ignoring...\n", data)
	}

	// fmt.Printf("incoming data is type of: %T\n", data)
	return payload
}
