package router

import (
	"fmt"

	db "github.com/Preloading/SkyglowNotificationServer/database"
)

type DataToSend struct {
	DeviceUUID string `json:"device_uuid,omitempty"`
	Message    string `json:"message"`
	Topic      string `json:"topic"`
	MessageId  string `json:"message_id,omitempty"`
}

type DataUpdate struct {
	DataToSend DataToSend
	Disconnect bool
}

// This function routes notifications from the HTTP server to both the TCP server and the database.

var (
	connections map[string]chan DataUpdate
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

func SendMessageToRouter(msg DataToSend) {
	if ch, ok := connections[msg.DeviceUUID]; ok {
		select {
		case ch <- DataUpdate{DataToSend: msg, Disconnect: false}:
			// Message sent to connection
		default:
			// Channel is full or blocked, optionally handle this case
			fmt.Println("Channel is full or blocked, message not sent to connection")
		}
	}

	db.AddMessage(msg.MessageId, msg.Message, msg.DeviceUUID, msg.Topic)
}
