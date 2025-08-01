package http

import (
	"github.com/Preloading/SkyglowNotificationServer/router"
	"github.com/gofiber/fiber/v2"
)

func NotificationSend(c *fiber.Ctx) error {
	var data router.DataToSend
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": err.Error(),
		})
	}

	err := router.SendMessageToRouter(data)

	if err != nil {
		return c.SendString(err.Error())
	}

	return c.JSON(fiber.Map{
		"status": "sucess",
		"data":   data,
	})
}
