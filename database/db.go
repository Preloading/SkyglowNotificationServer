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
	ID            uint64 `gorm:"primaryKey"`
	MessageId     string `gorm:"index"`
	DeviceAddress string
	Message       string
	Topic         string
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

type NotificationTokens struct {
	gorm.Model
	Token            string `gorm:"primaryKey"`
	DeviceAddress    string
	NotificationType int       // Notifcation types, I'm not sure of the order yet but None, Badge, Sound, Alert
	AppBundleId      string    // example: com.atebits.tweetie2
	IssuedAt         time.Time `gorm:"autoCreateTime"`
	IsValid          bool      // Unsure if this should be kept
}

type Devices struct {
	gorm.Model
	DeviceAddress string `gorm:"primaryKey"`
	PublicKey     string
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
	db.Delete(&UnacknowledgedMessages{}, "message_id = ? AND device_address = ?", message_id, device_uuid)
}

func AddMessage(message_id string, message string, device_uuid string, topic string) {
	db.Create(&UnacknowledgedMessages{
		MessageId:     message_id,
		DeviceAddress: device_uuid,
		Message:       message,
		Topic:         topic,
		CreatedAt:     time.Now(),
	})
}

func SaveNewUser(device_address string, public_key rsa.PublicKey) error {
	encodedPubKey, err := x509.MarshalPKIXPublicKey(&public_key)
	if err != nil {
		return err
	}

	db.Create(&Devices{
		DeviceAddress: device_address,
		PublicKey:     string(encodedPubKey),
	})
	return nil
}

func GetUser(device_address string) (*rsa.PublicKey, error) {
	var device Devices
	result := db.First(&device, "device_address = ?", device_address)
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

func GetUnacknowledgedMessages(device_address string) []UnacknowledgedMessages {
	var messages []UnacknowledgedMessages
	db.Find(&messages, "device_address = ?", device_address)
	return messages
}
