package http

import (
	"github.com/Preloading/SkyglowNotificationServer/config"
	configPkg "github.com/Preloading/SkyglowNotificationServer/config"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

type StatusOnly struct {
	Status string `json:"status"`
}

var (
	Config configPkg.Config
	keys   config.CryptoKeys
)

func CreateHTTPServer(_keys configPkg.CryptoKeys, _config configPkg.Config) {
	keys = _keys
	Config = _config
	app := fiber.New()
	app.Use(logger.New())

	app.Post("/send", NotificationSend)

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

	// Device specific
	app.Post("/snd/register_device", CreateUser)
	app.Get("/get_feedback", RequestFeedback)
	app.Post("/register_token_for_feedback", RegisterForFeedback)

	app.Listen(":7878")
}
