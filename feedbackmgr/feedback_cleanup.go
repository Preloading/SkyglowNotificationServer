package feedbackmgr

import db "github.com/Preloading/SkyglowNotificationServer/database"

func StartFeedbackCycle() {

}

func ProcessFeedback() {
	feedbacks, err := db.GetAllFeedback()
	if err != nil {
		return
	}

	for _, feedback := range feedbacks {
		switch feedback.Type {
		case 0:
			// deleted token
			// db.RemoveDeviceToken(feedback.RoutingToken, feedback)
		}
	}
}
