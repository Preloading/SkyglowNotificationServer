package tcpproto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/Preloading/SkyglowNotificationServer/config"
	"github.com/Preloading/SkyglowNotificationServer/router"
)

var (
	keys       config.CryptoKeys
	configData config.Config
)

type Notification struct {
	// Message
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

	// handleV2Connection(c, channel)
	// return

	// send hello in old format for compatibility
	if err := sendMessageToClientV1(c, nil, 0); err != nil {
		return
	}
	startByte := make([]byte, 1)
	_, err := c.Read(startByte)
	if err != nil {
		log.Printf("Read error from %s: %v\n", c.RemoteAddr().String(), err)
		// too soon to find client version
		return
	}
	if startByte[0] == 0x53 {
		// probably the new client

		// ignore whatever was sent within this packet for the client to like me <3
		header := make([]byte, 7)
		if _, err := io.ReadFull(c, header); err != nil {
			return
		}
		messageSize := uint32(header[3])<<24 | uint32(header[4])<<16 | uint32(header[5])<<8 | uint32(header[6])
		if messageSize > 4096 { // we probably guessed wrong, so reject it
			disconnectClientV2(c, 0x02, 0)
			return
		}
		if messageSize > 0 {
			if _, err := io.CopyN(io.Discard, c, int64(messageSize)); err != nil {
				return
			}
		}

		// finally send it off to the actual handler
		handleV2Connection(c, channel)
		return
	} else {
		// probably the old client
		handleV1Connection(c, channel, startByte[0])
		return
	}

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
