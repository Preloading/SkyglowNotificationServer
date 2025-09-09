package http

import (
	"encoding/hex"
	"time"

	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/gofiber/fiber/v2"
)

type RequestFeedbackResBodyContents struct {
	RoutingToken []byte    `json:"routing_token"`
	Type         int       `json:"type"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
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

func RequestFeedback(c *fiber.Ctx) error {
	if c.Query("feedback_key") == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "missing feedback key",
		})
	}

	feedbackKey, err := hex.DecodeString(c.Query("feedback_key"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "missing feedback key",
		})
	}

	var after *time.Time
	if c.Query("after") != "" {
		afterPtr, err := time.Parse(time.RFC3339, c.Query("after"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "after time invalid",
			})
		}
		after = &afterPtr
	}

	// 1. Get feedback data with the specific feedback key after a date
	feedbackToSendRaw, err := db.GetFeedbackWithSecret(feedbackKey, after)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	feedbackToSend := make([]RequestFeedbackResBodyContents, len(feedbackToSendRaw))
	for i := range feedbackToSendRaw {
		feedbackToSend[i] = RequestFeedbackResBodyContents{
			RoutingToken: feedbackToSendRaw[i].RoutingToken,
			Type:         feedbackToSendRaw[i].Type,
			Reason:       feedbackToSendRaw[i].Reason,
			CreatedAt:    feedbackToSendRaw[i].CreatedAt,
		}
	}

	// 2. send that data off!
	return c.JSON(fiber.Map{
		"status": "sucess",
		"data":   feedbackToSend,
	})
}
