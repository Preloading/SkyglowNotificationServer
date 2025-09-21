package feedbackmgr

import (
	db "github.com/Preloading/SkyglowNotificationServer/database"
)

// only for this server
func PreformInstantFeedbackActionsForOurToken(typeOfFeedback int, reason string, routing_token []byte, server_address string) {

	// now do a switch case you cunt
	switch typeOfFeedback {
	case 0:
		db.KillThatToken(routing_token)
	}
}

func SaveFeedbackWhenProviderIsUs(typeOfFeedback int, reason string, routing_token []byte, server_address string) {
	feedbackKey, err := db.GetTokenFeedbackKey(routing_token, server_address)
	if err != nil || feedbackKey == nil {
		return
	}
	db.AddFeedback(routing_token, *feedbackKey, server_address, typeOfFeedback, reason)
}
