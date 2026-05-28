package tcpproto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/Preloading/SkyglowNotificationServer/router"
	"github.com/google/uuid"
)

const (
	V2ProtocolVersion = 0x02
)

type clientMessage struct {
	messageType uint8
	data        []byte
	offset      uint
}

func handleV2Connection(c net.Conn, channel chan router.DataUpdate) {
	// var rsaClientPublicKey *rsa.PublicKey
	// client info
	userAddress := ""
	// device := &db.Device{}
	// userLang := ""
	// // auth
	var authenticationNonce []byte
	var clientPubKey *rsa.PublicKey

	isRegistering := false
	isAuthenticated := false

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
		messageData := make([]byte, messageSize)
		n, err = c.Read(messageData)
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
		message := clientMessage{
			messageType: messageId,
			data:        messageData,
		}

		// lastContactTimestamp = time.Now().Unix() // feed the dog
		log.Printf("got a packet 0x%x\n", messageId)
		switch messageId {
		// global protocol stuff
		case 0x27: // ping
			sendMessageToClientV2(c, messageData, 0x16) // pings can miss, thats fine.
		case 0x24: // client disconnect
			log.Printf("%s has disconnected with code \n %d", c.RemoteAddr().String(), message.readInt8())
			return

		// unauthenticated
		case 0x28: // register
			if isAuthenticated {
				disconnectClientV2(c, 0x02, 0)
				return
			}

			rawPubKey := message.readBytesWithLen(uint(message.readUint16()))
			fmt.Printf("public key: %x\n", rawPubKey)

			pubInterface, err := x509.ParsePKIXPublicKey(rawPubKey)
			if err != nil {
				disconnectClientV2(c, 0x02, 0)
				return
			}

			var ok bool
			clientPubKey, ok = pubInterface.(*rsa.PublicKey)
			if !ok {
				disconnectClientV2(c, 0x02, 0)
				return
			}

			isRegistering = true

			// create challenge
			authenticationNonce = make([]byte, 32)
			_, err = rand.Read(authenticationNonce)
			if err != nil {
				panic(fmt.Errorf("could not generate nonce")) // worthy of a panic
			}

			log.Printf("%s is attempting to register\n", c.RemoteAddr().String())
			if err := sendMessageToClientV2(c, authenticationNonce, 0x11); err != nil {
				return
			}

		case 0x29:
			if !isRegistering || isAuthenticated {
				disconnectClientV2(c, 0x02, 0)
				return
			}

			// check the sig
			authUnixTimestamp := message.readInt64()
			currentTimestamp := time.Now().UTC().Unix()
			if authUnixTimestamp > currentTimestamp+300 || authUnixTimestamp < currentTimestamp-300 {
				disconnectClientV2(c, 0x01, 0)
				return
			}

			signature := message.readBytesWithLen(uint(message.readUint16()))
			expectedData := make([]byte, 8)
			binary.BigEndian.PutUint64(expectedData, uint64(authUnixTimestamp))
			expectedData = append(authenticationNonce, expectedData...)

			msgHash := sha256.New()
			_, err = msgHash.Write(expectedData)
			if err != nil {
				disconnectClientV2(c, 0x03, 0)
				return
			}
			msgHashSum := msgHash.Sum(nil)

			err = rsa.VerifyPSS(clientPubKey, crypto.SHA256, msgHashSum, signature, nil)
			if err != nil {
				fmt.Println("could not verify signature: ", err)
				disconnectClientV2(c, 0x02, 0)
				return
			}
			// creating the user time
			uuid := uuid.New().String()
			uuidWithoutHyphens := strings.Replace(uuid, "-", "", -1)

			userAddress = fmt.Sprintf("%s@%s", uuidWithoutHyphens, configData.ServerAddress)

			err := db.SaveNewUser(userAddress, *clientPubKey)
			if err != nil {
				disconnectClientV2(c, 0x03, 0)
				return
			}

			isAuthenticated = true

			log.Printf("%s has registered a new account (%s)\n", c.RemoteAddr().String(), userAddress)

			payload := []byte{0x00, 0x00, 0x00, 0x02} // why
			addToPayload(&payload, uint16(len(userAddress)))
			addToPayload(&payload, userAddress)
			sendMessageToClientV2(c, payload, 0x18)
		case 0x00:

		default:
		}
	}
}

func (m *clientMessage) readUint8() uint8 {
	data := uint8(m.data[m.offset])
	m.offset += 1
	return data
}

func (m *clientMessage) readUint16() uint16 {
	data := uint16(m.data[m.offset])<<8 | uint16(m.data[m.offset+1])
	m.offset += 2
	return data
}

func (m *clientMessage) readUint32() uint32 {
	data := uint32(m.data[m.offset])<<24 | uint32(m.data[m.offset+1])<<16 | uint32(m.data[m.offset+2])<<8 | uint32(m.data[m.offset+3])
	m.offset += 4
	return data
}

func (m *clientMessage) readUint64() uint64 {
	data := uint64(m.data[m.offset])<<56 | uint64(m.data[m.offset+1])<<48 | uint64(m.data[m.offset+2])<<40 | uint64(m.data[m.offset+3]<<32) | uint64(m.data[m.offset+4])<<24 | uint64(m.data[m.offset+5])<<16 | uint64(m.data[m.offset+6])<<8 | uint64(m.data[m.offset+7])
	m.offset += 8
	return data
}

func (m *clientMessage) readInt8() int8 {
	data := int8(m.data[m.offset])
	m.offset += 1
	return data
}

func (m *clientMessage) readInt16() int16 {
	data := int16(m.data[m.offset])<<8 | int16(m.data[m.offset+1])
	m.offset += 2
	return data
}

func (m *clientMessage) readInt32() int32 {
	data := int32(m.data[m.offset])<<24 | int32(m.data[m.offset+1])<<16 | int32(m.data[m.offset+2])<<8 | int32(m.data[m.offset+3])
	m.offset += 4
	return data
}

func (m *clientMessage) readInt64() int64 {
	data := int64(m.data[m.offset])<<56 | int64(m.data[m.offset+1])<<48 | int64(m.data[m.offset+2])<<40 | int64(m.data[m.offset+3]<<32) | int64(m.data[m.offset+4])<<24 | int64(m.data[m.offset+5])<<16 | int64(m.data[m.offset+6])<<8 | int64(m.data[m.offset+7])
	m.offset += 8
	return data
}

func (m *clientMessage) readStringWithLen(len uint) string {
	data := string(m.data[m.offset : m.offset+len])
	m.offset += len
	return data
}

func (m *clientMessage) readBytesWithLen(len uint) []byte {
	data := m.data[m.offset : m.offset+len]
	m.offset += len
	return data
}

func disconnectClientV2(c net.Conn, reason uint8, reconnectAfter uint32) {
	payload := []byte{reason}
	addToPayload(&payload, reconnectAfter)
	log.Printf("disconnected client for %d", reason)
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
	case uint16:
		*payload = append(*payload, byte(v>>8), byte(v))

	case int8:
		*payload = append(*payload, byte(v))
	case byte:
		*payload = append(*payload, v)
	case string:
		*payload = append(*payload, []byte(v)...)

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
