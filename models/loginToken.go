package models

import (
	"gorm.io/gorm"
	"time"
)

type LoginToken struct {
	gorm.Model
	Token          string
	ExpirationTime time.Time
	UserID         uint
	Role           string
}
