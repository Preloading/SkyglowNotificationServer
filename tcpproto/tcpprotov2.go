package tcpproto

import (
	"errors"
	"log"
	"net"

	"github.com/Preloading/SkyglowNotificationServer/router"
)

const (
	V2ProtocolVersion = 0x02
)

func handleV2Connection(c net.Conn, channel chan router.DataUpdate) {
	// var rsaClientPublicKey *rsa.PublicKey
	// client info
	// userAddress := ""
	// device := &db.Device{}
	// userLang := ""
	// // auth
	// authTimestamp := ""
	// authenticationNonce := ""

	// isAuthenticated := false

	// lastContactTimestamp := time.Now().Unix()

	// send hello
	helloPayload := []byte{}
	addToPayload(&helloPayload, uint32(V2ProtocolVersion))

	if err := sendMessageToClientV2(c, helloPayload, 0x10); err != nil {
		return
	}

	for {
		header := make([]byte, 8)
		n, err := c.Read(header)
		if err != nil {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			disconnectClientV2(c, 0x02, 0)
			return
		}
		if n != 8 {
			log.Printf("Header len mismatch from %s: %v\n", c.RemoteAddr().String(), err)
			disconnectClientV2(c, 0x02, 0)
			return
		}

		// magic value
		if header[0] != 0x53 {
			log.Printf("Magic value missing from %s", c.RemoteAddr().String())
			disconnectClientV2(c, 0x02, 0)
			return
		}

		// check version
		if !(header[1] <= 0x02) {
			log.Printf("Version of client %s is too outdated", c.RemoteAddr().String())
			disconnectClientV2(c, 0x02, 0)
			return
		}

		messageSize := uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7]) // hate. this was copilot, if there's a nicer way, PR.

		if messageSize > 4096 { // spec says this is the max packet size, can probably be risen later on
			log.Printf("Protocol violaton: message size too big (%d vs 4096) for %s", messageSize, c.RemoteAddr().String())
			disconnectClientV2(c, 0x02, 0)
			return
		}
		messageId := header[2]
		message := make([]byte, messageSize)
		n, err = c.Read(message)
		if err != nil {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			disconnectClientV2(c, 0x02, 0)
			return
		}
		if n != int(messageSize) {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			disconnectClientV2(c, 0x02, 0)
			return
		}

		// we now have the message

		// lastContactTimestamp = time.Now().Unix() // feed the dog
		log.Printf("got a packet 0x%x\n", messageId)
		// global protocol stuff
		switch messageId {
		case 0x27: // ping
			sendMessageToClientV2(c, message, 0x16) // pings can miss, thats fine.
		case 0x24:
			log.Printf("%s has disconnected with code \n %d", c.RemoteAddr().String(), int(message[0]))
			return
		default:
		}
	}
}

func disconnectClientV2(c net.Conn, reason uint8, reconnectAfter uint32) {
	payload := []byte{reason}
	addToPayload(&payload, reconnectAfter)

	sendMessageToClientV2(c, payload, 0x14)
}

func addToPayload(payload *[]byte, data interface{}) {
	switch v := data.(type) {
	case int64:
		*payload = append(*payload, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	case uint64:
		*payload = append(*payload, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	case int:
		val := uint32(v)
		*payload = append(*payload, byte(val>>24), byte(val>>16), byte(val>>8), byte(val))
	case int32:
		val := uint32(v)
		*payload = append(*payload, byte(val>>24), byte(val>>16), byte(val>>8), byte(val))

	case uint32:
		*payload = append(*payload, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))

	case int16:
		*payload = append(*payload, byte(v>>8), byte(v))

	case int8:
		*payload = append(*payload, byte(v))
	case byte:
		*payload = append(*payload, v)

	default:
		panic("type not implemented in tcpprotov2")
	}
}

func sendMessageToClientV2(c net.Conn, payload []byte, messageType uint8) error {
	// header
	_, err := c.Write([]byte{0x53, V2ProtocolVersion, byte(messageType), 0x00})
	if err != nil {
		return err
	}

	// length
	messageLen := make([]byte, 4)
	length := len(payload)
	log.Printf("Length %d\n", length)
	messageLen[0] = byte(length >> 24)
	messageLen[1] = byte(length >> 16)
	messageLen[2] = byte(length >> 8)
	messageLen[3] = byte(length)

	_, err = c.Write(messageLen)
	if err != nil {
		log.Printf("Write error to %s: %v\n", c.RemoteAddr().String(), err)
		return errors.New("write error in len")
	}

	_, err = c.Write(payload)
	if err != nil {
		log.Printf("Write error to %s: %v\n", c.RemoteAddr().String(), err)
		return errors.New("write error in data")
	}
	log.Printf("sent a packet with 0x%x\n", messageType)
	return nil
}

// func sendNotificationToClientV2(c net.Conn, data router.DataToSend) error {
// 	dataToSend := Notification{
// 		Message:    MessageV1{Type: 2},
// 		DataToSend: data,
// 	}

// 	if err := sendMessageToClientV1(c, dataToSend, 2); err != nil {
// 		return err
// 	}
// 	return nil
// }
