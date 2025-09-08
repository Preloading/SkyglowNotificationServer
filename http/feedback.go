package http

import (
	"encoding/hex"
	"time"

	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/gofiber/fiber/v2"
)

type RequestFeedbackReqBody struct {
	After       time.Time `json:"after"`
	FeedbackKey []byte    `json:"feedback_key"`
}

type RegisterForFeedbackReqBody struct {
	FeedbackKeyStr string `json:"feedback_key"`
	FeedbackKey    []byte `json:"-"`
	DeviceTokenStr string `json:"device_token"`
	DeviceToken    []byte `json:"-"`
}

func RegisterForFeedback(c *fiber.Ctx) error {
	var data RegisterForFeedbackReqBody
	var err error
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	// decode the hex
	data.DeviceToken, err = hex.DecodeString(data.DeviceTokenStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	data.FeedbackKey, err = hex.DecodeString(data.FeedbackKeyStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	// store data into DB
	err = db.SaveNewFeedbackForToken(data.DeviceToken, data.FeedbackKey)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "failed to store item in db, (already exists?)",
		})
	}

	return c.JSON(fiber.Map{
		"status": "sucess",
	})
}

// func RequestFeedback(c *fiber.Ctx) error {
// 	var data RequestFeedbackReqBody
// 	if err := c.BodyParser(&data); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"status": err.Error(),
// 		})
// 	}

// 	// 1. Get feedback data with the specific feedback key after a date

// 	// 2.

// 	// err := router.SendMessageToRouter(data)

// 	// if err != nil {
// 	// 	return c.SendString(err.Error())
// 	// }

// 	// return c.JSON(fiber.Map{
// 	// 	"status": "sucess",
// 	// 	"data":   data,
// 	// })
// }
