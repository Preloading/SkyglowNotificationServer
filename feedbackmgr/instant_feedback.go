package feedbackmgr

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/Preloading/SkyglowNotificationServer/router"
)

// only for this server
func PreformInstantFeedbackActionsForOurToken(typeOfFeedback int, reason string, routing_token []byte, server_address string) {
	switch typeOfFeedback {
	case 0:
		db.MarkTokenForRemoval(routing_token)
	}
}

func SaveFeedbackWhenProviderIsUs(typeOfFeedback int, reason string, routing_token []byte, server_address string) {
	feedbackKey, err := db.GetTokenFeedbackKey(routing_token, server_address)
	if err != nil || feedbackKey == nil {
		return
	}
	db.AddFeedback(routing_token, *feedbackKey, server_address, typeOfFeedback, reason)
}

func RemoveToken(typeOfFeedback int, reasonForFeedback string, routingToken []byte, our_address string, feedbackAddress *string) error {
	PreformInstantFeedbackActionsForOurToken(typeOfFeedback, reasonForFeedback, routingToken, our_address)

	if feedbackAddress != nil {
		if *feedbackAddress == our_address {
			SaveFeedbackWhenProviderIsUs(typeOfFeedback, reasonForFeedback, routingToken, our_address)
		} else {
			type RelayFeedback struct {
				DeviceTokenStr string `json:"device_token"`
				ServerAddress  string `json:"server_address"`
				Type           int    `json:"type"`
				Reason         string `json:"reason"`
			}

			deviceTokenStr := hex.EncodeToString(routingToken)

			setTokenFeedbackProviderJson, err := json.Marshal(RelayFeedback{
				DeviceTokenStr: deviceTokenStr,
				ServerAddress:  our_address,
				Type:           typeOfFeedback,
				Reason:         reasonForFeedback,
			})
			if err != nil {
				return err
			}

			txts, err := net.LookupTXT(fmt.Sprintf("_sgn.%s", *feedbackAddress))
			if err != nil {
				return err
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
			if found {
				http.Post(fmt.Sprintf("%s/relay_feedback", serverData.HTTPAddress), "application/json", bytes.NewBuffer(setTokenFeedbackProviderJson))
			}
		}
	}
	return nil
}
