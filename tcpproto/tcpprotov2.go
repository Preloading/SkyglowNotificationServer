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
	"github.com/Preloading/SkyglowNotificationServer/feedbackmgr"
	"github.com/Preloading/SkyglowNotificationServer/router"
	"github.com/google/uuid"
)

const (
	V2ProtocolVersion                    = 0x02
	V2MinProtocolVersion                 = 0x02
	SERVER_DISCONNECT_NORMAL             = 0x00
	SERVER_DISCONNECT_AUTH_FAIL          = 0x01
	SERVER_DISCONNECT_PROTOCOL_ERROR     = 0x02
	SERVER_DISCONNECT_INTERNAL_ERROR     = 0x03
	SERVER_DISCONNECT_REPLACED           = 0x04
	SERVER_DISCONNECT_VERSION_MISMATCHED = 0x05
)

type clientMessage struct {
	messageType uint8
	data        []byte
	offset      uint
}

type tokenUpdate struct {
	enabledState uint8
	routingKey   []byte
	bundle_id    string
}

func handleV2Connection(c net.Conn, channel chan router.DataUpdate) {
	// var rsaClientPublicKey *rsa.PublicKey
	// client info
	userAddress := ""
	device := &db.Device{}
	// userLang := ""
	// // auth
	var authenticationNonce []byte
	var clientPubKey *rsa.PublicKey

	isRegistering := false
	loginPhase := 0
	isAuthenticated := false
	reloadedTokens := []tokenUpdate{}

	readNotifications := func() {
		for msg := range channel {
			if msg.Disconnect {
				log.Printf("Disconnecting from %s\n", c.RemoteAddr().String())
				return
			}
			log.Printf("[%s] Sending Message from channel\n", c.RemoteAddr().String())
			if err := sendNotificationToClientV2(c, msg.DataToSend); err != nil {
				if err.Error() == "write error" {
					log.Printf("Write error to %s, disconnecting...\n", c.RemoteAddr().String())
					return
				}
				log.Printf("Error sending notification to %s: %v\n", c.RemoteAddr().String(), err)
				disconnectClientV2(c, SERVER_DISCONNECT_INTERNAL_ERROR, 0)
				return
			}
		}
	}

	// lastContactTimestamp := time.Now().Unix()

	// send hello
	helloPayload := []byte{}
	addToPayload(&helloPayload, uint32(V2ProtocolVersion))

	if err := sendMessageToClientV2(c, helloPayload, 0x10); err != nil {
		return
	}

	for {
		if err := c.SetReadDeadline(time.Now().Add(60 * time.Minute)); err != nil {
			log.Printf("Error setting read deadline for %s: %v\n", c.RemoteAddr().String(), err)
			return
		}
		header := make([]byte, 8)
		n, err := c.Read(header)
		if err != nil {
			log.Printf("Read error in packet from %s: %v, disconnecting\n", c.RemoteAddr().String(), err)
			disconnectClientV2(c, 0x02, 0)
			return
		}
		if n != 8 {
			log.Printf("Header len mismatch in packet from %s: %v, disconnecting\n", c.RemoteAddr().String(), err)
			disconnectClientV2(c, 0x02, 0)
			return
		}

		// magic value
		if header[0] != 0x53 {
			log.Printf("Magic value missing in packet from %s, disconnecting....", c.RemoteAddr().String())
			disconnectClientV2(c, 0x02, 0)
			return
		}

		// check version
		if !(header[1] <= V2MinProtocolVersion) {
			log.Printf("Version of client with IP %s is too outdated, disconnecting...", c.RemoteAddr().String())
			disconnectClientV2(c, SERVER_DISCONNECT_VERSION_MISMATCHED, 0)
			return
		}

		messageSize := uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7]) // hate. this was copilot, if there's a nicer way, PR.

		if messageSize > 4096 { // spec says this is the max packet size, can probably be risen later on
			log.Printf("Protocol violaton: message size too big (%d vs 4096) for connection from IP %s", messageSize, c.RemoteAddr().String())
			disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
			return
		}
		messageId := header[2]
		messageData := make([]byte, messageSize)
		n, err = c.Read(messageData)
		if err != nil {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
			return
		}
		if n != int(messageSize) {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
			return
		}

		// we now have the message
		message := clientMessage{
			messageType: messageId,
			data:        messageData,
		}

		// lastContactTimestamp = time.Now().Unix() // feed the dog
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
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}

			rawPubKey := message.readBytesWithLen(uint(message.readUint16()))
			fmt.Printf("public key: %x\n", rawPubKey)

			pubInterface, err := x509.ParsePKIXPublicKey(rawPubKey)
			if err != nil {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}

			var ok bool
			clientPubKey, ok = pubInterface.(*rsa.PublicKey)
			if !ok {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
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
		case 0x20: // login
			if loginPhase != 0 {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}

			userAddress = message.readStringWithLen(uint(message.readUint16()))
			if userAddress == "" {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}
			clockSkewedTS := message.readInt64()
			currentTimestamp := time.Now().UTC().Unix()
			if clockSkewedTS > currentTimestamp+300 || clockSkewedTS < currentTimestamp-300 {
				disconnectClientV2(c, SERVER_DISCONNECT_AUTH_FAIL, 0)
				return
			}

			// load client data
			device, err = db.GetUser(userAddress)
			if err != nil {
				disconnectClientV2(c, SERVER_DISCONNECT_AUTH_FAIL, 0)
				return
			}

			clientPubKey = device.PublicKey

			// create challenge
			authenticationNonce = make([]byte, 32)
			_, err = rand.Read(authenticationNonce)
			if err != nil {
				panic(fmt.Errorf("could not generate nonce")) // worthy of a panic
			}

			log.Printf("%s is attempting to login to %s\n", c.RemoteAddr().String(), device.DeviceAddress)

			// send da challenge
			if err := sendMessageToClientV2(c, authenticationNonce, 0x11); err != nil {
				return
			}
			loginPhase = 1
		case 0x29: // register resp
			if !isRegistering || isAuthenticated {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}

			// check the sig
			authUnixTimestamp := message.readInt64()
			currentTimestamp := time.Now().UTC().Unix()
			if authUnixTimestamp > currentTimestamp+300 || authUnixTimestamp < currentTimestamp-300 {
				disconnectClientV2(c, SERVER_DISCONNECT_AUTH_FAIL, 0)
				return
			}

			signature := message.readBytesWithLen(uint(message.readUint16()))
			expectedData := make([]byte, 8)
			binary.BigEndian.PutUint64(expectedData, uint64(authUnixTimestamp))
			expectedData = append(authenticationNonce, expectedData...)

			msgHash := sha256.New()
			_, err = msgHash.Write(expectedData)
			if err != nil {
				disconnectClientV2(c, SERVER_DISCONNECT_INTERNAL_ERROR, 0)
				return
			}
			msgHashSum := msgHash.Sum(nil)

			err = rsa.VerifyPSS(clientPubKey, crypto.SHA256, msgHashSum, signature, nil)
			if err != nil {
				fmt.Println("could not verify signature: ", err)
				disconnectClientV2(c, SERVER_DISCONNECT_AUTH_FAIL, 0)
				return
			}
			// creating the user time
			uuid := uuid.New().String()
			uuidWithoutHyphens := strings.Replace(uuid, "-", "", -1)

			userAddress = fmt.Sprintf("%s@%s", uuidWithoutHyphens, configData.ServerAddress)

			err := db.SaveNewUser(userAddress, *clientPubKey)
			if err != nil {
				disconnectClientV2(c, SERVER_DISCONNECT_INTERNAL_ERROR, 0)
				return
			}

			// load client data. is it a bit wasteful? kinda. do i care? no
			device, err = db.GetUser(userAddress)
			if err != nil {
				disconnectClientV2(c, SERVER_DISCONNECT_INTERNAL_ERROR, 0)
				return
			}

			clientPubKey = device.PublicKey

			isAuthenticated = true
			loginPhase = 99999

			log.Printf("%s has registered a new account (%s)\n", c.RemoteAddr().String(), userAddress)

			// start notification stream
			go readNotifications()
			router.AddConnection(userAddress, channel)
			defer router.RemoveConnection(userAddress)

			payload := []byte{0x00, 0x00, 0x00, V2ProtocolVersion} // why
			addToPayload(&payload, uint16(len(userAddress)))
			addToPayload(&payload, userAddress)
			sendMessageToClientV2(c, payload, 0x18)
		case 0x21: // login resp
			if loginPhase != 1 {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}

			// check the sig
			authUnixTimestamp := message.readInt64()
			currentTimestamp := time.Now().UTC().Unix()
			if authUnixTimestamp > currentTimestamp+300 || authUnixTimestamp < currentTimestamp-300 {
				disconnectClientV2(c, SERVER_DISCONNECT_AUTH_FAIL, 0)
				return
			}

			signature := message.readBytesWithLen(uint(message.readUint16()))
			timestampEncoded := make([]byte, 8)
			binary.BigEndian.PutUint64(timestampEncoded, uint64(authUnixTimestamp))
			expectedData := append(authenticationNonce, []byte(device.DeviceAddress)...)
			expectedData = append(expectedData, timestampEncoded...)

			msgHash := sha256.New()
			_, err = msgHash.Write(expectedData)
			if err != nil {
				disconnectClientV2(c, SERVER_DISCONNECT_INTERNAL_ERROR, 0)
				return
			}
			msgHashSum := msgHash.Sum(nil)

			err = rsa.VerifyPSS(clientPubKey, crypto.SHA256, msgHashSum, signature, nil)
			if err != nil {
				fmt.Println("could not verify signature: ", err)
				disconnectClientV2(c, SERVER_DISCONNECT_AUTH_FAIL, 0)
				return
			}

			// we passed :D
			clientPubKey = device.PublicKey

			isAuthenticated = true
			loginPhase = 99999

			// start notification stream
			go readNotifications()
			router.AddConnection(userAddress, channel)
			defer router.RemoveConnection(userAddress)

			sendMessageToClientV2(c, nil, 0x12)

		// authenticated land
		case 0x22: // poll
			if !isAuthenticated {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}

			pollAfter := message.readUint64()

			unackedNotifications, err := db.GetUnacknowledgedMessagesAfterUnixTime(userAddress, time.Unix(int64(pollAfter), 0))
			if err != nil {
				fmt.Println(err.Error())
				disconnectClientV2(c, SERVER_DISCONNECT_INTERNAL_ERROR, 0)
				return
			}
			if len(unackedNotifications) == 0 {
				continue
			}
			for _, unackedNotification := range unackedNotifications {
				log.Printf("Sending %s a message from database\n", device.DeviceAddress)
				if unackedNotification.IsEncrypted {
					sendNotificationToClientV2(c, router.DataToSend{
						IsEncrypted: unackedNotification.IsEncrypted,

						Ciphertext: *unackedNotification.Ciphertext,
						DataType:   *unackedNotification.DataType,
						IV:         *unackedNotification.IV,

						DeviceAddress: unackedNotification.DeviceAddress,
						RoutingKey:    unackedNotification.RoutingKey,
						MessageId:     unackedNotification.MessageId,

						CreatedAt: unackedNotification.CreatedAt,
					})
				} else {
					sendNotificationToClientV2(c, router.DataToSend{
						IsEncrypted: unackedNotification.IsEncrypted,

						Data: unackedNotification.Data,

						DeviceAddress: unackedNotification.DeviceAddress,
						RoutingKey:    unackedNotification.RoutingKey,
						MessageId:     unackedNotification.MessageId,

						CreatedAt: unackedNotification.CreatedAt,
					})
				}
			}
		case 0x23: // C_ACK
			if !isAuthenticated {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}

			uuidRawBytes := message.readBytesWithLen(16)
			// status := message.readUint8()
			messageId, err := uuid.FromBytes(uuidRawBytes)
			if err != nil {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}
			db.AckMessage(messageId.String(), userAddress)
		case 0x2b:
			if !isAuthenticated {
				disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
				return
			}

			flag := message.readUint8()
			entriesInChunk := message.readUint16()
			for i := 0; i < int(entriesInChunk); i++ {
				tag := message.readUint8()
				reloadedTokens = append(reloadedTokens, tokenUpdate{
					enabledState: tag,
					routingKey:   message.readBytesWithLen(32),
					bundle_id:    message.readStringWithLen(uint(message.readUint16())),
				})
				log.Printf("a new token %x\n", reloadedTokens[0].routingKey)
			}
			if flag == 0x00 { // completed download
				currentTokensPtr, err := db.GetAllTokens(userAddress)
				if err != nil {
					log.Fatalf("failed to fetch tokens for user %s\n", userAddress)
					disconnectClientV2(c, SERVER_DISCONNECT_INTERNAL_ERROR, 0)
					return
				}

				currentTokens := *currentTokensPtr

				oldTokensMap := make(map[string]db.NotificationToken, len(currentTokens))
				for _, oldToken := range currentTokens {
					oldTokensMap[string(oldToken.RoutingToken)] = oldToken
				}

				createdTokens := []db.NotificationToken{}
				modfiedTokens := []db.NotificationToken{}

				for _, newToken := range reloadedTokens {
					keyStr := string(newToken.routingKey)
					oldVersion, ok := oldTokensMap[keyStr]
					if ok {
						if oldVersion.DeviceAddress != userAddress {
							disconnectClientV2(c, SERVER_DISCONNECT_PROTOCOL_ERROR, 0)
							return
						}
						oldVersion.NotificationType = int(newToken.enabledState)
						oldVersion.IsValid = true
						modfiedTokens = append(modfiedTokens, oldVersion)

						delete(oldTokensMap, keyStr)
					} else {
						newToken := db.NotificationToken{
							RoutingToken:            newToken.routingKey,
							DeviceAddress:           userAddress,
							FeedbackProviderAddress: nil,
							NotificationType:        int(newToken.enabledState),
							AppBundleId:             newToken.bundle_id,
							IssuedAt:                time.Now(),
							IsValid:                 true,
							MarkedForRemovalAt:      nil,
							LastUsed:                nil,
						}
						createdTokens = append(createdTokens, newToken)
					}
				}

				removedTokens := [][]byte{}

				for _, removedToken := range oldTokensMap {
					removedTokens = append(removedTokens, removedToken.RoutingToken)
					feedbackmgr.RemoveToken(0, "unknown", removedToken.RoutingToken, removedToken.DeviceAddress, removedToken.FeedbackProviderAddress)
				}

				if err := db.SyncTokens(removedTokens, createdTokens, modfiedTokens); err != nil {
					log.Fatalf("failed to sync tokens for user %s.\n", userAddress)
					disconnectClientV2(c, SERVER_DISCONNECT_INTERNAL_ERROR, 0)
					return
				}
			}
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
	case []byte:
		*payload = append(*payload, []byte(v)...)
	case string:
		*payload = append(*payload, []byte(v)...)
	case time.Time:
		u := v.Unix()
		*payload = append(*payload, byte(u>>56), byte(u>>48), byte(u>>40), byte(u>>32), byte(u>>24), byte(u>>16), byte(u>>8), byte(u))

	default:
		panic(fmt.Sprintf("type %T not implemented in tcpprotov2", data))
	}
}

func sendMessageToClientV2(c net.Conn, payload []byte, messageType uint8) error {
	// header

	messageData := []byte{0x53, V2ProtocolVersion, byte(messageType), 0x00,
		0x00, 0x00, 0x00, 0x00} // placeholder for length

	// length
	length := len(payload)
	messageData[4] = byte(length >> 24)
	messageData[5] = byte(length >> 16)
	messageData[6] = byte(length >> 8)
	messageData[7] = byte(length)

	messageData = append(messageData, payload...)
	_, err := c.Write(messageData)
	if err != nil {
		log.Printf("Write error to %s: %v\n", c.RemoteAddr().String(), err)
		return errors.New("write error sending message")
	}
	return nil
}

// SGPayloadFormatTLV       = 0x00, depricated
// SGPayloadFormatJSON      = 0x01,
// SGPayloadFormatPlist     = 0x02,
// SGPayloadFormatTLVStruct = 0x03,

func sendNotificationToClientV2(c net.Conn, data router.DataToSend) error {
	payload := []byte{}
	addToPayload(&payload, data.RoutingKey)
	// addToPayload(&payload, data.MessageId)

	messageId := uuid.MustParse(data.MessageId)
	uuidRaw, err := messageId.MarshalBinary()
	if err != nil {
		panic(err)
	}
	addToPayload(&payload, uuidRaw)
	addToPayload(&payload, data.CreatedAt)
	addToPayload(&payload, uint64(0)) // expiration
	flags := byte(0x00)
	if data.IsEncrypted {
		flags |= (1 << 0) // set encryption byte at pos 0
	}
	addToPayload(&payload, flags)

	if data.IsEncrypted {
		switch data.DataType {
		case "json":
			addToPayload(&payload, uint8(0x01))
		case "plist":
			addToPayload(&payload, uint8(0x02))
		case "tlv":
			addToPayload(&payload, uint8(0x03))
		}
		addToPayload(&payload, uint32(len(data.Ciphertext)))
		addToPayload(&payload, data.Ciphertext)
		addToPayload(&payload, data.IV)
	} else {
		addToPayload(&payload, uint8(0x03))

		tlv := ConvertToTLV(data.Data)
		// unencryptedNotification, err := plist.Marshal(data.Data, plist.BinaryFormat)
		// if err != nil {
		// 	return err
		// }
		addToPayload(&payload, uint32(len(tlv)))
		addToPayload(&payload, tlv)
	}

	if err := sendMessageToClientV2(c, payload, 0x13); err != nil {
		return err
	}
	return nil
}
