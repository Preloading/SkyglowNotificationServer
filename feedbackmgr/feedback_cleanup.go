package feedbackmgr

import (
	"fmt"
	"log"
	"time"

	configPkg "github.com/Preloading/SkyglowNotificationServer/config"
	db "github.com/Preloading/SkyglowNotificationServer/database"
)

var (
	Config configPkg.Config
	ticker *time.Ticker
)

func StartFeedbackCycle(_config configPkg.Config) {
	Config = _config

	ticker = time.NewTicker(2 * time.Hour)

	go func() {
		for range ticker.C {
			ProcessFeedback()
		}
	}()
}

// this includes our token, and other people.
func ProcessFeedback() {
	log.Println("running feedback cycle")
	err := db.HideTheTracksOfKilledTokens(Config.ServerAddress)
	if err != nil {
		fmt.Println(err.Error())
	}

	feedbacks, err := db.GetAllFeedback()
	if err != nil {
		return
	}

	for _, feedback := range feedbacks {
		switch feedback.Type {
		case 0:
			// deleted token
			db.RemoveDeviceToken(feedback.RoutingToken, feedback.ServerAddress, feedback.ServerAddress == Config.ServerAddress)
		}
	}
}
