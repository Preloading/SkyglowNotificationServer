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
	RouterAddress []byte
	Message       string
	Topic         string
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

type NotificationToken struct {
	gorm.Model
	RoutingToken     []byte `gorm:"primaryKey"`
	DeviceAddress    string
	NotificationType int       // Notifcation types, I'm not sure of the order yet but None, Badge, Sound, Alert
	AppBundleId      string    // example: com.atebits.tweetie2
	IssuedAt         time.Time `gorm:"autoCreateTime"`
	IsValid          bool      // Unsure if this should be kept
}

type DevicesDB struct {
	gorm.Model
	DeviceAddress string `gorm:"primaryKey"`
	PublicKey     string
	Language      string
}

type Device struct {
	DeviceAddress string `gorm:"primaryKey"`
	PublicKey     *rsa.PublicKey
	Language      string
}

func InitDB(dsn string) {
	// Initialize the database connection
	var err error
	db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	db.AutoMigrate(&NotificationToken{})
	db.AutoMigrate(&UnacknowledgedMessages{})
	db.AutoMigrate(&DevicesDB{})
}

func AckMessage(message_id string, device_uuid string) {
	db.Delete(&UnacknowledgedMessages{}, "message_id = ? AND device_address = ?", message_id, device_uuid)
}

func AddMessage(message_id string, message string, device_uuid string, topic string, routingKey []byte) {
	db.Create(&UnacknowledgedMessages{
		MessageId:     message_id,
		DeviceAddress: device_uuid,
		RouterAddress: routingKey,
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

	db.Create(&DevicesDB{
		DeviceAddress: device_address,
		PublicKey:     string(encodedPubKey),
	})
	return nil
}

func UpdateLanguage(device_address string, language string) error {
	var device DevicesDB
	result := db.First(&device, "device_address = ?", device_address)
	if result.Error != nil {
		return result.Error
	}
	device.Language = language
	result = db.Save(device)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func SaveNewToken(device_address string, routingToken []byte, bundleId string, notificationType int) error {
	db.Create(&NotificationToken{
		DeviceAddress:    device_address,
		AppBundleId:      bundleId,
		RoutingToken:     routingToken,
		NotificationType: notificationType,
		IsValid:          true,
	})
	return nil
}

func GetUser(device_address string) (*Device, error) {
	var device DevicesDB
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

	return &Device{
		DeviceAddress: device.DeviceAddress,
		PublicKey:     pubKey,
		Language:      device.Language,
	}, nil
}

func GetToken(routing_token []byte) (*NotificationToken, error) {
	var notificationToken NotificationToken
	result := db.First(&notificationToken, "routing_key = ?", routing_token)
	if result.Error != nil {
		return nil, result.Error
	}

	// Decode the public key
	return &notificationToken, nil
}

func GetUnacknowledgedMessages(device_address string) []UnacknowledgedMessages {
	var messages []UnacknowledgedMessages
	db.Find(&messages, "device_address = ?", device_address)
	return messages
}
