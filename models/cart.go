package models

import "gorm.io/gorm"

type Cart struct {
	gorm.Model
	UserID            uint
	AnonymousCartUUID string     `gorm:"unique"`
	CartItems         []CartItem `gorm:"foreignKey:CartID"`
}
