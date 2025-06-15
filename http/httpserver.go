package http

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func CreateHTTPServer() {
	app := fiber.New()

	app.Post("/send", NotificationSend)

	app.Post("/request_keys", CreateUser)

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

	app.Get("/ws", websocket.New(BaseWebsocket))

	app.Listen(":7878")
}
