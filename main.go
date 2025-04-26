package main

import (
	"fmt"

	"github.com/Preloading/SkyglowNotificationServer/config"
	"github.com/Preloading/SkyglowNotificationServer/tcpproto"
)

func main() {
	keys, err := config.LoadCryptoKeys()
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded keys successfully")

	config, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded config successfully")

	fmt.Println("Starting Server...")
	tcpproto.CreateTCPServer(uint16(config.TCPPort), keys)
}
