package http

import (
	"encoding/json"
	"log"

	"github.com/Preloading/SkyglowNotificationServer/router"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func BaseWebsocket(c *websocket.Conn) {
	// websocket.Conn bindings https://pkg.go.dev/github.com/fasthttp/websocket?tab=doc#pkg-index
	var (
		msg []byte
		err error
	)
	for {
		if _, msg, err = c.ReadMessage(); err != nil {
			log.Println("read:", err)
			break
		}

		var data router.DataToSend
		if err := json.Unmarshal(msg, &data); err != nil {
			log.Println("unmarshal:", err)
			break
		}

		router.SendMessageToLocalRouter(data)

		if err = c.WriteJSON(fiber.Map{
			"status": "Message sent",
			"data":   data,
		}); err != nil {
			log.Println("write:", err)
			break
		}
	}

}
