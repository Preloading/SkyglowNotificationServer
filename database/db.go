package db

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
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
	db.Delete(&UnacknowledgedMessages{}, "message_id = ? AND device_uuid = ?", message_id, device_uuid)
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

func CreateUser(uuid string, public_key rsa.PublicKey) error {
	encodedPubKey, err := x509.MarshalPKIXPublicKey(&public_key)
	if err != nil {
		return err
	}

	db.Create(&Devices{
		UUID:      uuid,
		PublicKey: string(encodedPubKey),
	})
	return nil
}

func GetUser(uuid string) (*rsa.PublicKey, error) {
	var device Devices
	result := db.First(&device, "uuid = ?", uuid)
	if result.Error != nil {
		return nil, result.Error
	}

	// Decode the public key
	var err error
	parsedKey, err := x509.ParsePKIXPublicKey([]byte(device.PublicKey))
	if err != nil {
		return nil, err
	}

	pubKey, ok := parsedKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA public key")
	}

	return pubKey, nil
}

func GetUnacknowledgedMessages(device_uuid string) []UnacknowledgedMessages {
	var messages []UnacknowledgedMessages
	db.Find(&messages, "device_uuid = ?", device_uuid)
	return messages
}
