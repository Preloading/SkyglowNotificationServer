package db

import (
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed init.sql
var initDbSQL string

var db *sql.DB

type QueuedMessage struct {
	MessageId     string `gorm:"primaryKey"`
	DeviceAddress string
	RoutingKey    []byte
	Message       string
	Topic         string
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

type NotificationToken struct {
	RoutingToken     []byte `gorm:"primaryKey"`
	DeviceAddress    string
	NotificationType int       // Notifcation types, I'm not sure of the order yet but None, Badge, Sound, Alert
	AppBundleId      string    // example: com.atebits.tweetie2
	IssuedAt         time.Time `gorm:"autoCreateTime"`
	IsValid          bool      // Unsure if this should be kept
	LastUsed         *time.Time
}

type Device struct {
	DeviceAddress string `gorm:"primaryKey"`
	PublicKey     *rsa.PublicKey
	Language      string
}

func ResetDatabase() error {
	fmt.Println("Could not find tables, creating...")
	_, err := db.Exec(initDbSQL)
	return err
}

func InitDB(dsn string, database_type string) {
	// Initialize the database connection
	var err error
	switch database_type {
	case "sqlite":
		db, err = sql.Open("sqlite3", dsn)
	case "mysql":
		db, err = sql.Open("mysql", dsn)
	case "postgres":
		db, err = sql.Open("postgres", dsn)
	default:
		panic("unsupported database type")
	}
	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
		panic(pingErr)
	}

	err = ResetDatabase()
	if err != nil {
		panic(err)
	}
}

func AckMessage(message_id string, device_uuid string) error {
	_, err := db.Exec("DELETE FROM queued_messages WHERE message_id = ? AND device_address = ?", message_id, device_uuid)
	return err
}

func AddMessage(message_id string, message string, device_address string, topic string, routingKey []byte) error {
	_, err := db.Exec("INSERT INTO queued_messages (message_id, device_address, routing_key, message, topic, created_at) VALUES (?, ?, ?, ?, ?, ?)", message_id, device_address, routingKey, message, topic, time.Now())
	return err
}

func SaveNewUser(device_address string, public_key rsa.PublicKey) error {
	encodedPubKey, err := x509.MarshalPKIXPublicKey(&public_key)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO devices (device_address, pub_key, lang) VALUES (?, ?, ?)", device_address, encodedPubKey, "")
	if err != nil {
		panic(err)
	}
	return err
}

func UpdateLanguage(device_address string, language string) error {
	_, err := db.Exec("UPDATE devices WHERE device_address = ? SET lang = ?", device_address, language)

	return err
}

func SaveNewToken(device_address string, routingToken []byte, bundleId string, notificationType int) error {
	_, err := db.Exec("INSERT INTO notification_tokens (device_address, bundle_id, routing_token, allowed_notification_types, is_valid, issued_at) VALUES (?, ?, ?, ?, ?, ?)",
		device_address, bundleId, routingToken, notificationType, true, time.Now(),
	)
	fmt.Println(err)
	return err
}

func GetUser(device_address string) (*Device, error) {
	device := Device{}
	row := db.QueryRow("SELECT * FROM devices WHERE device_address = ?", device_address)

	byteKey := []byte{}
	if err := row.Scan(&device.DeviceAddress, &byteKey, &device.Language); err != nil {
		return nil, err
	}

	// Decode the public key
	// var err error
	parsedKey, err := x509.ParsePKIXPublicKey(byteKey)
	if err != nil {
		return nil, err
	}

	ok := false
	device.PublicKey, ok = parsedKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA public key")
	}

	return &device, nil
}

func GetToken(routing_token []byte) (*NotificationToken, error) {
	notificationToken := NotificationToken{}

	row := db.QueryRow("SELECT * FROM notification_tokens WHERE routing_token = ?", routing_token)

	if err := row.Scan(&notificationToken.RoutingToken, &notificationToken.DeviceAddress, &notificationToken.NotificationType, &notificationToken.AppBundleId, &notificationToken.IssuedAt, &notificationToken.IsValid, &notificationToken.LastUsed); err != nil {
		panic(err)
		return nil, err
	}
	return &notificationToken, nil
}

func GetUnacknowledgedMessages(device_address string) ([]QueuedMessage, error) {
	var messages []QueuedMessage

	rows, err := db.Query("SELECT * FROM queued_messages WHERE device_address = ?", device_address)
	if err != nil {
		return messages, err
	}

	defer rows.Close()

	for rows.Next() {
		var message QueuedMessage
		if err := rows.Scan(&message.MessageId, &message.DeviceAddress, &message.RoutingKey, &message.Message, &message.Topic, &message.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}
