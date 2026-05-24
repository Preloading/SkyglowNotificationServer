package http

import (
	"github.com/Preloading/SkyglowNotificationServer/config"
	configPkg "github.com/Preloading/SkyglowNotificationServer/config"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"howett.net/plist"
)

type StatusOnly struct {
	Status string `json:"status" plist:"status"`
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

	// feedback
	app.Get("/get_feedback", GetFeedback)                                     // service calls this
	app.Post("/register_token_for_feedback", RegisterForFeedback)             // service calls this
	app.Post("/set_feedback_provider_for_token", SetFeedbackProviderForToken) // server calls this to another server, sends domain.
	app.Post("/relay_feedback", RelayedFeedback)                              // sends feedback from a server to another server.

	app.Listen(":7878")
}

func SendAsRequestType(c *fiber.Ctx, v interface{}, isPlist bool, format int) error {
	if isPlist {
		return SendAsPlist(c, v, format)
	} else {
		return c.JSON(v)
	}
}

func SendAsPlist(c *fiber.Ctx, v interface{}, format int) error {
	plistData, err := plist.Marshal(v, format)

	if err != nil {
		return c.Status(fiber.ErrInternalServerError.Code).SendString("failed to encode the responce! report this as a bug")
	}
	return c.Send(plistData)
}
