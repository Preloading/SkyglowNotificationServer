package http

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func CreateUser(c *fiber.Ctx) error {
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

	// Marshal the server public key
	serverPublicKeyBytes := x509.MarshalPKCS1PublicKey(privateKey.Public().(*rsa.PublicKey))
	serverPublicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: serverPublicKeyBytes,
	})

	return c.JSON(fiber.Map{
		"status":            "success",
		"address":           req.UUID,
		"server_public_key": string(serverPublicKeyPEM),
	})
}
