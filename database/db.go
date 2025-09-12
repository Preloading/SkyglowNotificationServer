package db

import (
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed init.sql
var initDbSQL string

var db *sql.DB

type QueuedMessage struct {
	MessageId string
	CreatedAt time.Time

	// mostly copied from DataToSend
	IsEncrypted bool `json:"is_encrypted,omitempty" plist:"is_encrypted"`

	// Unencrypted message data
	AlertBody   *string `json:"message,omitempty" plist:"message"`
	AlertAction *string `json:"alert_action,omitempty" plist:"alert_action"`
	AlertSound  *string `json:"alert_sound,omitempty" plist:"alert_sound"`
	BadgeNumber *int    `json:"badge_number,omitempty" plist:"badge_number,omitempty"`
	// UserInfo      *interface{} `json:"user_info,omitempty" plist:"user_info"`       // https://developer.apple.com/documentation/uikit/uilocalnotification/userinfo?language=objc

	// Encrypted message data
	Ciphertext *[]byte `json:"ciphertext" plist:"ciphertext"`
	DataType   *string `json:"data_type" plist:"data_type"` // json or plist
	IV         *[]byte `json:"iv" plist:"iv"`

	// Routing info
	RoutingKey    []byte `json:"-" plist:"routing_key"`
	DeviceAddress string `json:"-" plist:"-"`
}

type NotificationToken struct {
	RoutingToken            []byte
	DeviceAddress           string
	FeedbackProviderAddress *string
	NotificationType        int    // Notifcation types, I'm not sure of the order yet but None, Badge, Sound, Alert
	AppBundleId             string // example: com.atebits.tweetie2
	IssuedAt                time.Time
	IsValid                 bool // Unsure if this should be kept
	MarkedForRemovalAt      *time.Time
	LastUsed                *time.Time
}

type Device struct {
	DeviceAddress string
	PublicKey     *rsa.PublicKey
	Language      string
}

type FeedbackToSend struct {
	FeedbackKey  []byte
	RoutingToken []byte
	Type         int
	Reason       string
	CreatedAt    time.Time
}

func ResetDatabase() error {
	_, err := db.Exec(initDbSQL)
	return err
}

func InitDB(dsn string) {
	// Initialize the database connection
	var err error
	db, err = sql.Open("pgx", dsn)

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
	_, err := db.Exec("DELETE FROM queued_messages WHERE message_id = $1 AND device_address = $2", message_id, device_uuid)
	return err
}

func QueueEncryptedMessage(m QueuedMessage) error {
	db.Exec("DELETE FROM queued_messages WHERE routing_key = $1", m.RoutingKey) // clean out old msgs.
	_, err := db.Exec("INSERT INTO queued_messages (message_id, created_at, is_encrypted, ciphertext, data_type, iv, device_address, routing_key) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		m.MessageId, m.CreatedAt, m.IsEncrypted, *m.Ciphertext, *m.DataType, *m.IV, m.DeviceAddress, m.RoutingKey,
	)

	return err
}

func QueueUnencryptedMessage(m QueuedMessage) error {
	db.Exec("DELETE FROM queued_messages WHERE routing_key = $1", m.RoutingKey) // clean out old msgs.
	_, err := db.Exec("INSERT INTO queued_messages (message_id, created_at, is_encrypted, alert_body, alert_action, alert_sound, badge_number, device_address, routing_key) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		m.MessageId, m.CreatedAt, m.IsEncrypted, *m.AlertBody, *m.AlertAction, *m.AlertSound, *m.BadgeNumber, m.DeviceAddress, m.RoutingKey,
	)

	return err
}

func SaveNewUser(device_address string, public_key rsa.PublicKey) error {
	encodedPubKey, err := x509.MarshalPKIXPublicKey(&public_key)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO devices (device_address, pub_key, lang) VALUES ($1, $2, $3)", device_address, encodedPubKey, "")
	if err != nil {
		panic(err)
	}
	return err
}

func UpdateLanguage(device_address string, language string) error {
	_, err := db.Exec("UPDATE devices WHERE device_address = $1 SET lang = $2", device_address, language)

	return err
}

func SaveNewToken(device_address string, routingToken []byte, bundleId string, notificationType int) error {
	_, err := db.Exec("INSERT INTO notification_tokens (device_address, bundle_id, routing_token, allowed_notification_types, is_valid, issued_at) VALUES ($1, $2, $3, $4, $5, $6)",
		device_address, bundleId, routingToken, notificationType, true, time.Now(),
	)
	fmt.Println(err)
	return err
}

func GetUser(device_address string) (*Device, error) {
	device := Device{}
	row := db.QueryRow("SELECT * FROM devices WHERE device_address = $1", device_address)

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

	row := db.QueryRow("SELECT * FROM notification_tokens WHERE routing_token = $1", routing_token)

	if err := row.Scan(&notificationToken.RoutingToken, &notificationToken.DeviceAddress, &notificationToken.FeedbackProviderAddress, &notificationToken.NotificationType, &notificationToken.AppBundleId, &notificationToken.IssuedAt, &notificationToken.IsValid, &notificationToken.LastUsed); err != nil {
		return nil, err
	}
	return &notificationToken, nil
}

func GetUnacknowledgedMessages(device_address string) ([]QueuedMessage, error) {
	var messages []QueuedMessage

	rows, err := db.Query("SELECT * FROM queued_messages WHERE device_address = $1", device_address)
	if err != nil {
		return messages, err
	}

	defer rows.Close()

	for rows.Next() {
		var message QueuedMessage
		if err := rows.Scan(&message.CreatedAt,
			&message.IsEncrypted, &message.AlertBody, &message.AlertAction, &message.AlertSound, &message.BadgeNumber, // Unencrypted info
			&message.Ciphertext, &message.DataType, &message.IV, // Encrypted info
			&message.DeviceAddress, &message.RoutingKey, &message.MessageId, // Routing info
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

// Feedback
func SaveNewFeedbackToken(routingToken []byte, server_address string, feedbackSecret []byte) error {
	if len(server_address) > 16 {
		return errors.New("server address too big")
	}

	_, err := db.Exec("INSERT INTO feedback_token (feedback_key, routing_token, routing_domain, last_used) VALUES ($1, $2, $3, $4)",
		routingToken, routingToken, feedbackSecret, time.Now(),
	)
	return err
}

func GetFeedbackWithSecret(feedbackSecret []byte, after *time.Time) ([]FeedbackToSend, error) {
	var feedbackToSend []FeedbackToSend

	latestTime := time.Now().Add(2 * time.Hour)
	if after == nil {
		after = &latestTime
	} else if after.Before(latestTime) {
		after = &latestTime
	}

	rows, err := db.Query("SELECT * FROM feedback_to_send WHERE feedback_key = $1 AND created_at >= $2", feedbackSecret, after)
	if err != nil {
		return feedbackToSend, err
	}

	defer rows.Close()

	for rows.Next() {
		var f FeedbackToSend
		if err := rows.Scan(
			&f.FeedbackKey, &f.RoutingToken, &f.Type, &f.Reason, &f.CreatedAt,
		); err != nil {
			return nil, err
		}
		feedbackToSend = append(feedbackToSend, f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return feedbackToSend, nil
}

func GetTokenFeedbackKey(routing_token []byte, serverAddress string) (*[]byte, error) {
	var feedback_key []byte

	row := db.QueryRow("SELECT feedback_key FROM feedback_token WHERE routing_token = $1 AND server_address = $2", routing_token, serverAddress)

	if err := row.Scan(feedback_key); err != nil {
		return nil, err
	}
	return &feedback_key, nil
}

func AddFeedback(routingToken []byte, feedbackSecret []byte, serverAddress string, typeOfFeedback int, reason string) error {
	if len(reason) > 64 {
		return errors.New("reason too big")
	}
	_, err := db.Exec("INSERT INTO feedback_to_send (feedback_key, routing_token, type, reason, created_at) VALUES ($1, $2, $3, $4, $5)",
		feedbackSecret, routingToken, typeOfFeedback, reason, time.Now(),
	)

	return err
}

func SetTokenFeedbackProviderAddress(routingToken []byte, feedbackServer string) error {
	if len(feedbackServer) > 16 {
		return errors.New("feedback server address too big")
	}
	_, err := db.Exec("UPDATE notification_tokens SET feedback_provider = $1 WHERE routing_token = $2 AND feedback_provider IS NULL",
		feedbackServer, routingToken,
	)

	return err
}
