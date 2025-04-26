package db

import "gorm.io/gorm"

type UnacknowledgedMessages struct {
	gorm.Model
	ID        uint64 `gorm:"primaryKey"`
	MessageId string
	DeviceId  string
	Message   string
	CreatedAt string `gorm:"autoCreateTime"`
}

type Devices struct {
	gorm.Model
	ID   int    `gorm:"primaryKey"`
	UUID string `gorm:"index"`
}
