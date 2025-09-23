package main

import (
	"errors"
	"fmt"

	"github.com/Preloading/SkyglowNotificationServer/config"
	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/Preloading/SkyglowNotificationServer/feedbackmgr"
	"github.com/Preloading/SkyglowNotificationServer/http"
	"github.com/Preloading/SkyglowNotificationServer/router"
	"github.com/Preloading/SkyglowNotificationServer/tcpproto"
)

func main() {
	c, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded config successfully")
	if len(c.ServerAddress) > 16 {
		panic(errors.New("server address is greater than 16 in length! Please change to be 16 or under charactors"))
	}

	keys, err := config.LoadCryptoKeys(c.KEY_PATH)
	if err != nil {
		panic(err)
	}
	fmt.Println("Loaded keys successfully")

	// Initialize the database connection
	db.InitDB(c.DB_DSN)
	router.Config = c
	fmt.Println("Starting TCP Server...")
	go tcpproto.CreateTCPServer(uint16(c.TCPPort), *keys, c)
	fmt.Println("Starting HTTP Server...")
	go http.CreateHTTPServer(*keys, c)
	feedbackmgr.StartFeedbackCycle(c)
	select {}
}
