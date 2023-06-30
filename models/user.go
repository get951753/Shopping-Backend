package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username    string `gorm:"unique;not null"`
	Email       string `gorm:"unique;not null"`
	Password    string `gorm:"not null"`
	Name        string
	Address     string
	Phone       string
	Cart        Cart
	Orders      []Order
	LoginTokens []LoginToken
	Role        string
}
