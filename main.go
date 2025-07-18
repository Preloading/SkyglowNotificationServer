package main

import (
	"fmt"

	"github.com/Preloading/SkyglowNotificationServer/config"
	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/Preloading/SkyglowNotificationServer/http"
	"github.com/Preloading/SkyglowNotificationServer/router"
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

	// Initialize the database connection
	db.InitDB("sqlite.db")
	router.Config = config
	fmt.Println("Starting TCP Server...")
	go tcpproto.CreateTCPServer(uint16(config.TCPPort), *keys, config)
	fmt.Println("Starting HTTP Server...")
	go http.CreateHTTPServer(*keys, config)
	select {}
}
