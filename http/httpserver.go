package http

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"

	db "github.com/Preloading/SkyglowNotificationServer/database"
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

	app.Post("/request_keys", func(c *fiber.Ctx) error {
		type UUIDRequest struct {
			UUID string `json:"uuid"`
		}
		var req UUIDRequest

		if err := c.BodyParser(&req); err != nil || req.UUID == "" {
			req.UUID = uuid.New().String()
		}

		// Generate a new RSA key pair
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error",
				"error":  "Failed to generate keys",
			})
		}
		publicKey := &privateKey.PublicKey

		db.CreateUser(req.UUID, *publicKey)

		// Marshal the public key to PEM format with correct header
		publicKeyBytes := x509.MarshalPKCS1PublicKey(publicKey)
		publicKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyBytes,
		})
		// Marshal the private key to PEM format with correct header
		privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privateKeyBytes,
		})
		// Marshal the server public key
		serverPublicKeyBytes := x509.MarshalPKCS1PublicKey(privateKey.Public().(*rsa.PublicKey))
		serverPublicKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: serverPublicKeyBytes,
		})

		return c.JSON(fiber.Map{
			"status":             "success",
			"uuid":               req.UUID,
			"client_public_key":  string(publicKeyPEM),
			"client_private_key": string(privateKeyPEM),
			"server_public_key":  string(serverPublicKeyPEM),
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
