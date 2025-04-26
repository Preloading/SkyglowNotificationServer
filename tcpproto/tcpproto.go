package tcpproto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/Preloading/SkyglowNotificationServer/config"
)

var (
	keys config.CryptoKeys
)

func CreateTCPServer(port uint16, _keys config.CryptoKeys) {
	keys = _keys
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
			fmt.Println("Received ACK:", decryptedStr)
		} else {
			// Is a UUID
			// I could check if it's correct but that lame

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
