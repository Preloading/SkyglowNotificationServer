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
	"howett.net/plist"
)

type DeviceRegisterRequest struct {
	PubKey  string `json:"pub_key" plist:"pub_key"`
	Version int    `json:"version" plist:"version"`
}

type DeviceRegisterResponce struct {
	Status        string `json:"status" plist:"status"`
	DeviceAddress string `json:"device_address" plist:"device_address"`
	ServerPubKey  string `json:"server_pub_key" plist:"server_pub_key"`
}

func CreateUser(c *fiber.Ctx) error {
	var req DeviceRegisterRequest
	var err error

	isPlist := false
	format := 0
	if c.Get("Content-Type") == "application/x-plist" || c.Get("Content-Type") == "application/xml" {
		isPlist = true
		if format, err = plist.Unmarshal(c.Body(), &req); err != nil {
			return SendAsRequestType(c.Status(401), StatusOnly{Status: "malformed request"}, true, 0)
		}
	} else {
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return SendAsRequestType(c.Status(401), StatusOnly{Status: "malformed request"}, false, 0)
		}
	}

	// if !(req.Version >= 2) {
	// 	return SendAsRequestType(c.Status(fiber.ErrBadRequest.Code), StatusOnly{Status: "outdated client!"}, isPlist, format)
	// }

	// parse pub key
	block, _ := pem.Decode([]byte(req.PubKey))
	if block == nil {
		return SendAsRequestType(c.Status(fiber.ErrBadRequest.Code), StatusOnly{Status: "invalid public key format"}, isPlist, format)
	}

	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return SendAsRequestType(c.Status(fiber.ErrBadRequest.Code), StatusOnly{Status: "invalid public key"}, isPlist, format)
	}

	clientPubKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		if isPlist {

		} else {
			return SendAsRequestType(c.Status(fiber.ErrBadRequest.Code), StatusOnly{Status: "not an RSA public key"}, isPlist, format)
		}
	}

	// Generate the device address
	uuid := uuid.New().String()
	uuidWithoutHyphens := strings.Replace(uuid, "-", "", -1)

	client_address := fmt.Sprintf("%s@%s", uuidWithoutHyphens, Config.ServerAddress)

	db.SaveNewUser(client_address, *clientPubKey)
	return SendAsRequestType(c, DeviceRegisterResponce{
		Status:        "sucess",
		DeviceAddress: client_address,
		ServerPubKey:  *keys.ServerPublicKeyString,
	}, isPlist, format)
}
