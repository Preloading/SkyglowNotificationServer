package http

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"

	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type DeviceRegisterRequest struct {
	PubKey string `json:"pub_key"`
}

type DeviceRegisterResponce struct {
	Status        string `json:"status"`
	DeviceAddress string `json:"device_address"`
	ServerPubKey  string `json:"server_pub_key"`
}

func CreateUser(c *fiber.Ctx) error {
	fmt.Println(string(c.Body()))
	var req DeviceRegisterRequest

	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(401).JSON(StatusOnly{Status: "malformed request"})
	}

	// parse pub key
	block, _ := pem.Decode([]byte(req.PubKey))
	if block == nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(StatusOnly{Status: "invalid public key format"})
	}

	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return c.Status(fiber.ErrBadRequest.Code).JSON(StatusOnly{Status: "invalid public key"})
	}

	clientPubKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return c.Status(fiber.ErrBadRequest.Code).JSON(StatusOnly{Status: "not an RSA public key"})
	}

	// Generate the device address
	uuid := uuid.New().String()
	uuidWithoutHyphens := strings.Replace(uuid, "-", "", -1)

	client_address := fmt.Sprintf("%s@%s", uuidWithoutHyphens, Config.ServerAddress)

	db.SaveNewUser(client_address, *clientPubKey)

	// Marshal the server public key
	serverPublicKeyBytes := x509.MarshalPKCS1PublicKey(keys.ServerPublicKey)
	serverPublicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: serverPublicKeyBytes,
	})

	return c.JSON(DeviceRegisterResponce{
		Status:        "sucess",
		DeviceAddress: client_address,
		ServerPubKey:  string(serverPublicKeyPEM),
	})
}
