package tcpproto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/Preloading/SkyglowNotificationServer/config"
	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/Preloading/SkyglowNotificationServer/router"
)

var (
	keys       config.CryptoKeys
	configData config.Config
)

func CreateTCPServer(port uint16, _keys config.CryptoKeys, _config config.Config) {
	keys = _keys
	configData = _config
	PORTSTR := ":" + strconv.FormatUint(uint64(port), 10)
	l, err := net.Listen("tcp4", PORTSTR)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

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
	connectionUUID := ""
	channel := make(chan router.DataUpdate)
	var rsaClientPublicKey *rsa.PublicKey
	defer close(channel)

	// Create a buffer for reading chunks
	buffer := make([]byte, 1024)

	for {
		n, err := c.Read(buffer)
		if err != nil {
			log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
			return
		}

		// Try to decrypt the raw data
		decrypted, err := decryptWithPrivateKey(buffer[:n], keys.ServerPrivateKey)
		if err != nil {
			log.Printf("Decryption error from %s: %v\n", c.RemoteAddr().String(), err)
			continue
		}

		decryptedStr := string(*decrypted)
		fmt.Println("Decrypted data:", decryptedStr)

		// Handling
		if strings.HasPrefix(decryptedStr, "ACK:") {
			// Handle ACK
			if connectionUUID == "" {
				fmt.Println("ACK received but no UUID set, ignoring...")
				continue
			}

			db.AckMessage(strings.TrimPrefix(decryptedStr, "ACK:"), connectionUUID)

			fmt.Println("Received ACK:", decryptedStr)
		} else {
			// Is a UUID
			// I could check if it's correct but that lame
			if connectionUUID == "" {
				connectionUUID = decryptedStr
				if configData.WhitelistOn {
					if !config.IsWhitelisted(connectionUUID, configData) {
						return
					}
				} else {
					if config.IsBlacklisted(connectionUUID, configData) {
						return
					}
				}
				fmt.Println("Connection UUID set:", connectionUUID)
				pubKey, err := db.GetUser(connectionUUID)
				if err != nil {
					log.Printf("Error getting public key for UUID %s: %v\n", connectionUUID, err)
					return
				}
				if pubKey == nil {
					log.Printf("No public key found for UUID %s\n", connectionUUID)
					return
				}
				rsaClientPublicKey = pubKey

				// Make a channel to receive messages
				router.AddConnection(connectionUUID, channel)
				defer router.RemoveConnection(connectionUUID)
				go func() {
					for {
						select {
						case msg := <-channel:
							// Check if the channel is closed
							if msg.Disconnect {
								log.Printf("Disconnecting from %s\n", c.RemoteAddr().String())
								return
							}
							log.Printf("[%s] Sending Message from channel\n", c.RemoteAddr().String())
							dataJson, err := json.Marshal(msg.DataToSend)
							if err != nil {
								log.Printf("JSON Marshal error: %v\n", err)
								continue
							}

							// Send the message to the TCP connection
							encrypted, err := encryptWithPubKey([]byte(dataJson), rsaClientPublicKey)
							if err != nil {
								log.Printf("Encryption error: %v\n", err)
								continue
							}

							base64Data := base64.StdEncoding.EncodeToString(*encrypted)

							_, err = c.Write([]byte(base64Data))
							if err != nil {
								log.Printf("Write error to %s: %v\n", c.RemoteAddr().String(), err)
								return
							}
						}
					}
				}()

			}
			if connectionUUID != decryptedStr {
				fmt.Println("UUID mismatch, disconnecting...")
				return
			}

			// Send messages that haven't been ACKed
			messages := db.GetUnacknowledgedMessages(connectionUUID)
			if len(messages) == 0 {
				continue
			}
			for _, message := range messages {
				log.Printf("[%s] Sending Message from database\n", c.RemoteAddr().String())
				dataJson, err := json.Marshal(router.DataToSend{
					Message: message.Message,
					Topic:   message.Topic,
				})

				if err != nil {
					log.Printf("JSON Marshal error: %v\n", err)
					continue
				}

				// Send the message to the TCP connection
				encrypted, err := encryptWithPubKey([]byte(dataJson), rsaClientPublicKey)
				if err != nil {
					log.Printf("Encryption error: %v\n", err)
					continue
				}

				base64Data := base64.StdEncoding.EncodeToString(*encrypted)

				_, err = c.Write([]byte(base64Data))
				if err != nil {
					log.Printf("Write error to %s: %v\n", c.RemoteAddr().String(), err)
					return
				}
			}

		}

	}
}

func decryptWithPrivateKey(data []byte, pkey *rsa.PrivateKey) (*[]byte, error) {
	// Decrypt the data using PKCS1 OAEP
	decrypted, err := rsa.DecryptOAEP(
		sha1.New(),
		rand.Reader,
		pkey,
		data,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("decryption error: %w", err)
	}

	return &decrypted, nil
}

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
