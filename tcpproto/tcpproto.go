package tcpproto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/Preloading/SkyglowNotificationServer/config"
	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/Preloading/SkyglowNotificationServer/router"
	"howett.net/plist"
)

var (
	keys       config.CryptoKeys
	configData config.Config
)

type Message struct {
	Type int `plist:"$type"`
}

type LoginChallenge struct {
	Message
	Challenge []byte `plist:"challenge"`
}

type AckDeviceToken struct {
	Message
	RoutingToken []byte `plist:"routingToken"`
	BundleId     string `plist:"bundleId"`
}

type Notification struct {
	Message
	router.DataToSend
}

func CreateTCPServer(port uint16, _keys config.CryptoKeys, _config config.Config) {
	keys = _keys
	configData = _config
	PORTSTR := ":" + strconv.FormatUint(uint64(port), 10)

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*_keys.ServerTLSCert},
		MinVersion:   tls.VersionTLS13,
	}

	// Use TLS listener instead of raw TCP
	l, err := tls.Listen("tcp", PORTSTR, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	log.Printf("TLS server listening on port %d", port)

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go handleConnection(c)
	}
}

func handleConnection(c net.Conn) {
	log.Printf("Client Connected: %s\n", c.RemoteAddr().String())
	defer c.Close()
	// connectionUUID := ""
	channel := make(chan router.DataUpdate)
	// var rsaClientPublicKey *rsa.PublicKey
	defer close(channel)

	// client info
	userAddress := ""
	device := &db.Device{}
	userLang := ""
	// auth
	authTimestamp := ""
	authenticationNonce := ""

	isAuthenticated := false
	messageLen := make([]byte, 4)

	// send hello
	if err := sendMessageToClient(c, nil, 0); err != nil {
		return
	}

	for {
		n, err := c.Read(messageLen)
		if err != nil {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			sendMessageToClient(c, nil, 4)
			return
		}
		if n != 4 {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			sendMessageToClient(c, nil, 4)
			return
		}
		messageSize := uint32(messageLen[0])<<24 | uint32(messageLen[1])<<16 | uint32(messageLen[2])<<8 | uint32(messageLen[3]) // hate. this was copilot, if there's a nicer way, PR.
		// fmt.Printf("Receiving a message with a length of %d\n", messageSize)

		// Read the data we care about
		plistMessage := make([]byte, messageSize)
		n, err = c.Read(plistMessage)
		if err != nil {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			sendMessageToClient(c, nil, 4)
			return
		}
		if n != int(messageSize) {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			sendMessageToClient(c, nil, 4)
			return
		}

		message := make(map[string]interface{})
		if _, err := plist.Unmarshal(plistMessage, message); err != nil {
			log.Printf("Read error (plist decode) from %s: %v\n", c.RemoteAddr().String(), err)
			sendMessageToClient(c, nil, 4)
			return
		}

		//////////////////////////////////////
		//        Message Handling          //
		//////////////////////////////////////

		if typeVal, ok := message["$type"].(uint64); ok {
			if !isAuthenticated {
				switch typeVal {
				case 0: // Login Request
					log.Printf("A client just asked to login! from %s\n", c.RemoteAddr().String())
					fmt.Printf("Login request from address: %v, version: %v\n", message["address"], message["version"])

					// verify they actually sent what we need
					userAddress, ok = message["address"].(string)
					if !ok {
						sendMessageToClient(c, nil, 4)
						return
					}
					// load client data
					device, err = db.GetUser(userAddress)
					if err != nil {
						sendMessageToClient(c, nil, 4)
						return
					}

					// create challenge
					authTimestamp = fmt.Sprint(time.Now().UTC().Unix())
					authenticationNonceBytes := make([]byte, 32)
					_, err := rand.Read(authenticationNonceBytes)
					if err != nil {
						panic(fmt.Errorf("could not generate nonce")) // worthy of a panic
					}
					authenticationNonce = base64.StdEncoding.EncodeToString(authenticationNonceBytes)

					// create challenge plaintext
					challengeDecrypted := fmt.Sprintf("%s,%s,%s", userAddress, authenticationNonce, authTimestamp)
					challengeEncrypted, err := encryptWithPubKey([]byte(challengeDecrypted), device.PublicKey)
					if err != nil {
						sendMessageToClient(c, nil, 4)
						return
					}

					// store the user's language for this session
					userLang, ok = message["lang"].(string)
					if !ok {
						sendMessageToClient(c, nil, 4)
						return
					}

					if err := sendMessageToClient(c, LoginChallenge{
						Message:   Message{Type: 1},
						Challenge: *challengeEncrypted,
					}, 1); err != nil {
						sendMessageToClient(c, nil, 4)
						return
					}

				case 1: // Challenge Response
					if authenticationNonce == "" {
						return // they did the responce before the request, very fishy.
					}

					if authenticationNonce == message["nonce"] && authTimestamp == message["timestamp"] {
						// login sucessful

						// update lang settings
						if device.Language != userLang {
							db.UpdateLanguage(device.DeviceAddress, userLang)
						}

						isAuthenticated = true
						router.AddConnection(userAddress, channel)
						defer router.RemoveConnection(userAddress)
						go func() {
							for msg := range channel {
								if msg.Disconnect {
									log.Printf("Disconnecting from %s\n", c.RemoteAddr().String())
									return
								}
								log.Printf("[%s] Sending Message from channel\n", c.RemoteAddr().String())
								if err := sendNotificationToClient(c, msg.DataToSend); err != nil {
									if err.Error() == "write error" {
										log.Printf("Write error to %s, disconnecting...\n", c.RemoteAddr().String())
										return
									}
									log.Printf("Error sending notification to %s: %v\n", c.RemoteAddr().String(), err)
									sendMessageToClient(c, nil, 4)
									return
								}
							}
						}()
						if err := sendMessageToClient(c, nil, 3); err != nil {
							sendMessageToClient(c, nil, 4)
							return
						}
					} else {
						sendMessageToClient(c, nil, 4)
						return
					}

				default:
					log.Printf("An invalid unauthenticated message type was sent from %s: %v\n", c.RemoteAddr().String(), typeVal)
					sendMessageToClient(c, nil, 4)
					return
				}
			} else {
				// Authenticated requests
				switch typeVal {
				case 2: // Poll Unacked Notifications
					unackedNotifications, err := db.GetUnacknowledgedMessages(userAddress)
					if err != nil {
						fmt.Println(err.Error())
					}
					if len(unackedNotifications) == 0 {
						continue
					}
					for _, unackedNotification := range unackedNotifications {
						log.Printf("[%s] Sending Message from database\n", c.RemoteAddr().String())
						if unackedNotification.IsEncrypted {
							sendNotificationToClient(c, router.DataToSend{
								IsEncrypted: unackedNotification.IsEncrypted,

								Ciphertext: *unackedNotification.Ciphertext,
								DataType:   *unackedNotification.DataType,
								IV:         *unackedNotification.IV,

								DeviceAddress: unackedNotification.DeviceAddress,
								RoutingKey:    unackedNotification.RoutingKey,
								MessageId:     unackedNotification.MessageId,
							})
						} else {
							sendNotificationToClient(c, router.DataToSend{
								IsEncrypted: unackedNotification.IsEncrypted,

								Data: unackedNotification.Data,

								DeviceAddress: unackedNotification.DeviceAddress,
								RoutingKey:    unackedNotification.RoutingKey,
								MessageId:     unackedNotification.MessageId,
							})
						}
					}

				case 3: // Ack Notification
					// get the notification id
					if message["notification"] == nil || message["notification"] == "" {
						sendMessageToClient(c, nil, 4)
						return
					}
					notificationId, ok := message["notification"].(string)
					if !ok {
						sendMessageToClient(c, nil, 4)
						return
					}

					db.AckMessage(notificationId, userAddress)
				case 4: // disconnect
					return
				case 5: // Recieve token
					routingId, ok := message["deviceTokenChecksum"].([]byte)
					if !ok {
						sendMessageToClient(c, nil, 4)
						return
					}
					bundleId, ok := message["appBundleId"].(string)
					if !ok {
						sendMessageToClient(c, nil, 4)
						return
					}

					db.SaveNewToken(userAddress, routingId, bundleId, 0b111)

					hexRouting := hex.EncodeToString(routingId)
					log.Printf("Saved a new token. Token checksum %s", hexRouting)

					if err := sendMessageToClient(c, AckDeviceToken{
						Message:      Message{Type: 5},
						RoutingToken: routingId,
						BundleId:     bundleId,
					}, 5); err != nil {
						sendMessageToClient(c, nil, 4)
						return
					}
				case 6: // token removed
					routingToken, ok := message["deviceTokenChecksum"].([]byte)
					if !ok {
						sendMessageToClient(c, nil, 4)
						return
					}

					typeOfFeedback, ok := message["type"].(int)
					if !ok {
						sendMessageToClient(c, nil, 4)
						return
					}
					reasonForFeedback, ok := message["reason"].(string)
					if !ok {
						sendMessageToClient(c, nil, 4)
						return
					}

					token, err := db.GetToken(routingToken)
					if err != nil {
						if token.FeedbackProviderAddress != nil {
							if token.DeviceAddress != device.DeviceAddress {
								sendMessageToClient(c, nil, 4)
								return
							}
							if token.FeedbackProviderAddress == &configData.ServerAddress {
								feedbackKey, err := db.GetTokenFeedbackKey(routingToken, configData.ServerAddress)
								if err != nil || feedbackKey == nil {
									continue
								}
								db.AddFeedback(routingToken, *feedbackKey, configData.ServerAddress, typeOfFeedback, reasonForFeedback)

							} else {
								type RelayFeedback struct {
									DeviceTokenStr string `json:"device_token"`
									ServerAddress  string `json:"server_address"`
									Type           int    `json:"type"`
									Reason         string `json:"reason"`
								}

								deviceTokenStr := hex.EncodeToString(routingToken)

								setTokenFeedbackProviderJson, err := json.Marshal(RelayFeedback{
									DeviceTokenStr: deviceTokenStr,
									ServerAddress:  configData.ServerAddress,
									Type:           typeOfFeedback,
									Reason:         reasonForFeedback,
								})
								if err != nil {
									sendMessageToClient(c, nil, 4)
									return
								}

								txts, err := net.LookupTXT(fmt.Sprintf("_sgn.%s", *token.FeedbackProviderAddress))
								if err != nil {
									continue
								}
								var serverData router.ServerTXT

								found := false
								for _, txt := range txts {
									serverData, err = router.ParseServerTXT(txt)
									if err == nil {
										found = true
										break
									}
								}
								if found {
									http.Post(fmt.Sprintf("%s/relay_feedback", serverData.HTTPAddress), "application/json", bytes.NewBuffer(setTokenFeedbackProviderJson))
								}
							}
						}
					}

				default:
					log.Printf("An invalid authenticated message type was sent from %s: %v\n", c.RemoteAddr().String(), typeVal)
					sendMessageToClient(c, nil, 4)
					return
				}
			}
		} else {
			log.Printf("Invalid message format from %s: $type is not uint64\n", c.RemoteAddr().String())
			sendMessageToClient(c, nil, 4)
			return
		}
	}
}

func sendMessageToClient(c net.Conn, dataToSend interface{}, messageType int) error {
	var plistEncoded []byte
	var err error

	if dataToSend == nil {
		// Just create a basic message with the specified type
		data := Message{
			Type: messageType,
		}
		plistEncoded, err = plist.Marshal(data, plist.BinaryFormat)
	} else {
		// fine
		val := reflect.ValueOf(dataToSend)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}

		if val.Kind() == reflect.Struct {
			if typeField := val.FieldByName("Type"); typeField.IsValid() && typeField.CanSet() {
				typeField.SetInt(int64(messageType))
			}
		}

		plistEncoded, err = plist.Marshal(dataToSend, plist.BinaryFormat)
	}

	if err != nil {
		return err
	}

	// length stuff
	messageLen := make([]byte, 4)
	length := len(plistEncoded)
	messageLen[0] = byte(length >> 24)
	messageLen[1] = byte(length >> 16)
	messageLen[2] = byte(length >> 8)
	messageLen[3] = byte(length)

	_, err = c.Write(messageLen)
	if err != nil {
		log.Printf("Write error to %s: %v\n", c.RemoteAddr().String(), err)
		return errors.New("write error in len")
	}

	_, err = c.Write(plistEncoded)
	if err != nil {
		log.Printf("Write error to %s: %v\n", c.RemoteAddr().String(), err)
		return errors.New("write error in data")
	}
	return nil
}

func sendNotificationToClient(c net.Conn, data router.DataToSend) error {
	dataToSend := Notification{
		Message:    Message{Type: 2},
		DataToSend: data,
	}

	if err := sendMessageToClient(c, dataToSend, 2); err != nil {
		return err
	}
	return nil
}

// func decryptWithPrivateKey(data []byte, pkey *rsa.PrivateKey) (*[]byte, error) {
// 	// Decrypt the data using PKCS1 OAEP
// 	decrypted, err := rsa.DecryptOAEP(
// 		sha1.New(),
// 		rand.Reader,
// 		pkey,
// 		data,
// 		nil,
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("decryption error: %w", err)
// 	}

// 	return &decrypted, nil
// }

func encryptWithPubKey(data []byte, key *rsa.PublicKey) (*[]byte, error) {
	// Decrypt the data using PKCS1 OAEP
	encrypted, err := rsa.EncryptOAEP(
		sha1.New(),
		rand.Reader,
		key,
		data,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("decryption error: %w", err)
	}

	return &encrypted, nil
}
