package http

import (
	"encoding/json"
	"log"

	"github.com/Preloading/SkyglowNotificationServer/router"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func CreateHTTPServer() {
	app := fiber.New()

	app.Post("/send", func(c *fiber.Ctx) error {
		var data router.DataToSend
		if err := c.BodyParser(&data); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid request",
			})
		}
		data.MessageId = uuid.New().String()

		router.SendMessageToRouter(data)

		return c.JSON(fiber.Map{
			"status": "Message sent",
			"data":   data,
		})
	})

	// Websocket route
	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
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
			data.MessageId = uuid.New().String()

			router.SendMessageToRouter(data)

			if err = c.WriteJSON(fiber.Map{
				"status": "Message sent",
				"data":   data,
			}); err != nil {
				log.Println("write:", err)
				break
			}
		}

	}))

	app.Listen(":7878")
}
