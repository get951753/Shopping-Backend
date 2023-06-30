package models

import "gorm.io/gorm"

type Order struct {
	gorm.Model
	UserID         uint `gorm:"foreignKey:UserID"`
	User           User
	OrderItems     []OrderItem
	Total          uint   `gorm:"not null"`
	ShippingMethod string `gorm:"not null"`
	Name           string `gorm:"not null"`
	Address        string `gorm:"not null"`
	Phone          string `gorm:"not null"`
	Status         string `gorm:"not null"`
}
