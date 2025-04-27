package db

import (
	"log"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	db *gorm.DB
)

type UnacknowledgedMessages struct {
	gorm.Model
	ID         uint64 `gorm:"primaryKey"`
	MessageId  string `gorm:"index"`
	DeviceUUID string
	Message    string
	Topic      string
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

type Devices struct {
	gorm.Model
	UUID      string `gorm:"primaryKey"`
	PublicKey string
}

func InitDB(dsn string) {
	// Initialize the database connection
	var err error
	db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	db.AutoMigrate(&UnacknowledgedMessages{})
	db.AutoMigrate(&Devices{})
}

func AckMessage(message_id string, device_uuid string) {
	db.Delete(&UnacknowledgedMessages{}, "message_id = ? device_uuid = ?", message_id, device_uuid)
}

func AddMessage(message_id string, message string, device_uuid string, topic string) {
	db.Create(&UnacknowledgedMessages{
		MessageId:  message_id,
		DeviceUUID: device_uuid,
		Message:    message,
		Topic:      topic,
		CreatedAt:  time.Now(),
	})
}
func GetUnacknowledgedMessages(device_uuid string) []UnacknowledgedMessages {
	var messages []UnacknowledgedMessages
	db.Find(&messages)
	return messages
}
