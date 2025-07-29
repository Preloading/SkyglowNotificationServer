package router

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	configPkg "github.com/Preloading/SkyglowNotificationServer/config"
	db "github.com/Preloading/SkyglowNotificationServer/database"
	"github.com/google/uuid"
)

type DataToSend struct {
	RoutingKeyStr string `json:"routing_key" plist:"-"`
	RoutingKey    []byte `json:"-" plist:"routing_key"`
	DeviceAddress string `json:"-" plist:"-"`
	ServerAddress string `json:"server_address" plist:"-"`
	AlertBody     string `json:"message" plist:"message"`
	Topic         string `json:"topic" plist:"topic"`
	AlertAction   string `json:"alert_action,omitempty" plist:"alert_action"` // Default to Open
	AlertSound    string `json:"alert_sound,omitempty" plist:"alert_sound"`   // Default to UILocalNotificationDefaultSoundName
	BadgeNumber   int    `json:"badge_number,omitempty" plist:"badge_number,omitempty"`
	// UserInfo      *interface{} `json:"user_info,omitempty" plist:"user_info"`       // https://developer.apple.com/documentation/uikit/uilocalnotification/userinfo?language=objc

	// Data for the server
	StrictBundleId bool     `json:"strict_bundle_id,omitempty" plist:"-"`
	MessageId      string   `json:"message_id,omitempty" plist:"message_id"` // Don't let other users set this!
	TotalHops      int      `json:"total_hops,omitempty" plist:"-"`
	Hops           []string `json:"hops,omitempty" plist:"-"`
}

type DataUpdate struct {
	DataToSend DataToSend
	Disconnect bool
}

type ServerTXT struct {
	TCPAddress  string
	TCPPort     int
	HTTPAddress string
}

var (
	connections map[string]chan DataUpdate
	Config      configPkg.Config
)

func AddConnection(deviceUUID string, messageChan chan DataUpdate) {
	if connections == nil {
		connections = make(map[string]chan DataUpdate)
	}
	if _, ok := connections[deviceUUID]; ok {
		RemoveConnection(deviceUUID)
		DisconnectConnection(deviceUUID)
	}

	connections[deviceUUID] = messageChan
}
func DisconnectConnection(deviceUUID string) {
	if ch, ok := connections[deviceUUID]; ok {
		select {
		case ch <- DataUpdate{Disconnect: true}:
			// Message sent to connection
		default:
			// Channel is full or blocked, optionally handle this case
			fmt.Println("Channel is full or blocked, message not sent to connection")
		}
	}
}

func RemoveConnection(deviceUUID string) {
	if connections == nil {
		return
	}
	if _, ok := connections[deviceUUID]; !ok {
		return
	}
	connections[deviceUUID] = nil
	delete(connections, deviceUUID)

}

func SendMessageToRouter(msg DataToSend) error {
	if msg.ServerAddress == "" {
		return errors.New("server address is empty, cannot send message")
	}

	if msg.ServerAddress == Config.ServerAddress {
		// This is one of us, lets send it off to the local router
		err := SendMessageToLocalRouter(msg)
		return err
	} else {
		// This message is to be sent to someone else's server, lets go find them
		_, err := RouteMessageToProperServer(msg, msg.ServerAddress)
		return err // TODO
	}
}

func SendMessageToLocalRouter(msg DataToSend) error {
	msg.MessageId = uuid.New().String()

	if msg.AlertSound == "default" {
		msg.AlertSound = "UILocalNotificationDefaultSoundName" // I checked, and it does UILocalNotificationDefaultSoundName is set to UILocalNotificationDefaultSoundName
	}

	// decode routing key hex
	bs, err := hex.DecodeString(msg.RoutingKeyStr)
	if err != nil {
		return errors.New("routing key not in hex")
	}

	msg.RoutingKey = bs

	// query device address
	deviceInfo, err := db.GetToken(bs)
	if err != nil {
		return errors.New("routing key invalid")
	}

	if msg.StrictBundleId {
		if msg.Topic != deviceInfo.AppBundleId {
			return errors.New("bundle id isn't correct for this routing key")
		}
	} else {
		if msg.Topic != deviceInfo.AppBundleId {
			msg.Topic = deviceInfo.AppBundleId
		}
	}

	msg.DeviceAddress = deviceInfo.DeviceAddress

	db.AddMessage(msg.MessageId, msg.AlertBody, msg.DeviceAddress, msg.Topic, bs)
	if ch, ok := connections[msg.DeviceAddress]; ok {
		select {
		case ch <- DataUpdate{DataToSend: msg, Disconnect: false}:
			// Message sent to connection
		default:
			// Channel is full or blocked, optionally handle this case
			fmt.Println("Channel is full or blocked, message not sent to connection")
		}
	}
	return nil
}

func RouteMessageToProperServer(msg DataToSend, server string) (*http.Response, error) {
	txts, err := net.LookupTXT(fmt.Sprintf("_sgn.%s", server))
	if err != nil {
		return nil, errors.New("failed to lookup txt record")
	}
	var serverData ServerTXT

	found := false
	for _, txt := range txts {
		serverData, err = ParseServerTXT(txt)
		if err == nil {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("server could not be found")
	}

	relayMsg := msg
	relayMsg.TotalHops = relayMsg.TotalHops + 1
	if relayMsg.TotalHops > 10 {
		return nil, errors.New("hop limit exceeded")
	}

	relayMsg.Hops = append(relayMsg.Hops, Config.ServerAddress)

	relayMsgJson, err := json.Marshal(relayMsg)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(fmt.Sprintf("%s/send", serverData.HTTPAddress), "application/json", bytes.NewBuffer(relayMsgJson))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func ParseServerTXT(input string) (ServerTXT, error) {
	var result ServerTXT

	// Split the input by spaces to get key-value pairs
	parts := strings.Fields(input)

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return result, fmt.Errorf("invalid format in part: %s", part)
		}

		key := kv[0]
		value := kv[1]

		switch key {
		case "tcp_addr":
			// TODO: Validate that this is not localhost or reserved IPs
			result.TCPAddress = value
		case "tcp_port":
			port, err := strconv.Atoi(value)
			if err != nil {
				return result, fmt.Errorf("invalid TCP port: %v", err)
			}
			result.TCPPort = port
		case "http_addr":
			// TODO: Validate this is starts with either https or http, and that it is not localhost or reserved IPs
			result.HTTPAddress = value
		}
	}

	return result, nil
}
