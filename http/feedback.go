package http

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/Preloading/SkyglowNotificationServer/router"
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
	ServerAddress  string `json:"server_address"`
}

type SetFeedbackProviderForTokenReqBody struct {
	ProviderDomain string `json:"provider_domain"`
	DeviceTokenStr string `json:"device_token"`
	DeviceToken    []byte `json:"-"`
}

type RelayFeedback struct {
	DeviceTokenStr string `json:"device_token"`
	DeviceToken    []byte `json:"-"`
	ServerAddress  string `json:"server_address"`
	Type           int    `json:"type"`
	Reason         string `json:"reason"`
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
	err = db.SaveNewFeedbackToken(data.DeviceToken, data.ServerAddress, data.FeedbackKey)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "failed to store item in db, (already exists?)",
		})
	}

	if Config.ServerAddress == data.ServerAddress {
		if err := db.SetTokenFeedbackProviderAddress(data.DeviceToken, Config.ServerAddress); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": err.Error(),
			})
		}
	} else {
		setTokenFeedbackProviderJson, err := json.Marshal(SetFeedbackProviderForTokenReqBody{
			ProviderDomain: Config.ServerAddress,
			DeviceTokenStr: data.DeviceTokenStr,
		})
		if err != nil {
			return err
		}

		txts, err := net.LookupTXT(fmt.Sprintf("_sgn.%s", data.ServerAddress))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "could not find token's notification server",
			})
		}
		var serverData router.ServerTXT

		found := false
		for _, txt := range txts {
			serverData, err = router.ParseServerTXT(txt)
			if err == nil {
				found = true
				break
			}
		}
		if !found {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "could not find token's notification server",
			})
		}

		resp, err := http.Post(fmt.Sprintf("%s/set_feedback_provider_for_token", serverData.HTTPAddress), "application/json", bytes.NewBuffer(setTokenFeedbackProviderJson))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "failed to relay feedback registration to token owner ",
			})
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || string(body) != "{\"status\":\"success\"}" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "unexpected response from token owner",
			})
		}
	}

	return c.JSON(fiber.Map{
		"status": "sucess",
	})
}

func SetFeedbackProviderForToken(c *fiber.Ctx) error {
	// This handles getting the domain to send the feedback to.
	var data SetFeedbackProviderForTokenReqBody
	var err error
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	// decode the hex device token
	data.DeviceToken, err = hex.DecodeString(data.DeviceTokenStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	if err := db.SetTokenFeedbackProviderAddress(data.DeviceToken, data.ProviderDomain); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status": "sucess",
	})

}

func GetFeedback(c *fiber.Ctx) error {
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

func RelayedFeedback(c *fiber.Ctx) error {
	// This handles getting the domain to send the feedback to.
	var data RelayFeedback
	var err error
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	// decode the hex device token
	data.DeviceToken, err = hex.DecodeString(data.DeviceTokenStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	feedbackKey, err := db.GetTokenFeedbackKey(data.DeviceToken, data.ServerAddress)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "could not find token (is registered for feedback?)",
		})
	}

	if feedbackKey == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "could not find token (is registered for feedback?)",
		})
	}

	if err := db.AddFeedback(data.DeviceToken, *feedbackKey, data.ServerAddress, data.Type, data.Reason); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status": "sucess",
	})

}
