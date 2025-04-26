package db

import (
	"log"

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
	CreatedAt  string `gorm:"autoCreateTime"`
}

type Devices struct {
	gorm.Model
	ID   int    `gorm:"primaryKey"`
	UUID string `gorm:"index"`
}

func InitDB(dsn string) {
	// Initialize the database connection
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	db.AutoMigrate(&UnacknowledgedMessages{})
	db.AutoMigrate(&Devices{})
}

func AckMessage(message_id string) {
	db.Delete(&UnacknowledgedMessages{}, "message_id = ?", message_id)
}

func AddMessage(message_id string, device_uuid string, message string) {
	db.Create(&UnacknowledgedMessages{
		MessageId:  message_id,
		DeviceUUID: device_uuid,
		Message:    message,
	})
}
func GetUnacknowledgedMessages(device_uuid string) []UnacknowledgedMessages {
	var messages []UnacknowledgedMessages
	db.Find(&messages)
	return messages
}
